// Preserve runtime Date identity through JSON without coercing ordinary text.
export function observeMetrics(wb, metrics) {
  const values={}, dateMetrics=[];
  for (const metric of metrics) {
    const value=wb.worksheets.getItem(metric.sheet).getRange(metric.cell).values[0][0];
    if (value instanceof Date) {
      if (!Number.isFinite(value.getTime())) throw new Error(`Invalid Date metric ${metric.id}`);
      values[metric.id]=value.toISOString();dateMetrics.push(metric.id);
    } else values[metric.id]=value;
  }
  return {values,dateMetrics};
}

export function checkMetrics(expected, observed) {
  return expected.map(e=>{
    const actual=observed.values[e.metric], isDate=observed.dateMetrics.includes(e.metric);
    let want=e.value, passed=false;
    if (isDate) {
      if (want && typeof want==='object' && !Array.isArray(want) &&
          Object.keys(want).length===1 && typeof want.date==='string' &&
          /^\d{4}-\d{2}-\d{2}$/.test(want.date)) {
        const iso=want.date+'T00:00:00.000Z', parsed=new Date(iso);
        want=Number.isFinite(parsed.getTime()) && parsed.toISOString()===iso ? iso : null;
      }
      passed=typeof want==='string' && want===actual;
    } else passed=typeof want==='number' ? typeof actual==='number' &&
      Math.abs(want-actual)<=(e.tolerance??1e-6) : want===actual;
    return {metric:e.metric,expected:e.value,actual,passed,...(isDate?{actual_type:'date'}:{})};
  });
}
