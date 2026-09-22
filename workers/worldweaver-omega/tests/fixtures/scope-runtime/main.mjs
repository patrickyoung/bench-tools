// Synthetic real renderer/input/storage fixture. NOT a complete generated world.
// Fault modes are labeled by the external test, never by the adapter's verdict.
import * as T from './three.module.js';
import {normalize} from './coordinates.mjs';
import {perlin3} from './noise.mjs';
import {openStore} from './store.mjs';
const {mode,editing}=await (await fetch('/config.json')).json();
const canvas=document.querySelector('canvas');
const renderer=new T.WebGLRenderer({canvas,antialias:false,preserveDrawingBuffer:true});
renderer.setSize(innerWidth,Math.round(innerHeight*.65),false);
const dom=mode==='dom-scene'?document.createElement('div'):null;
if(dom){
 canvas.removeAttribute('data-ww');canvas.style.display='none';
 dom.dataset.ww='scene';dom.style.cssText='height:65vh;position:relative;overflow:hidden;background:#243849';
 document.body.prepend(dom);
}
if(mode==='hidden-scene')canvas.style.display='none';
let fixedTime=null,simulationTime=0;
const scene=new T.Scene();scene.background=new T.Color('#182333');
const camera=new T.PerspectiveCamera(60,innerWidth/(innerHeight*.65),.1,200);
const ground=new T.Mesh(new T.PlaneGeometry(90,90),new T.MeshStandardMaterial({color:0x456b62,roughness:1}));
ground.rotation.x=-Math.PI/2;scene.add(ground,new T.HemisphereLight(0xffffff,0x334455,3));
const geom=new T.ConeGeometry(1,1,8),material=new T.MeshStandardMaterial({color:0x759d89,roughness:.9});
const terrain=new T.InstancedMesh(geom,material,25);scene.add(terrain);
const marker=new T.Mesh(new T.SphereGeometry(1.2,16,12),new T.MeshStandardMaterial({color:0xff8844,emissive:0x442200}));
marker.position.set(2,2,0);marker.visible=false;scene.add(marker);
const actor=new T.Mesh(new T.ConeGeometry(.7,2,5),new T.MeshBasicMaterial({color:0xffcc55}));scene.add(actor);
let player={chunk:['0','0','0'],local:[1,2,1]},view={yaw:0,pitch:0},edits=[],revision=0;
let jobs=0,workers=1,paused=false,disposed=false,pending=Promise.resolve(),counter=0;
const key=()=>player.chunk.join(',');
const db=editing&&mode!=='lost-restart'?await openStore({name:'scope-fixture',world:'fixture',version:'1',fingerprint:'fixture-noise-1'}):null;
function render(){
 if(disposed||paused)return;
 simulationTime=fixedTime??performance.now()/1000;
 if(mode==='uncontrolled-clock')simulationTime=++counter;
 actor.position.set(Math.sin(simulationTime)*4,3,-2);
 const pose=mode==='motion-only-nav'?{local:[1,2,1]}:player;
 camera.position.set(pose.local[0]+8,pose.local[1]+7,pose.local[2]+12);
 const yaw=mode==='motion-only-look'?0:view.yaw;
 camera.lookAt(pose.local[0]+Math.sin(yaw)*12,0,pose.local[2]-Math.cos(yaw)*6);
 const row=edits.find(e=>e.id==='marker');marker.visible=!!row&&!row.value.removed;
 if(mode==='motion-only-edit')marker.visible=false;
 if(dom){
  dom.innerHTML=`<div style="position:absolute;left:${35+player.local[0]*2}%;top:30%;width:70px;height:100px;background:#d8b377;transform:rotate(${view.yaw*90}deg)"></div>`;
 }else renderer.render(scene,camera);
}
const sample=chunk=>Array.from({length:25},(_,i)=>perlin3('fixture','terrain',[BigInt(chunk[0])*16n+BigInt(i%5),BigInt(chunk[1])*16n,BigInt(chunk[2])*16n+BigInt(Math.floor(i/5))]));
const workerURL=URL.createObjectURL(new Blob([`import {perlin3} from '${location.origin}/noise.mjs';
onmessage=({data:c})=>{const a=new Float64Array(Array.from({length:25},(_,i)=>perlin3('fixture','terrain',[BigInt(c[0])*16n+BigInt(i%5),BigInt(c[1])*16n,BigInt(c[2])*16n+BigInt(Math.floor(i/5))])));postMessage(a.buffer,[a.buffer]);};`],{type:'text/javascript'}));
const worker=new Worker(workerURL,{type:'module'});
async function generate(){
 jobs=1;
 const values=await new Promise((res,rej)=>{worker.onmessage=e=>res(new Float64Array(e.data));worker.onerror=rej;worker.postMessage(player.chunk);});
 jobs=0;
 const matrix=new T.Matrix4();
 for(let i=0;i<25;i++){const h=2+values[i]*4;matrix.makeScale(1,h,1);matrix.setPosition((i%5-2)*5,h/2,(Math.floor(i/5)-2)*5);terrain.setMatrixAt(i,matrix);}
 terrain.instanceMatrix.needsUpdate=true;terrain.computeBoundingSphere();
}
async function load(){
 if(!editing)return;
 if(mode==='lost-restart'){
  edits=JSON.parse(sessionStorage.getItem(key())||'[]');revision=0;return;
 }
 const page=await db.page({chunk:key(),limit:32});
 edits=page.rows.filter(r=>!r.key[3].startsWith('@')).map(r=>({id:r.key[3],value:r.value}));
 revision=await db.get(key(),'@revision')||0;
}
async function travel(p){
 player=structuredClone(p);
 if(mode!=='flight'){player.chunk[1]='0';player.local[1]=mode==='bad-height'?10:2;}
 if(mode==='fence')player.chunk=player.chunk.map(s=>String(Math.max(-4,Math.min(4,Number(s)))));
 await generate();await load();render();
}
function enqueue(fn){pending=pending.then(fn);pending.catch(e=>{document.querySelector('#save').textContent='Save failed: '+e.message;});return pending;}
async function move(){
 for(const axis of [0,2]){const n=normalize(player.chunk[axis],player.local[axis]+1);player.chunk[axis]=String(n.chunk);player.local[axis]=n.local;}
 if(mode!=='motion-only-nav')await generate();await load();render();
}
function look(){view.yaw+=.15;render();}
document.querySelector('[data-ww="navigate-forward"]').onclick=()=>enqueue(move);
document.querySelector('[data-ww="camera-right"]').onclick=()=>enqueue(look);
const keyHandler=e=>{if(e.key==='w')enqueue(move);if(e.key==='ArrowRight')enqueue(look);};
addEventListener('keydown',keyHandler);
async function commit(rows){
 document.querySelector('#save').textContent='Saving';
 if(mode==='lost-restart')sessionStorage.setItem(key(),JSON.stringify(rows));
 else revision=await db.commit(key(),revision,rows);
 edits=structuredClone(rows);document.querySelector('#save').textContent='Saved';render();
}
if(editing){
 for(const id of ['place','remove']){
  const b=document.createElement('button');b.dataset.wwEdit=id;b.textContent=id+' light';
  b.onclick=()=>enqueue(async()=>{
   if(mode==='counter-only'){counter++;return;}
   await commit([{id:'marker',value:{position:[2,2,0],removed:id==='remove'}}]);
  });document.querySelector('#editing').append(b);
 }
}
const api={
 ready:travel(player),
 settle:async()=>{await api.ready;await pending;render();await new Promise(requestAnimationFrame);},
 travel:p=>enqueue(()=>travel(p)),
 baseHash:async c=>Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',new Float64Array(sample(c)).buffer))).map(x=>x.toString(16).padStart(2,'0')).join(''),
 snapshot:()=>structuredClone({player,vertical:mode==='flight'?{mode:'flight',velocityY:0}:{mode:'grounded',velocityY:0,supported:mode!=='unsupported',support:{chunk:[player.chunk[0],'0',player.chunk[2]],local:[player.local[0],0,player.local[2]]}},camera:view,simulationTime:mode==='uncontrolled-clock'?0:simulationTime,...(editing?{edits}:{}),...(mode==='counter-only'?{counter}:{})}),
 stats:()=>({queueJobs:mode==='queue'?100000:jobs,queueBytes:jobs*200,residentChunks:1,cpuBytes:1200,gpuBytes:4096,drawCalls:renderer.info.render.calls,workers,uploadsLastFrame:0,particles:0,dpr:1,backend:'webgl2-synthetic-fixture'}),
 setSimulationTime:t=>{fixedTime=t;render();},
 pause:p=>{paused=p;if(!p)render();},
 dispose:()=>{disposed=true;worker.terminate();workers=0;URL.revokeObjectURL(workerURL);db?.close();removeEventListener('keydown',keyHandler);for(const m of [ground,terrain,marker,actor]){m.geometry.dispose();m.material.dispose();}renderer.dispose();}
};
if(editing){
 api.backup=async()=>JSON.stringify({format:'scope-edit-v1',world:'fixture',version:'1',chunk:player.chunk,edits});
 api.restore=async text=>{
  const b=JSON.parse(text);
  if(b.format!=='scope-edit-v1'||b.world!=='fixture'||b.version!=='1'||!Array.isArray(b.chunk)||b.chunk.length!==3||!Array.isArray(b.edits)||b.edits.length>1||
    b.edits.some(e=>e.id!=='marker'||typeof e.value?.removed!=='boolean'||JSON.stringify(e.value.position)!=='[2,2,0]'))throw Error('bad backup');
  await travel({chunk:b.chunk,local:[1,2,1]});await commit(b.edits);
 };
}
window.worldweaver=api;

if(mode==='hung-ready')api.ready=new Promise(()=>{});
if(mode==='hung-js')api.setSimulationTime=()=>{while(true){}};
if(mode==='hung-travel')api.travel=()=>new Promise(()=>{});
