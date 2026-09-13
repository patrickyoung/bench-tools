#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import crypto from 'node:crypto';
import assert from 'node:assert/strict';
import { chromium } from 'playwright';
const [file,outArg,...flags]=process.argv.slice(2);
if(!file||!outArg||!(flags.length===0||(flags.length===2&&flags[0]==='--checks'))) throw Error('usage: browser-check.mjs FILE EVIDENCE_DIR [--checks TRUSTED_MODULE]');
const extraChecks=flags.length?(await import(pathToFileURL(path.resolve(flags[1])).href)).default:null;
if(extraChecks!==null&&typeof extraChecks!=='function') throw Error('checks module must export a function');
const out=path.resolve(outArg); fs.mkdirSync(out,{recursive:true});
const html=fs.readFileSync(file,'utf8'), failures=[], scenarios=[];
const report={html_sha256:crypto.createHash('sha256').update(html).digest('hex'),passed:false,visible_asset_sha256:[],failures,scenarios};
let browser;
async function journeys(page){
  const data=JSON.parse(await page.locator('#page-tests').textContent());
  assert.ok(Array.isArray(data)&&data.length>0&&data.length<=12,'representative interaction journeys');
  for(const journey of data){
    assert.ok(typeof journey.name==='string'&&journey.steps.length>=2&&journey.steps.length<=20);
    assert.ok(journey.steps.some(s=>s.action.startsWith('expect')),'journey needs an assertion');
    for(const s of journey.steps){
      const loc=page.locator(s.selector);
      switch(s.action){
        case 'click': await loc.click(); break;
        case 'fill': await loc.fill(s.value); break;
        case 'select': await loc.selectOption(s.value); break;
        case 'expectVisible': assert.ok(await loc.isVisible(),journey.name); break;
        case 'expectHidden': assert.ok(!await loc.isVisible(),journey.name); break;
        case 'expectText': assert.ok((await loc.innerText()).includes(s.value),journey.name); break;
        case 'expectCount': assert.equal(await loc.count(),s.value,journey.name); break;
        default: throw Error('unknown journey action '+s.action);
      }
    }
  }
}
try{
  browser=await chromium.launch({headless:true,executablePath:process.env.PAGE_TEAM_BROWSER||undefined});
  report.browser=browser.version();
  for(const [name,width,reduced,fallback] of [['desktop',1440,false,false],['mobile',390,false,false],['narrow',320,true,false],['tablet',768,false,false],['reduced',1440,true,false],['fallback',390,true,true]]){
    const page=await browser.newPage({viewport:{width,height:960},reducedMotion:reduced?'reduce':'no-preference',serviceWorkers:'block'});
    page.setDefaultTimeout(4000);
    const errors=[];
    page.on('pageerror',e=>errors.push(e.message.slice(0,600)));
    page.on('console',m=>{if(m.type()==='error')errors.push(m.text().slice(0,600))});
    page.on('download',d=>d.cancel());
    const entry='https://page-team.invalid/entry'; let served=false;
    await page.route('**/*',r=>{
      if(!served&&r.request().isNavigationRequest()&&r.request().url()===entry){
        served=true;return r.fulfill({status:200,contentType:'text/html',body:html});
      }
      if(errors.length<20)errors.push('runtime request: '+r.request().url().slice(0,400));
      return r.abort();
    });
    await page.addInitScript(({fallback})=>{
      window.__observedFrames=0; const frame=window.requestAnimationFrame.bind(window);
      window.requestAnimationFrame=cb=>frame(t=>{window.__observedFrames++;cb(t)});
      if(fallback) HTMLCanvasElement.prototype.getContext=function(){return null};
    },{fallback});
    try{
      // Fulfill one navigation in memory: init scripts run before the app,
      // large embedded pages have no URL limit, and no listener/network is used.
      await page.goto(entry,{waitUntil:'load',timeout:15000});
      await page.waitForTimeout(500);
      assert.ok(!await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),'horizontal overflow');
      assert.equal(await page.locator('h1').count(),1,'one page heading');
      assert.ok(await page.locator('h1').isVisible(),'visible heading');
      assert.equal(await page.locator('img').evaluateAll(xs=>xs.filter(x=>x.getClientRects().length&&(!x.complete||x.naturalWidth===0)).length),0,'loaded embedded images');
      const sources=await page.locator('img').evaluateAll(xs=>xs.filter(x=>x.getClientRects().length).map(x=>x.currentSrc));
      for(const source of sources){const m=source.match(/^data:image\/[^;,]+;base64,(.*)$/s);if(m){const h=crypto.createHash('sha256').update(Buffer.from(m[1],'base64')).digest('hex');if(!report.visible_asset_sha256.includes(h))report.visible_asset_sha256.push(h);}}
      const frames=await page.evaluate(()=>window.__observedFrames);
      await page.waitForTimeout(300);
      const delta=await page.evaluate(()=>window.__observedFrames)-frames;
      const animated=await page.evaluate(()=>document.getAnimations().filter(a=>a.playState==='running').length);
      if(reduced){assert.ok(delta<=2,'reduced motion keeps scheduling animation');assert.equal(animated,0,'reduced motion leaves CSS animation');}
      else assert.ok(delta>2||animated>0,'purposeful animation is running');
      if(fallback) assert.ok(await page.locator('[data-visual-fallback]').isVisible(),'visible rendering fallback');
      await page.screenshot({path:path.join(out,name+'.png'),fullPage:true,animations:'disabled'});
      if(extraChecks) await extraChecks(page); else await journeys(page);
      assert.ok(!await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),'interaction caused overflow');
      assert.deepEqual(errors,[],'browser diagnostics');
      scenarios.push({name,passed:true});
    }catch(e){failures.push(name+': '+e.message.slice(0,1500));scenarios.push({name,passed:false});await page.screenshot({path:path.join(out,name+'-failure.png'),fullPage:true}).catch(()=>{});}
    finally{await page.close();}
  }
}catch(e){failures.push(e.message)}
finally{if(browser)await browser.close();report.passed=failures.length===0;fs.writeFileSync(path.join(out,'browser.json'),JSON.stringify(report,null,2)+'\n');}
console.log(JSON.stringify(report));
process.exitCode=report.passed?0:1;
