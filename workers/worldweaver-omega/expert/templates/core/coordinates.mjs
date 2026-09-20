// MIT. Canonical coordinates never pass through global Number or Float32.
export function integer(s) {
  if (typeof s === 'bigint') return s;
  if (typeof s !== 'string' || !/^(0|-?[1-9][0-9]*)$/.test(s)) throw Error('canonical integer string required');
  return BigInt(s);
}
export function floorDiv(a,b) {
  if (typeof a!=='bigint'||typeof b!=='bigint'||b<=0n) throw Error('positive bigint divisor required');
  const q=a/b, r=a%b;
  return r<0n?q-1n:q;
}
export function normalize(chunk, local, size=16) {
  if (!Number.isFinite(local)||!Number.isSafeInteger(size)||size<=0||Math.abs(local)>1e9) throw Error('local coordinate out of range');
  const shift=Math.floor(local/size);
  return {chunk:integer(chunk)+BigInt(shift), local:local-shift*size};
}
export function globalCell(chunk, cell, size=16) {
  if (!Number.isSafeInteger(cell)||!Number.isSafeInteger(size)||size<=0) throw Error('invalid cell');
  return integer(chunk)*BigInt(size)+BigInt(cell);
}
export function localRender(chunk,origin,cell,size=16,maxChunks=512) {
  const d=integer(chunk)-integer(origin);
  if (d>BigInt(maxChunks)||d< -BigInt(maxChunks)) throw Error('not resident: global position cannot become render float');
  return Number(d)*size+cell;
}
export function chunkKey(x,y,z) { return [x,y,z].map(v=>integer(v).toString()).join(','); }
export function identity(world,version,chunk,object) {
  return JSON.stringify([world,version,chunk,object]); // no delimiter collision
}
