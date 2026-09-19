"""Trusted read-only contract. Never executes worker-authored code."""
import hashlib, json, math, re, sys, zipfile
from datetime import date,datetime
from pathlib import Path
from xml.etree import ElementTree as ET
NS={'s':'http://schemas.openxmlformats.org/spreadsheetml/2006/main'}
ERR={'#REF!','#DIV/0!','#VALUE!','#NAME?','#N/A','#NUM!','#NULL!','#SPILL!','#CALC!'}
def digest(p): return hashlib.sha256(p.read_bytes()).hexdigest()
def require(ok,msg):
    if not ok: raise ValueError(msg)
def require_matrix_shape(matrix,rows,columns,location,message='Bad matrix shape'):
    # Keep the existing shape predicate; only explain its failures more precisely.
    try:valid=len(matrix)==rows and all(len(row)==columns for row in matrix)
    except TypeError:valid=False
    if valid:return
    try:
        widths=[]
        for row in matrix:
            try:widths.append(len(row))
            except TypeError:widths.append(type(row).__name__+' (no column count)')
        actual=str(len(matrix))+' rows with column counts '+repr(widths)
    except TypeError:actual=type(matrix).__name__+' (no matrix dimensions)'
    raise ValueError(message+' at '+location+': expected '+str(rows)+' rows x '+str(columns)+' columns; actual '+actual)
def require_metric_reference(metric,ids,location,message='Unknown metric'):
    if metric not in ids:
        raise ValueError(message+' at '+location+': unknown metric ID '+repr(metric)+'; known IDs: '+repr(sorted(ids,key=repr)))
def cell(s):
    m=re.fullmatch(r'([A-Z]{1,3})([1-9][0-9]{0,4})',s) if isinstance(s,str) else None;require(m is not None,'Invalid cell '+repr(s)+': expected an A1 cell address');c=0
    for ch in m[1]:c=c*26+ord(ch)-64
    return int(m[2]),c
def bounds(s):
    require(isinstance(s,str),'Invalid range '+repr(s)+': expected an A1 cell or rectangle')
    a=s.split(':');require(len(a)<=2,'Invalid range '+repr(s)+': expected an A1 cell or rectangle')
    try:r,c=cell(a[0]);R,C=cell(a[-1])
    except ValueError as e:raise ValueError('Invalid range '+repr(s)+': '+str(e)) from e
    require(0<r<=R<=10000 and 0<c<=C<=100,'Range exceeds bounds '+repr(s)+': rows 1-10000, columns A-CV, ordered top-left to bottom-right');return r,c,R,C
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
    for sheet_index,sh in enumerate(s['sheets']):
        bounds(sh['display_range']);occupied={};cells[sh['name']]=occupied
        for block_index,b in enumerate(sh['blocks']):
            r,c,R,C=bounds(b['range']);require(('values' in b)^('formulas' in b),'Need values OR formulas')
            kind='formulas' if 'formulas' in b else 'values';a=b[kind]
            require_matrix_shape(a,R-r+1,C-c+1,'sheets['+str(sheet_index)+'].blocks['+str(block_index)+'] ('+kind+') sheet='+repr(sh['name'])+' range='+repr(b['range']),'Block shape')
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
            require(ch['type'] in ['bar','line','pie','doughnut','area','scatter'],'Unsupported chart');bounds(ch['range']);r,c=cell(ch['from']);R,C=cell(ch['to'])
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
    for test_index,t in enumerate(s['tests']):
        require(t.get('name') and t.get('edits') and t.get('expect'),'Incomplete mutation')
        for e in t['edits']:
            require(e['sheet'] in cells,'Unknown edit sheet');cell(e['cell'])
            require(cells[e['sheet']].get(e['cell'],('values',None))[0]=='values','Test replaces formula');edited.add((e['sheet'],e['cell']))
        for expect_index,e in enumerate(t['expect']):require_metric_reference(e['metric'],{x['id'] for x in s['metrics']},'tests['+str(test_index)+'] ('+repr(t['name'])+').expect['+str(expect_index)+'].metric')
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
                if t=='str' and value is None:value=''
                if t=='s' and value is not None:value=shared[int(value)]
                elif t=='inlineStr':value=''.join(c.find('s:is',NS).itertext())
                elif t=='b':value=value=='1'
                elif value is not None and t in (None,'n'):
                    try:value=float(value)
                    except ValueError:pass
                vals[c.get('r')]={'formula':f.text if f is not None else None,'value':value,'type':t}
            result[sh.get('name')]=vals
        return result,stats
