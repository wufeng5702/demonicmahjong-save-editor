import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { slotState, requestFor, matches, isDirty, selectionFits, saveLabel, deckKeysForGroup, toggleDeckKeys, warningEffect } from '../web/model.js';

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
test('deck is omitted until changed, then sends all canonical entries',() => {
  const deck=Array.from({length:34},(_,index)=>({key:`tile${index}`,score:'0.0',baoId:0,lingId:0}));
  const withDeck={...save,deck};
  const draft=structuredClone(withDeck);
  assert.equal(requestFor(withDeck,draft).deck,undefined);
  draft.deck[0].score='20';
  const request=requestFor(withDeck,draft);
  assert.equal(request.deck.length,34);
  assert.deepEqual(request.deck[0],{key:'tile0',score:'20',baoId:0,lingId:0});
  assert.equal(isDirty(withDeck,draft),true);
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
  assert.equal(script.includes('点击卡片多选'),false);
});
test('inventory uses shared minimums and candidates fit their content',() => {
  const css=readFileSync(new URL('../web/compact.css',import.meta.url),'utf8');
  assert.match(css,/\.slots\s*\{[^}]*minmax\(128px,max-content\)/);
  assert.match(css,/\.slot\s*\{[^}]*min-height:\s*128px/);
  assert.match(css,/\.slot \.portrait\s*\{[^}]*width:\s*72px;\s*height:\s*72px/);
  assert.match(css,/\.catalog-grid\s*\{[^}]*grid-auto-rows:\s*max-content/);
  assert.match(css,/\.catalog-grid\s*\{[^}]*align-items:\s*stretch/);
  assert.match(css,/\.catalog-card\s*\{[^}]*min-height:\s*initial;\s*height:\s*auto/);
  assert.doesNotMatch(css,/\.catalog-card\s*\{[^}]*min-height:\s*0/);
});
test('save selector is not an editable path input',() => {
  const html=readFileSync(new URL('../web/index.html',import.meta.url),'utf8');
  assert.match(html,/<select id="path"/);
  assert.doesNotMatch(html,/<input id="path"/);
  assert.equal(html.includes('save-paths'),false);
});
test('deck tab and fixed four-row grid are present',() => {
  const html=readFileSync(new URL('../web/index.html',import.meta.url),'utf8');
  const css=readFileSync(new URL('../web/compact.css',import.meta.url),'utf8');
  const script=readFileSync(new URL('../web/app.js',import.meta.url),'utf8');
  assert.match(html,/data-tab="deck">牌库/);
  assert.match(css,/grid-template-columns:repeat\(9,minmax\(0,1fr\)\)/);
  assert.match(script,/\[\.\.\.honors\.slice\(0,4\),null,\.\.\.honors\.slice\(4\),null\]/);
  assert.doesNotMatch(script,/deck-face-stack|261 \* 0\.8|paimianDimensions/);
  assert.match(script,/class="deck-summary\$\{lingClass\}"/);
  assert.match(script,/class="deck-summary\$\{baoClass\}"/);
  assert.doesNotMatch(script,/deck-summary score|deck-bao-text|showBaoText/);
  assert.equal(existsSync(new URL('../assets/baopai/',import.meta.url)),false);
  assert.equal(existsSync(new URL('../web/paimian-dimensions.js',import.meta.url)),false);
  assert.doesNotMatch(script,/class="deck-name"/);
  assert.match(script,/quickButton\('wan','万'\).*quickButton\('tong','筒'\).*quickButton\('suo','索'\).*quickButton\('zi','字'\).*quickButton\('odd','奇'\).*quickButton\('even','偶'\)/s);
});
test('deck quick selection supports suits, honors, odd and even numbers',() => {
  const deck=[...Array.from({length:9},(_,i)=>({key:`wan${i+1}`,group:'万'})),...Array.from({length:9},(_,i)=>({key:`tong${i+1}`,group:'筒'})),...Array.from({length:9},(_,i)=>({key:`suo${i+1}`,group:'索'})),...['dong','nan','xi','bei','zhong','fa','bai'].map(key=>({key,group:'字'}))];
  assert.equal(deckKeysForGroup(deck,'wan').length,9);
  assert.equal(deckKeysForGroup(deck,'zi').length,7);
  assert.equal(deckKeysForGroup(deck,'odd').length,15);
  assert.equal(deckKeysForGroup(deck,'even').length,12);
  const selected=new Set();
  toggleDeckKeys(selected,deckKeysForGroup(deck,'wan'));
  toggleDeckKeys(selected,deckKeysForGroup(deck,'odd'));
  assert.equal(selected.size,19);
  toggleDeckKeys(selected,deckKeysForGroup(deck,'wan'));
  assert.deepEqual([...selected].sort(),deckKeysForGroup(deck,'odd').filter(key=>!key.startsWith('wan')).sort());
});
test('all 34 embedded tile images are present',() => {
  const dir=new URL('../assets/paimian/',import.meta.url);
  const files=readdirSync(dir).filter(name=>name.endsWith('.png')).sort();
  assert.equal(files.length,34);
  for(const name of ['Wan1.png','Wan9.png','Bing1.png','Bing9.png','Tiao1.png','Tiao9.png','Dong.png','Nan.png','XI.png','Bei.png','Zhong.png','Fa.png','Bai.png']) assert.equal(files.includes(name),true,name);
});
test('negative pailing and baopai names are highlighted',() => {
  for(const name of ['嗔念','痴念','贪念','执念','杂念','恶念']) assert.equal(warningEffect('pailing',name),true,name);
  assert.equal(warningEffect('pailing','鼠灵'),false);
  for(const name of ['迷幻牌','毒宝牌','雾宝牌']) assert.equal(warningEffect('baopai',name),true,name);
  assert.equal(warningEffect('baopai','红宝牌'),false);
  assert.equal(warningEffect('baopai','—'),false);
});
test('application icon is used in the page header and favicon',() => {
  const html=readFileSync(new URL('../web/index.html',import.meta.url),'utf8');
  assert.match(html,/<link rel="icon"[^>]+href="\/app-icon\.png"/);
  assert.match(html,/<img class="logo" src="\/app-icon\.png"/);
  assert.equal(html.includes('<span class="logo">俑</span>'),false);
});
test('slot labels include account, slot number and complete path',() => {
  const a='E:/Steam/userdata/111/3444020/remote/Slot0.json';
  const b='E:/Steam/userdata/222/3444020/remote/Slot0.json';
  assert.equal(saveLabel(a),`111 · 1号存档 · ${a}`);
  assert.equal(saveLabel(b),`222 · 1号存档 · ${b}`);
  assert.equal(saveLabel('C:/copies/Slot0 copy.json'),'外部文件 · Slot0 copy.json · C:/copies/Slot0 copy.json');
  assert.equal(saveLabel('C:\\copies\\Slot2.json'),'外部文件 · 3号存档 · C:\\copies\\Slot2.json');
});
test('display slots 1/2/3 keep filenames Slot0/1/2 unchanged',() => {
  for (const index of [0,1,2]) {
    const path=`C:/copies/Slot${index}.json`;
    assert.equal(saveLabel(path),`外部文件 · ${index+1}号存档 · ${path}`);
  }
});
