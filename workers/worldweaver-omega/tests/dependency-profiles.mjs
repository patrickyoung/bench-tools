// Offline profile installation/bundling test, not a world or performance benchmark.
import {resolve,join} from 'node:path';
import {mkdir,copyFile,writeFile,readFile} from 'node:fs/promises';
import {spawnSync} from 'node:child_process';
import assert from 'node:assert/strict';
const home=resolve(process.env.WW_TEST_HOME||'expert');
const out=resolve(process.argv[2]||'tests/breadth-results/profiles-'+Date.now());
await mkdir(out,{recursive:false});
const results=[];
for(const profile of ['base','effects','spatial','effects-spatial']){
 const dir=join(out,profile);await mkdir(dir);
 const template=join(home,profile==='base'?'templates/runtime':'templates/profiles/'+profile);
 for(const f of ['package.json','package-lock.json'])await copyFile(join(template,f),join(dir,f));
 const before=await readFile(join(dir,'package-lock.json'),'utf8');
 const run=(exe,args)=>{
  const r=spawnSync(exe,args,{cwd:dir,encoding:'utf8',timeout:120000});
  return {argv:[exe,...args],exit:r.status,stdout:r.stdout,stderr:r.stderr,error:r.error?.message};
 };
 const install=run('npm',['ci','--offline','--ignore-scripts','--no-audit','--no-fund','--logs-dir',join(dir,'npm-logs')]);
 const record={profile,install};results.push(record);
 await writeFile(join(out,'results.json'),JSON.stringify(results,null,2));
 assert.equal(install.exit,0,profile+': '+install.stderr);
 assert.equal(await readFile(join(dir,'package-lock.json'),'utf8'),before,'npm changed admitted lock');
 let source="import * as T from 'three';\nconsole.log(T.REVISION);\n";
 if(profile.includes('effects'))source+="import {EffectComposer,OutlineEffect} from 'postprocessing'; console.log(EffectComposer,OutlineEffect);\n";
 if(profile.includes('spatial'))source+="import {MeshBVH} from 'three-mesh-bvh'; console.log(new MeshBVH(new T.BoxGeometry()).raycast(new T.Ray(new T.Vector3(0,0,2),new T.Vector3(0,0,-1))).length);\n";
 await writeFile(join(dir,'probe.js'),source);
 record.build=run(join(dir,'node_modules/.bin/esbuild'),['probe.js','--bundle','--format=esm','--outfile=bundle.mjs']);
 await writeFile(join(out,'results.json'),JSON.stringify(results,null,2));
 assert.equal(record.build.exit,0,record.build.stderr);
 record.execute=run(process.execPath,['bundle.mjs']);
 await writeFile(join(out,'results.json'),JSON.stringify(results,null,2));
 assert.equal(record.execute.exit,0,record.execute.stderr);
 console.log(profile+': offline clean install, immutable lock, local bundle/import passed');
}
