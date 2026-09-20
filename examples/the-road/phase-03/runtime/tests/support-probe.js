import {generate,support,offset,roadInfo} from '../src/generation.js';
let worst=0,where=null,count=0;
for(const c of [['0','0','0'],['-1','0','6'],['0','0','6']]){
 const g=generate({chunk:c,kind:'near'}).ground,p=g.p;
 for(let i=0;i<16*16*6;i+=3){
  const vs=[g.index[i],g.index[i+1],g.index[i+2]].map(v=>[p[v*3],p[v*3+1],p[v*3+2]]);
  const q=vs.reduce((a,v)=>a.map((n,j)=>n+v[j]/3),[0,0,0]);
  const pos=offset({chunk:c,local:[0,0,0]},q[0],q[2]),r=roadInfo(pos);
  if(r&&Math.abs(r.l)<3.1)continue;
  const delta=Math.abs(support(pos)-q[1]);if(delta>worst){worst=delta;where={chunk:c,local:q};}count++;
 }
}
console.log(JSON.stringify({kind:'analytic support versus actual terrain triangle centroids',maximumErrorMetres:worst,where,samples:count},null,2));
