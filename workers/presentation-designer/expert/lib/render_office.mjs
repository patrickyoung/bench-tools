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
  const r=sheet.getRange(`A1:${cell(cols-1)}${last}`);r.format.font={name:t.font,size:11,color:t.ink};r.format.rowHeight=26;r.format.columnWidth=20;r.format.verticalAlignment='top';
 };
 const summaryEnd=16+m.totals.length;style(summary,6,summaryEnd);style(matrix,8,m.cells.length+7);style(evidence,4,4);
 for(const sh of [summary,matrix,evidence,anchors,sensitivity])sh.showGridLines=false;
 matrix.freezePanes.freezeRows(4);matrix.freezePanes.freezeColumns(2);evidence.freezePanes.freezeRows(4);evidence.freezePanes.freezeColumns(2);
 const title=(sheet,title,cols)=>{sheet.getRange(`A2:${cell(cols-1)}2`).merge();sheet.getRange('A2').values=[[safe(title)]];sheet.getRange('A2').format.font={name:t.font,size:17,bold:true,color:t.ink};};
 const header=(sheet,row,values)=>{let r=sheet.getRange(`A${row}:${cell(values.length-1)}${row}`);r.values=[values];r.format={fill:t.ink,font:{name:t.font,color:'#FFFFFF',bold:true},rowHeight:32,wrapText:true,horizontalAlignment:'center',verticalAlignment:'center'};};
 title(summary,spec.title,6);summary.getRange('A4:F5').merge();summary.getRange('A4').values=[[safe(spec.workbook_intro)]];summary.getRange('A4:F5').format.wrapText=true;summary.getRange('A4:F5').format.rowHeight=Math.max(32,Math.ceil(spec.workbook_intro.length/145)*9);
 header(summary,7,['Option','Lower bound','Upper bound','Coverage (%)','Known fit','Gate status']);
 for(let i=0;i<m.totals.length;i++){
  let r=m.totals[i],row=8+i;summary.getRange(`A${row}:F${row}`).values=[[safe(r.name),r.lower_bound,r.upper_bound,r.coverage_percent,r.known_only_fit,safe(r.eligibility)]];summary.getRange(`B${row}:E${row}`).setNumberFormat('0.0');
  summary.getRange(`F${row}`).format.horizontalAlignment='center';
 }
 const decisionRow=10+m.totals.length,decisionText=`Recommendation: ${src.decision.status}${src.decision.candidate_id?' / '+src.candidate_names[src.decision.candidate_id]:''}. ${story.messages[0].qualification}`;
 summary.getRange(`A${decisionRow}:F${decisionRow+2}`).merge();summary.getRange(`A${decisionRow}`).values=[[safe(decisionText)]];summary.getRange(`A${decisionRow}:F${decisionRow+2}`).format.wrapText=true;summary.getRange(`A${decisionRow}:F${decisionRow+2}`).format.rowHeight=Math.max(26,Math.ceil(decisionText.length/145)*7);
 const noteRow=decisionRow+4;summary.getRange(`A${noteRow}:F${noteRow+2}`).merge();summary.getRange(`A${noteRow}`).values=[['Bounds describe missing score evidence, not statistical confidence intervals. This is a saved evaluation. Points formulas expose the calculation; revised weights or scores require a fresh evaluation and review.']];summary.getRange(`A${noteRow}:F${noteRow+2}`).format.wrapText=true;
 title(matrix,'Scoring detail',8);header(matrix,4,['Option','Criterion','Weight (%)','Score /5','Points','Confidence','Status','Source locators']);
 let crit=Object.fromEntries(m.criteria.map(c=>[c.id,c]));
 for(let i=0;i<m.cells.length;i++){
  const x=m.cells[i],row=5+i,c=crit[x.criterion_id];matrix.getRange(`A${row}:H${row}`).values=[[safe(src.candidate_names[x.candidate_id]),safe(c.name),c.weight,x.score,null,x.confidence,x.status,safe(x.refs.map(r=>r.source_id+' · '+r.locator).join('; '))]];
  matrix.getRange(`E${row}`).formulas=[[`=IF(ISNUMBER(D${row}),C${row}*D${row}/5,"")`]];matrix.getRange(`C${row}:E${row}`).setNumberFormat('0.0');matrix.getRange(`A${row}:H${row}`).format.wrapText=true;matrix.getRange(`A${row}:H${row}`).format.rowHeight=58;
 }
 matrix.getRange(`B1:B${m.cells.length+7}`).format.columnWidth=30;matrix.getRange(`H1:H${m.cells.length+7}`).format.columnWidth=44;matrix.tables.add(`A4:H${m.cells.length+4}`,true,'ComparisonScores');
 title(evidence,'Evidence and rationale',4);header(evidence,4,['Option','Criterion','Part','Rationale / cited evidence']);let er=5;
 for(const x of m.cells){
  const text=x.rationale+'\nSources: '+x.refs.map(r=>r.source_id+' / '+r.locator+': '+r.quote).join('\n');let parts=chunks(text);
  for(let i=0;i<parts.length;i++,er++){evidence.getRange(`A${er}:D${er}`).values=[[safe(src.candidate_names[x.candidate_id]),safe(crit[x.criterion_id].name),i+1,safe(parts[i])]];evidence.getRange(`A${er}:D${er}`).format.font={name:t.font,size:11,color:t.ink};evidence.getRange(`A${er}:D${er}`).format.wrapText=true;evidence.getRange(`A${er}:D${er}`).format.rowHeight=72;}
 }
 evidence.getRange(`D1:D${er}`).format.columnWidth=95;evidence.getRange(`B1:B${er}`).format.columnWidth=29;
 let ar=5;const anchorRows=[];for(const c of m.criteria){for(const [score,meaning] of [['Reason',c.reason],...Object.entries(c.anchors)])for(const part of chunks(meaning))anchorRows.push([safe(c.name),c.weight,score==='Reason'?score:Number(score),safe(part)]);}
 style(anchors,4,anchorRows.length+4);title(anchors,'Criteria, weights and exact score anchors',4);header(anchors,4,['Criterion','Weight (%)','Score /5','Definition and rationale']);anchors.getRange(`A5:D${anchorRows.length+4}`).values=anchorRows;anchors.getRange(`A5:D${anchorRows.length+4}`).format.wrapText=true;anchors.getRange(`A5:D${anchorRows.length+4}`).format.rowHeight=68;anchors.getRange(`A1:A${anchorRows.length+4}`).format.columnWidth=31;anchors.getRange(`D1:D${anchorRows.length+4}`).format.columnWidth=100;anchors.freezePanes.freezeRows(4);
 const scenarios=[];for(const s of m.sensitivity)for(const r of m.totals)scenarios.push([safe(crit[s.criterion_id].name),s.factor,safe(r.name),s.lower_bounds[r.candidate_id],safe(m.criteria.map(c=>c.name+': '+s.weights[c.id]+'%').join('; '))]);
 style(sensitivity,5,scenarios.length+5);title(sensitivity,'One-at-a-time relative-weight scenarios',5);header(sensitivity,4,['Changed criterion','Multiplier','Option','Supported score','Renormalized weights']);
 if(scenarios.length){sensitivity.getRange(`A5:E${scenarios.length+4}`).values=scenarios;sensitivity.getRange(`A5:E${scenarios.length+4}`).format.wrapText=true;sensitivity.getRange(`A5:E${scenarios.length+4}`).format.rowHeight=68;sensitivity.getRange(`B5:B${scenarios.length+4}`).setNumberFormat('0.0');sensitivity.getRange(`D5:D${scenarios.length+4}`).setNumberFormat('0.000000');}
 sensitivity.getRange(`A1:A${scenarios.length+5}`).format.columnWidth=31;sensitivity.getRange(`E1:E${scenarios.length+5}`).format.columnWidth=100;sensitivity.freezePanes.freezeRows(4);wb.recalculate();
 const computed=matrix.getRange(`E5:E${m.cells.length+4}`).values;
 for(let i=0;i<m.cells.length;i++){let x=m.cells[i];if(x.score!==null&&Math.abs(Number(computed[i][0])-x.score*crit[x.criterion_id].weight/5)>1e-8)throw Error('Workbook points differ from checked source');if(x.score===null&&computed[i][0]!==''&&computed[i][0]!==null)throw Error('Unknown score became numeric');}
 await (await SpreadsheetFile.exportXlsx(wb)).save(path.join(out,'comparison.xlsx'));
 const regions=[['Overview',`A1:F${summaryEnd}`]];
 for(const [sheet,last,col] of [['Matrix',m.cells.length+4,'H'],['Evidence',er-1,'D'],['Scoring basis',anchorRows.length+4,'D'],['Weight scenarios',scenarios.length+4,'E']])for(let start=1;start<=last;start+=12)regions.push([sheet,`A${start}:${col}${Math.min(start+11,last)}`]);
 for(let index=0;index<regions.length;index++){
  const [sheet,range]=regions[index],png=await wb.render({sheetName:sheet,range,scale:1.3,format:'png'});await fs.writeFile(path.join(pre,`${index+1}-${sheet.toLowerCase()}.png`),new Uint8Array(await png.arrayBuffer()));
 }
 await fs.writeFile(path.join(root,'workbook-check.json'),JSON.stringify({weighted_points:computed,all_rows:m.cells.length,evidence_rows:er-5,anchor_rows:anchorRows.length,scenario_rows:scenarios.length,regions}));
}else{
 const SKILL=process.env.PUBLICATION_SLIDE_SKILL;
 const {resolvePresentationFont,applyPresentationChartFont,finalizePresentation}=await import(pathToFileURL(path.join(SKILL,'container_tools/artifact_tool_utils.mjs')).href);
 const family=resolvePresentationFont({fontFamily:t.font});const p=Presentation.create({slideSize:{width:1280,height:720}});let chartOwners=[],tableOwners=[];
 function text(slide,value,x,y,w,h,size=26,color=t.ink,bold=false){const box=slide.shapes.add({geometry:'textbox',position:{left:x,top:y,width:w,height:h},fill:'none',line:{fill:'none',width:0}});box.text=value;box.text.style={typeface:family,fontSize:size,color,bold,autoFit:'none'};return box;}
 const slides=spec.slides.flatMap(s=>{const per=s.kind==='comparison_table'?4:s.kind==='score_chart'?6:0;return per?Array.from({length:Math.ceil(m.totals.length/per)},(_,i)=>({...s,rows:m.totals.slice(i*per,i*per+per),continuation:i})): [s];});
 for(let i=0;i<slides.length;i++){
  const s=slides[i],sl=p.slides.add();sl.background.fill=i===0?t.ink:t.paper;
  if(s.kind==='cover'){
   text(sl,s.title,76,120,1080,180,64,'#FFFFFF',true);text(sl,s.lead,80,340,1060,110,32,'#FFFFFF');text(sl,`Decision status: ${src.decision.status}`,80,565,1080,45,24,'#DDE8EC');
  }else{
   text(sl,s.title,70,45,1130,95,44,t.ink,true);text(sl,s.lead,74,157,1120,86,27,t.accent,true);
   if(s.kind==='score_chart'){
    const chart=sl.charts.add('bar',{position:{left:75,top:265,width:810,height:335},categories:s.rows.map(r=>r.name),series:[{name:'Supported points',values:s.rows.map(r=>r.lower_bound),fill:t.accent},{name:'Unresolved potential',values:s.rows.map(r=>r.upper_bound-r.lower_bound),fill:'#DAE2E8'}],barOptions:{direction:'bar',grouping:'stacked'},xAxis:{visible:true,textStyle:{fontSize:24}},yAxis:{visible:true,min:0,max:100,majorUnit:25,textStyle:{fontSize:24},numberFormatCode:'0'},hasLegend:true,legend:{position:'bottom',textStyle:{fontSize:24}},dataLabels:{showValue:false}});applyPresentationChartFont(chart,{fontFamily:family});chartOwners.push(i+1);text(sl,s.body.join('\n\n'),920,275,290,315,23);text(sl,'Score bounds describe missing evidence. They are not statistical confidence intervals.',75,620,1120,48,18,'#526476');
   }else if(s.kind==='comparison_table'){
    const vals=[['Option','Fit bounds /100','Coverage','Gates'],...s.rows.map(r=>[r.name,`${r.lower_bound}–${r.upper_bound}`,`${r.coverage_percent}%`,r.eligibility])];
    const tab=sl.tables.add({rows:vals.length,columns:4,left:76,top:275,width:1125,height:Math.min(295,70*vals.length),values:vals,columnWidths:[345,280,220,280]});
    for(let r=0;r<vals.length;r++)for(let c=0;c<4;c++){const z=tab.getCell(r,c);z.fill=r===0?t.ink:r%2?'#EDF1F4':t.paper;z.text.style={typeface:family,fontSize:24,color:r===0?'#FFFFFF':t.ink,bold:r===0};}
    tableOwners.push(i+1);text(sl,s.body.join(' '),76,600,1120,78,22,'#526476');
   }else if(s.kind==='tradeoff'){
    s.body.forEach((b,j)=>{text(sl,String(j+1).padStart(2,'0'),76,285+j*91,70,55,35,t.accent,true);text(sl,b,175,285+j*91,1020,76,27);});
   }else if(s.kind==='decision'){
    text(sl,src.decision.status.toUpperCase(),75,279,1120,85,56,t.accent,true);s.body.forEach((b,j)=>text(sl,b,78,400+j*67,1110,61,26));
   }else{
    s.body.forEach((b,j)=>text(sl,b,78,287+j*88,1100,78,29));
   }
  }
  sl.speakerNotes.textFrame.setText(s.notes+'\n\nEvidence references: '+s.fact_ids.join(', ')+'\nMessages: '+s.message_ids.join(', '));
 }
 const stage=await fs.mkdtemp(path.join(root,'.codex-finalizer-'));const candidate=path.join(stage,'candidate.pptx');await (await PresentationFile.exportPptx(p)).save(candidate);
 await finalizePresentation({workspaceDir:root,candidatePath:candidate,finalPath:path.join(out,'presentation.pptx'),pythonExecutable:process.env.PUBLICATION_PYTHON,integrityValidatorPath:path.join(SKILL,'container_tools/inspect_presentation_package_integrity.py'),layoutValidatorPath:path.join(SKILL,'container_tools/inspect_presentation_layout_geometry.py'),layoutArgs:['--expected-slide-size-emu','12192000,6858000','--validate-heading-fit',...tableOwners.flatMap(n=>['--require-native-table-slide',String(n)])],explicitTotalSlideCount:slides.length,requiredNativeTableOwnerSlides:tableOwners,requiredNativeChartOwnerSlides:chartOwners,materializeLiteralChartWorkbooks:true,fontPolicy:{basis:'design',families:[family]},verifyArtifactToolImport:true,receiptPath:path.join(stage,'validation.json')});
}
