"""Real public team entries/delivery adapters with deterministic collaborators.

Manage, Agent and comparison phase helpers here are explicit protocol fixtures.
These cases prove adapter composition, not model quality or the collaborators'
internal checks; the existing team suites cover their real handoff contracts.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

from test_process import p, ROOT, TEND


@unittest.skipUnless(TEND and (ROOT/'teams/page-team').exists(), 'requires Tend and Bench team source')
class TeamEntryTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.base=Path(self.temp.name).resolve(); self.root=self.base/'commitments'
        self.agent=self.program('agent', '#!/bin/sh\nexit 0\n')

    def program(self,name,source):
        path=self.base/name;path.write_text(source);path.chmod(0o755);return path

    def export(self,name):
        # Assemble only approved team template source. Collaborator fixtures
        # deliberately replace external boundaries below; this is not a release.
        self.exported=self.base/name;self.expert=self.exported/'expert'
        metadata=p.load(ROOT/'teams'/name/'team.json')
        for source in metadata['files']:
            relative=Path(source).relative_to(Path('teams')/name/'expert')
            target=self.expert/relative;target.parent.mkdir(parents=True,exist_ok=True)
            shutil.copy2(ROOT/source,target)
        self.members=metadata['members'];self.team=name

    def seal(self):
        files={path.relative_to(self.exported).as_posix():{'sha256':p.sha(path.read_bytes()),
              'mode':'100755' if os.access(path,os.X_OK) else '100644'}
              for path in self.expert.rglob('*') if path.is_file()}
        (self.exported/'team.lock.json').write_bytes(p.encode({'schema':'bench.team-lock/v1',
             'team':self.team,'source':{'commit':'2'*40},'members':self.members,'files':files,'adaptations':[]}))

    def request(self,inputs,environment):
        return {'schema':'bench.commitment/v1','id':'pilot','namespace':self.team,'owner':'Fixture owner',
                'objective':'Protocol pilot, not a live model evaluation.','not_before':'2000-01-01T00:00:00Z',
                'due_at':'2030-01-01T00:00:00Z','timezone':'UTC','inputs':inputs,
                'environment':environment,'pass_env':[],'milestones':[]}

    def work(self):
        result=subprocess.run([TEND,'work'],env=dict(os.environ,TEND_ROOT=str(self.root/'tend')),
                              capture_output=True,timeout=15)
        self.assertEqual(result.returncode,0,result.stderr.decode())

    def prepare_page(self,mode='accepted'):
        self.export('page-team')
        self.manage=self.program('manage','''#!/usr/bin/env python3
# Deterministic public-Manage protocol fixture; no actual model review.
import hashlib,json,pathlib,sys
MODE='''+repr(mode)+'''
command=sys.argv[1]
if command == 'run':
    root=pathlib.Path(sys.argv[sys.argv.index('-C')+1]);root.mkdir(exist_ok=True)
    brief=sys.stdin.read();assert brief
    (root/'manifest.json').write_text(json.dumps({'goal':brief}))
    output=root/'jobs/bench-manage-000001/work/output/index.html';output.parent.mkdir(parents=True)
    data=b'<!doctype html><title>Protocol fixture</title><p>Fixture only</p>'
    output.write_bytes(data)
    result=json.dumps({'files':[{'path':'output/index.html','source':str(output),
      'sha256':hashlib.sha256(data).hexdigest(),'bytes':len(data)}]})
    (root/'fixture-result').write_text(result)
    if MODE == 'unknown': sys.exit(2)
    print(result)
elif command == 'resume':
    root=pathlib.Path(sys.argv[2]);print((root/'fixture-result').read_text())
elif command == 'status':
    root=pathlib.Path(sys.argv[-1]);print(json.dumps({'outcome':MODE,'result':(root/'fixture-result').read_text()}))
else:sys.exit(1)
''')
        self.seal()
        brief=self.base/'brief.md';brief.write_text('Create the explicit protocol fixture.')
        return self.request({'brief':str(brief)},{'AGENT':str(self.agent),'BENCH_MANAGE':str(self.manage),'ASK_MODEL':'fixture/no-model'})

    def test_page_actual_entry_and_delivery_check(self):
        request=self.prepare_page()
        instance=p.admit(self.exported,request,self.root,TEND);p.submit(instance);self.work()
        state=p.status(instance,'2035-01-01T00:00:00Z')
        self.assertEqual(state['acceptance'],'accepted',state)
        self.assertTrue((instance/'run/accepted-manifest.json').exists())
        self.assertEqual((instance/'run/result.html').read_bytes(),
                         (self.root/'tend/jobs'/instance.name/'attempts/001.out').read_bytes())
        # A changed delivery fails the actual new delivery adapter.
        (instance/'run/result.html').write_text('unreviewed change')
        checked=subprocess.run([str(self.expert/'bin/check-process'),str(instance/'run')],
                    env=dict(os.environ,BENCH_MANAGE=str(self.manage)),capture_output=True)
        self.assertNotEqual(checked.returncode,0)

    def test_page_nested_unknown_keeps_outer_fence(self):
        request=self.prepare_page('unknown')
        instance=p.admit(self.exported,request,self.root,TEND);p.submit(instance);self.work()
        state=p.status(instance,'2035-01-01T00:00:00Z')
        self.assertEqual(state['execution'],'unknown',state)
        self.assertEqual(state['acceptance'],'unconfirmed',state)
        request['id']='next-occurrence'
        next_instance=p.admit(self.exported,request,self.root,TEND);p.submit(next_instance)
        result=subprocess.run([TEND,'work'],env=dict(os.environ,TEND_ROOT=str(self.root/'tend')),capture_output=True)
        self.assertEqual(result.returncode,1)
        self.assertFalse((next_instance/'run').exists())

    def test_page_resume_after_wrapper_marker_before_inner_admission(self):
        request=self.prepare_page()
        instance=p.admit(self.exported,request,self.root,TEND)
        # Crash-window fixture: wrapper started, inner manifest not yet created.
        (instance/'control/started.json').write_text('{}')
        p.submit(instance);self.work()
        self.assertEqual(p.status(instance,'2035-01-01T00:00:00Z')['acceptance'],'accepted')

    def test_page_plain_html_without_manage_acceptance_is_rejected(self):
        request=self.prepare_page('incomplete')
        instance=p.admit(self.exported,request,self.root,TEND);p.submit(instance);self.work()
        state=p.status(instance,'2035-01-01T00:00:00Z')
        self.assertEqual(state['acceptance'],'needs-revision',state)
        self.assertEqual(state['entry_exit'],0)

    def prepare_comparison(self,verdict):
        self.export('vendor-comparison-team')
        # The shell entry and both check adapters remain the actual team source.
        # Replace its phase collaborator with a deterministic, disclosed fixture.
        (self.expert/'tools/team_io.py').write_text('''import hashlib,json,pathlib,sys
phase=sys.argv[1]
if phase=='prepare':
    job=pathlib.Path(sys.argv[2]);root=pathlib.Path(sys.argv[3]);root.mkdir()
    spec=json.loads(job.read_text());assert (job.parent/'materials/source.txt').read_text()=='selected source'
    (root/'control').mkdir();(root/'result').mkdir()
    (root/'control/job.json').write_text(json.dumps(spec))
    (root/'control/analyst-request.json').write_text('{}')
    (root/'control/comparison-admission.json').write_text(json.dumps({'packet_sha256':'0'*64}))
    for role in ['manager','analyst','comparison','synthesis','reviewer']:
        (root/'stages'/role/'inputs').mkdir(parents=True)
elif phase=='finish':
    root=pathlib.Path(sys.argv[2]);job=json.loads((root/'control/job.json').read_text())
    verdict=job['verdict'];status={'proceed':'reviewed','revise':'requires-revision','hold':'needs-input'}[verdict]
    for name in ['analysis.json','matrix.json','matrix.csv','evidence.md','intake.md','intake.json',
       'review.md','statistics.json','statistical-plan.json','statistics.md','decision-brief.md','decision-brief.json','report.md']:
        (root/'result'/name).write_text('Protocol fixture only')
    (root/'result/review.json').write_text(json.dumps({'status':'ready','details':{'decision':verdict}}))
    (root/'status.json').write_text(json.dumps({'status':status}))
    (root/'result/manifest.json').write_text(json.dumps({'status':status}))
    # Exit-zero malformed contract deliberately tests the stronger delivery gate.
    sys.exit(job.get('finish_exit',0))
elif phase=='check':
    root=pathlib.Path(sys.argv[2]);assert (root/'result/manifest.json').is_file()
# Other phase fixtures are deliberate no-ops, not real handoff validation.
''')
        self.seal()
        job=self.base/'job.json';job.write_text(json.dumps({'verdict':verdict}))
        source=self.base/'source.txt';source.write_text('selected source')
        request=self.request({'job':str(job)},{'COMPARISON_AGENT':str(self.agent),'ASK_MODEL':'fixture/no-model'})
        request['supporting_files']={'materials/source.txt':str(source)}
        return request

    def test_comparison_actual_entry_and_delivery_check(self):
        request=self.prepare_comparison('proceed')
        instance=p.admit(self.exported,request,self.root,TEND);p.submit(instance);self.work()
        state=p.status(instance,'2035-01-01T00:00:00Z')
        self.assertEqual(state['acceptance'],'accepted',state)
        self.assertEqual(len(p.load(instance/'control'/next((instance/'control').glob('receipt-*')).name)['artifacts']),16)

    def test_comparison_structural_pass_with_revision_is_unaccepted(self):
        request=self.prepare_comparison('revise')
        instance=p.admit(self.exported,request,self.root,TEND);p.submit(instance);self.work()
        state=p.status(instance,'2035-01-01T00:00:00Z')
        self.assertEqual(state['acceptance'],'needs-revision',state)
        self.assertEqual(state['entry_exit'],0)

    def test_comparison_retained_business_verdict_survives_nonzero_entry(self):
        request=self.prepare_comparison('hold')
        job=Path(request['inputs']['job']);value=p.load(job);value['finish_exit']=75;job.write_bytes(p.encode(value))
        instance=p.admit(self.exported,request,self.root,TEND);p.submit(instance);self.work()
        state=p.status(instance,'2035-01-01T00:00:00Z')
        self.assertEqual((state['execution'],state['acceptance'],state['entry_exit']),('waiting','needs-input',75),state)


if __name__=='__main__':unittest.main(verbosity=2)
