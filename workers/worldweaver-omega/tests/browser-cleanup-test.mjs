// Injected process-table/signal tests, NOT actual browser evidence.
import {resolve} from 'node:path';
import assert from 'node:assert/strict';
const {cleanup}=await import('file://'+resolve(process.env.WW_TEST_HOME||'expert','lib/browser-cleanup.mjs'));
const row=(pid,ppid,pgid,state='S')=>({pid,ppid,pgid,state});
let n=0,signals=[];
let result=await cleanup({pid:10,groups:new Set([10,20]),table:()=>++n===1?[row(10,1,10),row(20,10,20),row(30,20,30)]:[row(20,1,20,'Z')],
 signal:pid=>{signals.push(pid);throw Object.assign(Error('transient race'),{code:'EPERM'});}});
assert.equal(result.killError,null);assert(result.confirmed);assert(result.attempts.length);assert(signals.includes(30));
result=await cleanup({pid:10,groups:new Set([10]),limitMs:220,table:()=>[row(10,1,10)],
 signal:()=>{throw Object.assign(Error('denied'),{code:'EPERM'});}});
assert(result.killError);assert(!result.confirmed);assert(result.attempts.every(a=>a.code==='EPERM'));
result=await cleanup({pid:10,groups:new Set([10]),limitMs:220,table:()=>{throw Error('inspection denied');},signal:()=>{}});
assert.match(result.killError,/inspection denied/);assert(!result.confirmed);
console.log('3/3 cleanup cases: transient EPERM/zombie, persistent live EPERM, unknown inspection');
