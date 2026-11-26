import { slotState, requestFor, matches, isDirty, selectionFits, saveLabel, deckKeysForGroup, toggleDeckKeys, warningEffect } from './model.js';

const $ = id => document.getElementById(id);
const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const browserPreview = location.hostname === '127.0.0.1' || location.hostname === 'localhost';
let runtime, token, boot, save, draft, tab = 'lingyong', pending;
let search = '', rarity = '', tag = '', busy = false;
let savePaths = [];
const selectedEquipped = new Set(), selectedWarehouse = new Set(), selectedRelics = new Set(), selectedCandidates = new Set();
const selectedDeck = new Set();
const catalogs = {};
let deckEdit = null;

async function call(method, ...args) {
  if (!browserPreview) return runtime.ByName(`main.Service.${method}`, ...args);
  if (method === 'Bootstrap') {
    const response = await fetch('/api/Bootstrap');
    const result = await response.json();
    if (result.error) throw new Error(result.error);
    token = result.token;
    return result.boot;
  }
  const response = await fetch(`/api/${method}`, { method: 'POST', headers: {'Content-Type':'application/json','X-Editor-Token':token}, body: JSON.stringify(args) });
  const result = await response.json();
  if (!response.ok || result?.error) throw new Error(result?.error || '请求失败');
  return result;
}

function toast(message, error = true) {
  $('toast').textContent = message;
  $('toast').className = error ? 'error' : 'success';
  $('toast').hidden = false;
  clearTimeout(toast.timer);
  toast.timer = setTimeout(() => { $('toast').hidden = true; }, error ? 12000 : 8000);
}

async function action(fn) {
  if (busy) return;
  busy = true;
  $('path').disabled = true;
  document.body.classList.add('busy');
  try { await fn(); } catch (error) { toast(error.message || String(error)); }
  finally { busy = false; $('path').disabled = !savePaths.length; document.body.classList.remove('busy'); }
}

function updateSaveOptions(path) {
  if (path && !savePaths.includes(path)) savePaths.push(path);
  savePaths.sort((a,b) => a.localeCompare(b,'zh',{numeric:true}));
  $('path').innerHTML = savePaths.length ? savePaths.map(p => `<option value="${esc(p)}" title="${esc(p)}">${esc(saveLabel(p))}</option>`).join('') : '<option value="">未检测到存档，请手动选择文件</option>';
  $('path').value = path || save?.path || savePaths[0] || '';
  $('path').title = $('path').value;
  $('path').disabled = busy || !savePaths.length;
}

function adopt(snapshot) {
  save = snapshot;
  draft = { fields: {...save.fields}, equipped: [...save.equipped], warehouse: [...save.warehouse], relics: save.relics.map(r => ({...r})), deck: save.deck.map(row => ({...row})) };
  updateSaveOptions(save.path);
  selectedEquipped.clear(); selectedWarehouse.clear(); selectedRelics.clear(); selectedCandidates.clear(); selectedDeck.clear();
  deckEdit = null;
  pending = undefined;
  render();
}

function itemFor(id, kind = 'lingyong') {
  return catalogs[kind][id] || {id, name: `未知物品 · ID ${id}`, rar: '未知', desc: '当前 data.json 没有对应数据。保留原记录，不会自动删除。', tags:[], derived:false};
}

function portrait(item, kind, size = '') {
  return `<span class="portrait ${size}"><img src="/icons/${kind}/${item.id}.png?v=${esc(boot.dataVersion)}" alt="${esc(item.name)}" loading="lazy"><span class="fallback" hidden>俑</span></span>`;
}

function fixImages() {
  for (const img of document.querySelectorAll('img')) {
    const fallback = img.nextElementSibling;
    if (!fallback || !fallback.classList.contains('fallback')) continue;
    const failed = () => { img.hidden = true; fallback.hidden = false; };
    img.onerror = failed;
    if (img.complete && !img.naturalWidth) failed();
  }
}


function updateState() {
  if (!save) return;
  const dirty = isDirty(save, draft);
  $('dirty').textContent = dirty ? '有未保存修改' : '与读取时一致';
  $('dirty').className = `badge ${dirty ? 'amber' : 'neutral'}`;
  $('preview').disabled = false;
  $('save').disabled = boot.readOnly || !dirty;
  $('backups').disabled = false;
}

