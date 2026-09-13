// Trusted, example-specific checks; never part of the worker export.
import assert from 'node:assert/strict';
const selector=(key,value)=>`[data-${key}=${JSON.stringify(value)}]`;
export default async function check(page){
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
