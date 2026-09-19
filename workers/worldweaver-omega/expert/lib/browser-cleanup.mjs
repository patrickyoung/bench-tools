// Unix supervisor cleanup; no browser/runtime dependency. Zombies are not LIVE.
import {execFileSync} from 'node:child_process';
export function processTable(){
  return execFileSync('ps',['-axo','pid=,ppid=,pgid=,stat='],{encoding:'utf8',timeout:500,maxBuffer:4194304})
    .trim().split('\n').filter(Boolean).map(line=>{
      const [pid,ppid,pgid,state]=line.trim().split(/\s+/);
      if(![pid,ppid,pgid].every(x=>/^\d+$/.test(x))||!state)throw Error('invalid process table');
      return {pid:+pid,ppid:+ppid,pgid:+pgid,state};
    });
}
export async function cleanup({pid,groups,register=()=>{},table=processTable,signal=process.kill.bind(process),limitMs=2500}){
  const known=new Set([pid]),attempts=[],start=Date.now();
  let live=[],scanError=null,emptyScans=0;
  do {
    try {
      register();
      const rows=table();
      let changed;
      do {changed=false;for(const p of rows)if((known.has(p.ppid)||groups.has(p.pgid))&&!known.has(p.pid)){
        known.add(p.pid);changed=true;
      }}while(changed);
      live=rows.filter(p=>(known.has(p.pid)||groups.has(p.pgid))&&!p.state.startsWith('Z'));
      scanError=null;
      if(!live.length){if(++emptyScans>=2)return {killError:null,attempts,live:[],confirmed:true};}
      else emptyScans=0;
      // Kill registered groups AND observed descendants (including escaped groups).
      for(const target of new Set([...groups].map(g=>-g).concat(live.map(p=>p.pid)))){
        try{signal(target,'SIGKILL');}catch(e){
          if(e.code!=='ESRCH')attempts.push({target,code:e.code||'UNKNOWN',error:String(e)});
        }
      }
    }catch(e){
      scanError=String(e);emptyScans=0;
      for(const target of new Set([...groups].map(g=>-g).concat([...known]))){
        try{signal(target,'SIGKILL');}catch(err){if(err.code!=='ESRCH')attempts.push({target,code:err.code||'UNKNOWN',error:String(err)});}
      }
    }
    await new Promise(r=>setTimeout(r,100));
  }while(Date.now()-start<limitMs);
  return {killError:scanError||'unresolved LIVE processes after cleanup: '+live.map(p=>p.pid).join(','),
    attempts,live,confirmed:false};
}
