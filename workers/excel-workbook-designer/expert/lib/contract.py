"""Trusted read-only contract. Never executes worker-authored code."""
import hashlib, json, math, re, sys, zipfile
from datetime import date
from pathlib import Path
from xml.etree import ElementTree as ET
NS={'s':'http://schemas.openxmlformats.org/spreadsheetml/2006/main'}
ERR={'#REF!','#DIV/0!','#VALUE!','#NAME?','#N/A','#NUM!','#NULL!','#SPILL!','#CALC!'}
def digest(p): return hashlib.sha256(p.read_bytes()).hexdigest()
def require(ok,msg):
    if not ok: raise ValueError(msg)
def cell(s):
    m=re.fullmatch(r'([A-Z]{1,3})([1-9][0-9]{0,4})',s);require(m is not None,'Invalid cell '+str(s));c=0
    for ch in m[1]:c=c*26+ord(ch)-64
    return int(m[2]),c
def bounds(s):
    a=s.split(':');require(len(a)<=2,'Invalid range');r,c=cell(a[0]);R,C=cell(a[-1])
    require(0<r<=R<=10000 and 0<c<=C<=100,'Range exceeds bounds');return r,c,R,C
def col(n):
    s=''
    while n:n,k=divmod(n-1,26);s=chr(65+k)+s
    return s
def input_bindings(root,req):
    require(req.get('schema')=='bench.workbook-request/v1','Invalid request')
    require(req.get('brief') and req.get('inputs'),'Missing brief or inputs');seen=set()
    for x in req['inputs']:
        p=Path(x['path']);f=root/p
        require(not p.is_absolute() and p.parts[0]=='inputs' and '..' not in p.parts,'Invalid input path')
        require(x['path'] not in seen,'Duplicate input');seen.add(x['path'])
        require(f.is_file() and not f.is_symlink() and f.resolve().is_relative_to(root.resolve()),'Missing/unsafe input')
        require(f.stat().st_size<=20_000_000 and digest(f)==x['sha256'],'Input size/hash mismatch '+str(p))
    return sorted(req['inputs'],key=lambda x:x['path'])
