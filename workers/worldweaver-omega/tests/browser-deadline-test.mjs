// Watchdog process-contract tests, NOT browser/renderer/semantic evidence.
import {resolve,join} from 'node:path';
import {mkdir,writeFile,readFile} from 'node:fs/promises';
import {spawn} from 'node:child_process';
import assert from 'node:assert/strict';
const home=resolve(process.env.WW_TEST_HOME||'expert');
const {processTable}=await import('file://'+join(home,'lib/browser-cleanup.mjs'));
const out=resolve(process.argv[2]);
await mkdir(out,{recursive:false});
const results=[];
for(const mode of ['promise','blocked-js','blocked-close']){
 const dir=join(out,mode);await mkdir(dir);
 const stub=join(dir,'playwright-process-fixture.mjs');
 await writeFile(stub,`
 // Deliberately NOT Playwright. Only exercise host cleanup when an edge stalls.
 import {spawn} from 'node:child_process';
 import {writeFileSync} from 'node:fs';
 export const chromium={executablePath:()=>process.execPath,launchPersistentContext:async(profile,options)=>{
  const child=spawn(options.executablePath,['-e','setInterval(()=>{},1000)'],{stdio:'ignore',detached:true,env:options.env});
  writeFileSync(${JSON.stringify(join(dir,'started.json'))},JSON.stringify({pid:process.pid,descendant:child.pid,mode:${JSON.stringify(mode)}}));
  const page={setDefaultTimeout(){},on(){},goto:async()=>{},waitForFunction:async()=>{},
   evaluate:async()=>{
    ${mode==='blocked-js'?'while(true){}':mode==='blocked-close'?"throw Error('intentional adapter failure')":'await new Promise(()=>{})'}
   }};
  return {newPage:async()=>page,close:async()=>{${mode==='blocked-close'?'while(true){}':'await new Promise(()=>{})'}}};
 }};
 `);
 const request=join(dir,'request.json'),blueprint=join(dir,'blueprint.json');
 await writeFile(request,JSON.stringify({roughDescription:'Process fixture only',style:'none'}));
 await writeFile(blueprint,JSON.stringify({interactions:{editing:{enabled:false}}}));
 const audit=join(dir,'audit'),start=Date.now();
 const child=spawn(process.execPath,[join(home,'tools/browser-audit.mjs'),'--url','http://localhost:8080',
  '--request',request,'--blueprint',blueprint,'--out',audit,'--playwright',stub,'--boundary','PROCESS-FIXTURE-NOT-BROWSER',
  '--candidate-hash','0'.repeat(64),'--deadline-ms','1200'],{stdio:'inherit'});
 const exit=await new Promise((res,rej)=>{child.on('exit',res);child.on('error',rej);});
 assert.equal(exit,1);assert(Date.now()-start<6000);
 const started=JSON.parse(await readFile(join(dir,'started.json'),'utf8'));
 const report=JSON.parse(await readFile(join(audit,'supervisor.json'),'utf8'));
 assert.equal(report.timedOut,true);
 const denied=process.env.WW_TEST_EXPECT_PS_DENIED==='1';
 if(denied){assert.match(report.killError,/spawnSync ps EPERM/);assert.equal(report.cleanup.confirmed,false);}
 else assert.equal(report.killError,null);
 assert.equal(JSON.parse(await readFile(join(audit,'browser.json'),'utf8')).status,'failed');
 await new Promise(r=>setTimeout(r,300));
 if(denied){
  for(const pid of [report.pid,started.descendant,-report.processGroup,...report.browserGroups.map(p=>-p)])
   assert.throws(()=>process.kill(pid,0),e=>e.code==='ESRCH','process/group remains');
 }else{
  assert.equal(report.cleanup.confirmed,true);
  const live=processTable().filter(p=>!p.state.startsWith('Z'));
  assert(!live.some(p=>p.pid===report.pid||p.pid===started.descendant||[report.processGroup,...report.browserGroups].includes(p.pgid)),'LIVE process/group leaked');
 }
 assert(report.browserGroups.includes(started.descendant),'detached browser group not registered/killed');
 results.push({mode,exit,elapsedMs:report.elapsedMs,cleanup:denied?'audit correctly failed inspection; independent kill(0) found ESRCH':'no live group/descendant processes'});
}
await writeFile(join(out,'results.json'),JSON.stringify({scope:'Process fixtures only, not actual Chromium page validation',results},null,2));
console.log('3/3 synthetic process deadlines; inspectionDeniedExpected='+ (process.env.WW_TEST_EXPECT_PS_DENIED==='1'));
