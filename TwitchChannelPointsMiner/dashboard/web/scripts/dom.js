// Small helpers for reading and building page elements.
const $ = id => document.getElementById(id);
const root = document.documentElement;
function el(tag, props, ...kids) {
  const e = Object.assign(document.createElement(tag), props || {});
  e.append(...kids);
  return e;
}
function text(node, s) { if (node.textContent !== s) node.textContent = s; }
