"""Offline Ask protocol tests. Fake judgments never establish semantic quality."""
import json,os,subprocess,sys,unittest
from pathlib import Path
from check_test import Checks,HOME
from contracts import *
class Review(Checks):
 # Reuses fixtures and tests; new cases exercise controller-only wiring.
 def test_snapshot_and_review_statuses(self):
  self.put('request.json',{'style':'Toon'})
  self.put('rejection.json',{'schemaVersion':1,'status':'rejected','code':'missing-input','conflicts':[{'requestSpan':'roughDescription','reason':'Missing'}],'requiredChanges':['Supply required fields']})
  self.bind()
  self.original.write_text(json.dumps(admission(self.w)))
  snap=self.base/'snapshot'
  p=subprocess.run([sys.executable,str(HOME/'tools/snapshot.py'),str(self.w),str(snap),'--admission',str(self.original)],capture_output=True,text=True)
  self.assertEqual(p.returncode,0,p.stderr)
  self.assertEqual(bindings(snap),bindings(self.w))
  evidence=self.base/'evidence.json';evidence.write_text(json.dumps({'candidateHash':bindings(self.w)['candidateHash'],'observations':{'scope':'offline'},'files':[]}))
  fake=self.base/'fake-ask';fake.write_text("""#!/usr/bin/env python3
import json,sys
a=sys.argv[1:];model=a[a.index('-m')+1]
payload=sys.stdin.read()
assert 'untrusted' in a[a.index('-S')+1]
if model=='offline/fail': sys.exit(1)
if model=='offline/malformed': print('{}');sys.exit(0)
schema=json.load(open(a[a.index('-schema')+1]))
status='insufficient_evidence' if model=='offline/unknown' else 'satisfied'
print(json.dumps({k:{'outcome':status,'evidence':'Offline protocol fixture, no real model review.'} for k in schema['required']}))
""");fake.chmod(0o755)
  for model,expected in [('good',0),('unknown',1),('malformed',2),('fail',2)]:
   out=self.base/(model+'.json');session=self.base/(model+'.jsonl')
   p=subprocess.run([sys.executable,str(HOME/'tools/review.py'),'--snapshot',str(snap),'--admission',str(self.original),'--evidence',str(evidence),'--output',str(out),'--session',str(session),'--model','offline/'+model,'--ask',str(fake),'--max-tokens','2000','--timeout','10'],text=True,capture_output=True,env={**os.environ,'PYTHONDONTWRITEBYTECODE':'1'})
   self.assertEqual(p.returncode,expected,p.stdout+p.stderr)
   if model=='good':
    self.review=out;self.check(0)
   if model=='unknown':
    self.review=out;self.check(1)
  # Make fixture dirs writable for cleanup, not part of production workflow.
  for f in snap.rglob('*'):f.chmod(0o755 if f.is_dir() else 0o644)
  snap.chmod(0o755)
if __name__=='__main__':unittest.main()
