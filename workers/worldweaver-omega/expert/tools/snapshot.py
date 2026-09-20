#!/usr/bin/env python3
"""Controller utility. Copy bound bytes only; never run the candidate."""
import argparse,json,os,sys
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'lib'))
from contracts import *
p=argparse.ArgumentParser(description='Create read-only source snapshot outside worker paths; controller must enforce ownership boundary.')
p.add_argument('workspace',type=Path);p.add_argument('destination',type=Path)
p.add_argument('--admission',required=True,type=Path)
a=p.parse_args()
try:
    root=a.workspace.resolve(); dest=a.destination.resolve()
    need(not dest.exists() and not dest.is_relative_to(root),'snapshot must be NEW and outside workspace')
    before=bindings(root);check_admission(a.admission,root,before['inputHashes']);need(read(root,'manifest.json')==before,'stale manifest')
    dest.mkdir(parents=True)
    for rel,h in {**before['inputHashes'],**before['outputHashes']}.items():
        if h is None: continue
        target=dest/rel;target.parent.mkdir(parents=True,exist_ok=True)
        with target.open('xb') as out:
            need(file_digest(safe(root,rel),target=out)==h,'candidate changed while snapshotting')
        target.chmod(0o444)
    (dest/'manifest.json').write_text(json.dumps(before,indent=2)+'\n')
    need(bindings(root)==before and bindings(dest)==before,'snapshot race or stale source')
    for f in dest.rglob('*'):
        if f.is_file():f.chmod(0o444)
    for d in sorted((p for p in dest.rglob('*') if p.is_dir()),reverse=True):d.chmod(0o555)
    dest.chmod(0o555)
    print(before['candidateHash'])
except Reject as e: print(e,file=sys.stderr);sys.exit(1)
