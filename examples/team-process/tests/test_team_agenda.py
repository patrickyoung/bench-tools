"""Public-command composition with real Agenda and Tend; deterministic teams."""
import importlib.util
from datetime import datetime, timezone
import os
from pathlib import Path
import shlex
import shutil
import unittest

import test_process as fixtures

spec = importlib.util.spec_from_file_location('team_bridge',fixtures.APP.with_name('team.py'))
team = importlib.util.module_from_spec(spec); spec.loader.exec_module(team)
AGENDA = os.environ.get('PROCESS_TEST_AGENDA')


@unittest.skipUnless(AGENDA and fixtures.TEND,'select real Agenda and Tend')
class AgendaBridgeTests(unittest.TestCase):
    setUp = fixtures.TendIntegrationTests.setUp
    seal_bundle = fixtures.TendIntegrationTests.seal_bundle
    tend = fixtures.TendIntegrationTests.tend

    def admit(self):
        request = self.base/'request.json'; request.write_bytes(team.encode(self.request))
        return team.admit(self.export,request,self.root,fixtures.TEND,AGENDA,self.base/'agenda')

    def observe(self,root=None):
        return team.observations(AGENDA,self.base/'agenda',root or self.root,'2035-01-01T00:00:00Z')

    def projection(self,rows):
        saved = self.base/'snapshot.json'
        saved.write_bytes(team.encode(team.export(AGENDA,self.base/'agenda')))
        return team.call([AGENDA,'project',str(saved),'--as-of',team.now()],b''.join(team.encode(r) for r in rows))

    def test_admit_execute_observe_without_private_store_access(self):
        result = self.admit(); instance = Path(result['instance'])
        self.assertEqual(result,self.admit())
        self.assertEqual(self.observe()[0]['state'],'unsubmitted')
        team.submit(instance); self.tend('work')
        rows = self.observe()
        self.assertEqual(rows[0]['state'],'accepted')
        self.assertEqual(self.projection(rows)['obligations'][0]['acceptance'],'accepted')
        self.assertEqual(len(team.export(AGENDA,self.base/'agenda')['records']),1)

    def test_unknown_is_retained_as_observation_with_evidence(self):
        self.request['environment']={'MODE':'unknown'}
        result = self.admit(); team.submit(result['instance']); self.tend('work')
        rows=self.observe()
        self.assertEqual(rows[0]['state'],'unknown')
        self.assertEqual(self.projection(rows)['obligations'][0]['execution'],'unknown')

    def test_changed_agenda_commitment_refuses_submit_and_acceptance(self):
        result=self.admit(); record=team.export(AGENDA,self.base/'agenda')['records'][0]
        value=record['change']['value']; value['title']='A changed promise'
        team.apply(AGENDA,self.base/'agenda','item',record['change']['id'],value,'Owner','Correct title',record['revision'])
        with self.assertRaisesRegex(ValueError,'commitment changed'): team.submit(result['instance'])
        self.assertEqual(self.observe()[0]['state'],'unverified')
        self.assertFalse((self.root/'tend').exists())

    def test_selected_run_root_and_evidence_tampering_fail_closed(self):
        result=self.admit(); instance=Path(result['instance'])
        self.assertEqual(self.observe(self.base/'another-root')[0]['state'],'unverified')
        team.submit(instance); self.tend('work')
        (instance/'run/result.txt').write_text('changed')
        self.assertNotEqual(self.observe()[0]['state'],'accepted')

    def test_adopt_legacy_admission_preserves_pins_and_does_not_submit(self):
        instance=fixtures.p.admit(self.export,self.request,self.root,fixtures.TEND)
        before=(instance/'admission.json').read_bytes()
        result=team.adopt(instance,AGENDA,self.base/'agenda')
        self.assertEqual((instance/'admission.json').read_bytes(),before)
        self.assertEqual(team.adopt(instance,AGENDA,self.base/'agenda'),result)
        self.assertFalse((self.root/'tend').exists())
        team.submit(instance); self.tend('work')
        self.assertEqual(self.observe()[0]['state'],'accepted')

    def test_stopped_observer_expires_independently_of_viewer(self):
        result=self.admit(); team.submit(result['instance']); self.tend('work')
        rows=self.observe(); rows[0]['observed_at']='2020-01-01T00:00:00Z'
        rows[0]['valid_until']='2020-01-01T00:01:00Z'
        rows[0]['completed_at']='2020-01-01T00:00:00Z'
        projection=self.projection(rows)
        self.assertIn('stale-observation',projection['obligations'][0]['attention'])

    def test_changed_selected_agenda_program_cannot_observe_accepted_work(self):
        selected=self.base/'agenda-command'
        shutil.copy2(AGENDA,selected)
        request=self.base/'request.json'; request.write_bytes(team.encode(self.request))
        result=team.admit(self.export,request,self.root,fixtures.TEND,str(selected),self.base/'agenda')
        team.submit(result['instance']); self.tend('work')
        # This caller-selected replacement returns identical public output but
        # no longer has the executable identity pinned during admission.
        selected.write_text('#!/bin/sh\nexec '+shlex.quote(AGENDA)+' "$@"\n')
        selected.chmod(0o755)
        rows=team.observations(str(selected),self.base/'agenda',self.root,'2035-01-01T00:00:00Z')
        self.assertEqual(rows[0]['state'],'unverified')

    def schedule(self):
        year=datetime.now(timezone.utc).year+1
        first=f'{year}-01-01'; occurrence=f'{year}-01-02'; future=f'{year}-01-03'
        value={'title':'Daily team review','owner':'Case owner','timezone':'UTC',
               'start_date':first,'end_date':f'{year}-01-31','weekdays':list(range(7)),
               'excluded_dates':[],'start_time':'09:00','due_time':'17:00','due_day_offset':0,
               'missed':'all','effective_from':first,'check_every_seconds':300}
        record=team.apply(AGENDA,self.base/'agenda','schedule','review/team',value,'Owner','Register schedule')
        self.request.update(not_before=occurrence+'T09:00:00Z',due_at=occurrence+'T17:00:00Z')
        return record,value,'review/team/'+occurrence,future

    def test_repeat_adoption_keeps_historical_schedule_revision(self):
        original,value,item,future=self.schedule()
        request=self.base/'request.json'; request.write_bytes(team.encode(self.request))
        first=team.admit(self.export,request,self.root,fixtures.TEND,AGENDA,self.base/'agenda',item)
        team.apply(AGENDA,self.base/'agenda','schedule','review/team',
                   {**value,'effective_from':future,'due_time':'18:00'},'Owner','Change future dates',original['revision'])
        self.assertEqual(team.adopt(first['instance'],AGENDA,self.base/'agenda',item),first)
        current=team.latest(team.export(AGENDA,self.base/'agenda'),'item')[item]
        self.assertEqual(current['change']['value']['occurrence']['schedule_revision'],original['revision'])

    def test_admission_selects_governing_revision_not_latest_future_revision(self):
        original,value,item,future=self.schedule()
        team.apply(AGENDA,self.base/'agenda','schedule','review/team',
                   {**value,'effective_from':future,'due_time':'18:00'},'Owner','Change future dates',original['revision'])
        request=self.base/'request.json'; request.write_bytes(team.encode(self.request))
        team.admit(self.export,request,self.root,fixtures.TEND,AGENDA,self.base/'agenda',item)
        current=team.latest(team.export(AGENDA,self.base/'agenda'),'item')[item]
        self.assertEqual(current['change']['value']['occurrence']['schedule_revision'],original['revision'])

    def test_occurrence_requires_existing_schedule_and_matching_dates(self):
        original,value,item,future=self.schedule()
        request=self.base/'request.json'; request.write_bytes(team.encode(self.request))
        with self.assertRaisesRegex(ValueError,'existing schedule occurrence'):
            team.admit(self.export,request,self.root,fixtures.TEND,AGENDA,self.base/'agenda','typo/2030-01-01')
        instance=next(self.root.glob('process-*'))
        self.request['due_at']=self.request['due_at'].replace('17:00','18:00')
        # A separate immutable execution request permits a deliberately different
        # promise, but it cannot masquerade as the existing scheduled obligation.
        self.request['id']='different-due';request.write_bytes(team.encode(self.request))
        with self.assertRaisesRegex(ValueError,'differs from the selected schedule'):
            team.admit(self.export,request,self.root,fixtures.TEND,AGENDA,self.base/'agenda',item)
        self.assertFalse((instance/'control/agenda-binding.json').exists())
        self.assertEqual(team.latest(team.export(AGENDA,self.base/'agenda'),'item'),{})
        self.assertFalse((self.root/'tend').exists())
