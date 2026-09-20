import assert from 'node:assert/strict';
import {ShaderLib,ShaderChunk,Matrix4,Object3D,Vector3} from 'three';
import {terrain,support,terrainSurface,at,offset,roadPoint,generate,treesForChunk} from '../src/generation.js';
import {Atmosphere,localParticle} from '../src/atmosphere.js';
import {bindKey} from '../src/interactions.js';
import {assetsAvailable} from '../src/readiness.js';
import {EnvironmentalAudio,soundProfile} from '../src/audio.js';
import {taperShader,wornPaving} from '../src/materials.js';
import {treeShape} from '../src/trees.js';
import {R} from '../src/config.js';
const results=[];
function test(name,fn){fn();results.push({name,status:'pass'});}
test('Actual standard and depth shader injection strings contain GLSL newlines, not escaped text',()=>{
 for(const kind of ['standard','depth']){
  const shader={vertexShader:ShaderLib[kind].vertexShader};taperShader(shader);
  const expand=s=>s.replace(/#include <([\w\d_]+)>/g,(_,key)=>expand(ShaderChunk[key]));
  const expanded=expand(shader.vertexShader);
  assert.ok(!expanded.includes('\\n'));assert.ok(expanded.includes('\nattribute float branchTaper;\n'));
  assert.ok(expanded.includes('transformed.xz*=mix(1.,branchTaper'));
 }
});
test('Every authored X/Z hard cutoff and transition endpoint is continuous on-road and off-road',()=>{
 let worstTerrain=0,worstSupport=0,count=0;
 const eps=1e-5;
 for(const axis of ['x','z']){
  const cuts=axis==='x'?[-4096,-2800,2800,4096,4128]:[-4096,-2200,4000,5120,5152];
  const across=axis==='x'?[-4096,-2200,0,201.44,1240,4000,5120,5152]:[0,1.06,-1.06,3.2,-5.1,12,-75,500,2800,4096];
  for(const cut of cuts)for(const other of across){
   const a=axis==='x'?at(cut-eps,other):at(other,cut-eps),b=axis==='x'?at(cut+eps,other):at(other,cut+eps);
   worstTerrain=Math.max(worstTerrain,Math.abs(terrain(a)-terrain(b)));
   worstSupport=Math.max(worstSupport,Math.abs(support(a)-support(b)));count++;
   assert.ok(Math.abs(terrain(a)-terrain(b))<.0001,`${axis} ${cut} ${other}`);
   assert.ok(Math.abs(support(a)-support(b))<.0001,`support ${axis} ${cut} ${other}`);
  }
 }
 results.push({name:'boundary maxima',samples:count,terrainMetres:worstTerrain,supportMetres:worstSupport});
});
test('Generated near triangles match support beside each authored cutoff',()=>{
 let maxError=0,count=0;
 for(const [x,z] of [[4095.4,75],[-4096.4,75],[4127.4,75],[75,-4096.4],[75,5151.4],[-75,5120.4],[1.1,5151.4]]){
  const point=at(x,z),g=generate({chunk:point.chunk,kind:'near'});
  for(const [kind,mesh] of [['terrain',g.ground],...g.roads.map(r=>['road',r])]){
   const limit=kind==='terrain'?16*16*6:mesh.index.length;
   for(let i=0;i<limit;i+=3){
    const v=[0,0,0];for(let j=0;j<3;j++)for(let a=0;a<3;a++)v[a]+=mesh.p[mesh.index[i+j]*3+a]/3;
    const p=offset({chunk:point.chunk,local:[0,0,0]},v[0],v[2]);
    const actual=kind==='road'?support(p):terrainSurface(p);
    maxError=Math.max(maxError,Math.abs(actual-v[1]));count++;
   }
  }
 }
 assert.ok(maxError<.00001);results.push({name:'cutoff triangle agreement',count,maxErrorMetres:maxError});
});
test('Atmosphere overlap preserves global IDs and equal-time poses across signed X/Z/Y origin handoffs',()=>{
 const population=new Atmosphere();let overlaps=0;
 for(const [a,b] of [[at(0,31.99),at(0,32.01)],[at(-.01,35),at(.01,35)],[at(0,-.01),at(0,.01)],[roadPoint(199.99),roadPoint(200.01)]]){
  const prior=new Map(population.sample(a,12).map(p=>[p.id,p])),next=population.sample(b,12);
  assert.ok(next.length<=350);assert.ok(population.cache.size<=9);
  for(const p of next)if(prior.has(p.id)){
   assert.deepEqual(p,prior.get(p.id));overlaps++;
   const local=localParticle(p.worldPosition,b.chunk),shifted=localParticle(p.worldPosition,[b.chunk[0],String(BigInt(b.chunk[1])+1n),b.chunk[2]]);
   assert.ok(Math.abs(local[1]-shifted[1]-32)<1e-10);
  }
 }
 assert.ok(overlaps>250);
 const p=roadPoint(35),zero=population.sample(p,0);assert.notDeepEqual(population.sample(p,12),zero);assert.deepEqual(population.sample(p,0),zero);
 assert.ok(zero.some(p=>p.kind==='midge'));assert.ok(zero.some(p=>p.kind==='dust'));assert.ok(zero.some(p=>p.kind==='smoke'));
 population.sample({chunk:['9007199254740999','0','-9007199254740999'],local:[1,0,1]},0);assert.ok(population.cache.size<=9);
 assert.deepEqual(population.sample(p,0),zero);population.dispose();assert.equal(population.items.length,0);
 results.push({name:'retained atmosphere overlaps',overlaps});
});
test('Remapping accepts only binding keys and does not trap Tab, Shift+Tab or Escape',()=>{
 const bindings={forward:'KeyW'},input={dataset:{binding:'forward'},value:'W'};
 for(const [code,shiftKey] of [['Tab',false],['Tab',true],['Escape',false],['ArrowLeft',false]]){
  let prevented=false;assert.equal(bindKey({code,key:code,shiftKey,preventDefault(){prevented=true;}},input,bindings,()=>{}),false);assert.equal(prevented,false);
 }
 let changed=0,prevented=0;assert.equal(bindKey({code:'KeyZ',key:'z',preventDefault(){prevented++;}},input,bindings,()=>changed++),true);
 assert.equal(bindings.forward,'KeyZ');assert.equal(prevented,1);assert.equal(changed,1);
});
test('Asset readiness is false on loading, failed load, context loss, and disposal',()=>{
 const good={assetsReady:true,view:{assetsReady:true,disposed:false},loading:false,disposed:false,terminal:false,contextLost:false};
 assert.equal(assetsAvailable(good),true);
 for(const patch of [{assetsReady:false},{loading:true},{terminal:true},{disposed:true},{contextLost:true},{view:{assetsReady:false}},{view:{assetsReady:true,disposed:true}}])assert.equal(assetsAvailable({...good,...patch}),false);
});
class FakeParam{constructor(){this.value=0;}setValueAtTime(v){this.value=v;}setTargetAtTime(v){this.value=v;}cancelScheduledValues(){}}
class FakeNode{constructor(){this.gain=new FakeParam();this.frequency=new FakeParam();this.Q=new FakeParam();}connect(){}disconnect(){this.disconnected=true;}start(){this.started=true;}stop(){this.stopped=true;}}
class FakeContext{
 constructor(){this.state='suspended';this.sampleRate=1000;this.currentTime=0;this.destination={};this.sources=[];}
 createBuffer(_,n){return {getChannelData:()=>new Float32Array(n)};}createGain(){return new FakeNode();}createBiquadFilter(){return new FakeNode();}
 createBufferSource(){const n=new FakeNode();this.sources.push(n);return n;}
 async resume(){this.state='running';}async suspend(){this.state='suspended';}async close(){this.state='closed';}
}
let created=0;const audio=new EnvironmentalAudio(()=>{created++;return new FakeContext();});
assert.equal(created,0);await audio.setActive(true);assert.equal(created,0);
await audio.toggleFromGesture(true,roadPoint(0));assert.equal(created,1);assert.equal(audio.snapshot().contextState,'running');
const open=soundProfile(roadPoint(0)),canopy=soundProfile(roadPoint(35)),stream=soundProfile(roadPoint(R.bridgeStation));
assert.ok(canopy.cutoffHz<open.cutoffHz&&canopy.windGain<open.windGain&&stream.streamGain>open.streamGain);
audio.update(roadPoint(35));assert.equal(audio.snapshot().canopy,true);
await audio.setActive(false);assert.equal(audio.snapshot().contextState,'suspended');assert.equal(audio.snapshot().masterGain,0);
await audio.setActive(true);assert.equal(audio.snapshot().contextState,'running');
await audio.toggleFromGesture(true,roadPoint(0));assert.equal(audio.snapshot().contextState,'suspended');
const sources=[...audio.sources];audio.dispose();await audio.queue;assert.equal(audio.snapshot().contextState,'closed');assert.ok(sources.every(s=>s.stopped&&s.disconnected));audio.dispose();
results.push({name:'Production audio consent, canopy/stream profile, pause/hidden/context suspension and teardown with fake WebAudio nodes (not audible browser evidence)',status:'pass'});
test('Tree shapes retain centers/IDs but have differentiated multilevel tapering and thin branch tips',()=>{
 const oak=treesForChunk(at(10,110).chunk).find(t=>t.id==='great-oak');assert.ok(oak);
 const a=treeShape(oak,()=>oak.y),b=treeShape(oak,()=>oak.y);
 assert.deepEqual(a,b);assert.ok(a.filter(x=>x.kind==='wood').every(x=>x.taper>0&&x.taper<=1));
 assert.ok(a.some(x=>x.id.includes(':twig:')&&x.scale[0]*x.taper<.005));
 assert.ok(new Set(a.filter(x=>x.id.includes(':limb:')&&x.id.endsWith(':0')).map(x=>x.p[1].toFixed(2))).size>=4);
 assert.ok(a.length<150);
});
test('Pavement uses three angular block variants, dense bounded batches, actual vertex relief <=3cm',()=>{
 const geometries=[0,1,2].map(wornPaving),obj=new Object3D(),v=new Vector3();let count=0,maxRelief=-Infinity,visible=0;
 const data=generate({chunk:['0','0','5'],kind:'near'});
 for(let variant=0;variant<3;variant++){
  const batch=data.batches['paving'+variant],g=geometries[variant];assert.ok(batch.ids.length>30&&batch.ids.length<900);
  for(let i=0;i<batch.ids.length;i++){
   const a=i*12;obj.position.fromArray(batch.data,a);obj.scale.fromArray(batch.data,a+3);obj.rotation.set(batch.data[a+6],batch.data[a+7],batch.data[a+8],'YXZ');obj.updateMatrix();
   let peak=-Infinity;
   for(let j=0;j<g.attributes.position.count;j++){
    v.fromBufferAttribute(g.attributes.position,j).applyMatrix4(obj.matrix);
    const p=offset({chunk:data.chunk,local:[0,0,0]},v.x,v.z),relief=v.y-support(p);peak=Math.max(peak,relief);maxRelief=Math.max(maxRelief,relief);
   }
   if(peak>0)visible++;count++;
  }
 }
 assert.ok(count>=180&&count<500);assert.ok(maxRelief<=.0301);assert.ok(visible>count*.9);
 geometries.forEach(g=>g.dispose());results.push({name:'actual paving geometry',count,visible,maxReliefMetres:maxRelief});
});
console.log(JSON.stringify({boundary:'Node production-module tests; final native shader/audio/pixel review remains controller-owned',results},null,2));
