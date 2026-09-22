import {floorDiv,integer} from './coordinates.mjs';
// 3D gradient Perlin: arbitrary signed integer lattice, no permutation-table tiling.
// FNV-1a + avalanche is deterministic, NOT a cryptographic identity function.
const MASK=(1n<<64n)-1n, enc=new TextEncoder();
export function hash64(seed,stream,...coords) {
  let h=14695981039346656037n;
  for (const byte of enc.encode(JSON.stringify([seed,stream,...coords.map(x=>integer(x).toString())]))) {
    h=((h^BigInt(byte))*1099511628211n)&MASK;
  }
  h=((h^(h>>30n))*0xbf58476d1ce4e5b9n)&MASK;
  h=((h^(h>>27n))*0x94d049bb133111ebn)&MASK;
  return h^(h>>31n);
}
const G=[[1,1,0],[-1,1,0],[1,-1,0],[-1,-1,0],[1,0,1],[-1,0,1],[1,0,-1],[-1,0,-1],[0,1,1],[0,-1,1],[0,1,-1],[0,-1,-1]];
const fade=t=>t*t*t*(t*(t*6-15)+10), mix=(a,b,t)=>a+(b-a)*t;
function split(cell,frac,n,d) {
  if (!Number.isFinite(frac)||frac<0||frac>=1) throw Error('fraction must be [0,1)');
  const product=integer(cell)*n, q=floorDiv(product,d), rem=product-q*d;
  const f=(Number(rem)+frac*Number(n))/Number(d), shift=Math.floor(f);
  return [q+BigInt(shift),f-shift];
}
export function perlin3(seed,stream,cells,fractions=[0,0,0],numerator=1,denominator=64) {
  if (![numerator,denominator].every(v=>Number.isSafeInteger(v)&&v>0&&v<=1e9)||cells.length!==3||fractions.length!==3) throw Error('invalid noise coordinates/frequency');
  const p=cells.map((v,i)=>split(v,fractions[i],BigInt(numerator),BigInt(denominator)));
  const f=p.map(x=>x[1]), u=f.map(fade), values=[];
  for(let z=0;z<2;z++) for(let y=0;y<2;y++) for(let x=0;x<2;x++) {
    const g=G[Number(hash64(seed,stream,p[0][0]+BigInt(x),p[1][0]+BigInt(y),p[2][0]+BigInt(z))%12n)];
    values.push(g[0]*(f[0]-x)+g[1]*(f[1]-y)+g[2]*(f[2]-z));
  }
  return mix(mix(mix(values[0],values[1],u[0]),mix(values[2],values[3],u[0]),u[1]),
             mix(mix(values[4],values[5],u[0]),mix(values[6],values[7],u[0]),u[1]),u[2]);
}
export function octaves(seed,stream,cells,fractions,layers) {
  return layers.reduce((v,o,i)=>v+o.amplitude*perlin3(seed,`${stream}:${i}`,cells,fractions,o.numerator,o.denominator),0);
}
