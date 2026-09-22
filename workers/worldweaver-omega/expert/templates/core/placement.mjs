// Deterministic local AABB repair. Caller supplies actual support sampler.
// Finite candidate search, never silently drops an unresolved structure.
export function overlaps(a,b,clearance=0) {
  return [0,1,2].every(i=>Math.abs(a.center[i]-b.center[i])<(a.size[i]+b.size[i])/2+clearance);
}
export function repairPlacement(original,occupied,heightAt,{step=1,rings=8,clearance=0}={}) {
  if (!(step>0)||!Number.isInteger(rings)||rings<0||rings>64||clearance<0) throw Error('invalid repair bounds');
  if (!original.center?.every(Number.isFinite)||!original.size?.every(v=>Number.isFinite(v)&&v>0)) throw Error('invalid AABB');
  for(let r=0;r<=rings;r++) for(let dz=-r;dz<=r;dz++) for(let dx=-r;dx<=r;dx++) {
    if(Math.max(Math.abs(dx),Math.abs(dz))!==r) continue;
    const c={...original,center:[original.center[0]+dx*step,original.center[1],original.center[2]+dz*step]};
    const h=heightAt(c.center[0],c.center[2],c.size);
    if (!Number.isFinite(h)) continue;
    c.center[1]=h+c.size[1]/2;
    if(occupied.some(v=>overlaps(c,v,clearance))) continue;
    return {status:'repaired',original,placement:c,reason:'support and clearance',offset:[dx*step,dz*step]};
  }
  return {status:'unresolved',original,reason:'no supported non-overlapping placement in bounded search'};
}
