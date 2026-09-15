#!/usr/bin/env python3
"""Bounded Unix team handoffs for a reviewed decision's publication package."""
import argparse,hashlib,json,os,shutil,subprocess,sys,csv
from pathlib import Path
from publication_contract import read,sha,inputs,validate,safe_file
ROLES=['editorial-director','information-designer','executive-writer','presentation-designer','publication-reviewer']
def write(p,v):p.parent.mkdir(parents=True,exist_ok=True);p.write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
def prepare(analysis,run,profile=None):
 analysis=Path(analysis).resolve();run=Path(run).resolve()
 if run.exists():raise ValueError('New publication directory required')
 status=read(analysis/'status.json')
 if status.get('status')!='reviewed' or status.get('exit_code')!=0:raise ValueError('Only a completed, reviewed analysis can be published')
 manifest=read(analysis/'result/manifest.json')
 for f in manifest['artifacts']:
  if sha(safe_file(analysis/'result',f['path']))!=f['sha256']:raise ValueError('Changed reviewed result')
 packet=read(analysis/'control/packet.json');matrix=read(analysis/'result/matrix.json');detail=read(analysis/'result/analysis.json');pm=read(analysis/'result/decision-brief.json');stats=read(analysis/'result/statistics.json');plan=read(analysis/'result/statistical-plan.json')
 facts=[{'id':'business-case','label':'Business need','text':packet['job']['business_case']+'\n'+packet['job'].get('notes',''),'source':'Business brief and human notes'}, {'id':'decision','label':'Evaluation Lead recommendation','text':json.dumps(pm['details'],ensure_ascii=False),'source':'Reviewed Product Manager decision brief'}, {'id':'weights','label':'Criteria and weights','text':json.dumps(matrix['criteria'],ensure_ascii=False),'source':'Reviewed comparison, criteria and anchored weights'}, {'id':'uncertainty','label':'Assumptions and uncertainty','text':json.dumps({'assumptions':detail['assumptions'],'gaps':detail['gaps'],'statistics_limits':stats.get('limitations',[]),'statistical_plan':plan},ensure_ascii=False),'source':'Comparison assumptions and gaps; statistical assessment'}]
 for r in matrix['totals']:facts.append({'id':'score-'+r['candidate_id'],'label':r['name']+' score and coverage','text':json.dumps(r,ensure_ascii=False),'source':'Reviewed comparison totals'})
 for c in matrix['cells']:facts.append({'id':'cell-'+c['candidate_id']+'-'+c['criterion_id'],'label':c['candidate_id']+' / '+c['criterion_id'],'text':json.dumps(c,ensure_ascii=False),'source':'; '.join(r['source_id']+' / '+r['locator'] for r in c['refs']) or 'Explicit evidence gap'})
 for i,g in enumerate(matrix['gates']):facts.append({'id':'gate-'+str(i+1),'label':'Mandatory requirement','text':json.dumps(g,ensure_ascii=False),'source':'Reviewed eligibility assessment'})
 for t in stats['comparisons']:facts.append({'id':'test-'+t['id'],'label':t['metric_id']+' analysis','text':json.dumps(t,ensure_ascii=False),'source':'Computed statistical results and declared study design'})
 for s in packet['sources']:facts.append({'id':'source-'+s['id'],'label':s.get('title',s['id']),'text':json.dumps(s,ensure_ascii=False),'source':'Supplied source '+s['id']})
 src={'schema':'bench.publication-source/v1','analysis_manifest_sha256':sha(analysis/'result/manifest.json'),'business_case':packet['job']['business_case'],'candidate_names':{r['candidate_id']:r['name'] for r in matrix['totals']},'decision':pm['details']['selection'],'decision_brief':(analysis/'result/decision-brief.md').read_text(),'matrix':matrix,'statistics':stats,'facts':facts}
 pr={'audience':'Executive decision makers, with a complete supporting report and auditable table data','brand':None}
 if profile:
  supplied=read(profile)
  if not set(supplied)<=set(pr):raise ValueError('Publication profile supports audience and brand only')
  pr.update(supplied)
 run.mkdir(parents=True);write(run/'control/source.json',src);write(run/'control/profile.json',pr);write(run/'control/source-binding.json',{'analysis':str(analysis),'manifest_sha256':sha(analysis/'result/manifest.json'),'files':manifest['artifacts']});(run/'records').mkdir();stage(run,ROLES[0])
