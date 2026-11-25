// Activity log: the shown events, newest first, grouped by day.
let logSig = '';
function renderLog() {
  const d = state.data, evs = d ? shownEvents(d) : [];
  const n = Math.min(evs.length, state.logN);
  const sig = d ? [d.name, d.range, evs.length, evs[evs.length - 1]?.t, n, [...state.hidden]].join('|') : '';
  if (sig === logSig) return;
  logSig = sig;
  text($('log-n'), !d ? '' : evs.length ? (d.events_truncated ? 'Latest ' : '') + fmt(evs.length) + (evs.length === 1 ? ' event' : ' events') : '');
  const out = [], today = dfLong.format(new Date());
  let day = '';
  for (let i = evs.length - 1; i >= evs.length - n; i--) {
    const e = evs[i], when = new Date(e.t), label = dfLong.format(when);
    if (label !== day) { day = label; out.push(el('li', { className: 'day', textContent: label === today ? 'Today' : label })); }
    const time = el('time', { className: 'num', textContent: dfTime.format(when) });
    time.dateTime = when.toISOString();
    out.push(el('li', { className: 'ev' }, time, tag(e.type), el('span', { className: 'm', textContent: e.msg })));
  }
  if (d && !evs.length) out.push(el('li', { className: 'empty', textContent: 'No events ' + RANGE_LONG[d.range].replace('all time', 'yet') }));
  $('log').replaceChildren(...out);
  const more = $('more'), left = evs.length - n;
  more.hidden = left <= 0;
  if (left > 0) more.textContent = 'Show ' + fmt(Math.min(left, LOG_PAGE * 2)) + ' more';
}
$('more').onclick = () => { state.logN += LOG_PAGE * 2; renderLog(); };
