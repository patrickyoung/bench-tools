import {B,SIZE} from './config.js';
import {integer,chunkKey,floorDiv} from './coordinates.js';
import {terrain,buffersOf} from './generation.js';
export const NEAR_RESERVE=524288,COARSE_RESERVE=4194304;
export function desiredChunks(player,yaw=0){
 const desired=[];
 // Bounded radial XYZ enumeration, rejecting empty vertical layers using canonical support.
 for(let dz=-4;dz<=4;dz++)for(let dx=-4;dx<=4;dx++)for(let dy=-2;dy<=2;dy++){
  const radius=Math.hypot(dx,dy,dz);if(radius>4.4)continue;
  const chunk=[(integer(player.chunk[0])+BigInt(dx)).toString(),(integer(player.chunk[1])+BigInt(dy)).toString(),(integer(player.chunk[2])+BigInt(dz)).toString()];
  const ground=terrain({chunk,local:[16,0,16]});
  if(BigInt(Math.floor(ground/SIZE))!==integer(chunk[1]))continue;
  const ahead=(Math.sin(yaw)*dx+Math.cos(yaw)*dz);
  if(Math.hypot(dx,dz)>3.7&&ahead<2.2)continue;
  desired.push({key:chunkKey(...chunk),chunk,kind:'near',priority:radius-.22*ahead});
 }
 desired.sort((a,b)=>a.priority-b.priority||a.key.localeCompare(b.key));
 const chunk=[(floorDiv(integer(player.chunk[0]),4n)*4n).toString(),'0',(floorDiv(integer(player.chunk[2]),4n)*4n).toString()];
 desired.splice(Math.min(6,desired.length),0,{key:'coarse:'+chunkKey(...chunk),chunk,kind:'coarse',priority:2});
 return desired.slice(0,80);
}
export class Scheduler{
 constructor({upload,evict,status,workerFactory=()=>new Worker(new URL('./chunk-worker.js',import.meta.url),{type:'module'})}){
  Object.assign(this,{upload,evict,status,workerFactory});this.resident=new Map();this.tickets=new Map();this.completed=[];this.slots=[];
  this.desired=[];this.wanted=new Set();this.epoch=0;this.serial=0;this.queueBytes=0;this.paused=false;this.disposed=false;this.failure=null;this.boots=0;this.maxBoots=6;this.maxResidentBytes=12582912;this.retries=new Map();this.uploadsLastFrame=0;this.workerErrors=0;this.staleDiscards=0;
  for(let i=0;i<B.budgets.workers;i++)this.slots.push({worker:null,ready:false,job:null,started:0});
 }
 boot(slot){
  if(this.paused||this.disposed||this.failure)return;
  if(this.boots>=this.maxBoots){this.fatal('Landscape workers could not start. Reload to try again; your walk has not been saved.');return;}
  this.boots++;slot.started=performance.now();
  try{
   slot.worker=this.workerFactory();
   slot.worker.onmessage=e=>this.message(slot,e.data);
   slot.worker.onerror=e=>{e.preventDefault?.();this.failSlot(slot,'Worker failed');};
   slot.worker.onmessageerror=()=>this.failSlot(slot,'Worker reply could not be read');
  }catch(e){this.failSlot(slot,String(e));}
 }
 release(t){if(!t||!this.tickets.has(t.id))return;this.queueBytes-=t.reserved;this.tickets.delete(t.id);}
 failSlot(slot,why){
  if(this.disposed)return;
  this.workerErrors++;slot.worker?.terminate();slot.worker=null;slot.ready=false;
  const t=slot.job;slot.job=null;if(t){this.release(t);const n=(this.retries.get(t.key)||0)+1;this.retries.set(t.key,n);if(n>2){this.fatal(`Landscape generation stopped: ${why}. Please reload.`);return;}}
  // Never replace here. Only an unpaused tick may boot, with a separate total boot cap.
 }
 fatal(message){this.failure=message;this.status(message,true);for(const s of this.slots){s.worker?.terminate();s.worker=null;s.ready=false;s.job=null;}for(const t of [...this.tickets.values()])this.release(t);this.completed.length=0;}
 message(slot,m){
  if(this.disposed)return;
  if(m.type==='ready'){slot.ready=true;return;}
  const t=slot.job;
  if(!t||m.id!==t.id)return;
  if(m.type==='job-error'){this.failSlot(slot,m.message);return;}
  slot.job=null;
  const valid=m.type==='result'&&m.seed===B.seed&&m.version===B.generationVersion&&m.epoch===t.epoch&&m.revision===0&&JSON.stringify(m.chunk)===JSON.stringify(t.chunk);
  const actual=valid?buffersOf(m.payload).reduce((n,b)=>n+b.byteLength,0):Infinity;
  if(!valid||actual>t.reserved){this.release(t);this.fatal('Invalid landscape reply; reload to recover.');return;}
  if(t.epoch!==this.epoch||!this.wanted.has(t.key)){this.staleDiscards++;this.release(t);return;}
  if(this.completed.length>=4){this.release(t);return;}
  this.completed.push({t,payload:m.payload,bytes:actual});
 }
 setDesired(player,yaw,force=false){
  const list=desiredChunks(player,yaw),keys=list.map(t=>t.key).sort().join('|');
  if(!force&&keys===this.signature){this.desired=list;return;}
  this.signature=keys;this.epoch++;this.desired=list;this.wanted=new Set(list.map(t=>t.key));this.player=player;
  for(const c of [...this.completed])if(c.t.epoch!==this.epoch){this.completed.splice(this.completed.indexOf(c),1);this.release(c.t);this.staleDiscards++;}
  for(const [key,r] of this.resident){
   const dx=integer(r.chunk[0])-integer(player.chunk[0]),dz=integer(r.chunk[2])-integer(player.chunk[2]);
   const outside=dx>6n||dx< -6n||dz>6n||dz< -6n||(Number(dx*dx+dz*dz)>34);
   if(r.kind==='near'&&outside){this.evict(r.object);this.resident.delete(key);}
  }
 }
 tick(){
  this.uploadsLastFrame=0;if(this.disposed||this.paused||this.failure)return;
  for(const s of this.slots){
   if(!s.worker)this.boot(s);
   else if((!s.ready||s.job)&&performance.now()-s.started>45000)this.failSlot(s,'Worker timeout');
  }
  for(let n=0;n<B.budgets.uploadsPerFrame&&this.completed.length;n++){
   const c=this.completed.shift();this.release(c.t);
   if(c.t.epoch!==this.epoch||!this.wanted.has(c.t.key)){this.staleDiscards++;continue;}
   if(c.t.kind==='coarse')for(const [key,r] of this.resident)if(r.kind==='coarse'){this.evict(r.object);this.resident.delete(key);}
   if(this.resident.size>=B.budgets.residentChunks){const old=[...this.resident].find(([k])=>!this.wanted.has(k));if(old){this.evict(old[1].object);this.resident.delete(old[0]);}else continue;}
   let residentBytes=[...this.resident.values()].reduce((n,r)=>n+r.bytes,0);
   for(const [key,r] of this.resident){
    if(residentBytes+c.bytes<=this.maxResidentBytes)break;
    if(!this.wanted.has(key)){residentBytes-=r.bytes;this.evict(r.object);this.resident.delete(key);}
   }
   if(residentBytes+c.bytes>this.maxResidentBytes){this.fatal('Landscape detail exceeded its memory allowance. Reload to recover.');return;}
   const object=this.upload(c.payload);this.resident.set(c.t.key,{chunk:c.t.chunk,kind:c.t.kind,object,bytes:c.bytes});this.uploadsLastFrame++;
  }
  for(const s of this.slots){
   if(!s.ready||s.job||this.completed.length>=4||this.tickets.size>=B.budgets.pendingJobs)continue;
   const active=new Set([...this.tickets.values()].map(t=>t.key));
   const d=this.desired.find(d=>!this.resident.has(d.key)&&!active.has(d.key));if(!d)continue;
   const reserved=d.kind==='coarse'?COARSE_RESERVE:NEAR_RESERVE;
   if(this.queueBytes+reserved>B.budgets.pendingBytes||this.stats().cpuBytes+reserved>B.budgets.cpuBytes)continue;
   const t={...d,id:++this.serial,epoch:this.epoch,seed:B.seed,version:B.generationVersion,revision:0,reserved};
   this.queueBytes+=reserved;this.tickets.set(t.id,t);s.job=t;s.started=performance.now();
   try{s.worker.postMessage(t);}catch(e){this.failSlot(s,String(e));}
  }
 }
 get settled(){return !this.failure&&this.desired.length>0&&this.desired.every(d=>this.resident.has(d.key))&&this.tickets.size===0&&this.completed.length===0;}
 setPaused(value){this.paused=value;}
 stats(){
  const bytes=[...this.resident.values()].reduce((n,r)=>n+r.bytes,0);
  return {queueJobs:this.tickets.size,queueBytes:this.queueBytes,residentChunks:this.resident.size,completedJobs:this.completed.length,desiredJobs:this.desired.length,workers:this.slots.filter(s=>s.worker).length,workerBoots:this.boots,workerErrors:this.workerErrors,staleDiscards:this.staleDiscards,uploadsLastFrame:this.uploadsLastFrame,
   cpuBytes:this.disposed?0:bytes*2+this.queueBytes+this.slots.filter(s=>s.worker).length*33554432+8388608,
   gpuBytes:this.disposed?0:bytes*2+12582912};
 }
 clearResidents(){for(const r of this.resident.values())this.evict(r.object);this.resident.clear();this.signature='';}
 dispose(){if(this.disposed)return;this.disposed=true;for(const s of this.slots){s.worker?.terminate();s.worker=null;s.job=null;}for(const t of [...this.tickets.values()])this.release(t);this.completed.length=0;this.clearResidents();this.desired=[];this.wanted.clear();}
}
