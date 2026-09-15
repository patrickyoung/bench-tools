#!/usr/bin/env python3
"""Trusted renderer entry. Never loads executable code from an authored spec."""
import hashlib,json,os,shutil,subprocess,sys
from pathlib import Path
from publication_contract import validate,sha,read
root=Path(sys.argv[1] if len(sys.argv)>1 else '.').resolve();expert=Path(__file__).resolve().parents[1];role=read(expert/'role.json')['role'];validate(root,role,False)
out=root/'output';pre=root/'previews';pre.mkdir(exist_ok=True);build=root/'build';build.mkdir(exist_ok=True)
# Keep prior owned renders for diagnosis while producing a fresh receipt and pixels.
if any(p.name!='spec.json' for p in out.iterdir()) or any(pre.iterdir()):
 archive=build/'prior-renders';archive.mkdir(exist_ok=True);revision=archive/str(len(list(archive.iterdir()))+1);revision.mkdir()
 for p in list(out.iterdir()):
  if p.name!='spec.json':shutil.move(str(p),str(revision/p.name))
 shutil.move(str(pre),str(revision/'previews'));pre.mkdir()
env=os.environ.copy();env['MPLCONFIGDIR']=str(root/'build/matplotlib');env['TMPDIR']=str(root/'build/tmp');Path(env['TMPDIR']).mkdir(exist_ok=True)
env['RUNTIME_NODE_MODULES']=env.get('PUBLICATION_NODE_MODULES','')
if role=='information-designer':
 subprocess.run([env['PUBLICATION_PLOT_PYTHON'],str(expert/'lib/render_graphics.py'),str(root)],env=env,check=True,timeout=150)
elif role=='executive-writer':
 subprocess.run([env['PUBLICATION_PYTHON'],str(expert/'lib/render_document.py'),str(root)],env=env,check=True,timeout=210)
if role in ['information-designer','presentation-designer']:
 script=build/'render_office.mjs';shutil.copyfile(expert/'lib/render_office.mjs',script);modules=build/'node_modules'
 if modules.exists() or modules.is_symlink():
  if not modules.is_symlink() or modules.resolve()!=Path(env['PUBLICATION_NODE_MODULES']).resolve():raise ValueError('Unexpected build dependencies')
 else:modules.symlink_to(env['PUBLICATION_NODE_MODULES'],target_is_directory=True)
 subprocess.run([env['PUBLICATION_NODE'],str(script),str(root),role],env=env,check=True,timeout=240)
if role=='presentation-designer':
 subprocess.run([env['PUBLICATION_SOFFICE'],'-env:UserInstallation='+ (build/'lo-profile').as_uri(),'--headless','--convert-to','pdf','--outdir',str(out),str(out/'presentation.pptx')],env=env,check=True,timeout=120)
 if not (out/'presentation.pdf').is_file():raise RuntimeError('Missing presentation PDF')
 subprocess.run([env['PUBLICATION_PDFTOPPM'],'-scale-to','1600','-png',str(out/'presentation.pdf'),str(pre/'slides/slide')],env=env,check=True,timeout=120)
files=[];previews=[]
# Artifact Tool's inspection sidecars are build evidence, not publications.
for p in out.glob('*.inspect.ndjson'):shutil.move(str(p),str(build/p.name))
for p in sorted(out.rglob('*')):
 if p.is_file() and p.name not in ['spec.json','artifacts.json']:files.append({'path':p.relative_to(root).as_posix(),'sha256':sha(p)})
for p in sorted(pre.rglob('*.png')):previews.append({'path':p.relative_to(root).as_posix(),'sha256':sha(p)})
if role=='information-designer':
 for p in sorted((out/'graphics').glob('*.png')):previews.append({'path':p.relative_to(root).as_posix(),'sha256':sha(p)})
receipt={'schema':'bench.publication-artifacts/v1','role':role,'spec_sha256':sha(out/'spec.json'),'renderer_sha256':sha(__file__),'files':files,'previews':previews}
(out/'artifacts.json').write_text(json.dumps(receipt,indent=2)+'\n');validate(root,role);print('Rendered and bound',role,len(files),'files',len(previews),'previews')
