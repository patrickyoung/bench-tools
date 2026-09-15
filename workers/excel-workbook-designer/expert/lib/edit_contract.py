"""Bounded prompt edits, workbook inventory and preservation checks. No code eval."""
import json,re,sys,zipfile,posixpath
from pathlib import Path
from xml.etree import ElementTree as E
from contract import require,digest,input_bindings,bounds,cell,col,read_xlsx,NS,ERR
S=NS['s'];R='http://schemas.openxmlformats.org/officeDocument/2006/relationships'
def xmlbytes(doc):
    namespace=doc.tag.split('}')[0].lstrip('{')
    if namespace in ['http://schemas.openxmlformats.org/package/2006/content-types','http://schemas.openxmlformats.org/package/2006/relationships']:E.register_namespace('',namespace)
    return E.tostring(doc,encoding='utf-8',xml_declaration=True)
def xml(z,p):return E.fromstring(z.read(p))
def relpath(part,target):return target.lstrip('/') if target.startswith('/') else posixpath.normpath(posixpath.join(posixpath.dirname(part),target))
def sheet_parts(z):
    rels={r.get('Id'):relpath('xl/workbook.xml',r.get('Target')) for r in xml(z,'xl/_rels/workbook.xml.rels')}
    return [(s.get('name'),rels[s.get('{'+R+'}id')],dict(s.attrib)) for s in xml(z,'xl/workbook.xml').find('s:sheets',NS)]
def canon(e):
    if e is None:return None
    return [e.tag.split('}')[-1],sorted(e.attrib.items()),(e.text or '').strip(),[canon(x) for x in e]]
def style_map(z):
    st=xml(z,'xl/styles.xml');formats={int(x.get('numFmtId')):x.get('formatCode') for x in st.findall('s:numFmts/s:numFmt',NS)}
    groups={k:list(st.find('s:'+k,NS)) if st.find('s:'+k,NS) is not None else [] for k in ['fonts','fills','borders','cellXfs']}
    result=[]
    for x in groups['cellXfs']:
        result.append({'font':canon(groups['fonts'][int(x.get('fontId','0'))]),'fill':canon(groups['fills'][int(x.get('fillId','0'))]),'border':canon(groups['borders'][int(x.get('borderId','0'))]),'numFmt':formats.get(int(x.get('numFmtId','0')),int(x.get('numFmtId','0'))),'alignment':canon(x.find('s:alignment',NS)),'protection':canon(x.find('s:protection',NS))})
    return result
