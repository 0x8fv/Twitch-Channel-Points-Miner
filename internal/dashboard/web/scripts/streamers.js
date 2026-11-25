// Sidebar: totals, filter, sort and the streamer list.
function sorted() {
  const by = { points: (a, b) => b.points - a.points, gain: (a, b) => b.session_gain - a.session_gain, name: () => 0 }[state.sort];
  const q = state.q;
  return state.streamers.filter(s => !q || s.name.includes(q)).sort((a, b) => by(a, b) || a.name.localeCompare(b.name));
}
const list = $('list'), rows = new Map();
const emptyRow = el('li', { className: 'empty' });
function renderSide() {
  const t = state.totals;
  if (t) {
    text($('t-points'), fmt(t.points));
    text($('t-gain'), signed(t.session_gain)); $('t-gain').className = 'num ' + cls(t.session_gain);
    text($('t-online'), t.online + ' of ' + t.count);
  }
  for (const b of document.querySelectorAll('[data-sort]')) b.setAttribute('aria-pressed', b.dataset.sort === state.sort);
  const items = sorted(), gain = state.sort === 'gain';
  let nodes;
  if (!items.length) {
    text(emptyRow, state.streamers.length ? 'No streamer matches' : 'No streamers yet, they appear once the miner has loaded them');
    nodes = [emptyRow];
  } else nodes = items.map(s => {
    let r = rows.get(s.name);
    if (!r) {
      const val = el('span', { className: 'val num' });
      const btn = el('button', { type: 'button' }, el('span', { className: 'dot' }), el('span', { className: 'nm', textContent: s.name }), val);
      btn.dataset.name = s.name;
      rows.set(s.name, r = { li: el('li', null, btn), btn, val });
    }
    text(r.val, gain ? signed(s.session_gain) : fmt(s.points));
    r.val.className = 'val num ' + (gain ? cls(s.session_gain) : '');
    r.btn.classList.toggle('on', s.online);
    r.btn.title = s.online ? 'Online' : 'Offline';
    r.btn.setAttribute('aria-current', s.name === state.sel);
    return r.li;
  });
  // Rows are reused; the DOM is only touched when the order really changed.
  const cur = list.children;
  let same = cur.length === nodes.length;
  for (let i = 0; same && i < nodes.length; i++) same = cur[i] === nodes[i];
  if (!same) list.replaceChildren(...nodes);
}
function select(name) {
  if (!name || name === state.sel) return;
  state.sel = name; state.hover = null; state.logN = LOG_PAGE;
  store.set('sel', name);
  history.replaceState(null, '', '#' + encodeURIComponent(name));
  renderSide();
  rows.get(name)?.btn.scrollIntoView({ block: 'nearest' });
  show(PREFETCH_MAX_AGE).catch(() => {});
}
function step(dir) {
  const items = sorted(), i = items.findIndex(s => s.name === state.sel);
  const next = items[Math.max(0, Math.min(items.length - 1, i < 0 ? 0 : i + dir))];
  if (next) select(next.name);
  return next;
}
list.onclick = e => { const b = e.target.closest('button[data-name]'); if (b) select(b.dataset.name); };
let warmTimer = 0;
list.onpointerover = e => {
  const b = e.target.closest('button[data-name]');
  clearTimeout(warmTimer);
  if (b) warmTimer = setTimeout(warm, 70, b.dataset.name, state.range);
};
list.onkeydown = e => {
  if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
  e.preventDefault();
  const next = step(e.key === 'ArrowDown' ? 1 : -1);
  if (next) rows.get(next.name).btn.focus();
};
const q = $('q');
q.oninput = () => { state.q = q.value.trim().toLowerCase(); renderSide(); };
q.onkeydown = e => {
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') { e.preventDefault(); step(e.key === 'ArrowDown' ? 1 : -1); }
  else if (e.key === 'Enter') { const f = sorted()[0]; if (f) select(f.name); }
  else if (e.key === 'Escape') { q.value = ''; q.oninput(); q.blur(); }
};
document.addEventListener('keydown', e => {
  if (e.key === '/' && !e.ctrlKey && !e.metaKey && !e.altKey && document.activeElement !== q) { e.preventDefault(); q.focus(); q.select(); }
});
for (const b of document.querySelectorAll('[data-sort]')) b.onclick = () => { state.sort = b.dataset.sort; store.set('sort', state.sort); renderSide(); };
for (const b of document.querySelectorAll('[data-range]')) {
  b.onclick = () => {
    if (state.range === b.dataset.range) return;
    state.range = b.dataset.range; store.set('range', state.range); state.hover = null; state.logN = LOG_PAGE;
    show(PREFETCH_MAX_AGE).catch(() => {});
  };
  b.onpointerenter = () => warm(state.sel, b.dataset.range);
}
