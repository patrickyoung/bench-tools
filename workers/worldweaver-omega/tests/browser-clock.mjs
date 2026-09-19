// Controlled-time fixture comparison. No assertion of real generated-world life.
import {resolve,join,sep} from 'node:path';
import {pathToFileURL} from 'node:url';
import {readFile,writeFile,mkdir} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import assert from 'node:assert/strict';
const deps=resolve(process.argv[2]||'../browser-runtime/node_modules');
const runtime=resolve(process.argv[3]||'tests/breadth-results/browser-v1/moving-scene');
const out=resolve(process.argv[4]||'tests/breadth-results/clock-'+Date.now());
await mkdir(out,{recursive:false});
const {chromium}=await import(pathToFileURL(join(deps,'playwright/index.mjs')));
const browser=await chromium.launch({headless:true});
const result={scope:'Synthetic rendered clock contract; not generated actors or style fidelity'};
try{
 const context=await browser.newContext();
 await context.route('**/*',async r=>{
  const u=new URL(r.request().url()),file=resolve(runtime,'.'+(u.pathname==='/'?'/index.html':u.pathname));
  if(u.origin!=='http://localhost:8080'||!file.startsWith(runtime+sep))return r.abort();
  try{await r.fulfill({body:await readFile(file),contentType:file.endsWith('.html')?'text/html':file.endsWith('.json')?'application/json':'text/javascript'});}
  catch{await r.fulfill({status:404,body:''});}
 });
 const page=await context.newPage();await page.goto('http://localhost:8080');
 await page.waitForFunction(()=>!!window.worldweaver);
 await page.evaluate(()=>window.worldweaver.ready);
 const hash=b=>createHash('sha256').update(b).digest('hex');
 const sample=async(t,name)=>{
  const state=await page.evaluate(async t=>{
   const w=window.worldweaver;await w.setSimulationTime(t);await w.settle();
   return {snapshot:w.snapshot(),base:await w.baseHash(['0','0','0'])};
  },t);
  state.pixels=hash(await page.locator('[data-ww="scene"]').screenshot({path:join(out,name+'.png')}));
  return state;
 };
 const a=await sample(0,'zero'),b=await sample(1,'one'),c=await sample(0,'zero-again');
 assert.equal(a.snapshot.simulationTime,0);assert.equal(b.snapshot.simulationTime,1);
 assert.equal(a.base,b.base);assert.equal(a.base,c.base);
 assert.notEqual(a.pixels,b.pixels,'time must actually move rendered actor');
 assert.equal(a.pixels,c.pixels,'same time must reproduce pixels');
 assert.deepEqual(a.snapshot.player,b.snapshot.player,'ambient motion cannot move navigation state');
 result.samples={a,b,c};result.status='passed';
}catch(e){result.status='failed';result.failure=String(e);process.exitCode=1;}
finally{await browser.close();await writeFile(join(out,'results.json'),JSON.stringify(result,null,2));console.log(result.status,result.failure||'equal-time reproducibility, different-time motion, static identity');}
