"""Optional visibility gate contracts. No model/network calls in this suite."""
import copy
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.dont_write_bytecode=True
EXPERT=Path(__file__).resolve().parents[1]/'expert'
sys.path.insert(0,str(EXPERT/'lib'))
import visual_check as v


class VisualCheck(unittest.TestCase):
    def setUp(self):
        temporary=tempfile.TemporaryDirectory(dir=Path(__file__).resolve().parent)
        self.addCleanup(temporary.cleanup);self.base=Path(temporary.name)
        self.work=self.base/'work';self.home=self.base/'expert';self.records=self.base/'records'
        for path in (self.work,self.home,self.records):path.mkdir()
        self.ask=self.base/'ask';self.record=self.base/'record'
        for path in (self.ask,self.record):path.write_text('#!/bin/sh\nexit 0\n');path.chmod(0o700)
        self.env={'WORKBOOK_VISUAL_ASK':str(self.ask),'WORKBOOK_VISUAL_MODEL':'fixture/model','WORKBOOK_VISUAL_RECORDS':str(self.records),'AGENT_RECORD':str(self.record)}
        self.doc={'cells':[{'id':'cell-001','sheet':'Records','cell':'A2','text':'Full text','crop_id':'crop-01'}]}
        self.good={'findings':[{'id':'cell-001','verdict':'pass','visible_text':'Full text','evidence':'Every character is visible.','feedback':''}],'limitations':['Only the selected crop was reviewed.']}

    def test_configuration_absent_and_complete(self):
        with patch.dict(os.environ,{},clear=True):self.assertIsNone(v.configuration(self.work,self.home))
        with patch.dict(os.environ,self.env,clear=True):self.assertEqual(v.configuration(self.work,self.home)['model'],'fixture/model')

    def test_partial_empty_unavailable_configuration(self):
        for env in ({'WORKBOOK_VISUAL_MODEL':'x'},dict(self.env,WORKBOOK_VISUAL_MODEL=''),dict(self.env,WORKBOOK_VISUAL_ASK='/missing-ask')):
            with self.subTest(env=env),patch.dict(os.environ,env,clear=True),self.assertRaises(v.Broken):v.configuration(self.work,self.home)

    def test_records_cannot_overlap_any_worker_write_root(self):
        for path,extra in [(self.work/'records',{}),(self.home/'records',{}),(self.base,{}),(self.records,{'AGENT_STATE':str(self.records)}),(self.records,{'AGENT_ACTION_TMP':str(self.base)}),(self.records,{'TMPDIR':str(self.base)}),(self.records,{'PLY_DIR':str(self.base)})]:
            with self.subTest(path=path,extra=extra),patch.dict(os.environ,{**self.env,**extra,'WORKBOOK_VISUAL_RECORDS':str(path)},clear=True),self.assertRaises(v.Broken):v.configuration(self.work,self.home)
        link=self.base/'linked';link.symlink_to(self.records,target_is_directory=True)
        with patch.dict(os.environ,{**self.env,'WORKBOOK_VISUAL_RECORDS':str(link)},clear=True),self.assertRaises(v.Broken):v.configuration(self.work,self.home)

    def test_executable_cannot_be_worker_controlled(self):
        executable=self.work/'ask';executable.write_text('#!/bin/sh\n');executable.chmod(0o700)
        with patch.dict(os.environ,{**self.env,'WORKBOOK_VISUAL_ASK':str(executable)},clear=True),self.assertRaises(v.Broken):v.configuration(self.work,self.home)

    def test_renderer_fingerprint_binds_selected_path_and_actual_bytes(self):
        selected={'WORKBOOK_VISUAL_SOFFICE':str(self.ask),'WORKBOOK_VISUAL_PDFTOPPM':str(self.record)}
        with patch.dict(os.environ,selected,clear=True):
            first=v.render_identities(self.work,self.home)
            self.ask.write_text('#!/bin/sh\nexit 1\n')
            second=v.render_identities(self.work,self.home)
            self.assertNotEqual(first['soffice']['sha256'],second['soffice']['sha256'])
            self.assertEqual(first['pdftoppm'],second['pdftoppm'])
            link=self.base/'alias';link.symlink_to(self.ask)
            with patch.dict(os.environ,{'WORKBOOK_VISUAL_SOFFICE':str(link)}):
                alias=v.render_identities(self.work,self.home)['soffice']
                self.assertEqual(alias['path'],str(self.ask));self.assertEqual(alias['sha256'],second['soffice']['sha256'])
                self.assertNotEqual(alias['selected_path'],second['soffice']['selected_path'])

    def test_missing_unavailable_relative_or_worker_owned_renderer(self):
        with patch.dict(os.environ,{},clear=True):self.assertTrue(all(x is None for x in v.render_identities(self.work,self.home).values()))
        for value in ('relative','/missing-renderer',str(self.work/'renderer')):
            with patch.dict(os.environ,{'WORKBOOK_VISUAL_SOFFICE':value},clear=True),self.assertRaises((v.Broken,OSError)):v.render_identities(self.work,self.home)

    def test_complete_pass_and_wrapped_transcript(self):
        self.assertEqual(v.verdict(v.encode(self.good),self.doc)[1],[])
        self.good['findings'][0]['visible_text']='Full\ntext'
        self.assertEqual(v.verdict(v.encode(self.good),self.doc)[1],[])

    def test_concrete_fail_uncertain_and_incomplete_pass_are_repairable(self):
        for kind in ('fail','uncertain','pass'):
            answer=copy.deepcopy(self.good);answer['findings'][0].update(verdict=kind,visible_text='Full tex',feedback='Wrap A2 and give that row sufficient height.')
            feedback=v.verdict(v.encode(answer),self.doc)[1]
            self.assertTrue(feedback);self.assertIn('Records!A2',feedback[0])

    def test_missing_extra_duplicate_and_forged_findings_are_broken(self):
        cases=[]
        for findings in ([],self.good['findings']*2,[dict(self.good['findings'][0],id='unknown')]):cases.append(dict(self.good,findings=findings))
        cases.extend([dict(self.good,accepted=True),dict(self.good,limitations=[]),dict(self.good,findings=[dict(self.good['findings'][0],evidence='')]),dict(self.good,findings=[dict(self.good['findings'][0],verdict='fail')])])
        for case in cases:
            with self.subTest(case=case),self.assertRaises(v.Broken):v.verdict(v.encode(case),self.doc)
        with self.assertRaises(v.Broken):v.verdict(b'{"findings":[],"findings":[],"limitations":["x"]}',self.doc)

    def test_timeout_retains_attempt_and_blocks_unchanged_resampling(self):
        (self.work/'request.json').write_text('{"mode":"edit"}')
        config={'records':self.records,'ask':str(self.ask),'record':str(self.record),'model':'fixture/model'}
        doc={**self.doc,'crops':[{'image':{'path':'crop.png'}}]};calls=[]
        def recorded(config,run,stage,*args,**kwargs):
            calls.append(stage)
            if stage=='judge':raise v.Broken('Synthetic recorded process timeout')
            (run/'context').mkdir();(run/'context/manifest.json').write_bytes(b'{}');return b'{}'
        with patch.object(v,'fingerprint',return_value={'files':{}}),patch.object(v,'context_current',return_value=(doc,[])),patch.object(v,'recorded',side_effect=recorded):
            with self.assertRaisesRegex(v.Broken,'timeout'):v.visual_check(self.work,self.home,config)
            self.assertEqual(len(list(self.records.glob('pending-*'))),1)
            with self.assertRaisesRegex(v.Broken,'pending'):v.visual_check(self.work,self.home,config)
        self.assertEqual(calls,['context','judge'])

    def test_preparation_failure_has_no_inference_tombstone(self):
        (self.work/'request.json').write_text('{"mode":"edit"}')
        config={'records':self.records,'ask':str(self.ask),'record':str(self.record),'model':'fixture/model'}
        with patch.object(v,'fingerprint',return_value={'files':{}}),patch.object(v,'recorded',side_effect=v.Broken('Preparation failed')):
            for _ in range(2):
                with self.assertRaises(v.Broken):v.visual_check(self.work,self.home,config)
                self.assertFalse(list(self.records.glob('pending-*')))


if __name__=='__main__':unittest.main()
