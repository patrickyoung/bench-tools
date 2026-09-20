import {B} from './config.js';
import {hash64} from './noise.js';
function random(seed){let a=seed;return()=>{a=(Math.imul(a,1664525)+1013904223)>>>0;return a/4294967296;};}
const lerp=(a,b,t)=>a.map((v,i)=>v+(b[i]-v)*t);
function bezier(a,b,c,d,t){const u=1-t;return a.map((v,i)=>u*u*u*v+3*u*u*t*b[i]+3*u*t*t*c[i]+t*t*t*d[i]);}
export function treeShape(tree,ground){
 const rand=random(Number(hash64(B.seed,tree.id,17n)&0xffffffffn)),out=[];
 const oak=tree.id==='great-oak',alder=tree.id.startsWith('alder:'),base=[tree.x,tree.y,tree.z],h=tree.h;
 function limb(id,a,b,c,d,r0,r1,segments=4){
  for(let j=0;j<segments;j++){
   const u=j/segments,v=(j+1)/segments,p=bezier(a,b,c,d,u),q=bezier(a,b,c,d,v);
   const radius=t=>r1+(r0-r1)*Math.pow(1-t,1.3),ra=radius(u),rb=radius(v),dx=q[0]-p[0],dy=q[1]-p[1],dz=q[2]-p[2],length=Math.hypot(dx,dy,dz);
   out.push({kind:'wood',id:id+':'+j,p:lerp(p,q,.5),scale:[ra,length*1.025,ra],rotation:[Math.acos(dy/length),Math.atan2(dx,dz),0],taper:rb/ra});
  }
 }
 function leaves(id,p,sx,sy,sz){
  out.push({kind:'foliage',id,p,scale:[sx,sy,sz],rotation:[(rand()-.5)*.5,rand()*6.28,(rand()-.5)*.5],shade:.83+rand()*.36});
 }
 const leanX=oak?-1.1:(rand()-.5)*1.8,leanZ=alder?((Number(tree.id.split(':')[1])%2)?-1.7:1.7):(rand()-.5)*1.7;
 const top=[tree.x+leanX,tree.y+h*(alder?.79:.70),tree.z+leanZ];
 const control1=[tree.x+leanX*.13,tree.y+h*.22,tree.z+leanZ*.10],control2=[tree.x+leanX*1.2,tree.y+h*.49,tree.z+leanZ*.8];
 limb(tree.id+':trunk',base,control1,control2,top,tree.r,tree.r*(oak?.22:.14),oak?7:6);
 const count=oak?5:3+Math.floor(rand()*3);
 for(let b=0;b<count;b++){
  const level=.35+(b+.5)/count*.52+(rand()-.5)*.11,attach=bezier(base,control1,control2,top,level);
  const angle=rand()*Math.PI*2,length=h*(oak?.39+rand()*.16:alder?.17+rand()*.12:.24+rand()*.17);
  const end=[attach[0]+Math.cos(angle)*length,tree.y+h*(.61+rand()*.19),attach[2]+Math.sin(angle)*length];
  const c1=[attach[0]+Math.cos(angle+.7)*length*.28,attach[1]+(oak?-.25:.65),attach[2]+Math.sin(angle+.7)*length*.28];
  const c2=[end[0]-Math.cos(angle-.5)*length*.25,end[1]-.8-rand()*.6,end[2]-Math.sin(angle-.5)*length*.25];
  limb(tree.id+`:limb:${b}`,attach,c1,c2,end,tree.r*(oak?.46+rand()*.21:.25+rand()*.17),.018,5);
  for(let fork=0;fork<2;fork++){
   const start=bezier(attach,c1,c2,end,.58+fork*.23),direction=angle+(fork?1:-1)*(.45+rand()*.75);
   const tip=[end[0]+Math.cos(direction)*(1.0+rand()*1.0),Math.min(tree.y+h-1,end[1]+.5+rand()*1.2),end[2]+Math.sin(direction)*(1.0+rand())];
   const mid=lerp(start,tip,.45);mid[1]-=.15;
   limb(tree.id+`:twig:${b}:${fork}`,start,lerp(start,mid,.5),mid,tip,.045,.0045,3);
   const scale=oak?1.55:alder?1.00:1.3;
   leaves(tree.id+`:foliage:${b}:${fork}:0`,tip,scale*(.9+rand()*.25),.95+rand()*.27,scale);
   leaves(tree.id+`:foliage:${b}:${fork}:1`,[tip[0]+(rand()-.5)*1.5,tip[1]-.25,tip[2]+(rand()-.5)*1.5],scale,.8+rand()*.35,scale);
  }
 }
 leaves(tree.id+':top-leaves',top,alder?1.1:1.45,1.3,alder?1.1:1.5);
 leaves(tree.id+':top-offset',[top[0]+.6,Math.min(tree.y+h-1,top[1]+1.0),top[2]-.5],1.3,1.1,1.3);
 for(let i=0;i<5;i++){
  const a=rand()*6.28,length=tree.r*(2.6+rand()*1.5),end=[tree.x+Math.cos(a)*length,0,tree.z+Math.sin(a)*length];end[1]=ground(end[0],end[2])+.01;
  const start=[tree.x,tree.y+.35,tree.z],mid=lerp(start,end,.55);mid[1]=Math.max(end[1],tree.y)+.05;
  limb(tree.id+':root:'+i,start,lerp(start,mid,.5),mid,end,tree.r*.27,.008,3);
 }
 return out;
}
