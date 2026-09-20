import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {generate,terrainSurface,at,streamS,center,support,offset,buffersOf} from '../src/generation.js';
import {R} from '../src/config.js';
import {NEAR_RESERVE} from '../src/scheduler.js';
const coarse=generate({chunk:['0','0','4'],kind:'coarse'}).ground;
const nx=coarse.p.filter((_,i)=>i%3===2&&coarse.p[i]===coarse.p[2]).length;
const x0=coarse.p[0],z0=coarse.p[2],stride=8;
function sample(x,s){
 const z=s-128,u=(x-x0)/stride,v=(z-z0)/stride,i=Math.floor(u),j=Math.floor(v),a=u-i,b=v-j;
 const h=(dx,dz)=>coarse.p[((j+dz)*nx+i+dx)*3+1];
 return a+b<=1?h(0,0)+(h(1,0)-h(0,0))*a+(h(0,1)-h(0,0))*b:h(1,1)+(h(0,1)-h(1,1))*(1-a)+(h(1,0)-h(1,1))*(1-b);
}
let maximumExcess=-Infinity,worst=null,samples=0;
for(let x=-64;x<=64;x+=1)for(let s=R.bridgeStation-18;s<=R.bridgeStation+15;s+=.5){
 const channel=Math.abs(s-streamS(x)),l=x-center(s);
 if(channel>12||Math.abs(l)<3.1)continue;
 const excess=sample(x,s)-terrainSurface(at(x,s));
 if(excess>maximumExcess){maximumExcess=excess;worst={x,station:s};}samples++;
}
const unaffected=[];
for(let i=0;i<coarse.p.length;i+=3){
 const x=coarse.p[i],s=coarse.p[i+2]+128;
 if(Math.abs(s-streamS(x))>=28)unaffected.push(coarse.p[i],coarse.p[i+1],coarse.p[i+2]);
}
let legacyCobbles=0,pavement=0,maxRelief=-Infinity,maxStones=0,maxNearTransferBytes=0;
const ids=new Set();
for(let z=4;z<=6;z++){
 const g=generate({chunk:['0','0',String(z)],kind:'near'});
 maxNearTransferBytes=Math.max(maxNearTransferBytes,buffersOf(g).reduce((n,b)=>n+b.byteLength,0));
 for(const b of [g.batches.stone,g.batches.paving0,g.batches.paving1,g.batches.paving2]){
 maxStones=Math.max(maxStones,b.ids.length);maxNearTransferBytes=Math.max(maxNearTransferBytes,buffersOf(g).reduce((n,b)=>n+b.byteLength,0));
 for(let i=0;i<b.ids.length;i++){
  if(b.ids[i].includes(':cobble:'))legacyCobbles++;
  if(!b.ids[i].startsWith('road-pavement:'))continue;
  pavement++;assert.ok(!ids.has(b.ids[i]));ids.add(b.ids[i]);
  const k=i*12,p=offset({chunk:g.chunk,local:[0,0,0]},b.data[k],b.data[k+2]);
  const rx=b.data[k+6],rz=b.data[k+8];
  const extent=b.data[k+4]*Math.cos(rx)*Math.cos(rz);
  maxRelief=Math.max(maxRelief,b.data[k+1]+extent-support(p));
 }
}
}
const result={coarseMaximumExcessMetres:maximumExcess,worst,samples,unaffectedCoarseHash:createHash('sha256').update(new Float32Array(unaffected)).digest('hex'),legacyCobbles,pavement,maxStones,maxNearTransferBytes,maxPavementCentreReliefMetres:Number.isFinite(maxRelief)?maxRelief:null};
console.log(JSON.stringify(result,null,2));
if(process.argv.includes('--require-corrected')){
 assert.ok(maximumExcess<0,'coarse backing must remain below detailed bank/ramp');
 assert.ok(pavement>=180&&pavement<500,'dense, bounded exposed pavement');
 assert.ok(maxStones<900);assert.ok(maxNearTransferBytes<NEAR_RESERVE);
 assert.ok(maxRelief>0&&maxRelief<=.031);
}
