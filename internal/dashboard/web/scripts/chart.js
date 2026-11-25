// Balance line and the event lane beneath it.
const plot = $('plot'), cv = $('cv'), ov = $('ov'), tip = $('tip');
const ctx = cv.getContext('2d'), octx = ov.getContext('2d');
let geo = null, W = 0, H = 0, DPR = 1;
function fit() {
  W = plot.clientWidth; H = plot.clientHeight; DPR = window.devicePixelRatio || 1;
  const pw = Math.round(W * DPR), ph = Math.round(H * DPR);
  for (const c of [cv, ov]) if (c.width !== pw || c.height !== ph) { c.width = pw; c.height = ph; }
  return W > 0 && H > 0;
}
// The balance line has the plot to itself. Events sit in a lane underneath, counted per
// time slot: positive and neutral ones rise from the baseline, negative ones hang below.
const LANE_UP = 28, LANE_DOWN = 14, SLOT = 8;
// The base canvas is redrawn only when data, size or theme change. Pointer movement
// repaints the small overlay canvas alone.
function draw() {
  if (!fit()) return;
  const c = colors(), d = state.data;
  ctx.setTransform(DPR, 0, 0, DPR, 0, 0);
  ctx.clearRect(0, 0, W, H);
  ctx.font = c.font;
  geo = null;
  if (!d || !d.series.length) {
    ctx.fillStyle = c.muted; ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
    ctx.fillText(d ? 'No data in this range yet' : '', W / 2, H / 2);
    drawHover();
    return;
  }
  const S = d.series, y0 = 8, base = H - 28 - LANE_DOWN, y1 = base - LANE_UP - 20;
  const pad = Math.max(1, (d.stats.max - d.stats.min) * 0.12);
  const yt = niceTicks(d.stats.min - pad, d.stats.max + pad, 4), yf = axisFormat(yt);
  const lo = yt[0], hi = yt[yt.length - 1];
  const x0 = Math.ceil(Math.max(ctx.measureText('Events').width, ...yt.map(v => ctx.measureText(yf(v)).width))) + 12, x1 = W - 8;
  const span = Math.max(1, d.to - d.from);
  const X = t => x0 + (t - d.from) / span * (x1 - x0), Y = v => y1 - (v - lo) / (hi - lo) * (y1 - y0);

  ctx.lineWidth = 1; ctx.strokeStyle = c.grid; ctx.fillStyle = c.ink2;
  ctx.textBaseline = 'middle'; ctx.textAlign = 'right';
  ctx.beginPath();
  for (const v of yt) {
    const y = Math.round(Y(v)) + .5;
    ctx.moveTo(x0, y); ctx.lineTo(x1, y);
    ctx.fillText(yf(v), x0 - 12, y);
  }
  ctx.moveTo(x0, base + .5); ctx.lineTo(x1, base + .5);
  ctx.stroke();
  ctx.fillStyle = c.muted;
  ctx.fillText('Events', x0 - 12, base);
  const tt = timeTicks(d.from, d.to, Math.max(2, Math.floor((x1 - x0) / 130)));
  ctx.fillStyle = c.ink2; ctx.textBaseline = 'top'; ctx.textAlign = 'center';
  for (const t of tt.t) {
    const x = X(t);
    if (x >= x0 + 24 && x <= x1 - 24) ctx.fillText(timeLabel(t, tt.step), x, base + LANE_DOWN + 10);
  }

  // Step line: a balance holds until the next sample.
  const lastY = Y(S[S.length - 1][1]);
  ctx.beginPath();
  ctx.moveTo(X(S[0][0]), Y(S[0][1]));
  for (let i = 1; i < S.length; i++) { const x = X(S[i][0]); ctx.lineTo(x, Y(S[i - 1][1])); ctx.lineTo(x, Y(S[i][1])); }
  ctx.lineTo(x1, lastY);
  ctx.strokeStyle = c.line; ctx.lineWidth = 2; ctx.lineJoin = 'round'; ctx.lineCap = 'round'; ctx.stroke();
  ctx.lineTo(x1, y1); ctx.lineTo(X(S[0][0]), y1); ctx.closePath();
  ctx.fillStyle = c.wash; ctx.fill();
  // The current balance, where the line ends.
  ctx.beginPath(); ctx.arc(x1, lastY, 3.5, 0, Math.PI * 2);
  ctx.fillStyle = c.line; ctx.fill(); ctx.strokeStyle = c.card; ctx.lineWidth = 2; ctx.stroke();

  // Event lane.
  const n = Math.max(1, Math.floor((x1 - x0) / SLOT)), sw = (x1 - x0) / n, slots = new Array(n).fill(null);
  for (const e of shownEvents(d)) {
    const i = Math.min(n - 1, Math.max(0, Math.floor((X(e.t) - x0) / sw)));
    const slot = slots[i] || (slots[i] = { pos: 0, neu: 0, neg: 0, ev: [] });
    slot[EV[e.type].tone]++; slot.ev.push(e);
  }
  let up = 0, down = 0;
  for (const q of slots) if (q) { up = Math.max(up, q.pos + q.neu); down = Math.max(down, q.neg); }
  const unit = Math.min(6, up ? LANE_UP / up : 6, down ? LANE_DOWN / down : 6);
  const tall = k => k ? Math.max(2, Math.round(k * unit)) : 0;
  for (let i = 0; i < n; i++) {
    const q = slots[i];
    if (!q) continue;
    const x = Math.round(x0 + i * sw) + 1, w = Math.max(2, Math.round(sw) - 2);
    const hp = tall(q.pos), hn = tall(q.neu), hd = tall(q.neg);
    if (hp) { ctx.fillStyle = c.up; ctx.fillRect(x, base - hp, w, hp); }
    if (hn) { ctx.fillStyle = c.neu; ctx.fillRect(x, base - hp - hn, w, hn); }
    if (hd) { ctx.fillStyle = c.down; ctx.fillRect(x, base + 1, w, hd); }
  }
  geo = { Y, x0, x1, y0, y1, base, d, slots, sw };
  drawHover();
}
