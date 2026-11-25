// Legend under the chart; its toggles decide which events are shown.
function renderLegend() {
  const counts = {};
  for (const e of state.data?.events || []) if (EV[e.type]) counts[e.type] = (counts[e.type] || 0) + 1;
  $('legend').replaceChildren(...['pos', 'neg', 'neu'].map(tone => {
    const types = Object.keys(EV).filter(t => counts[t] && EV[t].tone === tone);
    if (!types.length) return null;
    const row = el('div', { className: 'lg' }, el('span', { className: 'k lbl', textContent: TONE_LABEL[tone] }));
    row.setAttribute('role', 'group');
    row.setAttribute('aria-label', TONE_LABEL[tone]);
    for (const t of types) {
      const b = el('button', { type: 'button' }, tag(t), el('span', { className: 'num', textContent: fmt(counts[t]) }));
      b.setAttribute('aria-pressed', !state.hidden.has(t));
      b.onclick = () => {
        state.hidden.has(t) ? state.hidden.delete(t) : state.hidden.add(t);
        store.set('hidden', JSON.stringify([...state.hidden]));
        b.setAttribute('aria-pressed', !state.hidden.has(t));
        renderLog(); draw();
      };
      row.append(b);
    }
    return row;
  }).filter(Boolean));
}
const shownEvents = d => d.events.filter(e => EV[e.type] && !state.hidden.has(e.type) && e.t >= d.from);
