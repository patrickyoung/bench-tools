#!/usr/bin/env python3
"""Offline orchestration fixtures; fake tools never contact a provider."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

HERE = Path(__file__).resolve().parent

FAKE_ASK = r'''#!/usr/bin/env python3
import hashlib,json,os,pathlib,sys
a=sys.argv[1:]; request=json.load(sys.stdin)
assert a[a.index('-m')+1]=='fixture/vision'
assert '-schema' in a
schema=json.loads(pathlib.Path(a[a.index('-schema')+1]).read_text())
assert set(schema['properties'])=={'observations','limitations'}
attached=[a[i+1] for i,x in enumerate(a) if x=='-a']
assert attached==[x['snapshot'] for x in request['images']]
for path,item in zip(attached,request['images']):
 assert hashlib.sha256(pathlib.Path(path).read_bytes()).hexdigest()==item['sha256']
pathlib.Path(a[a.index('-f')+1]).write_text('fixture session\n')
mode=os.environ.get('VISUAL_FIXTURE','')
if mode=='fail':sys.exit(7)
rows=[{'criterion_id':c['id'],'image_id':i['id'],'status':'observed','observation':'Visible red square.'} for c in request['rubric']['criteria'] for i in request['images']]
if mode=='uncertain':rows[0]['status']='uncertain'
if mode=='not_visible':rows[0]['status']='not_visible'
if mode=='missing':rows.pop()
if mode=='duplicate':rows.append(rows[0])
if mode=='wrong_id':rows[0]['criterion_id']='invented'
if mode=='wrong_image':rows[0]['image_id']='image-999'
if mode=='bad_status':rows[0]['status']='pass'
if mode=='change_source':pathlib.Path(request['images'][0]['source']).write_bytes(b'changed')
if mode=='bad_json':print('{broken');sys.exit(0)
print(json.dumps({'observations':rows,'limitations':[] if mode=='no_limitations' else ['Only the supplied views were inspected.']}))
'''

FAKE_RECORD = r'''#!/usr/bin/env python3
import json,os,pathlib,subprocess,sys
a=sys.argv[1:]; target=pathlib.Path(a[a.index('-f')+1])
if a[0]=='check':
 json.loads(target.read_text());sys.exit(1 if os.environ.get('VISUAL_FIXTURE')=='bad_record' else 0)
assert a[0]=='run' and '-timeout' in a and '-session' in a
for i,x in enumerate(a):
 if x=='-input':assert pathlib.Path(a[i+1]).is_file()
raw=sys.stdin.buffer.read();r=subprocess.run(a[a.index('--')+1:],input=raw,capture_output=True)
assert pathlib.Path(a[a.index('-session')+1]).is_file()
target.write_text(json.dumps({'argv':a,'stdin':raw.decode(),'stdout':r.stdout.decode(),'stderr':r.stderr.decode(),'status':r.returncode}))
sys.stdout.buffer.write(r.stdout);sys.stderr.buffer.write(r.stderr);sys.exit(r.returncode)
'''

FAKE_CHECK = r'''#!/usr/bin/env python3
import hashlib,json,os,pathlib,sys
a=sys.argv[1:];candidate=pathlib.Path(a[a.index('--candidate')+1]).read_bytes();doc=json.loads(candidate)
assert '--rubric' in a and '--backend' in a
mode=os.environ.get('VISUAL_FIXTURE','')
if mode=='checker_change':pathlib.Path(doc['images'][0]['source']).write_bytes(b'changed')
if mode=='checker_change_candidate':pathlib.Path(a[a.index('--candidate')+1]).write_text('{}')
if mode=='checker_empty':sys.exit(0)
report={'version':1,'verdict':'reject' if mode=='checker_reject' else 'accept','feedback':[],
 'candidate_sha256':hashlib.sha256(candidate).hexdigest(),
 'rubric_sha256':hashlib.sha256(pathlib.Path(a[a.index('--rubric')+1]).read_bytes()).hexdigest()}
if mode=='checker_wrong_candidate':report['candidate_sha256']='0'*64
if mode=='checker_boolean_version':report['version']=True
if mode=='checker_wrong_verdict':report['verdict']='reject'
print(json.dumps(report))
sys.exit(1 if mode=='checker_reject' else 0)
'''


class VisualTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.rubric = self.root / 'rubric.json'
        self.rubric.write_text(json.dumps({'version':1,'criteria':[
            {'id':'color','requirement':'The square is red.','feedback':'Use red.'},
            {'id':'visible','requirement':'The square is visible.','feedback':'Show the square.'}]}))
        self.images = [self.root / 'first.png', self.root / 'second.png']
        for i, image in enumerate(self.images):
            image.write_bytes(b'\x89PNG\r\n\x1a\nfixture-' + str(i).encode())
        for name, content in [('ask',FAKE_ASK),('record',FAKE_RECORD),('checker',FAKE_CHECK)]:
            path=self.root/name;path.write_text(content);path.chmod(0o700)
        self.records=self.root/'records'
        self.env=dict(os.environ)
        self.env.pop('VISUAL_FIXTURE',None)

    def tearDown(self):
        self.tmp.cleanup()

    def observe(self, mode=''):
        env=dict(self.env,VISUAL_FIXTURE=mode)
        args=[sys.executable,str(HERE/'observe.py'),'--model','fixture/vision','--rubric',str(self.rubric),
              '--records',str(self.records),'--ask',str(self.root/'ask'),'--record',str(self.root/'record')]
        for image in self.images:args+=['--image',str(image)]
        return subprocess.run(args,capture_output=True,env=env)

    def check(self, mode='', extra=None):
        return subprocess.run([sys.executable,str(HERE/'check-current.py'),'--observations',str(self.records/'observations.json'),
            '--rubric',str(self.rubric),'--checker',str(self.root/'checker'),'--','--backend','ask']+(extra or []),
            capture_output=True,env=dict(self.env,VISUAL_FIXTURE=mode))

    def test_multiple_images_exact_snapshots_and_binding(self):
        result=self.observe();self.assertEqual(result.returncode,0,result.stderr)
        doc=json.loads(result.stdout)
        self.assertEqual(len(doc['observations']),4)
        for image,binding in zip(self.images,doc['images']):
            self.assertEqual(image.read_bytes(),Path(binding['snapshot']).read_bytes())
            self.assertEqual(binding['sha256'],hashlib.sha256(image.read_bytes()).hexdigest())
        self.assertEqual(doc['observer']['model'],'fixture/vision')
        self.assertEqual(self.records.stat().st_mode & 0o777,0o700)
        result=self.check();self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(json.loads(result.stdout)['verdict'],'accept')

    def test_semantic_checker_inherits_weigh_opt_in_without_bypassing_acceptance(self):
        self.assertEqual(self.observe().returncode, 0)
        records = self.root / 'semantic-records'
        argv = [sys.executable, str(HERE / 'check-current.py'),
                '--observations', str(self.records / 'observations.json'),
                '--rubric', str(self.rubric),
                '--checker', str(HERE.parent / 'semantic-check' / 'check.py'), '--',
                '--backend', 'weigh', '--model', 'fixture/decision',
                '--accept-at', '.8', '--reject-at', '.8', '--records', str(records),
                '--record', str(self.root / 'not-installed'),
                '--weigh', str(self.root / 'not-installed')]
        for value in (None, '', '0', 'invalid'):
            with self.subTest(value=value):
                env = dict(self.env)
                env.pop('BENCH_WEIGH', None)
                if value is not None:
                    env['BENCH_WEIGH'] = value
                result = subprocess.run(argv, capture_output=True, env=env, timeout=15)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertEqual(result.stdout, b'')
                self.assertIn(b'BENCH_WEIGH', result.stderr)
                self.assertFalse(records.exists())

    def test_bad_observer_outputs_never_emit_success(self):
        for mode in ['missing','duplicate','wrong_id','wrong_image','bad_status','bad_json','no_limitations','fail','bad_record','change_source']:
            with self.subTest(mode=mode):
                self.records=self.root/('record-'+mode)
                result=self.observe(mode)
                self.assertNotEqual(result.returncode,0)
                self.assertEqual(result.stdout,b'')

    def test_uncertain_and_unseen_observations_are_retained_but_rejected(self):
        for mode in ['uncertain','not_visible']:
            self.records=self.root/('record-'+mode)
            result=self.observe(mode);self.assertEqual(result.returncode,0,result.stderr)
            result=self.check();self.assertEqual(result.returncode,1,result.stderr)
            self.assertEqual(json.loads(result.stdout)['verdict'],'reject')

    def test_changed_source_or_snapshot_cannot_pass(self):
        self.assertEqual(self.observe().returncode,0)
        self.images[0].write_bytes(b'changed')
        result=self.check();self.assertEqual(result.returncode,2);self.assertEqual(result.stdout,b'')

    def test_changed_snapshot_cannot_pass(self):
        self.assertEqual(self.observe().returncode,0)
        (self.records/'image-1.png').write_bytes(b'changed')
        result=self.check();self.assertEqual(result.returncode,2);self.assertEqual(result.stdout,b'')

    def test_changed_rubric_cannot_pass(self):
        self.assertEqual(self.observe().returncode,0)
        self.rubric.write_text(self.rubric.read_text()+' ')
        result=self.check();self.assertEqual(result.returncode,2);self.assertEqual(result.stdout,b'')

    def test_changed_raw_observation_cannot_pass(self):
        self.assertEqual(self.observe().returncode,0)
        (self.records/'observations.raw.json').write_text('{}')
        result=self.check();self.assertEqual(result.returncode,2);self.assertEqual(result.stdout,b'')

    def test_strict_envelope_versions_shapes_and_paths(self):
        self.assertEqual(self.observe().returncode,0)
        path=self.records/'observations.json'
        original=path.read_text()
        mutations=[('boolean_version',lambda doc:doc.update(version=True)),
                   ('float_version',lambda doc:doc.update(version=1.0)),
                   ('extra_field',lambda doc:doc.update(unsupported='ignored')),
                   ('extra_image_field',lambda doc:doc['images'][0].update(unsupported=True)),
                   ('extra_observer_field',lambda doc:doc['observer'].update(unsupported=True)),
                   ('relative_session',lambda doc:doc['observer'].update(session='observer.jsonl')),
                   ('relative_record',lambda doc:doc['observer'].update(record='invocation.jsonl')),
                   ('relative_raw',lambda doc:doc['observer'].update(raw_output='observations.raw.json')),
                   ('missing_session',lambda doc:doc['observer'].pop('session'))]
        for name,mutate in mutations:
            with self.subTest(name=name):
                doc=json.loads(original);mutate(doc);path.write_text(json.dumps(doc))
                result=self.check();self.assertEqual(result.returncode,2);self.assertEqual(result.stdout,b'')
        path.write_text(original)

    def test_strict_rubric_versions_rejected_before_observation(self):
        original=json.loads(self.rubric.read_text())
        for version in [True,1.0,'1']:
            with self.subTest(version=version):
                self.rubric.write_text(json.dumps(dict(original,version=version)))
                result=self.observe();self.assertEqual(result.returncode,1);self.assertEqual(result.stdout,b'')
                self.assertFalse(self.records.exists())

    def test_inputs_rechecked_after_checker(self):
        for mode in ['checker_change','checker_change_candidate']:
            with self.subTest(mode=mode):
                self.records=self.root/('record-'+mode)
                self.images[0].write_bytes(b'\x89PNG\r\n\x1a\nrestored')
                self.assertEqual(self.observe().returncode,0)
                result=self.check(mode);self.assertEqual(result.returncode,2);self.assertEqual(result.stdout,b'')

    def test_candidate_override_and_help_do_not_bypass(self):
        self.assertEqual(self.observe().returncode,0)
        for extra in [['--candidate','x'],['--cand=x'],['--rub','x'],['--help']]:
            result=self.check(extra=extra);self.assertEqual(result.returncode,2);self.assertEqual(result.stdout,b'')

    def test_rejection_feedback_is_preserved(self):
        self.assertEqual(self.observe().returncode,0)
        result=self.check('checker_reject');self.assertEqual(result.returncode,1)
        self.assertEqual(json.loads(result.stdout)['verdict'],'reject')

    def test_no_check_or_unbound_checker_output_cannot_pass(self):
        self.assertEqual(self.observe().returncode,0)
        for mode in ['checker_empty','checker_wrong_candidate','checker_boolean_version','checker_wrong_verdict']:
            with self.subTest(mode=mode):
                result=self.check(mode);self.assertEqual(result.returncode,2);self.assertEqual(result.stdout,b'')

    def test_existing_records_and_symlink_images_refused(self):
        self.assertEqual(self.observe().returncode,0)
        self.assertNotEqual(self.observe().returncode,0)
        self.records=self.root/'another'
        linked=self.root/'link.png';linked.symlink_to(self.images[0]);self.images=[linked]
        result=self.observe();self.assertNotEqual(result.returncode,0);self.assertFalse(self.records.exists())

    def test_unknown_and_oversized_images_refused_before_invocation(self):
        for mode in ['unknown', 'oversized']:
            with self.subTest(mode=mode):
                self.records=self.root/('record-'+mode)
                if mode=='unknown':
                    self.images[0].write_bytes(b'not an image')
                else:
                    with self.images[0].open('wb') as stream:
                        stream.write(b'\x89PNG\r\n\x1a\n')
                        stream.truncate(16*1024*1024+1)
                result=self.observe()
                self.assertNotEqual(result.returncode,0)
                self.assertEqual(result.stdout,b'')
                self.assertFalse(self.records.exists())


if __name__=='__main__':
    unittest.main()
