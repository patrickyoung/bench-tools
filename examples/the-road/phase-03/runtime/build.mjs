import {build} from 'esbuild';
import {mkdir,copyFile,rm,cp} from 'node:fs/promises';
await rm('dist',{recursive:true,force:true});await mkdir('dist/licenses',{recursive:true});
await build({entryPoints:['src/main.js'],bundle:true,format:'esm',outfile:'dist/app.js',minify:true,legalComments:'eof',target:['es2022']});
await build({entryPoints:['src/chunk-worker.js'],bundle:true,format:'esm',outfile:'dist/chunk-worker.js',minify:true,legalComments:'eof',target:['es2022']});
await copyFile('index.html','dist/index.html');
await cp('assets','dist/assets',{recursive:true});
for(const [pkg,file] of [['three','LICENSE'],['esbuild','LICENSE.md'],['playwright','LICENSE']])await copyFile(`node_modules/${pkg}/${file}`,`dist/licenses/${pkg}.txt`);
