// Independent browser journeys against actual public-command-generated files.
// Set TRACE_TEST_ASK, TRACE_TEST_RECORD, TRACE_PLAYWRIGHT and optionally TRACE_CHROMIUM.
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { spawn, execFileSync } from 'node:child_process';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const require=createRequire(import.meta.url);
const {chromium}=require(process.env.TRACE_PLAYWRIGHT||'playwright');
const here=path.dirname(fileURLToPath(import.meta.url));
const ask=process.env.TRACE_TEST_ASK||'ask', record=process.env.TRACE_TEST_RECORD||'record', python=process.env.TRACE_PYTHON||'python3';
const work=await fs.mkdtemp(path.join(os.tmpdir(),'trace-browser-'));
const output=process.env.TRACE_TEST_OUTPUT||await fs.mkdtemp(path.join(os.tmpdir(),'trace-browser-evidence-'));
await fs.mkdir(output,{recursive:true});
const evidence=path.join(work,'evidence');await fs.mkdir(evidence);
const bytes=Buffer.from(Array.from({length:1024},(_,i)=>i%256));
await fs.writeFile(path.join(work,'artifact.bin'),bytes);
const run=(program,args,input)=>execFileSync(program,args,{cwd:work,input,stdio:['pipe','pipe','pipe'],timeout:30000});
run(record,['run','-ask',ask,'-f',path.join(evidence,'producer.jsonl'),'-output',path.join(work,'artifact.bin'),'--',python,'-c',"import sys;sys.stdout.buffer.write(bytes(range(256))*4);sys.stderr.write('separate stderr')"]);
run(record,['run','-ask',ask,'-f',path.join(evidence,'consumer.jsonl'),'-input',path.join(work,'artifact.bin'),'--','/usr/bin/true']);
const server=spawn(python,[path.join(here,'trace.py'),'--ask',ask,'--record',record,'--no-open','--interval','.1',evidence],{cwd:work,stdio:['ignore','pipe','pipe']});
let stderr='';server.stderr.on('data',c=>stderr+=c);let browser;
const checks=[],errors=[],external=[];
const check=(name,details={})=>{checks.push({name,passed:true,...details});console.log('ok '+name);};
try{
 const url=await new Promise((resolve,reject)=>{let data='';const timer=setTimeout(()=>reject(Error('Server did not start: '+stderr)),30000);server.stdout.on('data',c=>{data+=c;if(data.includes('\n')){clearTimeout(timer);resolve(data.split('\n')[0])}});server.on('exit',code=>{clearTimeout(timer);reject(Error('Server exit '+code+': '+stderr))})});
 browser=await chromium.launch({headless:true,...(process.env.TRACE_CHROMIUM?{executablePath:process.env.TRACE_CHROMIUM}:{})});
 const context=await browser.newContext({viewport:{width:1440,height:1000}});
 await context.route('**/*',route=>{const u=route.request().url();if(u.startsWith(new URL(url).origin)||u.startsWith('file:')||u.startsWith('blob:')||u.startsWith('data:'))return route.continue();external.push(u);return route.abort()});
 const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message||String(e)));
 await page.goto(url);await page.waitForFunction(()=>window.__BENCH_TRACE_DATA__?.sessions.length===2&&!!window.BenchTraceUI);
 assert.equal(await page.locator('.lane').count(),2);assert.equal(await page.locator('.edge').count(),1);
 assert.equal(await page.locator('#follow-button').getAttribute('aria-pressed'),'true');
 check('real receipts, independent lanes, matching-artifact edge and initial live follow');
 const selected=await page.locator('#detail-title').textContent();await page.locator('#prev-button').click();assert.notEqual(await page.locator('#detail-title').textContent(),selected);await page.locator('#next-button').click();
 check('previous and next event');
 await page.locator('#show-all').click();
 await page.locator('#search').fill('stdout · 1024 bytes');await page.locator('.event-row').first().click();
 await page.locator('#tab-streams').click();await page.getByRole('button',{name:'Hex',exact:true}).click();await page.waitForFunction(()=>document.querySelector('#panel-streams').textContent.includes('00000000'));
 const downloadPromise=page.waitForEvent('download');await page.getByRole('button',{name:'Download bytes',exact:true}).click();const download=await downloadPromise;
 assert.deepEqual(await fs.readFile(await download.path()),bytes);check('full binary hex and byte-exact download');
 await page.locator('#tab-raw').click();await page.getByRole('button',{name:'Load complete event',exact:true}).click();await page.waitForFunction(()=>document.querySelector('#panel-raw').textContent.includes('record.chunk/v1'));check('complete raw event via public replay');
 const pausedTitle=await page.locator('#detail-title').textContent();
 const live=path.join(evidence,'live.jsonl');run(ask,['init','-f',live]);
 run(ask,['note','-q','-f',live,'-s','fixture','-k','example/v1','-json','-','-seal'],JSON.stringify({text:'</script><script>window.EVIDENCE_EXECUTED=true</script> __DATA__ __OFFLINE__ __PREFIX__'}));
 await page.waitForFunction(()=>window.__BENCH_TRACE_DATA__.sessions.length===3);
 assert.equal(await page.locator('#detail-title').textContent(),pausedTitle);
 assert.ok((await page.locator('#panel-raw').textContent()).includes('record.chunk/v1'));
 assert.equal(await page.evaluate(()=>window.EVIDENCE_EXECUTED),undefined);check('live append preserves paused selection and loaded raw detail; evidence is inert');
 await page.locator('#reset-filters').click();await page.locator('#follow-button').click();
 run(record,['run','-ask',ask,'-f',path.join(evidence,'new-process.jsonl'),'--','/bin/echo','new observed process']);
 await page.waitForFunction(()=>window.__BENCH_TRACE_DATA__.sessions.length===4);
 assert.equal(await page.locator('#follow-button').getAttribute('aria-pressed'),'true');
 assert.match(await page.locator('#detail-title').textContent(),/complete|accepted|Process/);check('live follow advances to newly completed evidence');
 await page.locator('#status-filter').selectOption('open');assert.ok(await page.locator('#result-count').textContent());await page.locator('#reset-filters').click();
 const exportPromise=page.waitForEvent('download');await page.locator('#export-button').click();const exported=await exportPromise;const offlinePath=path.join(output,'replay.html');await exported.saveAs(offlinePath);
 const offline=await context.newPage();offline.on('pageerror',e=>errors.push(e.message||String(e)));await offline.goto(pathToFileURL(offlinePath).href);await offline.waitForFunction(()=>window.__BENCH_TRACE_DATA__?.live===false&&!!window.BenchTraceUI);
 assert.equal(await offline.locator('#follow-button').isDisabled(),true);
 const offlineStream=await offline.evaluate(async()=>{const s=window.__BENCH_TRACE_DATA__.sessions.find(s=>s.streams.some(t=>t.name==='stdout'&&t.bytes===1024));return window.BenchTrace.readStream(s.id,'stdout')});assert.deepEqual(Buffer.from(offlineStream.base64,'base64'),bytes);
 assert.equal(await offline.evaluate(()=>window.EVIDENCE_EXECUTED),undefined);
 check('single HTML offline export retains exact streams, no source service dependency');
 await page.setViewportSize({width:768,height:900});await page.waitForFunction(()=>document.querySelector('#details-panel').inert);assert.equal(await page.locator('#details-panel').evaluate(e=>e.inert),true);await page.locator('#details-button').click();assert.equal(await page.locator('#details-panel').evaluate(e=>e.inert),false);await page.keyboard.press('Escape');assert.equal(await page.locator('#details-panel').evaluate(e=>e.inert),true);check('mobile inspector focus isolation and Escape');
 for(const width of [1440,768,320]){const responsive=await context.newPage({viewport:{width,height:900}});await responsive.setViewportSize({width,height:900});await responsive.goto(url);await responsive.waitForFunction(()=>!!window.BenchTraceUI&&window.__BENCH_TRACE_DATA__?.sessions.length===4);assert.ok(await responsive.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));if(width===320){assert.equal(await responsive.locator('#filter-body').isVisible(),false);await responsive.locator('#filter-toggle').click();assert.equal(await responsive.locator('#agent-filter').isVisible(),true);await responsive.locator('#filter-toggle').click();assert.equal(await responsive.locator('#connection').isVisible(),true)}await responsive.screenshot({path:path.join(output,'viewport-'+width+'.png'),fullPage:false});await responsive.close();check('responsive '+width)}
 await page.setViewportSize({width:1440,height:1000});await page.emulateMedia({reducedMotion:'reduce'});
 assert.ok(await page.evaluate(()=>matchMedia('(prefers-reduced-motion: reduce)').matches));
 const original=await page.evaluate(()=>structuredClone(window.__BENCH_TRACE_DATA__));
 await page.locator('#narration-toggle').click();await page.waitForFunction(()=>document.querySelector('#narration-status').textContent==='Narration ready.');
 assert.match(await page.locator('#narration-content').textContent(),/Matching evidence across agents/);
 const narrationDownload=page.waitForEvent('download');await page.locator('#narration-download').click();const narrationFile=await narrationDownload;const markdown=await fs.readFile(await narrationFile.path(),'utf8');assert.match(markdown,/# Timeline narration/);assert.match(markdown,/Evidence:/);assert.match(markdown,/not proof/);
 await offline.locator('#narration-toggle').click();assert.match(await offline.locator('#narration-content').textContent(),/Matching evidence across agents/);check('narration from real receipts downloads and works offline');
 const narrated={schema:'bench.trace/v1',revision:400,live:true,clockSync:'unknown',sources:[],lanes:[{id:'story',title:'Builder',kind:'agent'}],sessions:[{id:'story-session',laneId:'story',kind:'agent',verification:'verified',streams:[]}],edges:[],issues:[],events:[
  {type:'start',title:'Start'},
  {type:'action',title:'Build page',details:{argv:['build','page']},status:'complete',duration:9999},
  {type:'result',title:'Build failed',details:{exit:7},status:'failed'},
  {type:'check',title:'Check rejected',details:{outcome:'rejected',verifier_sha256:'same',directory:'/work'},status:'failed',text:'Missing label'},
  {type:'check',title:'Check accepted',details:{outcome:'accepted',verifier_sha256:'same',directory:'/work'},status:'accepted',text:'Label present'},
  {type:'end',title:'Runner accepted',status:'accepted'}
 ].map((e,i)=>({...e,id:'story-'+i,sessionId:'story-session',laneId:'story',order:i,seq:i+1,time:1000+i*100}))};
 await page.evaluate(s=>window.BenchTraceUI.setData(s),narrated);await page.locator('#scrubber').fill('1');assert.match(await page.locator('#narration-content').textContent(),/No process result is recorded/);assert.doesNotMatch(await page.locator('#narration-content').textContent(),/Exit status: 7|later accepted|reached acceptance/);
 await page.locator('#narration-scope').selectOption('all');assert.match(await page.locator('#narration-content').textContent(),/The same check later accepted/);
 await page.locator('[data-chapter-id="recovery:story-4"]').getByRole('button',{name:'Show event',exact:true}).click();assert.equal(await page.locator('#detail-title').textContent(),'Check accepted');check('narration respects cursor and explicit full scope; evidence navigates to the event');
 const manyChapters={...narrated,revision:401,events:Array.from({length:80},(_,i)=>({...narrated.events[0],id:'chapter-'+i,order:i,seq:i+1,time:1000+i*10}))};
 await page.evaluate(s=>window.BenchTraceUI.setData(s),manyChapters);assert.equal(await page.locator('.narration-chapter').count(),50);await page.locator('.narration-more').click();assert.equal(await page.locator('.narration-chapter').count(),80);
 const kept=page.locator('[data-chapter-id="start:chapter-79"] button');await kept.focus();await page.locator('.narration-list').evaluate(e=>e.scrollTop=e.scrollHeight);const oldScroll=await page.locator('.narration-list').evaluate(e=>e.scrollTop);
 await page.evaluate(s=>window.BenchTraceUI.setData(s),{...manyChapters,revision:402});assert.equal(await page.locator('.narration-chapter').count(),80);assert.ok(await page.locator('.narration-list').evaluate(e=>e.scrollTop)>=oldScroll-1);assert.equal(await page.evaluate(()=>document.activeElement.closest('[data-chapter-id]')?.dataset.chapterId),'start:chapter-79');check('narration keeps expanded paragraphs, scroll and keyboard focus during updates');
 await page.keyboard.press('Escape');
 for(const width of [1440,768,320]){await page.setViewportSize({width,height:1000});await page.locator('#narration-panel').scrollIntoViewIfNeeded();assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));await page.screenshot({path:path.join(output,'narration-'+width+'.png')});}check('narration layout at 1440, 768 and 320');
 const speech=await context.newPage();speech.on('pageerror',e=>errors.push(e.message));await speech.addInitScript(()=>{
  const state={calls:[],cancels:0,voices:[{name:'Remote default',lang:'en-US',localService:false,default:true},{name:'Local fixture',lang:'en-US',localService:true}],listeners:[]};window.speechTest=state;
  Object.defineProperty(window,'speechSynthesis',{value:{getVoices:()=>state.voices,addEventListener:(kind,fn)=>state.listeners.push(fn),speak:u=>state.calls.push(u),cancel:()=>{state.cancels++;state.calls.at(-1)?.onend?.()}}});
  window.SpeechSynthesisUtterance=class{constructor(text){this.text=text}};
 });await speech.goto(url);await speech.locator('#narration-toggle').click();assert.equal(await speech.evaluate(()=>speechTest.calls.length),0);await speech.locator('#narration-speak').click();assert.equal(await speech.evaluate(()=>speechTest.calls[0].voice.localService),true);assert.equal(await speech.locator('#narration-stop').isDisabled(),false);await speech.locator('#narration-stop').click();assert.equal(await speech.evaluate(()=>speechTest.calls.length),1);
 await speech.locator('#narration-speak').click();await speech.locator('#prev-button').click();assert.equal(await speech.locator('#narration-stop').isDisabled(),true);
 await speech.evaluate(()=>{speechTest.voices=speechTest.voices.filter(v=>!v.localService);speechTest.listeners.forEach(fn=>fn())});assert.equal(await speech.locator('#narration-speak').isDisabled(),true);assert.match(await speech.locator('#narration-speech-state').textContent(),/no on-device voice/);await speech.close();check('controlled speech API: explicit start, local voice only, stop, cursor cancellation and unavailable fallback');
 await page.setViewportSize({width:1440,height:1000});await page.locator('#narration-toggle').click();
 const synthetic={schema:'bench.trace/v1',revision:500,live:false,clockSync:'unknown',sources:[],lanes:[{id:'many',title:'Explicit UI regression fixture',kind:'agent',state:'complete',verification:'unsealed'}],sessions:[{id:'many-session',laneId:'many',state:'complete',verification:'unsealed',complete:true,streams:[]}],edges:[],issues:[],events:Array.from({length:650},(_,i)=>({id:'test-'+i,sessionId:'many-session',laneId:'many',seq:i+1,order:i,time:1000000+i*20,type:'action',title:'Event '+i,status:'complete',text:'Visible UI regression fixture'}))};
 await page.evaluate(s=>window.BenchTraceUI.setData(s),synthetic);const initialRange=await page.locator('#feed-count').textContent();await page.locator('#feed-earlier').click();assert.notEqual(await page.locator('#feed-count').textContent(),initialRange);check('bounded feed pagination reaches earlier events');
 const allMarks=await page.locator('.event-mark').count();await page.locator('[data-zoom="4"]').click();assert.ok(await page.locator('.event-mark').count()<allMarks/2);check('zoom clips out-of-range events instead of piling them on boundaries');
 await page.locator('#fit-all').click();
 const playback={...synthetic,revision:1000,live:true,events:synthetic.events.slice(0,3).map((e,i)=>({...e,time:1000000+i*400}))};
 await page.evaluate(s=>window.BenchTraceUI.setData(s),playback);await page.locator('#scrubber').fill('0');await page.locator('#play-button').click();
 for(let i=0;i<8;i++){await page.waitForTimeout(100);await page.evaluate(s=>window.BenchTraceUI.setData(s),{...playback,revision:1001+i})}
 assert.notEqual(await page.locator('#detail-title').textContent(),'Event 0');check('replay continues while other snapshot updates arrive');
 await page.evaluate(s=>window.BenchTraceUI.setData(s),original);await page.locator('#reset-filters').click();
 assert.deepEqual(errors,[]);assert.deepEqual(external,[]);check('no runtime exceptions or external resource requests');
}catch(error){checks.push({name:'failure',passed:false,message:error.stack||String(error)});throw error}
finally{
 await fs.writeFile(path.join(output,'browser-checks.json'),JSON.stringify({checks,errors,external,serverDiagnostics:stderr},null,2));
 if(browser)await browser.close();server.kill('SIGINT');await new Promise(resolve=>{if(server.exitCode!==null)return resolve();server.once('exit',resolve);setTimeout(()=>{server.kill('SIGKILL');resolve()},3000).unref()});await fs.rm(work,{recursive:true,force:true});console.log('Browser evidence: '+output);
}
