#!/usr/bin/env node
// Trusted host tool. Runtime and executable choices are operator authority.
// Uses Mermaid CLI's public renderMermaid API with its bundled ELK registration.
import fs from 'node:fs';
import path from 'node:path';
import {createRequire} from 'node:module';
import {pathToFileURL} from 'node:url';
import {createHash} from 'node:crypto';

const hash = b => createHash('sha256').update(b).digest('hex');
const job = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
if (Number(process.versions.node.split('.')[0]) < 22) throw Error('Node >=22 required');
if (job.outerCage !== true) throw Error('Explicit outer Cage invocation required');
const req = createRequire(path.join(job.runtime, 'package.json'));
const cliPath = req.resolve('@mermaid-js/mermaid-cli');
const cliReq = createRequire(cliPath);
function metadata(entry) {
  let dir = path.dirname(entry);
  for (let n = 0; n < 12; n++, dir = path.dirname(dir)) {
    const p = path.join(dir, 'package.json');
    if (fs.existsSync(p)) return JSON.parse(fs.readFileSync(p, 'utf8'));
  }
  throw Error('Package metadata not found');
}
const cliMeta = metadata(cliPath);
const mermaidMeta = metadata(cliReq.resolve('mermaid'));
const puppetPath = cliReq.resolve('puppeteer');
const puppetMeta = metadata(puppetPath);
if (cliMeta.version !== '11.17.0' || mermaidMeta.version !== '11.17.2' ||
    puppetMeta.version !== '25.11.0') throw Error('Unsupported runtime versions');
