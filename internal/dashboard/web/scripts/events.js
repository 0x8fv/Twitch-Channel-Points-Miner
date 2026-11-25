// Event types: how each one is named, drawn and which polarity it has.
// Event glyphs in unit coordinates, used by the legend, the tooltip and the activity list.
const GLYPH = {
  dot:     { d: 'M1,0A1,1 0 1,1 -1,0A1,1 0 1,1 1,0Z' },
  square:  { d: 'M-.9,-.9H.9V.9H-.9Z' },
  up:      { d: 'M0,-1.2L1.1,.8L-1.1,.8Z' },
  down:    { d: 'M0,1.2L1.1,-.8L-1.1,-.8Z' },
  diamond: { d: 'M0,-1.25L1.25,0L0,1.25L-1.25,0Z' },
  hollow:  { d: 'M0,-1.25L1.25,0L0,1.25L-1.25,0Z', line: true, open: true },
  ring:    { d: 'M1,0A1,1 0 1,1 -1,0A1,1 0 1,1 1,0Z', line: true, open: true },
  cross:   { d: 'M-1,-1L1,1M1,-1L-1,1', line: true },
  bar:     { d: 'M-1.2,0H1.2', line: true },
};
// tone is the polarity from constants.EventPolarity: pos, neg, neu. A glyph's shape
// names the event and its colour names the polarity; the accent stays with the line.
const EV = {
  WATCH_STREAK:     { label: 'Watch streak', glyph: 'dot',     tone: 'pos' },
  CLAIM:            { label: 'Bonus claim',  glyph: 'dot',     tone: 'pos', scale: .7 },
  RAID:             { label: 'Raid',         glyph: 'square',  tone: 'pos' },
  BET_WIN:          { label: 'Bet won',      glyph: 'up',      tone: 'pos' },
  BET_LOSE:         { label: 'Bet lost',     glyph: 'down',    tone: 'neg' },
  BET_REFUND:       { label: 'Bet refund',   glyph: 'bar',     tone: 'neu' },
  DROP_CLAIM:       { label: 'Drop claim',   glyph: 'diamond', tone: 'pos' },
  MOMENT_CLAIM:     { label: 'Moment claim', glyph: 'hollow',  tone: 'pos' },
  STREAMER_ONLINE:  { label: 'Went online',  glyph: 'ring',    tone: 'pos' },
  STREAMER_OFFLINE: { label: 'Went offline', glyph: 'cross',   tone: 'neg' },
};
const TONE_LABEL = { pos: 'Positive', neg: 'Negative', neu: 'Neutral' };
const TONE_COLOR = { pos: 'up', neg: 'down', neu: 'neu' };
function tag(type) {
  const k = EV[type], g = GLYPH[k.glyph], s = k.scale || 1;
  const e = el('span', { className: 'tag' });
  const paint = g.line ? 'fill="none" stroke="currentColor" stroke-width=".5" stroke-linecap="round"' : 'fill="currentColor"';
  e.innerHTML = `<svg width="12" height="12" viewBox="-2 -2 4 4" aria-hidden="true" style="color:var(--${TONE_COLOR[k.tone]})"><path transform="scale(${s})" d="${g.d}" ${paint}/></svg>`;
  e.append(k.label);
  return e;
}
