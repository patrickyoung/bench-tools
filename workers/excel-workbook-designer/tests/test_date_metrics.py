"""Synthetic public-check tests for typed Date metrics; no model or renderer."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import zipfile
from xml.sax.saxutils import quoteattr

sys.dont_write_bytecode=True
EXPERT=Path(__file__).resolve().parents[1]/'expert'
sys.path.insert(0,str(EXPERT/'lib'))
from contract import excel_date_serial,metric_equal

S='http://schemas.openxmlformats.org/spreadsheetml/2006/main'
R='http://schemas.openxmlformats.org/officeDocument/2006/relationships'
P='http://schemas.openxmlformats.org/package/2006/relationships'
ISO='2026-09-19T00:00:00.000Z'
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()

class DateMetrics(unittest.TestCase):
    def setUp(self):
        temporary=tempfile.TemporaryDirectory(prefix='date-metric-contract-');self.addCleanup(temporary.cleanup)
        self.root=Path(temporary.name);(self.root/'inputs').mkdir();(self.root/'output').mkdir()
    def fixture(self,serial=46284,date1904=False,text=False,date_format=True,markers=None,baseline=ISO,expected=None,custom_format=None,typed_operation=None):
        cell=f'<c r="B2" s="1" t="inlineStr"><is><t>{ISO}</t></is></c>' if text else f'<c r="B2" s="1"><v>{serial}</v></c>'
        format_id=164 if custom_format is not None else 14 if date_format else 0
        formats=f'<numFmts count="1"><numFmt numFmtId="164" formatCode={quoteattr(custom_format)}/></numFmts>' if custom_format is not None else ''
        parts={
            'xl/workbook.xml':f'<workbook xmlns="{S}" xmlns:r="{R}"><workbookPr date1904="{int(date1904)}"/><sheets><sheet name="Records" sheetId="1" r:id="r1"/></sheets></workbook>',
            'xl/_rels/workbook.xml.rels':f'<Relationships xmlns="{P}"><Relationship Id="r1" Type="{R}/worksheet" Target="worksheets/sheet1.xml"/></Relationships>',
            'xl/worksheets/sheet1.xml':f'<worksheet xmlns="{S}"><sheetData><row r="2">{cell}<c r="C2"><v>7</v></c></row></sheetData></worksheet>',
            'xl/styles.xml':f'<styleSheet xmlns="{S}">{formats}<fonts count="1"><font/></fonts><fills count="1"><fill/></fills><borders count="1"><border/></borders><cellXfs count="2"><xf fontId="0" fillId="0" borderId="0" numFmtId="0"/><xf fontId="0" fillId="0" borderId="0" numFmtId="{format_id}"/></cellXfs></styleSheet>'}
        for path in [self.root/'inputs/source.xlsx',self.root/'output/workbook.xlsx']:
            with zipfile.ZipFile(path,'w') as z:
                for name,data in parts.items():z.writestr(name,data)
        inputs=[{'path':'inputs/source.xlsx','sha256':sha(self.root/'inputs/source.xlsx')}]
        (self.root/'request.json').write_text(json.dumps({'schema':'bench.workbook-request/v1','mode':'edit','workbook':'inputs/source.xlsx','brief':'Retain the date and confirm the recorded count.','inputs':inputs,'required_metrics':['day']}))
        spec={'schema':'bench.workbook-edit/v1','request_sha256':sha(self.root/'request.json'),'inputs':inputs,'purpose':'Synthetic date metric','interpretation':'Retain source facts.',
            'changes':[{'sheet':'Records','range':'C2','reason':'Confirm recorded count.'}],
            'operations':[{'op':'values','sheet':'Records','range':'C2','values':[[7]]}],
            'metrics':[{'id':'day','sheet':'Records','cell':'B2'}],
            'assertions':[{'metric':'day','value':{'date':'2026-09-19'} if expected is None else expected}],
            'tests':[],'previews':[{'sheet':'Records','range':'A1:C3'}],'mappings':[],'pivots':[]}
        if typed_operation is not None:
            spec['changes'][0]['range']='B2:C2'
            spec['operations'].append({'op':'values','sheet':'Records','range':'B2','values':[[{'date':typed_operation}]]})
        (self.root/'output/spec.json').write_text(json.dumps(spec))
        (self.root/'output/guide.md').write_text('Synthetic public-check fixture, not a rendered workbook or a model evaluation. No visual quality is claimed.\n')
        (self.root/'output/preview.txt').write_text('Synthetic preview bytes for binding only.\n')
        qa={'spec_sha256':sha(self.root/'output/spec.json'),'workbook_sha256':sha(self.root/'output/workbook.xlsx'),'baseline':{'day':baseline},
            'baseline_date_metrics':['day'] if markers is None else markers,'assertions':[{'passed':True}],'tests':[],
            'previews':[{'path':'output/preview.txt','sha256':sha(self.root/'output/preview.txt')}]}
        (self.root/'output/qa.json').write_text(json.dumps(qa))
    def check(self):
        return subprocess.run([str(EXPERT/'bin/check')],cwd=self.root,env=dict(os.environ,WORKBOOK_PYTHON=sys.executable,PYTHONDONTWRITEBYTECODE='1'),capture_output=True,text=True,timeout=15)
    def test_known_date_system_serials_and_leap_boundary(self):
        for value,system,want in [('1900-01-01',False,1),('1900-02-28',False,59),('1900-03-01',False,61),('1904-01-01',True,0),('1904-01-02',True,1),('2026-09-19',False,46284),('2026-09-19',True,44822)]:
            self.assertEqual(excel_date_serial(value+'T00:00:00.000Z',system),want)
        self.assertIsNone(excel_date_serial('1900-02-29T00:00:00.000Z'))
    def test_public_typed_date_and_exact_iso_pass(self):
        for expected in [{'date':'2026-09-19'},ISO]:
            self.fixture(expected=expected);result=self.check();self.assertEqual(result.returncode,0,result.stderr)
    def test_public_1904_date_passes(self):
        self.fixture(serial=44822,date1904=True);result=self.check();self.assertEqual(result.returncode,0,result.stderr)
    def test_typed_operations_use_saved_date_system_and_1900_leap_boundary(self):
        for day,serial,system in [('2026-09-19',44822,True),('1900-01-01',1,False),('1900-02-28',59,False),('1900-03-01',61,False)]:
            self.fixture(serial=serial,date1904=system,baseline=day+'T00:00:00.000Z',expected={'date':day},typed_operation=day)
            result=self.check();self.assertEqual(result.returncode,0,result.stderr)
        self.fixture(serial=44822,date1904=True,typed_operation='2026-09-18');result=self.check();self.assertNotEqual(result.returncode,0);self.assertIn('Requested values Records!B2',result.stderr)
    def test_wrong_day_and_wrong_saved_serial_reject(self):
        self.fixture(expected={'date':'2026-09-18'});result=self.check();self.assertNotEqual(result.returncode,0);self.assertIn('Independent asserted baseline day',result.stderr)
        self.fixture(serial=46283);result=self.check();self.assertNotEqual(result.returncode,0);self.assertIn('Saved date metric disagrees',result.stderr)
    def test_date_marker_cannot_reclassify_text_or_plain_number(self):
        for kwargs in [{'text':True},{'date_format':False}]:
            self.fixture(**kwargs);result=self.check();self.assertNotEqual(result.returncode,0);self.assertIn('not a saved numeric date cell' if kwargs.get('text') else 'Unsupported calendar date format',result.stderr)
    def test_unmarked_text_does_not_satisfy_typed_date(self):
        self.fixture(text=True,markers=[]);result=self.check();self.assertNotEqual(result.returncode,0);self.assertIn('Independent asserted baseline day',result.stderr)
        self.fixture(text=True,markers=[],expected=ISO);result=self.check();self.assertEqual(result.returncode,0,result.stderr)
    def test_custom_calendar_guard_rejects_literals_time_and_sections(self):
        self.fixture(custom_format='yyyy-mm-dd');result=self.check();self.assertEqual(result.returncode,0,result.stderr)
        for fmt in ['0 "days"','hh:mm:ss','0;yyyy-mm-dd','[>0]0;yyyy-mm-dd','yyyy-mm-dd;@']:
            self.fixture(custom_format=fmt);result=self.check();self.assertNotEqual(result.returncode,0);self.assertIn('Unsupported calendar date format',result.stderr)
    def test_duplicate_unknown_and_bad_marker_types_reject(self):
        for markers in [['day','day'],['missing'],[1],'day']:
            self.fixture(markers=markers);result=self.check();self.assertNotEqual(result.returncode,0);self.assertIn('date metric',result.stderr)
    def test_numeric_serial_workaround_stays_numeric(self):
        self.fixture(markers=[],baseline=46284,expected=46284);result=self.check();self.assertEqual(result.returncode,0,result.stderr)
    def test_strict_numeric_boolean_and_text_comparisons(self):
        for actual,expected in [(1,True),(True,1),('1',1),(1,'1'),(False,0),(0,False)]:self.assertFalse(metric_equal(actual,expected))
        for value in [1,True,'1',False,0,None]:self.assertTrue(metric_equal(value,value))
        self.assertTrue(metric_equal(2,2.0001,0.001));self.assertFalse(metric_equal(2,2.1,0.001))
    def test_invalid_dates_and_noncanonical_expectations_reject(self):
        for expected in [{'date':'2026-02-31'},{'date':'2026-09-19','extra':1},'2026-09-19']:
            self.fixture(expected=expected);self.assertNotEqual(self.check().returncode,0)
    def test_invalid_typed_operation_cannot_match_unmarked_blank_cell(self):
        for day in ['1900-02-29','2026-02-31']:
            self.fixture()
            path=self.root/'output/spec.json';spec=json.loads(path.read_text())
            spec['changes'].append({'sheet':'Records','range':'D2','reason':'Synthetic invalid date must fail.'})
            spec['operations'].append({'op':'values','sheet':'Records','range':'D2','values':[[{'date':day}]]})
            path.write_text(json.dumps(spec))
            qa_path=self.root/'output/qa.json';qa=json.loads(qa_path.read_text());qa['spec_sha256']=sha(path);qa_path.write_text(json.dumps(qa))
            result=self.check();self.assertNotEqual(result.returncode,0);self.assertIn('Requested values Records!D2',result.stderr)

if __name__=='__main__':unittest.main(verbosity=2)