def inventory(path):
    values,stats=read_xlsx(path);out={'sha256':digest(path),'sheets':[],'features':stats,'blockers':[]}
    with zipfile.ZipFile(path) as z:
        require(sum(i.file_size for i in z.infolist())<=100_000_000,'Expanded workbook exceeds 100MB')
        unsupported=['vba','externallinks/','connections.xml','querytables/','slicer','model/','comments','threadedcomments','customxml/','_xmlsignatures','ctrlprops/','embeddings/','printersettings/']
        out['blockers'] += ['Unsupported preserving import: '+n for n in z.namelist() if any(t in n.lower() for t in unsupported)]
        wb=xml(z,'xl/workbook.xml');out['names']=canon(wb.find('s:definedNames',NS));out['calc']=dict(wb.find('s:calcPr',NS).attrib) if wb.find('s:calcPr',NS) is not None else {}
        if wb.find('s:workbookProtection',NS) is not None:out['blockers'].append('Workbook protection')
        styles=style_map(z)
        for name,part,attrs in sheet_parts(z):
            sh=xml(z,part);cells=values[name];require(len(cells)<=50000,'Too many populated/styled cells for bounded editing')
            for c in sh.findall('.//s:sheetData/s:row/s:c',NS):cells[c.get('r')]['style']=styles[int(c.get('s','0'))]
            if sh.find('s:sheetProtection',NS) is not None:out['blockers'].append(name+': sheet protection')
            if sh.findall('.//s:f[@t]',NS):out['blockers'].append(name+': array/shared/data-table formula requires native adapter')
            item={'name':name,'part':part,'state':attrs.get('state','visible'),'cells':cells,'tables':[],'charts':[],
                  'validations':canon(sh.find('s:dataValidations',NS)),'conditional_formats':[canon(x) for x in sh.findall('s:conditionalFormatting',NS)],
                  'panes':[canon(x) for x in sh.findall('.//s:pane',NS)],'merges':canon(sh.find('s:mergeCells',NS)),
                  'rows':[{k:v for k,v in x.attrib.items() if k!='spans'} for x in sh.findall('s:sheetData/s:row',NS) if any(k in x.attrib for k in ('hidden','outlineLevel'))],
                  'row_heights':{x.get('r'):x.get('ht') for x in sh.findall('s:sheetData/s:row',NS) if x.get('ht')},
                  'columns':[dict(x.attrib) for x in sh.findall('s:cols/s:col',NS)],
                  'preview_range':'A1:'+col(min(14,max([cell(a)[1] for a in cells] or [10])))+str(min(30,max([cell(a)[0] for a in cells] or [20])))}
            rp=posixpath.dirname(part)+'/_rels/'+posixpath.basename(part)+'.rels'
            if rp in z.namelist():
                for rel in xml(z,rp):
                    if rel.get('TargetMode')=='External':out['blockers'].append(name+': external relationship')
                    target=relpath(part,rel.get('Target'))
                    if rel.get('Type','').endswith('/table'):
                        t=xml(z,target);item['tables'].append({'name':t.get('name'),'range':t.get('ref'),'headers':[x.get('name') for x in t.findall('s:tableColumns/s:tableColumn',NS)],'totals':t.get('totalsRowCount','0')})
                        if t.get('totalsRowCount','0')!='0':out['blockers'].append(name+': existing totals row requires native preserving adapter')
            out['sheets'].append(item)
        chartns={'c':'http://schemas.openxmlformats.org/drawingml/2006/chart'}
        out['charts']=[{'part':n,'references':[x.text for x in xml(z,n).findall('.//c:f',chartns)],'xml':canon(xml(z,n))} for n in z.namelist() if re.fullmatch(r'xl/(?:drawings/)?charts/chart\d+.xml',n)]
        out['pivots']=[{'name':xml(z,n).get('name'),'part':n,'xml':canon(xml(z,n))} for n in z.namelist() if re.fullmatch(r'xl/pivotTables/pivotTable\d+.xml',n)]
        for n in z.namelist():
            if re.fullmatch(r'xl/drawings/drawing\d+.xml',n):
                if any(x.tag.split('}')[-1] in ('pic','sp','cxnSp','grpSp') for x in xml(z,n).iter()):out['blockers'].append('Non-chart drawing '+n)
    return out
def contained(a,b):
    r,c,R,C=bounds(a);x,y,X,Y=bounds(b);return x<=r<=R<=X and y<=c<=C<=Y
def affected(s,sh,a):return any(x['sheet']==sh and contained(a,x['range']) for x in s['changes'])
def operation_affects(s,sh,a,kinds):return any(o['op'] in kinds and o['sheet']==sh and contained(a,o['range']) for o in s['operations'])
def prepare_import(root):
    # Artifact import/render is unreliable for native pivots. Remove only their
    # package metadata from a disposable import copy. Original pivot and cache
    # bytes are restored after export by features.restore_existing.
    req=json.loads((root/'request.json').read_text());input_bindings(root,req)
    with zipfile.ZipFile(root/req['workbook']) as z:parts={n:z.read(n) for n in z.namelist()}
    for n in list(parts):
        if n.startswith(('xl/pivotTables/','xl/pivotCache/')):del parts[n];continue
        if not (n.endswith('.xml') or n.endswith('.rels')):continue
        if not (n=='[Content_Types].xml' or n=='xl/workbook.xml' or n.endswith('.rels') or n.startswith('xl/worksheets/')):continue
        doc=E.fromstring(parts[n]);changed=False
        for parent in doc.iter():
            for e in list(parent):
                if e.tag.split('}')[-1] in ('pivotCaches','pivotTableParts') or e.tag.split('}')[-1]=='Relationship' and 'pivot' in e.get('Type','').lower() or e.tag.split('}')[-1]=='Override' and 'pivot' in e.get('ContentType','').lower():parent.remove(e);changed=True
        if changed:parts[n]=xmlbytes(doc)
    (root/'.runtime').mkdir(exist_ok=True)
    with zipfile.ZipFile(root/'.runtime/import.xlsx','w',zipfile.ZIP_DEFLATED) as z:
        for n,b in parts.items():z.writestr(n,b)
