"use strict";
// Tema düğmesi: localStorage + data-theme. <head>'teki inline script ilk
// boyayı yapar (kırpışma yok); bu dosya yalnız düğmeyi bağlar.
document.addEventListener("DOMContentLoaded", () => {
  const btn = document.getElementById("theme-btn");
  if (!btn) return;
  const paint = () => {
    const light = document.documentElement.dataset.theme === "light";
    btn.textContent = light ? "☀" : "☾";
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
