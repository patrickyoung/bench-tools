import fs from 'node:fs/promises';
import path from 'node:path';
import {pathToFileURL} from 'node:url';
import {Workbook,SpreadsheetFile,Presentation,PresentationFile} from '@oai/artifact-tool';
const root=path.resolve(process.argv[2]),role=process.argv[3];
const read=async p=>JSON.parse(await fs.readFile(path.join(root,p),'utf8'));
const src=await read('inputs/source.json'),story=(await read('inputs/story.json')).content,spec=(await read('output/spec.json')).content,t=story.design,m=src.matrix;
const out=path.join(root,'output'),pre=path.join(root,'previews',role==='information-designer'?'workbook':'slides');await fs.mkdir(pre,{recursive:true});
const safe=v=>typeof v==='string'&&/^[=+@-]/.test(v)?"'"+v:v;
const cell=(n)=>{let a='';for(let i=n+1;i>0;i=Math.floor((i-1)/26))a=String.fromCharCode(65+(i-1)%26)+a;return a;};
const chunks=text=>{let parts=[],start=0;while(start<text.length){let end=Math.min(start+200,text.length);if(end<text.length){const space=text.lastIndexOf(' ',end);if(space>start+100)end=space+1;}parts.push(text.slice(start,end));start=end;}if(parts.join('')!==text)throw Error('Evidence text loss');return parts;};
if(role==='information-designer'){
 const wb=Workbook.create(),summary=wb.worksheets.add('Overview'),matrix=wb.worksheets.add('Matrix'),evidence=wb.worksheets.add('Evidence'),anchors=wb.worksheets.add('Scoring basis'),sensitivity=wb.worksheets.add('Weight scenarios');
 const style=(sheet,cols,last)=>{
  const r=sheet.getRange(`A1:${cell(cols-1)}${last}`);r.format.font={name:t.font,size:11,color:t.ink};r.format.rowHeight=26;r.format.columnWidth=20;r.format.verticalAlignment='top';r.format.borders={insideVertical:{style:'thin',color:'#D7DFE5'},insideHorizontal:{style:'thin',color:'#E8EDF1'}};
 };
 const summaryEnd=16+m.totals.length;style(summary,6,summaryEnd);style(matrix,8,m.cells.length+7);style(evidence,4,4);
 for(const sh of [summary,matrix,evidence,anchors,sensitivity])sh.showGridLines=false;
 matrix.freezePanes.freezeRows(4);matrix.freezePanes.freezeColumns(2);evidence.freezePanes.freezeRows(4);evidence.freezePanes.freezeColumns(2);
 const title=(sheet,title,cols)=>{sheet.getRange(`A2:${cell(cols-1)}2`).merge();sheet.getRange('A2').values=[[safe(title)]];sheet.getRange('A2').format.font={name:t.font,size:17,bold:true,color:t.ink};};
 const header=(sheet,row,values)=>{let r=sheet.getRange(`A${row}:${cell(values.length-1)}${row}`);r.values=[values];r.format={fill:t.ink,font:{name:t.font,color:'#FFFFFF',bold:true},rowHeight:32,wrapText:true,horizontalAlignment:'center',verticalAlignment:'center'};};
 title(summary,spec.title,6);summary.getRange('A4:F5').merge();summary.getRange('A4').values=[[safe(spec.workbook_intro)]];summary.getRange('A4:F5').format.wrapText=true;summary.getRange('A4:F5').format.rowHeight=Math.max(32,spec.workbook_intro.split('\n').reduce((n,line)=>n+Math.max(1,Math.ceil(line.length/125)),0)*8+8);
 header(summary,7,['Option','Lower bound','Upper bound','Coverage (%)','Known fit','Gate status']);
 for(let i=0;i<m.totals.length;i++){
  let r=m.totals[i],row=8+i;summary.getRange(`A${row}:F${row}`).values=[[safe(r.name),r.lower_bound,r.upper_bound,r.coverage_percent,r.known_only_fit,safe(r.eligibility)]];summary.getRange(`B${row}:E${row}`).setNumberFormat('0.0');
  summary.getRange(`F${row}`).format.horizontalAlignment='center';
 }
 const decisionRow=10+m.totals.length,decisionText=`Recommendation: ${src.decision.status}${src.decision.candidate_id?' / '+src.candidate_names[src.decision.candidate_id]:''}. ${story.messages[0].qualification}`;
 summary.getRange(`A${decisionRow}:F${decisionRow+2}`).merge();summary.getRange(`A${decisionRow}`).values=[[safe(decisionText)]];summary.getRange(`A${decisionRow}:F${decisionRow+2}`).format.wrapText=true;summary.getRange(`A${decisionRow}:F${decisionRow+2}`).format.rowHeight=Math.max(26,Math.ceil(decisionText.length/145)*7);
 const noteRow=decisionRow+4;summary.getRange(`A${noteRow}:F${noteRow+2}`).merge();summary.getRange(`A${noteRow}`).values=[['Bounds describe missing score evidence, not statistical confidence intervals. This is a saved evaluation. Points formulas expose the calculation; revised weights or scores require a fresh evaluation and review.']];summary.getRange(`A${noteRow}:F${noteRow+2}`).format.wrapText=true;
 title(matrix,'Scoring detail',8);header(matrix,4,['Option','Criterion','Weight (%)','Score /5','Points','Confidence','Status','Source locators']);
 matrix.getRange('A3:H3').merge();matrix.getRange('A3').values=[['Confidence describes evidence mapping and provenance, not statistical certainty or population benefit. Unknown scores remain unscored.']];matrix.getRange('A3:H3').format.font.size=10;
 let crit=Object.fromEntries(m.criteria.map(c=>[c.id,c]));
 for(let i=0;i<m.cells.length;i++){
  const x=m.cells[i],row=5+i,c=crit[x.criterion_id];matrix.getRange(`A${row}:H${row}`).values=[[safe(src.candidate_names[x.candidate_id]),safe(c.name),c.weight,x.score,null,x.confidence,x.status,safe(x.refs.map(r=>r.source_id+' · '+r.locator).join('; '))]];
  matrix.getRange(`E${row}`).formulas=[[`=IF(ISNUMBER(D${row}),C${row}*D${row}/5,"")`]];matrix.getRange(`C${row}:E${row}`).setNumberFormat('0.0');matrix.getRange(`C${row}:G${row}`).format.horizontalAlignment='center';matrix.getRange(`A${row}:H${row}`).format.wrapText=true;matrix.getRange(`A${row}:H${row}`).format.rowHeight=Math.max(58,Math.ceil(String(matrix.getRange(`H${row}`).values[0][0]).length/42)*14+14);
 }
 matrix.getRange(`B1:B${m.cells.length+7}`).format.columnWidth=30;matrix.getRange(`H1:H${m.cells.length+7}`).format.columnWidth=44;matrix.tables.add(`A4:H${m.cells.length+4}`,true,'ComparisonScores');
 title(evidence,'Evidence and rationale',4);header(evidence,4,['Option','Criterion','Section','Rationale and supporting references']);let er=5;
 const evidenceRow=(x,section,content)=>{evidence.getRange(`A${er}:D${er}`).values=[[safe(src.candidate_names[x.candidate_id]),safe(crit[x.criterion_id].name),section,safe(content)]];evidence.getRange(`A${er}:D${er}`).format.font={name:t.font,size:11,color:t.ink};evidence.getRange(`A${er}:D${er}`).format.wrapText=true;evidence.getRange(`A${er}:D${er}`).format.verticalAlignment='top';const height=Math.max(48,content.split('\n').reduce((a,s)=>a+Math.max(1,Math.ceil(s.length/100)),0)*15+15);if(height>400)throw Error('Evidence needs a shorter analytical rationale for readable workbook publication');evidence.getRange(`A${er}:D${er}`).format.rowHeight=height;if(section==='Score rationale'){evidence.getRange(`A${er}:D${er}`).format.borders={top:{style:'thin',color:'#9CAEBB'}};evidence.getRange(`A${er}:C${er}`).format.font.bold=true;}er++;};
 for(const x of m.cells){
  if(x.rationale.length>2500)throw Error('Rationale exceeds a readable Excel row; refine the analytical rationale before publication');
  const sentences=x.rationale.split(/(?<=[.!?])\s+(?=[A-Z])/u),paragraphs=[];for(let i=0;i<sentences.length;i+=2)paragraphs.push(sentences.slice(i,i+2).join(' '));evidenceRow(x,'Score rationale',paragraphs.join('\n\n'));
  evidenceRow(x,'Source references',x.refs.length?x.refs.map(r=>r.source_id+' / '+r.locator).join('\n')+'\nFull verbatim quotations: comparison-data.json.':'No supporting source was supplied. This is an explicit evidence gap.');
 }
 evidence.getRange(`A1:A${er}`).format.columnWidth=23;evidence.getRange(`B1:B${er}`).format.columnWidth=28;evidence.getRange(`C1:C${er}`).format.columnWidth=20;evidence.getRange(`D1:D${er}`).format.columnWidth=90;
 const anchorRows=[];for(const c of m.criteria){for(const [score,meaning] of [['Reason',c.reason],...Object.entries(c.anchors)])anchorRows.push([safe(c.name),c.weight,score==='Reason'?score:Number(score),safe(meaning)]);}
 style(anchors,4,anchorRows.length+4);title(anchors,'Criteria, weights and exact score anchors',4);header(anchors,4,['Criterion','Weight (%)','Score /5','Definition and rationale']);anchors.getRange(`A5:D${anchorRows.length+4}`).values=anchorRows;anchors.getRange(`A5:D${anchorRows.length+4}`).format.wrapText=true;anchors.getRange(`A5:D${anchorRows.length+4}`).format.rowHeight=45;anchors.getRange(`A1:A${anchorRows.length+4}`).format.columnWidth=31;anchors.getRange(`D1:D${anchorRows.length+4}`).format.columnWidth=100;anchors.freezePanes.freezeRows(4);
 for(let i=0;i<anchorRows.length;i++){anchors.getRange(`A${i+5}:D${i+5}`).format.rowHeight=Math.max(32,Math.ceil(anchorRows[i][3].length/100)*14+14);anchors.getRange(`B${i+5}:C${i+5}`).format.horizontalAlignment='center';if(anchorRows[i][2]==='Reason'){anchors.getRange(`A${i+5}:D${i+5}`).format.fill='#EDF1F4';anchors.getRange(`A${i+5}:C${i+5}`).format.font.bold=true;}}
 const scenarios=[];for(const s of m.sensitivity)for(const r of m.totals)scenarios.push([safe(crit[s.criterion_id].name),s.factor,safe(r.name),s.lower_bounds[r.candidate_id],...m.criteria.map(c=>s.weights[c.id])]);
 const scenarioCol=cell(3+m.criteria.length);style(sensitivity,4+m.criteria.length,scenarios.length+5);title(sensitivity,'Separate relative-weight scenarios · conservative scores /100',4+m.criteria.length);header(sensitivity,4,['Changed criterion','Multiplier','Option','Supported /100',...m.criteria.map(c=>c.name+' (%)')]);sensitivity.getRange(`A4:${scenarioCol}4`).format.rowHeight=60;
 if(scenarios.length){sensitivity.getRange(`A5:${scenarioCol}${scenarios.length+4}`).values=scenarios;sensitivity.getRange(`A5:${scenarioCol}${scenarios.length+4}`).format.wrapText=true;sensitivity.getRange(`A5:${scenarioCol}${scenarios.length+4}`).format.rowHeight=38;sensitivity.getRange(`B5:B${scenarios.length+4}`).setNumberFormat('0.0');sensitivity.getRange(`D5:${scenarioCol}${scenarios.length+4}`).setNumberFormat('0.00');sensitivity.getRange(`B5:${scenarioCol}${scenarios.length+4}`).format.horizontalAlignment='center';for(let i=0;i<scenarios.length;i++)if(Math.floor(i/m.totals.length)%2===0)sensitivity.getRange(`A${i+5}:${scenarioCol}${i+5}`).format.fill='#EDF1F4';}
 sensitivity.getRange(`A1:A${scenarios.length+5}`).format.columnWidth=31;sensitivity.freezePanes.freezeRows(4);wb.recalculate();
 const computed=matrix.getRange(`E5:E${m.cells.length+4}`).values;
 for(let i=0;i<m.cells.length;i++){let x=m.cells[i];if(x.score!==null&&Math.abs(Number(computed[i][0])-x.score*crit[x.criterion_id].weight/5)>1e-8)throw Error('Workbook points differ from checked source');if(x.score===null&&computed[i][0]!==''&&computed[i][0]!==null)throw Error('Unknown score became numeric');}
 await (await SpreadsheetFile.exportXlsx(wb)).save(path.join(out,'comparison.xlsx'));
 await fs.writeFile(path.join(out,'comparison-data.json'),JSON.stringify(m,null,2)+'\n');
 const regions=[['Overview',`A1:F${summaryEnd}`]];
 for(const [sheet,last,col] of [['Matrix',m.cells.length+4,'H'],['Evidence',er-1,'D'],['Scoring basis',anchorRows.length+4,'D'],['Weight scenarios',scenarios.length+4,scenarioCol]]){
  const sh=wb.worksheets.getItem(sheet);let start=1,height=100;
  if(sheet==='Scoring basis'){for(let row=5;row<=last;row+=7)regions.push([sheet,`A${row===5?1:row}:${col}${Math.min(last,row+6)}`]);continue;}
  if(sheet==='Weight scenarios'||sheet==='Evidence'){const group=sheet==='Evidence'?2:m.totals.length;for(let row=5;row<=last;row+=group){let h=0;for(let r=row;r<Math.min(row+group,last+1);r++)h+=Number(sh.getRange(`A${r}`).format.rowHeight);if(row>5&&height+h>600){regions.push([sheet,`A${start}:${col}${row-1}`]);start=row;height=100;}height+=h;}regions.push([sheet,`A${start}:${col}${last}`]);continue;}
  for(let row=5;row<=last;row++){const h=Number(sh.getRange(`A${row}`).format.rowHeight);if(row>Math.max(5,start)&&height+h>600){regions.push([sheet,`A${start}:${col}${row-1}`]);start=row;height=100;}height+=h;}
  regions.push([sheet,`A${start}:${col}${last}`]);
 }
 const previewSheet=wb.worksheets.add('_Publication preview');previewSheet.showGridLines=false;
 for(let index=0;index<regions.length;index++){
  const [sheet,range]=regions[index];let renderSheet=sheet,renderRange=range;
  if(sheet!=='Overview'){
   const sh=wb.worksheets.getItem(sheet),match=range.match(/^A(\d+):([A-Z]+)(\d+)$/),first=Math.max(5,Number(match[1])),last=Number(match[3]),col=match[2],count=last-first+1;
   previewSheet.getRange('A1:Z100').unmerge();previewSheet.getRange('A1:Z100').clear({applyTo:'all'});previewSheet.getRange(`A1:${col}4`).copyFrom(sh.getRange(`A1:${col}4`),'values');previewSheet.getRange(`A5:${col}${count+4}`).values=sh.getRange(`A${first}:${col}${last}`).values;
   const cols=col.charCodeAt(0)-64;style(previewSheet,cols,count+4);previewSheet.getRange(`A5:${col}${count+4}`).format.wrapText=true;title(previewSheet,sh.getRange('A2').values[0][0],cols);header(previewSheet,4,sh.getRange(`A4:${col}4`).values[0]);
   if(sh.getRange('A3').values[0][0]){previewSheet.getRange(`A3:${col}3`).merge();previewSheet.getRange('A3').format.font.size=10;}
   if(sheet==='Matrix'){previewSheet.getRange(`C5:G${count+4}`).format.horizontalAlignment='center';previewSheet.getRange(`C5:E${count+4}`).setNumberFormat('0.0');}
   if(sheet==='Scoring basis'){previewSheet.getRange(`B5:C${count+4}`).format.horizontalAlignment='center';for(let j=0;j<count;j++)if(anchorRows[first+j-5][2]==='Reason'){previewSheet.getRange(`A${j+5}:D${j+5}`).format.fill='#EDF1F4';previewSheet.getRange(`A${j+5}:C${j+5}`).format.font.bold=true;}}
   if(sheet==='Evidence')for(let j=0;j<count;j++)if((first+j-5)%2===0){previewSheet.getRange(`A${j+5}:C${j+5}`).format.font.bold=true;previewSheet.getRange(`A${j+5}:D${j+5}`).format.borders={top:{style:'thin',color:'#9CAEBB'}};}
   if(sheet==='Weight scenarios'){previewSheet.getRange(`B5:${col}${count+4}`).format.horizontalAlignment='center';previewSheet.getRange(`B5:B${count+4}`).setNumberFormat('0.0');previewSheet.getRange(`D5:${col}${count+4}`).setNumberFormat('0.00');for(let j=0;j<count;j++)if(Math.floor((first+j-5)/m.totals.length)%2===0)previewSheet.getRange(`A${j+5}:${col}${j+5}`).format.fill='#EDF1F4';}
   for(let c=0;c<=col.charCodeAt(0)-65;c++)previewSheet.getRange(`${cell(c)}1:${cell(c)}${count+4}`).format.columnWidth=Number(sh.getRange(`${cell(c)}1`).format.columnWidth);
   for(let r=1;r<=count+4;r++)previewSheet.getRange(`A${r}:${col}${r}`).format.rowHeight=Number(sh.getRange(`A${r<=4?r:first+r-5}`).format.rowHeight);
   renderSheet=previewSheet.name;renderRange=`A1:${col}${count+4}`;
  }
  const png=await wb.render({sheetName:renderSheet,range:renderRange,scale:1.3,format:'png'});await fs.writeFile(path.join(pre,`${index+1}-${sheet.toLowerCase()}.png`),new Uint8Array(await png.arrayBuffer()));
 }
 await fs.writeFile(path.join(root,'workbook-check.json'),JSON.stringify({weighted_points:computed,all_rows:m.cells.length,evidence_rows:er-5,anchor_rows:anchorRows.length,scenario_rows:scenarios.length,regions}));
}else{
 const SKILL=process.env.PUBLICATION_SLIDE_SKILL;
 const {resolvePresentationFont,applyPresentationChartFont,finalizePresentation}=await import(pathToFileURL(path.join(SKILL,'container_tools/artifact_tool_utils.mjs')).href);
 const family=resolvePresentationFont({fontFamily:t.font});const p=Presentation.create({slideSize:{width:1280,height:720}});let chartOwners=[],tableOwners=[];
 function text(slide,value,x,y,w,h,size=26,color=t.ink,bold=false){const box=slide.shapes.add({geometry:'textbox',position:{left:x,top:y,width:w,height:h},fill:'none',line:{fill:'none',width:0}});box.text=value;box.text.style={typeface:family,fontSize:size,color,bold,autoFit:'none'};return box;}
 function rect(sl,x,y,w,h,fill,geometry='rect'){return sl.shapes.add({geometry,position:{left:x,top:y,width:w,height:h},fill,line:{fill:'none',width:0}});}
 function paragraphs(sl,body,numbered=false){let size=numbered?27:29,width=numbered?1020:1100;const total=()=>body.reduce((sum,b)=>sum+(Math.ceil(b.length/(width/(size*.53)))||1)*size*1.18+24,0);while(total()>390&&size>24)size--;if(total()>390)throw Error('Slide body requires editorial shortening');let y=285;body.forEach((b,j)=>{const h=Math.max(1,Math.ceil(b.length/(width/(size*.53))))*size*1.18+8;if(numbered)text(sl,String(j+1).padStart(2,'0'),76,y,70,55,35,t.accent,true);text(sl,b,numbered?175:78,y,width,h,size);y+=h+16;});}
 const slides=spec.slides.flatMap(s=>{const per=s.kind==='comparison_table'?4:s.kind==='score_chart'?6:0;const pages=per?Array.from({length:Math.ceil(m.totals.length/per)},(_,i)=>({...s,rows:m.totals.slice(i*per,i*per+per),continuation:i})): [s];return s.kind==='comparison_table'&&s.table_view==='criteria'?pages.flatMap(p=>Array.from({length:Math.ceil(m.criteria.length/5)},(_,j)=>({...p,criteria:m.criteria.slice(j*5,j*5+5)}))):pages;});
 for(let i=0;i<slides.length;i++){
  const s=slides[i],sl=p.slides.add();sl.background.fill=i===0?t.ink:t.paper;
  if(s.kind==='cover'){
   text(sl,s.title,76,70,1120,205,54,'#FFFFFF',true);text(sl,s.lead,80,300,1060,110,30,'#FFFFFF');text(sl,s.body.join('\n'),80,435,1080,180,24,'#FFFFFF');text(sl,`Decision status: ${src.decision.status}`,80,655,1080,35,20,'#DDE8EC');
  }else{
   text(sl,s.title,70,45,1130,95,44,t.ink,true);text(sl,s.lead,74,157,1120,86,27,t.accent,true);
   if(s.kind==='score_chart'){
    const chart=sl.charts.add('bar',{position:{left:75,top:265,width:810,height:335},categories:s.rows.map(r=>r.name),series:[{name:'Supported points',values:s.rows.map(r=>r.lower_bound),fill:t.accent},{name:'Unresolved potential',values:s.rows.map(r=>r.upper_bound-r.lower_bound),fill:'#DAE2E8'}],barOptions:{direction:'bar',grouping:'stacked'},xAxis:{visible:true,textStyle:{fontSize:24}},yAxis:{visible:true,min:0,max:100,majorUnit:25,textStyle:{fontSize:24},numberFormatCode:'0'},hasLegend:true,legend:{position:'bottom',textStyle:{fontSize:24}},dataLabels:{showValue:false}});applyPresentationChartFont(chart,{fontFamily:family});chartOwners.push(i+1);text(sl,s.body.join('\n\n'),920,275,290,315,23);text(sl,'Score bounds describe missing evidence. They are not statistical confidence intervals.',75,620,1120,60,20,'#526476');
   }else if(s.kind==='effect_plot'){
    const e=src.statistics.comparisons.find(e=>e.id===s.comparison_id&&e.status==='inferential'),design=(await read('inputs/design.json')).content;
    const refs=design.visuals.flatMap(v=>v.reference_lines||[]).filter(r=>r.comparison_id===e.id);if(new Set(refs.map(r=>r.value)).size>1)throw Error('Conflicting practical thresholds');const ref=refs[0];
    const low=Math.min(0,e.ci_low,ref?.value??0),high=Math.max(0,e.ci_high,ref?.value??0),rough=(high-low)/4,power=10**Math.floor(Math.log10(rough)),fraction=rough/power,step=power*(fraction<=1?1:fraction<=2?2:fraction<=5?5:10),lo=Math.floor(low/step)*step,hi=Math.ceil(high/step)*step;
    const x=v=>130+(v-lo)/(hi-lo)*1020,fmt=v=>Number(v.toFixed(3)).toString();
    text(sl,`Mean ${fmt(e.mean_difference_a_minus_b)} ${e.unit}`,76,266,450,57,38,t.accent,true);text(sl,`${100*e.confidence_level}% marginal CI [${fmt(e.ci_low)}, ${fmt(e.ci_high)}]`,555,274,640,55,27,t.ink,true);
    text(sl,(e.paired_count!==null?`${e.paired_count} matched pairs`:`n = ${e.n_a} / ${e.n_b}`)+` · Holm-adjusted p = ${Number(e.p_holm.toPrecision(3))}`,78,330,1120,44,24);
    for(let y=388;y<470;y+=12)rect(sl,x(0)-1,y,2,6,'#9BA8B0');text(sl,'No difference',Math.min(1000,Math.max(130,x(0)+9)),382,190,30,18,'#526476');
    if(ref){rect(sl,x(ref.value)-1,388,2,82,t.secondary);text(sl,`Practical threshold ${fmt(ref.value)} ${e.unit}`,Math.min(825,Math.max(130,x(ref.value)+10)),438,380,30,20,t.secondary,true);}
    rect(sl,x(e.ci_low),419,x(e.ci_high)-x(e.ci_low),5,t.accent);rect(sl,x(e.ci_low)-2,409,4,25,t.accent);rect(sl,x(e.ci_high)-2,409,4,25,t.accent);rect(sl,x(e.mean_difference_a_minus_b)-9,412,18,18,t.accent,'ellipse');
    rect(sl,130,477,1020,2,'#AAB8C2');for(let tick=lo;tick<=hi+step/10;tick+=step){rect(sl,x(tick),477,1,9,t.ink);text(sl,fmt(tick),x(tick)-48,492,96,35,23).text.alignment="center";}
    text(sl,`${src.candidate_names[e.groups[0]]||e.groups[0]} minus ${src.candidate_names[e.groups[1]]||e.groups[1]} · mean difference (${e.unit})`,78,536,1120,40,22);text(sl,s.body.join(' '),78,594,1120,80,23);
   }else if(s.kind==='comparison_table'){
    let vals,widths,size=24;
    if(s.table_view==='criteria'){
     const delta=s.rows.length===2;size=20;vals=[['Criterion','Weight (%)',...s.rows.map(r=>r.name+' score / points'),...(delta?['First minus second (points)']:[])]];
     for(const criterion of s.criteria){const cells=s.rows.map(r=>m.cells.find(c=>c.candidate_id===r.candidate_id&&c.criterion_id===criterion.id));const points=cells.map(c=>c.score===null?null:c.score*criterion.weight/5);vals.push([criterion.name,String(criterion.weight),...cells.map((c,i)=>c.score===null?'Unknown':`${c.score}/5 · ${points[i].toFixed(1)} pts`),...(delta?[points.includes(null)?'Unknown':(points[0]-points[1]>0?'+':'')+(points[0]-points[1]).toFixed(1)]:[])]);}
     widths=[285,105,...s.rows.map(()=>delta?240:735/s.rows.length),...(delta?[255]:[])];
    }else{vals=[['Option','Fit bounds /100','Coverage','Gates'],...s.rows.map(r=>[r.name,`${r.lower_bound}–${r.upper_bound}`,`${r.coverage_percent}%`,r.eligibility])];widths=[345,280,220,280];}
    const tab=sl.tables.add({rows:vals.length,columns:vals[0].length,left:76,top:275,width:1125,height:Math.min(305,(s.table_view==='criteria'?50:70)*vals.length),values:vals,columnWidths:widths});
    for(let r=0;r<vals.length;r++)for(let c=0;c<vals[0].length;c++){const z=tab.getCell(r,c);z.fill=r===0?t.ink:r%2?'#EDF1F4':t.paper;z.text.style={typeface:family,fontSize:size,color:r===0?'#FFFFFF':t.ink,bold:r===0};}
    tableOwners.push(i+1);text(sl,s.body.join(' '),76,600,1120,78,22,'#526476');
   }else if(s.kind==='tradeoff'){
    paragraphs(sl,s.body,true);
   }else if(s.kind==='decision'){
    paragraphs(sl,s.body,true);
   }else{
    paragraphs(sl,s.body);
   }
  }
  sl.speakerNotes.textFrame.setText(s.notes+'\n\nEvidence references: '+s.fact_ids.join(', ')+'\nMessages: '+s.message_ids.join(', '));
 }
 const stage=await fs.mkdtemp(path.join(root,'.codex-finalizer-'));const candidate=path.join(stage,'candidate.pptx');await (await PresentationFile.exportPptx(p)).save(candidate);
 await finalizePresentation({workspaceDir:root,candidatePath:candidate,finalPath:path.join(out,'presentation.pptx'),pythonExecutable:process.env.PUBLICATION_PYTHON,integrityValidatorPath:path.join(SKILL,'container_tools/inspect_presentation_package_integrity.py'),layoutValidatorPath:path.join(SKILL,'container_tools/inspect_presentation_layout_geometry.py'),layoutArgs:['--expected-slide-size-emu','12192000,6858000','--validate-heading-fit',...tableOwners.flatMap(n=>['--require-native-table-slide',String(n)])],explicitTotalSlideCount:slides.length,requiredNativeTableOwnerSlides:tableOwners,requiredNativeChartOwnerSlides:chartOwners,materializeLiteralChartWorkbooks:true,fontPolicy:{basis:'design',families:[family]},verifyArtifactToolImport:true,receiptPath:path.join(stage,'validation.json')});
}
