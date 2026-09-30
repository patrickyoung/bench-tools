"""Offline contracts; runtime cases invoke real Tend and labeled team fixtures."""
import copy
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from concurrent.futures import ThreadPoolExecutor

APP = Path(__file__).resolve().parents[1] / 'process.py'
ROOT = APP.parents[2]
spec = importlib.util.spec_from_file_location('team_process', APP)
p = importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)
TEND = os.environ.get('PROCESS_TEST_TEND')


def definition():
    return {
        'schema': 'bench.team-process/v1', 'id': 'fixture', 'team': 'fixture',
        'purpose': 'Deterministic process-boundary fixture, not model quality.',
        'owner_role': 'manager', 'inputs': ['brief'],
        'stages': [{'id': 'deliver', 'role': 'manager', 'needs': [],
                    'outcome': 'Copy exact selected input.', 'evidence': ['result.txt']}],
        'entry': {'argv': ['bin/run', '{input.brief}', '{run}'], 'stdin': None,
                  'environment': {}, 'continuation': 'new-run'},
        'verification': ['bin/verify', '{run}'], 'artifacts': ['result.txt'],
        'configuration': {'MODE': 'text'},
        'agreements': [{'requirement': 'Use exact supplied bytes.', 'enforcement': 'verification'}],
        'escalation': 'Report to the commitment owner; no automatic effect.'}


def calendar():
    return {'schema': 'bench.team-calendar/v1', 'id': 'daily-review',
            'timezone': 'America/New_York', 'start_date': '2026-03-06', 'end_date': '2026-03-09',
            'weekdays': [0, 1, 2, 3, 4], 'excluded_dates': [], 'start_time': '09:00',
            'due_time': '17:00', 'due_day_offset': 0, 'missed': 'all'}


class CalendarTests(unittest.TestCase):
    def test_weekday_dst_and_stable_identity(self):
        first = p.plan(definition(), calendar(), '2026-03-09T14:00:00Z')
        again = p.plan(definition(), calendar(), '2026-03-10T14:00:00Z')
        self.assertEqual([{k:v for k,v in i.items() if k != 'overdue'} for i in first],
                         [{k:v for k,v in i.items() if k != 'overdue'} for i in again])
        self.assertEqual([i['not_before'] for i in first], ['2026-03-06T14:00:00Z', '2026-03-09T13:00:00Z'])
        self.assertEqual(first[0]['id'], 'daily-review/2026-03-06')

    def test_missed_tick_policies_and_exclusions(self):
        c = calendar()
        c['missed'] = 'latest'
        self.assertEqual(len(p.plan(definition(), c, '2026-03-09T15:00:00Z')), 1)
        c['missed'] = 'skip'
        self.assertEqual(len(p.plan(definition(), c, '2026-03-09T15:00:00Z')), 1)
        self.assertEqual(p.plan(definition(), c, '2026-03-10T15:00:00Z'), [])
        c['missed'] = 'all'; c['excluded_dates'] = ['2026-03-09']
        self.assertEqual(len(p.plan(definition(), c, '2026-03-10T15:00:00Z')), 1)

    def test_exact_start_and_due_boundary(self):
        c = calendar(); c['missed'] = 'skip'
        self.assertEqual(p.plan(definition(), c, '2026-03-06T13:59:59Z'), [])
        self.assertEqual(len(p.plan(definition(), c, '2026-03-06T14:00:00Z')), 1)
        self.assertFalse(p.plan(definition(), c, '2026-03-06T22:00:00Z')[0]['overdue'])
        self.assertEqual(p.plan(definition(), c, '2026-03-06T22:00:01Z'), [])

    def test_nonexistent_and_ambiguous_local_times(self):
        for day, clock in [('2026-03-08', '02:30'), ('2026-11-01', '01:30')]:
            c = calendar(); c.update(start_date=day, end_date=day, weekdays=[6], start_time=clock)
            with self.assertRaisesRegex(ValueError, 'ambiguous or nonexistent'):
                p.plan(definition(), c, '2026-12-01T00:00:00Z')

    def test_finite_calendar_and_invalid_shapes(self):
        for update in ({'end_date': '2028-01-01'}, {'due_time': '08:00'}, {'weekdays': [True]},
                       {'missed': 'invent'}, {'due_day_offset': -1}):
            c = calendar(); c.update(update)
            with self.assertRaises(ValueError):
                p.plan(definition(), c, '2026-12-01T00:00:00Z')

    def test_strict_json_and_process_graph(self):
        for raw in (b'{"id":1,"id":2}', b'{"number":NaN}', b'"\\ud800"'):
            with self.assertRaises((ValueError, UnicodeError)):
                p.decode(raw)
        d = definition(); d['stages'][0]['needs'] = ['deliver']
        with self.assertRaises(ValueError): p.process(d)

    def test_shipped_processes_validate(self):
        if not (ROOT / 'teams/page-team').exists():
            self.skipTest('source-catalog check requires the Bench checkout')
        for team in ('page-team', 'vendor-comparison-team'):
            d = p.process(p.load(ROOT / 'teams' / team / 'expert/process.json'))
            self.assertEqual(d['team'], team)


