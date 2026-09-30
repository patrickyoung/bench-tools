"""Real Tend plus May's implementation with test-only state isolation; no effects."""
import os
import http.client
from pathlib import Path
import pty
import select
import shutil
import signal
import subprocess
import sys
import time
import tempfile
import threading
import unittest
from unittest.mock import patch

import test_kanban as kanban_tests
import test_process as fixtures

p = fixtures.p
TEND = fixtures.TEND
MAY_SOURCE = Path(os.environ.get('PROCESS_TEST_MAY_SOURCE',fixtures.ROOT/'tools/may'))


@unittest.skipUnless(TEND and MAY_SOURCE.is_dir() and shutil.which('go'), 'select Tend, May source and Go for isolated composition')
class HumanIntegrationTests(unittest.TestCase):
    seal_bundle = fixtures.TendIntegrationTests.seal_bundle
    tend = fixtures.TendIntegrationTests.tend
    board = kanban_tests.KanbanTests.board
    human = kanban_tests.KanbanTests.human

    @classmethod
    def setUpClass(cls):
        # Production May intentionally selects the OS user's home, ignoring HOME.
        # Build a test main around its existing injectable app.stateDir seam;
        # never exercise synthetic decisions against the operator's real state.
        cls.fixture_dir = tempfile.TemporaryDirectory()
        cls.addClassCleanup(cls.fixture_dir.cleanup)
        source = Path(cls.fixture_dir.name)
        for original in MAY_SOURCE.iterdir():
            if original.suffix == '.go' or original.name in ('go.mod','go.sum'):
                shutil.copy2(original,source/original.name)
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
        kanban_tests.KanbanTests.setUp(self)
        self.home = self.base/'home'; self.home.mkdir()
        env = patch.dict(os.environ, HOME=str(self.home))
        env.start(); self.addCleanup(env.stop)
        self.activity['state'] = 'ready'
        self.activity_record = p.record_activity(self.root,self.activity)
        self.workdir = self.base/'human-work'; self.workdir.mkdir()
        self.action = self.workdir/'action'; self.action.write_bytes(b'Record this synthetic test decision.\n')
        self.answer = self.base/'answer'; self.answer.write_bytes(b'Use the blue mockup.\n')
        self.mailbox = self.workdir/'response'
        self.response = {'schema':'bench.human-response/v1','namespace':'test-team','id':self.activity['id'],
                         'by':'Fixture operator','reason':'Synthetic response for executable verification'}

    def wait(self, kind='input'):
        worker = self.workdir/'worker.py'
        worker.write_text('''import os, pathlib, subprocess, sys
kind, may = sys.argv[1:]
if kind == 'approval':
    result = subprocess.run([may, 'request', 'synthetic-may-job'], input=pathlib.Path('action').read_bytes())
    if result.returncode != 75: sys.exit(result.returncode)
else:
    if pathlib.Path('response').exists():
        print(pathlib.Path('response').read_text())
        sys.exit(0)
subprocess.run([os.environ['TEND'], 'defer', 'signal', 'human-request-1'], check=True)
sys.exit(75)
''')
        self.job = self.tend('submit','-id','human-fixture','-C',str(self.workdir),'--',sys.executable,
                             str(worker),kind,self.may).stdout.decode().strip()
        self.tend('work')
        self.link = {'schema':'bench.human-link/v1','namespace':'test-team','id':self.activity['id'],
            'activity_revision':self.activity_record['revision'],'kind':kind,'tend':TEND,
            'queue':str(self.root/'tend'),'job':self.job,'signal':'human-request-1',
            'by':'Fixture operator','reason':'Explicitly bind a synthetic human request'}
        if kind == 'approval': self.link.update(may=self.may,may_job='synthetic-may-job',action=str(self.action))
        else:
            self.link['response_path'] = str(self.mailbox)
            self.response['input'] = str(self.answer)
        return p.link_human(self.root,self.link)

    def view(self):
        return p.human_link_view(self.root,'test-team',self.activity['id'])

    def terminal_response(self, answer):
        """Test-only PTY: answers apply solely to May in this temporary HOME."""
        request = self.base/'response.json'; request.write_bytes(p.encode(self.response))
        pid, fd = pty.fork()
        if pid == 0:
            os.execv(sys.executable,[sys.executable,str(fixtures.APP),'respond-human',str(self.root),str(request)])
        output = b''; sent = False; status = None
        try:
            deadline = time.monotonic()+15
            while time.monotonic() < deadline:
                readable, _, _ = select.select([fd],[],[],.1)
                if readable:
                    try: chunk = os.read(fd,65536)
                    except OSError: break
                    if not chunk: break
                    output += chunk
                    if b'[y/N]' in output and not sent:
                        os.write(fd,answer+b'\n'); sent = True
                finished, status = os.waitpid(pid,os.WNOHANG)
                if finished: break
            else: self.fail('synthetic terminal decision timed out')
        finally:
            os.close(fd)
            try:
                finished, polled = os.waitpid(pid,os.WNOHANG)
                if finished: status = polled
                else:
                    os.kill(pid,signal.SIGKILL); _, status = os.waitpid(pid,0)
            except ChildProcessError: pass
        self.assertTrue(sent,output.decode(errors='replace'))
        self.assertEqual(status,0,output.decode(errors='replace'))

    def test_input_response_wakes_once_and_does_not_complete_parent(self):
        binding = self.wait()
        self.assertEqual(p.link_human(self.root,self.link),binding)
        self.assertEqual(self.human(self.board())['coordination']['state'],'awaiting-input')
        self.assertEqual(self.human(self.board())['column'],'Needs attention')
        view = p.respond_human(self.root,self.response)
        self.assertEqual(view['state'],'wake-recorded')
        self.assertEqual(self.mailbox.read_bytes(),self.answer.read_bytes())
        p.respond_human(self.root,self.response)
        self.assertEqual(len(self.tend('signals',self.job).stdout.splitlines()),1)
        self.tend('work')
        self.assertEqual(self.view()['state'],'job-done')
        parent = next(c for c in self.board()['cards'] if c['kind']=='commitment' and c['id']==self.activity['parent'])
        self.assertEqual(parent['acceptance'],'unconfirmed')
        self.assertEqual(self.human(self.board())['acceptance'],'unconfirmed')

    def test_may_approval_is_spent_only_by_resumed_worker(self):
        binding = self.wait('approval')
        self.assertEqual(self.view()['state'],'awaiting-approval')
        # Reading and editing a card never grants or consumes approval.
        self.board(); self.board()
        self.assertTrue(p.human_pending(binding))
        self.terminal_response(b'y')
        self.assertEqual(self.view()['state'],'wake-recorded')
        may_root = self.home/'.local/state/may'
        self.assertTrue((may_root/'granted'/(binding['digest']+'.json')).exists())
        self.tend('work')
        self.assertEqual(self.view()['state'],'job-done')
        self.assertFalse((may_root/'granted'/(binding['digest']+'.json')).exists())
        self.assertTrue(list((may_root/'spent').glob(binding['digest']+'*')))
        p.respond_human(self.root,self.response)
        self.assertEqual(len(self.tend('signals',self.job).stdout.splitlines()),1)

    def test_may_refusal_resumes_to_failed_not_approved(self):
        self.wait('approval')
        self.terminal_response(b'n')
        self.tend('work')
        self.assertEqual(self.view()['state'],'job-failed')
        self.assertEqual(self.human(self.board())['column'],'Needs attention')

    def test_no_terminal_leaves_pending_and_sends_no_signal(self):
        self.wait('approval')
        request = self.base/'request.json'; request.write_bytes(p.encode(self.response))
        result = subprocess.run([sys.executable,str(fixtures.APP),'respond-human',str(self.root),str(request)],
                                input=b'y\n',capture_output=True,start_new_session=True,timeout=10)
        self.assertEqual(result.returncode,2)
        self.assertEqual(self.view()['state'],'awaiting-approval')
        self.assertEqual(self.tend('signals',self.job).stdout,b'')

    def test_changed_action_cannot_spend_old_approval(self):
        self.wait('approval'); self.terminal_response(b'y')
        self.action.write_bytes(b'Changed action.\n')
        self.tend('work')
        self.assertEqual(self.view()['state'],'waiting-again')
        self.assertEqual(self.human(self.board())['column'],'Needs attention')
        p.respond_human(self.root,self.response)
        self.assertEqual(len(self.tend('signals',self.job).stdout.splitlines()),1)

    def test_lost_signal_reply_recovers_without_duplicate_wakeup(self):
        self.wait()
        original = p.tend_call
        def lost(binding,args,stdin=b''):
            result = original(binding,args,stdin)
            if args[0] == 'signal': raise OSError('simulated lost reply')
            return result
        with patch.object(p,'tend_call',side_effect=lost):
            with self.assertRaisesRegex(OSError,'lost reply'): p.respond_human(self.root,self.response)
        self.tend('work')
        self.answer.unlink()
        self.assertEqual(p.respond_human(self.root,self.response)['state'],'job-done')
        self.assertEqual(len(self.tend('signals',self.job).stdout.splitlines()),1)
        self.answer.write_bytes(b'Different response')
        with self.assertRaisesRegex(ValueError,'different bytes'): p.respond_human(self.root,self.response)

    def test_conflicting_mailbox_and_retained_input_corruption_fail_closed(self):
        self.wait(); self.mailbox.write_bytes(b'Somebody else answered')
        with self.assertRaisesRegex(ValueError,'conflicting'): p.respond_human(self.root,self.response)
        self.assertEqual(self.tend('signals',self.job).stdout,b'')
        retained = self.root/'human-links'/p.tracking_key('test-team',self.activity['id'])/'input'
        retained.write_bytes(b'corruption')
        self.assertEqual(self.human(self.board())['execution'],'unverified')
        with self.assertRaisesRegex(ValueError,'incomplete'): p.reconcile(self.root)

    def test_changed_wait_or_cancelled_job_cannot_be_woken(self):
        self.wait(); self.tend('cancel',self.job)
        with self.assertRaisesRegex(ValueError,'wait changed'): p.respond_human(self.root,self.response)
        self.assertEqual(self.tend('signals',self.job).stdout,b'')
        self.assertFalse(self.mailbox.exists())

    def test_binding_rejects_wrong_wait_action_and_revision(self):
        self.wait('approval')
        with self.assertRaisesRegex(ValueError,'immutable'):
            p.link_human(self.root,{**self.link,'signal':'another-wait'})
        # New activity cannot reuse the same job wait or point to another action.
        second = {**self.activity,'id':'another-approval'}
        record = p.record_activity(self.root,second)
        with self.assertRaisesRegex(ValueError,'already linked'):
            p.link_human(self.root,{**self.link,'id':second['id'],'activity_revision':record['revision']})

    def test_done_card_does_not_wake_or_hide_pending_approval(self):
        self.wait('approval')
        p.record_activity(self.root,{**self.activity,'state':'done','previous':self.activity_record['revision'],
            'evidence':[{'path':str(self.answer),'sha256':p.sha(self.answer.read_bytes())}]})
        item = self.human(self.board())
        self.assertEqual(item['column'],'Needs attention')
        self.assertEqual(item['coordination']['state'],'awaiting-approval')
        self.assertEqual(self.tend('signals',self.job).stdout,b'')
        page = p.render_kanban(self.board())
        self.assertIn('Human handoff: awaiting-approval',page)
        self.assertIn('Wakeup is not approval',page)

    def test_unknown_job_is_visible_and_response_cannot_retry_it(self):
        self.wait()
        self.tend('signal',self.job,'human-request-1')
        (self.workdir/'worker.py').write_text('import sys; sys.exit(125)\n')
        self.tend('work')
        self.assertEqual(self.view()['state'],'job-unknown')
        with self.assertRaisesRegex(ValueError,'wait changed'): p.respond_human(self.root,self.response)
        self.assertEqual(len(self.tend('signals',self.job).stdout.splitlines()),1)
        self.assertEqual(self.human(self.board())['column'],'Needs attention')

    def test_wait_changed_during_decision_is_rechecked_before_signal(self):
        self.wait('approval')
        calls = []
        def pending(binding):
            calls.append(1)
            if len(calls) == 2: self.tend('cancel',self.job)
            return len(calls) == 1
        with patch.object(p,'human_pending',side_effect=pending):
            with self.assertRaisesRegex(ValueError,'changed before wakeup'): p.respond_human(self.root,self.response)
        self.assertEqual(self.tend('signals',self.job).stdout,b'')

    def test_prepared_input_survives_source_removal_before_signal(self):
        self.wait()
        original = p.tend_call
        def unavailable(binding,args,stdin=b''):
            if args[0] == 'signal': raise OSError('temporarily unavailable')
            return original(binding,args,stdin)
        with patch.object(p,'tend_call',side_effect=unavailable):
            with self.assertRaises(OSError): p.respond_human(self.root,self.response)
        self.answer.unlink()
        self.assertEqual(p.respond_human(self.root,self.response)['state'],'wake-recorded')

    def test_mailbox_symlink_drift_is_refused(self):
        self.wait()
        target = self.base/'unselected'; target.write_bytes(b'Untouched')
        self.mailbox.symlink_to(target)
        with self.assertRaisesRegex(ValueError,'symlink'): p.respond_human(self.root,self.response)
        self.assertEqual(target.read_bytes(),b'Untouched')
        self.assertEqual(self.tend('signals',self.job).stdout,b'')

    def test_live_board_observes_response_and_job_completion(self):
        self.wait()
        with p.kanban_server(self.root,0,1) as server:
            thread = threading.Thread(target=server.serve_forever,daemon=True); thread.start()
            try:
                def page():
                    connection = http.client.HTTPConnection('127.0.0.1',server.server_port,timeout=10)
                    connection.request('GET','/view'); response = connection.getresponse()
                    self.assertEqual(response.status,200)
                    raw = response.read().decode(); connection.close(); return raw
                self.assertIn('awaiting-input',page())
                p.respond_human(self.root,self.response); self.tend('work')
                time.sleep(1.05)
                self.assertIn('job-done',page())
            finally:
                server.shutdown(); thread.join(timeout=5)
