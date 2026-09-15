import fs from 'node:fs/promises';
import path from 'node:path';
import crypto from 'node:crypto';
import {FileBlob,SpreadsheetFile} from '@oai/artifact-tool';
import {refreshPivots,addChart} from './features.mjs';
const root=path.resolve(process.argv[2]), inspectOnly=process.argv[3]==='inspect';
const req=JSON.parse(await fs.readFile(path.join(root,'request.json'),'utf8'));
const wb=await SpreadsheetFile.importXlsx(await FileBlob.load(path.join(root,'.runtime/import.xlsx')));
const digest=async p=>crypto.createHash('sha256').update(await fs.readFile(p)).digest('hex');
const typed=v=>v&&typeof v==='object'&&v.date?new Date(v.date+'T00:00:00Z'):typeof v==='string'&&v.startsWith('=')?"'"+v:v;
async function render(previews,folder){
 await fs.mkdir(path.join(root,folder),{recursive:true});const output=[];
 for(let i=0;i<previews.length;i++){
  const p=previews[i],file=`${folder}/${String(i+1).padStart(2,'0')}.png`;
  const blob=await wb.render({sheetName:p.sheet,range:p.range,scale:1.3,format:'png'});
  await fs.writeFile(path.join(root,file),new Uint8Array(await blob.arrayBuffer()));
  output.push({...p,path:file,sha256:await digest(path.join(root,file))});
 }return output;
}
if(inspectOnly){
 const inventory=JSON.parse(await fs.readFile(path.join(root,'inspection/workbook.json'),'utf8'));
 inventory.previews=await render(inventory.sheets.map(s=>({sheet:s.name,range:s.preview_range})),'inspection/previews');
 await fs.writeFile(path.join(root,'inspection/workbook.json'),JSON.stringify(inventory,null,2));
 console.log('Read inspection/workbook.json and inspection/previews/*.png');
}else{
 const specPath=path.join(root,'output/spec.json'),spec=JSON.parse(await fs.readFile(specPath,'utf8'));
 for(const op of spec.operations){
  if(op.op==='add_sheet'){wb.worksheets.add(op.sheet);continue;}
  const sh=wb.worksheets.getItem(op.sheet),r=op.range?sh.getRange(op.range):null;
  switch(op.op){
   case 'values':
    for(let i=0;i<op.values.length;i++)for(let j=0;j<op.values[i].length;j++)if(typeof op.values[i][j]==='string'&&/^[+-]?\d/.test(op.values[i][j]))r.getCell(i,j).setNumberFormat('@');
    r.values=op.values.map(row=>row.map(typed));break;
   case 'formulas':r.formulas=op.formulas;break;
   case 'copy':r.copyFrom(sh.getRange(op.source_range),'all');break;
   case 'format':r.format=op.format;break;
   case 'table':{
    const old=sh.tables.items.find(t=>t.name===op.name);
    const flags=old?{style:old.style,showBandedColumns:old.showBandedColumns,showFilterButton:old.showFilterButton}:null;
    if(old)old.delete();const t=sh.tables.add(op.range,true,op.name);
    if(flags){t.style=flags.style;t.showBandedColumns=flags.showBandedColumns;t.showFilterButton=flags.showFilterButton;}break;
   }
   case 'validation':r.dataValidation={rule:{type:'list',values:op.values}};break;
   case 'conditional_format':r.conditionalFormats.addCustom(op.formula,{fill:op.fill||'#FDE9E7',font:{color:op.color||'#9C231B'}});break;
   case 'chart':addChart(sh,op);break;
   case 'chart_data':sh.charts.items[op.index].setData(r);break;
   default:throw new Error(`Unknown op ${op.op}`);
  }
 }
 wb.recalculate();refreshPivots(wb,spec.pivots||[]);
 const metrics=()=>Object.fromEntries(spec.metrics.map(m=>[m.id,wb.worksheets.getItem(m.sheet).getRange(m.cell).values[0][0]]));
 const check=(expected,actual)=>expected.map(e=>({metric:e.metric,expected:e.value,actual:actual[e.metric],passed:typeof e.value==='number'?typeof actual[e.metric]==='number'&&Math.abs(e.value-actual[e.metric])<=(e.tolerance??1e-6):e.value===actual[e.metric]}));
 const baseline=metrics(),assertions=check(spec.assertions,baseline),tests=[];
 for(const t of spec.tests){
  const saved=[];
  try{
   for(const e of t.edits){const r=wb.worksheets.getItem(e.sheet).getRange(e.cell);saved.push([r,r.values,r.formulas]);r.values=[[typed(e.value)]];}
   wb.recalculate();refreshPivots(wb,spec.pivots||[]);const checks=check(t.expect,metrics());tests.push({name:t.name,passed:checks.every(c=>c.passed),checks});
  }finally{for(const [r,v,f] of saved){if(f[0]?.[0])r.formulas=f;else r.values=v.map(row=>row.map(typed));}wb.recalculate();refreshPivots(wb,spec.pivots||[]);}
 }
 const previews=await render(spec.previews,'previews');
 await (await SpreadsheetFile.exportXlsx(wb)).save(path.join(root,'output/workbook.xlsx'));
 await fs.writeFile(path.join(root,'output/qa.json'),JSON.stringify({schema:'bench.workbook-edit-qa/v1',engine:'Artifact Tool',spec_sha256:await digest(specPath),workbook_sha256:await digest(path.join(root,'output/workbook.xlsx')),baseline,assertions,tests,previews,limitations:['Native Excel UI not tested; independent semantic/visual review required.']},null,2)+'\n');
 console.log(JSON.stringify({baseline,assertions,tests:tests.map(t=>({name:t.name,passed:t.passed})),previews:previews.length}));
 if(assertions.some(x=>!x.passed)||tests.some(x=>!x.passed))process.exitCode=1;
}
