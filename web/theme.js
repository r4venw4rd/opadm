"use strict";
// Tema düğmesi: localStorage + data-theme. <head>'teki inline script ilk
// boyayı yapar (kırpışma yok); bu dosya yalnız düğmeyi bağlar.
document.addEventListener("DOMContentLoaded", () => {
  const btn = document.getElementById("theme-btn");
  if (!btn) return;
  const iconSun = `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>`;
  const iconMoon = `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>`;

  const paint = () => {
    const light = document.documentElement.dataset.theme === "light";
    btn.innerHTML = light ? iconSun : iconMoon;
    btn.setAttribute("aria-label", light ? "Koyu temaya geç" : "Açık temaya geç");
  };
  btn.addEventListener("click", () => {
    const next = document.documentElement.dataset.theme === "light" ? "dark" : "light";
    document.documentElement.dataset.theme = next;
    try { localStorage.setItem("opadm-theme", next); } catch { /* yoksay */ }
    paint();
  });
  paint();
});
