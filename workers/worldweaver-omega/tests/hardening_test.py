"""Independent mechanical AMEND-01 cases; no semantic acceptance or world claim."""
import os,json,sys,subprocess,unittest
from unittest.mock import patch
from pathlib import Path
from check_test import Checks,HOME
from contracts import *
import contracts
class Hardening(Checks):
 def sparse(self,p,n):
  p.parent.mkdir(parents=True,exist_ok=True)
  with p.open('wb') as f:f.truncate(n)
 def test_canonical_spellings(self):
  self.put('a/file.txt','valid')
  for rel in ['./a/file.txt','a/./file.txt','a//file.txt','a/file.txt/','/a/file.txt','a/../file.txt','','.','a\\file.txt','a\0file']:
   with self.subTest(rel=rel),self.assertRaises(Reject):safe(self.w,rel,False)
  self.assertEqual(safe(self.w,'a/file.txt'),self.w/'a/file.txt')
 def test_regular_and_size_before_open(self):
  f=self.w/'huge.json';self.sparse(f,JSON_LIMIT+1)
  with patch('contracts.os.open',side_effect=AssertionError('payload opened')):
   with self.assertRaises(Reject):load(f)
  f.unlink();os.mkfifo(f)
  with patch('contracts.os.open',side_effect=AssertionError('FIFO opened')):
   with self.assertRaises(Reject):load(f)
  f.unlink();f.symlink_to('/dev/zero')
  with self.assertRaises(Reject):file_digest(f)
 def test_outputs_preflight_before_any_hash(self):
  self.put('request.json',self.r)
  for n in [FILE_LIMIT+1,FILE_LIMIT//2+1]:
   out=self.w/'phase-03/runtime/dist'
   self.sparse(out/'a.bin',n)
   if n<FILE_LIMIT:self.sparse(out/'b.bin',n)
   with patch('contracts.file_digest',side_effect=AssertionError('hash before aggregate preflight')):
    with self.assertRaises(Reject):bindings(self.w)
 def test_references_preflight_before_hash(self):
  refs=[f'refs/{i}.bin' for i in range(5)]
  self.put('request.json',{**self.r,'references':refs})
  for rel in refs:self.sparse(self.w/rel,REFERENCE_LIMIT)
  with patch('contracts.file_digest',side_effect=AssertionError('hash before aggregate preflight')):
   with self.assertRaises(Reject):input_hashes(self.w)
  self.sparse(self.w/refs[0],REFERENCE_LIMIT+1)
  with self.assertRaises(Reject):input_hashes(self.w)
 def test_evidence_preflight_no_payload_read(self):
  self.fixture();m=bindings(self.w)
  def check_files(kind,sizes):
   fs=[]
   for i,n in enumerate(sizes):
    p=self.base/f'e-{i}.{"png" if kind=="image" else "txt"}';self.sparse(p,n)
    fs.append({'path':str(p),'sha256':'0'*64,'kind':kind})
   with patch('contracts.bounded_bytes',side_effect=AssertionError('evidence payload read')),patch('contracts.file_digest',side_effect=AssertionError('evidence hashed')):
    with self.assertRaises(Reject):review_plan(self.w,m,{'observations':{},'files':fs})
  check_files('record',[512001])
  check_files('image',[REFERENCE_LIMIT+1])
  check_files('record',[500000]*3)
  check_files('image',[REFERENCE_LIMIT]*3)
  self.put('phase-03/runtime/src/large.js','x'*(TEXT_LIMIT+1))
  with self.assertRaises(Reject):review_plan(self.w,bindings(self.w),{'observations':{},'files':[]})
 def test_original_input_rebinding_rejected_through_public_tools(self):
  # Capture originals before even constructing candidate outputs.
  self.put('request.json',{**self.r,'references':['ref.txt']});self.put('ref.txt','Original reference')
  cmd=[sys.executable,str(HOME/'tools/admit.py'),str(self.w),str(self.original)]
  p=subprocess.run(cmd,capture_output=True,text=True);self.assertEqual(p.returncode,0,p.stderr)
  original=self.original.read_bytes()
  self.put('rejection.json',{'schemaVersion':1,'status':'rejected','code':'causality-conflict','conflicts':[{'requestSpan':'fixture','reason':'protocol only'}],'requiredChanges':['review']})
  self.bind();self.receipt(renew_admission=False);self.check(0)
  snap=self.base/'original-snapshot'
  p=subprocess.run([sys.executable,str(HOME/'tools/snapshot.py'),str(self.w),str(snap),'--admission',str(self.original)],capture_output=True,text=True)
  self.assertEqual(p.returncode,0,p.stderr)
  # Rewrite request then recompute manifest AND mock review of current bytes.
  self.put('request.json',{**self.r,'roughDescription':'altered original intent','references':['ref.txt']})
  self.bind();self.receipt(renew_admission=False)
  self.assertIn('original admission',self.check(1).stderr)
  p=subprocess.run([sys.executable,str(HOME/'tools/snapshot.py'),str(self.w),str(self.base/'bad-snap'),'--admission',str(self.original)],capture_output=True,text=True)
  self.assertEqual(p.returncode,1,p.stderr);self.assertIn('original admission',p.stderr)
  # Direct review must fail even if an attacker supplies a rebound read-only snapshot.
  for f in self.w.rglob('*'):
   if f.is_file():f.chmod(0o444)
  p=subprocess.run([sys.executable,str(HOME/'tools/review.py'),'--snapshot',str(self.w),'--admission',str(self.original),'--evidence',str(self.base/'absent.json'),'--output',str(self.base/'no-receipt.json'),'--session',str(self.base/'session'),'--model','openai-codex/gpt-6-astra','--max-tokens','0','--timeout','10','--ask','/must-not-execute'],capture_output=True,text=True)
  self.assertEqual(p.returncode,1,p.stderr);self.assertIn('original admission',p.stderr)
  self.assertFalse((self.base/'no-receipt.json').exists())
  for f in self.w.rglob('*'):
   if f.is_file():f.chmod(0o644)
  self.put('request.json',{**self.r,'references':['ref.txt']});self.put('ref.txt','Altered reference');self.bind();self.receipt(renew_admission=False)
  self.assertIn('original admission',self.check(1).stderr)
  self.assertEqual(self.original.read_bytes(),original)
  for f in snap.rglob('*'):f.chmod(0o755 if f.is_dir() else 0o644)
  snap.chmod(0o755)
 def test_snapshot_streams_and_codex_stdin_large_packet(self):
  self.put('request.json',{'style':'Toon'})
  self.put('rejection.json',{'schemaVersion':1,'status':'rejected','code':'missing-input','conflicts':[{'requestSpan':'description','reason':'Missing'}],'requiredChanges':['supply']})
  self.put('notes.md','x'*300000)
  self.bind();self.original.write_text(json.dumps(admission(self.w)))
  snap=self.base/'snapshot'
  cmd=[sys.executable,str(HOME/'tools/snapshot.py'),str(self.w),str(snap),'--admission',str(self.original)]
  p=subprocess.run(cmd,capture_output=True,text=True);self.assertEqual(p.returncode,0,p.stderr)
  evidence=self.base/'evidence.json';evidence.write_text(json.dumps({'candidateHash':bindings(self.w)['candidateHash'],'observations':{},'files':[]}))
  fake=self.base/'ask-fixture'
  fake.write_text("""#!/usr/bin/env python3
import sys,json
a=sys.argv[1:];payload=sys.stdin.read()
assert '-max-tokens' not in a
assert sum(map(len,a))<10000 and len(payload)>300000
packet=json.loads(payload.split('\\n',1)[1])
assert packet['originalAdmission']['inputHashes']['request.json']
s=json.load(open(a[a.index('-schema')+1]))
print(json.dumps({k:{'outcome':'insufficient_evidence','evidence':'Protocol-only fixture; no actual semantic review.'} for k in s['required']}))
""");fake.chmod(0o755)
  p=subprocess.run([sys.executable,str(HOME/'tools/review.py'),'--snapshot',str(snap),'--admission',str(self.original),'--evidence',str(evidence),'--output',str(self.review),'--session',str(self.base/'session'),'--model','openai-codex/gpt-6-astra','--max-tokens','0','--timeout','10','--ask',str(fake)],capture_output=True,text=True)
  self.assertEqual(p.returncode,1,p.stderr);self.assertTrue(self.review.exists(),p.stderr)
  self.check(1)
  for f in snap.rglob('*'):f.chmod(0o755 if f.is_dir() else 0o644)
  snap.chmod(0o755)
if __name__=='__main__':unittest.main()
