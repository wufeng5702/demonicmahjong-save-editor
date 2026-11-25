export function slotState(save, draft, rules) {
  const count = rows => rows.reduce((m, r) => { m[r.id] = (m[r.id] || 0) + 1; return m; }, {});
  const before = count(save.relics), after = count(draft.relics);
  const delta = [...new Set([...Object.keys(before), ...Object.keys(after)])]
    .reduce((n, id) => n + (rules[id] || 0) * ((after[id] || 0) - (before[id] || 0)), 0);
  const total = Number(draft.fields.LingYongSlotNum) + delta;
  return { total, limit: Math.max(0, Math.min(total || 0, 8)), warehouse: Number(draft.fields.XiaoChouRepositorySlot), delta };
}

export function requestFor(save, draft) {
  return {
    path: save.path, hash: save.hash,
    fields: Object.fromEntries(Object.entries(draft.fields).filter(([key, value]) => value !== save.fields[key])),
    equipped: [...draft.equipped], warehouse: [...draft.warehouse],
    relics: draft.relics.map(r => r.ref === undefined ? { id: r.id } : { ref: r.ref }),
  };
}

export function matches(item, search, rarity, tag) {
  return (!rarity || item.rar === rarity) &&
    (!tag || item.tags.includes(tag)) &&
    `${item.name} ${item.en} ${item.id} ${item.desc} ${item.tags.join(' ')}`.toLowerCase().includes(search.trim().toLowerCase());
}

export function selectionFits(count, free) {
  return Number.isInteger(count) && count > 0 && count <= free;
}

export function isDirty(save, draft) {
  const req = requestFor(save, draft);
  return Object.keys(req.fields).length > 0 || JSON.stringify(save.equipped) !== JSON.stringify(draft.equipped) ||
    JSON.stringify(save.warehouse) !== JSON.stringify(draft.warehouse) ||
    JSON.stringify(save.relics.map(r => ({ ref: r.ref }))) !== JSON.stringify(req.relics);
}

export function saveLabel(path) {
  const name = path.split(/[\\/]/).pop();
  const match = /^Slot(\d+)\.json$/i.exec(name);
  const account = /[\\/]userdata[\\/](\d+)[\\/]3444020[\\/]/i.exec(path);
  const slot = match ? `${Number(match[1]) + 1}号存档` : name;
  return `${account ? account[1] : '外部文件'} · ${slot} · ${path}`;
}
