#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import assert from 'node:assert/strict';
import { chromium } from 'playwright';
const [file,outArg,...flags]=process.argv.slice(2);
if(!file||!outArg) throw Error('usage: browser-check.mjs FILE EVIDENCE_DIR [--epl]');
const out=path.resolve(outArg); fs.mkdirSync(out,{recursive:true});
const html=fs.readFileSync(file,'utf8'), failures=[], scenarios=[];
const report={html_sha256:crypto.createHash('sha256').update(html).digest('hex'),passed:false,visible_asset_sha256:[],failures,scenarios};
let browser;
const selector=(key,value)=>`[data-${key}=${JSON.stringify(value)}]`;
async function epl(page){
  assert.match(await page.locator('body').innerText(),/illustrative/i,'sample-data disclosure');
  const cards=page.locator('[data-player-card]');
  const players=await cards.evaluateAll(xs=>xs.map(x=>({id:x.dataset.playerId,name:x.dataset.playerName,position:x.dataset.position,age:Number(x.dataset.age)})));
  assert.ok(players.length>=4,'at least four profiles');
  assert.ok(players.every(p=>p.id&&p.name&&p.position&&Number.isFinite(p.age)),'player metadata');
  const visible=()=>cards.evaluateAll(xs=>xs.filter(x=>x.getClientRects().length).map(x=>x.dataset.playerId));
  const search=page.locator('[data-search]');
  await search.fill(players[0].name); await page.waitForTimeout(150);
  assert.deepEqual(await visible(),[players[0].id],'search filters actual profiles');
  await search.fill('NO_MATCH_7F2D'); await page.waitForTimeout(100);
  assert.equal((await visible()).length,0,'empty search state');
  await search.fill('');
  const position=players[0].position;
  await page.locator('[data-position-filter]').selectOption(position);
  assert.deepEqual(new Set(await visible()),new Set(players.filter(p=>p.position===position).map(p=>p.id)),'position filter');
  await page.locator('[data-position-filter]').selectOption('');
  const ages=players.map(p=>p.age), cap=Math.floor((Math.min(...ages)+Math.max(...ages))/2);
  await page.locator('[data-age-filter]').fill(String(cap));
  await page.locator('[data-age-filter]').dispatchEvent('change');
  assert.deepEqual(new Set(await visible()),new Set(players.filter(p=>p.age<=cap).map(p=>p.id)),'age filter');
  await page.locator('[data-age-filter]').fill(String(Math.max(...ages)));
  await page.locator('[data-age-filter]').dispatchEvent('change');
  const opener=page.locator(selector('open-profile',players[0].id));
  await opener.focus(); await page.keyboard.press('Enter');
  const dialog=page.locator('[data-profile-dialog]'); await dialog.waitFor({state:'visible'});
  assert.ok((await dialog.innerText()).toLocaleLowerCase().includes(players[0].name.toLocaleLowerCase()),'profile identity');
  for(const key of ['strengths','uncertainties','next-action']) assert.ok((await dialog.locator(`[data-${key}]`).innerText()).trim().length>25,`meaningful ${key}`);
  await dialog.locator('[data-close-profile]').click();
  await dialog.waitFor({state:'hidden'});
  assert.ok(await opener.evaluate(x=>x===document.activeElement),'profile close restores focus');
  for(const p of players.slice(0,2)){
    const b=page.locator(selector('compare',p.id)); await b.click();
    assert.equal(await b.getAttribute('aria-pressed'),'true','comparison selection');
  }
  await page.locator('[data-open-comparison]').click();
  const comparison=page.locator('[data-comparison]'); await comparison.waitFor({state:'visible'});
  for(const p of players.slice(0,2)) assert.ok(await comparison.locator(selector('compared-player',p.id)).isVisible(),'selected comparison profile');
  assert.match(await comparison.innerText(),/(%|\/90|per 90|years|cm|kg|score|rating)/i,'metric units');
  await comparison.locator('[data-close-comparison]').click();
  const save=page.locator(selector('shortlist',players[0].id));
  const count=page.locator('[data-shortlist-count]');
  const before=Number((await count.innerText()).trim());
  assert.equal(await save.getAttribute('aria-pressed'),'false','fresh shortlist state');
  await save.click(); assert.equal(await save.getAttribute('aria-pressed'),'true');
  assert.equal(Number((await count.innerText()).trim()),before+1,'shortlist add count');
  await save.click(); assert.equal(await save.getAttribute('aria-pressed'),'false');
  assert.equal(Number((await count.innerText()).trim()),before,'shortlist remove count');
  const unnamed=await page.locator('button,input,select').evaluateAll(xs=>xs.filter(x=>x.getClientRects().length&&!x.textContent.trim()&&!x.getAttribute('aria-label')&&!x.getAttribute('aria-labelledby')&&!(x.labels&&x.labels.length)).map(x=>x.outerHTML.slice(0,100)));
  assert.deepEqual(unnamed,[],'accessible control labels');
}
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
      if(flags.includes('--epl')) await epl(page); else await journeys(page);
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
