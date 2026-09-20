import {B,R,SIZE} from './config.js';
import {integer,normalize} from './coordinates.js';
import {canonicalSample,roadPoint,center,authored,support,at} from './generation.js';
import {Walker,Inputs} from './interactions.js';
import {Scheduler} from './scheduler.js';
import {View} from './renderer.js';
import {assetsAvailable} from './readiness.js';
import {EnvironmentalAudio} from './audio.js';

const canvas=document.querySelector('#scene'),statusNode=document.querySelector('#status'),pauseButton=document.querySelector('#pause'),diagnostic=document.querySelector('#diagnostic');
const abort=new AbortController(),signal=abort.signal;
const state={manualPause:false,retained:false,hidden:document.hidden,contextLost:false,loading:true,assetsReady:false,disposed:false,terminal:false,time:0,fixedTime:null,raf:0,last:0,view:null,recoveries:0,frames:[],longTasks:[],coldStartMs:null};
const started=performance.now(),walker=new Walker(),audio=new EnvironmentalAudio();
const running=()=>!state.manualPause&&!state.retained&&!state.hidden&&!state.contextLost&&!state.loading&&!state.disposed&&!state.terminal;
function status(text,fatal=false){statusNode.textContent=text;if(fatal){state.terminal=true;reconcile();}}
let scheduler,inputs,observer;
const on=(target,event,fn)=>target.addEventListener(event,fn,{signal});
function initializeView(){
 state.loading=true;state.assetsReady=false;const view=new View(canvas);state.view=view;
 view.ready.then(()=>{
  if(state.disposed||state.view!==view)return;
  state.loading=false;state.assetsReady=view.assetsReady===true;updateDesired(true);status(state.manualPause?'Paused — materials ready.':'Opening the lane…');reconcile();
 }).catch(e=>{
  if(state.disposed||state.view!==view)return;
  state.loading=false;state.assetsReady=false;view.dispose();status('Local landscape assets could not be loaded. Rebuild/serve the complete dist folder and reload. '+String(e.message),true);
 });
}
try{initializeView();}catch(e){state.terminal=true;statusNode.textContent='This landscape needs WebGL2. Enable graphics acceleration or try a browser with WebGL2 support. '+String(e.message);diagnostic.textContent='Graphics unavailable';}
scheduler=new Scheduler({upload:d=>state.view.upload(d),evict:g=>state.view?.evict(g),status});
inputs=new Inputs(canvas,walker,running,signal);
const media=matchMedia('(prefers-reduced-motion: reduce)'),reduced=document.querySelector('#reduced');reduced.checked=media.matches;
on(media,'change',()=>{reduced.checked=media.matches;});
let scheduleKey='',statsTime=0;
function updateDesired(force=false){
 const key=walker.player.chunk.join(',')+':'+Math.round(walker.yaw/(Math.PI/3));
 if(force||key!==scheduleKey){scheduleKey=key;scheduler.setDesired(walker.player,walker.yaw,force);}
}
function frame(now){
 state.raf=0;if(!running())return;
 const hadLast=!!state.last,ms=hadLast?now-state.last:0;state.last=now;
 const dt=Math.min(ms/1000,.05);inputs.tick(dt);
 if(state.fixedTime===null)state.time+=dt;else state.time=state.fixedTime;
 audio.update(walker.player);updateDesired();scheduler.tick();
 if(state.terminal)return;
 state.view.draw(walker.player,walker.yaw,walker.pitch,state.time,state.fixedTime===null?ms:0,reduced.checked);
 if(hadLast)state.frames.push(ms);if(state.frames.length>5400)state.frames.shift();
 if(state.coldStartMs===null&&scheduler.resident.size>=6){state.coldStartMs=now-started;status('');}
 if(now-statsTime>700){statsTime=now;const s=stats();diagnostic.textContent=`${s.backend} · seed ${B.seed} · version ${B.generationVersion}\n${s.residentChunks} patches · ${s.queueJobs} queued · ${s.drawCalls} draws · DPR ${s.dpr.toFixed(2)}`;}
 state.raf=requestAnimationFrame(frame);
}
function reconcile(){
 const suspend=!running();audio.setActive(!suspend);scheduler?.setPaused(suspend);if(suspend){inputs?.clear();if(state.raf)cancelAnimationFrame(state.raf);state.raf=0;state.last=0;}
 else if(!state.raf)state.raf=requestAnimationFrame(frame);
 pauseButton.textContent=state.manualPause?'Resume':'Pause';pauseButton.setAttribute('aria-pressed',String(state.manualPause));
}
function pause(value){if(state.disposed)return;state.manualPause=!!value;reconcile();if(!state.terminal)status(state.manualPause?'Paused — take your time.':'');}
on(pauseButton,'click',()=>pause(!state.manualPause));
on(document.querySelector('#sound'),'click',async e=>{
 if(!running())return;
 await audio.toggleFromGesture(running(),walker.player);
 e.target.textContent=audio.enabled?'Sound on':'Sound off';e.target.setAttribute('aria-pressed',String(audio.enabled));
 if(audio.error)status('Sound unavailable: '+audio.error);
});
on(window,'keydown',e=>{if(e.code==='Escape'){e.preventDefault();pause(!state.manualPause);}});
function reset(){if(!running())return;walker.reset();scheduleKey='';updateDesired(true);inputs.clear();}
on(document.querySelector('#reset'),'click',reset);on(document.querySelector('#guide-reset'),'click',reset);
export const routePositions=()=>({
 arrival:{...roadPoint(0),yaw:0,pitch:.045},
 oak:{...roadPoint(R.oakStation-8,-.6),yaw:.22,pitch:.025},
 bridge:{...roadPoint(R.bridgeStation-11),yaw:0,pitch:-.045},
 water:{...roadPoint(R.bridgeStation-6,-10.6),yaw:-1.2,pitch:-.13},
 milestone:{...roadPoint(R.milestoneStation-6),yaw:.12,pitch:.02},
 crest:{...roadPoint(R.crestStation+8),yaw:0,pitch:.06},
 village:{...roadPoint(-18),yaw:Math.PI,pitch:.02}
});
function jump(name){if(!running())return;const p=routePositions()[name];if(!p)return;walker.travel(p);walker.yaw=p.yaw;walker.pitch=p.pitch;inputs.clear();scheduleKey='';updateDesired(true);}
on(document.querySelector('#landmark'),'change',e=>{jump(e.target.value);e.target.value='';});
on(window,'resize',()=>{if(!state.disposed&&!state.contextLost)state.view?.resize();});
on(document,'visibilitychange',()=>{state.hidden=document.hidden;reconcile();});
on(window,'pagehide',e=>{if(e.persisted){state.retained=true;reconcile();}else dispose();});
on(window,'pageshow',e=>{if(e.persisted&&!state.disposed){state.retained=false;state.hidden=document.hidden;reconcile();}});
on(canvas,'webglcontextlost',e=>{e.preventDefault();state.contextLost=true;state.assetsReady=false;reconcile();status('The graphics context was interrupted. Waiting to restore the view…');});
on(canvas,'webglcontextrestored',()=>{
 if(state.disposed)return;
 if(++state.recoveries>3){status('Graphics recovery stopped after repeated interruptions. Please reload.',true);return;}
 try{scheduler.clearResidents();state.view?.dispose();initializeView();state.contextLost=false;updateDesired(true);status(state.manualPause?'Paused — view restored.':'');reconcile();}
 catch(e){status('The view could not be restored. Please reload. '+String(e.message),true);}
});
try{observer=new PerformanceObserver(list=>{for(const e of list.getEntries()){state.longTasks.push(e.duration);if(state.longTasks.length>512)state.longTasks.shift();}});observer.observe({entryTypes:['longtask']});}catch{}
function stats(){
 const s=scheduler.stats(),v=state.view?.stats()||{backend:'unavailable',drawCalls:0,dpr:0,particles:0,renderTargetBytes:0};
 return {...s,...v,gpuBytes:s.gpuBytes+v.renderTargetBytes+(v.assetTextureBytes||0),cpuBytes:s.cpuBytes+(v.assetCpuBytes||0)+audio.bufferBytes,assetsReady:assetsAvailable(state),manualPause:state.manualPause,hidden:state.hidden,retained:state.retained,disposed:state.disposed,contextLost:state.contextLost,rafCount:state.raf?1:0};
}
function dispose(){
 if(state.disposed)return;state.disposed=true;state.assetsReady=false;reconcile();audio.dispose();abort.abort();observer?.disconnect();scheduler.dispose();state.view?.dispose();inputs.clear();
}
function settle(){
 const begun=performance.now(),minimum=(state.view?.frames||0)+2;
 return new Promise((resolve,reject)=>{
  function poll(){
   if(state.disposed)return reject(Error('World disposed'));
   if(state.terminal||scheduler.failure)return reject(Error(scheduler.failure||statusNode.textContent));
   if(!running()&&!state.loading)return reject(Error('Resume and show this page before settling'));
   if(!state.loading&&scheduler.settled&&state.view.frames>=minimum)return resolve();
   if(performance.now()-begun>60000)return reject(Error('Landscape settling exceeded 60 seconds'));
   setTimeout(poll,50);
  }poll();
 });
}
async function digest(value){const data=new TextEncoder().encode(JSON.stringify(value)),hash=await crypto.subtle.digest('SHA-256',data);return [...new Uint8Array(hash)].map(n=>n.toString(16).padStart(2,'0')).join('');}
reconcile();
if(new URLSearchParams(location.search).get('test')==='1'){
 const ready=state.terminal?Promise.reject(Error(statusNode.textContent)):settle();ready.catch(()=>{});
 window.worldweaver=Object.freeze({
  ready,settle,
  async travel(p){if(!running())throw Error('Resume the visible page before travelling');walker.travel(p);scheduleKey='';updateDesired(true);return settle();},
  baseHash:chunk=>digest(canonicalSample(chunk.map(v=>integer(v).toString()))),
  snapshot:()=>({player:structuredClone(walker.player),camera:{yaw:walker.yaw,pitch:walker.pitch,eyeHeight:1.68,fov:66},simulationTime:state.time,vertical:{mode:'grounded',velocityY:0,supported:true,support:structuredClone(walker.player)}}),
  stats,pause,dispose,
  setSimulationTime(value){if(value!==null&&(!Number.isFinite(value)||value<0))throw Error('Nonnegative finite seconds or null required');state.fixedTime=value;if(value!==null)state.time=value;},
  routePositions:()=>structuredClone(routePositions()),
  particleSample:()=>state.view?.particleSnapshot()||{particles:[]},
  audioState:()=>audio.snapshot(),
  async visit(name){jump(name);return settle();},
  supportAt:p=>({heightMetres:support(p),eyeHeightMetres:1.68,mode:'canonical grounded terrain/road/bridge/ramp'}),
  measurements:()=>({coldStartMs:state.coldStartMs,framesMs:[...state.frames],longTasksMs:[...state.longTasks],stats:stats(),memoryMethod:'Conservative estimates: transferred buffers x2 for CPU/GPU conversion, 32 MiB per worker, 8 MiB CPU shared, 12 MiB GPU shared, plus actual render-target pixel dimensions x32 and 2048² x8 shadow color/depth. Plus eight decoded 1k RGBA mipmapped scans, foliage/glyph atlas, actual PMREM target dimensions and 56 MiB CPU image/loading reserve. The enabled eight-second shared mono audio buffer is counted at its actual sample rate. Not measured GPU allocation.'})
 });
}