def canonical_date(value):
    if not isinstance(value,str) or not re.fullmatch(r'[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{3}Z',value):return None
    try:return datetime.strptime(value,'%Y-%m-%dT%H:%M:%S.%fZ')
    except ValueError:return None
def excel_date_serial(value,date1904=False):
    parsed=canonical_date(value)
    if parsed is None:return None
    if date1904:return (parsed-datetime(1904,1,1)).total_seconds()/86400
    serial=(parsed-datetime(1899,12,31)).total_seconds()/86400
    return serial+(1 if parsed>=datetime(1900,3,1) else 0)
def workbook_date1904(path):
    with zipfile.ZipFile(path) as z:wb=ET.fromstring(z.read('xl/workbook.xml'))
    props=wb.find('s:workbookPr',NS);flag=props.get('date1904') if props is not None else None
    require(flag in (None,'0','1','false','true'),'Invalid workbook date system')
    return flag in ('1','true')
def metric_equal(actual,expected,tolerance=1e-6,is_date=False):
    if is_date:
        if isinstance(expected,dict) and set(expected)=={'date'} and isinstance(expected['date'],str) and re.fullmatch(r'[0-9]{4}-[0-9]{2}-[0-9]{2}',expected['date']):expected=expected['date']+'T00:00:00.000Z'
        return canonical_date(actual) is not None and canonical_date(expected) is not None and actual==expected
    if type(actual) in (int,float) and type(expected) in (int,float):return abs(actual-expected)<=tolerance
    return type(actual)==type(expected) and type(actual) in (str,bool,type(None)) and actual==expected
def validated_date_metrics(path,qa,metrics,actual):
    """Validate every runtime Date claim against saved numeric date-cell facts."""
    marked=qa.get('baseline_date_metrics',[]);known={m['id']:m for m in metrics}
    require(isinstance(marked,list) and all(isinstance(x,str) for x in marked),'Invalid baseline date metrics')
    require(len(marked)==len(set(marked)) and set(marked)<=set(known),'Duplicate/unknown baseline date metric')
    if not marked:return set()
    date1904=workbook_date1904(path)
    with zipfile.ZipFile(path) as z:
        wb=ET.fromstring(z.read('xl/workbook.xml'))
        st=ET.fromstring(z.read('xl/styles.xml'));formats={int(x.get('numFmtId')):x.get('formatCode','') for x in st.findall('s:numFmts/s:numFmt',NS)};styles=st.findall('s:cellXfs/s:xf',NS)
        def date_style(index):
            if not 0<=index<len(styles):return False
            ident=int(styles[index].get('numFmtId','0'))
            if ident in {14,15,16,17,22}:return True
            fmt=re.sub(r'"[^"]*"|\\.|_.|\*.','',formats.get(ident,'')).lower()
            if ';' in fmt or re.search(r'\[[<>=]',fmt):return False
            fmt=re.sub(r'\[[^\]]*\]','',fmt)
            return bool(re.search('[yd]',fmt) or 'm' in fmt and not re.search('[hs]',fmt))
        rels={x.get('Id'):x.get('Target') for x in ET.fromstring(z.read('xl/_rels/workbook.xml.rels'))};cells={}
        for sh in wb.find('s:sheets',NS):
            target=rels[sh.get('{http://schemas.openxmlformats.org/officeDocument/2006/relationships}id')];part=target.lstrip('/') if target.startswith('/') else 'xl/'+target
            cells[sh.get('name')]={c.get('r'):c for c in ET.fromstring(z.read(part)).findall('.//s:sheetData/s:row/s:c',NS)}
        for ident in marked:
            m=known[ident];saved=actual.get(m['sheet'],{}).get(m['cell'],{});node=cells.get(m['sheet'],{}).get(m['cell']);serial=excel_date_serial(qa['baseline'].get(ident),date1904)
            require(node is not None and saved.get('type') in (None,'n') and type(saved.get('value')) in (int,float),'Date metric is not a saved numeric date cell '+ident)
            require(date_style(int(node.get('s','0'))),'Unsupported calendar date format for metric '+ident+': use a single calendar date format such as yyyy-mm-dd; conditional, multi-section, arbitrary or time-only formats are not recognized')
            require(serial is not None and abs(saved['value']-serial)<=1e-9,'Saved date metric disagrees with QA '+ident)
    return set(marked)
