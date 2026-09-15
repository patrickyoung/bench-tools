"""Attach bounded native PivotTable parts missing from the Artifact export API.

Only native-feature packaging; cell/layout/formula authoring stays in Artifact.
Specification: Microsoft Open XML, Working with PivotTables, 2025-01-14.
"""
import json,re,sys,zipfile,posixpath
from pathlib import Path
from xml.etree import ElementTree as E
from contract import read_xlsx,bounds,col,cell,require,digest,NS
from edit_contract import sheet_parts,xmlbytes
S=NS['s'];R='http://schemas.openxmlformats.org/officeDocument/2006/relationships';P='http://schemas.openxmlformats.org/package/2006/relationships';C='http://schemas.openxmlformats.org/package/2006/content-types'
def node(tag,attrs=None,parent=None):
    return E.SubElement(parent,'{'+S+'}'+tag,{k:str(v) for k,v in (attrs or {}).items()}) if parent is not None else E.Element('{'+S+'}'+tag,{k:str(v) for k,v in (attrs or {}).items()})
def typed(parent,v):
    if v is None or v=='':return node('m',parent=parent)
    if isinstance(v,bool):return node('b',{'v':int(v)},parent)
    return node('n' if isinstance(v,(int,float)) else 's',{'v':v},parent)
def restore_existing(root):
    req=json.loads((root/'request.json').read_text())
    if req.get('mode')!='edit':return
    with zipfile.ZipFile(root/req['workbook']) as z:
        old={n:z.read(n) for n in z.namelist()};old_sheets={a:b for a,b,_ in sheet_parts(z)}
    pivot_parts=[n for n in old if n.startswith(('xl/pivotTables/','xl/pivotCache/'))]
    if not pivot_parts:return
    path=root/'output/workbook.xlsx'
    with zipfile.ZipFile(path) as z:parts={n:z.read(n) for n in z.namelist()};sheets={a:b for a,b,_ in sheet_parts(z)}
    for n in list(parts):
        if n.startswith(('xl/pivotTables/','xl/pivotCache/')):del parts[n]
    for n in pivot_parts:parts[n]=old[n]
    def put(n,e):parts[n]=xmlbytes(e)
    def rels(n):return E.fromstring(parts[n]) if n in parts else E.Element('{'+P+'}Relationships')
    wb=E.fromstring(parts['xl/workbook.xml']);wr=rels('xl/_rels/workbook.xml.rels');ow=E.fromstring(old['xl/workbook.xml']);orr={e.get('Id'):e for e in E.fromstring(old['xl/_rels/workbook.xml.rels'])}
    for e in list(wb):
        if e.tag=='{'+S+'}pivotCaches':wb.remove(e)
    for e in list(wr):
        if e.get('Type','').endswith('/pivotCacheDefinition'):wr.remove(e)
    caches=ow.find('s:pivotCaches',NS);require(caches is not None,'Existing pivot cache registry missing')
    for i,e in enumerate(caches,1):
        rel=orr[e.get('{'+R+'}id')];rid='rIdPreservedPivotCache'+str(i);e.set('{'+R+'}id',rid);rel.set('Id',rid);wr.append(rel)
    idx=next((i for i,e in enumerate(wb) if e.tag.split('}')[-1] in ['smartTagPr','smartTagTypes','webPublishing','fileRecoveryPr','webPublishObjects','extLst']),len(wb));wb.insert(idx,caches)
    put('xl/workbook.xml',wb);put('xl/_rels/workbook.xml.rels',wr)
    for name,oldpart in old_sheets.items():
        sh=E.fromstring(old[oldpart]);pps=sh.find('s:pivotTableParts',NS)
        if pps is None:continue
        part=sheets[name];doc=E.fromstring(parts[part]);srp=posixpath.dirname(part)+'/_rels/'+posixpath.basename(part)+'.rels';sr=rels(srp)
        osrp=posixpath.dirname(oldpart)+'/_rels/'+posixpath.basename(oldpart)+'.rels';oldrels={e.get('Id'):e for e in E.fromstring(old[osrp])}
        for e in list(doc):
            if e.tag=='{'+S+'}pivotTableParts':doc.remove(e)
        for e in list(sr):
            if e.get('Type','').endswith('/pivotTable'):sr.remove(e)
        for i,e in enumerate(pps,1):
            rel=oldrels[e.get('{'+R+'}id')];rid='rIdPreservedPivot'+str(i);e.set('{'+R+'}id',rid);rel.set('Id',rid);sr.append(rel)
        doc.append(pps);put(part,doc);put(srp,sr)
    ct=E.fromstring(parts['[Content_Types].xml'])
    for e in list(ct):
        if 'pivot' in e.get('ContentType','').lower():ct.remove(e)
    for e in E.fromstring(old['[Content_Types].xml']):
        if 'pivot' in e.get('ContentType','').lower():ct.append(e)
    put('[Content_Types].xml',ct)
    with zipfile.ZipFile(path,'w',zipfile.ZIP_DEFLATED) as z:
        for n,b in parts.items():z.writestr(n,b)
    qa=json.loads((root/'output/qa.json').read_text());qa['workbook_sha256']=digest(path);qa['preserved_pivot_parts']=pivot_parts;(root/'output/qa.json').write_text(json.dumps(qa,indent=2)+'\n')