function render() {
  if (!save) return;
  updateState();
  document.querySelectorAll('[data-tab]').forEach(button => button.classList.toggle('active', button.dataset.tab === tab));
  $('content').className = `${tab}-mode`;
  if (tab === 'numbers') { renderNumbers(); return; }
  if (tab === 'deck') { renderDeck(); return; }
  const kind = tab === 'relics' ? 'relics' : 'lingyong';
  $('content').innerHTML = `
    <div class="page-title"><div class="title-line"><h2>${kind === 'lingyong' ? '灵俑编辑' : '持有遗物'}</h2>${kind === 'lingyong' ? '<div id="slot-info"></div>' : ''}<span class="page-inline-tip">${kind === 'lingyong' ? '多选上场灵俑删除或移动；右侧多选添加，不改变发现记录和商店解锁池。' : '左侧多选删除，右侧多选添加；现有遗物计数保留，供台槽位自动联动。'}</span></div></div>
    <div class="workspace"><div id="held"></div><section class="panel library">
      <div class="panel-heading"><h3>${kind === 'lingyong' ? '添加灵俑' : '添加遗物'}</h3>${kind === 'lingyong' ? '<span class="pool-scope">全部玩家灵俑（含衍生），不包含 BOSS 与角色技能</span>' : ''}<span id="result-count" class="muted"></span></div>
      <input id="search" class="search" aria-label="搜索物品" placeholder="搜索名称、ID、标签或效果描述" value="${esc(search)}">
      <div class="filters"><div id="rarities"></div><select id="tag" aria-label="标签筛选"><option value="">全部标签</option></select></div>
      ${kind === 'relics' ? '<div class="library-note"><span>未知的已有遗物会保留；不会模拟随机拾取事件。</span></div>' : ''}
      <div id="catalog-grid" class="catalog-grid"></div><div id="detail" class="detail"></div>
    </section></div>`;
  const rarities = [...new Set(boot.catalog[kind].map(i => i.rar).filter(Boolean))];
  $('rarities').innerHTML = ['', ...rarities].map(r => `<button class="chip ${rarity === r ? 'active' : ''}" data-rarity="${esc(r)}">${esc(r || '全部')}</button>`).join('');
  const tags = [...new Set(boot.catalog[kind].flatMap(i => i.tags))].sort((a,b) => a.localeCompare(b,'zh'));
  $('tag').innerHTML += tags.map(t => `<option value="${esc(t)}">${esc(t)}</option>`).join('');
  $('tag').value = tag;
  $('search').oninput = event => { search = event.target.value; renderLibrary(); };
  $('tag').onchange = event => { tag = event.target.value; renderLibrary(); };
  $('rarities').onclick = event => { const button = event.target.closest('[data-rarity]'); if (button) { rarity = button.dataset.rarity; render(); } };
  renderSlots(); renderHeld(); renderLibrary();
}

function renderSlots() {
  if (!$('slot-info')) return;
  const slots = slotState(save, draft, boot.slotRules);
  $('slot-info').innerHTML = `<div class="slot-stats" title="累计值包含供台加成，上场最多8个；仓库容量读取自存档。${slots.delta ? `供台变化联动${slots.delta>0?'+':''}${slots.delta}槽。` : ''}"><span class="badge">累计槽位 ${esc(slots.total)}</span><span class="badge">上场上限 ${esc(slots.limit)}</span><span class="badge">仓库容量 ${esc(slots.warehouse)}</span></div>`;
}

function slotCard(location, index, locked = false) {
  const id = draft[location][index], item = itemFor(id);
  const selected = (location === 'equipped' ? selectedEquipped : selectedWarehouse).has(index);
  if (locked) return `<div class="slot locked"><span class="slot-number">槽 ${index+1}</span><span class="lock-symbol">◇</span><span>未开放</span></div>`;
  if (id === undefined) return `<div class="slot empty-slot"><span class="slot-number">${location === 'equipped' ? '槽' : '仓'} ${index+1}</span><span class="add-symbol">＋</span><span>空闲位置</span></div>`;
  return `<label class="slot ${selected ? 'selected' : ''}">
    <span class="slot-number">${location === 'equipped' ? '槽' : '仓'} ${index+1}</span>
    <input type="checkbox" class="held-check" data-location="${location}" data-index="${index}" aria-label="选择${location === 'equipped' ? '上场' : '仓库'}灵俑 ${esc(item.name)} (第 ${index+1} 槽)" ${selected ? 'checked' : ''}>${item.derived ? '<span class="badge amber derived">衍生</span>' : ''}${portrait(item,'lingyong')}<strong class="held-name" title="ID ${id} ${esc(item.name)}"><span class="inline-id">ID ${id}</span> ${esc(item.name)}</strong>
  </label>`;
}

function syncHeldSelection() {
  document.querySelectorAll('#held [data-location]').forEach(input => {
    const selection = input.dataset.location === 'equipped' ? selectedEquipped : selectedWarehouse;
    input.checked = selection.has(Number(input.dataset.index));
    input.closest('.slot').classList.toggle('selected',input.checked);
  });
  document.querySelectorAll('#held [data-held-relic]').forEach(input => {
    input.checked = selectedRelics.has(Number(input.dataset.heldRelic));
    input.closest('.slot').classList.toggle('selected',input.checked);
  });
  document.querySelectorAll('#held [data-all]').forEach(button => {
    const location = button.dataset.all, selection = location === 'equipped' ? selectedEquipped : selectedWarehouse;
    button.textContent = selection.size === draft[location].length && draft[location].length ? '取消全选' : '全选';
  });
  document.querySelectorAll('#held [data-delete]').forEach(button => {
    const selection = button.dataset.delete === 'equipped' ? selectedEquipped : selectedWarehouse;
    button.textContent = `删除选中 (${selection.size})`; button.disabled = !selection.size;
  });
  document.querySelectorAll('#held [data-move]').forEach(button => {
    const selection = button.dataset.move === 'equipped' ? selectedEquipped : selectedWarehouse;
    button.textContent = `${button.dataset.move === 'equipped' ? '移入仓库' : '移至上场'} (${selection.size})`; button.disabled = !selection.size;
  });
  if ($('select-all-relics')) $('select-all-relics').textContent = selectedRelics.size === draft.relics.length && draft.relics.length ? '取消全选' : '全选';
  if ($('delete-relics')) { $('delete-relics').textContent = `删除选中 (${selectedRelics.size})`; $('delete-relics').disabled = !selectedRelics.size; }
}

