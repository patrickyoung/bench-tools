import {R} from './config.js';
import {authored,tunnel,clamp} from './generation.js';
export function soundProfile(player){
 const a=authored(player),inside=!!a&&tunnel(a.s),distance=a?Math.abs(a.s-R.bridgeStation):Infinity;
 return {canopy:inside,windGain:inside?.035:.08,leavesGain:inside?.018:.035,streamGain:.16*Math.pow(clamp(1-distance/42),2),cutoffHz:inside?850:5600};
}
export class EnvironmentalAudio{
 constructor(factory=()=>new (globalThis.AudioContext||globalThis.webkitAudioContext)()){
  this.factory=factory;this.context=null;this.enabled=false;this.userActivated=false;this.active=false;this.disposed=false;this.error=null;this.bufferBytes=0;this.nodes=[];this.sources=[];this.queue=Promise.resolve();this.profile={canopy:false,windGain:0,leavesGain:0,streamGain:0,cutoffHz:5600};
 }
 async toggleFromGesture(active,player){
  if(this.disposed||!active)return;
  try{
   if(!this.context){
    this.userActivated=true;const ctx=this.context=this.factory();
    const buffer=ctx.createBuffer(1,ctx.sampleRate*8,ctx.sampleRate),data=buffer.getChannelData(0);this.bufferBytes=data.byteLength;
    let r=98371,brown=0;
    for(let i=0;i<data.length;i++){r=(Math.imul(r,1664525)+1013904223)>>>0;brown=(brown+.035*(r/4294967296*2-1))/.9998;brown=Math.max(-.55,Math.min(.55,brown));data[i]=(r/4294967296*2-1)*.35+brown*.3;}
    this.master=ctx.createGain();this.master.gain.value=0;this.master.connect(ctx.destination);this.nodes.push(this.master);
    this.filter=ctx.createBiquadFilter();this.filter.type='lowpass';this.filter.frequency.value=5600;this.filter.connect(this.master);this.nodes.push(this.filter);
    this.gains={};
    for(const [name,type,hz] of [['wind','lowpass',500],['leaves','bandpass',1600],['stream','bandpass',2300]]){
     const source=ctx.createBufferSource(),filter=ctx.createBiquadFilter(),gain=ctx.createGain();
     source.buffer=buffer;source.loop=true;filter.type=type;filter.frequency.value=hz;filter.Q.value=.55;gain.gain.value=0;
     source.connect(filter);filter.connect(gain);gain.connect(this.filter);source.start(0,this.sources.length*1.7);
     this.sources.push(source);this.nodes.push(filter,gain);this.gains[name]=gain;
    }
   }
   this.enabled=!this.enabled;this.active=active;this.update(player);await this.synchronize();
  }catch(e){this.error=String(e.message);this.enabled=false;this.master?.gain.setValueAtTime(0,this.context.currentTime);await this.context?.suspend().catch(()=>{});}
 }
 update(player){
  this.profile=soundProfile(player);if(!this.context||this.disposed)return;
  const key=JSON.stringify(this.profile);if(this.lastProfile===key)return;this.lastProfile=key;
  const t=this.context.currentTime;
  for(const name of ['wind','leaves','stream']){const gain=this.gains[name].gain;const value=gain.value;gain.cancelScheduledValues(0);gain.setValueAtTime(value,t);gain.setTargetAtTime(this.profile[name+'Gain'],t,.12);}
  const f=this.filter.frequency,value=f.value;f.cancelScheduledValues(0);f.setValueAtTime(value,t);f.setTargetAtTime(this.profile.cutoffHz,t,.15);
 }
 setActive(active){this.active=active;if(!active&&this.master)this.master.gain.setValueAtTime(0,this.context.currentTime);return this.synchronize();}
 synchronize(){
  if(!this.context)return this.queue;
  this.queue=this.queue.catch(()=>{}).then(async()=>{
   if(this.disposed)return;
   const run=this.enabled&&this.active,ctx=this.context;
   if(run){await ctx.resume();if(!this.disposed&&this.enabled&&this.active){this.master.gain.cancelScheduledValues(0);this.master.gain.setTargetAtTime(.18,ctx.currentTime,.08);}}
   else{this.master.gain.cancelScheduledValues(0);this.master.gain.setValueAtTime(0,ctx.currentTime);await ctx.suspend();}
  }).catch(e=>{this.error=String(e.message);this.master?.gain.setValueAtTime(0,this.context.currentTime);});
  return this.queue;
 }
 snapshot(){return {bufferBytes:this.bufferBytes,enabled:this.enabled,userActivated:this.userActivated,active:this.active&&!this.disposed,contextState:this.context?.state||'not-created',disposed:this.disposed,masterGain:this.master?.gain.value||0,...this.profile,error:this.error};}
 dispose(){
  if(this.disposed)return;this.disposed=true;this.enabled=false;this.active=false;
  if(this.master)this.master.gain.setValueAtTime(0,this.context.currentTime);
  for(const s of this.sources){try{s.stop();}catch{}s.disconnect();s.buffer=null;}
  for(const n of this.nodes)n.disconnect();
  this.sources=[];this.nodes=[];this.bufferBytes=0;this.context?.close().catch(()=>{});
 }
}
