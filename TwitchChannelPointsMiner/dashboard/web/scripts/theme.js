// Light and dark theme, and the colours the chart reads from it.
let C = null;
function colors() {
  if (C) return C;
  const cs = getComputedStyle(root);
  C = { font: '12px ' + cs.getPropertyValue('--mono').trim() };
  for (const k of ['card', 'ink2', 'muted', 'grid', 'hover', 'line', 'wash', 'up', 'down', 'neu']) C[k] = cs.getPropertyValue('--' + k).trim();
  return C;
}
function setTheme(t) {
  root.dataset.theme = t; C = null;
  $('theme').textContent = t === 'dark' ? 'Light theme' : 'Dark theme';
  draw();
}
$('theme').onclick = () => {
  const t = root.dataset.theme === 'dark' ? 'light' : 'dark';
  store.set('theme', t); setTheme(t);
};