function renderHeld() {
  const slots = slotState(save, draft, boot.slotRules);
  if (tab === 'relics') {
    $('held').innerHTML = `<section class="panel"><div class="panel-heading"><h3>持有遗物</h3><span class="badge">${draft.relics.length} 个</span><div class="panel-controls"><button id="delete-relics" class="small danger" ${!selectedRelics.size ? 'disabled' : ''}>删除选中 (${selectedRelics.size})</button><button id="select-all-relics" class="small select-all">${selectedRelics.size === draft.relics.length && draft.relics.length ? '取消全选' : '全选'}</button></div></div>
      <div class="held-relic-grid">${draft.relics.map((r,i) => {
        const item = itemFor(r.id,'relics');
        const selected = selectedRelics.has(i);
        return `<label class="slot relic-held ${selected ? 'selected' : ''}" title="ID ${r.id} ${esc(item.name)}；${r.ref === undefined ? '新增' : '原计数保留'}；${esc(item.desc)}"><span class="slot-number">${i+1}${r.ref === undefined ? ' · 新' : ''}</span><input type="checkbox" class="held-check" data-held-relic="${i}" aria-label="选择持有遗物 ${esc(item.name)} (第 ${i+1} 件)" ${selected ? 'checked' : ''}>${portrait(item,'relics')}<strong class="held-name"><span class="inline-id">ID ${r.id}</span> ${esc(item.name)}</strong></label>`;
      }).join('') || '<p class="empty-state">尚未持有遗物，可在右侧多选添加。</p>'}</div>
      <p class="hint">选中只改变界面；删除后供台槽位联动，确认保存才写入文件。</p></section>`;
    $('held').onclick = null;
    $('held').onchange = event => {
      const checkbox = event.target.closest('[data-held-relic]');
      if (checkbox) {
        const index = Number(checkbox.dataset.heldRelic);
        if (checkbox.checked) selectedRelics.add(index); else selectedRelics.delete(index);
        syncHeldSelection();
      }
    };
    $('select-all-relics').onclick = () => { if (selectedRelics.size === draft.relics.length) selectedRelics.clear(); else draft.relics.forEach((_,i) => selectedRelics.add(i)); syncHeldSelection(); };
    $('delete-relics').onclick = () => { draft.relics = draft.relics.filter((_,i) => !selectedRelics.has(i)); selectedRelics.clear(); render(); };
    return;
  }
  $('held').innerHTML = `<section class="panel"><div class="panel-heading"><h3>上场灵俑</h3><span class="badge ${draft.equipped.length > slots.limit ? 'amber' : ''}">${draft.equipped.length} / ${slots.limit}</span><div class="panel-controls"><button class="small" data-move="equipped" ${!selectedEquipped.size ? 'disabled' : ''}>移入仓库 (${selectedEquipped.size})</button><button data-delete="equipped" class="small danger" ${!selectedEquipped.size ? 'disabled' : ''}>删除选中 (${selectedEquipped.size})</button><button class="small select-all" data-all="equipped">${selectedEquipped.size === draft.equipped.length && draft.equipped.length ? '取消全选' : '全选'}</button></div></div>
    <div class="slots">${Array.from({length:8},(_,i) => slotCard('equipped',i,i >= slots.limit && i >= draft.equipped.length)).join('')}</div>
    <p class="hint">点击多选。删除先作用于界面，保存前会预览并确认。</p></section>
    <section class="panel warehouse"><div class="panel-heading"><h3>灵俑仓库</h3><span class="badge ${draft.warehouse.length > slots.warehouse ? 'amber' : ''}">${draft.warehouse.length} / ${slots.warehouse}</span><div class="panel-controls"><button class="small" data-move="warehouse" ${!selectedWarehouse.size ? 'disabled' : ''}>移至上场 (${selectedWarehouse.size})</button><button data-delete="warehouse" class="small danger" ${!selectedWarehouse.size ? 'disabled' : ''}>删除选中 (${selectedWarehouse.size})</button><button class="small select-all" data-all="warehouse">${selectedWarehouse.size === draft.warehouse.length && draft.warehouse.length ? '取消全选' : '全选'}</button></div></div>
    <div class="slots">${Array.from({length:Math.min(64,Math.max(slots.warehouse,draft.warehouse.length))},(_,i) => slotCard('warehouse',i)).join('') || '<p class="hint">仓库容量为 0。</p>'}</div>
    <p class="hint">默认 2 个，天赋可额外增加 2 个。读取当前存档容量，不重置天赋。</p></section>`;
  $('held').onchange = event => {
    const checkbox = event.target.closest('[data-location]');
    if (checkbox) {
      const selection = checkbox.dataset.location === 'equipped' ? selectedEquipped : selectedWarehouse;
      const index = Number(checkbox.dataset.index);
      if (checkbox.checked) selection.add(index); else selection.delete(index);
      syncHeldSelection();
    }
  };
  $('held').onclick = event => {
    const all = event.target.closest('[data-all]');
    if (all) {
      const location = all.dataset.all, selection = location === 'equipped' ? selectedEquipped : selectedWarehouse;
      if (selection.size === draft[location].length) selection.clear(); else draft[location].forEach((_,i) => selection.add(i));
      syncHeldSelection(); return;
    }
    const remove = event.target.closest('[data-delete]'), move = event.target.closest('[data-move]');
    if (!remove && !move) return;
    const location = (remove || move).dataset[remove ? 'delete' : 'move'];
    const selection = location === 'equipped' ? selectedEquipped : selectedWarehouse;
    if (!selection.size) return;
    if (move) {
      const destination = location === 'warehouse' ? 'equipped' : 'warehouse';
      const free = (destination === 'equipped' ? slots.limit : slots.warehouse) - draft[destination].length;
      if (!selectionFits(selection.size,free)) return toast(`已选 ${selection.size} 个，目标仅剩 ${Math.max(0,free)} 个空位，不能移动`);
      draft[destination].push(...draft[location].filter((_,i) => selection.has(i)));
    }
    draft[location] = draft[location].filter((_,i) => !selection.has(i));
    selection.clear(); render();
  };
}

