"""Synthetic contract probes, never actual world/semantic judge evaluations."""
import copy,json,unittest
from check_test import Checks,HOME
from contracts import *
class Breadth(Checks):
 def test_identity_projection(self):
  d={'roughDescription':'Suspended observatory linked by ramps.','style':'Ink wash'}
  original=copy.deepcopy(d);r=request(d)
  variants=[
   {**d,'style':'Hammered copper'},
   {**d,'performanceRequired':True,'verificationOnly':True},
   dict(reversed(list(d.items()))),
   {**d,'seed':r['seed'],'generationVersion':'1','physicalParameters':{},
    'references':[],'editingRequested':False,'abstractOptIn':False}]
  for v in variants:self.assertEqual(request(v)['worldId'],r['worldId'])
  for key,value in [('seed','new'),('roughDescription','A spiral library.'),('generationVersion','2')]:
   self.assertNotEqual(request({**d,key:value})['worldId'],r['worldId'])
  self.assertEqual(request({**d,'worldId':'explicit'})['worldId'],'explicit')
  self.assertEqual(d,original)
  self.put('request.json','{\n "style":"Ink wash", "roughDescription":"Suspended observatory linked by ramps." }\n')
  before=(self.w/'request.json').read_bytes();request(read(self.w,'request.json'))
  self.assertEqual(before,(self.w/'request.json').read_bytes())
 def test_diverse_contracts_and_optional_population(self):
  cases=[
   ('Suspended observatory linked by ramps.','Ink wash',None),
   ('Tidal grotto with rays swimming under the shelf.','Glazed ceramic','rays swimming'),
   ('Orchard terraces with windblown leaves.','Woven textile',None),
   ('Transit concourse with walkers following platforms.','Charcoal etching','walkers following platforms')]
  for description,style,life in cases:
   with self.subTest(description=description):
    self.r={'roughDescription':description,'style':style};self.fixture()
    self.b['features'][0]['requestSpan']=description
    self.b['composition']={'largeForms':['layered connected volumes'],
     'landmarks':['focal silhouette'],'relationships':['clear connected access'],
     'detail':['domain-specific intermediate forms'],'atmosphere':['depth with readable contrast']}
    if life:self.b['population']={'maxActive':12,'groups':[{'id':'local-life','requestSpan':life,
     'representation':'coherent articulated silhouette','habitat':'bounded accessible region',
     'motion':'seeded local route at simulation time','maxActive':12}]}
    self.put('phase-01/blueprint.json',self.b);self.bind();self.receipt();self.check(0)
    # Independent bad labels, not verdict-derived ground truth or semantic classifier.
    for criterion in (['C03','C04','C08']+(['C01'] if life else [])):
     self.receipt({criterion:'violated'});self.check(1)
    if life:
     valid=copy.deepcopy(self.b)
     for mutation in ('overcap','unbound','missing-motion','duplicate'):
      self.b=copy.deepcopy(valid);g=self.b['population']['groups'][0]
      if mutation=='overcap':g['maxActive']=13
      if mutation=='unbound':g['requestSpan']='absent request span'
      if mutation=='missing-motion':del g['motion']
      if mutation=='duplicate':self.b['population']['groups'].append(copy.deepcopy(g))
      self.put('phase-01/blueprint.json',self.b);self.bind();self.receipt();self.check(1)
 def test_dependency_profiles(self):
  for name in ('base','effects','spatial','effects-spatial'):
   folder=HOME/('templates/runtime' if name=='base' else 'templates/profiles/'+name)
   p=load(folder/'package.json');l=load(folder/'package-lock.json')
   self.assertEqual(dependencies(p,l,HOME,'webgl2'),name)
   self.fixture()
   self.put('phase-03/runtime/package.json',p);self.put('phase-03/runtime/package-lock.json',l)
   self.bind();self.receipt();self.check(0)
   if 'effects' in name:
    with self.assertRaises(Reject):dependencies(p,l,HOME,'webgpu')
   else:self.assertEqual(dependencies(p,l,HOME,'webgpu'),name)
   for fault in ('pin','integrity','extra-package','extra-dev','unknown-profile','missing-graph'):
    badp=copy.deepcopy(p);badl=copy.deepcopy(l)
    if fault=='pin':badp['dependencies']['three']='^0.186.0'
    if fault=='integrity':badl['packages']['node_modules/three']['integrity']='fake'
    if fault=='extra-package':badp['dependencies']['unchecked']='1.0.0'
    if fault=='extra-dev':badp['devDependencies']={'unchecked':'1.0.0'}
    if fault=='unknown-profile':badp['worldweaverProfile']='../../unchecked'
    if fault=='missing-graph':del badl['packages']['node_modules/three']
    with self.assertRaises(Reject,msg=name+':'+fault):dependencies(badp,badl,HOME,'webgl2')
 def test_omissions_keep_independent_hard_review(self):
  self.fixture()
  for criteria in ({'C01':'violated','C03':'violated'}, {'C03':'violated'},
                   {'C04':'violated'}, {'C08':'violated'},
                   {'C08':'violated','C09':'violated','C10':'violated'}):
   self.receipt(criteria);self.check(1)
  for cid in ('C01','C03','C04','C06'):
   self.receipt({cid:'insufficient_evidence'});self.check(1)
def load_tests(loader,tests,pattern):
 return unittest.TestSuite(Breadth(n) for n in Breadth.__dict__ if n.startswith('test_'))
if __name__=='__main__':unittest.main()
