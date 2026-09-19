#!/usr/bin/env node
// Controller-executed smoke probe, ONLY in a selected browser boundary.
// Production UI + adapter observations still require independent source/intent review.
import {readFileSync} from 'node:fs';
import {spawn} from 'node:child_process';
import {cleanup} from '../lib/browser-cleanup.mjs';
import {fileURLToPath} from 'node:url';
import {parseArgs} from 'node:util';
import {pathToFileURL} from 'node:url';
import {resolve,extname,sep} from 'node:path';
import {readFile,mkdir,writeFile,unlink} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import assert from 'node:assert/strict';
const {values:a}=parseArgs({options:{
  url:{type:'string'},'runtime-dir':{type:'string'},blueprint:{type:'string'},request:{type:'string'},out:{type:'string'},
  scenario:{type:'string'},'deadline-ms':{type:'string',default:'180000'},'browser-arg':{type:'string',multiple:true,default:[]},
  playwright:{type:'string'},boundary:{type:'string'},'candidate-hash':{type:'string'},help:{type:'boolean'}
}});
if(a.help){console.log('node browser-audit.mjs --url http://127.0.0.1:PORT --request ORIGINAL_REQUEST --blueprint FILE --out NEW_DIR --playwright /installed/playwright/index.mjs --boundary LABEL --candidate-hash SHA256\nOptional --deadline-ms 180000 (1000..900000), --browser-arg=--use-angle=metal (or --use-gl=angle); sandbox remains enabled. --scenario FILE selects grounded clearance or explicit production flight and reachable edit targets. Optional --runtime-dir serves exact local files by route fulfillment. Browser execution is not a sandbox or semantic/FPS certification.');process.exit(0);}
for(const k of ['url','request','blueprint','out','playwright','boundary','candidate-hash'])if(!a[k])throw Error('missing --'+k);
if(!['127.0.0.1','localhost','[::1]'].includes(new URL(a.url).hostname))throw Error('local server required');
const deadline=Number(a['deadline-ms']);
assert(Number.isInteger(deadline)&&deadline>=1000&&deadline<=900000,'invalid controller deadline');
assert(a['browser-arg'].every(x=>['--use-angle=metal','--use-gl=angle'].includes(x)),'unreviewed browser arg; sandbox weakening forbidden');
if(process.env.WW_AUDIT_CHILD!==String(process.ppid)){
  // Unix process-group watchdog lives OUTSIDE Playwright and the page event loop.
  // It also bounds launch, screenshots, adapter calls and browser close.
  await mkdir(a.out,{recursive:false});
  const started=Date.now();
  const child=spawn(process.execPath,[fileURLToPath(import.meta.url),...process.argv.slice(2)],{
    detached:true,stdio:'inherit',env:{...process.env,WW_AUDIT_CHILD:String(process.pid)}});
  let timedOut=false,interrupted=false,code=null;
  const browserGroups=new Set(),groups=new Set([child.pid]);
  const register=()=>{
    try{
      const pid=Number(readFileSync(resolve(a.out,'browser.pid'),'utf8').trim());
      if(!Number.isSafeInteger(pid)||pid<=1)throw Error('invalid registered browser pid');
      browserGroups.add(pid);groups.add(pid);
    }catch(e){if(e.code!=='ENOENT')throw e;}
  };
  let wake;
  const stopped=new Promise(res=>{wake=res;});
  const timer=setTimeout(()=>{timedOut=true;wake();},deadline);
  const stop=()=>{interrupted=true;wake();};
  process.on('SIGINT',stop);process.on('SIGTERM',stop);
  child.on('exit',value=>{code=value;wake();});
  child.on('error',()=>{code=1;wake();});
  await stopped;
  clearTimeout(timer);
  const teardown=await cleanup({pid:child.pid,groups,register});
  const killError=teardown.killError;
  process.removeListener('SIGINT',stop);process.removeListener('SIGTERM',stop);
  if(timedOut||interrupted||killError){
    await writeFile(resolve(a.out,'browser.json'),JSON.stringify({status:'failed',
      candidateHash:a['candidate-hash'],tests:[],failure:timedOut?'controller deadline exceeded':killError||'interrupted',
      limits:['Incomplete smoke run; no semantic or FPS claim.']},null,2)+'\n');
  }
  await writeFile(resolve(a.out,'supervisor.json'),JSON.stringify({pid:child.pid,processGroup:child.pid,
    browserGroups:[...browserGroups],elapsedMs:Date.now()-started,deadlineMs:deadline,timedOut,interrupted,killError,cleanup:teardown,childExit:code},null,2)+'\n');
  process.exit(timedOut||interrupted||killError?1:code===0?0:1);
}
const {chromium}=await import(pathToFileURL(a.playwright));
// Playwright launches Chromium in its OWN process group. Register that group
// before exec so the outer deadline can kill it even if Playwright close hangs.
const browserWrapper=resolve(a.out,'browser-launch.sh');
await writeFile(browserWrapper,`#!/bin/sh
set -eu
echo "$$" > "$WW_AUDIT_BROWSER_PID"
exec "$WW_AUDIT_BROWSER_EXECUTABLE" "$@"
`,{mode:0o700,flag:'wx'});
const requestBytes=await readFile(a.request),blueprintBytes=await readFile(a.blueprint);
const request=JSON.parse(requestBytes),blueprint=JSON.parse(blueprintBytes);
const hash=b=>createHash('sha256').update(b).digest('hex');
const scenarioBytes=a.scenario?await readFile(a.scenario):null;
const scenario=scenarioBytes?JSON.parse(scenarioBytes):{vertical:{mode:'grounded',clearance:[0,3]}};
const vertical=scenario.vertical;
assert(['grounded','flight'].includes(vertical?.mode),'select supported vertical scenario');
if(vertical.mode==='grounded')assert(Array.isArray(vertical.clearance)&&vertical.clearance.length===2&&
  vertical.clearance.every(Number.isFinite)&&vertical.clearance[0]>=0&&vertical.clearance[1]>=vertical.clearance[0]&&vertical.clearance[1]<=100,'invalid clearance');
