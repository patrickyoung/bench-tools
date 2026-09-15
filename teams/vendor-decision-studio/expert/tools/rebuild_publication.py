#!/usr/bin/env python3
"""Re-render explicitly selected authored content, then require entirely fresh reviews.

This is not fresh specialist authorship. Provenance retains every original spec.
Use a new Agent assignment for content revisions; this command fixes production.
"""
import os,sys,shutil,subprocess
from pathlib import Path
from publication_contract import read,sha,inputs,validate,bindings
from publication_io import write,stage,check_stage,visual_inputs,snapshot,finish,ROLES
team=Path(__file__).resolve().parents[1];old=Path(sys.argv[1]).resolve();run=Path(sys.argv[2]).resolve()
if run.exists():raise ValueError('New rebuild directory required')
selected={}
for role in ROLES[:4]:
 root=old/'stages'/role
 if not (root/'output/spec.json').exists():break
 if inputs(root)!=read(old/'control'/(role+'-inputs.json')):raise ValueError('Changed prior admitted input')
 selected[role]=validate(root,role,False)
 if sha(root/'inputs/source.json')!=sha(old/'control/source.json'):raise ValueError('Conflicting prior source')
run.mkdir(parents=True);(run/'control').mkdir();(run/'records').mkdir()
for name in ['source.json','profile.json','source-binding.json']:shutil.copyfile(old/'control'/name,run/'control'/name)
if not selected:raise ValueError('No valid completed specialist content to rebuild')
write(run/'control/rebuild.json',{'schema':'bench.publication-rebuild/v1','source_run':str(old),'original_specs':{r:{'sha256':sha(old/'stages'/r/'output/spec.json'),'content_reused_without_edit':True} for r in selected},'fresh_authorship':[r for r in ROLES[:4] if r not in selected],'meaning':'Fresh rendering and review of explicitly selected previous specialist content; missing production stages use fresh Agent assignments.'})
def run_agent(role):
 root=run/'stages'/role;definition=team/'agents'/role
 argv=[os.environ.get('COMPARISON_AGENT','agent'),'run','-C',str(root),'-evidence',str(run/'records'/role),'-m',os.environ['ASK_MODEL'],'-effort','high','-turns',os.environ.get('PUBLICATION_TURNS','20'),'-timeout',os.environ.get('PUBLICATION_TIMEOUT','15m'),'-record-input','request.json','-record-output','output/spec.json']
 for p in sorted((root/'inputs').rglob('*')):
  if p.is_file():argv+=['-record-input',p.relative_to(root).as_posix()]
 argv+=[str(definition),'--',f'Read request.json and selected inputs. Definition root: {definition}. Read its CONTRACT.md, role.json and skills from that absolute root. Produce the contracted complete output. The absolute tools/render accepts this workspace path; run absolute bin/check with this workspace as current directory. Reviewers must use the actual fresh image critiques and give a candid publication or revision judgment.']
 with (run/'records'/role/'stdout.txt').open('w') as out,(run/'records'/role/'stderr.txt').open('w') as err:subprocess.run(argv,stdout=out,stderr=err,check=True)
for role in ROLES[:4]:
 stage(run,role);root=run/'stages'/role
 if role not in selected:
  run_agent(role);check_stage(run,role);print('New specialist output:',role,flush=True);continue
 write(root/'output/spec.json',{**bindings(root,role),'content':selected[role]['content']})
 if role!=ROLES[0]:
  argv=[os.environ['BENCH_PREFIX']+'/bin/record','run','-f',str(run/'records'/role/'render.jsonl'),'-ask',os.environ['BENCH_PREFIX']+'/bin/ask','-input',str(root/'request.json'),'-input',str(root/'output/spec.json'),'-output',str(root/'output/artifacts.json'),'--',str(team/'agents'/role/'tools/render'),str(root)]
  with (run/'records'/role/'render.stdout').open('w') as out,(run/'records'/role/'render.stderr').open('w') as err:subprocess.run(argv,stdout=out,stderr=err,check=True)
 check_stage(run,role);print('Rebuilt',role,flush=True)
visual_inputs(run);snapshot(run)
subprocess.run([sys.executable,str(team/'tools/visual_review.py'),str(run),str(team/'agents/publication-reviewer')],check=True)
role=ROLES[-1];stage(run,role);run_agent(role)
sys.exit(finish(run))
