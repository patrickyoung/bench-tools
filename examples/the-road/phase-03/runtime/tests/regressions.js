import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {Worker} from 'node:worker_threads';
import {B,R} from '../src/config.js';
import {normalize,floorDiv,globalCell} from '../src/coordinates.js';
import {perlin3} from '../src/noise.js';
import {generate,canonicalSample,buffersOf,terrain,support,roadPoint,at,center,profile,blocked,offset,cross,authored,roadInfo} from '../src/generation.js';
import {Walker,groundPosition} from '../src/interactions.js';
import {Scheduler,desiredChunks,NEAR_RESERVE,COARSE_RESERVE} from '../src/scheduler.js';
const results=[],test=(name,fn)=>{const start=performance.now();fn();results.push({name,status:'pass',ms:Math.round((performance.now()-start)*100)/100});};
const hash=v=>createHash('sha256').update(JSON.stringify(v)).digest('hex');
test('Signed floor and normalized bounded locals',()=>{
 assert.equal(floorDiv(-1n,32n),-1n);assert.deepEqual(normalize('0',-.25,32),{chunk:-1n,local:31.75});
 assert.equal(globalCell('9007199254740993',31,32),288230376151711807n);
});
test('Continuous 3D rational Perlin at signed and >2^53 seams',()=>{
 for(const x of [-9007199254740999n,-33n,-1n,0n,9007199254740999n]){
  const a=perlin3(B.seed,'land',[x,17n,19n],[.999999,0,0],1,64),b=perlin3(B.seed,'land',[x+1n,17n,19n],[0,0,0],1,64);
  assert.ok(Math.abs(a-b)<1e-6);
 }
 assert.notEqual(perlin3(B.seed,'land',[7n,0n,13n]),perlin3(B.seed,'land',[7n,19n,13n]));
});
test('Exact terrain sample identities across shared 32m edges',()=>{
 for(const cx of ['-9007199254741007','-1','0','9007199254741007']){
  const a={chunk:[cx,'0','-1'],local:[32,0,9.7]},b={chunk:[(BigInt(cx)+1n).toString(),'0','-1'],local:[0,0,9.7]};
  assert.equal(terrain(a),terrain(b));assert.equal(support(a),support(b));
 }
});
test('Revisit, completion order, style independence and seed sensitivity',()=>{
 const chunks=[['0','0','0'],['0','0','3'],['-1','0','6'],['9007199254740997','0','-9007199254740999']];
 const original=chunks.map(c=>hash(canonicalSample(c)));
 const style=B.style;B.style='A style-only alternate';assert.deepEqual([...chunks].reverse().map(c=>hash(canonicalSample(c))).reverse(),original);B.style=style;
 assert.notEqual(hash(canonicalSample(chunks[3],'alternate-seed')),original[3]);
 assert.notEqual(original[3],hash(canonicalSample(['9007199254740998','0','-9007199254740999'])));
});
test('Authored dimensions and unblocked road, oak, bridge and climb',()=>{
 assert.ok(Math.abs(R.bridgeStation-R.oakStation-91.44)<1e-9);assert.equal(R.width,6.096);assert.equal(R.bridgeSpan,4.572);
 assert.ok(cross(0)>cross(R.rutOffset));
 for(let s=0;s<R.length;s+=3.3)assert.equal(blocked(roadPoint(s)),false,`road blocked ${s}`);
 assert.ok(profile(201.44)<profile(110));assert.ok(profile(1240)>profile(730));
 for(let t=0;t<=1;t+=.05){const p=roadPoint(R.bridgeStation-12+10*t,-4-11*t);assert.equal(blocked(p),false);assert.ok(Number.isFinite(support(p)));}
 assert.ok(blocked(roadPoint(35,6)));assert.ok(blocked(roadPoint(R.bridgeStation,3.3)));
});
test('Production Walker moves, looks, grounds, revisits and stops at parapet',()=>{
 const w=new Walker(),before=structuredClone(w.player);w.move(1,0,1);assert.notDeepEqual(w.player,before);w.look(.2,.1);assert.equal(w.yaw,-.2);
 w.travel(roadPoint(R.bridgeStation));w.yaw=0;for(let i=0;i<12;i++)w.move(0,1,.2);assert.ok(Math.abs(authored(w.player).x-center(R.bridgeStation))<3.1);
 const p=roadPoint(R.crestStation);p.chunk[1]='999999999999999999999';p.local[1]=31;w.travel(p);assert.notEqual(w.player.chunk[1],p.chunk[1]);
 w.reset();assert.deepEqual(w.player,before);
});
test('Near/coarse actual transfer sizes and stable geometry',()=>{
 for(const [chunk,kind] of [[['0','0','0'],'near'],[['0','0','3'],'near'],[['0','0','6'],'near'],[['0','0','0'],'coarse']]){
  const g=generate({chunk,kind}),bytes=buffersOf(g).reduce((a,b)=>a+b.byteLength,0);
  assert.ok(bytes<(kind==='coarse'?COARSE_RESERVE:NEAR_RESERVE));
  assert.ok(g.ground.p.every(Number.isFinite));for(const b of Object.values(g.batches)){assert.equal(b.ids.length*12,b.data.length);assert.equal(new Set(b.ids).size,b.ids.length);}
  results.push({name:`measured-${kind}-${chunk.join(',')}`,bytes,vertices:g.ground.p.length/3,instances:Object.values(g.batches).reduce((n,b)=>n+b.ids.length,0)});
 }
});
test('Ground and road support agree with actual generated triangles including ramp',()=>{
 let maxGround=0,maxRoad=0;
 for(const chunk of [['0','0','0'],['0','0','6'],['-1','0','6']]){
  const result=generate({chunk,kind:'near'});
  for(const [kind,g] of [['ground',result.ground],...result.roads.map(g=>['road',g])]){
   const count=kind==='ground'?16*16*6:g.index.length;
   for(let i=0;i<count;i+=3){
    const q=[0,0,0];for(let j=0;j<3;j++){const k=g.index[i+j]*3;for(let axis=0;axis<3;axis++)q[axis]+=g.p[k+axis]/3;}
    const p=offset({chunk,local:[0,0,0]},q[0],q[2]),r=roadInfo(p);
    if(kind==='ground'&&r&&Math.abs(r.l)<3.1)continue;
    const error=Math.abs(support(p)-q[1]);if(kind==='ground')maxGround=Math.max(maxGround,error);else maxRoad=Math.max(maxRoad,error);
   }
  }
 }
 assert.ok(maxGround<.00001);assert.ok(maxRoad<.00002);
 results.push({name:'support maximum errors',groundMetres:maxGround,roadMetres:maxRoad});
});
test('Live blueprint crown parameter changes geometry without relocating landmarks',()=>{
 const before=generate({chunk:['0','0','0'],kind:'near'}).roads[0].p[16];
 const old=R.crown;R.crown=old+.03;
 const after=generate({chunk:['0','0','0'],kind:'near'}).roads[0].p[16];
 R.crown=old;assert.notEqual(before,after);assert.equal(R.oakStation,110);
});
test('Radial desired XYZ selection, cap and far positive/negative travel',()=>{
 for(const c of ['-9007199254740999','0','9007199254740999']){
  const p=groundPosition({chunk:[c,'0',c],local:[1,1,1]}),list=desiredChunks(p);assert.ok(list.length<=80&&list.length>20);assert.equal(list.filter(d=>d.kind==='coarse').length,1);assert.ok(list.some(d=>d.chunk[0]===c));
 }
});
function harness(){let uploads=0,evictions=0;const workers=[];const s=new Scheduler({upload:()=>{uploads++;return {};},evict:()=>evictions++,status:()=>{},workerFactory:()=>{const w={terminate(){this.dead=true;},postMessage(t){this.job=t;}};workers.push(w);return w;}});return {s,workers,get uploads(){return uploads;},get evictions(){return evictions;}};}
const payload=()=>({ground:{p:new Float32Array([0,0,0])}});
function reply(s,slot){const t=slot.job;s.message(slot,{type:'result',id:t.id,chunk:t.chunk,seed:t.seed,version:t.version,epoch:t.epoch,revision:t.revision,payload:payload()});}
test('Reserved stale ownership, completion cap, pause and disposal',()=>{
 const h=harness(),s=h.s,p=groundPosition(roadPoint(0));s.setDesired(p);s.tick();for(const slot of s.slots)s.message(slot,{type:'ready'});s.tick();
 assert.equal(s.tickets.size,2);const reserved=s.queueBytes;assert.ok(reserved>0);
 s.setDesired(groundPosition(roadPoint(730)));assert.equal(s.queueBytes,reserved);
 for(const slot of s.slots)reply(s,slot);assert.equal(s.queueBytes,0);assert.equal(s.staleDiscards,2);
 s.tick();s.setPaused(true);for(const slot of s.slots)reply(s,slot);assert.ok(s.queueBytes>0);s.tick();assert.equal(h.uploads,0);
 s.setPaused(false);s.tick();assert.ok(h.uploads<=2);s.dispose();assert.equal(s.queueBytes,0);assert.equal(s.tickets.size,0);assert.equal(s.stats().workers,0);
});
test('Separate total boot cap and paused replacement deferral',()=>{
 const {s}=harness();s.setDesired(groundPosition(roadPoint(0)));s.tick();s.setPaused(true);
 for(const slot of s.slots)s.failSlot(slot,'synthetic boot fault');s.tick();assert.equal(s.boots,2);
 s.setPaused(false);
 for(let i=0;i<5;i++){s.tick();for(const slot of s.slots)if(slot.worker)s.failSlot(slot,'synthetic boot fault');}
 assert.equal(s.boots,6);assert.ok(s.failure);assert.equal(s.queueBytes,0);assert.equal(s.stats().workers,0);
});
const start=performance.now();
const worker=new Worker(new URL('./worker-node.mjs',import.meta.url),{type:'module'});
await new Promise((resolve,reject)=>{
 const timer=setTimeout(()=>reject(Error('worker timeout')),30000);
 worker.on('error',reject);
 worker.on('message',m=>{
  if(m.type==='ready')worker.postMessage({id:1,chunk:['0','0','3'],seed:B.seed,version:B.generationVersion,epoch:1,revision:0,kind:'near'});
  else if(m.type==='result'){assert.equal(m.epoch,1);assert.ok(m.payload.ground.p.byteLength>0);assert.equal(m.payload.batches.foliage.data.length,m.payload.batches.foliage.ids.length*12);clearTimeout(timer);resolve();}
  else reject(Error(JSON.stringify(m)));
 });
});
await worker.terminate();
results.push({name:'Real Node worker-thread production protocol/transfer (not browser)',status:'pass',ms:performance.now()-start});
const buffer=new ArrayBuffer(16);structuredClone({buffer},{transfer:[buffer]});assert.equal(buffer.byteLength,0);results.push({name:'Transfer detachment',status:'pass'});
console.log(JSON.stringify({boundary:'Node math/worker tests in action Cage, no rendered browser',results},null,2));