const results={candidateHash:a['candidate-hash'],requestHash:hash(requestBytes),blueprintHash:hash(blueprintBytes),boundary:a.boundary,tests:[],screenshots:[],
  launch:{args:a['browser-arg'],chromiumSandbox:true,headless:true},scenarioHash:scenarioBytes?hash(scenarioBytes):null,scenario,
  limits:['Smoke tests require independent original-intent and adapter/source/UI review under C08-C10, including disabled editing.',
    'Not full lifecycle/failure matrix or generated-world quality.','No 90 FPS certification.'],errors:[]};
let context;
async function route(c) {
  if(!a['runtime-dir']) return;
  const root=resolve(a['runtime-dir']);
  await c.route('**/*',async r=>{
    const u=new URL(r.request().url());
    if(u.origin!==new URL(a.url).origin){await r.abort();return;}
    const path=resolve(root,'.'+decodeURIComponent(u.pathname==='/'?'/index.html':u.pathname));
    if(!path.startsWith(root+sep)){await r.abort();return;}
    try {
      const body=await readFile(path);
      const type={'.js':'text/javascript','.mjs':'text/javascript','.json':'application/json','.html':'text/html','.png':'image/png','.css':'text/css'}[extname(path)]||'application/octet-stream';
      await r.fulfill({contentType:type,body});
    }catch{await r.fulfill({status:u.pathname==='/favicon.ico'?204:404,body:''});}
  });
}
const mark=(name,details)=>results.tests.push({name,status:'passed',details});
try {
  assert.equal(typeof blueprint.interactions?.editing?.enabled,'boolean','editing applicability missing');
  const editing=blueprint.interactions.editing;
  assert(request.editingRequested!==true||editing.enabled,'explicit requested editing omitted');
  if(editing.enabled)assert(Array.isArray(editing.actions)&&editing.actions.length,'requested edit actions missing');
  results.editing={enabled:editing.enabled,applicability:'Mechanical field check only; independent original-description review required.'};
  let page;
  const bounded=async(p,label)=>{
    let timer;
    try{return await Promise.race([p,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error('controller call timeout: '+label)),15000);})]);}
    finally{clearTimeout(timer);}
  };
  const call=(name,...args)=>bounded(page.evaluate(({name,args})=>window.worldweaver[name](...args),{name,args}),name);
  async function open(profile,mobile=false) {
    context=await chromium.launchPersistentContext(resolve(a.out,profile),{
      executablePath:browserWrapper,env:{...process.env,WW_AUDIT_BROWSER_PID:resolve(a.out,'browser.pid'),
        WW_AUDIT_BROWSER_EXECUTABLE:chromium.executablePath()},headless:true,chromiumSandbox:true,args:a['browser-arg'],timeout:15000,
      viewport:mobile?{width:390,height:844}:{width:1280,height:720},hasTouch:mobile,isMobile:mobile,reducedMotion:'reduce'});
    await route(context);page=await context.newPage();page.setDefaultTimeout(15000);
    page.on('pageerror',e=>results.errors.push(String(e)));
    page.on('console',m=>{if(m.type()==='error')results.errors.push(m.text());});
    await page.goto(a.url);await ready();
  }
  async function ready(){
    await page.waitForFunction(()=>!!window.worldweaver);
    await bounded(page.evaluate(()=>window.worldweaver.ready),'ready');
    await call('setSimulationTime',0);await call('settle');
    assert.equal((await call('snapshot')).simulationTime,0,'simulation clock not controlled');
  }
  const capmap={queueJobs:'pendingJobs',queueBytes:'pendingBytes',residentChunks:'residentChunks',cpuBytes:'cpuBytes',gpuBytes:'gpuBytes',drawCalls:'drawCalls',workers:'workers',uploadsLastFrame:'uploadsPerFrame',particles:'particles',dpr:'dpr'};
  function caps(s){for(const [key,budget] of Object.entries(capmap)){
    assert(Number.isFinite(s[key])&&s[key]>=0&&s[key]<=blueprint.budgets[budget],`${key} bound ${s[key]}`);
  }}
  async function scene(){
    const root=page.locator('[data-ww="scene"]');
    assert.equal(await root.count(),1,'exactly one visible scene root required');
    assert(await root.isVisible(),'scene root hidden');
    assert(await root.evaluate(el=>{
      for(let p=el;p;p=p.parentElement){
        const s=getComputedStyle(p);
        if(s.display==='none'||s.visibility!=='visible'||Number(s.opacity)===0)return false;
      }
      const r=el.getBoundingClientRect();return r.width>0&&r.height>0;
    }),'scene root hidden/transparent');
    assert.equal((await call('snapshot')).simulationTime,0,'simulation clock advanced during action comparison');
    return root.screenshot();
  }
  async function frozenPixels(){
    const pixels=await scene();await call('settle');
    assert.equal(hash(await scene()),hash(pixels),'scene changes without action at fixed time');
    return pixels;
  }
  async function controls(mobile=false){
    for(const [control,field] of [['navigate-forward','player'],['camera-right','camera']]){
      const before=await call('snapshot'),pixels=await frozenPixels();
      assert(before[field]&&typeof before[field]==='object',field+' state missing');
      const button=page.locator(`[data-ww="${control}"]`);
      if(mobile)await button.tap();else await button.click();
      await call('settle');
      assert.notDeepEqual((await call('snapshot'))[field],before[field],control+' UI had no canonical effect');
      assert.notEqual(hash(await scene()),hash(pixels),control+' did not change rendered scene at fixed time');
    }
    mark(mobile?'touch navigation/camera':'navigation/camera UI','Each production control changed canonical transforms and visible scene pixels at simulationTime=0.');
  }
  await open('profile');await controls();
  function position(p){
    assert(Array.isArray(p?.chunk)&&p.chunk.length===3&&p.chunk.every(x=>typeof x==='string'&&/^(0|-?[1-9][0-9]*)$/.test(x)),'invalid canonical player chunk');
    assert(Array.isArray(p.local)&&p.local.length===3&&p.local.every(x=>Number.isFinite(x)&&x>=0&&x<blueprint.coordinates.chunkSize),'invalid normalized player local');
  }
  function destination(s,target){
    position(s.player);
    for(const axis of [0,2]){
      assert.equal(s.player.chunk[axis],target.chunk[axis],'finite fence/wrap/precision changed destination');
      assert.equal(s.player.local[axis],target.local[axis],'horizontal local destination changed');
    }
    const v=s.vertical;
    assert.equal(v?.mode,vertical.mode,'vertical mode differs from admitted scenario');
    assert(Number.isFinite(v.velocityY)&&Math.abs(v.velocityY)<=0.01,'unsettled vertical velocity');
    if(vertical.mode==='flight'){
      assert.equal(s.player.chunk[1],target.chunk[1],'flight destination Y changed');
      assert.equal(s.player.local[1],target.local[1],'flight destination local Y changed');
    }else{
      assert.equal(v.supported,true,'player not supported');
      position(v.support);
      for(const axis of [0,2]){
        assert.equal(v.support.chunk[axis],s.player.chunk[axis],'support not under player');
        assert.equal(v.support.local[axis],s.player.local[axis],'support not under player');
      }
      const dy=BigInt(s.player.chunk[1])-BigInt(v.support.chunk[1]);
      assert(dy>=-10n&&dy<=10n,'support too far vertically');
      const gap=Number(dy)*blueprint.coordinates.chunkSize+s.player.local[1]-v.support.local[1];
      assert(gap>=vertical.clearance[0]-1e-6&&gap<=vertical.clearance[1]+1e-6,'unsupported vertical clearance');
    }
  }
  const travel=async target=>{
    position(target);await call('travel',target);await call('settle');
    destination(await call('snapshot'),target);
  };
  const coords=[['0','0','0'],['1','0','-1'],['-1','0','1'],['9007199254740993','0','-9007199254740993'],['9007199254740994','1','-9007199254740992']];
  const base={};
  for(const chunk of coords){
    await travel({chunk,local:[1,2,1]});caps(await call('stats'));
    base[chunk]=await call('baseHash',chunk);assert.match(base[chunk],/^[0-9a-f]{64}$/);
  }
  for(const chunk of [...coords].reverse()){
    await travel({chunk,local:[1,2,1]});
    assert.equal(await call('baseHash',chunk),base[chunk],'eviction/order changed base');caps(await call('stats'));
  }
  mark('signed/far travel and regeneration',base);
  await travel({chunk:coords[0],local:[1,2,1]});
  let backup,saved;
  if(editing.enabled){
    for(const action of editing.actions){
      assert(scenario.edits?.[action.id],'controller reachable edit target required');
      await travel(scenario.edits[action.id]);
      assert.match(action.id,/^[a-z][a-z0-9-]*$/);
      const before=await call('snapshot'),pixels=await frozenPixels();
      assert(Array.isArray(before.edits),'canonical edit array missing');
      // Real visible controls, not unconditional legacy actions or counter-only diagnostics.
      await page.locator(`[data-ww-edit="${action.id}"]`).click();await call('settle');
      saved=await call('snapshot');
      assert(Array.isArray(saved.edits),'canonical edit array missing');
      assert.notDeepEqual(saved.edits,before.edits,'edit UI had no canonical effect');
      assert.notEqual(hash(await scene()),hash(pixels),'edit had no rendered scene effect');
      await travel({chunk:coords[3],local:[1,2,1]});
      await travel(saved.player);
      assert.deepEqual((await call('snapshot')).edits,saved.edits,'edits lost on far revisit');
      await page.reload();await ready();await travel(saved.player);
      assert.deepEqual((await call('snapshot')).edits,saved.edits,'save lost on reload');
      await context.close();context=null;await unlink(resolve(a.out,'browser.pid')).catch(()=>{});await open('profile');
      await travel(saved.player);
      assert.deepEqual((await call('snapshot')).edits,saved.edits,'edit lost on browser restart');
      mark('direct-edit UI/revisit/reload/restart: '+action.id,'Canonical changes and visible scene effects at fixed time; process restart reused profile. Camera saving is not required.');
      backup=await call('backup');assert.equal(typeof backup,'string');assert(backup.length>20&&backup.length<=8*1024*1024);
      await writeFile(`${a.out}/backup-${action.id}.ndjson`,backup);
      let rejected=false;try{await call('restore','{"corrupt":true}\n');}catch{rejected=true;}
      assert(rejected,'corrupt backup accepted');assert.deepEqual((await call('snapshot')).edits,saved.edits,'corrupt import changed edits');
      // Each action's state (including removal) must restore into truly fresh storage.
      await context.close();context=null;await unlink(resolve(a.out,'browser.pid')).catch(()=>{});await open('restore-'+action.id,true);
      await call('restore',backup);await travel(saved.player);
      assert.deepEqual((await call('snapshot')).edits,saved.edits,'fresh-context restore mismatch');
      mark('backup/corrupt/fresh restore: '+action.id,'Production import/export preserves exact sparse edits.');
      await context.close();context=null;await unlink(resolve(a.out,'browser.pid')).catch(()=>{});await open('profile');
      await travel(saved.player);
    }
  }else{
    assert.equal(await page.locator('[data-ww-edit]').count(),0,'unrequested edit UI');
    results.tests.push({name:'edit persistence',status:'not-run',details:'Declared exploration-only; C08-C10 must independently confirm original intent. No edit/backup API or state required.'});
  }
  await context.close();context=null;await unlink(resolve(a.out,'browser.pid')).catch(()=>{});await open('mobile',true);
  if(editing.enabled){
    await call('restore',backup);await travel(saved.player);
    assert.deepEqual((await call('snapshot')).edits,saved.edits,'mobile restore mismatch');
  }
  await controls(true);
  await page.screenshot({path:`${a.out}/mobile.png`});
  results.screenshots.push({path:'mobile.png',sha256:hash(await readFile(`${a.out}/mobile.png`)),viewport:[390,844],camera:(await call('snapshot')).player});
  await call('setSimulationTime',null);
  await call('pause',true);caps(await call('stats'));await call('pause',false);
  await call('dispose');assert.equal((await call('stats')).workers,0,'workers not cleaned up');
  assert.equal(results.errors.length,0,'browser console/page errors');
  mark('pause/dispose','No page errors; worker count zero on dispose.');
  results.status='passed';
} catch(e){results.status='failed';results.failure=String(e);process.exitCode=1;}
finally {
  if(context){await context.close();await unlink(resolve(a.out,'browser.pid')).catch(()=>{});}
  await writeFile(`${a.out}/browser.json`,JSON.stringify(results,null,2)+'\n');
  console.log(JSON.stringify({status:results.status,tests:results.tests.length,failure:results.failure}));
}