def validate(root):
    req=json.loads((root/'request.json').read_text());inputs=input_bindings(root,req);s=json.loads((root/'output/spec.json').read_text())
    require(req.get('mode')=='edit' and req.get('workbook') in [x['path'] for x in inputs],'Edit needs bound workbook')
    require(s.get('schema')=='bench.workbook-edit/v1','Invalid edit schema');require(s['request_sha256']==digest(root/'request.json') and s['inputs']==inputs,'Stale edit bindings')
    inv=inventory(root/req['workbook']);require(not inv['blockers'],'Cannot preserve: '+', '.join(inv['blockers'][:5]))
    names=[x['name'] for x in inv['sheets']];known={x['name']:x for x in inv['sheets']}
    require(s.get('purpose') and s.get('interpretation') and s.get('changes'),'Missing edit plan')
    for ch in s['changes']:bounds(ch['range']);require(ch.get('reason'),'Change needs reason')
    allowed={'add_sheet','values','formulas','copy','format','table','validation','conditional_format','chart','chart_data'}
    for op in s['operations']:
        require(op['op'] in allowed,'Unknown operation')
        if op['op']=='add_sheet':
            require(op['sheet'] not in names and 0<len(op['sheet'])<=31 and not re.search(r'[\[\]:*?/\\]',op['sheet']),'Invalid added sheet');names.append(op['sheet']);continue
        require(op['sheet'] in names,'Unknown operation sheet');bounds(op['range'])
        if op['op'] not in ('chart','chart_data'):require(affected(s,op['sheet'],op['range']),'Operation outside scope '+op['range'])
        if op['op'] in ('values','formulas'):
            a=op[op['op']];r,c,R,C=bounds(op['range']);require(len(a)==R-r+1 and all(len(row)==C-c+1 for row in a),'Bad matrix shape')
            if op['op']=='formulas':
                for row in a:
                    for f in row:require(isinstance(f,str) and f.startswith('=') and not re.search(r'\[|https?:|WEBSERVICE|DDE|RTD|HYPERLINK',f,re.I),'Unsafe/unsupported edit formula')
        if op['op']=='table':
            old=next((t for t in known.get(op['sheet'],{}).get('tables',[]) if t['name']==op['name']),None)
            if old:require(bounds(old['range'])[:2]==bounds(op['range'])[:2],'Table start cannot move')
        if op['op']=='format' and 'columnWidth' in op['format']:require(0<op['format']['columnWidth']<=80,'columnWidth is character units, not pixels; keep compact')
        if op['op']=='chart':
            require(op['type'] in ['pie','doughnut','bar','line','area','scatter'],'Unsupported chart type');cell(op['from']);cell(op['to'])
    require(len(names)<=8,'Too many sheets')
    require(s.get('metrics') and s.get('assertions'),'Need metrics and independent baseline assertions');ids={m['id'] for m in s['metrics']}
    require(len(ids)==len(s['metrics']) and set(req.get('required_metrics',[]))<=ids,'Required/duplicate metrics')
    for m in s['metrics']:require(m['sheet'] in names,'Unknown metric sheet');cell(m['cell'])
    require(all(e['metric'] in ids for e in s['assertions']),'Unknown assertion')
    live=any(o['op'] in ['formulas','chart','chart_data'] for o in s['operations']) or bool(s.get('pivots'))
    require(not live or s.get('tests'),'Live changes require mutation test')
    for t in s['tests']:
        require(t.get('name') and t.get('edits') and t.get('expect'),'Incomplete test')
        for e in t['edits']:require(e['sheet'] in names,'Unknown mutation sheet');cell(e['cell'])
        require(all(e['metric'] in ids for e in t['expect']),'Unknown test metric')
    require(s.get('previews') and all(p['sheet'] in names for p in s['previews']),'Missing previews')
    for p in s['previews']:bounds(p['range'])
    for m in s.get('mappings',[]):require(m.get('confidence') in ['high','medium'] and all(m.get(k) for k in ['source','column','target','reason']),'Unresolved or incomplete mapping')
    return req,s,inv