def validate_output(root):
    root=Path(root);req,s,cells=validate_spec(root)
    from features import verify_pivots
    verify_pivots(root,s)
    require((root/'output/guide.md').stat().st_size>100,'Missing guide');qa=json.loads((root/'output/qa.json').read_text())
    require(qa['spec_sha256']==digest(root/'output/spec.json'),'Stale render spec')
    require(qa['workbook_sha256']==digest(root/'output/workbook.xlsx'),'Changed workbook bytes')
    require(len(qa['tests'])==len(s['tests']) and all(x['passed'] for x in qa['tests']),'Mutation failed')
    require(len(qa['previews'])==len(s['sheets']),'Missing preview coverage')
    for x in qa['previews']:require(digest(root/x['path'])==x['sha256'],'Stale preview')
    actual,stats=read_xlsx(root/'output/workbook.xlsx');require(list(actual)==[x['name'] for x in s['sheets']],'Sheet structure changed')
    date1904=workbook_date1904(root/'output/workbook.xlsx') if any(isinstance(v,dict) for expected in cells.values() for kind,v in expected.values()) else False
    for sh,expected in cells.items():
        for a,(kind,v) in expected.items():
            obs=actual[sh].get(a,{})
            if kind=='formulas':require(obs.get('formula')==v[1:],'Changed formula '+sh+'!'+a)
            elif isinstance(v,dict):
                serial=excel_date_serial(v['date']+'T00:00:00.000Z',date1904)
                require(serial is not None and type(obs.get('value')) in (int,float) and obs.get('value')==serial,'Changed typed date '+sh+'!'+a)
            else:require(obs.get('value')==v or (isinstance(v,str) and obs.get('value')=="'"+v),'Changed value '+sh+'!'+a)
        for a,x in actual[sh].items():require(x['type']!='e' and not(isinstance(x['value'],str) and x['value'] in ERR),'Formula error '+sh+'!'+a)
    require(stats['tables']==sum(len(x.get('tables',[])) for x in s['sheets']),'Lost native table')
    require(stats['charts']==sum(len(x.get('charts',[])) for x in s['sheets']),'Lost chart')
    require(stats['charts_with_cell_references']==stats['charts'],'Chart missing cell binding')
    require(stats['validations']>=len(s['controls']),'Lost control validation')
    date_metrics=validated_date_metrics(root/'output/workbook.xlsx',qa,s['metrics'],actual)
    for metric in s['metrics']:
        require(metric['id'] in date_metrics or metric_equal(actual[metric['sheet']][metric['cell']]['value'],qa['baseline'][metric['id']],0),'Cached metric disagrees with QA '+metric['id'])
    return {'status':'accepted-mechanically','native':stats,'metrics':qa['baseline'],'limits':'Requires independent semantic, visual and target Excel review'}
if __name__=='__main__':
    try:
        if '--spec' in sys.argv:validate_spec(Path.cwd());print('Valid bound spec; artifacts not yet checked')
        else:print(json.dumps(validate_output(Path.cwd())))
    except Exception as e:print(str(e),file=sys.stderr);sys.exit(1)
