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
created_run=False
def failed(kind,error,tb):
 if created_run:write(run/'status.json',{'status':'unfinished','stage':globals().get('role','prepare'),'exit_code':1,'reason':kind.__name__})
 sys.__excepthook__(kind,error,tb)
sys.excepthook=failed
revise=set(sys.argv[3].split(',')) if len(sys.argv)>3 else set()
revision_briefs=read(os.environ['PUBLICATION_REVISION_BRIEF']) if os.environ.get('PUBLICATION_REVISION_BRIEF') else {}
if not isinstance(revision_briefs,dict) or not set(revision_briefs)<=revise or any(not isinstance(v,str) or not 1<=len(v)<=6000 for v in revision_briefs.values()):raise ValueError('Revision brief must map selected revision roles to bounded text')
if not revise <= set(ROLES[:4]):raise ValueError('Revision must name existing production roles')
if run.exists():raise ValueError('New rebuild directory required')
selected={}
for role in ROLES[:4]:
 root=old/'stages'/role
 if not (root/'output/spec.json').exists():break
 if inputs(root)!=read(old/'control'/(role+'-inputs.json')):raise ValueError('Changed prior admitted input')
 try:selected[role]=validate(root,role,False)
 except ValueError:
  if role not in revise:raise
  prior=read(root/'output/spec.json')
  if any(prior.get(k)!=v for k,v in bindings(root,role).items()):raise ValueError('Prior revision spec has stale bindings')
  selected[role]=prior  # Invalid content is selected only as revision evidence, never reused.
 if sha(root/'inputs/source.json')!=sha(old/'control/source.json'):raise ValueError('Conflicting prior source')
run.mkdir(parents=True);created_run=True;(run/'control').mkdir();(run/'records').mkdir()
for name in ['source.json','profile.json','source-binding.json']:shutil.copyfile(old/'control'/name,run/'control'/name)
if not selected:raise ValueError('No valid completed specialist content to rebuild')
write(run/'control/rebuild.json',{'schema':'bench.publication-rebuild/v1','source_run':str(old),'original_specs':{r:{'sha256':sha(old/'stages'/r/'output/spec.json'),'content_reused_without_edit':r not in revise} for r in selected},'fresh_authorship':[r for r in ROLES[:4] if r not in selected or r in revise],'meaning':'Fresh rendering and review of explicitly selected previous content; selected revisions and missing stages use fresh Agent assignments.'})
def run_agent(role):
 root=run/'stages'/role;definition=team/'agents'/role
 argv=[os.environ.get('COMPARISON_AGENT','agent'),'run','-C',str(root),'-evidence',str(run/'records'/role),'-m',os.environ['ASK_MODEL'],'-effort','high','-turns',os.environ.get('PUBLICATION_REVIEW_TURNS','40') if role=='publication-reviewer' else os.environ.get('PUBLICATION_TURNS','30'),'-timeout',os.environ.get('PUBLICATION_TIMEOUT','15m'),'-record-input','request.json','-record-output','output/spec.json']
 for p in sorted((root/'inputs').rglob('*')):
  if p.is_file():argv+=['-record-input',p.relative_to(root).as_posix()]
 argv+=[str(definition),'--',f'Read request.json and selected inputs. Definition root: {definition}. Read its CONTRACT.md, role.json and skills from that absolute root. Produce the contracted complete output. The absolute tools/render accepts this workspace path; run absolute bin/check with this workspace as current directory. Reviewers must use the actual fresh image critiques and give a candid publication or revision judgment. Use targeted batched reads of large JSON, avoid repeatedly dumping whole duplicated source payloads. Once the current bin/check passes and required content is complete, finish the assignment without redundant validation. Reserve time to write the required verdict; a revise verdict is a completed review when quality fails.']
 env=os.environ.copy();env['PUBLICATION_AUTHORING_ONLY']='1'
 with (run/'records'/role/'stdout.txt').open('w') as out,(run/'records'/role/'stderr.txt').open('w') as err:subprocess.run(argv,stdout=out,stderr=err,check=True,env=env)
def render_role(role):
 root=run/'stages'/role
 argv=[os.environ['BENCH_PREFIX']+'/bin/record','run','-f',str(run/'records'/role/'render.jsonl'),'-ask',os.environ['BENCH_PREFIX']+'/bin/ask','-input',str(root/'request.json'),'-input',str(root/'output/spec.json'),'-output',str(root/'output/artifacts.json'),'--',str(team/'agents'/role/'tools/render'),str(root)]
 with (run/'records'/role/'render.stdout').open('w') as out,(run/'records'/role/'render.stderr').open('w') as err:subprocess.run(argv,stdout=out,stderr=err,check=True)
for role in ROLES[:4]:
 stage(run,role);root=run/'stages'/role
 if role in revise and role in selected:
  shutil.copyfile(old/'stages'/role/'output/spec.json',root/'inputs/prior-spec.json')
  if (old/'control/visual-review.json').exists():
   prior=read(old/'control/visual-review.json');write(root/'inputs/prior-visual-review.json',{'source_review_sha256':sha(old/'control/visual-review.json'),'images':[im for im in prior['images'] if im['id'].startswith(role+'/')],'meaning':'Role-specific selection from the exact prior visual review; not a fresh approval.'})
  if (old/'stages/publication-reviewer/output/spec.json').exists():shutil.copyfile(old/'stages/publication-reviewer/output/spec.json',root/'inputs/prior-publication-review.json')
  request=read(root/'request.json');request['revision']='Revise your prior content using the supplied reviews and the current CONTRACT.md capabilities. Preserve every checked fact, decision condition and shared message. Address all material content findings applicable to your format. The current renderer supports sourced effect reference_lines, editable native effect_plot slides, comparison_table table_view=criteria, visible cover body, concise chart sidebars, native Word decision cards and grouped workbook views. Choose the supported encoding that makes the intended argument visible. Write clear decision branches, human terminology, concise paragraphs, explicit candidate labels and score denominators. Editorial: set practical display precision for readers (usually 1–3 decimals, p-values 3 decimals or <0.001), preserve exact values in audit data, and replace any previous blanket six-decimal display requirement. Never change decisions or calculations through rounding. Information design: include signed source-supported practical thresholds when material. Word: put the controlling branch first; avoid repeating every anchor, scenario or ownership block because the workbook and native decision cards preserve them. Do not merely rebind the previous content.';write(root/'request.json',request);write(run/'control'/(role+'-inputs.json'),inputs(root))
 if role not in selected or role in revise:
  if role in revision_briefs:
   (root/'inputs/revision-brief.txt').write_text(revision_briefs[role]+'\n');request=read(root/'request.json');request['revision_scope']='The explicitly selected inputs/revision-brief.txt narrows this assignment. Apply it before general revision suggestions; preserve unaffected content and all authoritative facts.';write(root/'request.json',request);write(run/'control'/(role+'-inputs.json'),inputs(root))
  run_agent(role)
  if role in ROLES[1:4]:render_role(role)
  check_stage(run,role);print('New specialist output:',role,flush=True);continue
 write(root/'output/spec.json',{**bindings(root,role),'content':selected[role]['content']})
 if role!=ROLES[0]:
  render_role(role)
 check_stage(run,role);print('Rebuilt',role,flush=True)
visual_inputs(run);snapshot(run)
subprocess.run([sys.executable,str(team/'tools/visual_review.py'),str(run),str(team/'agents/publication-reviewer')],check=True)
role=ROLES[-1];stage(run,role);run_agent(role)
sys.exit(finish(run))