function renderLibrary() {
  const kind = tab === 'relics' ? 'relics' : 'lingyong';
  for (const id of selectedCandidates) if (!catalogs[kind][id]) selectedCandidates.delete(id);
  const items = boot.catalog[kind].filter(item => matches(item,search,rarity,tag));
  $('result-count').textContent = `${items.length} 个候选`;
  $('catalog-grid').innerHTML = items.map(item => {
    const heldCount = kind === 'relics' ? draft.relics.filter(r => r.id === item.id).length : Number(draft.equipped.includes(item.id) || draft.warehouse.includes(item.id));
    const owned = kind === 'lingyong' && heldCount >= 1;
    const selected = selectedCandidates.has(item.id);
    return `<label class="catalog-card with-effect ${owned ? 'owned' : ''} ${selected ? 'selected' : ''}"><input type="checkbox" class="candidate-check" data-candidate="${item.id}" aria-label="选择 ${esc(item.name)} (ID ${item.id})" ${selected ? 'checked' : ''} ${owned ? 'disabled' : ''}>${portrait(item,kind)}<div><strong>${esc(item.name)}</strong><small class="rarity r${item.rarity}">ID ${item.id} · ${esc(item.rar)}${heldCount ? ` · 已持有 ${heldCount}` : ''}</small></div><span class="tags">${esc(item.tags.slice(0,4).join(' / '))}</span><p class="card-effect">${esc(item.desc)}</p></label>`;
  }).join('') || '<p class="empty-state">没有匹配的物品。</p>';
  $('catalog-grid').onchange = event => {
    const checkbox = event.target.closest('[data-candidate]');
    if (checkbox) {
      const id = Number(checkbox.dataset.candidate);
      if (checkbox.checked) selectedCandidates.add(id); else selectedCandidates.delete(id);
      const card = checkbox.closest('.catalog-card');
      card.classList.toggle('selected',checkbox.checked);
      renderDetail();
    }
  };
  renderDetail(); fixImages();
}

function renderDetail() {
  const kind = tab === 'relics' ? 'relics' : 'lingyong';
  if (kind === 'lingyong') {
    const slots = slotState(save,draft,boot.slotRules);
    const count = selectedCandidates.size, freeEquipped = Math.max(0,slots.limit-draft.equipped.length), freeWarehouse = Math.max(0,slots.warehouse-draft.warehouse.length);
    $('detail').innerHTML = `<div class="detail-title"><h3>已选 ${count} 个灵俑</h3><button id="clear-candidates" class="small" ${!count ? 'disabled' : ''}>清空选择</button></div><p class="selection-names">${esc([...selectedCandidates].map(id => itemFor(id).name).join('、') || '效果直接显示在卡片内，可跨搜索结果多选。')}</p><p class="hint">上场空余 ${freeEquipped} 槽 · 仓库空余 ${freeWarehouse} 槽。选中数量超出空位时，不能添加。</p><div class="detail-actions"><button id="add-equipped" class="primary" ${selectionFits(count,freeEquipped) ? '' : 'disabled'}>添加到上场 (${count})</button><button id="add-warehouse" ${selectionFits(count,freeWarehouse) ? '' : 'disabled'}>添加到仓库 (${count})</button></div>`;
    $('clear-candidates').onclick = () => { selectedCandidates.clear(); renderLibrary(); };
    const add = location => {
      const limit = location === 'equipped' ? slots.limit : slots.warehouse;
      if (!selectionFits(selectedCandidates.size,limit-draft[location].length)) return toast('所选数量超出空余槽位，不能添加');
      const ids = [...selectedCandidates];
      if (ids.some(id => draft.equipped.includes(id) || draft.warehouse.includes(id))) return toast('选中的灵俑包含已有物品，不能新增同 ID 副本');
      draft[location].push(...ids); selectedCandidates.clear(); render();
    };
    $('add-equipped').onclick = () => add('equipped');
    $('add-warehouse').onclick = () => add('warehouse');
    return;
  }
  const count = selectedCandidates.size;
  $('detail').innerHTML = `<div class="detail-title"><h3>已选 ${count} 种遗物</h3><button id="clear-candidates" class="small" ${!count ? 'disabled' : ''}>清空选择</button></div><p class="selection-names">${esc([...selectedCandidates].map(id => itemFor(id,'relics').name).join('、') || '效果直接显示在卡片内，可跨搜索结果多选。')}</p><p class="hint">每次为每种选中遗物添加 1 件，可重复添加；不限制同种遗物持有份数，不执行随机拾取事件。</p><div class="detail-actions"><button id="add-relics" class="primary" ${count ? '' : 'disabled'}>添加选中遗物 (${count})</button></div>`;
  $('clear-candidates').onclick = () => { selectedCandidates.clear(); renderLibrary(); };
  $('add-relics').onclick = () => {
    const ids = [...selectedCandidates];
    draft.relics.push(...ids.map(id => ({id}))); selectedCandidates.clear(); render();
  };
}

