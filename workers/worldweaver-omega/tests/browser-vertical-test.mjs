// Execute the actual pure position/destination assertions without pretending to render.
import {readFile,writeFile,mkdir} from 'node:fs/promises';
import {resolve,join} from 'node:path';
import assert from 'node:assert/strict';
const home=resolve(process.env.WW_TEST_HOME||'expert');
const source=await readFile(join(home,'tools/browser-audit.mjs'),'utf8');
const code=source.slice(source.indexOf('  function position('),source.indexOf('  const travel='));
assert(code.includes('function destination'));
const check=vertical=>new Function('assert','blueprint','vertical',code+';return destination;')(assert,{coordinates:{chunkSize:16}},vertical);
const target={chunk:['9007199254740993','8','-9007199254740993'],local:[1,2,1]};
const player={chunk:[target.chunk[0],'0',target.chunk[2]],local:[1,2,1]};
const state={player,vertical:{mode:'grounded',velocityY:0,supported:true,support:{chunk:player.chunk,local:[1,0,1]}}};
const grounded=check({mode:'grounded',clearance:[1.9,2.1]});
grounded(state,target); // Gravity correction from arbitrary requested chunk Y=8.
let count=1;
for(const change of [
 s=>s.player.chunk[0]='9007199254740992',s=>s.player.chunk[2]='-9007199254740992',
 s=>s.player.local[0]=2,s=>s.player.local[1]=10,s=>s.player.local[1]=NaN,
 s=>s.player.chunk[1]='00',s=>s.vertical.supported=false,s=>s.vertical.velocityY=1,
 s=>s.vertical.support.local[2]=2,s=>s.vertical.mode='flight']){
 const s=structuredClone(state);change(s);assert.throws(()=>grounded(s,target));count++;
}
const flight=check({mode:'flight'});
flight({player:target,vertical:{mode:'flight',velocityY:0}},target);count++;
assert.throws(()=>flight({...state,vertical:{mode:'flight',velocityY:0}},target));count++;
const out=resolve(process.argv[2]);await mkdir(out,{recursive:false});
await writeFile(join(out,'results.json'),JSON.stringify({scope:'Pure destination assertions, not production physics or actual browser',expectedOutcomes:count},null,2));
console.log(count+'/13 exact X/Z, supported-Y, selected-flight outcomes');
