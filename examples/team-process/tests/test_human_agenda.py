"""Agenda + Tend + isolated May compose through their public executables."""
import importlib.util
import json
import os
from pathlib import Path
import pty
import select
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

APP = Path(__file__).resolve().parents[1]
ROOT = APP.parents[1]
sys.path.insert(0, str(APP))
import human as h

TEND = os.environ.get('PROCESS_TEST_TEND')
AGENDA = os.environ.get('PROCESS_TEST_AGENDA')
MAY_SOURCE = Path(os.environ.get('PROCESS_TEST_MAY_SOURCE', ROOT/'tools/may'))


@unittest.skipUnless(TEND and AGENDA and MAY_SOURCE.is_dir() and shutil.which('go'),
                     'select real Agenda, Tend and May source for executable composition')
class AgendaHumanTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.fixture = tempfile.TemporaryDirectory()
        cls.addClassCleanup(cls.fixture.cleanup)
        source = Path(cls.fixture.name)
        for original in MAY_SOURCE.iterdir():
            if original.suffix == '.go' or original.name in ('go.mod','go.sum'):
                shutil.copy2(original,source/original.name)
        # Only this fixture overrides May's injectable stateDir. Synthetic
        # terminal answers can never reach the operator's production May store.
        (source/'hil_fixture_test.go').write_text('''package main
import ("os"; "path/filepath"; "testing")
func TestMain(m *testing.M) {
 a := newApp()
 a.stateDir = filepath.Join(os.Getenv("HOME"), ".local", "state", "may")
 os.Exit(a.run(os.Args[1:]))
}
''')
        cls.may = str(source/'may-isolated-test')
        subprocess.run(['go','test','-c','-o',cls.may,'.'],cwd=source,check=True,capture_output=True,timeout=90)

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name).resolve()
        self.root = self.base/'handoffs'
        self.agenda_root = self.base/'agenda'
        self.queue = self.base/'queue'
        self.work = self.base/'work'; self.work.mkdir()
        self.home = self.base/'home'; self.home.mkdir()
        selected = patch.dict(os.environ,HOME=str(self.home))
        selected.start(); self.addCleanup(selected.stop)
        self.value = {'title':'Review a mockup','owner':'Fixture operator','basis':'human',
                      'not_before':'2026-09-01T09:00:00Z','due_at':'2035-01-01T17:00:00Z','timezone':'UTC'}
        self.item = self.apply('item','review',self.value)
        self.action = self.work/'action'; self.action.write_bytes(b'Record a synthetic decision.\n')
        self.answer = self.base/'answer'; self.answer.write_bytes(b'Use the blue mockup.\n')
        self.mailbox = self.work/'response'
        self.response = {'schema':'bench.agenda-human-response/v1','id':'review',
                         'by':'Fixture operator','reason':'Synthetic response'}

    def apply(self, kind, identifier, value, previous=None, evidence=None):
        change = {'schema':'agenda.change/v1','kind':kind,'id':identifier,'previous':previous,
                  'by':'Fixture operator','reason':'Synthetic contract fixture','value':value}
        if evidence: change['evidence'] = evidence
        change['request_id'] = h.sha(h.encode(change))
        result = subprocess.run([AGENDA,'apply',str(self.agenda_root)],input=h.encode(change),capture_output=True,timeout=15)
        self.assertEqual(result.returncode,0,result.stderr.decode())
        return json.loads(result.stdout)

    def tend(self, *args):
        result = subprocess.run([TEND,*args],env=dict(os.environ,TEND_ROOT=str(self.queue)),capture_output=True,timeout=15)
        self.assertEqual(result.returncode,0,result.stderr.decode())
        return result

    def wait(self, kind='input'):
        worker = self.work/'worker.py'
        worker.write_text('''import os, pathlib, subprocess, sys
kind, may = sys.argv[1:]
if kind == 'approval':
 result = subprocess.run([may,'request','synthetic-may-job'],input=pathlib.Path('action').read_bytes())
 if result.returncode != 75: sys.exit(result.returncode)
elif pathlib.Path('response').exists():
 print(pathlib.Path('response').read_text()); sys.exit(0)
subprocess.run([os.environ['TEND'],'defer','signal','human-request-1'],check=True)
sys.exit(75)
''')
        job = self.tend('submit','-id','human-fixture','-C',str(self.work),'--',sys.executable,str(worker),kind,self.may).stdout.decode().strip()
        self.tend('work')
        self.link = {'schema':'bench.agenda-human-link/v1','id':'review','item_revision':self.item['revision'],
                     'kind':kind,'tend':TEND,'queue':str(self.queue),'job':job,'signal':'human-request-1',
                     'by':'Fixture operator','reason':'Bind a synthetic request'}
        if kind == 'approval': self.link.update(may=self.may,may_job='synthetic-may-job',action=str(self.action))
        else:
            self.link['response_path'] = str(self.mailbox)
            self.response['input'] = str(self.answer)
        return h.link_human(self.root,self.link,AGENDA,self.agenda_root)

    def view(self):
        return h.human_link_view(self.root,'review',AGENDA,self.agenda_root)

    def respond(self):
        return h.respond_human(self.root,self.response,AGENDA,self.agenda_root)

    def command(self, *args):
        return [sys.executable,str(APP/'human.py'),*map(str,args),'--agenda',AGENDA,'--agenda-root',str(self.agenda_root)]

    def terminal_response(self, answer):
        request = self.base/'respond.json'; request.write_bytes(h.encode(self.response))
        pid, fd = pty.fork()
        if pid == 0: os.execv(sys.executable,self.command('respond',self.root,request))
        output=b''; sent=False; status=None
        try:
            deadline=time.monotonic()+15
            while time.monotonic()<deadline:
                ready,_,_=select.select([fd],[],[],.1)
                if ready:
                    try: chunk=os.read(fd,65536)
                    except OSError: break
                    if not chunk: break
                    output+=chunk
                    if b'[y/N]' in output and not sent: os.write(fd,answer+b'\n'); sent=True
                finished,status=os.waitpid(pid,os.WNOHANG)
                if finished: break
            else: self.fail('synthetic terminal decision timed out')
        finally:
            os.close(fd)
            try:
                finished,polled=os.waitpid(pid,os.WNOHANG)
                if finished: status=polled
                else: os.kill(pid,signal.SIGKILL); _,status=os.waitpid(pid,0)
            except ChildProcessError: pass
        self.assertTrue(sent,output.decode(errors='replace'))
        self.assertEqual(status,0,output.decode(errors='replace'))

    def test_input_is_idempotent_and_never_changes_business_completion(self):
        binding=self.wait()
        self.assertEqual(h.link_human(self.root,self.link,AGENDA,self.agenda_root),binding)
        before=subprocess.check_output([AGENDA,'export',str(self.agenda_root)])
        files={str(f):f.read_bytes() for f in self.root.rglob('*') if f.is_file()}
        self.assertEqual(self.view()['state'],'awaiting-input')
        self.assertEqual(files,{str(f):f.read_bytes() for f in self.root.rglob('*') if f.is_file()})
        self.assertEqual(self.respond()['state'],'wake-recorded')
        self.assertEqual(self.mailbox.read_bytes(),self.answer.read_bytes())
        self.respond(); self.tend('work')
        self.assertEqual(self.view()['state'],'job-done')
        self.assertEqual(len(self.tend('signals',self.link['job']).stdout.splitlines()),1)
        self.assertEqual(before,subprocess.check_output([AGENDA,'export',str(self.agenda_root)]))

    def test_changed_item_revision_refuses_response_and_inspect_remains_read_only(self):
        self.wait()
        self.apply('item','review',{**self.value,'owner':'Another person'},self.item['revision'])
        self.assertFalse(self.view()['item_current'])
        with self.assertRaisesRegex(ValueError,'current human item revision'): self.respond()
        self.assertEqual(self.tend('signals',self.link['job']).stdout,b'')
        self.assertFalse(self.mailbox.exists())

    def test_wrong_agenda_selection_cannot_redirect_a_bound_response(self):
        self.wait()
        with self.assertRaisesRegex(ValueError,'binding changed'):
            h.respond_human(self.root,self.response,AGENDA,self.base/'another-agenda')
        self.assertFalse((self.base/'another-agenda').exists())
        self.assertFalse(self.mailbox.exists())

    def test_lost_reply_recovers_one_signal_with_retained_input(self):
        self.wait(); original=h.tend_call
        def lost(binding,args,stdin=b''):
            result=original(binding,args,stdin)
            if args[0]=='signal': raise OSError('lost reply')
            return result
        with patch.object(h,'tend_call',side_effect=lost):
            with self.assertRaisesRegex(OSError,'lost reply'): self.respond()
        self.answer.unlink(); self.tend('work')
        self.assertEqual(self.respond()['state'],'job-done')
        self.assertEqual(len(self.tend('signals',self.link['job']).stdout.splitlines()),1)

    def test_may_approval_is_only_spent_by_resumed_controller(self):
        binding=self.wait('approval'); self.view(); self.view()
        self.assertTrue(h.human_pending(binding))
        self.assertEqual(self.tend('signals',self.link['job']).stdout,b'')
        self.terminal_response(b'y')
        may_root=self.home/'.local/state/may'
        self.assertTrue((may_root/'granted'/(binding['digest']+'.json')).exists())
        self.tend('work')
        self.assertEqual(self.view()['state'],'job-done')
        self.assertFalse((may_root/'granted'/(binding['digest']+'.json')).exists())
        self.assertTrue(list((may_root/'spent').glob(binding['digest']+'*')))

    def test_no_terminal_cannot_approve_and_done_report_cannot_wake(self):
        binding=self.wait('approval')
        request=self.base/'respond.json'; request.write_bytes(h.encode(self.response))
        result=subprocess.run(self.command('respond',self.root,request),input=b'y\n',capture_output=True,
                              start_new_session=True,timeout=15)
        self.assertEqual(result.returncode,2)
        self.assertTrue(h.human_pending(binding))
        self.assertEqual(self.tend('signals',self.link['job']).stdout,b'')
        self.apply('report','review',{'target':'review','state':'done'},
                   evidence=[{'path':str(self.answer),'sha256':h.sha(self.answer.read_bytes())}])
        self.assertEqual(self.view()['state'],'awaiting-approval')
        with self.assertRaisesRegex(ValueError,'open human item'): self.respond()
        self.assertEqual(self.tend('signals',self.link['job']).stdout,b'')

    def test_decline_is_not_approval(self):
        self.wait('approval'); self.terminal_response(b'n'); self.tend('work')
        self.assertEqual(self.view()['state'],'job-failed')

    def test_changed_action_cannot_spend_old_grant(self):
        self.wait('approval'); self.terminal_response(b'y')
        self.action.write_bytes(b'A different action.\n'); self.tend('work')
        self.assertEqual(self.view()['state'],'waiting-again')
        self.respond()
        self.assertEqual(len(self.tend('signals',self.link['job']).stdout.splitlines()),1)

    def test_wait_can_only_bind_to_one_item(self):
        self.wait()
        second=self.apply('item','another',self.value)
        with self.assertRaisesRegex(ValueError,'already linked'):
            h.link_human(self.root,{**self.link,'id':'another','item_revision':second['revision']},AGENDA,self.agenda_root)

    def test_cancelled_wait_cannot_receive_a_response(self):
        self.wait(); self.tend('cancel',self.link['job'])
        with self.assertRaisesRegex(ValueError,'wait changed'): self.respond()
        self.assertFalse(self.mailbox.exists())
        self.assertEqual(self.tend('signals',self.link['job']).stdout,b'')

    def test_observation_is_bounded_coordination_evidence_never_acceptance(self):
        self.wait()
        row=h.observation(self.root,'review',AGENDA,self.agenda_root)
        self.assertEqual(row['state'],'waiting')
        self.assertEqual(row['revision'],self.item['revision'])
        self.assertEqual((h.instant(row['valid_until'])-h.instant(row['observed_at'])).total_seconds(),60)
        self.assertEqual(row['extensions']['human_coordination']['state'],'awaiting-input')
        self.respond(); self.tend('work')
        self.assertEqual(h.observation(self.root,'review',AGENDA,self.agenda_root)['state'],'unsubmitted')
        self.apply('item','review',{**self.value,'title':'Changed request'},self.item['revision'])
        self.assertEqual(h.observation(self.root,'review',AGENDA,self.agenda_root)['state'],'unverified')

    def test_observation_reports_inspection_failure_without_waking(self):
        self.wait()
        with patch.object(h,'human_job',side_effect=ValueError('selected evidence unavailable')):
            row=h.observation(self.root,'review',AGENDA,self.agenda_root)
        self.assertEqual(row['state'],'unverified')
        self.assertIn('unavailable',row['extensions']['human_coordination']['error'])
        self.assertEqual(self.tend('signals',self.link['job']).stdout,b'')

    def test_public_projection_retains_coordination_without_accepting_human_work(self):
        self.wait()
        row=h.observation(self.root,'review',AGENDA,self.agenda_root)
        snapshot=self.base/'snapshot.json'
        snapshot.write_bytes(subprocess.check_output([AGENDA,'export',str(self.agenda_root)]))
        result=subprocess.run([AGENDA,'project',str(snapshot),'--as-of',h.now()],
                              input=h.encode(row),capture_output=True,timeout=15)
        self.assertEqual(result.returncode,0,result.stderr.decode())
        obligation=json.loads(result.stdout)['obligations'][0]
        self.assertIn('execution-waiting',obligation['attention'])
        self.assertNotEqual(obligation['acceptance'],'accepted')
        self.assertEqual(obligation['observation']['extensions']['human_coordination']['state'],'awaiting-input')


if __name__=='__main__': unittest.main()
