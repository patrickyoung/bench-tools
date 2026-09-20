import {B,R,SIZE} from './config.js';
import {integer,normalize,localRender,globalCell,floorDiv,chunkKey} from './coordinates.js';
import {perlin3,hash64} from './noise.js';
import {treeShape} from './trees.js';
export const clamp=(x,a=0,b=1)=>Math.max(a,Math.min(b,x));
export const mix=(a,b,t)=>a+(b-a)*t;
export const smooth=(a,b,x)=>{const t=clamp((x-a)/(b-a));return t*t*(3-2*t);};
export function at(x,z){const X=normalize('0',x,SIZE),Z=normalize('0',z,SIZE);return {chunk:[X.chunk.toString(),'0',Z.chunk.toString()],local:[X.local,0,Z.local]};}
export function offset(p,dx,dz){const x=normalize(p.chunk[0],p.local[0]+dx,SIZE),z=normalize(p.chunk[2],p.local[2]+dz,SIZE);return {chunk:[x.chunk.toString(),'0',z.chunk.toString()],local:[x.local,0,z.local]};}
// This is a bounded delta to the authored route origin, never a lossy global coordinate.
export function authored(p){
 if(integer(p.chunk[0]) < -128n || integer(p.chunk[0])>128n || integer(p.chunk[2]) < -128n || integer(p.chunk[2])>160n)return null;
 return {x:localRender(p.chunk[0],'0',p.local[0],SIZE,160),s:localRender(p.chunk[2],'0',p.local[2],SIZE,160)};
}
function routeWeight(s){return smooth(-4096,-2200,s)*(1-smooth(4000,5120,s));}
export function center(s){return (6*Math.sin(s/195)+3*Math.sin(s/71))*routeWeight(s);}
export function profile(s){
 const h=authoredProfile(s),w=routeWeight(s);
 return w===1?h:mix(30+relief(at(0,s)),h,w);
}
function authoredProfile(s){
 const k=R.profile;
 if(s<=k[0][0])return k[0][1];if(s>=k.at(-1)[0])return k.at(-1)[1];
 let i=0;while(k[i+1][0]<s)i++;
 const [a,h]=k[i],[b,j]=k[i+1],t=(s-a)/(b-a);
 // Hermite interpolation preserves the long grade while rounding hollow and crest.
 const m0=i? (j-k[i-1][1])/(b-k[i-1][0]):(j-h)/(b-a);
 const m1=i+2<k.length?(k[i+2][1]-h)/(k[i+2][0]-a):(j-h)/(b-a);
 return (2*t*t*t-3*t*t+1)*h+(t*t*t-2*t*t+t)*(b-a)*m0+(-2*t*t*t+3*t*t)*j+(t*t*t-t*t)*(b-a)*m1;
}
export function n3(p,y=37,stream='land',seed=B.seed,den=256){
 const yi=Math.floor(y),fx=Math.floor(p.local[0]),fz=Math.floor(p.local[2]);
 return perlin3(seed,stream,[globalCell(p.chunk[0],fx,SIZE),BigInt(yi),globalCell(p.chunk[2],fz,SIZE)],[p.local[0]-fx,y-yi,p.local[2]-fz],1,den);
}
const noiseCache=new Map();
function lattice(x,z,seed){
 const key=`${seed}:${x}:${z}`;if(noiseCache.has(key))return noiseCache.get(key);
 const v=B.noise.octaves.reduce((a,o,i)=>a+o.amplitude*perlin3(seed,`land:${i}`,[x,37n,z],[0,0,0],o.numerator,o.denominator),0);
 if(noiseCache.size>=16384)noiseCache.delete(noiseCache.keys().next().value);
 noiseCache.set(key,v);return v;
}
export function relief(p,seed=B.seed){
 const X=globalCell(p.chunk[0],Math.floor(p.local[0]),SIZE),Z=globalCell(p.chunk[2],Math.floor(p.local[2]),SIZE);
 const ax=floorDiv(X,8n)*8n,az=floorDiv(Z,8n)*8n;
 const u=(Number(X-ax)+p.local[0]%1)/8,v=(Number(Z-az)+p.local[2]%1)/8;
 return mix(mix(lattice(ax,az,seed),lattice(ax+8n,az,seed),u),mix(lattice(ax,az+8n,seed),lattice(ax+8n,az+8n,seed),u),v);
}
export function gap(s){return R.gaps.some(g=>Math.abs(s-g)<2.2)||Math.abs(s-R.oakStation)<9||Math.abs(s-R.bridgeStation)<11;}
export function tunnel(s){return R.canopyTunnels.some(([a,b])=>s>=a&&s<=b);}
export function cross(l){return R.crown*Math.max(0,1-(l/(R.width/2))**2)-R.rutDepth*Math.exp(-1*((Math.abs(l)-R.rutOffset)/.17)**2);}
export function roadInfo(p){
 const a=authored(p);
 if(a)return {...a,l:a.x-center(a.s),h:profile(a.s)};
 // Beyond authored geography, an ordinary continuous lane follows the unbounded terrain.
 if(integer(p.chunk[0])>=-2n&&integer(p.chunk[0])<=2n){
 const x=localRender(p.chunk[0],'0',p.local[0],SIZE,2);
 const mid={chunk:['0','0',p.chunk[2]],local:[0,0,p.local[2]]};
 return {x,s:null,l:x,h:30+relief(mid)};
 }return null;
}
export function streamS(x){return R.bridgeStation+6*Math.sin((x-center(R.bridgeStation))/42);}
export const waterLevel=()=>profile(R.bridgeStation)-1.4;
export function terrain(p,seed=B.seed){
 const a=authored(p),r=relief(p,seed);
 if(!a)return 30+r;
 const l=a.x-center(a.s),abs=Math.abs(l);
 let h=profile(a.s)+r*smooth(8,95,abs)+2.5*Math.sin(a.x/70+a.s/230)*smooth(14,100,abs);
 // Enclosed left earth bank; open right ditch. Bank tapers at every actual opening.
 if(a.s<R.hedgeEnd && a.s>-1600&&!gap(a.s)&&l>0)h+=R.bankHeight*smooth(4.5,5.6,l)*(1-smooth(8,13,l));
 h-=.42*Math.exp(-1*((l+5.1)/.72)**2);
 const channel=Math.abs(a.s-streamS(a.x));
 const bed=waterLevel()-.45+.12*Math.sin(a.x*.7);
 h=mix(h,bed,1-smooth(R.bridgeSpan/2,R.bridgeSpan/2+5.2,channel));
 // Beaten diagonal ramp, open-field side, continuous to the stream's gravel bed.
 if(l< -3.5&&l> -16.5){
 const t=clamp((-l-4)/11),s=R.bridgeStation-12+10*t;
 const ramp=mix(profile(R.bridgeStation-12)-.02,waterLevel()-.18,t);
 h=mix(h,ramp,1-smooth(.7,1.7,Math.abs(a.s-s)));
 }
 return mix(30+r,h,(1-smooth(2800,4096,Math.abs(a.x)))*routeWeight(a.s));
}
// Terrain vertices and collision query use exactly the same 2m triangles.
export function terrainVertex(p){
 let y=terrain(p);const r=roadInfo(p);
 if(r&&Math.abs(r.l)<R.width/2&&(r.s===null||Math.abs(r.s-R.bridgeStation)>R.bridgeSpan/2))y=Math.min(y,r.h-.15);
 return y;
}
export function terrainSurface(p){
 const x=p.local[0]%2,z=p.local[2]%2,base=offset(p,-x,-z),u=x/2,v=z/2;
 const h00=terrainVertex(base),h10=terrainVertex(offset(base,2,0)),h01=terrainVertex(offset(base,0,2)),h11=terrainVertex(offset(base,2,2));
 return u+v<=1?h00+(h10-h00)*u+(h01-h00)*v:h11+(h01-h11)*(1-u)+(h10-h11)*(1-v);
}
export const lanes=()=>[-R.width/2,-2.65,-1.34,-1.06,-.8,0,.8,1.06,1.34,2.65,R.width/2];
export function roadSurface(p,r=roadInfo(p)){
 const f=p.local[2]%1,prev=offset(p,0,-f),next=offset(p,0,1-f),a=roadInfo(prev),b=roadInfo(next);
 let l=r.l;
 if(r.s!==null)l=r.x-mix(center(a.s),center(b.s),f);
 const ll=lanes();let i=0;while(i<ll.length-2&&ll[i+1]<l)i++;
 const u=clamp((l-ll[i])/(ll[i+1]-ll[i]));
 return mix(a.h,b.h,f)+mix(cross(ll[i]),cross(ll[i+1]),u)+.012;
}
export function support(p){
 const r=roadInfo(p);
 if(r&&Math.abs(r.l)<=R.width/2)return roadSurface(p,r);
 return terrainSurface(p);
}
export function roadPoint(s,l=0){return at(center(s)+l,s);}
export function seeded(p,label,index=0){
 return Number(hash64(B.seed,label,integer(p.chunk[0]),integer(p.chunk[2]),BigInt(index))&0xffffffffn)/4294967296;
}
const treeCache=new Map();
export function treesForChunk(chunk){
 const cacheKey=chunk.join(',');if(treeCache.has(cacheKey))return treeCache.get(cacheKey);
 const out=[],p={chunk,local:[0,0,0]},a=authored(p);if(!a)return out;
 const add=(x,s,h,r,id)=>{const t=at(x,s);if(t.chunk[0]===chunk[0]&&t.chunk[2]===chunk[2])out.push({x:t.local[0],z:t.local[2],s,ax:x,h,r,id,y:terrain(t)});};
 for(const [ta,tb] of R.canopyTunnels)for(let s=ta;s<=tb;s+=9){
  for(const side of [-1,1])add(center(s)+side*(5.7+Math.sin(s)*.3),s,8.4+.7*Math.sin(s*4),.25,`grove:${ta}:${s}:${side}`);
 }
 add(center(R.oakStation)+6.7,R.oakStation,R.oakHeight,.49,'great-oak');
 for(let i=0;i<8;i++){const x=center(R.bridgeStation)+(i-3.5)*10;if(Math.abs(x-center(R.bridgeStation))<9)continue;const s=streamS(x)+(i%2?5.8:-5.8);add(x,s,6.5+(i%3),.18,`alder:${i}`);}
 // Scattered field trees, stable cells rather than visit-order RNG.
 for(let i=0;i<2;i++){
 const x=a.x+seeded(p,'tree-x',i)*32,s=a.s+seeded(p,'tree-z',i)*32,l=x-center(s);
 if(Math.abs(l)>24&&Math.abs(s-R.bridgeStation)>12&&seeded(p,'tree-presence',i)>.74)
 add(x,s,7+seeded(p,'tree-h',i)*4,.24,`field-tree:${chunkKey(...chunk)}:${i}`);
 }
 if(treeCache.size>=256)treeCache.delete(treeCache.keys().next().value);treeCache.set(cacheKey,out);
 return out;
}
export function blocked(p){
 const a=authored(p);if(!a)return false;
 const l=a.x-center(a.s);
 if(a.s<R.hedgeEnd&&a.s>-1600&&!gap(a.s)&&l>4.95&&l<8)return true;
 if(Math.abs(a.s-R.bridgeStation)<R.bridgeSpan/2+2.5&&Math.abs(l)>R.width/2-.06&&Math.abs(l)<R.bridgeWidth/2+.3)return true;
 for(let dx=-1;dx<=1;dx++)for(let dz=-1;dz<=1;dz++){
  const c=[(integer(p.chunk[0])+BigInt(dx)).toString(),'0',(integer(p.chunk[2])+BigInt(dz)).toString()];
  for(const t of treesForChunk(c)){const x=localRender(c[0],p.chunk[0],t.x,SIZE,1),z=localRender(c[2],p.chunk[2],t.z,SIZE,1);if(Math.hypot(x-p.local[0],z-p.local[2])<t.r+.28)return true;}
 }
 return false;
}
const rgb=hex=>{const v=parseInt(hex.slice(1),16);return [v>>16,(v>>8)&255,v&255].map(v=>{v/=255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4;});};
const colors=Object.fromEntries(Object.entries(B.palette).map(([k,v])=>[k,rgb(v)]));
function tint(c,f){return c.map(v=>v*f);}
function groundColor(p){
 const a=authored(p);if(!a)return colors.hedge;
 const l=a.x-center(a.s),v=.96+.10*clamp(relief(p)/14,-1,1);
 let c=l< -7&&a.s<R.hedgeEnd?colors.field:colors.hedge;
 if(Math.abs(l)<R.width/2+.2)c=colors.earth;
 if(l>4.5&&l<5.8&&a.s<R.hedgeEnd&&!gap(a.s))c=colors.earth;
 if(Math.hypot(l-6.7,(a.s-R.oakStation)*.85)<7)c=colors.earth;
 if(Math.abs(a.s-streamS(a.x))<5)c=colors.earth;
 if(l< -4&&l> -15&&Math.abs(a.s-(R.bridgeStation-12+10*clamp((-l-4)/11)))<1.3)c=colors.dust;
 const w=(1-smooth(2800,4096,Math.abs(a.x)))*routeWeight(a.s);
 return tint(c,v).map((value,i)=>mix(colors.hedge[i],value,w));
}
function meshGrid(nx,nz,point){
 const p=[],c=[],uv=[],idx=[];
 for(let j=0;j<=nz;j++)for(let i=0;i<=nx;i++){const v=point(i,j);p.push(...v.p);c.push(...v.c);uv.push(v.p[0]/3,v.p[2]/3);}
 for(let j=0;j<nz;j++)for(let i=0;i<nx;i++){let a=j*(nx+1)+i,b=a+1,d=a+nx+1;idx.push(a,d,b,b,d,d+1);}
 return {p:new Float32Array(p),c:new Float32Array(c),uv:new Float32Array(uv),index:new Uint32Array(idx)};
}
function addSkirt(g,n,depth){
 const p=Array.from(g.p),c=Array.from(g.c),uv=Array.from(g.uv),idx=Array.from(g.index);
 const edges=[];
 for(let i=0;i<n;i++)edges.push([i,i+1],[(i+1)*(n+1),i*(n+1)],[n*(n+1)+i+1,n*(n+1)+i],[i*(n+1)+n,(i+1)*(n+1)+n]);
 for(const [a,b] of edges){
  const start=p.length/3;
  for(const [v,dy] of [[a,0],[b,0],[a,-depth],[b,-depth]]){p.push(g.p[v*3],g.p[v*3+1]+dy,g.p[v*3+2]);c.push(...g.c.slice(v*3,v*3+3));uv.push(...g.uv.slice(v*2,v*2+2));}
  idx.push(start,start+1,start+2,start+1,start+3,start+2);
 }
 g.p=new Float32Array(p);g.c=new Float32Array(c);g.uv=new Float32Array(uv);g.index=new Uint32Array(idx);
}
function emptyBatch(){return {data:[],ids:[],tapers:[]};}
function append(batch,id,p,scale,rotation=[0,0,0],col=[1,1,1],taper=.95){if(batch.ids.length>=900)return;batch.ids.push(id);batch.tapers.push(taper);batch.data.push(...p,...scale,...rotation,...col);}
function rng(seed){let a=seed>>>0;return()=>{a=(Math.imul(a,1664525)+1013904223)>>>0;return a/4294967296;};}
export function generate(job){
 const chunk=job.chunk,p0={chunk,local:[0,0,0]},coarse=job.kind==='coarse',extent=coarse?R.sideTerrain+96:32,step=coarse?8:2;
 const lo=coarse?-extent:0,n=Math.round((coarse?extent*2:32)/step),longExtent=coarse?1472:32,nz=coarse?longExtent*2/step:n;
 const ground=meshGrid(n,nz,(i,j)=>{
  const x=lo+i*step,z=(coarse?-longExtent:0)+j*step,p=offset(p0,x,z),r=roadInfo(p);
  let y=coarse?terrain(p):terrainVertex(p);
  if(coarse){
   const a=authored(p),near=1-smooth(320,440,Math.hypot(x,z));
   y-=near*(a ? .1+2.3*(1-smooth(10,22,Math.abs(a.x-center(a.s)))):.1);
   if(a&&near>0){
    const channel=Math.abs(a.s-streamS(a.x)),weight=near*(1-smooth(20,28,channel));
    if(weight>0){
     // Every adjacent 8m cell is bounded by its actual 2m surface vertices.
     // Lower only the backing near the channel/ramp; never change canonical support.
     let floor=y;
     for(let dz=-8;dz<=8;dz+=2)for(let dx=-8;dx<=8;dx+=2)
      floor=Math.min(floor,terrainVertex(offset(p,dx,dz))-.15);
     y=mix(y,floor,weight);
    }
   }
  } // Continuous buried backing: no holes, and no global sinking of distant fields/road.
  return {p:[x,y,z],c:groundColor(p)};
 });
 if(!coarse)addSkirt(ground,n,2.7);
 const roadSegments=[];
 // Each 32m route section belongs to the chunk containing its center; its complete width crosses chunk edges without truncation.
 const aa=authored(p0),z0=aa?.s;
 const centerX=aa?center(aa.s+16):0;
 const owner=aa?at(centerX,aa.s+16).chunk[0]:'0';
 if(chunk[0]===owner&&!coarse){
  const laneNodes=lanes();
  roadSegments.push(meshGrid(laneNodes.length-1,32,(i,j)=>{
   const p=offset(p0,0,j),a=authored(p),s=a?.s,h=a?profile(s):roadInfo({...p,chunk:['0','0',p.chunk[2]],local:[0,0,p.local[2]]}).h;
   const cx=a?center(s)-aa.x:0,l=laneNodes[i];
   const shade=.96+.03*clamp(relief(p)/14,-1,1);
   const rut=1-.16*Math.exp(-1*((Math.abs(l)-R.rutOffset)/.18)**2);
   return {p:[cx+l,h+cross(l)+.012,j],c:tint(colors.dust,shade*rut)};
  }));
 }
 const batches={foliage:emptyBatch(),wood:emptyBatch(),stone:emptyBatch(),grass:emptyBatch(),flowers:emptyBatch(),litter:emptyBatch(),paving0:emptyBatch(),paving1:emptyBatch(),paving2:emptyBatch()};
 if(!coarse){
 const a=aa,baseId=chunkKey(...chunk),rand=rng(Number(hash64(B.seed,'detail',integer(chunk[0]),integer(chunk[2]))&0xffffffffn));
 if(a){
 // Dense asymmetric hedge: dissimilar overlapping leaf volumes, not extruded blocks.
 for(let z=0;z<32;z+=1.65){
  const s=a.s+z;if(s>R.hedgeEnd||s< -1600||gap(s))continue;
  const x=center(s)+6.45-a.x;
  if(x<0||x>=32)continue;
  const base=terrain(at(x+a.x,s));
  for(let k=0;k<3;k++)append(batches.foliage,`${baseId}:hedge:${z}:${k}`,[x+(rand()-.5)*1.4,base+.45+k*.77,z+(rand()-.5)*.8],[1.0+rand()*.5,R.hedgeHeight/3*(.75+rand()*.25),1.1+rand()*.4],[rand()*.4,rand()*6.28,rand()*.2],tint(colors.hedge,.7+rand()*.6));
  if(z%3.3<.1)append(batches.wood,`${baseId}:root:${z}`,[x-1.1,base-.25,z],[.065,1.2,.065],[.1,.5,1.0],colors.earth);
 }
 for(const t of treesForChunk(chunk))for(const part of treeShape(t,(x,z)=>terrain(offset(p0,x,z)))){
  append(batches[part.kind],part.id,part.p,part.scale,part.rotation,part.kind==='wood'?colors.earth:tint(colors.hedge,part.shade),part.taper??1);
 }
 // Irregular feathering remains outside the exact clear road width; no support movement.
 const edgeRand=rng(Number(hash64(B.seed,'verge-detail',integer(chunk[0]),integer(chunk[2]))&0xffffffffn));
 for(let i=0;i<150;i++){
  const s=a.s+edgeRand()*32,side=i%2?1:-1,l=side*(R.width/2+.08+edgeRand()*R.verge);
  const x=center(s)+l-a.x,z=s-a.s;if(x<0||x>=32||Math.abs(s-R.bridgeStation)<5)continue;
  const y=support(at(x+a.x,s)),h=.11+edgeRand()*.26;
  append(batches.grass,`${baseId}:verge:${i}`,[x,y+.015,z],[.8+edgeRand(),h,.8+edgeRand()],[0,edgeRand()*6.28,0],tint(colors.hedge,.8+edgeRand()*.38));
  if(i%4===0)append(batches.litter,`${baseId}:litter:${i}`,[x,y+.009,z],[.07,.008,.12],[edgeRand()*.2,edgeRand()*6.28,edgeRand()*.2],tint(colors.earth,.85+edgeRand()*.25));
 }
 // Stream-bank reeds, fallen leaves and gravel visually break the water edge.
 for(let i=0;i<95;i++){
  const x=a.x+edgeRand()*32,s=streamS(x)+(edgeRand()-.5)*11,z=s-a.s,l=x-center(s);
  if(z<0||z>=32||Math.abs(l)<3.8)continue;
  const y=support(at(x,s)),distance=Math.abs(s-streamS(x));
  if(distance>R.bridgeSpan/2+.15){
   append(batches.grass,`${baseId}:bank-grass:${i}`,[x-a.x,y,z],[1,.27+edgeRand()*.38,1],[0,edgeRand()*6.28,0],tint(colors.hedge,.72+edgeRand()*.35));
   if(i%3===0)append(batches.litter,`${baseId}:bank-leaf:${i}`,[x-a.x,y+.01,z],[.10,.009,.16],[0,edgeRand()*6.28,0],colors.earth);
  }else{
   append(batches.stone,`${baseId}:bed-gravel:${i}`,[x-a.x,y+.03,z],[.07+edgeRand()*.12,.04+edgeRand()*.04,.10+edgeRand()*.17],[0,edgeRand()*6.28,0],tint(colors.stone,.65+edgeRand()*.25));
  }
 }
 // One broken patch of old pavement; stable authored row/cell IDs, independent seed stream.
 const pavementMid=(R.cobbles[0]+R.cobbles[1])/2,pavementLength=Math.min(10.6,R.cobbles[1]-R.cobbles[0]-1);
 if(pavementLength>0&&a.s+32>=pavementMid-pavementLength/2&&a.s<=pavementMid+pavementLength/2){
  const rows=Math.floor(pavementLength/.38);
  for(let row=0;row<rows;row++)for(let col=-5;col<=5;col++){
   const pr=rng(Number(hash64(B.seed,'exposed-pavement',BigInt(row),BigInt(col))&0xffffffffn));
   const t=(row+.5)/rows,s=pavementMid+(t-.5)*pavementLength+(pr()-.5)*.05;
   const l=col*.32+(row%2?.08:-.08)+(pr()-.5)*.035;
   const halfWidth=(1.35+pr()*.20)*Math.pow(Math.max(0,1-Math.pow(2*t-1,4)),.25);
   if(Math.abs(l)>halfWidth||pr()<.07)continue;
   const pos=roadPoint(s,l);
   if(pos.chunk[0]!==chunk[0]||pos.chunk[2]!==chunk[2])continue;
   const sx=.142+pr()*.019,sy=.04,sz=.181+pr()*.01;
   const rx=-Math.atan((support(offset(pos,0,.1))-support(offset(pos,0,-.1)))/.2);
   const rz=Math.atan((support(offset(pos,.1,0))-support(offset(pos,-.1,0)))/.2);
   const yaw=(pr()-.5)*.20;
   let height=Infinity;
   // Fit the tilted top face against the SAME crowned/rutted road, not a world-axis AABB.
   for(let u=-1;u<=1;u+=.25)for(let v=-1;v<=1;v+=.25){
    const xx=sx*u*Math.cos(rz)-sy*Math.sin(rz),yy=sx*u*Math.sin(rz)+sy*Math.cos(rz);
    const dy=yy*Math.cos(rx)-sz*v*Math.sin(rx),zz=yy*Math.sin(rx)+sz*v*Math.cos(rx);
    const dx=xx*Math.cos(yaw)+zz*Math.sin(yaw),dz=-xx*Math.sin(yaw)+zz*Math.cos(yaw);
    height=Math.min(height,support(offset(pos,dx,dz))+.023-dy);
   }
   append(batches['paving'+((row*7+col+50)%3)],`road-pavement:${row}:${col}`,[pos.local[0],height,pos.local[2]],[sx,sy,sz],[rx,yaw,rz],tint(colors.stone,.82+pr()*.18));
  }
 }
 // Meadow tufts, dry stubble, gravel and gorse kept small and stable.
 for(let i=0;i<135;i++){
  const x=rand()*32,z=rand()*32,p=offset(p0,x,z),a=authored(p),l=a.x-center(a.s),y=terrain(p);
  if(Math.abs(l)>3.6&&Math.abs(a.s-streamS(a.x))>3){
   const golden=l< -7&&a.s<R.hedgeEnd;
   append(batches.grass,`${baseId}:tuft:${i}`,[x,y,z],[.55+rand(),golden?.32:.16,.55+rand()],[0,rand()*6.28,0],tint(golden?colors.field:colors.hedge,.6+rand()*.65));
   if(a.s>R.hedgeEnd&&Math.abs(l)>7&&i%26===0){
    append(batches.foliage,`${baseId}:gorse:${i}`,[x,y+.6,z],[.9,.7,.8],[0,rand()*6,0],tint(colors.hedge,.6));
    append(batches.flowers,`${baseId}:gorse-flower:${i}`,[x,y+1.05,z],[.85,.22,.8],[0,rand()*6,0],[.5,.29,.018]);
   }
  }
  if(Math.abs(l)<3&&a.s>R.cobbles[0]&&a.s<R.cobbles[1]&&rand()>.15){
   append(batches.stone,`${baseId}:cobble:${i}`,[x,support(p)-.025,z],[.14+rand()*.1,.065,.18+rand()*.08],[0,rand()*1,0],colors.stone);
  }else if(Math.abs(l)<3.1&&i%4===0)append(batches.stone,`${baseId}:gravel:${i}`,[x,support(p),z],[.035,.025,.055],[0,rand()*6,0],tint(colors.stone,.8));
  if(Math.abs(a.s-streamS(a.x))<3&&Math.abs(l)>4&&i%2===0)append(batches.stone,`${baseId}:riverstone:${i}`,[x,y+.05,z],[.17,.09,.22],[0,rand()*6,0],tint(colors.stone,.65));
 }
 }
 }
 for(const b of Object.values(batches)){b.data=new Float32Array(b.data);b.tapers=new Float32Array(b.tapers);}
 return {ground,roads:roadSegments,batches,chunk,kind:job.kind};
}
export function canonicalSample(chunk,seed=B.seed){
 const p={chunk,local:[0,0,0]};
 return {chunk,seed,version:B.generationVersion,samples:[[0,0],[1,1],[16,16],[31,31],[32,32]].map(([x,z])=>{const q=offset(p,x,z);return [terrain(q,seed),support(q),n3(q,2.31,'detail',seed,64)];}),trees:treesForChunk(chunk).map(t=>({id:t.id,x:t.x,y:t.y,z:t.z,h:t.h}))};
}
export function buffersOf(value,out=[]){if(ArrayBuffer.isView(value))out.push(value.buffer);else if(value&&typeof value==='object')for(const v of Object.values(value))buffersOf(v,out);return out;}