def attach(root):
    s=json.loads((root/'output/spec.json').read_text());pivots=s.get('pivots',[])
    if not pivots:return
    path=root/'output/workbook.xlsx';actual,_=read_xlsx(path)
    with zipfile.ZipFile(path) as z:parts={n:z.read(n) for n in z.namelist()};sheets={a:b for a,b,_ in sheet_parts(z)}
    wb=E.fromstring(parts['xl/workbook.xml']);wr=E.fromstring(parts['xl/_rels/workbook.xml.rels']);ct=E.fromstring(parts['[Content_Types].xml'])
    caches=wb.find('s:pivotCaches',NS)
    if caches is None:caches=node('pivotCaches',parent=wb)
    # ISO order: pivotCaches precedes smartTagPr, smartTagTypes, webPublishing,
    # fileRecoveryPr, webPublishObjects and extLst.
    wb.remove(caches);idx=next((i for i,x in enumerate(wb) if x.tag.split('}')[-1] in ['smartTagPr','smartTagTypes','webPublishing','fileRecoveryPr','webPublishObjects','extLst']),len(wb));wb.insert(idx,caches)
    def put(name,doc):parts[name]=xmlbytes(doc)
    def relation(doc,id,typ,target):E.SubElement(doc,'{'+P+'}Relationship',{'Id':id,'Type':R+'/'+typ,'Target':target})
    def content(name,typ):E.SubElement(ct,'{'+C+'}Override',{'PartName':'/'+name,'ContentType':'application/vnd.openxmlformats-officedocument.spreadsheetml.'+typ+'+xml'})
    summaries=[]
    offset=max([int(e.get('cacheId')) for e in caches]+[int(re.search(r'(\d+).xml$',n)[1]) for n in parts if re.fullmatch(r'xl/(?:pivotTables/pivotTable|pivotCache/pivotCacheDefinition)\d+.xml',n)]+[0])
    existing_names={E.fromstring(b).get('name') for n,b in parts.items() if re.fullmatch(r'xl/pivotTables/pivotTable\d+.xml',n)}
    for index,p in enumerate(pivots,offset+1):
        require(p['name'] not in existing_names,'Existing pivot layout changes require native adapter; choose unique name for additions')
        require(re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',p['name']),'Invalid pivot name');require(p['aggregate'] in ['sum','count','average','min','max'],'Unsupported pivot aggregate')
        r,c,Rr,Cc=bounds(p['source_range']);vs=actual[p['source_sheet']]
        headers=[vs.get(col(j)+str(r),{}).get('value') for j in range(c,Cc+1)]
        require(all(isinstance(x,str) and x for x in headers) and len(set(headers))==len(headers),'Pivot headers must be unique strings')
        ri=headers.index(p['row_field']);vi=headers.index(p['value_field']);require(ri!=vi,'Distinct pivot category/measure required')
        rows=[[vs.get(col(j)+str(i),{}).get('value') for j in range(c,Cc+1)] for i in range(r+1,Rr+1)]
        require(all(any(v is not None and v!='' for v in row) for row in rows),'Pivot range contains empty rows')
        require(all(row[ri] is not None and row[ri]!='' for row in rows),'Blank pivot category')
        require(all(row[vi] is None or row[vi]=='' or isinstance(row[vi],(int,float)) for row in rows),'Non-numeric pivot measure')
        unique=[]
        for j in range(len(headers)):
            vals=[]
            for row in rows:
                if row[j] not in vals:vals.append(row[j])
            unique.append(vals)
        cp=f'xl/pivotCache/pivotCacheDefinition{index}.xml';rp=f'xl/pivotCache/pivotCacheRecords{index}.xml';tp=f'xl/pivotTables/pivotTable{index}.xml'
        cache=node('pivotCacheDefinition',{'{'+R+'}id':'rId1','refreshOnLoad':'1','recordCount':len(rows),'createdVersion':'6','refreshedVersion':'6','minRefreshableVersion':'3'})
        source=node('cacheSource',{'type':'worksheet'},cache);node('worksheetSource',{'ref':p['source_range'],'sheet':p['source_sheet']},source)
        fields=node('cacheFields',{'count':len(headers)},cache)
        for h,vals in zip(headers,unique):
            f=node('cacheField',{'name':h,'numFmtId':'0'},fields);strings=any(isinstance(v,str) and v!='' for v in vals);numbers=any(isinstance(v,(int,float)) and not isinstance(v,bool) for v in vals);blanks=any(v is None or v=='' for v in vals)
            items=node('sharedItems',{'count':len(vals),'containsString':int(strings),'containsNumber':int(numbers),'containsBlank':int(blanks),'containsMixedTypes':int(strings and numbers),'containsSemiMixedTypes':int(blanks),'containsNonDate':'1'},f)
            for v in vals:typed(items,v)
        records=node('pivotCacheRecords',{'count':len(rows)})
        for row in rows:
            record=node('r',parent=records)
            for j,v in enumerate(row):node('x',{'v':unique[j].index(v)},record)
        ar,ac=cell(p['cell']);category_order=sorted(unique[ri],key=lambda x:str(x).lower());last=ar+len(category_order)+1
        pt=node('pivotTableDefinition',{'name':p['name'],'cacheId':index,'dataCaption':p['caption'],'grandTotalCaption':'Grand Total','rowGrandTotals':'1','colGrandTotals':'1','compact':'0','compactData':'0','gridDropZones':'0','multipleFieldFilters':'0','createdVersion':'6','updatedVersion':'6','minRefreshableVersion':'3','useAutoFormatting':'1','applyNumberFormats':'1','applyBorderFormats':'0','applyFontFormats':'0','applyPatternFormats':'0','applyAlignmentFormats':'0','applyWidthHeightFormats':'1'})
        node('location',{'ref':p['cell']+':'+col(ac+1)+str(last),'firstHeaderRow':'1','firstDataRow':'1','firstDataCol':'1'},pt)
        pf=node('pivotFields',{'count':len(headers)},pt)
        for j,h in enumerate(headers):
            f=node('pivotField',{'axis':'axisRow','showAll':'0','defaultSubtotal':'0','sortType':'ascending'} if j==ri else {'dataField':'1','showAll':'0'} if j==vi else {'showAll':'0'},pf)
            if j==ri:
                its=node('items',{'count':len(unique[j])+1},f)
                for v in category_order:node('item',{'x':unique[j].index(v)},its)
                node('item',{'t':'default'},its)
        rf=node('rowFields',{'count':1},pt);node('field',{'x':ri},rf)
        rit=node('rowItems',{'count':len(category_order)+1},pt)
        for k in range(len(category_order)):node('x',{'v':k},node('i',parent=rit))
        node('x',parent=node('i',{'t':'grand'},rit))
        df=node('dataFields',{'count':1},pt);node('dataField',{'name':p['caption'],'fld':vi,'subtotal':p['aggregate'],'baseField':0,'baseItem':0,'numFmtId':4},df)
        node('pivotTableStyleInfo',{'name':'PivotStyleMedium9','showRowHeaders':'1','showColHeaders':'1','showRowStripes':'0','showColStripes':'0','showLastColumn':'1'},pt)
        crels=E.Element('{'+P+'}Relationships');relation(crels,'rId1','pivotCacheRecords',posixpath.basename(rp));put(posixpath.dirname(cp)+'/_rels/'+posixpath.basename(cp)+'.rels',crels)
        trels=E.Element('{'+P+'}Relationships');relation(trels,'rId1','pivotCacheDefinition','../pivotCache/'+posixpath.basename(cp));put(posixpath.dirname(tp)+'/_rels/'+posixpath.basename(tp)+'.rels',trels)
        rid='rIdBenchPivot'+str(index);relation(wr,rid,'pivotCacheDefinition','pivotCache/'+posixpath.basename(cp));node('pivotCache',{'cacheId':index,'{'+R+'}id':rid},caches)
        sheetpart=sheets[p['sheet']];srp=posixpath.dirname(sheetpart)+'/_rels/'+posixpath.basename(sheetpart)+'.rels';sr=E.fromstring(parts[srp]) if srp in parts else E.Element('{'+P+'}Relationships');relation(sr,rid,'pivotTable','../pivotTables/'+posixpath.basename(tp));put(srp,sr)
        sh=E.fromstring(parts[sheetpart]);pps=sh.find('s:pivotTableParts',NS)
        if pps is None:pps=node('pivotTableParts',{'count':'0'},sh)
        node('pivotTablePart',{'{'+R+'}id':rid},pps);pps.set('count',str(len(pps)));put(sheetpart,sh)
        for n,d,t in [(cp,cache,'pivotCacheDefinition'),(rp,records,'pivotCacheRecords'),(tp,pt,'pivotTable')]:put(n,d);content(n,t)
        summaries.append({'name':p['name'],'source_rows':len(rows),'categories':len(category_order),'source_range':p['source_range'],'native_parts':[cp,rp,tp]})
    put('xl/workbook.xml',wb);put('xl/_rels/workbook.xml.rels',wr);put('[Content_Types].xml',ct)
    with zipfile.ZipFile(path,'w',zipfile.ZIP_DEFLATED) as z:
        for n,b in parts.items():z.writestr(n,b)
    qa=json.loads((root/'output/qa.json').read_text());qa['workbook_sha256']=digest(path);qa['native_pivots']=summaries;(root/'output/qa.json').write_text(json.dumps(qa,indent=2)+'\n')
def verify_pivots(root,spec):
    pivots=spec.get('pivots',[]);report=[];actual,_=read_xlsx(root/'output/workbook.xlsx')
    with zipfile.ZipFile(root/'output/workbook.xlsx') as z:
        parts=[n for n in z.namelist() if re.fullmatch(r'xl/pivotTables/pivotTable\d+.xml',n)]
        req=json.loads((root/'request.json').read_text());old_count=0;offset=0
        if req.get('mode')=='edit':
            with zipfile.ZipFile(root/req['workbook']) as orig:
                old_parts=[n for n in orig.namelist() if re.fullmatch(r'xl/pivotTables/pivotTable\d+.xml',n)];old_count=len(old_parts)
                offset=max([int(E.fromstring(orig.read(n)).get('cacheId')) for n in old_parts]+[int(re.search(r'(\d+).xml$',n)[1]) for n in orig.namelist() if re.fullmatch(r'xl/(?:pivotTables/pivotTable|pivotCache/pivotCacheDefinition)\d+.xml',n)]+[0])
                for n in orig.namelist():
                    if n.startswith(('xl/pivotTables/','xl/pivotCache/')):require(n in z.namelist() and orig.read(n)==z.read(n),'Existing pivot/cache part changed: '+n)
        require(len(parts)==old_count+len(pivots),'Native PivotTable count mismatch')
        for i,p in enumerate(pivots,offset+1):
            pt=E.fromstring(z.read(f'xl/pivotTables/pivotTable{i}.xml'));cache=E.fromstring(z.read(f'xl/pivotCache/pivotCacheDefinition{i}.xml'));records=E.fromstring(z.read(f'xl/pivotCache/pivotCacheRecords{i}.xml'))
            require(pt.get('name')==p['name'],'Native pivot name mismatch');ws=cache.find('s:cacheSource/s:worksheetSource',NS);require(ws.get('ref')==p['source_range'] and ws.get('sheet')==p['source_sheet'],'Native pivot source mismatch')
            r,c,Rr,Cc=bounds(p['source_range']);v=actual[p['source_sheet']];headers=[v[col(j)+str(r)]['value'] for j in range(c,Cc+1)];ri=headers.index(p['row_field']);vi=headers.index(p['value_field'])
            require(int(pt.find('s:rowFields/s:field',NS).get('x'))==ri,'Native row field mismatch');df=pt.find('s:dataFields/s:dataField',NS);require(int(df.get('fld'))==vi and df.get('subtotal')==p['aggregate'],'Native value field mismatch')
            rows=[[v.get(col(j)+str(k),{}).get('value') for j in range(c,Cc+1)] for k in range(r+1,Rr+1)]
            require(len(records)==len(rows)==int(cache.get('recordCount')),'Native pivot cache row count mismatch')
            fields=cache.find('s:cacheFields',NS);shared=[]
            for field in fields:
                vals=[]
                for x in field.find('s:sharedItems',NS):
                    typ=x.tag.split('}')[-1];vals.append(None if typ=='m' else float(x.get('v')) if typ=='n' else x.get('v')=='1' if typ=='b' else x.get('v'))
                shared.append(vals)
            for rec,row in zip(records,rows):require([shared[j][int(x.get('v'))] for j,x in enumerate(rec)]==[None if v=='' else v for v in row],'Pivot cache/source data mismatch')
            groups={}
            for row in rows:groups.setdefault(row[ri],[]).extend([row[vi]] if isinstance(row[vi],(int,float)) else [])
            def agg(a):
                if p['aggregate']=='count':return len(a)
                if not a:return None
                return sum(a) if p['aggregate']=='sum' else sum(a)/len(a) if p['aggregate']=='average' else min(a) if p['aggregate']=='min' else max(a)
            ar,ac=cell(p['cell']);dest=actual[p['sheet']];seen={}
            for k in range(ar+1,ar+1+len(groups)):seen[dest[col(ac)+str(k)]['value']]=dest[col(ac+1)+str(k)]['value']
            require(seen=={k:agg(a) for k,a in groups.items()},'Pivot displayed groups mismatch')
            require(dest[col(ac+1)+str(ar+len(groups)+1)]['value']==agg([x for a in groups.values() for x in a]),'Pivot grand total mismatch')
            report.append({'name':p['name'],'rows':len(rows),'groups':len(groups),'passed':True})
    return report
def zero_baselines(root):
    # Native axis bounds fill a missing documented API operation. Apply only to
    # newly created charts, preserving existing chart encodings on narrow edits.
    req=json.loads((root/'request.json').read_text());old_count=0
    if req.get('mode')=='edit':old_count=read_xlsx(root/req['workbook'])[1]['charts']
    path=root/'output/workbook.xlsx';actual,_=read_xlsx(path)
    with zipfile.ZipFile(path) as z:parts={n:z.read(n) for n in z.namelist()}
    Cn='http://schemas.openxmlformats.org/drawingml/2006/chart';N={'c':Cn};changed=[]
    charts=sorted([n for n in parts if re.fullmatch(r'xl/(?:drawings/)?charts/chart\d+.xml',n)],key=lambda n:int(re.search(r'(\d+).xml$',n)[1]))
    for n in charts[old_count:]:
        doc=E.fromstring(parts[n]);kind=doc.find('.//c:barChart',N)
        if kind is None:kind=doc.find('.//c:areaChart',N)
        if kind is None:continue
        nums=[float(x.text) for x in kind.findall('.//c:val/c:numRef/c:numCache/c:pt/c:v',N) if x.text]
        if not nums:
            for ref in kind.findall('.//c:val/c:numRef/c:f',N):
                if not ref.text or '!' not in ref.text:continue
                sheet,area=ref.text.rsplit('!',1);sheet=sheet.strip("'").replace("''", "'")
                rr,cc,RR,CC=bounds(area.replace('$',''))
                nums += [x for row in range(rr,RR+1) for column in range(cc,CC+1) if isinstance(x:=actual.get(sheet,{}).get(col(column)+str(row),{}).get('value'),(int,float))]
        if not nums:continue
        for axis in doc.findall('.//c:valAx',N):
            scaling=axis.find('c:scaling',N)
            if scaling is None:scaling=E.SubElement(axis,'{'+Cn+'}scaling')
            # Include zero, preserve negative observations, leave upper positive
            # extent automatic. Native Excel refresh may require axis review if
            # future values introduce a new negative range.
            minimum=min(0,min(nums));old=scaling.find('c:min',N)
            if old is None:old=E.SubElement(scaling,'{'+Cn+'}min')
            old.set('val',str(minimum))
            if max(nums)<0:
                upper=scaling.find('c:max',N)
                if upper is None:upper=E.SubElement(scaling,'{'+Cn+'}max')
                upper.set('val','0')
        parts[n]=E.tostring(doc,encoding='utf-8',xml_declaration=True);changed.append(n)
    if changed:
        with zipfile.ZipFile(path,'w',zipfile.ZIP_DEFLATED) as z:
            for n,b in parts.items():z.writestr(n,b)
    qa=json.loads((root/'output/qa.json').read_text());qa['workbook_sha256']=digest(path);qa['zero_baseline_charts']=changed;(root/'output/qa.json').write_text(json.dumps(qa,indent=2)+'\n')
if __name__=='__main__':restore_existing(Path.cwd());attach(Path.cwd());zero_baselines(Path.cwd())