def check(root):
    req,s,before=validate(root);qa=json.loads((root/'output/qa.json').read_text());after=inventory(root/'output/workbook.xlsx')
    from features import verify_pivots
    pivots=verify_pivots(root,s)
    require(qa['spec_sha256']==digest(root/'output/spec.json') and qa['workbook_sha256']==digest(root/'output/workbook.xlsx'),'Stale edit output')
    require(len(qa['assertions'])==len(s['assertions']) and all(x['passed'] for x in qa['assertions']),'Assertion failure')
    require(len(qa['tests'])==len(s['tests']) and all(x['passed'] for x in qa['tests']),'Mutation failure')
    for p in qa['previews']:require(digest(root/p['path'])==p['sha256'],'Stale preview')
    require(len(qa['previews'])==len(s['previews']),'Missing preview')
    original=[x['name'] for x in before['sheets']];require([x['name'] for x in after['sheets']][:len(original)]==original,'Unrequested sheet change')
    checks=[];errors=[]
    def verify(ok,message):
        checks.append({'check':message,'passed':bool(ok)})
        if not ok:errors.append(message)
    for a,b in zip(before['sheets'],after['sheets']):
        for addr,v in a['cells'].items():
            w=b['cells'].get(addr,{'value':None,'formula':None,'style':v['style']})
            if not operation_affects(s,a['name'],addr,{'values','formulas','copy'}):verify(v['formula']==w.get('formula') and (v['formula'] is not None or v['value']==w.get('value')),'Preserved value/formula '+a['name']+'!'+addr)
            if not operation_affects(s,a['name'],addr,{'format','copy','values'}):verify(v['style']==w.get('style'),'Preserved style '+a['name']+'!'+addr)
        for addr in b['cells']:
            if addr not in a['cells'] and b['cells'][addr].get('value') is not None:verify(affected(s,a['name'],addr),'New cell in scope '+a['name']+'!'+addr)
        for key in ['state','panes','merges','rows']:verify(a[key]==b[key],f'Preserved {a["name"]} {key}')
        for rn,height in a['row_heights'].items():
            changed=any(o['sheet']==a['name'] and o['op'] in ('format','copy') and bounds(o['range'])[0]<=int(rn)<=bounds(o['range'])[2] for o in s['operations'])
            if not changed:verify(b['row_heights'].get(rn)==height,f'Preserved {a["name"]} row {rn} height')
        def columns(sh):
            return {i:{k:v for k,v in c.items() if k not in ('min','max')} for c in sh['columns'] for i in range(int(c['min']),min(100,int(c['max']))+1)}
        ca,cb=columns(a),columns(b)
        for cn,props in ca.items():
            changed=any(o['sheet']==a['name'] and o['op']=='format' and any(k in o['format'] for k in ('columnWidth','columnWidthPx')) and bounds(o['range'])[1]<=cn<=bounds(o['range'])[3] for o in s['operations'])
            if not changed:verify(props==cb.get(cn),f'Preserved {a["name"]} column {col(cn)} width/state')
        for key,opname in [('validations','validation'),('conditional_formats','conditional_format')]:
            if not any(o['op']==opname and o['sheet']==a['name'] for o in s['operations']):verify(a[key]==b[key],f'Preserved {a["name"]} {key}')
        editedtables={o['name'] for o in s['operations'] if o['op']=='table' and o['sheet']==a['name']}
        for t in a['tables']:
            if t['name'] not in editedtables:verify(t in b['tables'],'Preserved table '+t['name'])
    verify(before['names']==after['names'],'Preserved named ranges')
    for p in before['pivots']:verify(p in after['pivots'],'Preserved native pivot '+p['name'])
    verify(after['features']['charts']==before['features']['charts']+sum(o['op']=='chart' for o in s['operations']),'Native chart count')
    verify(after['features']['charts_with_cell_references']==after['features']['charts'],'All charts cell-bound')
    if not any(o['op']=='chart_data' for o in s['operations']):
        def without_caches(v):
            if isinstance(v,list) and len(v)==4 and isinstance(v[0],str):return [v[0],v[1],v[2],[without_caches(x) for x in v[3] if x[0] not in ('numCache','strCache')]]
            return v
        for i,ch in enumerate(before['charts']):verify(without_caches(ch['xml'])==without_caches(after['charts'][i]['xml']),'Preserved chart structure/style '+str(i))
    actual,_=read_xlsx(root/'output/workbook.xlsx')
    expected_cells={}
    for o in s['operations']:
        if o['op'] not in ('values','formulas'):continue
        r,c,R,C=bounds(o['range'])
        for i,row in enumerate(o[o['op']]):
            for j,v in enumerate(row):expected_cells[(o['sheet'],col(c+j)+str(r+i))]=(o['op'],v)
    from datetime import date
    for (sh,a),(kind,v) in expected_cells.items():
        obs=actual[sh].get(a,{});want=v
        if isinstance(v,dict):want=(date.fromisoformat(v['date'])-date(1899,12,30)).days
        verify(obs.get('formula')==v[1:] if kind=='formulas' else obs.get('value')==want,'Requested '+kind+' '+sh+'!'+a)
    for o in s['operations']:
        if o['op']=='table':verify(any(t['name']==o['name'] and t['range']==o['range'] for sh in after['sheets'] if sh['name']==o['sheet'] for t in sh['tables']),'Requested table '+o['name'])
    for m in s['metrics']:verify(actual[m['sheet']].get(m['cell'],{}).get('value')==qa['baseline'][m['id']],'Saved metric '+m['id'])
    for assertion in s['assertions']:
        v=qa['baseline'].get(assertion['metric']);w=assertion['value'];ok=abs(v-w)<=(assertion.get('tolerance',1e-6)) if isinstance(v,(int,float)) and isinstance(w,(int,float)) else v==w
        verify(ok,'Independent asserted baseline '+assertion['metric'])
    for sh,cells in actual.items():
        for a,x in cells.items():
            old=next((z for z in before['sheets'] if z['name']==sh),{}).get('cells',{}).get(a,{})
            if x['type']=='e' or isinstance(x['value'],str) and x['value'] in ERR:verify(x==old,'No new formula error '+sh+'!'+a)
    report={'source_sha256':before['sha256'],'output_sha256':after['sha256'],'changes':s['changes'],'mappings':s.get('mappings',[]),'checks':checks,'pivots':pivots,'passed':not errors,'limitations':s.get('limitations',[])}
    (root/'output/change-report.json').write_text(json.dumps(report,indent=2)+'\n')
    require(not errors,'Preservation/output failure: '+'; '.join(errors[:8]));require((root/'output/guide.md').stat().st_size>50,'Missing guide')
    return {'status':'accepted-mechanically','preservation_checks':len(checks),'native':after['features'],'metrics':qa['baseline'],'limits':'Independent semantic, visual and native Excel review required'}
if __name__=='__main__':
    root=Path.cwd()
    try:
        if '--prepare-import' in sys.argv:prepare_import(root)
        elif '--inspect' in sys.argv:
            req=json.loads((root/'request.json').read_text());input_bindings(root,req);require(req['workbook'] in [x['path'] for x in req['inputs']],'Unbound workbook');inv=inventory(root/req['workbook']);(root/'inspection').mkdir(exist_ok=True);(root/'inspection/workbook.json').write_text(json.dumps(inv,indent=2)+'\n');print(json.dumps({'sheets':[x['name'] for x in inv['sheets']],'features':inv['features'],'blockers':inv['blockers']}))
        elif '--spec' in sys.argv:validate(root);print('Valid bound edit spec')
        else:print(json.dumps(check(root)))
    except Exception as e:print(str(e),file=sys.stderr);sys.exit(1)
