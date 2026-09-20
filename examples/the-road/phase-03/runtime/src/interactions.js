import {R,SIZE} from './config.js';
import {normalize,integer} from './coordinates.js';
import {roadPoint,support,offset,blocked,clamp} from './generation.js';
export function groundPosition(p){
 if(!p||!Array.isArray(p.chunk)||!Array.isArray(p.local)||p.chunk.length!==3||p.local.length!==3)throw Error('Position requires three signed decimal chunks and three local metre values');
 p.chunk.forEach(integer);if(!p.local.every(v=>Number.isFinite(v)&&v>=0&&v<SIZE))throw Error('Local metres must be within [0,32)');
 const y=normalize('0',support(p),SIZE);return {chunk:[p.chunk[0],y.chunk.toString(),p.chunk[2]],local:[p.local[0],y.local,p.local[2]]};
}
export class Walker{
 constructor(){this.player=groundPosition(roadPoint(0));this.yaw=0;this.pitch=.045;this.steps=0;}
 travel(p){const next=groundPosition(p);if(blocked(next))throw Error('Destination intersects a trunk, hedgebank or parapet. Choose the road center or documented water path.');this.player=next;}
 reset(){this.travel(roadPoint(0));this.yaw=0;this.pitch=.045;}
 look(dx,dy){this.yaw-=dx;this.pitch=clamp(this.pitch+dy,-1.18,1.18);}
 move(forward,right,seconds,brisk=false){
  const speed=brisk?R.briskSpeed:R.navigationSpeed,length=Math.hypot(forward,right);if(!length)return;
  const distance=speed*Math.min(seconds,1),n=Math.max(1,Math.ceil(distance/.14));
  const dx=(Math.sin(this.yaw)*forward-Math.cos(this.yaw)*right)/length*distance/n;
  const dz=(Math.cos(this.yaw)*forward+Math.sin(this.yaw)*right)/length*distance/n;
  for(let i=0;i<n;i++){
   const before=support(this.player),next=offset(this.player,dx,dz);
   if(blocked(next)||Math.abs(support(next)-before)>.34)break;
   this.player=groundPosition(next);this.steps++;
  }
 }
}
export function bindKey(event,input,bindings,changed){
 if(!/^(Key[A-Z]|Digit[0-9])$/.test(event.code)||event.ctrlKey||event.altKey||event.metaKey)return false;
 event.preventDefault();bindings[input.dataset.binding]=event.code;input.value=event.key.toUpperCase();changed();return true;
}
export class Inputs{
 constructor(canvas,walker,canAct,signal){
  this.walker=walker;this.canAct=canAct;this.keys=new Set();this.held=new Set();this.pointer=null;
  this.bindings={forward:'KeyW',back:'KeyS',left:'KeyA',right:'KeyD'};
  const on=(target,event,fn,options={})=>target.addEventListener(event,fn,{...options,signal});
  const isForm=e=>['INPUT','SELECT','TEXTAREA','BUTTON','SUMMARY'].includes(e.target.tagName);
  on(window,'keydown',e=>{
   if(isForm(e)||!canAct())return;
   if(Object.values(this.bindings).includes(e.code)||['ArrowUp','ArrowDown','ArrowLeft','ArrowRight','ShiftLeft','ShiftRight'].includes(e.code)){e.preventDefault();this.keys.add(e.code);}
  });
  on(window,'keyup',e=>this.keys.delete(e.code));on(window,'blur',()=>this.clear());
  on(canvas,'pointerdown',e=>{if(!canAct())return;canvas.focus({preventScroll:true});canvas.setPointerCapture(e.pointerId);this.pointer={id:e.pointerId,x:e.clientX,y:e.clientY};});
  on(canvas,'pointermove',e=>{if(!this.pointer||this.pointer.id!==e.pointerId||!canAct())return;walker.look((e.clientX-this.pointer.x)*.0035,-(e.clientY-this.pointer.y)*.0035);this.pointer.x=e.clientX;this.pointer.y=e.clientY;});
  for(const event of ['pointerup','pointercancel','lostpointercapture'])on(canvas,event,()=>this.pointer=null);
  for(const button of document.querySelectorAll('[data-move]')){
   const action=button.dataset.move;
   on(button,'pointerdown',e=>{if(!canAct())return;button.setPointerCapture(e.pointerId);this.held.add(action);});
   for(const event of ['pointerup','pointercancel','lostpointercapture'])on(button,event,()=>this.held.delete(action));
   on(button,'click',()=>{if(!canAct())return;this.action(action,.2);});
  }
  for(const input of document.querySelectorAll('[data-binding]')){
   on(input,'keydown',e=>bindKey(e,input,this.bindings,()=>this.clear()));
  }
 }
 action(a,dt){if(a==='turnleft')this.walker.look(-dt*1.4,0);else if(a==='turnright')this.walker.look(dt*1.4,0);else this.walker.move(a==='forward'?1:a==='back'?-1:0,a==='right'?1:a==='left'?-1:0,dt);}
 tick(dt){
  if(!this.canAct()){this.clear();return;}
  const has=(a,arrow)=>this.keys.has(this.bindings[a])||this.keys.has(arrow)||this.held.has(a);
  const f=+has('forward','ArrowUp')-has('back','ArrowDown'),r=+has('right','ArrowRight')-has('left','ArrowLeft');
  this.walker.move(f,r,dt,this.keys.has('ShiftLeft')||this.keys.has('ShiftRight'));
  if(this.held.has('turnleft'))this.action('turnleft',dt);if(this.held.has('turnright'))this.action('turnright',dt);
 }
 clear(){this.keys.clear();this.held.clear();this.pointer=null;}
}
