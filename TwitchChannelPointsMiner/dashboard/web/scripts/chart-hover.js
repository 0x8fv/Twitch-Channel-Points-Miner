// Crosshair and tooltip, drawn on the overlay canvas.
function drawHover() {
  octx.setTransform(DPR, 0, 0, DPR, 0, 0);
  octx.clearRect(0, 0, W, H);
  const hv = state.hover;
  if (!hv || !geo) { tip.hidden = true; return; }
  const c = colors(), { Y, x0, x1, y0, base, d, slots, sw } = geo;
  const px = Math.min(x1, Math.max(x0, hv.x));
  const t = d.from + (px - x0) / (x1 - x0) * (d.to - d.from);
  const sample = sampleAt(d.series, t), v = sample ? sample.value : null;
  const i = Math.min(slots.length - 1, Math.floor((px - x0) / sw)), slot = slots[i];
  // Every event of the hovered time slot shares the tooltip, newest six shown.
  const shown = slot ? slot.ev.slice(-6) : [], extra = slot ? slot.ev.length - shown.length : 0;
  if (slot) { octx.fillStyle = c.hover; octx.fillRect(Math.round(x0 + i * sw), base - LANE_UP - 4, Math.round(sw), LANE_UP + LANE_DOWN + 8); }
  octx.strokeStyle = c.muted; octx.lineWidth = 1; octx.globalAlpha = .6;
  octx.beginPath(); octx.moveTo(Math.round(px) + .5, y0); octx.lineTo(Math.round(px) + .5, base + LANE_DOWN); octx.stroke();
  octx.globalAlpha = 1;
  if (v !== null) {
    octx.beginPath(); octx.arc(px, Y(v), 4.5, 0, Math.PI * 2);
    octx.fillStyle = c.line; octx.fill(); octx.strokeStyle = c.card; octx.lineWidth = 2; octx.stroke();
  }
  const kids = [el('div', { className: 'v num', textContent: v === null ? 'No data' : fmt(v) })];
  if (sample && sample.prev !== null && sample.prev !== v) {
    kids.push(el('div', null, el('span', { className: 'num ' + cls(v - sample.prev), textContent: signed(v - sample.prev) }), ' vs previous sample'));
  }
  kids.push(el('div', { className: 't num', textContent: dfTip.format(new Date(t)) }));
  for (const e of shown) {
    const head = tag(e.type), msg = e.msg;
    head.append(el('span', { className: 't num', textContent: dfTime.format(new Date(e.t)) }));
    kids.push(el('div', { className: 'e' }, head, ...(msg ? [el('div', { className: 'm', textContent: msg })] : [])));
  }
  if (extra) kids.push(el('div', { className: 'more', textContent: fmt(extra) + (extra === 1 ? ' earlier event' : ' earlier events') + ' in this slot' }));
  tip.replaceChildren(...kids);
  tip.hidden = false;
  const tw = tip.offsetWidth, th = tip.offsetHeight;
  let left = px + 12; if (left + tw > W - 4) left = px - tw - 12;
  let top = hv.y - th - 12;
  if (top < 4) top = Math.min(H - th - 4, hv.y + 14);
  tip.style.transform = `translate(${Math.round(Math.max(4, left))}px,${Math.round(Math.max(4, top))}px)`;
}
let raf = 0, rafFull = false;
function schedule(full) {
  rafFull = rafFull || full;
  if (!raf) raf = requestAnimationFrame(() => { raf = 0; const f = rafFull; rafFull = false; f ? draw() : drawHover(); });
}
ov.addEventListener('pointermove', e => { const b = ov.getBoundingClientRect(); state.hover = { x: e.clientX - b.left, y: e.clientY - b.top }; schedule(false); });
ov.addEventListener('pointerleave', () => { state.hover = null; schedule(false); });
new ResizeObserver(() => schedule(true)).observe(plot);