@unittest.skipUnless(TEND, 'set PROCESS_TEST_TEND to a real Tend executable')
class TendIntegrationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name).resolve()
        self.export = self.base / 'export'
        self.expert = self.export / 'expert'
        (self.expert / 'bin').mkdir(parents=True)
        self.root = self.base / 'commitments'
        self.input = self.base / 'brief.txt'; self.input.write_text('Fresh selected input.\n')
        self.d = definition()
        (self.expert / 'bin/run').write_text('''#!/usr/bin/env python3
import os, pathlib, sys, time
source, root = map(pathlib.Path, sys.argv[1:])
root.mkdir()
(root/'invocation').write_text('one')
mode = os.environ.get('MODE','pass')
if mode == 'wait':
    (root/'started').touch()
    while not (root/'release').exists(): time.sleep(.02)
if mode == 'unknown':
    (root/'effect').write_text('external effect fixture')
    sys.exit(125)
if mode == 'signal':
    import signal
    (root/'effect').write_text('effect before signal')
    os.kill(os.getpid(), signal.SIGTERM)
if mode == 'ambient':
    assert 'UNDECLARED_SETTING' not in os.environ
if mode == 'needs-input': sys.exit(75)
if mode == 'revision': sys.exit(2)
print('TEAM STDOUT')
print('TEAM STDERR', file=sys.stderr)
(root/'result.txt').write_bytes(source.read_bytes())
''')
        (self.expert / 'bin/verify').write_text('''#!/usr/bin/env python3
import os, pathlib, sys
mode=os.environ.get('MODE','pass')
if mode == 'reject-check': sys.exit(1)
if mode == 'broken-check': sys.exit(2)
if mode == 'signal-check':
    import signal
    os.kill(os.getpid(), signal.SIGTERM)
assert (pathlib.Path(sys.argv[1])/'result.txt').read_bytes() == b'Fresh selected input.\\n'
print('FINAL CHECK')
''')
        for path in (self.expert / 'bin').iterdir(): path.chmod(0o755)
        self.seal_bundle()
        self.request = {'schema': 'bench.commitment/v1', 'id': 'daily/2026-09-28',
                        'namespace': 'test-team', 'owner': 'Case owner', 'objective': 'Copy supplied input.',
                        'not_before': '2000-01-01T00:00:00Z', 'due_at': '2030-01-01T00:00:00Z',
                        'timezone': 'UTC', 'inputs': {'brief': str(self.input)},
                        'environment': {}, 'pass_env': [], 'milestones': []}

    def seal_bundle(self):
        (self.expert / 'process.json').write_bytes(p.encode(self.d))
        files = {path.relative_to(self.export).as_posix(): {'sha256': p.sha(path.read_bytes()),
                   'mode': '100755' if os.access(path, os.X_OK) else '100644'}
                 for path in self.expert.rglob('*') if path.is_file()}
        (self.export / 'team.lock.json').write_bytes(p.encode({'schema': 'bench.team-lock/v1',
             'team': 'fixture', 'source': {'commit': '1' * 40}, 'files': files,
             'members': {'manager': {}}, 'adaptations': []}))

    def admit(self):
        return p.admit(self.export, self.request, self.root, TEND)

    def tend(self, *args, expected=0):
        result = subprocess.run([TEND, *args], env=dict(os.environ, TEND_ROOT=str(self.root / 'tend')),
                                input=b'', capture_output=True, timeout=15)
        self.assertEqual(result.returncode, expected, result.stderr.decode())
        return result

    def run_case(self):
        instance = self.admit(); p.submit(instance); self.tend('work')
        return instance, p.status(instance, '2035-01-01T00:00:00Z')

    def test_exact_streams_acceptance_and_duplicate_submission(self):
        instance, state = self.run_case()
        self.assertEqual((state['execution'], state['acceptance'], state['timeliness']), ('done', 'accepted', 'on-time'))
        self.assertEqual(self.admit(), instance)
        p.submit(instance); self.tend('work', expected=1)
        events = self.tend('events', instance.name).stdout
        self.assertEqual(events.count(b'"kind":"attempt.started"'), 1)
        attempts = self.root / 'tend/jobs' / instance.name / 'attempts'
        self.assertEqual((attempts / '001.out').read_text(), 'TEAM STDOUT\n')
        self.assertIn('TEAM STDERR', (attempts / '001.err').read_text())
        self.assertIn('FINAL CHECK', (attempts / '001.err').read_text())
        self.tend('check')

    def test_concurrent_duplicate_admission_and_conflicting_id(self):
        with ThreadPoolExecutor(max_workers=4) as pool:
            instances = list(pool.map(lambda _: self.admit(), range(4)))
        self.assertEqual(len(set(instances)), 1)
        with ThreadPoolExecutor(max_workers=4) as pool:
            results = list(pool.map(p.submit, instances))
        self.assertEqual(len({r['job_id'] for r in results}), 1)
        self.request['objective'] = 'Changed promise'
        with self.assertRaisesRegex(ValueError, 'different bytes'): self.admit()

    def test_lost_submission_response_needs_no_local_ack(self):
        instance = self.admit(); p.submit(instance)
        # There is deliberately no app-side submitted flag to lose or reconstruct.
        p.submit(instance); self.tend('work'); p.submit(instance)
        self.tend('work', expected=1)
        self.assertEqual(p.status(instance, '2035-01-01T00:00:00Z')['acceptance'], 'accepted')

    def test_accepted_late_keeps_both_dimensions(self):
        self.request['due_at'] = '2001-01-01T00:00:00Z'
        _, state = self.run_case()
        self.assertEqual((state['acceptance'], state['timeliness']), ('accepted', 'late'))

    def test_rejection_broken_check_and_business_wait(self):
        for mode, execution, acceptance in [('revision','failed','unfinished'),
                   ('needs-input','waiting','needs-input'), ('reject-check','failed','needs-revision'),
                   ('broken-check','failed','broken-verification')]:
            with self.subTest(mode=mode):
                self.request['id'] = mode; self.request['namespace'] = mode
                self.request['environment'] = {'MODE': mode}
                _, state = self.run_case()
                self.assertEqual((state['execution'], state['acceptance']), (execution, acceptance))

    def test_unknown_fences_later_occurrence_without_retry(self):
        self.request['environment'] = {'MODE': 'unknown'}
        instance, state = self.run_case()
        self.assertEqual(state['execution'], 'unknown')
        self.assertEqual(state['acceptance'], 'unconfirmed')
        p.submit(instance)
        self.request['id'] = 'next-day'; self.request['environment'] = {}
        later = self.admit(); p.submit(later)
        self.tend('work', expected=1)
        self.assertFalse((later / 'run').exists())
        self.assertEqual((instance / 'run/effect').read_text(), 'external effect fixture')
        self.tend('check')

    def test_early_admission_does_not_execute_early(self):
        self.request.update(not_before='2099-01-01T00:00:00Z', due_at='2099-01-02T00:00:00Z')
        instance = self.admit(); p.submit(instance); self.tend('work', expected=1)
        self.assertFalse((instance / 'run').exists())

    def test_snapshot_supporting_files_and_stale_admitted_input(self):
        support = self.base / 'source.txt'; support.write_text('supplied source')
        self.request['supporting_files'] = {'materials/source.txt': str(support)}
        instance = self.admit()
        self.input.write_text('Later source version')
        self.assertEqual((instance / 'inputs/brief').read_text(), 'Fresh selected input.\n')
        self.assertEqual((instance / 'inputs/materials/source.txt').read_text(), 'supplied source')
        (instance / 'inputs/brief').write_text('tampered')
        with self.assertRaisesRegex(ValueError, 'input changed'): p.submit(instance)

    def test_source_drift_after_submission_refuses_execution(self):
        instance = self.admit(); p.submit(instance)
        with (self.expert / 'bin/run').open('a') as out: out.write('\n# drift\n')
        self.tend('work')
        self.assertFalse((instance / 'run').exists())
        self.assertEqual(json.loads(self.tend('show', instance.name).stdout)['status'], 'unknown')

    def test_output_tampering_and_missing_receipt_do_not_pass(self):
        instance, _ = self.run_case()
        (instance / 'run/result.txt').write_text('wrong output')
        self.assertEqual(p.status(instance, '2035-01-01T00:00:00Z')['acceptance'], 'stale')
        for file in (instance / 'control').glob('receipt-*'): file.unlink()
        self.assertEqual(p.status(instance, '2035-01-01T00:00:00Z')['acceptance'], 'unconfirmed')

    def test_inspection_before_submission_has_no_queue_side_effect(self):
        instance = self.admit()
        state = p.status(instance, '2035-01-01T00:00:00Z')
        self.assertEqual(state['execution'], 'unsubmitted')
        self.assertFalse((self.root / 'tend').exists())

    def test_milestone_evidence_is_attributed_not_team_completion(self):
        self.request['milestones'] = [{'id':'review','owner':'Reviewer','due_at':'2001-01-01T00:00:00Z',
                                       'expectation':'Independent review available.'}]
        instance = self.admit()
        document = self.base / 'review.md'; document.write_text('selected review evidence')
        supplied = [{'id':'review','by':'Reviewer','at':'2002-01-01T00:00:00Z',
                     'artifact':str(document),'sha256':p.sha(document.read_bytes())}]
        state = p.status(instance, '2035-01-01T00:00:00Z', supplied)
        self.assertEqual(state['milestones'][0]['state'], 'reported-met')
        self.assertEqual(state['milestones'][0]['timeliness'], 'late')
        self.assertEqual(state['acceptance'], 'unconfirmed')
        supplied[0]['by'] = 'Someone else'
        self.assertEqual(p.status(instance, '2035-01-01T00:00:00Z', supplied)['acceptance'], 'unverified')

    def test_running_overdue_is_not_failed(self):
        self.request['environment'] = {'MODE': 'wait'}
        self.request['due_at'] = '2001-01-01T00:00:00Z'
        instance = self.admit(); p.submit(instance)
        child = subprocess.Popen([TEND, 'work'], env=dict(os.environ,TEND_ROOT=str(self.root/'tend')),
                                 stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            until = time.monotonic() + 8
            while not (instance/'run/started').exists() and time.monotonic() < until:
                time.sleep(.02)
            self.assertTrue((instance/'run/started').exists())
            state = p.status(instance, '2035-01-01T00:00:00Z')
            self.assertEqual((state['execution'],state['timeliness']),('running','overdue'))
            self.assertFalse(state['escalation']['sent'])
            (instance/'run/release').touch()
            stdout, stderr = child.communicate(timeout=8)
            self.assertEqual(child.returncode,0,stderr.decode())
        finally:
            if child.poll() is None: child.kill(); child.communicate()

    def test_new_run_team_cannot_be_blindly_woken(self):
        self.request['environment'] = {'MODE': 'needs-input'}
        instance, _ = self.run_case()
        self.tend('signal', instance.name, 'input')
        self.tend('work')
        state = p.status(instance, '2035-01-01T00:00:00Z')
        self.assertEqual(state['execution'], 'unknown')
        self.assertEqual(state['acceptance'], 'unconfirmed')

    def test_calendar_binding_rejects_changed_due_date(self):
        c = calendar(); path = self.base/'calendar.json'; path.write_bytes(p.encode(c))
        proposal = p.plan(definition(), c, '2026-03-09T14:00:00Z')[-1]
        for key in ('id','not_before','due_at','timezone','occurrence'): self.request[key] = proposal[key]
        self.request['calendar'] = str(path)
        instance = self.admit()
        self.assertTrue(instance.is_dir())
        self.request['due_at'] = '2026-03-10T00:00:00Z'
        with self.assertRaisesRegex(ValueError, 'calendar occurrence'): self.admit()

    def test_additional_acceptance_is_enforced(self):
        checker = self.base/'extra-check'
        checker.write_text('#!/bin/sh\nexit 1\n'); checker.chmod(0o755)
        self.request['additional_check'] = {'argv':[str(checker),'{run}','{commitment}'],'files':[]}
        _, state = self.run_case()
        self.assertEqual(state['acceptance'],'needs-revision')

    def test_signaled_team_or_verifier_retains_unknown_fence(self):
        for mode in ('signal','signal-check'):
            self.request['id'] = mode; self.request['namespace'] = mode
            self.request['environment'] = {'MODE':mode}
            _, state = self.run_case()
            self.assertEqual(state['execution'], 'unknown', state)
            self.assertEqual(state['acceptance'], 'unconfirmed', state)

    def test_tend_stream_and_receipt_tampering_is_visible(self):
        instance, _ = self.run_case()
        stream = self.root/'tend/jobs'/instance.name/'attempts/001.out'
        stream.write_text('changed execution evidence')
        state = p.status(instance,'2035-01-01T00:00:00Z')
        self.assertEqual(state['acceptance'],'unverified')
        self.assertIn('stream changed',state['error'])
        stream.write_text('TEAM STDOUT\n')
        receipt = next((instance/'control').glob('receipt-*.json'))
        value = p.load(receipt); value['accepted_at'] = '2001-01-01T00:00:00Z'
        receipt.write_bytes(p.encode(value))
        state = p.status(instance,'2035-01-01T00:00:00Z')
        self.assertEqual(state['acceptance'],'unverified')
        self.assertIn('sealed evidence',state['error'])

    def test_missing_artifact_or_changed_source_stays_visible(self):
        instance, _ = self.run_case()
        (instance/'run/result.txt').unlink()
        self.assertEqual(p.status(instance,'2035-01-01T00:00:00Z')['acceptance'],'stale')
        (self.expert/'process.json').write_text('{}')
        state = p.status(instance,'2035-01-01T00:00:00Z')
        self.assertEqual(state['acceptance'],'unverified')
        self.assertEqual(state['id'],self.request['id'])
        self.assertIn('changed exported source',state['error'])

    def test_undeclared_environment_is_not_inherited(self):
        self.request['environment'] = {'MODE':'ambient'}
        instance = self.admit(); p.submit(instance)
        result = subprocess.run([TEND,'work'], env=dict(os.environ,TEND_ROOT=str(self.root/'tend'),
             TEND_PASS='UNDECLARED_SETTING',UNDECLARED_SETTING='must not reach team'),capture_output=True)
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(p.status(instance,'2035-01-01T00:00:00Z')['acceptance'],'accepted')

    def test_virtualenv_invocation_path_and_symlink_binding(self):
        venv = self.base/'venv'
        subprocess.run([sys.executable,'-m','venv','--without-pip',str(venv)],check=True,capture_output=True)
        selected = venv/'bin/python'
        self.d['configuration']['VENV_PYTHON']='program'
        (self.expert/'bin/run').write_text('''#!/usr/bin/env python3
import os,pathlib,subprocess,sys
prefix=subprocess.check_output([os.environ['VENV_PYTHON'],'-c','import sys; print(sys.prefix)']).decode().strip()
assert prefix == str(pathlib.Path(os.environ['VENV_PYTHON']).parent.parent)
source,root=map(pathlib.Path,sys.argv[1:]);root.mkdir();(root/'result.txt').write_bytes(source.read_bytes())
''')
        self.seal_bundle(); self.request['environment']={'VENV_PYTHON':str(selected)}
        _,state=self.run_case()
        self.assertEqual(state['acceptance'],'accepted',state)

    def test_external_acceptance_module_is_pinned(self):
        module = self.base/'review.mjs';module.write_text('export default true;')
        self.d['configuration']['REVIEW_MODULE']='file';self.seal_bundle()
        self.request['environment']={'REVIEW_MODULE':str(module)}
        instance = self.admit();module.write_text('export default false;')
        with self.assertRaisesRegex(ValueError,'check source changed'):p.submit(instance)


if __name__ == '__main__':
    unittest.main(verbosity=2)
