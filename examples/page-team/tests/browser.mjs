// Offline check of the browser checker; needs the expert's pinned Playwright.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import assert from 'node:assert/strict';
const here=path.dirname(fileURLToPath(import.meta.url));
const expert=path.resolve(process.argv[2]||path.join(here,'../expert'));
const root=fs.mkdtempSync(path.join(os.tmpdir(),'page-team-browser-'));
const fixture=fs.readFileSync(path.join(here,'browser-fixture.html'),'utf8');
const cases=[['valid',fixture,0],
 ['large',fixture.replace('</body>','<!--'+ 'bounded fixture padding '.repeat(150000)+'--></body>'),0],
 ['interaction',fixture.replace('panel.hidden=false','panel.hidden=true'),1],
 ['motion',fixture.replace("if(!matchMedia('(prefers-reduced-motion:reduce)').matches)",'if(true)'),1],
 ['network',fixture.replace('</main>','<img src="https://example.invalid/forbidden.png" alt="Forbidden fixture"></main>'),1]];
try{
 for(const [name,html,expected] of cases){
  const file=path.join(root,name+'.html'), out=path.join(root,name);
  fs.writeFileSync(file,html);
  const result=spawnSync(process.execPath,[path.join(expert,'bin/browser-check.mjs'),file,out],{encoding:'utf8',timeout:90000});
  assert.equal(result.status,expected,result.stderr||result.error?.message);
  const report=JSON.parse(fs.readFileSync(path.join(out,'browser.json'),'utf8'));
  assert.equal(report.passed,expected===0);
  if(expected)assert.ok(report.failures.length>0);else assert.equal(report.scenarios.length,6);
  console.log('ok browser checker:',name);
 }
}finally{fs.rmSync(root,{recursive:true,force:true});}