function deckChangedCount() {
  return draft.deck.reduce((count,row,index) => {
    const old = save.deck[index];
    return count + (row.score !== old.score || row.baoId !== old.baoId || row.lingId !== old.lingId ? 1 : 0);
  },0);
}

function effectName(kind,id) {
  if (!id) return '—';
  return catalogs[kind][id]?.name || `未知 ID ${id}`;
}

function resetDeckEdit() {
  const row = selectedDeck.size === 1 ? draft.deck.find(item => selectedDeck.has(item.key)) : undefined;
  deckEdit = {
    score: null,
    lingId: row?.lingId ?? null,
    baoId: row?.baoId ?? null,
  };
}

function deckCard(row) {
  const selected=selectedDeck.has(row.key);
  const modified=Number(row.score)!==0 || row.lingId!==0 || row.baoId!==0;
  const lingName=effectName('pailing',row.lingId), baoName=effectName('baopai',row.baoId);
  const lingClass=warningEffect('pailing',lingName)?' warning':'', baoClass=warningEffect('baopai',baoName)?' warning':'';
  return `<label class="deck-card ${selected?'selected':''} ${modified?'modified':''}" title="${esc(row.name)}：${esc(lingName)} / ${esc(baoName)}">
    <input type="checkbox" class="deck-check" data-deck-key="${esc(row.key)}" aria-label="选择牌面 ${esc(row.name)}" ${selected?'checked':''}>
    <img class="deck-face" src="/paimian/${esc(row.asset)}" alt="${esc(row.name)}">
    <span class="deck-summary${lingClass}">${esc(lingName)}</span>
    <span class="deck-summary${baoClass}">${esc(baoName)}</span>
  </label>`;
}

function deckEffectCard(kind,id) {
  if (!id) return '';
  const item=catalogs[kind][id] || {id,name:`未知 ID ${id}`,desc:'当前图鉴没有对应数据，将保留存档中的 ID。'};
  return `<div class="deck-effect-card">${portrait(item,kind)}<div><strong>${esc(item.name)} · ID ${id}</strong><p>${esc(item.desc)}</p></div></div>`;
}

function deckEffectField(kind, id, label, controlId, disabled) {
  const item = catalogs[kind][id];
  const unknown = id && !item ? {id, name: `未知${label}`} : null;
  const items = unknown ? [unknown, ...boot.catalog[kind]] : boot.catalog[kind];
  const selected = id === null ? '保持不变' : id === 0 ? '清空' : `${(item || unknown).name} · ID ${id}`;
  const choice = value => `type="button" data-effect-value="${value ?? ''}" aria-pressed="${id === value}" ${id === value ? 'autofocus' : ''}`;
  return `<div class="deck-field">
    <div class="deck-field-title" id="${controlId}-label"><strong>${label}</strong></div>
    <button type="button" class="deck-effect-trigger" id="${controlId}" popovertarget="${controlId}-picker" aria-labelledby="${controlId}-label ${controlId}-value" ${disabled ? 'disabled' : ''}>
      <span id="${controlId}-value">${esc(selected)}</span><span aria-hidden="true">⌄</span>
    </button>
    <div id="${controlId}-picker" class="deck-effect-picker" popover role="group" aria-labelledby="${controlId}-label">
      <div class="deck-effect-actions"><button ${choice(0)}>清空</button><button ${choice(null)}>保持不变</button></div>
      <div class="deck-effect-choices">${items.map(option => `
        <button ${choice(option.id)} aria-label="${esc(option.name)} · ID ${option.id}">
          ${option === unknown ? '' : portrait(option,kind)}
          <span>${esc(option.name)}</span><small>ID ${option.id}</small>
        </button>`).join('')}</div>
    </div>
    ${deckEffectCard(kind,id)}
  </div>`;
}

