import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import {Workbook,SpreadsheetFile} from '@oai/artifact-tool';
import {refreshPivots,addChart} from './features.mjs';

// Deliberately declarative: no eval, subprocesses, arbitrary scripts or fetch.
const root=path.resolve(process.argv[2]);
const specPath=path.join(root,'output/spec.json');
const spec=JSON.parse(await fs.readFile(specPath,'utf8'));
const wb=Workbook.create();
const digest=async p=>crypto.createHash('sha256').update(await fs.readFile(p)).digest('hex');
const colors={ink:'#17324D',muted:'#526477',line:'#DCE3EA',input:'#FFF1CC',blue:'#244C78'};
const styles={
 title:{font:{name:'Arial',size:16,bold:true,color:colors.ink},rowHeight:30},
 header:{fill:colors.ink,font:{name:'Arial',size:11,bold:true,color:'#FFFFFF'},rowHeight:30,wrapText:true,horizontalAlignment:'center'},
 section:{fill:'#E8EEF5',font:{name:'Arial',size:11,bold:true,color:colors.ink},rowHeight:26},
 input:{fill:colors.input,font:{name:'Arial',size:11,color:'#153C66'}},
 metric:{font:{name:'Arial',size:16,bold:true,color:colors.blue},rowHeight:32},
 note:{font:{name:'Arial',size:11,color:colors.muted}},
 warning:{fill:'#FDE9E7',font:{name:'Arial',size:11,color:'#9C231B'}},
 total:{font:{name:'Arial',size:11,bold:true,color:colors.ink},borders:{top:{style:'thin',color:colors.line}}},
 body:{}
};
const value=v=>v && typeof v==='object' && 'date' in v?new Date(v.date+'T00:00:00Z'):typeof v==='string'&&v.startsWith('=')?"'"+v:v;
for(const s of spec.sheets)wb.worksheets.add(s.name);
for(const s of spec.sheets){
 const sh=wb.worksheets.getItem(s.name);
 sh.showGridLines=false;sh.tabColor=s.role==='output'?colors.ink:s.role==='work'?colors.blue:'#B9C8D7';
 const area=sh.getRange(s.display_range);
 area.format.font={name:'Arial',size:11,color:colors.ink};area.format.rowHeight=24;
 area.format.columnWidth=16;area.format.verticalAlignment='center';
 for(const [c,w] of Object.entries(s.widths||{}))sh.getRange(`${c}1:${c}1000`).format.columnWidth=w;
 for(const b of s.blocks){
  const r=sh.getRange(b.range);
  r.format.font={name:'Arial',size:11,color:colors.ink};r.format.rowHeight=24;r.format.verticalAlignment='center';
  if(b.values){
   // Prevent spreadsheet inference from collapsing text identifiers such as 001.
   for(let i=0;i<b.values.length;i++)for(let j=0;j<b.values[i].length;j++)
    if(typeof b.values[i][j]==='string')r.getCell(i,j).setNumberFormat('@');
   r.values=b.values.map(row=>row.map(value));
  }else r.formulas=b.formulas;
  if(b.style)r.format=styles[b.style]||{};
  if(b.format)r.setNumberFormat(b.format);
  if(b.height)r.format.rowHeight=b.height;
  if(b.wrap!==undefined)r.format.wrapText=b.wrap;
 }
 for(const t of s.tables||[]){const table=sh.tables.add(t.range,true,t.name);table.showFilterButton=true;}
 for(const d of s.validations||[])sh.getRange(d.range).dataValidation={rule:{type:'list',values:d.values}};
 for(const c of s.conditional_formats||[])sh.getRange(c.range).conditionalFormats.addCustom(c.formula,{fill:c.fill||'#FDE9E7',font:{color:c.color||'#9C231B'}});
 if(s.freeze_rows)sh.freezePanes.freezeRows(s.freeze_rows);
 if(s.freeze_columns)sh.freezePanes.freezeColumns(s.freeze_columns);
 for(const c of s.charts||[]){
  addChart(sh,c);
 }
}
for(const c of spec.controls){
 const r=wb.worksheets.getItem(c.sheet).getRange(c.cell);r.format=styles.input;
 r.dataValidation={rule:c.values?{type:'list',values:c.values}:{type:'decimal',operator:'between',formula1:c.min,formula2:c.max}};
}
wb.recalculate();
refreshPivots(wb,spec.pivots||[]);
const metrics=()=>Object.fromEntries(spec.metrics.map(m=>[m.id,wb.worksheets.getItem(m.sheet).getRange(m.cell).values[0][0]]));
const baseline=metrics(),tests=[];
for(const t of spec.tests){
 const saved=[];
 try{
  for(const e of t.edits){const r=wb.worksheets.getItem(e.sheet).getRange(e.cell);saved.push([r,r.values]);r.values=[[value(e.value)]];}
  wb.recalculate();refreshPivots(wb,spec.pivots||[]);const observed=metrics();
  const checks=t.expect.map(e=>({metric:e.metric,expected:e.value,actual:observed[e.metric],passed:typeof e.value==='number'?typeof observed[e.metric]==='number'&&Math.abs(e.value-observed[e.metric])<=(e.tolerance??1e-6):e.value===observed[e.metric]}));
  tests.push({name:t.name,passed:checks.every(c=>c.passed),checks});
 }finally{for(const [r,vs] of saved)r.values=vs;wb.recalculate();refreshPivots(wb,spec.pivots||[]);}
}
await fs.mkdir(path.join(root,'previews'),{recursive:true});
const previews=[];
for(let i=0;i<spec.sheets.length;i++){
 const s=spec.sheets[i],p=`previews/${String(i+1).padStart(2,'0')}.png`;
 const blob=await wb.render({sheetName:s.name,range:s.display_range,scale:1.5,format:'png'});
 await fs.writeFile(path.join(root,p),new Uint8Array(await blob.arrayBuffer()));
 previews.push({sheet:s.name,path:p,sha256:await digest(path.join(root,p))});
}
await (await SpreadsheetFile.exportXlsx(wb)).save(path.join(root,'output/workbook.xlsx'));
const qa={schema:'bench.workbook-qa/v1',engine:'Artifact Tool',spec_sha256:await digest(specPath),workbook_sha256:await digest(path.join(root,'output/workbook.xlsx')),baseline,tests,previews,limitations:['Native Microsoft Excel interaction has not been tested.','Declared mutation expectations require independent review.']};
await fs.writeFile(path.join(root,'output/qa.json'),JSON.stringify(qa,null,2)+'\n');
console.log(JSON.stringify({baseline,tests:tests.map(t=>({name:t.name,passed:t.passed,...(!t.passed?{checks:t.checks}:{})})),previews:previews.length}));
if(tests.some(t=>!t.passed))process.exitCode=1;
