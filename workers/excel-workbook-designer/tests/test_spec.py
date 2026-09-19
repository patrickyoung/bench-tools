"""Portable text-only contract regression cases; no model or workbook authoring."""
import importlib.util,json,shutil,tempfile,unittest
from pathlib import Path
BASE=Path(__file__).resolve().parents[1]
loader=importlib.util.spec_from_file_location('contract',BASE/'expert/lib/contract.py');m=importlib.util.module_from_spec(loader);loader.loader.exec_module(m)
class SpecTests(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.root=Path(self.tmp.name)/'case';shutil.copytree(BASE/'tests/fixture',self.root)
        (self.root/'output').mkdir()
        for name in ['spec.json','guide.md']:(self.root/name).rename(self.root/'output'/name)
    def tearDown(self):self.tmp.cleanup()
    def edit(self,fn):
        p=self.root/'output/spec.json';s=json.loads(p.read_text());fn(s);p.write_text(json.dumps(s))
    def rejects(self):
        with self.assertRaises(Exception):m.validate_spec(self.root)
    def test_valid(self):m.validate_spec(self.root)
    def test_stale_source(self):(self.root/'inputs/data.csv').write_text('changed');self.rejects()
    def test_stale_request(self):self.edit(lambda s:s.update(request_sha256='0'*64));self.rejects()
    def test_no_lineage(self):self.edit(lambda s:s.update(lineage=[]));self.rejects()
    def test_uncovered_control(self):self.edit(lambda s:s.update(tests=[s['tests'][0]]*3));self.rejects()
    def test_missing_metric(self):self.edit(lambda s:s.update(metrics=[]));self.rejects()
    def test_static_metric(self):
        def fn(s):b=s['sheets'][0]['blocks'][3];b.pop('formulas');b['values']=[[30]]
        self.edit(fn);self.rejects()
    def test_formula_overwrite(self):self.edit(lambda s:s['tests'][0]['edits'].append({'sheet':'Summary','cell':'B8','value':999}));self.rejects()
    def test_missing_growth_policy(self):self.edit(lambda s:s['update_policy'].pop('capacity'));self.rejects()
    def test_chart_overlap(self):self.edit(lambda s:s['sheets'][0]['charts'][0].update({'from':'A2','to':'D20'}));self.rejects()
if __name__=='__main__':unittest.main(verbosity=2)