function closeDeckEffectPicker() {
  document.querySelector('.deck-effect-picker:popover-open')?.hidePopover();
}

function renderDeck() {
  if (!deckEdit) resetDeckEdit();
  const selected = draft.deck.filter(row => selectedDeck.has(row.key));
  const initialScore = selected[0]?.score ?? '';
  const sharedScore = selected.every(row => row.score === initialScore);
  const scoreValue = deckEdit.score ?? (sharedScore ? initialScore : '');
  const scorePlaceholder = deckEdit.score === null && !sharedScore ? '原值不同' : '0';
  const disabled = selected.length ? '' : 'disabled';
  const byGroup=group=>draft.deck.filter(row=>row.group===group);
  const honors=byGroup('字');
  const honorCells=[...honors.slice(0,4),null,...honors.slice(4),null];
  const deckRow=rows=>rows.map(row=>row?deckCard(row):'<span class="deck-gap" aria-hidden="true"></span>').join('');
  const quickGroups=Object.fromEntries(['wan','tong','suo','zi','odd','even'].map(group=>[group,deckKeysForGroup(draft.deck,group)]));
  const quickButton=(key,label)=>`<button class="small deck-quick ${quickGroups[key].every(id=>selectedDeck.has(id))?'active':''}" data-deck-quick="${key}">${label}</button>`;
  $('content').innerHTML=`<div class="page-title"><div class="title-line"><h2>牌库编辑</h2><span class="badge">34 种牌面</span><span class="badge">存档已有 ${save.deckRows} 条</span><span class="badge ${deckChangedCount()?'amber':''}">待修改 ${deckChangedCount()} 种</span><span class="page-inline-tip">保存牌库修改时，固定写入 34 条牌面记录。</span></div></div>
  <div class="deck-workspace">
    <section class="panel deck-pool"><div class="panel-heading"><h3>选择牌面</h3><span class="muted">已选 ${selected.length} / 34</span><div class="deck-quick-actions">${quickButton('wan','万')}${quickButton('tong','筒')}${quickButton('suo','索')}${quickButton('zi','字')}${quickButton('odd','奇')}${quickButton('even','偶')}<button id="deck-select-all" class="small">${selected.length===34?'取消全选':'全选'}</button></div></div>
      <div class="deck-grid">${deckRow(byGroup('万'))}${deckRow(byGroup('筒'))}${deckRow(byGroup('索'))}${deckRow(honorCells)}</div>
    </section>
    <section class="panel deck-editor"><div class="panel-heading"><h3>编辑选中牌面</h3><span class="badge">${selected.length} 种</span></div>
      ${selected.length?`<div class="deck-selected-names">${esc(selected.map(row=>row.name).join('、'))}</div>`:'<div class="deck-editor-empty">请在左侧选择牌面</div>'}
      <div class="deck-field"><label class="deck-field-title" for="deck-score"><strong>牌面分加成</strong></label>
        <div class="deck-score-controls"><button id="deck-score-minus" ${disabled}>-10</button><input id="deck-score" type="text" inputmode="decimal" value="${esc(scoreValue)}" placeholder="${scorePlaceholder}" ${disabled}><button id="deck-score-plus" ${disabled}>+10</button></div></div>
      ${deckEffectField('pailing', deckEdit.lingId, '牌灵', 'deck-ling', !selected.length)}
      ${deckEffectField('baopai', deckEdit.baoId, '宝牌', 'deck-bao', !selected.length)}
      <div class="deck-actions"><button id="deck-apply" class="primary" ${selected.length?'':'disabled'}>应用到已选 ${selected.length} 种</button><button id="deck-restore">恢复读取值</button></div>
      <p class="hint">牌面分未编辑时保留原值；手动删空后应用为 0。牌灵、宝牌可选「保持不变」。</p>
    </section>
  </div>`;
  $('content').onchange=event=>{
    const check=event.target.closest('[data-deck-key]');
    if(check){if(check.checked)selectedDeck.add(check.dataset.deckKey);else selectedDeck.delete(check.dataset.deckKey);resetDeckEdit();renderDeck();return;}
  };
  const editor = document.querySelector('.deck-editor');
  editor.onscroll = closeDeckEffectPicker;
  for (const [controlId, field] of [['deck-ling', 'lingId'], ['deck-bao', 'baoId']]) {
    const trigger = $(controlId);
    const picker = $(`${controlId}-picker`);
    picker.onbeforetoggle = event => {
      if (event.newState !== 'open') return;
      const {left, top, bottom, width} = trigger.getBoundingClientRect();
      const below = innerHeight - bottom - 12, above = top - 12;
      const openAbove = below < 320 && above > below;
      Object.assign(picker.style, {
        left: `${left}px`,
        width: `${width}px`,
        top: openAbove ? 'auto' : `${bottom + 4}px`,
        bottom: openAbove ? `${innerHeight - top + 4}px` : 'auto',
        maxHeight: `${Math.min(320, openAbove ? above : below)}px`,
      });
    };
    picker.onclick = event => {
      const option = event.target.closest('[data-effect-value]');
      if (!option) return;
      const value = option.dataset.effectValue;
      deckEdit[field] = value === '' ? null : Number(value);
      const scrollTop = editor.scrollTop;
      picker.hidePopover();
      renderDeck();
      document.querySelector('.deck-editor').scrollTop = scrollTop;
      $(controlId).focus({preventScroll: true});
    };
  }
  $('deck-select-all').onclick=()=>{if(selectedDeck.size===34)selectedDeck.clear();else draft.deck.forEach(row=>selectedDeck.add(row.key));resetDeckEdit();renderDeck();};
  document.querySelectorAll('[data-deck-quick]').forEach(button=>{button.onclick=()=>{toggleDeckKeys(selectedDeck,quickGroups[button.dataset.deckQuick]);resetDeckEdit();renderDeck();};});
  const scoreInput = $('deck-score');
  scoreInput.oninput = () => { deckEdit.score = scoreInput.value; };
  $('deck-score-minus').onclick = () => { deckEdit.score = scoreInput.value = String((Number(scoreInput.value) || 0) - 10); };
  $('deck-score-plus').onclick = () => { deckEdit.score = scoreInput.value = String((Number(scoreInput.value) || 0) + 10); };
  $('deck-restore').onclick=()=>{draft.deck=save.deck.map(row=>({...row}));selectedDeck.clear();deckEdit=null;render();};
  $('deck-apply').onclick=()=>{
    if(!selectedDeck.size)return;
    const score = deckEdit.score?.trim();
    if (score && !/^[+-]?\d+(?:\.\d+)?(?:e[+-]?\d+)?$/i.test(score)) return toast('请输入有效的牌面分');
    for (const row of selected) {
      if (score !== undefined) row.score = score || '0.0';
      if (deckEdit.lingId !== null) row.lingId = deckEdit.lingId;
      if (deckEdit.baoId !== null) row.baoId = deckEdit.baoId;
    }
    updateState();resetDeckEdit();renderDeck();
  };
  fixImages();
}

