// Loading streamers and their history from the API.
async function getJSON(url) {
  const r = await fetch(url, { cache: 'no-store' });
  if (!r.ok) throw new Error(r.status);
  return r.json();
}
// Responses are kept per streamer and range, so going back to a view paints at once
// and the network only confirms it. Hovering a row or a range warms the entry.
const cache = new Map();
const key = (name, range) => name + '|' + range;
function load(name, range, maxAge) {
  const k = key(name, range), c = cache.get(k);
  if (c?.p) return c.p;
  if (c?.d && Date.now() - c.at < maxAge) return Promise.resolve(c.d);
  const p = getJSON('api/streamer/' + encodeURIComponent(name) + '?range=' + range).then(d => {
    cache.delete(k); cache.set(k, { d, at: Date.now() });
    if (cache.size > 48) cache.delete(cache.keys().next().value);
    return d;
  }, e => {
    c?.d ? cache.set(k, { d: c.d, at: c.at }) : cache.delete(k);
    throw e;
  });
  cache.set(k, { d: c?.d, at: c?.at || 0, p });
  return p;
}
const warm = (name, range) => { if (name) load(name, range, PREFETCH_MAX_AGE).catch(() => {}); };
async function loadList() {
  const d = await getJSON('api/streamers');
  state.streamers = d.streamers; state.totals = d.totals;
  if (!state.sel || !d.streamers.some(s => s.name === state.sel)) state.sel = sorted()[0]?.name || null;
  renderSide(); renderHead();
}
async function show(maxAge) {
  const name = state.sel, range = state.range;
  if (!name) { setData(null); return; }
  const c = cache.get(key(name, range));
  if (c?.d) setData(c.d); else { renderHead(); $('plot').classList.add('loading'); }
  try {
    const d = await load(name, range, maxAge);
    if (name === state.sel && range === state.range) setData(d);
  } finally {
    if (name === state.sel && range === state.range) $('plot').classList.remove('loading');
  }
}
function setData(d) {
  if (state.data === d) return;
  state.data = d;
  renderHead(); renderLegend(); renderLog(); draw();
}
async function refresh() {
  try {
    // List and chart are fetched together; a stale selection is corrected by the list.
    await Promise.all([loadList(), state.sel ? load(state.sel, state.range, 0).catch(() => {}) : null]);
    await show(REFRESH_MS / 2);
    text($('status'), 'Updated ' + dfClock.format(new Date()));
  } catch {
    text($('status'), 'Connection lost, retrying');
  }
}