def stage(run,role):
 run=Path(run).resolve();dest=run/'stages'/role
 if dest.exists():raise ValueError('Stage already exists')
 (dest/'inputs').mkdir(parents=True);(dest/'output').mkdir();(run/'records'/role).mkdir(exist_ok=True)
 shutil.copyfile(run/'control/source.json',dest/'inputs/source.json');profile=read(run/'control/profile.json');write(dest/'request.json',{'role':role,'audience':profile['audience'],'brand':profile['brand'],'task':'Transform the reviewed expert findings into the contracted professional publication output. Read CONTRACT.md and selected skills. Preserve evidence and the shared narrative. Produce spec and render required artifacts.'})
 if role!='editorial-director':shutil.copyfile(run/'stages/editorial-director/output/spec.json',dest/'inputs/story.json')
 if role in ['executive-writer','presentation-designer','publication-reviewer']:
  shutil.copyfile(run/'stages/information-designer/output/spec.json',dest/'inputs/design.json')
 if role=='executive-writer':shutil.copytree(run/'stages/information-designer/output/graphics',dest/'inputs/graphics')
 if role in ['presentation-designer','publication-reviewer']:shutil.copyfile(run/'stages/executive-writer/output/spec.json',dest/'inputs/document.json')
 if role=='publication-reviewer':
  shutil.copyfile(run/'stages/presentation-designer/output/spec.json',dest/'inputs/presentation.json');shutil.copyfile(run/'control/visual-review.json',dest/'inputs/visual-review.json');shutil.copyfile(run/'control/reader-text.json',dest/'inputs/reader-text.json')
 write(run/'control'/(role+'-inputs.json'),inputs(dest))
def check_stage(run,role):
 run=Path(run).resolve();dest=run/'stages'/role
 if inputs(dest)!=read(run/'control'/(role+'-inputs.json')):raise ValueError('Changed admitted stage inputs')
 return validate(dest,role)
def visual_inputs(run):
 run=Path(run).resolve();images=[];texts={}
 for role in ROLES[1:4]:
  check_stage(run,role);dest=run/'stages'/role;receipt=read(dest/'output/artifacts.json')
  for f in receipt['previews']:images.append({'id':role+'/'+f['path'],'path':str(dest/f['path']),'sha256':f['sha256'],'medium':'workbook' if 'workbook' in f['path'] else 'slide' if role=='presentation-designer' else 'document' if role=='executive-writer' else 'graphic'})
  for f in receipt['files']:
   p=dest/f['path']
   if p.suffix=='.pdf':
    r=subprocess.run([os.environ['PUBLICATION_PDFTOTEXT'],'-layout',str(p),'-'],check=True,capture_output=True,text=True);texts[role+'/'+p.name]=r.stdout
 write(run/'control/visual-inputs.json',images);write(run/'control/reader-text.json',texts)
def finish(run):
 run=Path(run).resolve();specs={role:check_stage(run,role) for role in ROLES};visual=read(run/'control/visual-review.json');imgs=read(run/'control/visual-inputs.json')
 check_visual(run)
 for im in imgs:
  if sha(im['path'])!=im['sha256']:raise ValueError('Changed inspected preview')
 if visual['input_sha256']!=sha(run/'control/visual-inputs.json'):raise ValueError('Stale image review')
 verdict=specs[ROLES[-1]]['content'];status='published' if verdict['verdict']=='publish' and visual['verdict']=='pass' else 'needs-revision';result=run/'result';result.mkdir()
 for role in ROLES[1:4]:
  src=run/'stages'/role/'output'
  for p in src.iterdir():
   if p.name in ['spec.json','artifacts.json']:continue
   if p.is_dir():shutil.copytree(p,result/p.name,dirs_exist_ok=True)
   else:shutil.copyfile(p,result/p.name)
 write(result/'narrative.json',specs[ROLES[0]]);write(result/'publication-review.json',verdict)
 src=read(run/'control/source.json');write(result/'comparison-data.json',src['matrix']);write(result/'statistics.json',src['statistics']);write(result/'source-notes.json',src['facts'])
 with (result/'comparison-data.csv').open('w',newline='') as f:
  w=csv.writer(f);w.writerow(['candidate_id','criterion_id','weight_percent','score_0_to_5','confidence','status','rationale','sources']);cs={c['id']:c for c in src['matrix']['criteria']}
  for c in src['matrix']['cells']:
   row=[c['candidate_id'],c['criterion_id'],cs[c['criterion_id']]['weight'],c['score'],c['confidence'],c['status'],c['rationale'],'; '.join(r['source_id']+' / '+r['locator'] for r in c['refs'])];w.writerow(["'"+x if isinstance(x,str) and x[:1] in '=+@-' else x for x in row])
 if (run/'control/rebuild.json').exists():shutil.copyfile(run/'control/rebuild.json',result/'rebuild.json')
 lines=['# '+specs[ROLES[0]]['content']['title'],'','Publication status: **'+status+'**','','- [Executive presentation](presentation.pptx)','- [Presentation PDF](presentation.pdf)','- [Long-form report](report.docx)','- [Report PDF](report.pdf)','- [Comparison workbook](comparison.xlsx)','- [Report text](report.md)','','All formats follow the same reviewed decision and narrative. See publication-review.json for the release verdict.']
 (result/'index.md').write_text('\n'.join(lines)+'\n');write(result/'manifest.json',{'schema':'bench.publication-result/v1','status':status,'source_sha256':sha(run/'control/source.json'),'visual_review_sha256':sha(run/'control/visual-review.json'),'specs':{r:sha(run/'stages'/r/'output/spec.json') for r in ROLES},'files':[{'path':p.relative_to(result).as_posix(),'sha256':sha(p)} for p in sorted(result.rglob('*')) if p.is_file() and p.name!='manifest.json']});write(run/'status.json',{'status':status,'exit_code':0 if status=='published' else 2,'message':str(result/'index.md')});return 0 if status=='published' else 2
