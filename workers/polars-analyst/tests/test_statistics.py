#!/usr/bin/env python3
"""Numerical references, design refusals, binding and Polars data auditing.
Reference p/critical values use a stdlib-only Student-t CDF quadrature,
not another call to the implementation's SciPy functions.
"""
import copy
import csv
import importlib.util
import io
import json
import math
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

EXPERT = Path(os.environ['POLARS_ANALYST_EXPERT']).resolve()
spec = importlib.util.spec_from_file_location('analyst_engine', EXPERT/'tools/analyze.py')
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)


def t_cdf(x, df):
    # Composite Simpson integration of the t density over [0,abs(x)].
    n = 10000
    h = abs(x) / n
    constant = math.exp(math.lgamma((df+1)/2)-math.lgamma(df/2)) / math.sqrt(df*math.pi)
    def density(t):
        return constant * (1+t*t/df)**(-(df+1)/2)
    total = density(0) + density(abs(x))
    total += sum((4 if i%2 else 2)*density(i*h) for i in range(1,n))
    area = total*h/3
    return .5 + (area if x >= 0 else -area)


def critical(df):
    lo,hi = 0.,20.
    for _ in range(45):
        mid=(lo+hi)/2
        if t_cdf(mid,df)<.975: lo=mid
        else: hi=mid
    return (lo+hi)/2


class StatisticsTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory()
        self.root=Path(self.temp.name).resolve()
        (self.root/'inputs').mkdir()
        (self.root/'output').mkdir()
        self.request={'schema':'bench.polars-analysis-request/v1','business_case':'Fictional numerical verification in original milliseconds.','groups':['a','b'],'confidence_level':.95,'datasets':[{'id':'sample','path':'inputs/data.csv','sha256':'','group_column':'group','unit_column':'unit','metrics':[{'id':'latency','column':'value','unit':'ms','measurement_level':'continuous'}]}],'comparisons':[{'id':'contrast','metric_id':'latency','groups':['a','b'],'method':'welch','design':{'independent_units':True,'sampling_basis':'Synthetic independent sampling model with distinct experimental units.','normality_basis':'Known normal generating model for this numerical fixture.'}}]}
        self.plan={'schema':'bench.polars-analysis-plan/v1','request_sha256':'','assessment':'Synthetic verified numerical fixture; source authenticity and production generalization are outside this test.','decisions':[{'id':'contrast','action':'infer','design_supported':True,'rationale':'Declared synthetic generating assumptions are supported for this test.'}],'questions':[]}
        self.rows=[['a',f'a{i}',x] for i,x in enumerate([2,4,6,8,10])]+[['b',f'b{i}',x] for i,x in enumerate([3,5,8,11,15,19])]

    def tearDown(self): self.temp.cleanup()

    def bind(self, parquet=False):
        if parquet:
            self.request['datasets'][0]['path']='inputs/data.parquet'
            a.pl.DataFrame(self.rows,schema=['group','unit','value'],orient='row').write_parquet(self.root/'inputs/data.parquet')
        else:
            out=io.StringIO(); writer=csv.writer(out);writer.writerow(['group','unit','value']);writer.writerows(self.rows)
            (self.root/'inputs/data.csv').write_text(out.getvalue())
        path=self.root/self.request['datasets'][0]['path']
        self.request['datasets'][0]['sha256']=a.digest(path.read_bytes())
        raw=a.dump(self.request); (self.root/'request.json').write_bytes(raw)
        self.plan['request_sha256']=a.digest(raw)
        (self.root/'output/analysis-plan.json').write_bytes(a.dump(self.plan))
        return a.compute(self.request,self.plan,self.root,a.digest(raw),a.digest(a.dump(self.plan)))

    def result(self): return self.bind()['comparisons'][0]

    def refused(self, phrase):
        r=self.result();self.assertEqual(r['status'],'descriptive-only');self.assertIsNone(r['p_value'])
        self.assertIn(phrase,' '.join(r['refusal_reasons']))

    def test_welch_independent_math_and_quadrature(self):
        r=self.result()
        x=[2,4,6,8,10];y=[3,5,8,11,15,19]
        avg=lambda z:sum(z)/len(z)
        var=lambda z:sum((v-avg(z))**2 for v in z)/(len(z)-1)
        vx,vy=var(x)/len(x),var(y)/len(y)
        se=math.sqrt(vx+vy);diff=avg(x)-avg(y)
        df=(vx+vy)**2/(vx**2/(len(x)-1)+vy**2/(len(y)-1))
        self.assertAlmostEqual(r['mean_difference_a_minus_b'],diff,12)
        self.assertAlmostEqual(r['statistic'],diff/se,12)
        self.assertAlmostEqual(r['df'],df,12)
        self.assertAlmostEqual(r['p_value'],2*(1-t_cdf(abs(diff/se),df)),10)
        self.assertAlmostEqual(r['ci_low'],diff-critical(df)*se,9)
        self.assertAlmostEqual(r['ci_high'],diff+critical(df)*se,9)

    def test_paired_join_by_id_shuffled_order(self):
        self.request['comparisons'][0]['method']='paired'
        self.rows=[['a',str(i),10+i] for i in range(1,6)]+[['b',str(i),10] for i in [4,1,5,2,3]]
        r=self.result();se=math.sqrt(2.5/5)
        self.assertEqual(r['paired_count'],5);self.assertEqual(r['df'],4)
        self.assertEqual(r['mean_difference_a_minus_b'],3)
        self.assertAlmostEqual(r['statistic'],3/se,12)
        self.assertAlmostEqual(r['p_value'],2*(1-t_cdf(3/se,4)),10)
        self.assertAlmostEqual(r['ci_low'],3-critical(4)*se,9)
        self.assertAlmostEqual(r['ci_high'],3+critical(4)*se,9)

    def test_holm_order_and_monotonicity(self):
        self.assertEqual(a.holm([.01,.04,.03]),[.03,.06,.06])
        self.assertEqual(a.holm([1,.02,.02]),[1,.06,.06])
        self.assertEqual(a.holm([]),[])

    def test_full_family_includes_refused_tests(self):
        second=copy.deepcopy(self.request['comparisons'][0]);second['id']='refused'
        self.request['comparisons'].append(second)
        self.plan['decisions'].append({'id':'refused','action':'describe','design_supported':False,'rationale':'Unsupported design in this separate planned comparison.'})
        value=self.bind();r=value['comparisons'][0]
        self.assertEqual(value['holm_family_size'],2)
        self.assertAlmostEqual(r['p_holm'],min(1,2*r['p_value']),12)
        self.assertIsNone(value['comparisons'][1]['p_holm'])
        self.assertEqual(r['ci_scope'],'marginal, not multiplicity-adjusted')

    def test_data_quality_counts_no_zero_fill(self):
        self.rows=[['a',str(i),v] for i,v in enumerate([1,3,'','bad','NaN','inf','-inf'])]+[['b','b1',5],['b','b2',6]]
        value=self.bind();p=value['profiles'][0]['groups'][0]
        self.assertEqual((p['n_rows'],p['n_valid'],p['n_units'],p['mean']), (7,2,7,2))
        self.assertEqual((p['missing'],p['invalid_numeric'],p['nonfinite'],p['nan'],p['positive_infinity'],p['negative_infinity']),(1,1,3,1,1,1))
        self.assertAlmostEqual(p['sample_sd'],math.sqrt(2),12)
        self.assertEqual((p['q25'],p['q75'],p['p95']),(1.5,2.5,2.9))
        self.assertEqual(value['comparisons'][0]['status'],'descriptive-only')

    def test_repeated_unit_pseudoreplication(self):
        self.rows[1][1]=self.rows[0][1]
        value=self.bind();p=value['profiles'][0]['groups'][0]
        self.assertEqual((p['n_rows'],p['n_units'],p['duplicate_unit_rows']),(5,4,1))
        self.refused('pseudoreplication')

    def test_unmatched_pairs_refused(self):
        self.request['comparisons'][0]['method']='paired'
        self.refused('Paired IDs differ')

    def test_shared_ids_not_independent(self):
        self.rows[5][1]='a0';self.refused('Shared unit IDs')

    def test_singleton(self):
        self.rows=[['a','a1',2],['b','b1',3]];self.refused('Fewer than two')

    def test_zero_sampling_variance(self):
        self.rows=[['a','a1',2],['a','a2',2],['b','b1',3],['b','b2',3]];self.refused('zero sampling variance')

    def test_ordinal_refused(self):
        self.request['datasets'][0]['metrics'][0]['measurement_level']='ordinal';self.refused('Ordinal')

    def test_design_false(self):
        self.request['comparisons'][0]['design']['independent_units']=False;self.refused('not supported')

    def test_basis_missing(self):
        self.request['comparisons'][0]['design']['normality_basis']='unknown';self.refused('basis absent')

    def test_analyst_can_downgrade(self):
        self.plan['decisions'][0]['action']='describe';self.refused('analyst decision')

    def test_descriptive_cannot_upgrade(self):
        self.request['comparisons'][0]['method']='descriptive'
        with self.assertRaisesRegex(ValueError,'cannot upgrade'):self.bind()

    def test_empty_request_valid(self):
        self.request['datasets']=[];self.request['comparisons']=[];self.plan['decisions']=[]
        raw=a.dump(self.request);self.plan['request_sha256']=a.digest(raw)
        value=a.compute(self.request,self.plan,self.root,a.digest(raw),a.digest(a.dump(self.plan)))
        self.assertEqual(value['profiles'],[]);self.assertEqual(value['comparisons'],[])
        self.assertIn(b'No observational dataset',a.report(value))

    def test_parquet_csv_equivalence(self):
        csv_value=self.bind();parquet_value=self.bind(parquet=True)
        self.assertEqual(csv_value['profiles'],parquet_value['profiles'])
        self.assertEqual(csv_value['comparisons'],parquet_value['comparisons'])

    def test_preserve_leading_zero_ids(self):
        self.rows[0][1]='001';self.rows[1][1]='1'
        value=self.bind();self.assertEqual(value['profiles'][0]['groups'][0]['n_units'],5)

    def test_unknown_groups_and_missing_ids_rejected(self):
        self.rows[0][0]='other'
        with self.assertRaisesRegex(ValueError,'unrecognized group'):self.bind()
        self.rows[0][0]='a';self.rows[0][1]=''
        with self.assertRaisesRegex(ValueError,'missing group/unit'):self.bind()

    def test_wrong_dataset_hash_rejected(self):
        self.bind();self.request['datasets'][0]['sha256']='0'*64
        with self.assertRaisesRegex(ValueError,'dataset changed'):a.profile(self.request,self.root)

    def test_plan_hash_and_admission_binding(self):
        self.bind();self.plan['request_sha256']='0'*64
        with self.assertRaisesRegex(ValueError,'stale request'):a.validate_plan(self.request,self.plan,a.digest(a.dump(self.request)))
        self.plan['request_sha256']=a.digest(a.dump(self.request))
        from unittest.mock import patch
        with patch.dict(os.environ,{'POLARS_REQUEST_SHA256':'0'*64}):
            with self.assertRaisesRegex(ValueError,'admitted request changed'):a.validate_plan(self.request,self.plan,self.plan['request_sha256'])

    def test_render_check_and_tamper(self):
        self.bind();command=[sys.executable,str(EXPERT/'tools/analyze.py')]
        for action in ['render','check']:
            outcome=subprocess.run(command+[action],cwd=self.root,capture_output=True)
            self.assertEqual(outcome.returncode,0,outcome.stderr.decode())
        value=json.loads((self.root/'output/statistics.json').read_text())
        self.assertEqual(value['plan_sha256'],a.digest((self.root/'output/analysis-plan.json').read_bytes()))
        (self.root/'output/report.md').write_text('Unsupported replacement')
        self.assertNotEqual(subprocess.run(command+['check'],cwd=self.root,capture_output=True).returncode,0)

    def test_json_ambiguity_rejected(self):
        for raw in [b'{"x":1,"x":2}',b'{"x":NaN}']:
            with self.assertRaises(ValueError):a.parse(raw)

    def test_row_limit_no_silent_truncation(self):
        self.rows=[['a',str(i),1] for i in range(100001)]
        with self.assertRaisesRegex(ValueError,'100000 rows'):self.bind()


if __name__=='__main__': unittest.main(verbosity=2)
