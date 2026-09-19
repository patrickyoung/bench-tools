import assert from 'node:assert/strict';
import {observeMetrics,checkMetrics} from '../expert/lib/metrics.mjs';

const fixture=value=>({worksheets:{getItem:()=>({getRange:()=>({values:[[value]]})})}});
const observe=value=>observeMetrics(fixture(value),[{id:'value',sheet:'Records',cell:'B2'}]);
const check=(actual,expected,tolerance)=>checkMetrics([{metric:'value',value:expected,...(tolerance===undefined?{}:{tolerance})}],observe(actual))[0];
let count=0;
function test(name,body){body();count++;console.log('ok '+name);}
const iso='2026-09-19T00:00:00.000Z';
test('Date is recorded as canonical ISO with an explicit marker',()=>assert.deepEqual(observe(new Date(iso)),{values:{value:iso},dateMetrics:['value']}));
test('exact ISO Date expectation passes',()=>assert.equal(check(new Date(iso),iso).passed,true));
test('typed Date expectation passes',()=>assert.equal(check(new Date(iso),{date:'2026-09-19'}).passed,true));
test('wrong day and invalid calendar date reject',()=>{
  for(const expected of [{date:'2026-09-18'},{date:'2026-02-31'},'2026-09-19'])assert.equal(check(new Date(iso),expected).passed,false);
});
test('typed expectation cannot accept ordinary ISO-looking text',()=>{
  assert.deepEqual(observe(iso),{values:{value:iso},dateMetrics:[]});
  assert.equal(check(iso,{date:'2026-09-19'}).passed,false);
  assert.equal(check(iso,iso).passed,true);
});
test('Date does not accept numeric serial, boolean or extra object fields',()=>{
  for(const expected of [46284,true,{date:'2026-09-19',note:'extra'}])assert.equal(check(new Date(iso),expected).passed,false);
});
test('numbers booleans and strings remain distinct',()=>{
  for(const [actual,expected] of [[1,true],[true,1],[1,'1'],['1',1],[false,0],[0,false]])assert.equal(check(actual,expected).passed,false);
  for(const value of [1,true,'1',false,0,null])assert.equal(check(value,value).passed,true);
});
test('numeric serial workaround and tolerance are unchanged',()=>{
  assert.equal(check(46284,46284).passed,true);assert.deepEqual(observe(46284).dateMetrics,[]);
  assert.equal(check(2,2.0001,0.001).passed,true);assert.equal(check(2,2.1,0.001).passed,false);
});
test('invalid runtime Date fails visibly',()=>assert.throws(()=>observe(new Date('invalid')),/Invalid Date metric/));
console.log(JSON.stringify({tests:count,passed:count}));