def check(run):
 run=Path(run).resolve();m=read(run/'result/manifest.json')
 check_visual(run)
 for r in ROLES:
  check_stage(run,r)
  if sha(run/'stages'/r/'output/spec.json')!=m['specs'][r]:raise ValueError('Changed publication spec')
 for f in m['files']:
  if sha(safe_file(run/'result',f['path']))!=f['sha256']:raise ValueError('Changed delivered artifact')
 if sha(run/'control/source.json')!=m['source_sha256'] or sha(run/'control/visual-review.json')!=m['visual_review_sha256']:raise ValueError('Changed source/review binding')
 for i in read(run/'control/visual-inputs.json'):
  if sha(i['path'])!=i['sha256']:raise ValueError('Changed reviewed pixels')
 required={'report.docx','report.pdf','report.md','presentation.pptx','presentation.pdf','comparison.xlsx','narrative.json','publication-review.json','index.md'}
 if not required<={f['path'] for f in m['files']}:raise ValueError('Incomplete delivery manifest')
 verdict=read(run/'stages/publication-reviewer/output/spec.json')['content']['verdict'];visual=read(run/'control/visual-review.json')['verdict'];expected='published' if verdict=='publish' and visual=='pass' else 'needs-revision'
 if m['status']!=expected or read(run/'status.json')['status']!=expected:raise ValueError('False publication status')
 print('Valid bound publication result:',m['status'])
def snapshot(run):
 run=Path(run).resolve();selected=set()
 for role in ROLES[:4]:
  root=run/'stages'/role;selected.update([root/'request.json',root/'output/spec.json']);selected.update(p for p in (root/'inputs').rglob('*') if p.is_file())
  if role!=ROLES[0]:
   r=read(root/'output/artifacts.json');selected.add(root/'output/artifacts.json');selected.add(root/'workbook-check.json') if role=='information-designer' else None
   selected.update(root/f['path'] for f in r['files']+r['previews'])
 argv=[os.environ['BENCH_PREFIX']+'/bin/record','run','-f',str(run/'records/publication-files.jsonl'),'-ask',os.environ['BENCH_PREFIX']+'/bin/ask','-label','Selected current publication inputs, specs, editable artifacts and all inspected pixels']
 for p in sorted(selected):argv+=['-input',str(p)]
 subprocess.run(argv+['--','/usr/bin/true'],check=True)
def check_visual(run):
 run=Path(run);ims=read(run/'control/visual-inputs.json');review=read(run/'control/visual-review.json');got=review['images']
 if review['input_sha256']!=sha(run/'control/visual-inputs.json'):raise ValueError('Stale visual review')
 expected={i['id']:i['sha256'] for i in ims}
 if len(got)!=len(expected) or {i['id']:i['image_sha256'] for i in got}!=expected:raise ValueError('Incomplete reviewed image set')
 passing=True
 for i in got:
  if set(i['rubric'])!={'legibility','hierarchy','composition','chart_integrity','consistency'} or any(type(v)!=int or not 1<=v<=5 for v in i['rubric'].values()):raise ValueError('Invalid visual rubric')
  if any(f['severity'] not in ['material','minor'] for f in i['findings']):raise ValueError('Unknown finding severity')
  okay=i['verdict']=='pass' and min(i['rubric'].values())>=4 and not any(f['severity']=='material' for f in i['findings'])
  if i['verdict']=='pass' and not okay:raise ValueError('False image pass')
  passing=passing and okay
 if review['verdict']!=('pass' if passing else 'revise'):raise ValueError('False visual review pass')
def main():
 cmd=sys.argv[1];args=sys.argv[2:]
 if cmd=='prepare':prepare(*args)
 elif cmd=='stage':check_stage(args[0],ROLES[ROLES.index(args[1])-1]);stage(*args)
 elif cmd=='visual-inputs':visual_inputs(*args)
 elif cmd=='snapshot':snapshot(*args)
 elif cmd=='finish':return finish(*args)
 elif cmd=='check':check(*args)
 else:raise ValueError('Unknown operation')
 return 0
if __name__=='__main__':
 try:sys.exit(main())
 except Exception as e:print(str(e),file=sys.stderr);sys.exit(1)
