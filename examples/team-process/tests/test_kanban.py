"""Kanban views, human activity authority, and retained assignment observations."""
from concurrent.futures import ThreadPoolExecutor
import http.client
import threading
from pathlib import Path
import subprocess
import sys
import unittest
from unittest.mock import patch

import test_calendar_tracking as calendar_tests
import test_process as fixtures

p = fixtures.p
TEND = fixtures.TEND


class KanbanTests(unittest.TestCase):
    seal_bundle = fixtures.TendIntegrationTests.seal_bundle
    tend = fixtures.TendIntegrationTests.tend
    admit_occurrence = calendar_tests.CalendarTrackingTests.admit_occurrence
    disposition = calendar_tests.CalendarTrackingTests.disposition

    def setUp(self):
        calendar_tests.CalendarTrackingTests.setUp(self)
        self.activity = {'schema':'bench.human-activity/v1','namespace':'test-team','id':'stakeholder-approval',
            'parent':'daily-review/2026-03-06','title':'Obtain stakeholder approval',
            'description':'Retain the named stakeholder decision.','assignee':'Delivery lead',
            'due_at':'2026-03-09T14:00:00Z','timezone':'UTC','state':'planned','by':'Case owner',
            'reason':'Assign approval follow-up','evidence':[]}

    def board(self, **kwargs):
        return p.kanban_view(self.root,'2026-03-09T15:00:00Z',**kwargs)

    def human(self, report):
        return next(c for c in report['cards'] if c['kind']=='human-activity')

    def test_missing_and_upcoming_obligations_reuse_calendar_identity(self):
        board = self.board()
        calendar = p.calendar_view(self.root,board['as_of'])
        self.assertEqual({(r['namespace'],r['id'],r['due_at']) for r in board['cards']},
                         {(r['namespace'],r['id'],r['due_at']) for r in calendar['obligations']})
        self.assertEqual(next(c for c in board['cards'] if c['id'].endswith('03-06'))['column'],'Needs attention')
        self.assertEqual(next(c for c in board['cards'] if c['id'].endswith('03-10'))['column'],'Planned')
        self.assertTrue(all(not c['editable'] for c in board['cards']))
        self.assertFalse((self.root/'tend').exists())

    def test_human_updates_are_audited_and_overdue_does_not_change_stage(self):
        first = p.record_activity(self.root,self.activity)
        self.assertEqual(p.record_activity(self.root,self.activity),first)
        active = {**self.activity,'state':'in-progress','assignee':'New owner',
                  'by':'Case owner','reason':'Reassigned for follow-up','previous':first['revision']}
        second = p.record_activity(self.root,active)
        item = self.human(self.board())
        self.assertEqual((item['column'],item['owner'],item['timeliness']),('In progress','New owner','overdue'))
        self.assertEqual(len(item['activity_history']),2)
        self.assertEqual(second['previous'],first['revision'])
        with self.assertRaisesRegex(ValueError,'current revision'):
            p.record_activity(self.root,{**active,'state':'ready','reason':'Stale update'})
        p.reconcile(self.root)
        p.record_activity(self.root,{**active,'state':'needs-attention','reason':'Awaiting decision','previous':second['revision']})
        self.assertEqual(self.human(self.board())['column'],'Needs attention')
        self.assertEqual(self.board()['monitor']['state'],'changed')

    def test_human_done_requires_retained_evidence_and_never_completes_parent(self):
        with self.assertRaisesRegex(ValueError,'requires selected evidence'):
            p.record_activity(self.root,{**self.activity,'state':'done'})
        artifact = self.base/'approval.txt'; artifact.write_text('Approval supplied by the selected human.\n')
        done = {**self.activity,'state':'done','reason':'Approval received',
                'evidence':[{'path':str(artifact),'sha256':p.sha(artifact.read_bytes())}]}
        record = p.record_activity(self.root,done)
        artifact.unlink()
        self.assertEqual(p.record_activity(self.root,done),record)
        report = self.board(); item = self.human(report)
        self.assertEqual((item['column'],item['acceptance'],item['timeliness']),('Done','reported-done','late'))
        parent = next(c for c in report['cards'] if c['id']==self.activity['parent'] and c['kind']=='commitment')
        self.assertEqual((parent['column'],parent['acceptance']),('Needs attention','unconfirmed'))
        retained = self.root/'activities'/p.tracking_key('test-team',self.activity['id'])/'evidence'/done['evidence'][0]['sha256']
        retained.write_text('changed')
        self.assertEqual(self.human(self.board())['column'],'Needs attention')
        self.assertEqual(self.board()['monitor']['state'],'unverified')
        with self.assertRaisesRegex(ValueError,'incomplete'): p.reconcile(self.root)

    def test_reopen_and_cancel_preserve_history_and_evidence(self):
        first=p.record_activity(self.root,self.activity)
        cancelled={**self.activity,'state':'cancelled','reason':'No longer required','previous':first['revision']}
        second=p.record_activity(self.root,cancelled)
        self.assertTrue(any(c['kind']=='human-activity' for c in self.board()['history']))
        p.record_activity(self.root,{**self.activity,'state':'ready','reason':'Reopened by owner','previous':second['revision']})
        self.assertEqual(self.human(self.board())['column'],'Ready')
        self.assertEqual(len(self.human(self.board())['activity_history']),3)

    def test_parent_and_record_type_cannot_be_changed_to_override_agent_work(self):
        for changes in ({'parent':'absent'}, {'schema':'bench.commitment/v1'}, {'state':'accepted'}, {'evidence':[{'path':'relative','sha256':'0'*64}]}):
            with self.assertRaises(ValueError): p.record_activity(self.root,{**self.activity,**changes})
        first=p.record_activity(self.root,self.activity)
        with self.assertRaisesRegex(ValueError,'parent cannot change'):
            p.record_activity(self.root,{**self.activity,'parent':'daily-review/2026-03-09',
                'reason':'Move existing work','previous':first['revision']})

    def test_concurrent_activity_updates_have_one_winner(self):
        first=p.record_activity(self.root,self.activity)
        def change(owner):
            try:
                p.record_activity(self.root,{**self.activity,'assignee':owner,'reason':'Reassign','previous':first['revision']})
                return 'saved'
            except ValueError: return 'conflict'
        with ThreadPoolExecutor(max_workers=2) as pool:
            results=list(pool.map(change,('Alice','Bob')))
        self.assertEqual(sorted(results),['conflict','saved'])
        self.assertEqual(len(self.human(self.board())['activity_history']),2)

    def test_closed_parent_does_not_silently_close_human_activity(self):
        p.record_activity(self.root,self.activity)
        p.record_disposition(self.root,self.disposition())
        item=self.human(self.board())
        self.assertEqual(item['column'],'Needs attention')
        self.assertIn('activity-parent-closed',item['attention'])

    def test_calendar_and_ics_include_human_due_dates_with_distinct_identity(self):
        p.record_activity(self.root,{**self.activity,'id':self.activity['parent']})
        report=p.calendar_view(self.root,'2026-03-09T15:00:00Z')
        events=p.calendar_events(report)
        human=next(e for e in events if e['identity'][0]=='human-activity')
        self.assertEqual(human['start'],human['due'])
        identities=[p.sha(p.encode(e['identity'])) for e in events]
        self.assertEqual(len(identities),len(set(identities)))
        board=self.board()
        self.assertEqual(len({c['card_id'] for c in board['cards']}),len(board['cards']))

    def test_malformed_activity_remains_visible_and_does_not_refresh_monitor(self):
        p.record_activity(self.root,self.activity); p.reconcile(self.root)
        checkpoint=(self.root/'last-reconciliation.json').read_bytes()
        target=next((self.root/'activities').glob('*/*.json'));target.write_text('{bad')
        report=self.board()
        self.assertEqual(report['monitor']['state'],'unverified')
        self.assertIn('activity-unverified',{e['reason'] for e in report['errors']})
        with self.assertRaisesRegex(ValueError,'incomplete'):p.reconcile(self.root)
        self.assertEqual((self.root/'last-reconciliation.json').read_bytes(),checkpoint)

    def test_html_escapes_titles_and_exposes_reported_completion(self):
        p.record_activity(self.root,{**self.activity,'title':'<img src=x onerror=alert(1)>','assignee':'<script>owner</script>'})
        output=p.render_kanban(self.board())
        self.assertNotIn('<img src=x',output)
        self.assertIn('&lt;img src=x',output)
        self.assertIn('Human activity · reported status',output)
        self.assertIn('Team commitment · read-only',output)
        for label in p.KANBAN_COLUMNS:self.assertIn(label,output)
        self.assertNotIn('draggable=',output)

    def test_public_cli_kanban_is_read_only(self):
        before=p.tracking_digest(self.root)
        result=subprocess.run([sys.executable,str(fixtures.APP),'kanban',str(self.root),
            '--as-of','2026-03-09T15:00:00Z','--format','html'],capture_output=True,text=True)
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertIn('<title>Team work board</title>',result.stdout)
        self.assertEqual(p.tracking_digest(self.root),before)
        self.assertFalse((self.root/'tend').exists())

    @unittest.skipUnless(TEND,'requires real Tend')
    def test_delivery_done_still_requires_verified_acceptance(self):
        instance=self.admit_occurrence()
        report=self.board(); card=next(c for c in report['cards'] if c['id']==self.request['id'])
        self.assertEqual(card['column'],'Planned')
        p.submit(instance)
        self.assertEqual(next(c for c in self.board()['cards'] if c['id']==self.request['id'])['column'],'Ready')
        self.tend('work')
        report=p.kanban_view(self.root,'2035-01-01T00:00:00Z')
        card=next(c for c in report['cards'] if c['id']==self.request['id'])
        self.assertEqual((card['column'],card['timeliness']),('Done','late'))
        (instance/'run/result.txt').unlink()
        card=next(c for c in p.kanban_view(self.root,'2035-01-01T00:00:00Z')['cards'] if c['id']==self.request['id'])
        self.assertEqual(card['column'],'Needs attention')

    @unittest.skipUnless(TEND,'requires real Tend')
    def test_unknown_and_cancellation_conflict_stay_on_active_board(self):
        instance=self.admit_occurrence(environment={'MODE':'unknown'})
        p.submit(instance);self.tend('work')
        p.record_disposition(self.root,self.disposition('cancelled'))
        card=next(c for c in self.board()['cards'] if c['id']==self.request['id'])
        self.assertEqual(card['column'],'Needs attention')
        self.assertEqual(card['execution'],'unknown')

    @unittest.skipUnless(TEND,'requires real Tend')
    def test_page_assignments_are_explicit_bound_read_only_snapshots(self):
        self.d['team']='page-team';self.seal_bundle()
        lock=p.load(self.export/'team.lock.json');lock['team']='page-team'
        (self.export/'team.lock.json').write_bytes(p.encode(lock))
        self.request['id']='page-case'
        instance=p.admit(self.export,self.request,self.root,TEND)
        (instance/'run').mkdir()
        path=instance/'run/snapshot.json'
        manifest=p.encode({'schema':'bench.manage.run/v1','goal':self.input.read_text().rstrip('\n'),'run':'fixture-one'})
        (instance/'run/manifest.json').write_bytes(manifest)
        snapshot={'schema':'bench.manage.snapshot/v1','run_sha256':'sha256:'+p.sha(manifest),
            'as_of':p.instant(p.now()).timestamp(),'goal':self.input.read_text().rstrip('\n'),'tasks':[
            {'id':'review','input':{'worker':'manager','goal':'Review complete artifact'},'state':'accepted',
             'needs':[],'receipt':{'check_exit':0},'job':'bench-manage-000001'}]}
        path.write_bytes(p.encode(snapshot))
        report=self.board(assignments={instance.name:str(path)})
        card=next(c for c in report['cards'] if c['id']=='page-case')
        self.assertEqual(card['column'],'Planned')
        self.assertEqual(card['assignments']['freshness'],'retained')
        self.assertEqual(card['assignments']['assignments'][0]['reported_state'],'accepted')
        self.assertFalse(card['assignments']['assignments'][0]['editable'])
        self.assertFalse((self.root/'tend').exists())
        snapshot['goal']='Unrelated brief';path.write_bytes(p.encode(snapshot))
        card=next(c for c in self.board(assignments={instance.name:str(path)})['cards'] if c['id']=='page-case')
        self.assertEqual(card['column'],'Needs attention')
        self.assertEqual(card['assignments']['coverage'],'unverified')
        snapshot['goal']=self.input.read_text().rstrip('\n');snapshot['run_sha256']='sha256:'+'0'*64
        path.write_bytes(p.encode(snapshot))
        with self.assertRaisesRegex(ValueError,'admitted Manage run'):
            p.assignment_snapshot(instance,path)

    @unittest.skipUnless(TEND,'requires real Tend')
    def test_running_overdue_work_stays_in_progress(self):
        instance=self.admit_occurrence(environment={'MODE':'wait'})
        p.submit(instance)
        with ThreadPoolExecutor(max_workers=1) as pool:
            future=pool.submit(self.tend,'work')
            try:
                deadline=fixtures.time.monotonic()+5
                while not (instance/'run/started').exists() and fixtures.time.monotonic()<deadline:
                    fixtures.time.sleep(.02)
                self.assertTrue((instance/'run/started').exists())
                report=p.kanban_view(self.root,'2035-01-01T00:00:00Z')
                card=next(c for c in report['cards'] if c['id']==self.request['id'])
                self.assertEqual((card['column'],card['timeliness']),('In progress','overdue'))
            finally:
                (instance/'run/release').touch()
            future.result()

    def test_editing_done_preserves_completion_but_reopening_starts_new_completion(self):
        evidence=self.base/'approval.txt';evidence.write_text('Decision evidence')
        done={**self.activity,'state':'done','due_at':'2026-03-09T16:00:00Z',
              'evidence':[{'path':str(evidence),'sha256':p.sha(evidence.read_bytes())}]}
        first=p.record_activity(self.root,done)
        with patch.object(p,'now',return_value='2026-03-10T15:00:00Z'):
            second=p.record_activity(self.root,{**done,'title':'Corrected title','reason':'Correct wording','previous':first['revision']})
            item=self.human(p.kanban_view(self.root,p.now()))
            self.assertEqual((item['completed_at'],item['timeliness']),('2026-03-09T15:00:00Z','on-time'))
            reopened=p.record_activity(self.root,{**done,'state':'ready','reason':'Reopen for follow-up','previous':second['revision']})
            p.record_activity(self.root,{**done,'reason':'Completed follow-up','previous':reopened['revision']})
            self.assertEqual(self.human(p.kanban_view(self.root,p.now()))['completed_at'],'2026-03-10T15:00:00Z')

    def test_reopened_activity_still_verifies_historical_completion_evidence(self):
        evidence=self.base/'approval.txt';evidence.write_text('Decision evidence')
        digest=p.sha(evidence.read_bytes())
        done=p.record_activity(self.root,{**self.activity,'state':'done','evidence':[{'path':str(evidence),'sha256':digest}]})
        p.record_activity(self.root,{**self.activity,'state':'ready','reason':'Reopen','previous':done['revision']})
        (self.root/'activities'/p.tracking_key('test-team',self.activity['id'])/'evidence'/digest).write_text('corrupted')
        self.assertEqual(self.human(self.board())['column'],'Needs attention')
        with self.assertRaisesRegex(ValueError,'incomplete'):p.reconcile(self.root)

    def test_live_http_view_refreshes_without_mutating_work_and_rejects_writes(self):
        server=p.kanban_server(self.root,port=0,interval=1)
        worker=threading.Thread(target=server.serve_forever,daemon=True);worker.start()
        self.addCleanup(server.server_close);self.addCleanup(server.shutdown)
        connection=http.client.HTTPConnection('127.0.0.1',server.server_port,timeout=5)
        self.addCleanup(connection.close)
        before=p.tracking_digest(self.root)
        connection.request('GET','/');response=connection.getresponse();page=response.read().decode()
        self.assertEqual(response.status,200)
        self.assertIn("fetch('/view'",page)
        self.assertIn('last successful refresh:',page)
        self.assertEqual(p.tracking_digest(self.root),before)
        self.assertFalse((self.root/'tend').exists())
        p.record_activity(self.root,self.activity)
        fixtures.time.sleep(1.05)
        connection.request('GET','/view');response=connection.getresponse();updated=response.read().decode()
        self.assertEqual(response.status,200)
        self.assertIn('Obtain stakeholder approval',updated)
        self.assertNotIn("fetch('/view'",updated)
        self.assertEqual(response.getheader('Cache-Control'),'no-store')
        for method,url,headers,expected in [('POST','/',{},405),('GET','/../process.py',{},404),
            ('GET','/',{'Host':'evil.example'},403),('GET','/',{'Origin':'https://evil.example'},403)]:
            connection.request(method,url,headers=headers);response=connection.getresponse();response.read()
            self.assertEqual(response.status,expected)
        fixtures.time.sleep(1.05)
        with patch.object(p,'render_kanban',side_effect=ValueError('fixture render failure')):
            connection.request('GET','/view');response=connection.getresponse();response.read()
            self.assertEqual(response.status,503)

    @unittest.skipUnless(TEND,'requires real Tend')
    def test_live_assignments_poll_only_pinned_public_manage_status(self):
        manage=self.base/'manage'
        manage.write_text('''#!/usr/bin/env python3
import os,pathlib,sys
assert sys.argv[1:3] == ['status','-json'] and len(sys.argv)==4
assert os.environ['PYTHONDONTWRITEBYTECODE']=='1'
print((pathlib.Path(sys.argv[3])/'fixture-status.json').read_text())
''');manage.chmod(0o755)
        self.d['team']='page-team';self.d['configuration']['BENCH_MANAGE']='program';self.seal_bundle()
        lock=p.load(self.export/'team.lock.json');lock['team']='page-team';(self.export/'team.lock.json').write_bytes(p.encode(lock))
        self.request.update(id='live-page',environment={'BENCH_MANAGE':str(manage)})
        instance=p.admit(self.export,self.request,self.root,TEND);(instance/'run').mkdir()
        manifest=p.encode({'schema':'bench.manage.run/v1','goal':self.input.read_text().rstrip('\n'),'run':'live-fixture'})
        (instance/'run/manifest.json').write_bytes(manifest)
        snapshot={'schema':'bench.manage.snapshot/v1','run_sha256':'sha256:'+p.sha(manifest),
            'as_of':p.instant(p.now()).timestamp(),'goal':self.input.read_text().rstrip('\n'),
            'tasks':[{'id':'design','input':{'worker':'manager','goal':'Design the page'},'state':'ready','needs':[]}]}
        output=instance/'run/fixture-status.json';output.write_bytes(p.encode(snapshot))
        report=p.kanban_view(self.root,p.now(),live_assignments=True)
        card=next(c for c in report['cards'] if c['id']=='live-page')
        self.assertEqual(card['assignments']['coverage'],'public-status')
        self.assertEqual(card['assignments']['assignments'][0]['reported_state'],'ready')
        snapshot['tasks'][0]['state']='running';output.write_bytes(p.encode(snapshot))
        observed=p.assignment_snapshot(instance,'manage-status')
        self.assertEqual(observed['assignments'][0]['reported_state'],'running')
        self.assertEqual(observed['freshness'],'current')
        self.assertFalse((instance/'run/tend').exists())
        (instance/'run/tend').mkdir()
        with self.assertRaisesRegex(ValueError,'not been initialized'):
            p.assignment_snapshot(instance,'manage-status')
        self.assertEqual(list((instance/'run/tend').iterdir()),[])


if __name__=='__main__':unittest.main()
