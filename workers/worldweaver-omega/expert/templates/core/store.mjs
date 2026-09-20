// Sparse IndexedDB helper; no getAll over history. Transactions acknowledge only
// oncomplete. Chunk CAS and optional camera/location commit share one transaction.
const req=r=>new Promise((resolve,reject)=>{r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error);});
const done=t=>new Promise((resolve,reject)=>{t.oncomplete=()=>resolve();t.onabort=()=>reject(t.error||Error('transaction aborted'));t.onerror=()=>{};});
function str(x){if(typeof x!=='string'||!x||x.length>4096)throw Error('invalid key');return x;}
export async function openStore({name='worldweaver-v1',world,version,fingerprint}) {
  [world,version,fingerprint].forEach(str);
  const opening=indexedDB.open(name,1);
  opening.onupgradeneeded=()=>opening.result.createObjectStore('records');
  const db=await req(opening);
  db.onversionchange=()=>db.close();
  const key=(chunk,id)=>[world,version,str(chunk),str(id)];
  {
    const t=db.transaction('records','readwrite'), end=done(t), s=t.objectStore('records');
    const k=key('@metadata','fingerprint'), old=await req(s.get(k));
    if(old!==undefined&&old!==fingerprint) {t.abort(); await end.catch(()=>{});db.close();throw Error('generator fingerprint conflict: select new version');}
    s.put(fingerprint,k); await end;
  }
  return {
    close(){db.close();},
    async get(chunk,id) {const t=db.transaction('records'),end=done(t);const v=await req(t.objectStore('records').get(key(chunk,id)));await end;return v;},
    async commit(chunk,expectedRevision,edits,player=undefined,{abort=false}={}) {
      str(chunk);
      if(!Number.isSafeInteger(expectedRevision)||expectedRevision<0||expectedRevision>=Number.MAX_SAFE_INTEGER||!Array.isArray(edits)||edits.length>512)throw Error('invalid bounded commit');
      if(chunk.startsWith('@'))throw Error('reserved chunk');
      for(const e of edits) {
        str(e.id);if(e.id.startsWith('@')||e.value===undefined||JSON.stringify(e.value).length>65536)throw Error('invalid edit');
      }
      if(player!==undefined&&JSON.stringify(player).length>65536)throw Error('player state exceeds bounded record');
      const t=db.transaction('records','readwrite'), end=done(t), s=t.objectStore('records');
      const revkey=key(chunk,'@revision'), rev=(await req(s.get(revkey)))||0;
      if(rev!==expectedRevision) {t.abort();await end.catch(()=>{});throw Error('revision conflict; reload before retry');}
      for(const e of edits)s.put(e.value,key(chunk,e.id));
      s.put(rev+1,revkey);
      if(player!==undefined)s.put(player,key('@player','state'));
      if(abort)t.abort();
      await end;return rev+1;
    },
    async page({chunk=null,after=null,limit=128}={}) {
      if(!Number.isInteger(limit)||limit<1||limit>512)throw Error('page limit');
      const prefix=chunk===null?[world,version]:[world,version,str(chunk)];
      if(after!==null&&(!Array.isArray(after)||prefix.some((v,i)=>after[i]!==v)))throw Error('foreign cursor');
      const range=IDBKeyRange.bound(after||prefix,[...prefix,[]],after!==null,true);
      const t=db.transaction('records'),end=done(t),s=t.objectStore('records'),rows=[];
      await new Promise((resolve,reject)=>{
        const r=s.openCursor(range);r.onerror=()=>reject(r.error);
        r.onsuccess=()=>{const c=r.result;if(!c){resolve();return;}
          rows.push({key:c.key,value:c.value});if(rows.length>=limit){resolve();return;}c.continue();};
      });
      await end;return {rows,next:rows.length===limit?rows.at(-1).key:null};
    }
  };
}
