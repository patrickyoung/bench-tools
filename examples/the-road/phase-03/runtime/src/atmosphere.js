import {B,R,SIZE} from './config.js';
import {integer,normalize,localRender} from './coordinates.js';
import {hash64} from './noise.js';
import {at,authored,center,support,terrain} from './generation.js';
function random(seed){let a=seed;return()=>{a=(Math.imul(a,1664525)+1013904223)>>>0;return a/4294967296;};}
export function shiftWorld(p,delta){
 const axes=p.chunk.map((v,i)=>normalize(v,p.local[i]+delta[i],SIZE));
 return {chunk:axes.map(v=>v.chunk.toString()),local:axes.map(v=>v.local)};
}
function grounded(p,y){const h=normalize('0',y,SIZE);return {chunk:[p.chunk[0],h.chunk.toString(),p.chunk[2]],local:[p.local[0],h.local,p.local[2]]};}
export function atmosphereTile(cx,cz){
 const origin={chunk:[cx,'0',cz],local:[0,0,0]},a=authored(origin);if(!a)return [];
 const rand=random(Number(hash64(B.seed,'global-atmosphere-tile',integer(cx),integer(cz))&0xffffffffn)),out=[];
 for(let i=0;i<30;i++){
  const kind=i<8?'dust':'midge',s=a.s+rand()*32,x=center(s)+(kind==='dust'?(rand()-.5)*5.7:-5.1+(rand()-.5)*1.0),p=at(x,s);
  const seed=rand();
  if(p.chunk[0]!==cx||p.chunk[2]!==cz)continue;
  out.push({id:`${kind}:${cx}:${cz}:${i}`,kind,seed,base:grounded(p,support(p)+(kind==='dust'?.45:.75+rand()*.6))});
 }
 return out;
}
export function chimneyParticles(){
 const out=[];
 for(let i=0;i<80;i++){
  const j=[0,3,6,9,12][i%5],x=(j%5-2)*17+Math.sin(j*7)*7+2,s=R.villageStation+Math.floor(j/5)*22+Math.sin(j*3)*8,p=at(x,s);
  const seed=Number(hash64(B.seed,'chimney-smoke',BigInt(i))&0xffffffffn)/4294967296;
  out.push({id:`smoke:village:${j}:${Math.floor(i/5)}`,kind:'smoke',seed,base:grounded(p,terrain(p)+7+j%3)});
 }
 return out;
}
export function particlePose(p,time,reduced=false){
 const t=time*(reduced?.35:1),seed=p.seed;let delta,size,opacity;
 if(p.kind==='dust'){delta=[Math.sin(t*.13+seed*34)*1.8,Math.sin(t*.4+seed*6)*.12,Math.sin(t*.09+seed*17)*1.6];size=.12;opacity=.045;}
 else if(p.kind==='midge'){delta=[Math.sin(t*2.1+seed*40)*.19,Math.sin(t*2.7+seed*60)*.19,Math.cos(t*1.7+seed*31)*.19];size=.018;opacity=.48;}
 else{const age=(seed+t*.022)%1;delta=[age*age*7+Math.sin(seed*12+t*.17)*age,age*25,0];size=2+age*5;opacity=(1-age)*.20;}
 return {id:p.id,kind:p.kind,worldPosition:shiftWorld(p.base,delta),size,opacity};
}
export function localParticle(p,origin){
 return p.chunk.map((c,i)=>localRender(c,origin[i],p.local[i],SIZE,512));
}
export class Atmosphere{
 constructor(){this.cache=new Map();this.smoke=chimneyParticles();this.items=[];this.key='';}
 select(player){
  const key=player.chunk[0]+','+player.chunk[2];if(this.key===key)return this.items;
  this.key=key;const wanted=new Set(),items=[];
  for(let z=-1;z<=1;z++)for(let x=-1;x<=1;x++){
   const cx=(integer(player.chunk[0])+BigInt(x)).toString(),cz=(integer(player.chunk[2])+BigInt(z)).toString(),k=cx+','+cz;
   wanted.add(k);if(!this.cache.has(k))this.cache.set(k,atmosphereTile(cx,cz));items.push(...this.cache.get(k));
  }
  for(const k of this.cache.keys())if(!wanted.has(k))this.cache.delete(k);
  if(authored(player))items.push(...this.smoke);
  this.items=items.slice(0,Math.min(350,B.budgets.particles));return this.items;
 }
 sample(player,time,reduced=false){return this.select(player).map(p=>particlePose(p,time,reduced));}
 dispose(){this.cache.clear();this.smoke=[];this.items=[];this.key='';}
}
