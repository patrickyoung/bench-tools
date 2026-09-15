#!/usr/bin/env python3
"""Guard against PM scope changes and confidence upgrades at the team boundary."""
import copy,importlib.util,os,unittest
from pathlib import Path
TEAM=Path(os.environ['COMPARISON_TEAM_EXPERT']).resolve()
spec=importlib.util.spec_from_file_location('pm_team_io',TEAM/'tools/team_io.py');io=importlib.util.module_from_spec(spec);spec.loader.exec_module(io)
class LeadHandoffTests(unittest.TestCase):
    def setUp(self):
        self.packet={'job':{'candidates':[{'id':'a'},{'id':'b'}],'gates':[{'requirement':'Current SAML SSO support'}],'criteria':[{'id':c,'name':c,'weight':w} for c,w in [('fit',50),('economics',30),('lifecycle',20)]]}}
        self.lead={'details':{'candidate_ids':['a','b'],'constraints':['Current SAML SSO support'],'criteria':copy.deepcopy(self.packet['job']['criteria']),'weight_basis':'supplied','selection':{'status':'not-assessed','candidate_id':None,'rationale':'Await evidence'}}}
        self.comparison={'criteria':copy.deepcopy(self.packet['job']['criteria']),'weight_basis':'supplied','recommendation':{'status':'conditional','candidate_id':'a'}}
    def validate(self,synthesis=False):io.validate_lead_scope(self.lead,self.packet,self.comparison if synthesis else None)
    def test_scope_accepted(self):self.validate()
    def test_candidate_added_rejected(self):
        self.lead['details']['candidate_ids'].append('c')
        with self.assertRaisesRegex(ValueError,'candidate scope'):self.validate()
    def test_gate_omission_rejected(self):
        self.lead['details']['constraints']=[]
        with self.assertRaisesRegex(ValueError,'mandatory gate'):self.validate()
    def test_supplied_weight_change_rejected(self):
        self.lead['details']['criteria'][0]['weight']=40;self.lead['details']['criteria'][1]['weight']=40
        with self.assertRaisesRegex(ValueError,'criteria or weights'):self.validate()
    def test_proposed_basis_cannot_misstate_supplied(self):
        self.lead['details']['weight_basis']='proposed'
        with self.assertRaisesRegex(ValueError,'basis'):self.validate()
    def test_synthesis_keeps_conditional(self):
        self.lead['details']['selection'].update(status='conditional',candidate_id='a');self.validate(True)
    def test_synthesis_cannot_upgrade_confidence(self):
        self.lead['details']['selection'].update(status='recommend',candidate_id='a')
        with self.assertRaisesRegex(ValueError,'upgraded'):self.validate(True)
    def test_synthesis_cannot_switch_winner(self):
        self.lead['details']['selection'].update(status='conditional',candidate_id='b')
        with self.assertRaisesRegex(ValueError,'changed'):self.validate(True)
    def test_synthesis_can_defer_for_revision(self):
        self.lead['details']['selection'].update(status='defer',candidate_id=None);self.validate(True)
    def test_synthesis_keeps_actual_proposed_framework(self):
        self.packet['job']['criteria']=[];self.lead['details']['weight_basis']='proposed';self.comparison['weight_basis']='proposed';self.lead['details']['selection'].update(status='conditional',candidate_id='a');self.validate(True)

if __name__=='__main__':unittest.main(verbosity=2)
