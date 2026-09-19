// Real Chromium/UI/Three/IDB protocol fixtures, not fresh generated worlds.
// Labels and expected failures are fixed independently of audit verdicts.
import {resolve,join} from 'node:path';
import {mkdir,writeFile,readFile,copyFile} from 'node:fs/promises';
import {spawn} from 'node:child_process';
import assert from 'node:assert/strict';
const deps=resolve(process.argv[2]||'../browser-runtime/node_modules');
const home=resolve(process.env.WW_TEST_HOME||'expert');
const {processTable}=await import('file://'+join(home,'lib/browser-cleanup.mjs'));
const root=resolve(process.argv[3]||'tests/scope-results/browser-'+Date.now());
await mkdir(root,{recursive:false});
const budgets={pendingJobs:64,pendingBytes:67108864,residentChunks:256,cpuBytes:268435456,gpuBytes:268435456,drawCalls:512,workers:4,uploadsPerFrame:4,particles:8192,dpr:2};
const cases=[
 {mode:'flight',editing:false,expected:'passed'},
 {mode:'unsupported',editing:false,expected:'failed',failure:/player not supported/},
 {mode:'bad-height',editing:false,expected:'failed',failure:/unsupported vertical clearance/},
 ...['hung-ready','hung-js','hung-travel'].map(mode=>({mode,editing:false,expected:'failed',failure:/controller deadline/})),
 {mode:'exploration',editing:false,expected:'passed'},
 {mode:'direct-edit',editing:true,expected:'passed'},
 {mode:'fence',editing:false,expected:'failed',failure:/finite fence/},
 {mode:'queue',editing:false,expected:'failed',failure:/queueJobs bound/},
 {mode:'omitted',editing:false,requested:true,expected:'failed',failure:/requested editing omitted/},
 {mode:'lost-restart',editing:true,expected:'failed',failure:/edit lost on browser restart/},
 {mode:'counter-only',editing:true,expected:'failed',failure:/edit UI had no canonical effect/},
 {mode:'dom-scene',editing:false,expected:'passed'},
 {mode:'hidden-scene',editing:false,expected:'failed',failure:/scene root hidden/},
 {mode:'moving-scene',editing:false,expected:'passed'},
 {mode:'motion-only-nav',editing:false,expected:'failed',failure:/navigate-forward did not change rendered/},
 {mode:'motion-only-look',editing:false,expected:'failed',failure:/camera-right did not change rendered/},
 {mode:'motion-only-edit',editing:true,expected:'failed',failure:/edit had no rendered scene effect/},
 {mode:'uncontrolled-clock',editing:false,expected:'failed',failure:/scene changes without action/}
];
const results=[];
for(const c of cases){
 const {mode,editing}=c,runtime=join(root,mode);await mkdir(runtime);
 for(const file of ['index.html','main.mjs'])await copyFile(join(process.env.WW_TEST_FIXTURES||'tests/fixtures','scope-runtime',file),join(runtime,file));
 for(const file of ['coordinates.mjs','noise.mjs','store.mjs'])await copyFile(join(home,'templates/core',file),join(runtime,file));
 for(const file of ['three.module.js','three.core.js'])await copyFile(join(deps,'three/build',file),join(runtime,file));
 await writeFile(join(runtime,'config.json'),JSON.stringify({mode,editing}));
 const request={roughDescription:'Explore rolling terrain with decorative lights.'+(editing||c.requested?' Let me directly place and remove lights.':''),style:'Low-Poly Diorama',...((editing||c.requested)?{editingRequested:true}:{})};
 const blueprint={coordinates:{chunkSize:16},budgets,interactions:{editing:{enabled:editing,requestSpans:editing?['Let me directly place and remove lights.']:[],
  ...(editing?{actions:[{id:'place',action:'place light',effect:'visible persistent light'},{id:'remove',action:'remove light',effect:'persistent removal'}]}:{})}}};
 const requestPath=join(root,mode+'-request.json'),blueprintPath=join(root,mode+'-blueprint.json');
 await writeFile(requestPath,JSON.stringify(request));await writeFile(blueprintPath,JSON.stringify(blueprint));
 const scenarioPath=join(root,mode+'-scenario.json');
 await writeFile(scenarioPath,JSON.stringify({vertical:mode==='flight'?{mode:'flight'}:{mode:'grounded',clearance:[1.9,2.1]},...(editing?{edits:{place:{chunk:['0','0','0'],local:[1,2,1]},remove:{chunk:['0','0','0'],local:[1,2,1]}}}:{})}));
 const out=join(root,mode+'-audit');
 const args=[join(home,'tools/browser-audit.mjs'),'--scenario',scenarioPath,'--deadline-ms',mode.startsWith('hung-')?'7000':'180000',...(mode==='flight'?['--browser-arg=--use-angle=metal']:[]),'--url','http://localhost:8080','--runtime-dir',runtime,'--request',requestPath,'--blueprint',blueprintPath,'--out',out,'--playwright',join(deps,'playwright/index.mjs'),'--boundary','existing-authoring-browser-boundary','--candidate-hash','0'.repeat(64)];
 const code=await new Promise((res,rej)=>{const p=spawn(process.execPath,args,{stdio:'inherit'});p.on('error',rej);p.on('exit',res);});
 const supervisor=JSON.parse(await readFile(join(out,'supervisor.json'),'utf8'));
 assert.equal(supervisor.killError,null);
 if(mode.startsWith('hung-')){
  assert.equal(supervisor.timedOut,true);
  assert(supervisor.elapsedMs<12000,'controller exceeded deadline/cleanup bound');
  await new Promise(r=>setTimeout(r,300));
  assert(supervisor.browserGroups.length>0,'no browser launched before deadline');
 assert.equal(supervisor.cleanup.confirmed,true);
 const live=processTable().filter(p=>!p.state.startsWith('Z'));
 assert(!live.some(p=>p.pid===supervisor.pid||[supervisor.processGroup,...supervisor.browserGroups].includes(p.pgid)),'LIVE process/group leaked');
 }
 const report=JSON.parse(await readFile(join(out,'browser.json'),'utf8'));
 results.push({mode,expected:c.expected,actual:report.status,reason:report.failure,exit:code});
 await writeFile(join(root,'summary.json'),JSON.stringify({scope:'Synthetic scope protocol only, not world quality',results},null,2));
 assert.equal(code,c.expected==='passed'?0:1,mode+': '+report.failure);
 assert.equal(report.status,c.expected,mode);
 if(c.failure)assert.match(report.failure,c.failure,mode);
 if(!editing&&c.expected==='passed'){
  assert.equal(report.tests.find(t=>t.name==='edit persistence').status,'not-run');
 }
}
console.log(`Scope/breadth browser probes: ${results.length}/${cases.length} expected outcomes (${cases.filter(c=>c.expected==='passed').length} positives).`);
