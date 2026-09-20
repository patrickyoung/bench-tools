import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {makePanorama,PANORAMA_CAPS} from '../src/panorama.js';
const p=makePanorama(),q=makePanorama();let vertices=0,disposed=0;const maxAdjacentProjectedSteps=[];
assert.equal(p.root.children.length,4);
assert.ok(p.root.children.length<=PANORAMA_CAPS.draws);
for(const [layer,mesh] of p.root.children.entries()){
 const g=mesh.geometry,pos=g.attributes.position,c=g.attributes.color,n=pos.count;
 vertices+=n;
 for(const attr of [pos,c]){
  assert.ok([...attr.array].every(Number.isFinite));
  assert.deepEqual([...attr.array.slice(0,6)],[...attr.array.slice(-6)]);
 }
 assert.deepEqual(pos.array,q.root.children[layer].geometry.attributes.position.array);
 assert.equal(g.index.count,1024*6);
 assert.ok([...g.index.array].every(i=>i>=0&&i<n));
 // Check every actual triangle, including the last seam pair, is nondegenerate.
 for(let i=0;i<g.index.count;i+=3){
  const v=[0,1,2].map(k=>{const j=g.index.array[i+k]*3;return [...pos.array.slice(j,j+3)];});
  const u=v[1].map((x,k)=>x-v[0][k]),w=v[2].map((x,k)=>x-v[0][k]);
  assert.ok(Math.hypot(u[1]*w[2]-u[2]*w[1],u[2]*w[0]-u[0]*w[2],u[0]*w[1]-u[1]*w[0])>1);
 }
 // Actual projected profiles cover every azimuth at bounded optical distances.
 const slopes=[];
 for(let i=0;i<1024;i++){
  const j=i*6,r=Math.hypot(pos.array[j],pos.array[j+2]);
  assert.ok(r>4000&&r<7300);
  for(let k=0;k<2;k++)assert.ok(Math.hypot(...pos.array.slice(j+k*3,j+k*3+3))<10000);
  slopes.push(pos.array[j+4]/r);
 }
 for(let start=0;start<1024;start++){
  const window=Array.from({length:64},(_,j)=>slopes[(start+j)%1024]);
  assert.ok(Math.max(...window)-Math.min(...window)>.0003,'no broad flat angular band');
  assert.ok(Math.abs(slopes[start]-slopes[(start+1)%1024])<.004,'continuous gentle profile');
 }
 maxAdjacentProjectedSteps.push(Math.max(...slopes.map((v,i)=>Math.abs(v-slopes[(i+1)%1024]))));
 if(layer===3){
  const peak=slopes.indexOf(Math.max(...slopes));
  assert.ok(peak<40||peak>984,'crown stays ahead');
  assert.ok(Math.max(...slopes)<.08,'sky-dominant silhouette');
 }
 g.addEventListener('dispose',()=>disposed++);
 mesh.material.addEventListener('dispose',()=>disposed++);
}
assert.ok(vertices<=PANORAMA_CAPS.vertices);assert.ok(vertices<=8200);
p.dispose();q.dispose();assert.equal(disposed,8);assert.equal(p.root.children.length,0);
const src=readFileSync(new URL('../src/renderer.js',import.meta.url),'utf8');
assert.ok(src.includes('this.panorama.root.position.set(this.camera.position.x,this.camera.position.y,this.camera.position.z)'));
assert.ok(src.includes('this.landmarks.root.position.set(-a.x+player.local[0],y0,-a.s+player.local[2])'));
console.log(JSON.stringify({test:'actual panorama seam/finite triangles/angular profiles/caps/disposal',layers:4,vertices,maxAdjacentProjectedSteps,triangles:8192,drawCap:4,outcome:'passed'}));
