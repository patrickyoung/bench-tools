// Bounded reservation ledger, not a scheduler. Charge worst-case bytes BEFORE
// dispatch and keep tickets charged through completion/upload until disposal.
export class Budget {
  constructor({jobs,bytes}) {
    if (![jobs,bytes].every(v=>Number.isSafeInteger(v)&&v>0)) throw Error('finite positive caps required');
    this.maxJobs=jobs; this.maxBytes=bytes; this.used=0; this.serial=0; this.epoch=0; this.live=new Map();
  }
  reserve(key,bytes) {
    if (!Number.isSafeInteger(bytes)||bytes<=0||bytes>this.maxBytes) throw Error('invalid reservation');
    if (this.live.size>=this.maxJobs||this.used+bytes>this.maxBytes) return null;
    const ticket=Object.freeze({id:++this.serial,key,bytes,epoch:this.epoch});
    this.live.set(ticket.id,ticket); this.used+=bytes; return ticket;
  }
  current(ticket) {return this.live.get(ticket.id)===ticket&&ticket.epoch===this.epoch;}
  release(ticket) {
    if (this.live.get(ticket.id)!==ticket) return false;
    this.used-=ticket.bytes; this.live.delete(ticket.id); return true;
  }
  invalidate() { this.epoch++; } // running stale jobs stay charged until disposed
  stats() { return {jobs:this.live.size,bytes:this.used,epoch:this.epoch}; }
}
export function radialOffsets(radius,look=[0,0,1]) {
  if (!Number.isInteger(radius)||radius<0||radius>8) throw Error('bounded radius required');
  const result=[];
  for(let y=-radius;y<=radius;y++) for(let z=-radius;z<=radius;z++) for(let x=-radius;x<=radius;x++) {
    const d=Math.hypot(x,y,z); if(d>radius) continue;
    // Nearby first, horizon/view direction bonus. Tie-break independent of visits.
    const horizon=(x*look[0]+y*look[1]+z*look[2])/(d||1);
    result.push({x,y,z,priority:d-0.45*horizon});
  }
  return result.sort((a,b)=>a.priority-b.priority||a.y-b.y||a.z-b.z||a.x-b.x);
}