function renderNumbers() {
  const groups = [
    ['金币与血量', ['Coin','Hp','HpMax','AccHpLost']],
    ['麻将对局', ['InitSwapPaiTotal','SwapPaiTurnTotal','HuPaiSlotsCount']],
    ['灵俑容量', ['LingYongSlotNum','XiaoChouRepositorySlot']],
    ['局外资源', ['AccountGameData.currency.lowTokenCount','AccountGameData.currency.premiumTokenCount']],
  ];
  $('content').innerHTML = `<div class="page-title"><div class="title-line"><h2>数值编辑</h2><span class="page-inline-tip">显示的是存档原始值。游戏可能因技能、遗物或 GameConfig 在运行时另行计算最终数值。</span></div></div>
    <div class="number-sections">${groups.map(([title,keys]) => `<section class="panel number-panel"><div class="panel-heading"><h3>${title}</h3></div><div class="number-grid">${keys.map(key => {
      const field = boot.fields.find(f => f.key === key);
      const current = save.fields[key];
      const available = current !== undefined;
      const caption = available ? `读取值 ${current}` : '存档中无有效字段';
      return `<label class="number-field" title="${esc(key)}；${esc(caption)}"><strong>${esc(field.label)}</strong><input data-field="${key}" aria-label="${esc(field.label)}" type="text" inputmode="${field.integer ? 'numeric' : 'decimal'}" value="${esc(draft.fields[key])}" ${available ? '' : 'disabled'}><small>${esc(caption)}</small></label>`;
    }).join('')}</div></section>`).join('')}</div>
    <button id="reset-numbers">撤销数值页修改</button>`;
  for (const input of document.querySelectorAll('[data-field]')) input.oninput = () => { draft.fields[input.dataset.field] = input.value; updateState(); };
  $('reset-numbers').onclick = () => { draft.fields = {...save.fields}; render(); };
}

async function showPreview() {
  if (!save) throw new Error('请先加载存档');
  const request = requestFor(save,draft);
  const preview = await call('Preview',request);
  pending = {...request,previewHash:preview.previewHash};
  $('dialog-content').innerHTML = `<div class="panel-heading"><h3>保存前预览</h3><button id="close-dialog">关闭</button></div>
    <p class="hint">${esc(save.path)}</p>${preview.changes.length ? `<table><thead><tr><th>修改项</th><th>原值</th><th>新值</th></tr></thead><tbody>${preview.changes.map(c => `<tr><th>${esc(c.label)}</th><td>${esc(c.before)}</td><td>${esc(c.after)}</td></tr>`).join('')}</tbody></table>` : '<p class="empty-state">没有需要保存的变化。</p>'}
    ${preview.warnings.map(w => `<p class="notice">${esc(w)}</p>`).join('')}
    <p class="hint">保存会备份当前文件，游戏运行或文件发生变化时拒绝写入。发现记录、解锁列表和其他非目标字段保持原文。</p>
    ${boot.readOnly ? '<p class="notice">这是浏览器／只读预览，保存被禁用。</p>' : ''}
    <label class="confirmation"><input type="checkbox" id="confirmed"> 我已核对修改内容，确认写入此存档</label>
    <div class="dialog-actions"><button id="cancel-save">取消</button><button id="confirm-save" class="primary" disabled>确认保存并备份</button></div>`;
  $('close-dialog').onclick = $('cancel-save').onclick = () => $('dialog').close();
  $('confirmed').onchange = () => { $('confirm-save').disabled = boot.readOnly || !preview.changes.length || !$('confirmed').checked; };
  $('confirm-save').onclick = () => action(async () => {
    const result = await call('Save',pending);
    $('dialog').close(); adopt(result.save);
    toast(`保存成功。备份：${result.backup}`,false);
  });
  $('dialog').showModal();
}

