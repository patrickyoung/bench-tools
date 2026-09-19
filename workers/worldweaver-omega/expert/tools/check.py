#!/usr/bin/env python3
import os,sys
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'lib'))
from contracts import *
def check():
    root=Path.cwd().resolve(); home=Path(__file__).resolve().parents[1]
    m,ids,kind=candidate(root,home)
    original=os.environ.get('WW_ADMISSION')
    need(bool(original),'unfinished: original controller admission required (WW_ADMISSION)')
    admitted=check_admission(original,root,m['inputHashes'])
    location=os.environ.get('WW_REVIEW')
    need(bool(location),'unfinished: independent controller review required (WW_REVIEW)')
    path=Path(location)
    if not path.is_absolute() or path.is_symlink() or path.resolve().is_relative_to(root):
        raise Broken('WW_REVIEW must be controller-owned absolute regular file outside workspace')
    # The controller must protect this path against worker writes; paths alone
    # are not a sandbox or an authenticity guarantee.
    try: r=load(path)
    except Reject as e: raise Broken(str(e))
    if not obj(r) or not all(k in r for k in ('candidateHash','inputHashes','policyHash','reviewer','process','criteria')):
        raise Broken('malformed controller receipt')
    need(r['candidateHash']==m['candidateHash'] and r['inputHashes']==m['inputHashes'],'stale independent review')
    need(r.get('admissionHash')==admitted['admissionHash'],'stale original admission review')
    need(r['policyHash']==definition_hash(home),'stale policy review')
    if not text(r['reviewer']) or not obj(r['process']) or not obj(r['criteria']): raise Broken('malformed review metadata')
    for cid in ids:
        v=r['criteria'].get(cid)
        if not obj(v) or v.get('outcome') not in ('satisfied','violated','insufficient_evidence') or not text(v.get('evidence')):
            raise Broken('missing/malformed criterion '+cid)
        need(v['outcome']=='satisfied',f'{cid}: {v["outcome"]}: {v["evidence"]}')
    files=r['process'].get('evidenceFiles')
    if not obj(files) or not files: raise Broken('controller evidence bindings missing')
    preflight([external_path(name,root) for name in files],IMAGE_LIMIT+TEXT_LIMIT+JSON_LIMIT,REFERENCE_LIMIT)
    for name,h in files.items():
        f=Path(name)
        if not f.is_absolute() or f.is_symlink() or f.resolve().is_relative_to(root): raise Broken('invalid external evidence path')
        need(f.is_file() and file_digest(f)==h,'stale/missing independent evidence')
    need(r['process'].get('independent') is True,'independence not attested')
    need(r['process'].get('snapshotReadOnly') is True,'immutable review missing')
    if kind=='world':
        need(r['process'].get('browserBoundary') and r['process'].get('renderedImagesReviewed') is True,'browser/perception evidence missing')
    need(bindings(root)==m,'candidate changed during check')
    print('Accepted only the bound contract and independently reviewed criteria; no universal quality/FPS claim.')
try: check()
except Reject as e: print(str(e),file=sys.stderr); sys.exit(1)
except Broken as e: print('broken check: '+str(e),file=sys.stderr); sys.exit(2)
except (KeyError,TypeError,AttributeError,ValueError) as e: print('malformed candidate: '+str(e),file=sys.stderr); sys.exit(1)
except Exception as e: print('broken check: '+str(e),file=sys.stderr); sys.exit(2)
