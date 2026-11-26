export function slotState(save, draft, rules) {
  const count = rows => rows.reduce((m, r) => { m[r.id] = (m[r.id] || 0) + 1; return m; }, {});
  const before = count(save.relics), after = count(draft.relics);
  const delta = [...new Set([...Object.keys(before), ...Object.keys(after)])]
    .reduce((n, id) => n + (rules[id] || 0) * ((after[id] || 0) - (before[id] || 0)), 0);
  const total = Number(draft.fields.LingYongSlotNum) + delta;
  return { total, limit: Math.max(0, Math.min(total || 0, 8)), warehouse: Number(draft.fields.XiaoChouRepositorySlot), delta };
}

export function requestFor(save, draft) {
  const request = {
    path: save.path, hash: save.hash,
    fields: Object.fromEntries(Object.entries(draft.fields).filter(([key, value]) => value !== save.fields[key])),
    equipped: [...draft.equipped], warehouse: [...draft.warehouse],
    relics: draft.relics.map(r => r.ref === undefined ? { id: r.id } : { ref: r.ref }),
  };
  const deck = (draft.deck || []).map(({key,score,baoId,lingId}) => ({key,score,baoId,lingId}));
  const original = (save.deck || []).map(({key,score,baoId,lingId}) => ({key,score,baoId,lingId}));
  if (JSON.stringify(deck) !== JSON.stringify(original)) request.deck = deck;
  return request;
}

export function matches(item, search, rarity, tag) {
  return (!rarity || item.rar === rarity) &&
    (!tag || item.tags.includes(tag)) &&
    `${item.name} ${item.en} ${item.id} ${item.desc} ${item.tags.join(' ')}`.toLowerCase().includes(search.trim().toLowerCase());
}

export function selectionFits(count, free) {
  return Number.isInteger(count) && count > 0 && count <= free;
}

export function deckKeysForGroup(deck, group) {
  if (group === 'wan') return deck.filter(row => row.group === '万').map(row => row.key);
  if (group === 'tong') return deck.filter(row => row.group === '筒').map(row => row.key);
  if (group === 'suo') return deck.filter(row => row.group === '索').map(row => row.key);
  if (group === 'zi') return deck.filter(row => row.group === '字').map(row => row.key);
  const parity = group === 'odd' ? 1 : group === 'even' ? 0 : -1;
  if (parity < 0) return [];
  return deck.filter(row => row.group !== '字' && Number(row.key.match(/\d+$/)?.[0]) % 2 === parity).map(row => row.key);
}

export function toggleDeckKeys(selection, keys) {
  const remove = keys.every(key => selection.has(key));
  for (const key of keys) {
    if (remove) selection.delete(key); else selection.add(key);
  }
}

export function warningEffect(kind, name) {
  if (!name || name === '—') return false;
  if (kind === 'pailing') return name.endsWith('念');
  if (kind === 'baopai') return ['迷幻', '毒', '雾'].some(part => name.includes(part));
  return false;
}

export function isDirty(save, draft) {
  const req = requestFor(save, draft);
  return Object.keys(req.fields).length > 0 || JSON.stringify(save.equipped) !== JSON.stringify(draft.equipped) ||
    JSON.stringify(save.warehouse) !== JSON.stringify(draft.warehouse) ||
    JSON.stringify(save.relics.map(r => ({ ref: r.ref }))) !== JSON.stringify(req.relics) || req.deck !== undefined;
}

export function saveLabel(path) {
  const name = path.split(/[\\/]/).pop();
  const match = /^Slot(\d+)\.json$/i.exec(name);
  const account = /[\\/]userdata[\\/](\d+)[\\/]3444020[\\/]/i.exec(path);
  const slot = match ? `${Number(match[1]) + 1}号存档` : name;
  return `${account ? account[1] : '外部文件'} · ${slot} · ${path}`;
}
