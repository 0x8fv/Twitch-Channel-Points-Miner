// First paint and the refresh timer.
setTheme(root.dataset.theme);
renderHead();
refresh();
setInterval(() => { if (!document.hidden) refresh(); }, REFRESH_MS);
document.addEventListener('visibilitychange', () => { if (!document.hidden) refresh(); });
