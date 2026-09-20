// Real browser tests of reviewed reusable primitives, NOT a generated world.
import {pathToFileURL} from 'node:url';
import {resolve,join} from 'node:path';
import {readFile,writeFile,mkdir} from 'node:fs/promises';
import {createServer} from 'node:http';
import assert from 'node:assert/strict';
const deps=resolve(process.argv[2]||'tests/deps/node_modules');
const out=resolve(process.argv[3]||'tests/browser-results');
const {chromium}=await import(pathToFileURL(join(deps,'playwright/index.mjs')));
await mkdir(out,{recursive:true});
const core=resolve('expert/templates/core');
const url='http://worldweaver.test';
async function route(context) {
 await context.route('http://worldweaver.test/**', async r=>{
  const name=new URL(r.request().url()).pathname.slice(1);
  if(!name){await r.fulfill({contentType:'text/html',body:'<!doctype html><title>Core probe</title><canvas></canvas>'});return;}
  if(!['coordinates.mjs','noise.mjs','store.mjs','three.module.js','three.core.js'].includes(name)){
   await r.fulfill({status:404,body:'not found'});return;
  }
  const f=name.startsWith('three.')?join(deps,'three/build',name):join(core,name);
  await r.fulfill({contentType:'text/javascript',body:await readFile(f)});
 });
}
let context;
const results={scope:'Actual Chromium/IndexedDB/Web Worker/Three primitives; not fresh world evaluation',tests:[]};
try{
 context=await chromium.launchPersistentContext(join(out,'profile'),{headless:true});
 await route(context);
 let page=await context.newPage();await page.goto(url);
 const config={name:'ww-'+Date.now(),world:'world-A',version:'v1',fingerprint:'seed+generator-hash'};
 const first=await page.evaluate(async cfg=>{
  const {openStore}=await import('/store.mjs');window.cfg=cfg;
  const s=await openStore(cfg);window.store=s;
  await s.commit('-1,0,9007199254740993',0,[{id:'light:1',value:{removed:true}},{id:'sculpture:1',value:{position:[1,2,3]}}],{position:{chunk:['-1','0','9007199254740993'],local:[1,2,3]}});
  let aborted=false;try{await s.commit('-1,0,9007199254740993',1,[{id:'bad',value:1}],undefined,{abort:true});}catch{aborted=true;}
  let conflict=false;try{await s.commit('-1,0,9007199254740993',0,[{id:'stale',value:1}]);}catch{conflict=true;}
  let wrongFingerprint=false;try{await openStore({...cfg,fingerprint:'different-seed'});}catch{wrongFingerprint=true;}
  const other=await openStore({...cfg,world:'world-B'}),version=await openStore({...cfg,version:'v2'});
  const isolated=(await other.get('-1,0,9007199254740993','light:1'))===undefined&&(await version.get('-1,0,9007199254740993','light:1'))===undefined;
  other.close();version.close();
  const rows=[];let after=null,maxPage=0;do{const p=await s.page({after,limit:2});maxPage=Math.max(maxPage,p.rows.length);rows.push(...p.rows);after=p.next;}while(after);
  return {aborted,conflict,wrongFingerprint,isolated,maxPage,rows,badAbsent:(await s.get('-1,0,9007199254740993','bad'))===undefined};
 },config);
 assert(first.aborted&&first.conflict&&first.wrongFingerprint&&first.isolated&&first.badAbsent);assert.equal(first.maxPage,2);
 results.tests.push('real IDB commit/abort/CAS/world-version-fingerprint isolation/bounded cursor pages');
 await page.close();await context.close();
 context=await chromium.launchPersistentContext(join(out,'profile'),{headless:true});
 await route(context);
 page=await context.newPage();await page.goto(url);
 const restored=await page.evaluate(async cfg=>{
  const {openStore}=await import('/store.mjs');const s=await openStore(cfg);
  const removed=await s.get('-1,0,9007199254740993','light:1'),placed=await s.get('-1,0,9007199254740993','sculpture:1'),player=await s.get('@player','state');s.close();return {removed,placed,player};
 },config);
 assert(restored.removed.removed);assert.deepEqual(restored.placed.position,[1,2,3]);assert.deepEqual(restored.player.position.local,[1,2,3]);
 results.tests.push('actual browser process/profile restart preserves requested sparse direct edits and optional position');
 const workerResult=await page.evaluate(async()=>{
  const source=`import {perlin3} from '${location.origin}/noise.mjs'; onmessage=e=>{const a=new Float64Array(e.data.map(x=>perlin3('s','t',[BigInt(x),2n,-3n])));postMessage(a.buffer,[a.buffer]);postMessage({detached:a.byteLength===0});}`;
  const u=URL.createObjectURL(new Blob([source],{type:'text/javascript'}));
  const worker=new Worker(u,{type:'module'});const messages=[];
  const answer=await new Promise((resolve,reject)=>{worker.onerror=reject;worker.onmessage=e=>{messages.push(e.data);if(messages.length===2)resolve(messages);};worker.postMessage(['-1','9007199254740993']);});
  worker.terminate();URL.revokeObjectURL(u);
  const {perlin3}=await import('/noise.mjs');
  return {detached:answer[1].detached,values:Array.from(new Float64Array(answer[0])),expected:[-1n,9007199254740993n].map(x=>perlin3('s','t',[x,2n,-3n]))};
 });
 assert(workerResult.detached);assert.deepEqual(workerResult.values,workerResult.expected);
 results.tests.push('real module worker transfer detachment and far deterministic samples');
 const rendered=await page.evaluate(async()=>{
  const T=await import('/three.module.js');
  const renderer=new T.WebGLRenderer({canvas:document.querySelector('canvas')});renderer.setSize(400,300);
  const scene=new T.Scene();scene.background=new T.Color('#203040');
  const camera=new T.PerspectiveCamera(55,4/3,.1,100);camera.position.set(3,3,5);camera.lookAt(0,0,0);
  const geometry=new T.BoxGeometry(1,1,1),material=new T.MeshStandardMaterial({color:'#44aa66',roughness:.8});
  const mesh=new T.InstancedMesh(geometry,material,2),m=new T.Matrix4();mesh.setMatrixAt(0,m);m.makeTranslation(1.5,0,0);mesh.setMatrixAt(1,m);
  mesh.instanceMatrix.needsUpdate=true;mesh.computeBoundingSphere();scene.add(mesh,new T.HemisphereLight(0xffffff,0x223344,3));
  renderer.render(scene,camera);
  const gl=renderer.getContext(),pixels=new Uint8Array(400*300*4);gl.readPixels(0,0,400,300,gl.RGBA,gl.UNSIGNED_BYTE,pixels);
  const colors=new Set();for(let i=0;i<pixels.length;i+=4)colors.add(`${pixels[i]},${pixels[i+1]},${pixels[i+2]}`);
  const result={calls:renderer.info.render.calls,geometries:renderer.info.memory.geometries,colors:colors.size,backend:gl.getParameter(gl.VERSION)};
  geometry.dispose();material.dispose();renderer.dispose();return result;
 });
 assert.equal(rendered.calls,1);assert(rendered.colors>5);results.tests.push('Three 0.186.0 actual WebGL2 instanced draw and varied rendered pixels');
 results.render=rendered;results.browser=context.browser()?.version()||'persistent Chromium';results.status='passed';
 await page.screenshot({path:join(out,'core.png')});
}catch(e){results.status='failed';results.error=String(e);process.exitCode=1;}
finally{if(context)await context.close();await writeFile(join(out,'results.json'),JSON.stringify(results,null,2)+'\n');console.log(JSON.stringify(results,null,2));}
