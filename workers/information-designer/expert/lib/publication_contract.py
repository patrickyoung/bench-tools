#!/usr/bin/env python3
"""Shared reviewed contract copied into each independently exported definition."""
import hashlib,json,math,re,zipfile,xml.etree.ElementTree as ET,os,subprocess
from pathlib import Path

SCHEMA='bench.publication-spec/v1'
ROLES={'editorial-director','information-designer','executive-writer','presentation-designer','publication-reviewer'}
RUBRIC=['evidence_fidelity','narrative_coherence','audience_usefulness','writing_quality','visual_craft','accessibility']
def sha(p):return hashlib.sha256(Path(p).read_bytes()).hexdigest()
def pairs(v):
 d={}
 for k,x in v:
  if k in d:raise ValueError('Duplicate JSON key: '+k)
  d[k]=x
 return d
def read(p,limit=2_000_000):
 p=Path(p)
 if p.is_symlink() or not p.is_file() or p.stat().st_size>limit:raise ValueError('Invalid bounded regular input: '+str(p))
 return json.loads(p.read_text(),object_pairs_hook=pairs,parse_constant=lambda v:(_ for _ in ()).throw(ValueError('Nonfinite JSON')))
def keys(d,required):
 if not isinstance(d,dict) or set(d)!=set(required.split()):raise ValueError('Expected exact fields: '+required)
def string(v,lo=1,hi=12000):
 if not isinstance(v,str) or not lo<=len(v)<=hi:raise ValueError('Invalid text length')
def array(v,lo=0,hi=100):
 if not isinstance(v,list) or not lo<=len(v)<=hi:raise ValueError('Invalid list length')
def strings(v,lo=0,hi=100):
 array(v,lo,hi)
 for x in v:string(x)
def inputs(root):
 out=[]
 for p in sorted((root/'inputs').rglob('*')):
  if p.is_symlink():raise ValueError('Symlink input')
  if p.is_file():
   if p.stat().st_size>20_000_000:raise ValueError('Oversized input')
   out.append({'path':p.relative_to(root).as_posix(),'sha256':sha(p)})
 if len(out)>120:raise ValueError('Too many input files')
 return out
def bindings(root,role):
 return {'schema':SCHEMA,'role':role,'request_sha256':sha(root/'request.json'),'inputs':inputs(root)}
def refs(block,facts,messages=None):
 strings(block['fact_ids'],1,100)
 if not set(block['fact_ids'])<=facts:raise ValueError('Unknown fact reference')
 if messages is not None:
  strings(block['message_ids'],1,20)
  if not set(block['message_ids'])<=messages:raise ValueError('Unknown message reference')
def source(root):return read(root/'inputs/source.json')
def safe_file(root,relative):
 p=Path(relative)
 if p.is_absolute() or not p.parts or '..' in p.parts:raise ValueError('Unsafe relative artifact path')
 dest=Path(root)/p
 if any(x.is_symlink() for x in [dest,*dest.parents] if x!=Path(root).parent):raise ValueError('Symlink artifact path')
 if not dest.is_file():raise ValueError('Missing artifact '+relative)
 return dest
def xlsx_cells(z,sheet):
 ns={'s':'http://schemas.openxmlformats.org/spreadsheetml/2006/main'};shared=[]
 if 'xl/sharedStrings.xml' in z.namelist():shared=[''.join(x.itertext()) for x in ET.fromstring(z.read('xl/sharedStrings.xml'))]
 d={}
 for c in ET.fromstring(z.read(f'xl/worksheets/sheet{sheet}.xml')).findall('.//s:c',ns):
  value=c.find('s:v',ns);v=value.text if value is not None else None
  if c.get('t')=='s' and v is not None:v=shared[int(v)]
  elif c.get('t')=='inlineStr':v=''.join(c.find('s:is',ns).itertext())
  elif v is not None and c.get('t') not in ['str','e','b']:v=float(v)
  d[c.get('r')]=v
 return d