const {renderMermaid} = await import(pathToFileURL(cliPath).href);
const {default: puppeteer} = await import(pathToFileURL(puppetPath).href);
const browser = await puppeteer.launch({
  executablePath: job.browser, headless: "shell", pipe: true, timeout: 30000,
  args: ['--no-sandbox', '--disable-gpu', '--disable-background-networking', '--disable-component-update',
         '--disable-sync', '--no-first-run', '--no-default-browser-check',
         '--host-resolver-rules=MAP * ~NOTFOUND']
});
const timer = setTimeout(() => { browser.close().finally(() => process.exit(1)); }, 180000);
try {
  const safeBrowser = {
    async newPage() {
      const page = await browser.newPage();
      page.setDefaultTimeout(20000);
      await page.setRequestInterception(true);
      const localHTML = pathToFileURL(path.resolve(path.dirname(cliPath), '../dist/index.html')).href;
      page.on('request', request => {
        const u = request.url();
        if (u.startsWith('https://mermaid-cli-intercept.invalid/')) {
          // CLI's cooperative handler serves these from its admitted runtime;
          // DNS is disabled, so a missed interception cannot reach a network.
          return;
        }
        if (u === localHTML) request.continue({}, 100);
        else request.abort('blockedbyclient', 100);
      });
      return page;
    }
  };
  const framePage = await browser.newPage();
  framePage.setDefaultTimeout(20000);
  await framePage.setRequestInterception(true);
  framePage.on('request', r => r.abort('blockedbyclient'));
  for (const j of job.views) {
    const {data} = await renderMermaid(safeBrowser, j.source, 'svg', {
      viewport: {width: 1600, height: 1000, deviceScaleFactor: 1},
      backgroundColor: 'white', mermaidConfig: j.config, svgId: 'diagram-' + j.base
    });
    let raw = Buffer.from(data).toString('utf8');
    // Remove only the two pinned, unused Mermaid animation definitions.
    // Active animation and every other at-rule remain forbidden.
    raw = raw.replace(/<style\b[^>]*>[\s\S]*?<\/style>/g, css =>
      css.replace(/@keyframes edge-animation-frame\{from\{stroke-dashoffset:0;\}\}/g, '')
         .replace(/@keyframes dash\{to\{stroke-dashoffset:0;\}\}/g, ''));
    // No raw input markup is used. Reject unexpected runtime-generated active content.
    if (/<(?:script|foreignObject|image|iframe)\b|<!DOCTYPE|<!ENTITY|\bon\w+=|\bhref=/i.test(raw))
      throw Error('Unexpected active/resource SVG');
    await framePage.setContent('<!doctype html><html><body style="margin:0;background:white"></body></html>');
    const {svg, width, height} = await framePage.evaluate(({raw, j}) => {
      const ns = 'http://www.w3.org/2000/svg';
      const doc = new DOMParser().parseFromString(raw, 'image/svg+xml');
      if (doc.querySelector('parsererror')) throw Error('Malformed SVG');
      const graph = doc.documentElement;
      for (const style of graph.querySelectorAll('style')) {
        const css = style.textContent;
        if (/\\|@|(?:https?|file|data|javascript):|\/\/|image-set|expression|behavior|-moz-binding/i.test(css))
          throw Error('Unsupported active/external CSS');
        for (const match of css.matchAll(/url\(([^)]*)\)/gi))
          if (!/^['"]?#[A-Za-z0-9_-]+['"]?$/.test(match[1].trim()))
            throw Error('External CSS URL');
      }
      // Accessible metadata is set as literal DOM text, never Mermaid entities.
      for (const el of graph.querySelectorAll('title, desc')) el.remove();
      graph.removeAttribute('aria-labelledby');
      graph.removeAttribute('aria-describedby');
      graph.removeAttribute('role');
      const vb = graph.getAttribute('viewBox').split(/[ ,]+/).map(Number);
      if (vb.length !== 4 || vb.some(x => !Number.isFinite(x)) ||
          vb[2] <= 0 || vb[3] <= 0 || vb[2] > 2400 || vb[3] > 5000)
        throw Error('Graph too large or invalid; decompose rather than shrink');
      const width = Math.max(1200, Math.min(1800, Math.ceil(vb[2] + 80)));
      const scale = Math.min(1, (width - 80) / vb[2]);
      if (scale < 0.72) throw Error('Graph text would shrink too far; split view');
      const root = document.createElementNS(ns, 'svg');
      root.setAttribute('xmlns', ns);
      root.setAttribute('role', 'img');
      root.setAttribute('aria-labelledby', 'figure-title figure-desc');
      function element(name, attrs, content) {
        const el = document.createElementNS(ns, name);
        for (const [k,v] of Object.entries(attrs)) el.setAttribute(k, String(v));
        if (content !== undefined) el.textContent = content;
        root.append(el);
        return el;
      }
      element('title', {id:'figure-title'}, j.view.title);
      element('desc', {id:'figure-desc'}, j.view.alternative + ' ' + j.view.caveats.join(' '));
      const backdrop = element('rect', {x:0,y:0,width, height:7000,fill:'#ffffff'});
      element('rect', {x:40,y:32,width:48,height:5,fill:'#0f766e'});
      let y = 70;
      // Wrap using real browser text measurements, not assumed character width.
      function paragraph(str, size, color, weight='400') {
        const words = str.split(/\s+/);
        const attrs = {x:40,y,'font-family':'Arial, sans-serif','font-size':size,
                       fill:color,'font-weight':weight};
        const probe = element('text', attrs, '');
        let line = '';
        const lines = [];
        for (const word of words) {
          const next = line ? line + ' ' + word : word;
          probe.textContent = next;
          if (probe.getComputedTextLength() > width - 80 && line) {
            lines.push(line);
            line = word;
          } else line = next;
        }
        lines.push(line);
        probe.remove();
        for (const content of lines) {
          element('text', {...attrs, y}, content);
          y += size * 1.4;
        }
        y += size * 0.1;
      }
      document.body.append(root); // Font measurement requires attachment.
      paragraph(j.view.title, 28, '#172b4d', '700');
      paragraph(j.view.audience + ' | ' + j.view.state + ' | ' + j.view.abstraction +
                ' | ' + j.status + ' - not approved', 16, '#475569');
      paragraph('Scope: ' + j.scope, 16, '#475569');
      y += 20;
      graph.removeAttribute('style');
      graph.setAttribute('x', String((width - vb[2]*scale)/2));
      graph.setAttribute('y', String(y));
      graph.setAttribute('width', String(vb[2]*scale));
      graph.setAttribute('height', String(vb[3]*scale));
      const inserted = document.importNode(graph, true);
      root.append(inserted);
      if ([...inserted.querySelectorAll('*')].some(el => getComputedStyle(el).animationName !== 'none'))
        throw Error('Active animation unsupported');
      const normalize = s => s.replace(/\s+/g, '');
      const texts = [...inserted.querySelectorAll('text')].map(el => el.textContent);
      const graphBox = inserted.getBoundingClientRect();
      for (const el of inserted.querySelectorAll('text')) {
        const b = el.getBoundingClientRect();
        if (b.left < graphBox.left - 1 || b.top < graphBox.top - 1 ||
            b.right > graphBox.right + 1 || b.bottom > graphBox.bottom + 1)
          throw Error('Text clipped by graph viewport');
      }
      if (j.view.type === 'sequence') {
        const boxes = [...inserted.querySelectorAll('rect.actor')].map(el => el.getBoundingClientRect());
        const headers = [...inserted.querySelectorAll('text.actor')];
        if (!boxes.length || !headers.length) throw Error('Missing sequence participant boxes/text');
        for (const el of headers) {
          const b = el.getBoundingClientRect();
          if (!boxes.some(box => b.left >= box.left - 1 && b.right <= box.right + 1 &&
                                b.top >= box.top - 1 && b.bottom <= box.bottom + 1))
            throw Error('Sequence participant text outside actor box');
        }
      }
      const combined = normalize(texts.join(' '));
      if (/&#\d+;|&#x[0-9a-f]+;|#\d+;/i.test(texts.join(' ')))
        throw Error('Numeric escape artifact in visible DOM');
      for (const expected of j.labels)
        if (!combined.includes(normalize(expected)))
          throw Error('Rendered label differs from model: ' + expected);
      y += Math.ceil(vb[3]*scale) + 35;
      element('line', {x1:40,y1:y-16,x2:width-40,y2:y-16,stroke:'#cbd5e1'});
      paragraph(j.view.caption, 17, '#172b4d');
      for (const c of j.view.caveats) paragraph('Caveat: ' + c, 16, '#92400e');
      for (const note of j.notes) paragraph(note, 16, '#475569');
      const frameText = normalize([...root.children].filter(el => el.localName === 'text')
        .map(el => el.textContent).join(' '));
      for (const note of j.notes)
        if (!frameText.includes(normalize(note))) throw Error('Missing literal frame note: ' + note);
      paragraph('Key: ' + j.key, 15, '#475569');
      paragraph(j.view.key, 15, '#475569');
      const height = Math.ceil(y + 25);
      if (height > 7000 || width * height > 24000000) throw Error('Framed image too large');
      backdrop.setAttribute('height', String(height));
      root.setAttribute('width', String(width));
      root.setAttribute('height', String(height));
      root.setAttribute('viewBox', `0 0 ${width} ${height}`);
      for (const el of root.querySelectorAll('text')) {
        if (/&#\d+;|&#x[0-9a-f]+;|#\d+;/i.test(el.textContent))
          throw Error('Numeric escape artifact in frame');
        const b = el.getBoundingClientRect();
        if (b.left < -1 || b.top < -1 || b.right > width + 1 || b.bottom > height + 1)
          throw Error('Text outside frame; decompose or shorten faithfully');
      }
      return {svg: new XMLSerializer().serializeToString(root), width, height};
    }, {raw, j});
    await framePage.setViewport({width, height, deviceScaleFactor:1});
    // Reuse the exact framed DOM serialized as SVG, not a second Mermaid layout.
    fs.writeFileSync(path.join(job.output, j.base + '.svg'), svg);
    await framePage.screenshot({path:path.join(job.output, j.base + '.png'), type:'png',
                               clip:{x:0,y:0,width,height}, captureBeyondViewport:true});
  }
  const runtime = {
    path: job.runtime, cli: cliMeta.version, mermaid: mermaidMeta.version,
    puppeteer: puppetMeta.version, node: process.version, browser: await browser.version(),
    cli_sha256: hash(fs.readFileSync(cliPath)),
    lock_sha256: hash(fs.readFileSync(path.join(job.runtime, 'package-lock.json'))),
    browser_sha256: hash(fs.readFileSync(job.browser))
  };
  fs.writeFileSync(path.join(job.output, 'runtime.json'), JSON.stringify(runtime, null, 2));
} finally {
  clearTimeout(timer);
  await browser.close();
}
