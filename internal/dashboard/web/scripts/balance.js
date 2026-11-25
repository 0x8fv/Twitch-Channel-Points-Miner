// Header of the selected streamer: balance, change and range stats.
function renderHead() {
  for (const b of document.querySelectorAll('[data-range]')) b.setAttribute('aria-pressed', b.dataset.range === state.range);
  const s = state.streamers.find(x => x.name === state.sel), d = state.data;
  const live = s && d && d.name === state.sel && d.range === state.range && d.series.length > 0;
  const name = $('d-name');
  text(name, s ? s.name : 'No streamer selected');
  if (s) name.href = 'https://www.twitch.tv/' + encodeURIComponent(s.name); else name.removeAttribute('href');
  $('d-state').replaceChildren(...(s ? [el('span', { className: 'dot' + (s.online ? ' on' : '') }), s.online ? 'Online' : 'Offline'] : []));
  text($('d-points'), s ? fmt(s.points) : '–');
  const delta = $('d-delta');
  if (live) delta.replaceChildren(el('span', { className: 'num ' + cls(d.stats.gain), textContent: d.stats.gain ? signed(d.stats.gain) : 'No change' }), ' ' + RANGE_LONG[d.range]);
  else delta.replaceChildren();
  text($('s-low-l'), 'Low, ' + RANGE_SHORT[state.range]);
  text($('s-high-l'), 'High, ' + RANGE_SHORT[state.range]);
  text($('s-low'), live ? fmt(d.stats.min) : '–');
  text($('s-high'), live ? fmt(d.stats.max) : '–');
  text($('s-session'), s ? signed(s.session_gain) : '–');
  $('s-session').className = 'num ' + (s ? cls(s.session_gain) : '');
}
