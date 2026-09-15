#!/usr/bin/env python3
"""Product Manager's reused byte envelope and new decision-framing contract."""
import copy,hashlib,json,os,subprocess,sys,tempfile,unittest
from pathlib import Path
EXPERT=Path(os.environ['PRODUCT_MANAGER_EXPERT']).resolve()


class ProductManagerTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.root=Path(self.temp.name).resolve()
        (self.root/'inputs').mkdir();(self.root/'output').mkdir()
        (self.root/'request.md').write_text('Fictional strategic option evaluation.')
        (self.root/'inputs/evidence.md').write_text('Fictional customer need and option evidence; no budget authorization.')
        (self.root/'output/response.md').write_text('A reviewable evaluation framework with explicit evidence gaps.')
        h=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
        self.document={'schema':'bench.product-manager/v1','mode':'evaluation','status':'ready','request_sha256':h(self.root/'request.md'),'response_sha256':h(self.root/'output/response.md'),'inputs':[{'path':'inputs/evidence.md','sha256':h(self.root/'inputs/evidence.md')}],'summary':'Frame a fictional strategic decision.','recommendation':'Evaluate against stated needs before choosing.','assumptions':[],'questions':[],'next_actions':[{'owner':'Analyst','action':'Assess evidence','acceptance':'Method limits are clear'}], 'details':{'problem':'A supplied customer problem','customers':['Internal service users'],'desired_outcome':'Reduce documented service friction','decision_scope':'Compare the two admitted options','strategic_fit':'Alignment to the supplied outcome; corporate roadmap unknown','economic_tradeoffs':'Prices and lifecycle costs require evidence','lifecycle':['Validate adoption, support and exit implications'],'candidate_ids':['a','b'],'constraints':['Mandatory requirement'],'weight_basis':'proposed','criteria':[{'id':c,'name':c,'weight':w,'rationale':'Explicit business rationale'} for c,w in [('fit',50),('economics',30),('lifecycle',20)]],'evidence_plan':[{'question':'What cost is supported?','owner':'Finance','decision_impact':'Could change the decision'}],'decision_rule':'Advance with validated requirements; stop on failed gate','delivery_handoff':'PO and team refine outcomes after a decision; no capacity estimate','selection':{'status':'not-assessed','candidate_id':None,'rationale':'Specialists have not assessed options.'}}}
    def tearDown(self):self.temp.cleanup()
    def check(self):
        (self.root/'output/decision.json').write_text(json.dumps(self.document)+'\n')
        return subprocess.run([str(EXPERT/'bin/check')],cwd=self.root,capture_output=True,text=True)
    def accepted(self):
        r=self.check();self.assertEqual(r.returncode,0,r.stderr)
    def rejected(self,phrase=None):
        r=self.check();self.assertEqual(r.returncode,1,r.stderr)
        if phrase:self.assertIn(phrase,r.stderr)
    def test_evaluation_accepted(self):self.accepted()
    def test_strategy_without_weighted_framework(self):
        self.document['mode']='strategy';self.document['details'].update(criteria=[],candidate_ids=[],weight_basis='not-applicable');self.accepted()
    def test_synthesis_accepted(self):
        self.document['mode']='synthesis';self.document['details']['selection'].update(status='conditional',candidate_id='a');self.accepted()
    def test_synthesis_can_defer(self):
        self.document['mode']='synthesis';self.document['details']['selection']['status']='defer';self.accepted()
    def test_unknown_problem_needs_input(self):
        self.document['status']='needs-input';self.document['questions']=['Which customer problem is in scope?'];self.document['details'].update(problem='',customers=[],desired_outcome='',criteria=[],candidate_ids=[],weight_basis='not-applicable');self.accepted()
    def test_framing_cannot_preselect_winner(self):
        self.document['details']['selection'].update(status='recommend',candidate_id='a');self.rejected('framing modes')
    def test_synthesis_requires_decision(self):
        self.document['mode']='synthesis';self.rejected('synthesis must')
    def test_unknown_selected_candidate(self):
        self.document['mode']='synthesis';self.document['details']['selection'].update(status='recommend',candidate_id='invented');self.rejected('admitted candidate')
    def test_defer_cannot_name_winner(self):
        self.document['mode']='synthesis';self.document['details']['selection'].update(status='defer',candidate_id='a');self.rejected('null candidate')
    def test_ready_requires_customer(self):
        self.document['details']['customers']=[];self.rejected('customers')
    def test_ready_requires_lifecycle(self):
        self.document['details']['lifecycle']=[];self.rejected('lifecycle')
    def test_no_fake_weights(self):
        for value in [True,0,-1,float('inf'),float('nan'),40]:
            with self.subTest(value=value):
                self.document['details']['criteria'][0]['weight']=value;self.rejected()
    def test_criteria_ids_unique(self):
        self.document['details']['criteria'][0]['id']='economics';self.rejected('duplicate criterion')
    def test_stale_request_response_and_input(self):
        for name in ['request.md','output/response.md','inputs/evidence.md']:
            with self.subTest(name=name):
                p=self.root/name;old=p.read_bytes();p.write_bytes(old+b'changed');self.rejected();p.write_bytes(old)
    def test_missing_or_unsorted_inputs(self):
        self.document['inputs']=[];self.rejected('inputs list')
    def test_old_product_owner_contract_rejected(self):
        self.document['schema']='bench.product-owner/v2';self.rejected('schema')
    def test_missing_strategy_field(self):
        del self.document['details']['strategic_fit'];self.rejected('keys invalid')
    def test_needs_input_requires_question(self):
        self.document['status']='needs-input';self.rejected('blocking question')
    def test_unexpected_output(self):
        (self.root/'output/extra.txt').write_text('extra');self.rejected('exactly')
    def test_symlink_input_refused(self):
        (self.root/'inputs/link.md').symlink_to(self.root/'request.md');self.rejected('symlink')
    def test_duplicate_json_keys_refused(self):
        self.accepted();p=self.root/'output/decision.json';raw=p.read_text();p.write_text(raw.replace('"mode": "evaluation"','"mode": "evaluation", "mode": "synthesis"'))
        r=subprocess.run([str(EXPERT/'bin/check')],cwd=self.root,capture_output=True,text=True);self.assertEqual(r.returncode,1);self.assertIn('duplicate JSON',r.stderr)


if __name__=='__main__':unittest.main(verbosity=2)
