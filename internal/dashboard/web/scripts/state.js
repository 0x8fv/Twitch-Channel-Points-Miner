// What the page is showing, plus the choices remembered between visits.
const REFRESH_MS = 45000, PREFETCH_MAX_AGE = 15000, LOG_PAGE = 100;
const RANGE_LONG = { '24h': 'in the last 24 hours', '7d': 'in the last 7 days', '30d': 'in the last 30 days', all: 'all time' };
const RANGE_SHORT = { '24h': '24h', '7d': '7d', '30d': '30d', all: 'all time' };
const store = {
  get(k, d) { try { return localStorage.getItem('tcpm.' + k) ?? d; } catch { return d; } },
  set(k, v) { try { localStorage.setItem('tcpm.' + k, v); } catch {} },
};
function safeHash() { try { return decodeURIComponent(location.hash.slice(1)); } catch { return ''; } }
function safeSet(raw) { try { return new Set(JSON.parse(raw)); } catch { return new Set(); } }
const state = {
  streamers: [], totals: null, sel: safeHash() || store.get('sel', '') || null,
  range: store.get('range', '24h'), sort: store.get('sort', 'points'), q: '',
  hidden: safeSet(store.get('hidden', '[]')),
  data: null, hover: null, logN: LOG_PAGE,
};
if (!RANGE_LONG[state.range]) state.range = '24h';
if (!['points', 'gain', 'name'].includes(state.sort)) state.sort = 'points';
