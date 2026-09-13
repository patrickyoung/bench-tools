"""Operator-owned dependency tests; invoke with the generated expert directory."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

expert = Path(sys.argv[1]).resolve()
results = []
with tempfile.TemporaryDirectory(prefix='page-team-handoff-') as temp:
    run = Path(temp).resolve()
    prior = run / 'jobs' / 'bench-manage-000001' / 'work'
    work = run / 'jobs' / 'bench-manage-000002' / 'work'
    (prior / 'output').mkdir(parents=True)
    work.mkdir(parents=True)
    payload = b'checked illustration placeholder for a contract test\n'
    source = prior / 'output' / 'asset.txt'
    source.write_bytes(payload)
    manifest = {'schema':'page-team.handoff/v1', 'task_id':'asset-task',
                'specialist':'blender', 'summary':'contract fixture',
                'provenance':[], 'limitations':[], 'files':[
                    {'path':'output/asset.txt','source':str(source),'bytes':len(payload),
                     'sha256':hashlib.sha256(payload).hexdigest(),'media_type':'text/plain'}]}
    def invoke(name, mutate=None, corrupt_digest=False, expected=0):
        value = json.loads(json.dumps(manifest))
        if mutate: mutate(value)
        content = json.dumps(value) + '\n'
        dep = {'id':'asset-task','job':'bench-manage-000001','content':content,
               'sha256':'sha256:'+('0'*64 if corrupt_digest else hashlib.sha256(content.encode()).hexdigest())}
        packet = {'task':{'id':'next-task','input':{'worker':'frontend'}},'dependencies':[dep]}
        attempt = work / name
        # Retain the controller's documented work layout for each independent case.
        import shutil
        if (work / 'inputs').exists(): shutil.rmtree(work / 'inputs')
        p = subprocess.run([str(expert/'bin/materialize-inputs')],cwd=work,
                           input=json.dumps(packet),text=True,capture_output=True,timeout=5)
        passed = p.returncode == 0 if expected == 0 else p.returncode != 0
        results.append({'case':name,'passed':passed,'exit':p.returncode,
                        'diagnostic':(p.stdout+p.stderr).strip()[:500]})
    invoke('valid')
    invoke('wrong-dependency-digest',corrupt_digest=True,expected=1)
    invoke('wrong-task-binding',lambda x:x.update(task_id='different-task'),expected=1)
    invoke('escaping-relative-target',lambda x:x['files'][0].update(path='../../../../escaped.txt'),expected=1)
    invoke('absolute-relative-target',lambda x:x['files'][0].update(path=str(run/'escaped-absolute.txt')),expected=1)
    invoke('changed-size',lambda x:x['files'][0].update(bytes=len(payload)+1),expected=1)
    other = run/'outside.txt'; other.write_bytes(payload)
    invoke('wrong-source-assignment',lambda x:x['files'][0].update(source=str(other)),expected=1)
    source.unlink(); source.symlink_to(other)
    invoke('symlink-source',expected=1)
print(json.dumps(results,indent=2))
raise SystemExit(0 if all(x['passed'] for x in results) else 1)
