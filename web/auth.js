"use strict";
// Üyelik sayfaları (login + signup): hangi form varsa o bağlanır.
// Girişli gelen /panel/'e yollanır.
const $ = (id) => document.getElementById(id);

async function meEmail() {
  try {
    const res = await fetch("/api/auth/me");
    const data = await res.json();
    if (res.ok && data.ok === true) return String(data.email || "");
  } catch { /* girişsiz say */ }
  return "";
}

async function postAuth(path, email, password) {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), 15000);
  try {
    const res = await fetch(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
      signal: ctrl.signal,
    });
    let data = {};
    try { data = await res.json(); } catch { /* yoksay */ }
    return { res, data };
  } catch {
    return { res: { ok: false, status: 0 }, data: {} };
  } finally {
    clearTimeout(timer);
  }
}

function authErr(id, text) {
  const p = $(id);
  p.textContent = text;
  p.hidden = false;
}
function authClear(id) {
  const p = $(id);
  p.textContent = "";
  p.hidden = true;
}

document.addEventListener("DOMContentLoaded", async () => {
  if (await meEmail()) {
    location.replace("/panel/");
    return;
  }
  const su = $("signup-form");
  const li = $("login-form");
  const msg = $("account-msg");

  if (su) {
    su.addEventListener("submit", async (e) => {
      e.preventDefault();
      authClear("su-err");
      const email = $("su-email").value.trim();
      const password = $("su-pass").value;
      const { res, data } = await postAuth("/api/auth/signup", email, password);
      if (res.status === 201) {
        // Üyelik tamam: aynı bilgilerle sessizce giriş yap, panele git.
        const login = await postAuth("/api/auth/login", email, password);
        if (login.res.ok) {
          location.replace("/panel/");
        } else if (msg) {
          msg.textContent = "Üyelik açıldı, şimdi giriş yapın.";
        }
      } else if (res.status === 409) {
        authErr("su-err", "Bu e-posta kayıtlı. Giriş yapın.");
      } else if (res.status === 400 && data.errors) {
        const first = Object.values(data.errors)[0];
        authErr("su-err", String(first || "Bilgileri kontrol edin."));
      } else if (res.status === 429) {
        authErr("su-err", "Çok fazla deneme. Bir dakika bekleyin.");
      } else {
        authErr("su-err", "Kayıt açılamadı. Daha sonra tekrar deneyin.");
      }
    });
  }

  if (li) {
    li.addEventListener("submit", async (e) => {
      e.preventDefault();
      authClear("li-err");
      const { res } = await postAuth("/api/auth/login", $("li-email").value.trim(), $("li-pass").value);
      if (res.ok) {
        location.replace("/panel/");
      } else if (res.status === 401) {
        authErr("li-err", "E-posta veya parola hatalı.");
      } else if (res.status === 429) {
        authErr("li-err", "Çok fazla deneme. Bir dakika bekleyin.");
      } else {
        authErr("li-err", "Giriş yapılamadı. Daha sonra tekrar deneyin.");
      }
    });
  }
});
