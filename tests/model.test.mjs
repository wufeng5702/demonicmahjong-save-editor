import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { slotState, requestFor, matches, isDirty, selectionFits } from '../web/model.js';

const save = {path:'Slot0.json',hash:'unchanged',fields:{Coin:'79228162514264337593543950335.0',LingYongSlotNum:'5',XiaoChouRepositorySlot:'4'},equipped:[1,2],warehouse:[],relics:[{ref:0,id:77}]};
const copy = () => structuredClone(save);

test('5-8 slots, capped at 8 even with accumulated 20',() => {
  for (const n of [5,6,7,8,20]) { const draft=copy();draft.fields.LingYongSlotNum=String(n);assert.equal(slotState(save,draft,{}).limit,Math.min(n,8)); }
});
test('altar delta, not raw reset to default',() => {
  const draft=copy();draft.fields.LingYongSlotNum='20';draft.relics=[];
  assert.deepEqual(slotState(save,draft,{77:1}),{total:19,limit:8,warehouse:4,delta:-1});
});
test('requests retain numeric strings and original relic references',() => {
  const draft=copy();draft.fields.Coin='161461040910412030.0';draft.relics.push({id:3});
  const req=requestFor(save,draft);
  assert.equal(req.fields.Coin,'161461040910412030.0');assert.deepEqual(req.relics,[{ref:0},{id:3}]);
  assert.equal('HadXiaoChouList' in req,false);assert.equal('UnlockedLingYongList' in req,false);
});
test('dirty tracking and no-op requests',() => {
  const draft=copy();assert.equal(isDirty(save,draft),false);assert.deepEqual(requestFor(save,draft).fields,{});
  draft.equipped.reverse();assert.equal(isDirty(save,draft),true);
});
test('derived items visible by default, search and filters',() => {
  const item={name:'雏凤',en:'ChuFeng',id:98,desc:'成长效果',tags:['鸟','衍生'],rar:'传说',derived:true};
  assert.equal(matches(item,'','',''),true);assert.equal(matches(item,'98','传说','鸟'),true);
  assert.equal(matches(item,'','','狗'),false);
});
test('batch add requires enough free slots; no partial additions',() => {
  assert.equal(selectionFits(0,4),false);assert.equal(selectionFits(2,2),true);
  assert.equal(selectionFits(3,2),false);assert.equal(selectionFits(1,0),false);
});
test('removed footer has no dangling DOM references',() => {
  const html=readFileSync(new URL('../web/index.html',import.meta.url),'utf8');
  const script=readFileSync(new URL('../web/app.js',import.meta.url),'utf8');
  assert.equal(html.includes('<footer'),false);
  assert.equal(script.includes("$('summary')"),false);
});
