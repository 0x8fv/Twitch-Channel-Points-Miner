// Axis ticks, labels and sample lookup for the chart.
function niceTicks(lo, hi, n) {
  if (hi <= lo) { lo -= 1; hi += 1; }
  const raw = (hi - lo) / n, mag = 10 ** Math.floor(Math.log10(raw)), f = raw / mag;
  const step = Math.max(1, (f <= 1 ? 1 : f <= 2 ? 2 : f <= 5 ? 5 : 10) * mag);
  const a = Math.floor(lo / step) * step, b = Math.ceil(hi / step) * step, t = [];
  for (let v = a; v <= b + step / 2; v += step) t.push(Math.round(v));
  return t;
}
// Short labels (1.2M, 340k) only when every tick stays exact; otherwise full numbers.
function axisFormat(ticks) {
  const step = ticks[1] - ticks[0], top = Math.max(...ticks.map(Math.abs));
  const unit = top >= 1e6 ? 1e6 : top >= 1e4 ? 1e3 : 1;
  if (unit === 1 || step % (unit / 10)) return fmt;
  return v => +(v / unit).toFixed(1) + (unit === 1e6 ? 'M' : 'k');
}
const MIN = 60000, HOUR = 60 * MIN, DAY = 24 * HOUR;
const STEPS = [5 * MIN, 15 * MIN, 30 * MIN, HOUR, 2 * HOUR, 3 * HOUR, 6 * HOUR, 12 * HOUR, DAY, 2 * DAY, 7 * DAY, 14 * DAY, 30 * DAY, 90 * DAY, 180 * DAY, 365 * DAY];
function timeTicks(from, to, maxCount) {
  const step = STEPS.find(s => (to - from) / s <= maxCount) || STEPS[STEPS.length - 1];
  const off = new Date(to).getTimezoneOffset() * MIN, t = [];
  for (let v = Math.ceil((from - off) / step) * step + off; v <= to; v += step) t.push(v);
  return { step, t };
}
function timeLabel(t, step) {
  const d = new Date(t);
  return step >= DAY || (d.getHours() === 0 && d.getMinutes() === 0) ? dfDay.format(d) : dfTime.format(d);
}
function sampleAt(series, t) {
  let lo = 0, hi = series.length - 1;
  if (!series.length || t < series[0][0]) return null;
  while (lo < hi) { const mid = (lo + hi + 1) >> 1; series[mid][0] <= t ? lo = mid : hi = mid - 1; }
  return { value: series[lo][1], prev: lo > 0 ? series[lo - 1][1] : null };
}