async function showBackups() {
  if (!save) throw new Error('请先加载存档');
  const paths = await call('Backups',save.path);
  $('dialog-content').innerHTML = `<div class="panel-heading"><h3>恢复备份</h3><button id="close-dialog">关闭</button></div><p class="notice">恢复会整份覆盖此存档（包括其他字段），并先备份当前文件。只显示本修改器创建的备份。</p><select id="backup-choice" aria-label="备份文件">${paths.map(p => `<option value="${esc(p)}">${esc(p.split(/[\\/]/).pop())}</option>`).join('')}</select><p class="hint">${paths.length ? '选择备份并确认。界面中未保存的修改会被丢弃。' : '目前还没有由本修改器生成的备份。'}</p><label class="confirmation"><input type="checkbox" id="restore-confirmed"> 确认整份恢复当前存档</label><div class="dialog-actions"><button id="restore" class="primary" disabled>恢复并备份当前文件</button></div>`;
  $('close-dialog').onclick = () => $('dialog').close();
  $('restore-confirmed').onchange = () => { $('restore').disabled = boot.readOnly || !paths.length || !$('restore-confirmed').checked; };
  $('restore').onclick = () => action(async () => {
    const result = await call('Restore',save.path,save.hash,$('backup-choice').value);
    $('dialog').close(); adopt(result.save); toast(`已恢复。恢复前备份：${result.backup}`,false);
  });
  $('dialog').showModal();
}

async function loadPath(path = $('path').value) {
  if (save && isDirty(save,draft) && !confirm('切换或重新加载会丢弃界面中未保存的修改，是否继续？')) {
    $('path').value = save.path;
    $('path').title = save.path;
    return;
  }
  try {
    adopt(await call('Load',path));
  } catch (error) {
    if (save) { $('path').value = save.path; $('path').title = save.path; }
    throw error;
  }
}

document.querySelectorAll('[data-tab]').forEach(button => { button.onclick = () => { tab = button.dataset.tab; search = ''; rarity = ''; tag = ''; selectedEquipped.clear(); selectedWarehouse.clear(); selectedRelics.clear(); selectedCandidates.clear(); selectedDeck.clear(); deckEdit = null; render(); }; });
$('load').onclick = () => action(loadPath);
$('path').onchange = () => action(loadPath);
$('choose').onclick = () => action(async () => {
  const path = await call('ChooseSave'); if (path) await loadPath(path);
});
$('preview').onclick = $('save').onclick = () => action(showPreview);
$('backups').onclick = () => action(showBackups);
$('refresh-data').onclick = () => action(async () => {
  boot = await call('RefreshCatalog');
  for (const kind of ['lingyong','relics','pailing','baopai']) catalogs[kind] = Object.fromEntries(boot.catalog[kind].map(item => [item.id,item]));
  $('source').textContent = `${boot.source} · ${boot.catalog.lingyong.length} 灵俑 / ${boot.catalog.relics.length} 遗物 / ${boot.catalog.pailing.length} 牌灵 / ${boot.catalog.baopai.length} 宝牌`;
  if (save) render();
  toast(`图鉴状态：${boot.source}。界面中的存档修改已保留。`,false);
});
window.addEventListener('beforeunload',event => { if (save && isDirty(save,draft)) { event.preventDefault(); event.returnValue = ''; } });
window.addEventListener('resize', closeDeckEffectPicker);

await action(async () => {
  if (!browserPreview) runtime = (await import('/wails/runtime.js')).Call;
  boot = await call('Bootstrap');
  for (const kind of ['lingyong','relics','pailing','baopai']) catalogs[kind] = Object.fromEntries(boot.catalog[kind].map(item => [item.id,item]));
  $('source').textContent = `${boot.source} · ${boot.catalog.lingyong.length} 灵俑 / ${boot.catalog.relics.length} 遗物 / ${boot.catalog.pailing.length} 牌灵 / ${boot.catalog.baopai.length} 宝牌`;
  $('source').title = boot.dataPath;
  savePaths = [...boot.saves];
  updateSaveOptions();
  $('choose').disabled = browserPreview;
  if (browserPreview) $('choose').title = '手动选择文件请使用桌面版；预览只显示已检测到的存档';
  if (boot.readOnly || boot.running) {
    $('status').textContent = boot.readOnly ? '只读预览模式：可以模拟编辑与查看差异，但不能保存或恢复存档。' : '检测到游戏正在运行：可以读取和预览，必须退出游戏后才能保存。';
    $('status').hidden = false;
  }
  if (savePaths.length) await loadPath();
});
