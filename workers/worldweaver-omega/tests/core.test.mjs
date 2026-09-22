import test from 'node:test';
import assert from 'node:assert/strict';
import {floorDiv,normalize,globalCell,localRender,identity} from '../expert/templates/core/coordinates.mjs';
import {perlin3,hash64,octaves} from '../expert/templates/core/noise.mjs';
import {Budget,radialOffsets} from '../expert/templates/core/budget.mjs';
import {repairPlacement,overlaps} from '../expert/templates/core/placement.mjs';
test('negative division, canonical identity and far local rendering',()=>{
 assert.equal(floorDiv(-1n,16n),-1n);assert.equal(floorDiv(-16n,16n),-1n);
 assert.deepEqual(normalize('9007199254740993',-0.25),{chunk:9007199254740992n,local:15.75});
 assert.equal(localRender('9007199254740994','9007199254740993',0.25),16.25);
 assert.throws(()=>localRender('9007199254740994','0',0));
 assert.notEqual(identity('a|b','c','d','e'),identity('a','b|c','d','e'));
 assert.throws(()=>normalize('01',0));
});
test('3D Perlin seams at negative/positive/far chunks, order/seed/parameter invariants',()=>{
 const samples=[];
 for(const chunk of [-10000000000000001n,-1n,0n,1n,10000000000000001n]){
  const cell=globalCell(chunk,16),next=globalCell(chunk+1n,0);assert.equal(cell,next);
  const sample=x=>perlin3('seed','terrain',[x,2n,-7n],[0,.25,.75],3,127);
  assert.equal(sample(cell),sample(next));
  const left=perlin3('seed','terrain',[cell-1n,2n,-7n],[.999999,.25,.75],3,127);
  assert(Math.abs(left-sample(cell))<.00001,'seam discontinuity');
  samples.push([cell,sample(cell)]);
 }
 for(const [cell,value] of samples.reverse())assert.equal(perlin3('seed','terrain',[cell,2n,-7n],[0,.25,.75],3,127),value);
 assert.notEqual(hash64('a','t',0n,1n,2n),hash64('b','t',0n,1n,2n));
 assert.notEqual(perlin3('a','t',[1n,2n,3n]),perlin3('b','t',[1n,2n,3n]));
 assert.notEqual(perlin3('a','t',[1n,2n,3n]),perlin3('a','t',[1n,3n,3n]));
 assert.notEqual(octaves('a','t',[1n,2n,3n],[0,0,0],[{numerator:1,denominator:64,amplitude:1}]),
                 octaves('a','t',[1n,2n,3n],[0,0,0],[{numerator:1,denominator:64,amplitude:2}]));
});
test('in-flight reservations survive epochs; delayed/reverse completions cannot exceed caps',()=>{
 const b=new Budget({jobs:3,bytes:100});const jobs=[b.reserve('a',40),b.reserve('b',40)];
 assert.equal(b.reserve('c',40),null);b.invalidate();
 assert.equal(b.current(jobs[0]),false);assert.equal(b.used,80);
 b.release(jobs[1]);assert.equal(b.used,40);
 const c=b.reserve('c',50);assert(b.current(c));assert.equal(b.reserve('d',20),null);
 b.release(jobs[0]);b.release(c);assert.equal(b.release(c),false);assert.equal(b.used,0);
 for(let i=0;i<1000;i++){const j=b.reserve(String(i),100);assert(j);assert.equal(b.reserve('overflow',1),null);b.release(j);}
 assert.throws(()=>new Budget({jobs:Infinity,bytes:100}));
 const offsets=radialOffsets(3);assert(offsets.every(p=>Math.hypot(p.x,p.y,p.z)<=3));
 assert(offsets.findIndex(p=>p.z===1&&p.x===0&&p.y===0)<offsets.findIndex(p=>p.z===-1&&p.x===0&&p.y===0));
});
test('mechanical placement repair preserves original and reports unresolved access',()=>{
 const original={center:[0,100,0],size:[2,2,2]},occupied=[{center:[0,1,0],size:[2,2,2]}];
 const a=repairPlacement(original,occupied,()=>0,{rings:5,clearance:1});
 assert.equal(a.status,'repaired');assert.equal(a.placement.center[1],1);assert(!overlaps(a.placement,occupied[0],1));
 assert.equal(original.center[1],100);assert.deepEqual(a,repairPlacement(original,occupied,()=>0,{rings:5,clearance:1}));
 assert.equal(repairPlacement(original,occupied,()=>NaN).status,'unresolved');
});