def validate_spec(root):
    root=Path(root);req=json.loads((root/'request.json').read_text());inputs=input_bindings(root,req)
    p=root/'output/spec.json';require(p.stat().st_size<=2_000_000,'Spec too large');s=json.loads(p.read_text())
    require(s.get('schema')=='bench.workbook-spec/v1','Invalid spec schema')
    require(s.get('request_sha256')==digest(root/'request.json'),'Stale request binding')
    require(s.get('inputs')==inputs,'Stale/incomplete input bindings')
    for k in ['title','purpose','audience','target_engine','design_notes','update_policy','limitations']:require(k in s,'Missing '+k)
    require(1<=len(s['sheets'])<=8,'Need 1-8 meaningful sheets');names=[x['name'] for x in s['sheets']]
    require(len(names)==len(set(n.lower() for n in names)),'Duplicate sheet')
    require(all(0<len(n)<=31 and not re.search(r'[\[\]:*?/\\]',n) for n in names),'Invalid sheet name')
    require(s['sheets'][0]['role']=='output','Output first');cells={};tn=set();formulas=0
    for sh in s['sheets']:
        bounds(sh['display_range']);occupied={};cells[sh['name']]=occupied
        for b in sh['blocks']:
            r,c,R,C=bounds(b['range']);require(('values' in b)^('formulas' in b),'Need values OR formulas')
            kind='formulas' if 'formulas' in b else 'values';a=b[kind]
            require(len(a)==R-r+1 and all(len(row)==C-c+1 for row in a),'Block shape '+b['range'])
            for i,row in enumerate(a):
                for j,v in enumerate(row):
                    if v is None:continue
                    addr=col(c+j)+str(r+i);require(addr not in occupied,'Overlapping '+sh['name']+'!'+addr)
                    if kind=='formulas':
                        require(isinstance(v,str) and v.startswith('='),'Invalid formula')
                        require(not re.search(r'\[|https?:|WEBSERVICE|HYPERLINK|DDE|RTD|INDIRECT|OFFSET|NOW\(|TODAY\(',v,re.I),'Unsupported external/volatile formula')
                        formulas+=1
                    elif isinstance(v,dict):require(set(v)=={'date'} and re.fullmatch(r'\d{4}-\d\d-\d\d',v['date']),'Invalid date')
                    else:require(isinstance(v,(str,int,float,bool)) and (not isinstance(v,(float,int)) or math.isfinite(v)),'Invalid value')
                    occupied[addr]=(kind,v)
        for t in sh.get('tables',[]):
            require(t['name'] not in tn and re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',t['name']),'Invalid table name');tn.add(t['name'])
            r,c,R,C=bounds(t['range']);require(R>r,'Table has no records')
            require(all(col(j)+str(r) in occupied for j in range(c,C+1)),'Missing table header')
        for ch in sh.get('charts',[]):
            require(ch['type'] in ['bar','line'],'Unsupported chart');bounds(ch['range']);r,c=cell(ch['from']);R,C=cell(ch['to'])
            require(all(not(r<=cell(a)[0]<R and c<=cell(a)[1]<C) for a in occupied),'Chart covers cells')
    require(formulas and tn,'Need formulas and tables')
    for cat,reqkey in [('controls','required_controls'),('metrics','required_metrics')]:
        items=s[cat];ids=[x['id'] for x in items];require(len(ids)==len(set(ids)),'Duplicate '+cat)
        require(set(req.get(reqkey,[]))<=set(ids),'Missing required '+cat)
        for x in items:
            require(x['sheet'] in cells and x['cell'] in cells[x['sheet']],'Unknown '+cat+' cell')
            require(cells[x['sheet']][x['cell']][0]==('formulas' if cat=='metrics' else 'values'),'Wrong '+cat+' cell type')
            if cat=='controls':require(('values' in x and 1<=len(x['values'])<=30) or ('min' in x and 'max' in x and x['min']<=x['max']),'Invalid control')
    require(len(s['tests'])>=3,'Need 3 mutation tests');edited=set()
    for t in s['tests']:
        require(t.get('name') and t.get('edits') and t.get('expect'),'Incomplete mutation')
        for e in t['edits']:
            require(e['sheet'] in cells,'Unknown edit sheet');cell(e['cell'])
            require(cells[e['sheet']].get(e['cell'],('values',None))[0]=='values','Test replaces formula');edited.add((e['sheet'],e['cell']))
        for e in t['expect']:require(e['metric'] in {x['id'] for x in s['metrics']},'Unknown metric')
    require(all((x['sheet'],x['cell']) in edited for x in s['controls']),'Untested control')
    require(s.get('lineage'),'Missing lineage')
    require({x['path'] for x in inputs}<={x['source'] for x in s['lineage']},'Untraced input')
    for x in s['lineage']:require(x['source'] in {y['path'] for y in inputs} and x.get('locator') and x.get('target') and x.get('note'),'Bad lineage')
    require(isinstance(s.get('issues'),list),'Missing issues')
    require(all(s['update_policy'].get(k) for k in ['instructions','capacity','filter_scope']),'Missing update policy')
    return req,s,cells
def read_xlsx(path):
    with zipfile.ZipFile(path) as z:
        require(not any('vba' in n.lower() or 'externallinks/' in n.lower() for n in z.namelist()),'Active/external content')
        shared=[]
        if 'xl/sharedStrings.xml' in z.namelist():shared=[''.join(x.itertext()) for x in ET.fromstring(z.read('xl/sharedStrings.xml'))]
        wb=ET.fromstring(z.read('xl/workbook.xml'));result={}
        stats={k:len([n for n in z.namelist() if re.fullmatch(p,n)]) for k,p in [('tables',r'xl/tables/table[0-9]+.xml'),('charts',r'xl/(?:drawings/)?charts/chart[0-9]+.xml')]}
        chart_parts=[n for n in z.namelist() if re.fullmatch(r'xl/(?:drawings/)?charts/chart[0-9]+.xml',n)]
        chart_ns={'c':'http://schemas.openxmlformats.org/drawingml/2006/chart'}
        stats.update(validations=0,conditional_formats=0,charts_with_cell_references=sum(bool(ET.fromstring(z.read(n)).findall('.//c:f',chart_ns)) for n in chart_parts))
        rels={r.attrib['Id']:r.attrib['Target'] for r in ET.fromstring(z.read('xl/_rels/workbook.xml.rels'))}
        for sh in wb.find('s:sheets',NS):
            rid=sh.attrib['{http://schemas.openxmlformats.org/officeDocument/2006/relationships}id'];target=rels[rid]
            part=target.lstrip('/') if target.startswith('/') else 'xl/'+target
            doc=ET.fromstring(z.read(part));vals={}
            stats['validations']+=len(doc.findall('.//s:dataValidation',NS));stats['conditional_formats']+=len(doc.findall('.//s:conditionalFormatting',NS))
            for c in doc.findall('.//s:sheetData/s:row/s:c',NS):
                f=c.find('s:f',NS);v=c.find('s:v',NS);value=v.text if v is not None else None;t=c.get('t')
                if t=='s' and value is not None:value=shared[int(value)]
                elif t=='inlineStr':value=''.join(c.find('s:is',NS).itertext())
                elif t=='b':value=value=='1'
                elif value is not None and t in (None,'n'):
                    try:value=float(value)
                    except ValueError:pass
                vals[c.get('r')]={'formula':f.text if f is not None else None,'value':value,'type':t}
            result[sh.get('name')]=vals
        return result,stats
def validate_output(root):
    root=Path(root);req,s,cells=validate_spec(root)
    require((root/'output/guide.md').stat().st_size>100,'Missing guide');qa=json.loads((root/'output/qa.json').read_text())
    require(qa['spec_sha256']==digest(root/'output/spec.json'),'Stale render spec')
    require(qa['workbook_sha256']==digest(root/'output/workbook.xlsx'),'Changed workbook bytes')
    require(len(qa['tests'])==len(s['tests']) and all(x['passed'] for x in qa['tests']),'Mutation failed')
    require(len(qa['previews'])==len(s['sheets']),'Missing preview coverage')
    for x in qa['previews']:require(digest(root/x['path'])==x['sha256'],'Stale preview')
    actual,stats=read_xlsx(root/'output/workbook.xlsx');require(list(actual)==[x['name'] for x in s['sheets']],'Sheet structure changed')
    for sh,expected in cells.items():
        for a,(kind,v) in expected.items():
            obs=actual[sh].get(a,{})
            if kind=='formulas':require(obs.get('formula')==v[1:],'Changed formula '+sh+'!'+a)
            elif isinstance(v,dict):
                d=date.fromisoformat(v['date']);epoch=date(1899,12,30) if d>=date(1900,3,1) else date(1899,12,31)
                require(obs.get('value')==(d-epoch).days,'Changed typed date '+sh+'!'+a)
            else:require(obs.get('value')==v or (isinstance(v,str) and obs.get('value')=="'"+v),'Changed value '+sh+'!'+a)
        for a,x in actual[sh].items():require(x['type']!='e' and not(isinstance(x['value'],str) and x['value'] in ERR),'Formula error '+sh+'!'+a)
    require(stats['tables']==sum(len(x.get('tables',[])) for x in s['sheets']),'Lost native table')
    require(stats['charts']==sum(len(x.get('charts',[])) for x in s['sheets']),'Lost chart')
    require(stats['charts_with_cell_references']==stats['charts'],'Chart missing cell binding')
    require(stats['validations']>=len(s['controls']),'Lost control validation')
    for metric in s['metrics']:
        require(actual[metric['sheet']][metric['cell']]['value']==qa['baseline'][metric['id']],'Cached metric disagrees with QA '+metric['id'])
    return {'status':'accepted-mechanically','native':stats,'metrics':qa['baseline'],'limits':'Requires independent semantic, visual and target Excel review'}
if __name__=='__main__':
    try:
        if '--spec' in sys.argv:validate_spec(Path.cwd());print('Valid bound spec; artifacts not yet checked')
        else:print(json.dumps(validate_output(Path.cwd())))
    except Exception as e:print(str(e),file=sys.stderr);sys.exit(1)
