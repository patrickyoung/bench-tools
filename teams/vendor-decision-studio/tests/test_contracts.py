"""Synthetic publication boundary tests; no model calls or real customer data."""
import copy,json,sys,tempfile,unittest,subprocess
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'expert/tools'))
from publication_contract import bindings,validate,read,safe_file,display_number
from publication_io import check_visual
def write(p,v):p.parent.mkdir(parents=True,exist_ok=True);p.write_text(json.dumps(v))
class PublicationTests(unittest.TestCase):
 def setUp(self):
  self.tmp=tempfile.TemporaryDirectory();self.root=Path(self.tmp.name)
  self.story={'title':'Conditional choice','subtitle':'Synthetic test','audience':'Decision makers','thesis':'Evidence supports conditional further work.','messages':[{'id':f'm{i}','text':'A supported finding','qualification':'Subject to the supplied limits','fact_ids':['decision']} for i in range(3)],'required_message_ids':['m0','m1'],'terminology':['Conditional'],'design':{'direction':'Clear evidence','font':'Arial','ink':'#112233','accent':'#456789','secondary':'#345678','paper':'#FFFFFF'},'report_arc':['a']*5,'deck_arc':['a']*7}
  self.src={'facts':[{'id':'decision'}],'statistics':{'comparisons':[]}}
 def tearDown(self):self.tmp.cleanup()
 def setup_role(self,role,content):
  write(self.root/'request.json',{'role':role});write(self.root/'inputs/source.json',self.src)
  if role!='editorial-director':write(self.root/'inputs/story.json',{'content':self.story})
  if role=='publication-reviewer':write(self.root/'inputs/visual-review.json',{'verdict':'pass'})
  self.save(role,content)
 def save(self,role,c):write(self.root/'output/spec.json',{**bindings(self.root,role),'content':c})
 def check(self,role):return validate(self.root,role,False)
 def review(self):return {'verdict':'publish','summary':'All outputs checked','rubric':{k:4 for k in ['evidence_fidelity','narrative_coherence','audience_usefulness','writing_quality','visual_craft','accessibility']},'findings':[],'cross_format_checks':['Sources agree']*5}
 def test_editorial_valid(self):self.setup_role('editorial-director',self.story);self.check('editorial-director')
 def test_small_nonzero_estimates_never_display_as_zero(self):
  for value in [-0.000001234,0.000001234]:
   shown=float(display_number(value));self.assertGreater(shown/value,0);self.assertLess(abs(shown-value)/abs(value),.005)
  self.assertEqual(float(display_number(0.0)),0.0)
 def test_rebuild_does_not_overwrite_existing_run_status(self):
  status=self.root/'status.json';status.write_text('original retained status')
  command=Path(__file__).resolve().parents[1]/'expert/tools/rebuild_publication.py'
  result=subprocess.run([sys.executable,str(command),str(self.root/'unused-old'),str(self.root)],capture_output=True,text=True)
  self.assertNotEqual(result.returncode,0);self.assertEqual(status.read_text(),'original retained status')
 def test_stale_request(self):
  self.setup_role('editorial-director',self.story);write(self.root/'request.json',{'role':'editorial-director','audience':'changed'})
  with self.assertRaisesRegex(ValueError,'Stale'):self.check('editorial-director')
 def test_stale_source(self):
  self.setup_role('editorial-director',self.story);write(self.root/'inputs/source.json',{'facts':[{'id':'new'}]})
  with self.assertRaisesRegex(ValueError,'Stale'):self.check('editorial-director')
 def test_added_unselected_input(self):
  self.setup_role('editorial-director',self.story);write(self.root/'inputs/extra.json',{})
  with self.assertRaisesRegex(ValueError,'Stale'):self.check('editorial-director')
 def test_invented_fact(self):
  self.story['messages'][0]['fact_ids']=['invented'];self.setup_role('editorial-director',self.story)
  with self.assertRaisesRegex(ValueError,'Unknown fact'):self.check('editorial-director')
 def test_duplicate_message(self):
  self.story['messages'][1]['id']='m0';self.setup_role('editorial-director',self.story)
  with self.assertRaisesRegex(ValueError,'Duplicate'):self.check('editorial-director')
 def test_unavailable_font(self):
  self.story['design']['font']='Invented Font';self.setup_role('editorial-director',self.story)
  with self.assertRaisesRegex(ValueError,'font'):self.check('editorial-director')
 def test_no_inference_without_data(self):
  vs=[{'id':f'v{i}','kind':k,'title':'A clear graphic','subtitle':'Qualified view','caption':'Read the limitations','alt':'Accessible description','fact_ids':['decision'],'message_ids':['m0'],'steps':[{'heading':'Validate','detail':'Resolve the evidence gap','fact_ids':['decision'],'message_ids':['m0']}]*3 if k=='decision_path' else []} for i,k in enumerate(['score_bounds','decision_path','effects'])]
  self.setup_role('information-designer',{'title':'Comparison','workbook_intro':'A qualified comparison','visuals':vs})
  with self.assertRaisesRegex(ValueError,'No inferential'):self.check('information-designer')
 def test_review_valid(self):self.setup_role('publication-reviewer',self.review());self.check('publication-reviewer')
 def threshold_spec(self,value=2):
  self.src['statistics']['comparisons']=[{'id':'sample-test','status':'inferential'}]
  vs=[{'id':f'v{i}','kind':k,'title':'A clear graphic','subtitle':'Qualified view','caption':'Read the limitations','alt':'Accessible description','fact_ids':['decision'],'message_ids':['m0'],'steps':[{'heading':'Validate','detail':'Resolve the evidence gap','fact_ids':['decision'],'message_ids':['m0']}]*3 if k=='decision_path' else []} for i,k in enumerate(['score_bounds','decision_path','effects'])]
  vs[2]['reference_lines']=[{'comparison_id':'sample-test','value':value,'label':'Supplied practical threshold','fact_ids':['decision']}]
  return {'title':'Comparison','workbook_intro':'Qualified result','visuals':vs}
 def test_sourced_numeric_threshold_contract(self):
  self.setup_role('information-designer',self.threshold_spec(-2));self.check('information-designer')
 def test_threshold_cannot_be_free_text_value(self):
  self.setup_role('information-designer',self.threshold_spec('2 ms'))
  with self.assertRaisesRegex(ValueError,'threshold value'):self.check('information-designer')
 def test_threshold_cannot_refer_to_nonexistent_inference(self):
  c=self.threshold_spec();c['visuals'][2]['reference_lines'][0]['comparison_id']='invented';self.setup_role('information-designer',c)
  with self.assertRaisesRegex(ValueError,'inferential comparison'):self.check('information-designer')
 def test_cannot_average_away_bad_score(self):
  c=self.review();c['rubric']['visual_craft']=3;self.setup_role('publication-reviewer',c)
  with self.assertRaisesRegex(ValueError,'quality gate'):self.check('publication-reviewer')
 def test_material_defect_blocks_publish(self):
  c=self.review();c['findings']=[{'severity':'material','artifact':'deck','location':'slide 2','issue':'Missing uncertainty','correction':'Restore uncertainty'}];self.setup_role('publication-reviewer',c)
  with self.assertRaisesRegex(ValueError,'quality gate'):self.check('publication-reviewer')
 def test_visual_failure_blocks_publish(self):
  c=self.review();self.setup_role('publication-reviewer',c);write(self.root/'inputs/visual-review.json',{'verdict':'revise'});self.save('publication-reviewer',c)
  with self.assertRaisesRegex(ValueError,'quality gate'):self.check('publication-reviewer')
 def test_revision_can_report_low_quality(self):
  c=self.review();c['verdict']='revise';c['rubric']['visual_craft']=2;self.setup_role('publication-reviewer',c);self.check('publication-reviewer')
 def test_unknown_severity(self):
  c=self.review();c['findings']=[{'severity':'maybe','artifact':'deck','location':'2','issue':'Bad','correction':'Fix'}];self.setup_role('publication-reviewer',c)
  with self.assertRaisesRegex(ValueError,'severity'):self.check('publication-reviewer')
 def test_json_duplicate(self):
  p=self.root/'duplicate.json';p.write_text('{"a":1,"a":2}')
  with self.assertRaisesRegex(ValueError,'Duplicate'):read(p)
 def test_json_nonfinite(self):
  p=self.root/'bad.json';p.write_text('{"a":NaN}')
  with self.assertRaisesRegex(ValueError,'Nonfinite'):read(p)
 def test_escape_path(self):
  with self.assertRaisesRegex(ValueError,'Unsafe'):safe_file(self.root,'../external')
 def test_symlink(self):
  p=self.root/'real';p.write_text('x');(self.root/'link').symlink_to(p)
  with self.assertRaisesRegex(ValueError,'Symlink'):safe_file(self.root,'link')
 def visual(self):
  from publication_contract import sha
  write(self.root/'control/visual-inputs.json',[{'id':'a','sha256':'abc'}])
  c={'input_sha256':sha(self.root/'control/visual-inputs.json'),'verdict':'pass','images':[{'id':'a','image_sha256':'abc','verdict':'pass','rubric':{k:4 for k in ['legibility','hierarchy','composition','chart_integrity','consistency']},'observations':['Synthetic fixture: labeled chart fits its plot bounds.'],'findings':[]}]};return c
 def test_visual_complete(self):write(self.root/'control/visual-review.json',self.visual());check_visual(self.root)
 def test_visual_empty_observations(self):
  c=self.visual();c['images'][0]['observations']=[];write(self.root/'control/visual-review.json',c)
  with self.assertRaisesRegex(ValueError,'inspection observations'):check_visual(self.root)
 def test_visual_omitted_page(self):
  c=self.visual();c['images']=[];write(self.root/'control/visual-review.json',c)
  with self.assertRaisesRegex(ValueError,'Incomplete'):check_visual(self.root)
 def test_visual_wrong_pixels(self):
  c=self.visual();c['images'][0]['image_sha256']='changed';write(self.root/'control/visual-review.json',c)
  with self.assertRaisesRegex(ValueError,'Incomplete'):check_visual(self.root)
 def test_visual_forged_pass(self):
  c=self.visual();c['images'][0]['rubric']['legibility']=2;write(self.root/'control/visual-review.json',c)
  with self.assertRaisesRegex(ValueError,'False image pass'):check_visual(self.root)
 def test_visual_forged_aggregate(self):
  c=self.visual();c['images'][0]['verdict']='revise';write(self.root/'control/visual-review.json',c)
  with self.assertRaisesRegex(ValueError,'False visual'):check_visual(self.root)
if __name__=='__main__':unittest.main()
