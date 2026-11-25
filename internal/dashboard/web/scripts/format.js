// Number and date formats, shared so every figure on the page reads the same.
const nf = new Intl.NumberFormat();
const fmt = n => nf.format(n);
const signed = n => (n > 0 ? '+' : n < 0 ? '−' : '') + fmt(Math.abs(n));
const cls = n => (n > 0 ? 'up' : n < 0 ? 'down' : '');
const dfDay = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' });
const dfLong = new Intl.DateTimeFormat(undefined, { weekday: 'long', month: 'short', day: 'numeric' });
const dfTime = new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' });
const dfClock = new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' });
const dfTip = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