def validate(root,role,require_artifacts=True):
 root=Path(root).resolve();request=read(root/'request.json');s=read(root/'output/spec.json');keys(s,'schema role request_sha256 inputs content')
 if role not in ROLES or request['role']!=role:raise ValueError('Wrong role')
 for k,v in bindings(root,role).items():
  if s[k]!=v:raise ValueError('Stale or wrong '+k)
 c=s['content'];src=source(root);facts={x['id'] for x in src['facts']}
 story=read(root/'inputs/story.json')['content'] if role!='editorial-director' else None
 messages={x['id'] for x in story['messages']} if story else set()
 required=set(story['required_message_ids']) if story else set()
 seen=set()
 if role=='editorial-director':
  keys(c,'title subtitle audience thesis messages required_message_ids terminology design report_arc deck_arc')
  for k in ['title','subtitle','audience','thesis']:string(c[k],1,1200)
  array(c['messages'],3,7);ids=[]
  for m in c['messages']:
   keys(m,'id text qualification fact_ids');string(m['id'],1,40);string(m['text'],1,700);string(m['qualification'],1,1000);refs(m,facts);ids.append(m['id'])
  if len(set(ids))!=len(ids):raise ValueError('Duplicate message IDs')
  strings(c['required_message_ids'],2,7)
  if not set(c['required_message_ids'])<=set(ids):raise ValueError('Unknown required message')
  strings(c['terminology'],1,25);strings(c['report_arc'],5,9);strings(c['deck_arc'],7,12)
  keys(c['design'],'direction font ink accent secondary paper')
  string(c['design']['direction']);string(c['design']['font'])
  if c['design']['font'] not in ['Arial','Helvetica','Liberation Sans']:raise ValueError('Unverified font selection')
  for k in ['ink','accent','secondary','paper']:
   if not re.fullmatch('#[0-9A-Fa-f]{6}',c['design'][k]):raise ValueError('Invalid color')
 elif role=='information-designer':
  keys(c,'title workbook_intro visuals');string(c['title']);string(c['workbook_intro'],1,1600);array(c['visuals'],3,6);kinds=[];ids=[]
  for v in c['visuals']:
   keys(v,'id kind title subtitle caption alt fact_ids message_ids steps');string(v['id'],1,40)
   if not re.fullmatch('[a-z][a-z0-9-]*',v['id']):raise ValueError('Invalid asset ID')
   if v['kind'] not in ['score_bounds','score_heatmap','coverage','effects','decision_path','sensitivity']:raise ValueError('Unknown visual encoding')
   for k in ['title','subtitle','caption','alt']:string(v[k],1,900)
   refs(v,facts,messages);seen.update(v['message_ids']);kinds.append(v['kind']);ids.append(v['id'])
   array(v['steps'],3 if v['kind']=='decision_path' else 0,4 if v['kind']=='decision_path' else 0)
   for step in v['steps']:
    keys(step,'heading detail fact_ids message_ids');string(step['heading'],1,55);string(step['detail'],1,180);refs(step,facts,messages)
  if len(set(ids))!=len(ids) or not {'score_bounds','decision_path'}<=set(kinds):raise ValueError('Need unique graphics, score chart and explanatory infographic')
  if 'effects' in kinds and not any(x['status']=='inferential' for x in src['statistics']['comparisons']):raise ValueError('No inferential data for effects plot')
 elif role=='executive-writer':
  keys(c,'title subtitle executive_summary sections');string(c['title']);string(c['subtitle']);string(c['executive_summary'],250,2200);array(c['sections'],5,9)
  text=[c['executive_summary']]
  design=read(root/'inputs/design.json')['content'];visuals={v['id'] for v in design['visuals']}
  for p in c['sections']:
   keys(p,'heading paragraphs fact_ids message_ids visual_id');string(p['heading']);strings(p['paragraphs'],2,6);refs(p,facts,messages);seen.update(p['message_ids']);text+=p['paragraphs']
   if p['visual_id'] is not None and p['visual_id'] not in visuals:raise ValueError('Unknown visual')
  n=len(' '.join(text).split())
  if not 1100<=n<=3000:raise ValueError('Long-form prose requires 1100-3000 words, found '+str(n))
 elif role=='presentation-designer':
  keys(c,'title slides');string(c['title']);array(c['slides'],7,12);kinds=[]
  for p in c['slides']:
   keys(p,'kind title lead body fact_ids message_ids notes')
   if p['kind'] not in ['cover','message','score_chart','comparison_table','tradeoff','decision']:raise ValueError('Unknown slide layout')
   string(p['title'],1,90);string(p['lead'],1,210);strings(p['body'],0,4);string(p['notes'],1,1800);refs(p,facts,messages);seen.update(p['message_ids']);kinds.append(p['kind'])
   if len(' '.join(p['body']).split())>75:raise ValueError('Slide copy too dense')
  if kinds[0]!='cover' or not {'score_chart','comparison_table','decision'}<=set(kinds):raise ValueError('Need cover, editable chart/table and decision')
 elif role=='publication-reviewer':
  keys(c,'verdict summary rubric findings cross_format_checks');string(c['summary']);array(c['findings'],0,40);strings(c['cross_format_checks'],5,20);keys(c['rubric'],' '.join(RUBRIC))
  if c['verdict'] not in ['publish','revise']:raise ValueError('Invalid verdict')
  for v in c['rubric'].values():
   if type(v)!=int or not 1<=v<=5:raise ValueError('Invalid rubric score')
  for f in c['findings']:
   keys(f,'severity artifact location issue correction')
   if f['severity'] not in ['material','minor']:raise ValueError('Invalid severity')
   for k in ['artifact','location','issue','correction']:string(f[k])
  visual=read(root/'inputs/visual-review.json')
  if c['verdict']=='publish' and (min(c['rubric'].values())<4 or any(f['severity']=='material' for f in c['findings']) or visual['verdict']!='pass'):raise ValueError('Publication quality gate failed')
 if role in ['executive-writer','presentation-designer'] and not required<=seen:raise ValueError('Required narrative message omitted')
 if require_artifacts and role in ['information-designer','executive-writer','presentation-designer']:
  receipt=read(root/'output/artifacts.json')
  if receipt['spec_sha256']!=sha(root/'output/spec.json'):raise ValueError('Stale render receipt')
  if not receipt['files'] or not receipt['previews']:raise ValueError('Missing artifacts or previews')
  for f in receipt['files']+receipt['previews']:
   p=Path(f['path'])
   if p.is_absolute() or '..' in p.parts or p.parts[0] not in ['output','previews']:raise ValueError('Unsafe output path')
   dest=safe_file(root,f['path'])
   if dest.is_symlink() or sha(dest)!=f['sha256']:raise ValueError('Changed render artifact')
  names={f['path'] for f in receipt['files']}
  required_files={'information-designer':{'output/comparison.xlsx'},'executive-writer':{'output/report.docx','output/report.pdf','output/report.md'},'presentation-designer':{'output/presentation.pptx','output/presentation.pdf'}}[role]
  if not required_files<=names:raise ValueError('Missing required publication format')
  if role=='information-designer':
   for v in c['visuals']:
    if not {'output/graphics/'+v['id']+'.png','output/graphics/'+v['id']+'.svg'}<=names:raise ValueError('Missing selected graphic')
   regions=read(root/'workbook-check.json')['regions'];expected={'previews/workbook/'+str(i+1)+'-'+r[0].lower()+'.png' for i,r in enumerate(regions)}
   actual={f['path'] for f in receipt['previews'] if f['path'].startswith('previews/workbook/')}
   if actual!=expected:raise ValueError('Incomplete workbook image coverage')
   wc=read(root/'workbook-check.json')
   for sheet,last in [('Matrix',len(src['matrix']['cells'])+4),('Evidence',wc['evidence_rows']+4),('Scoring basis',wc['anchor_rows']+4),('Weight scenarios',wc['scenario_rows']+4)]:
    covered=set()
    for sh,region in regions:
     if sh==sheet:
      lo,hi=map(int,re.findall('[0-9]+',region));covered.update(range(lo,hi+1))
    if covered!=set(range(1,last+1)):raise ValueError('Workbook rows omitted from inspection')
  for f in receipt['files']:
   p=root/f['path']
   if p.suffix=='.pdf' and not p.read_bytes().startswith(b'%PDF'):raise ValueError('Invalid PDF')
   if p.suffix=='.pdf':
    code='from pypdf import PdfReader;import sys;print(len(PdfReader(sys.argv[1]).pages))'
    pages=int(subprocess.check_output([os.environ['PUBLICATION_PYTHON'],'-c',code,str(p)],text=True))
    medium='document' if role=='executive-writer' else 'slides'
    if len([f for f in receipt['previews'] if f['path'].startswith('previews/'+medium+'/')])!=pages:raise ValueError('Every PDF page must have a reviewed preview')
   if p.suffix in ['.docx','.pptx','.xlsx']:
    with zipfile.ZipFile(p) as z:
     if sum(i.file_size for i in z.infolist())>100_000_000:raise ValueError('Oversized Office package')
     if '[Content_Types].xml' not in z.namelist():raise ValueError('Invalid Office package')
     if p.suffix=='.xlsx':
      overview=xlsx_cells(z,1);detail=xlsx_cells(z,2);criteria={c['id']:c for c in src['matrix']['criteria']}
      ac=xlsx_cells(z,4);sc=xlsx_cells(z,5)
      for criterion in criteria.values():
       for score,meaning in [('Reason',criterion['reason']),*criterion['anchors'].items()]:
        value=score if score=='Reason' else int(score);collected=''.join(str(ac.get('D'+str(i),'')) for i in range(5,wc['anchor_rows']+5) if ac.get('A'+str(i))==criterion['name'] and ac.get('C'+str(i))==value)
        if collected!=meaning:raise ValueError('Saved score anchors differ from source')
      si=5
      for scenario in src['matrix']['sensitivity']:
       for total in src['matrix']['totals']:
        if sc.get('D'+str(si))!=scenario['lower_bounds'][total['candidate_id']] or sc.get('B'+str(si))!=scenario['factor'] or sc.get('C'+str(si))!=total['name']:raise ValueError('Saved sensitivity differs from source')
        si+=1
      for i,row in enumerate(src['matrix']['totals'],8):
       for col,key in [('B','lower_bound'),('C','upper_bound'),('D','coverage_percent'),('E','known_only_fit')]:
        expected=row[key];actual=overview.get(col+str(i))
        if expected is None and actual not in [None,'']:raise ValueError('Missing summary became numeric')
        if expected is not None and (not isinstance(actual,(int,float)) or abs(actual-expected)>1e-7):raise ValueError('Saved workbook summary mismatch')
      for i,row in enumerate(src['matrix']['cells'],5):
       if detail.get('D'+str(i))!=row['score']:raise ValueError('Saved workbook score mismatch')
       if detail.get('C'+str(i))!=criteria[row['criterion_id']]['weight']:raise ValueError('Saved workbook weight mismatch')
       expected=None if row['score'] is None else row['score']*criteria[row['criterion_id']]['weight']/5;actual=detail.get('E'+str(i))
       if expected is None and actual not in [None,'']:raise ValueError('Missing point became numeric')
       if expected is not None and (not isinstance(actual,(int,float)) or abs(actual-expected)>1e-7):raise ValueError('Saved formula cache mismatch')
     if role=='executive-writer':
      tree=ET.fromstring(z.read('word/document.xml'));text=' '.join(tree.itertext())
      if c['title'] not in text or any(x['heading'] not in text for x in c['sections']):raise ValueError('Document content mismatch')
     if role=='presentation-designer':
      slides=[n for n in z.namelist() if re.fullmatch('ppt/slides/slide[0-9]+.xml',n)]
      expected=sum(math.ceil(len(src['matrix']['totals'])/4) if s['kind']=='comparison_table' else math.ceil(len(src['matrix']['totals'])/6) if s['kind']=='score_chart' else 1 for s in c['slides'])
      if len(slides)!=expected:raise ValueError('Slide count mismatch')
      if not any(re.fullmatch('ppt/(slides/)?charts/chart[0-9]+.xml',n) for n in z.namelist()):raise ValueError('Missing editable chart')
      ns={'c':'http://schemas.openxmlformats.org/drawingml/2006/chart'}
      chart_names=set();by_name={r['name']:r for r in src['matrix']['totals']}
      for n in z.namelist():
       if re.fullmatch('ppt/(slides/)?charts/chart[0-9]+.xml',n):
        chart=ET.fromstring(z.read(n));series=chart.findall('.//c:ser',ns)
        categories=[v.text for v in series[0].findall('./c:cat//c:pt/c:v',ns)];chart_names.update(categories)
        if not set(categories)<=set(by_name):raise ValueError('Unknown chart category')
        expected=[[by_name[name]['lower_bound'] for name in categories],[by_name[name]['upper_bound']-by_name[name]['lower_bound'] for name in categories]]
        actual=[[float(v.text) for v in s.findall('./c:val//c:pt/c:v',ns)] for s in series]
        if actual!=expected:raise ValueError('Native chart differs from checked numbers')
        axes=chart.findall('.//c:valAx',ns)
        if not axes or any(a.find('c:scaling/c:min',ns) is None or float(a.find('c:scaling/c:min',ns).get('val'))!=0 or a.find('c:scaling/c:max',ns) is None or float(a.find('c:scaling/c:max',ns).get('val'))!=100 for a in axes):raise ValueError('Score bar axis must explicitly span 0-100')
      if chart_names!=set(by_name):raise ValueError('Chart pagination omitted a candidate')
      texts=[ET.fromstring(z.read(n)) for n in slides];native_tables=sum(len(x.findall('.//{http://schemas.openxmlformats.org/drawingml/2006/main}tbl')) for x in texts)
      if native_tables<1:raise ValueError('Missing editable table')
      alltext=' '.join(' '.join(x.itertext()) for x in texts)
      if any(x['title'] not in alltext for x in c['slides']):raise ValueError('Slide content mismatch')
 return s
