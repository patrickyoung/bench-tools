import {build} from 'esbuild';
import {mkdir} from 'node:fs/promises';
import {spawnSync} from 'node:child_process';
await mkdir('../../scratch/tests',{recursive:true});
for(const name of ['regressions','worker-node','release-repairs','stream-pavement-probe']){
 await build({entryPoints:[`tests/${name}.js`],bundle:true,platform:'node',format:'esm',outfile:`../../scratch/tests/${name}.mjs`});
}
const reports=[];
for(const name of ['regressions','release-repairs','stream-pavement-probe']){
 const result=spawnSync(process.execPath,[`../../scratch/tests/${name}.mjs`,...(name==='stream-pavement-probe'?['--require-corrected']:[])],{encoding:'utf8'});
 if(result.status!==0){process.stderr.write(result.stderr);process.stdout.write(result.stdout);process.exit(result.status??1);}
 reports.push({suite:name,...JSON.parse(result.stdout)});
}
console.log(JSON.stringify({boundary:'Node mathematical/production-module checks; no native browser/audio/GLSL execution',results:reports.flatMap(r=>r.results||[{name:r.suite,...r}])},null,2));
