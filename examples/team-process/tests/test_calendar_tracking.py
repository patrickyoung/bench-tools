"""Calendar accountability, independent of catch-up policy and execution."""
import copy
from concurrent.futures import ThreadPoolExecutor
import subprocess
import sys
import unittest
from unittest.mock import patch

import test_process as fixtures

p = fixtures.p
TEND = fixtures.TEND


class CalendarTrackingTests(unittest.TestCase):
    seal_bundle = fixtures.TendIntegrationTests.seal_bundle
    tend = fixtures.TendIntegrationTests.tend

    def setUp(self):
        fixtures.TendIntegrationTests.setUp(self)
        self.calendar_file = self.base/'calendar.json'
        self.cal = fixtures.calendar()
        self.cal.update(start_date='2026-03-06', end_date='2026-04-06', missed='skip')
        self.calendar_file.write_bytes(p.encode(self.cal))
        self.registration = {'schema': 'bench.calendar-registration/v1', 'namespace': 'test-team',
            'owner': 'Calendar owner', 'calendar': str(self.calendar_file), 'effective_from': '2026-03-06',
            'reason': 'Initial daily review schedule', 'check_every_seconds': 3600}
        self.clock = patch.object(p, 'now', return_value='2026-03-09T15:00:00Z')
        self.clock.start(); self.addCleanup(self.clock.stop)
        self.revision = p.register_calendar(self.export, self.registration, self.root)

    def view(self, at='2026-03-09T15:00:00Z'):
        return p.calendar_view(self.root, at)

    def row(self, report, day='2026-03-06'):
        return next(r for r in report['obligations'] if r['id'] == 'daily-review/'+day)

    def disposition(self, kind='skipped', day='2026-03-06', **extra):
        return {'schema':'bench.calendar-disposition/v1', 'namespace':'test-team',
            'calendar_id':'daily-review', 'id':'daily-review/'+day,
            'calendar_revision':self.revision['revision'], 'kind':kind, 'by':'Calendar owner',
            'reason':'Explicit selected business decision', **extra}

    def admit_occurrence(self, day='2026-03-06', **extra):
        proposal = next(r for r in p.occurrences(self.d, self.cal) if r['id'].endswith(day))
        self.request.update({k: proposal[k] for k in ('id','not_before','due_at','timezone','occurrence')})
        self.request.update(calendar=str(self.calendar_file), **extra)
        return p.admit(self.export, self.request, self.root, TEND)

    def test_policy_cannot_hide_missing_or_upcoming_work(self):
        for policy in ('all','latest','skip'):
            registered = p.registered(self.root)[0]
            registered[0][0]['payload']['calendar']['missed'] = policy
            expected = p.expected_work(registered)
            self.assertIn(('test-team','daily-review/2026-03-06'), expected)
            self.assertIn(('test-team','daily-review/2026-04-06'), expected)
        report = self.view()
        self.assertEqual(self.row(report)['state'], 'missing')
        self.assertEqual(self.row(report)['attention'], ['missing-commitment','overdue'])
        self.assertEqual(self.row(report,'2026-03-10')['state'], 'upcoming')
        self.assertFalse((self.root/'tend').exists())
        self.assertFalse((self.root/'last-reconciliation.json').exists())

    def test_registration_snapshots_input_and_is_concurrently_idempotent(self):
        with ThreadPoolExecutor(max_workers=4) as pool:
            revisions = list(pool.map(lambda _: p.register_calendar(self.export,self.registration,self.root), range(4)))
        self.assertEqual({r['revision'] for r in revisions}, {self.revision['revision']})
        self.calendar_file.write_text('source later replaced')
        self.assertEqual(len(p.registered(self.root)[0][0]), 1)
        self.assertEqual(self.row(self.view())['state'], 'missing')

    def test_calendar_revision_preserves_history_and_old_obligations(self):
        self.cal['due_time'] = '16:00'
        self.cal['excluded_dates'] = ['2026-03-06']
        self.calendar_file.write_bytes(p.encode(self.cal))
        update = {**self.registration, 'effective_from':'2026-03-10', 'reason':'Earlier delivery from Tuesday',
                  'previous':self.revision['revision'], 'owner':'New owner'}
        revision = p.register_calendar(self.export, update, self.root)
        report = self.view()
        self.assertEqual(self.row(report)['due_at'], '2026-03-06T22:00:00Z')
        self.assertEqual(self.row(report)['owner'], 'Calendar owner')
        self.assertEqual(self.row(report,'2026-03-10')['due_at'], '2026-03-10T20:00:00Z')
        self.assertEqual(self.row(report,'2026-03-10')['owner'], 'New owner')
        self.assertEqual(revision['previous'], self.revision['revision'])
        self.assertEqual(len(report['calendars'][0]['history']), 2)
        with self.assertRaisesRegex(ValueError, 'current revision'):
            p.register_calendar(self.export,{**update,'effective_from':'2026-03-11','reason':'Racing update'},self.root)

    def test_retroactive_timezone_and_unpinned_registration_rejected(self):
        for effective in ('2026-03-08','2026-03-09'):
            with self.assertRaisesRegex(ValueError, 'after today'):
                p.register_calendar(self.export,{**self.registration,'effective_from':effective,
                    'reason':'Rewrite past','previous':self.revision['revision']},self.root)
        self.cal['timezone'] = 'UTC'; self.calendar_file.write_bytes(p.encode(self.cal))
        with self.assertRaisesRegex(ValueError, 'timezone'):
            p.register_calendar(self.export,{**self.registration,'effective_from':'2026-03-10'},self.root)
        (self.expert/'bin/run').write_text('changed')
        with self.assertRaisesRegex(ValueError, 'changed exported'):
            p.register_calendar(self.export,self.registration,self.root)

    def test_skip_reopen_and_retained_reason(self):
        decision = p.record_disposition(self.root,self.disposition())
        self.assertEqual(p.record_disposition(self.root,self.disposition()), decision)
        row = self.row(self.view())
        self.assertEqual(row['state'], 'skipped')
        self.assertEqual(row['attention'], [])
        reopened = p.record_disposition(self.root,self.disposition('reopened', previous=decision['revision']))
        row = self.row(self.view())
        self.assertEqual(row['state'],'missing')
        self.assertIn('overdue',row['attention'])
        self.assertEqual(len(row['disposition_history']),2)
        self.assertEqual(reopened['previous'],decision['revision'])
        with self.assertRaisesRegex(ValueError,'current revision'):
            p.record_disposition(self.root,self.disposition('cancelled', previous=decision['revision']))

    def test_disposition_requires_real_occurrence_and_current_revision(self):
        for changes in ({'id':'daily-review/2026-03-07'}, {'calendar_revision':'0'*64}, {'reason':''}):
            with self.assertRaises(ValueError): p.record_disposition(self.root,{**self.disposition(),**changes})
        with self.assertRaisesRegex(ValueError,'only a disposition'):
            p.record_disposition(self.root,self.disposition('reopened'))

    def test_monitor_age_and_changed_inventory_are_separate_from_work(self):
        self.assertEqual(self.view()['monitor']['state'],'never-reconciled')
        report = p.reconcile(self.root)
        self.assertEqual(report['monitor']['state'],'current')
        self.assertIn('missing-commitment', self.row(report)['attention'])
        self.assertEqual(report['monitor']['last_success_at'],'2026-03-09T15:00:00Z')
        with patch.object(p,'now',return_value='2026-03-09T16:00:00Z'):
            self.assertEqual(self.view()['monitor']['state'],'current')
        with patch.object(p,'now',return_value='2026-03-09T16:00:01Z'):
            self.assertEqual(self.view('2026-03-09T10:00:00Z')['monitor']['state'],'stale')
        p.record_disposition(self.root,self.disposition())
        self.assertEqual(self.view()['monitor']['state'],'changed')

    def test_failed_reconciliation_does_not_refresh_last_success(self):
        p.reconcile(self.root)
        checkpoint = (self.root/'last-reconciliation.json').read_bytes()
        target = next((self.root/'calendars').glob('*/*.json'))
        target.write_text('{malformed')
        report = self.view()
        self.assertEqual(report['monitor']['state'],'unverified')
        self.assertIn('calendar-unverified',{e['reason'] for e in report['errors']})
        with self.assertRaisesRegex(ValueError,'incomplete'): p.reconcile(self.root)
        self.assertEqual((self.root/'last-reconciliation.json').read_bytes(),checkpoint)

    def test_concurrent_registration_cannot_seal_an_incomplete_snapshot(self):
        original = p.tracking_digest
        calls = []
        def raced(root):
            calls.append(True)
            if len(calls) == 2:
                other = {**self.registration,'namespace':'another-team'}
                p.register_calendar(self.export,other,self.root)
            return original(root)
        with patch.object(p,'tracking_digest',side_effect=raced):
            with self.assertRaisesRegex(ValueError,'incomplete'): p.reconcile(self.root)
        self.assertFalse((self.root/'last-reconciliation.json').exists())
        report = p.reconcile(self.root)
        self.assertEqual(len(report['calendars']),2)
        self.assertEqual(report['monitor']['state'],'current')

    def test_future_revision_does_not_relax_current_monitor_cadence(self):
        update = {**self.registration,'effective_from':'2026-03-10','previous':self.revision['revision'],
                  'reason':'Weekly checking after Tuesday','check_every_seconds':604800}
        p.register_calendar(self.export,update,self.root)
        p.reconcile(self.root)
        with patch.object(p,'now',return_value='2026-03-09T17:00:00Z'):
            report = self.view('2026-03-11T00:00:00Z')
            self.assertEqual(report['monitor']['state'],'stale')
            self.assertEqual(report['monitor']['check_every_seconds'],3600)
        with patch.object(p,'now',return_value='2026-03-10T15:00:00Z'):
            self.assertEqual(self.view()['monitor']['check_every_seconds'],604800)

    def test_future_renewal_preserves_expiry_and_reports_coverage_gap(self):
        self.cal.update(start_date='2026-05-01',end_date='2026-06-01')
        self.calendar_file.write_bytes(p.encode(self.cal))
        p.register_calendar(self.export,{**self.registration,'effective_from':'2026-05-01',
            'reason':'Renew after a gap','previous':self.revision['revision']},self.root)
        report = self.view('2026-04-08T00:00:00Z')
        current = report['calendars'][0]
        self.assertEqual(current['state'],'expired')
        self.assertEqual(current['coverage_gaps'],[{'start_date':'2026-04-07','end_date':'2026-04-30','reason':'Renew after a gap'}])
        self.assertIn('calendar-coverage-gap',{r for a in report['attention'] for r in a['reasons']})
        self.assertEqual(self.view('2026-05-05T00:00:00Z')['calendars'][0]['state'],'current')

    def test_expiry_and_renewal_visible(self):
        report = self.view('2026-04-01T00:00:00Z')
        self.assertEqual(report['calendars'][0]['state'],'ending-soon')
        report = self.view('2026-04-08T00:00:00Z')
        self.assertEqual(report['calendars'][0]['state'],'expired')
        self.assertIn('calendar-expired',{r for a in report['attention'] for r in a['reasons']})

    def test_corrupt_disposition_and_removed_history_are_visible(self):
        p.record_disposition(self.root,self.disposition())
        target = next((self.root/'dispositions').glob('*/*.json'))
        target.rename(target.with_name('000002.json'))
        report = self.view()
        self.assertEqual(report['monitor']['state'],'unverified')
        self.assertEqual(self.row(report)['state'],'missing')
        self.assertIn('disposition-unverified',{e['reason'] for e in report['errors']})

    def test_html_and_ics_escape_untrusted_text_and_preserve_unicode(self):
        report = self.view()
        row = report['obligations'][0]
        row['objective'] = '</script><img src=x onerror=alert(1)> café '+('旅'*50)+'\nsecond,third;fourth'
        rendered = p.render_calendar(report)
        self.assertNotIn('<img src=x',rendered)
        self.assertIn('&lt;img src=x',rendered)
        self.assertIn('Needs attention',rendered)
        self.assertIn('Last successful check:',rendered)
        self.assertIn('data-month="2026-03"',rendered)
        ics = p.calendar_ics(report)
        self.assertTrue(all(len(line.encode()) <= 75 for line in ics.split('\r\n')))
        unfolded = ics.replace('\r\n ','')
        self.assertIn('旅'*50,unfolded)
        self.assertIn('\\nsecond\\,third\\;fourth',unfolded)
        self.assertEqual(ics.count('BEGIN:VEVENT'),len(report['obligations']))
        again = p.calendar_ics({**report,'observed_at':'2026-03-09T16:00:00Z'})
        self.assertEqual([l for l in ics.splitlines() if l.startswith('UID:')],
                         [l for l in again.splitlines() if l.startswith('UID:')])

    def test_cli_calendar_is_read_only_and_outputs_html(self):
        before = sorted(str(f.relative_to(self.root)) for f in self.root.rglob('*'))
        result = subprocess.run([sys.executable,str(fixtures.APP),'calendar',str(self.root),
            '--as-of','2026-03-09T15:00:00Z','--format','html'],capture_output=True,text=True)
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertIn('<title>Team calendar and attention</title>',result.stdout)
        self.assertEqual(before,sorted(str(f.relative_to(self.root)) for f in self.root.rglob('*')))

    def test_ics_work_and_milestone_identity_cannot_collide(self):
        report = self.view()
        work = report['obligations'][0]
        work['milestones'] = [{'id':'review','owner':'Reviewer','expectation':'Review',
            'due_at':work['due_at'],'state':'unconfirmed','timeliness':'overdue'}]
        other = copy.deepcopy(work)
        other['id'] += '/milestone/review'; other['milestones'] = []
        report['obligations'] = [work,other]
        lines = p.calendar_ics(report).splitlines()
        self.assertEqual(len({line for line in lines if line.startswith('UID:')}),3)

    @unittest.skipUnless(TEND,'requires real Tend')
    def test_admitted_work_and_missing_occurrences_are_both_visible(self):
        instance = self.admit_occurrence(milestones=[{'id':'review','owner':'Reviewer',
            'due_at':'2026-03-06T20:00:00Z','expectation':'Independent review'}])
        row = self.row(self.view())
        self.assertEqual(row['state'],'unsubmitted')
        self.assertIn('not-submitted',row['attention'])
        self.assertIn('milestone:review',row['attention'])
        self.assertEqual(self.row(self.view(),'2026-03-09')['state'],'missing')
        with self.assertRaisesRegex(ValueError,'cannot be skipped'):
            p.record_disposition(self.root,self.disposition())
        self.assertFalse((self.root/'tend').exists())
        p.submit(instance)
        row = self.row(self.view())
        self.assertEqual(row['state'],'active')
        self.assertIn('overdue',row['attention'])

    @unittest.skipUnless(TEND,'requires real Tend')
    def test_future_revised_calendar_does_not_hide_existing_commitment(self):
        self.admit_occurrence('2026-03-10')
        self.cal['excluded_dates']=['2026-03-10']; self.calendar_file.write_bytes(p.encode(self.cal))
        p.register_calendar(self.export,{**self.registration,'effective_from':'2026-03-10',
            'reason':'No work next Tuesday','previous':self.revision['revision']},self.root)
        row = self.row(self.view(),'2026-03-10')
        self.assertIn('outside-registered-calendar',row['attention'])

    @unittest.skipUnless(TEND,'requires real Tend')
    def test_calendar_drift_is_visible_without_rewriting_commitment(self):
        self.admit_occurrence('2026-03-10')
        self.cal['due_time']='16:00'; self.calendar_file.write_bytes(p.encode(self.cal))
        p.register_calendar(self.export,{**self.registration,'effective_from':'2026-03-10',
            'reason':'Earlier delivery','previous':self.revision['revision']},self.root)
        row = self.row(self.view(),'2026-03-10')
        self.assertIn('calendar-conflict',row['attention'])
        self.assertEqual(row['commitment_dates']['due_at'],'2026-03-10T21:00:00Z')
        self.assertEqual(row['due_at'],'2026-03-10T20:00:00Z')

    @unittest.skipUnless(TEND,'requires real Tend')
    def test_cancellation_does_not_release_unknown_or_hide_it(self):
        instance = self.admit_occurrence(environment={'MODE':'unknown'})
        p.submit(instance); self.tend('work')
        p.record_disposition(self.root,self.disposition('cancelled'))
        row = self.row(self.view())
        self.assertEqual(row['state'],'cancelled')
        self.assertEqual(row['execution'],'unknown')
        self.assertIn('execution-outcome-unknown',row['attention'])
        self.assertIn('disposition-execution-conflict',row['attention'])
        next_instance = self.admit_occurrence('2026-03-09',environment={})
        p.submit(next_instance); self.tend('work',expected=1)
        self.assertFalse((next_instance/'run').exists())

    @unittest.skipUnless(TEND,'requires real Tend')
    def test_completion_stale_artifacts_and_broken_records_never_look_missing(self):
        instance = self.admit_occurrence()
        p.submit(instance); self.tend('work')
        row = self.row(self.view('2035-01-01T00:00:00Z'))
        self.assertEqual((row['state'],row['acceptance'],row['timeliness']),('completed','accepted','late'))
        (instance/'run/result.txt').unlink()
        row = self.row(self.view('2035-01-01T00:00:00Z'))
        self.assertEqual(row['acceptance'],'stale')
        self.assertIn('stale',row['attention'])
        (instance/'admission.json').write_text('{}')
        report = self.view()
        self.assertTrue(report['errors'])
        self.assertEqual(report['monitor']['state'],'unverified')
        self.assertEqual(self.row(report)['state'],'unverified')


if __name__ == '__main__':
    unittest.main()
