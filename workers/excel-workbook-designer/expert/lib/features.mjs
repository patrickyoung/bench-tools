export function addChart(sh,c){
 const chart=sh.charts.add(c.type,sh.getRange(c.range));chart.title=c.title;
 chart.titleTextStyle.typeface='Arial';chart.titleTextStyle.fontSize=13;
 chart.hasLegend=['pie','doughnut'].includes(c.type)||chart.series.items.length>1;
 if(chart.hasLegend)chart.legend={position:'bottom',textStyle:{typeface:'Arial',fontSize:11}};
 if(!['pie','doughnut'].includes(c.type)){
  chart.xAxis={axisType:'textAxis',textStyle:{typeface:'Arial',fontSize:11}};
  chart.yAxis={numberFormatCode:c.number_format||'#,##0',numberFormatSourceLinked:false,textStyle:{typeface:'Arial',fontSize:11}};
 }
 chart.setPosition(c.from,c.to);return chart;
}
// The bounded pivot bridge owns these cached cells. Real native pivot structures
// are attached by features.py after export; never described as live formulas.
export function refreshPivots(wb,pivots){
 for(const p of pivots){
  const values=wb.worksheets.getItem(p.source_sheet).getRange(p.source_range).values;
  const h=values[0],ri=h.indexOf(p.row_field),vi=h.indexOf(p.value_field);
  if(ri<0||vi<0)throw new Error('Pivot source field missing');
  const groups=new Map();
  for(const r of values.slice(1)){
   if(r.every(x=>x===null||x===''))continue;
   const key=r[ri];if(key===null||key==='')throw new Error('Blank pivot category: resolve or explicitly normalize');
   const v=r[vi];if(v!==null&&v!==''&&typeof v!=='number')throw new Error('Non-numeric pivot measure');
   if(!groups.has(key))groups.set(key,[]);if(typeof v==='number')groups.get(key).push(v);
  }
  const agg=a=>p.aggregate==='count'?a.length:!a.length?null:p.aggregate==='average'?a.reduce((x,y)=>x+y,0)/a.length:p.aggregate==='min'?Math.min(...a):p.aggregate==='max'?Math.max(...a):a.reduce((x,y)=>x+y,0);
  const rows=[ [p.row_field,p.caption], ...[...groups].sort((a,b)=>String(a[0]).localeCompare(String(b[0]))).map(([k,a])=>[k,agg(a)]),['Grand Total',agg([...groups.values()].flat())] ];
  const sh=wb.worksheets.getItem(p.sheet),anchor=sh.getRange(p.cell);
  if(!p.__rows&&anchor.resize(rows.length,2).values.some(row=>row.some(v=>v!==null&&v!=='')))throw new Error('Pivot destination overlaps existing content');
  // Clear the previous test refresh's footprint when a category disappears.
  const old=p.__rows||0;if(old)anchor.resize(old,2).clear({applyTo:'contents'});
  anchor.resize(rows.length,2).values=rows;anchor.resize(1,2).format={fill:'#17324D',font:{name:'Arial',bold:true,color:'#FFFFFF'},rowHeight:26};
  anchor.offset(1,1).resize(rows.length-1,1).setNumberFormat(p.number_format||'#,##0.00');
  p.__rows=rows.length;
 }
}
