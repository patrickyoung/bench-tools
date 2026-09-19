"""Synthetic saved-XLSX feature evidence; no prior jobs, models or renderer."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import zipfile

sys.dont_write_bytecode = True
EXPERT = Path(__file__).resolve().parents[1] / 'expert'
sys.path.insert(0, str(EXPERT / 'lib'))
from edit_contract import saved_native_features

S = 'http://schemas.openxmlformats.org/spreadsheetml/2006/main'
R = 'http://schemas.openxmlformats.org/officeDocument/2006/relationships'
P = 'http://schemas.openxmlformats.org/package/2006/relationships'
C = 'http://schemas.openxmlformats.org/drawingml/2006/chart'
CHART_PART = 'xl/drawings/charts/chart7.xml'
REFERENCES = ['Records!$A$2:$A$3', 'Records!$B$2:$B$3']


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_json(path, value):
    path.write_text(json.dumps(value) + '\n')


def compact(value):
    return json.dumps(value, ensure_ascii=False, separators=(',', ':')).encode('utf-8')


def workbook(path, features=True, references=REFERENCES):
    """Small literal OOXML fixture; no claim of engine-rendered correctness."""
    parts = {
        '[Content_Types].xml': '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/></Types>',
        '_rels/.rels': f'<Relationships xmlns="{P}"><Relationship Id="r1" Type="{R}/officeDocument" Target="xl/workbook.xml"/></Relationships>',
        'xl/workbook.xml': f'<workbook xmlns="{S}" xmlns:r="{R}"><sheets><sheet name="Records" sheetId="1" r:id="r1"/></sheets></workbook>',
        'xl/_rels/workbook.xml.rels': f'<Relationships xmlns="{P}"><Relationship Id="r1" Type="{R}/worksheet" Target="worksheets/sheet1.xml"/></Relationships>',
        'xl/styles.xml': f'<styleSheet xmlns="{S}"><fonts count="1"><font/></fonts><fills count="1"><fill/></fills><borders count="1"><border/></borders><cellXfs count="1"><xf fontId="0" fillId="0" borderId="0" numFmtId="0"/></cellXfs></styleSheet>',
        'xl/worksheets/sheet1.xml': f'<worksheet xmlns="{S}" xmlns:r="{R}"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>Identifier</t></is></c><c r="B1" t="inlineStr"><is><t>Amount</t></is></c></row><row r="2"><c r="A2" t="inlineStr"><is><t>alpha</t></is></c><c r="B2"><v>7</v></c></row><row r="3"><c r="A3" t="inlineStr"><is><t>beta</t></is></c><c r="B3"><v>9</v></c></row></sheetData></worksheet>'}
    if features:
        parts['xl/worksheets/sheet1.xml'] = parts['xl/worksheets/sheet1.xml'].replace('</worksheet>', '<drawing r:id="d1"/><tableParts count="1"><tablePart r:id="t1"/></tableParts></worksheet>')
        parts.update({
            'xl/worksheets/_rels/sheet1.xml.rels': f'<Relationships xmlns="{P}"><Relationship Id="t1" Type="{R}/table" Target="../tables/table7.xml"/><Relationship Id="d1" Type="{R}/drawing" Target="../drawings/drawing7.xml"/></Relationships>',
            'xl/tables/table7.xml': f'<table xmlns="{S}" name="SyntheticLedger" displayName="SyntheticLedger" ref="A1:B3"><tableColumns count="2"><tableColumn id="1" name="Identifier"/><tableColumn id="2" name="Amount"/></tableColumns></table>',
            'xl/drawings/drawing7.xml': f'<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:c="{C}" xmlns:r="{R}"><xdr:twoCellAnchor><xdr:graphicFrame><c:chart r:id="c1"/></xdr:graphicFrame></xdr:twoCellAnchor></xdr:wsDr>',
            'xl/drawings/_rels/drawing7.xml.rels': f'<Relationships xmlns="{P}"><Relationship Id="c1" Type="{R}/chart" Target="charts/chart7.xml"/></Relationships>',
            CHART_PART: f'<c:chartSpace xmlns:c="{C}"><c:chart><c:plotArea><c:barChart><c:ser><c:cat><c:strRef><c:f>{references[0]}</c:f></c:strRef></c:cat><c:val><c:numRef><c:f>{references[1]}</c:f></c:numRef></c:val></c:ser></c:barChart></c:plotArea></c:chart></c:chartSpace>'})
    with zipfile.ZipFile(path, 'w', zipfile.ZIP_DEFLATED) as z:
        for name, text in parts.items():
            z.writestr(name, text)


class SavedNativeFeatures(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix='saved-native-features-')
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        (self.root/'inputs').mkdir();(self.root/'output').mkdir()

    def fixture(self, features=True):
        source, saved = self.root/'inputs/source.xlsx', self.root/'output/workbook.xlsx'
        workbook(source, features);workbook(saved, features)
        inputs = [{'path':'inputs/source.xlsx','sha256':sha(source)}]
        write_json(self.root/'request.json', {'schema':'bench.workbook-request/v1','mode':'edit','workbook':'inputs/source.xlsx',
            'brief':'Retain the synthetic native features and recorded amount.','inputs':inputs,'required_metrics':['amount']})
        specification = {'schema':'bench.workbook-edit/v1','request_sha256':sha(self.root/'request.json'),'inputs':inputs,
            'purpose':'Synthetic saved feature evidence','interpretation':'Retain source facts and inspect saved native features.',
            'changes':[{'sheet':'Records','range':'B2','reason':'Explicitly confirm the current amount.'}],
            'operations':[{'op':'values','sheet':'Records','range':'B2','values':[[7]]}],
            'metrics':[{'id':'amount','sheet':'Records','cell':'B2'}],'assertions':[{'metric':'amount','value':7}],
            'tests':[],'previews':[{'sheet':'Records','range':'A1:B3'}],'mappings':[],'pivots':[]}
        write_json(self.root/'output/spec.json', specification)
        (self.root/'output/guide.md').write_text('Synthetic contract fixture only; not a rendered, visually reviewed or model-authored workbook deliverable.\n')
        (self.root/'output/preview.txt').write_text('Synthetic preview binding only.\n')
        write_json(self.root/'output/qa.json', {'spec_sha256':sha(self.root/'output/spec.json'),'workbook_sha256':sha(saved),
            'assertions':[{'passed':True}],'tests':[],'baseline':{'amount':7},
            'previews':[{'path':'output/preview.txt','sha256':sha(self.root/'output/preview.txt')}]})

    def check(self):
        env = {'PATH':os.environ.get('PATH',''),'HOME':str(self.root),'WORKBOOK_PYTHON':sys.executable,'PYTHONDONTWRITEBYTECODE':'1'}
        return subprocess.run([str(EXPERT/'bin/check'),'--mechanical'],cwd=self.root,env=env,capture_output=True,text=True,timeout=15)

    def test_public_saved_table_headers_actual_chart_part_and_hash(self):
        self.fixture();result=self.check();self.assertEqual(result.returncode,0,result.stderr)
        facts=json.loads(result.stdout)['saved_native_features']
        self.assertEqual(facts['workbook_sha256'],sha(self.root/'output/workbook.xlsx'))
        self.assertEqual(facts['tables'],[{'sheet':'Records','name':'SyntheticLedger','range':'A1:B3','headers':['Identifier','Amount']}])
        self.assertEqual(facts['charts'],[{'part':CHART_PART,'references':REFERENCES}])
        self.assertEqual(facts['counts'],{'tables':1,'charts':1})
        self.assertEqual(facts['omitted'],{'tables':0,'charts':0});self.assertTrue(facts['complete'])
        self.assertNotIn('cells',facts);self.assertNotIn('xml',facts['charts'][0])
        self.assertEqual(json.loads((self.root/'output/change-report.json').read_text())['saved_native_features'],facts)
        self.assertEqual(json.loads((self.root/'output/result.json').read_text())['saved_native_features'],facts)

    def test_public_empty_features_are_valid_and_complete(self):
        self.fixture(False);result=self.check();self.assertEqual(result.returncode,0,result.stderr)
        facts=json.loads(result.stdout)['saved_native_features']
        self.assertEqual(facts['tables'],[]);self.assertEqual(facts['charts'],[])
        self.assertEqual(facts['counts'],{'tables':0,'charts':0});self.assertTrue(facts['complete'])

    def test_whole_record_budget_preserves_complete_ordered_strings(self):
        huge='É'*20000
        inventory={'sha256':'a'*64,'sheets':[{'name':'Records','tables':[
            {'name':'TooLarge','range':'A1:B2','headers':['Small',huge]},
            {'name':'Kept','range':'D1:E2','headers':['Second','First']}]}],
            'charts':[{'part':CHART_PART,'references':REFERENCES,'xml':'Not included'}]}
        facts=saved_native_features(inventory)
        self.assertLessEqual(len(compact(facts)),16384)
        self.assertFalse(facts['complete']);self.assertEqual(facts['omitted'],{'tables':1,'charts':0})
        self.assertEqual(facts['tables'][0]['headers'],['Second','First'])
        self.assertEqual(facts['charts'][0]['references'],REFERENCES)
        self.assertEqual(inventory['sheets'][0]['tables'][0]['headers'][1],huge)

    def test_stale_output_fails_without_refreshing_old_evidence(self):
        self.fixture();self.assertEqual(self.check().returncode,0)
        old=(self.root/'output/change-report.json').read_bytes()
        manifest=(self.root/'output/result.json').read_bytes()
        with zipfile.ZipFile(self.root/'output/workbook.xlsx','a') as z:z.writestr('stale-test.txt','Changed saved bytes after QA.')
        result=self.check();self.assertEqual(result.returncode,1);self.assertEqual(result.stdout,'')
        self.assertIn('Stale edit output',result.stderr)
        self.assertEqual((self.root/'output/change-report.json').read_bytes(),old)
        self.assertEqual((self.root/'output/result.json').read_bytes(),manifest)
        self.assertNotEqual(json.loads(old)['saved_native_features']['workbook_sha256'],sha(self.root/'output/workbook.xlsx'))

    def test_failed_metric_check_remains_failed_with_observed_facts(self):
        self.fixture();qa_path=self.root/'output/qa.json';qa=json.loads(qa_path.read_text())
        qa['baseline']['amount']=8;write_json(qa_path,qa)
        result=self.check();self.assertEqual(result.returncode,1);self.assertEqual(result.stdout,'')
        self.assertIn('Saved metric amount',result.stderr)
        report=json.loads((self.root/'output/change-report.json').read_text())
        self.assertFalse(report['passed']);self.assertEqual(report['saved_native_features']['workbook_sha256'],sha(self.root/'output/workbook.xlsx'))

    def test_observed_references_are_not_a_requested_binding_verdict(self):
        self.fixture()
        saved=self.root/'output/workbook.xlsx'
        wrong=['Records!$A$2:$A$3','Records!$B$1:$B$2']
        workbook(saved,True,wrong)
        spec_path=self.root/'output/spec.json';spec=json.loads(spec_path.read_text())
        spec['operations'].append({'op':'chart_data','sheet':'Records','index':0,'range':'A1:B3'})
        spec['tests']=[{'name':'Synthetic amount edit','edits':[{'sheet':'Records','cell':'B2','value':8}],'expect':[{'metric':'amount','value':8}]}]
        write_json(spec_path,spec)
        qa_path=self.root/'output/qa.json';qa=json.loads(qa_path.read_text())
        qa.update(spec_sha256=sha(spec_path),workbook_sha256=sha(saved),tests=[{'passed':True}]);write_json(qa_path,qa)
        result=self.check();self.assertEqual(result.returncode,0,result.stderr)
        facts=json.loads(result.stdout)['saved_native_features']
        self.assertEqual(facts['charts'][0]['references'],wrong)
        self.assertIn('not exact requested-chart-binding acceptance',facts['scope'])
        self.assertIn('Chart owner/type',facts['scope'])


if __name__=='__main__':
    unittest.main(verbosity=2)
