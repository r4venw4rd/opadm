"use strict";
// Yardımcılar: kullanıcı verisi hep textContent ile basılır, innerHTML yok.
const $ = (id) => document.getElementById(id);

const fields = ["name", "email", "service", "message"];

function setErr(field, msg) {
  const input = $(field);
  const p = $(field + "-err");
  if (msg) {
    p.textContent = msg; // textContent: HTML enjeksiyonu yok
    p.hidden = false;
    input.setAttribute("aria-invalid", "true");
  } else {
    p.textContent = "";
    p.hidden = true;
    input.removeAttribute("aria-invalid");
  }
}

function status(msg, kind) {
  const el = $("form-status");
  el.textContent = msg; // textContent: güvenli
  el.dataset.kind = kind || "";
}

// Hata özeti: klavye + ekran okuyucu için liste, her öğe alanına bağlı.
function showFormErrors(errs) {
  const box = $("form-errors");
  const list = $("form-errors-list");
  list.textContent = "";
  for (const f of fields) {
    if (!errs[f]) continue;
    const li = document.createElement("li");
    const a = document.createElement("a");
    a.href = "#" + f;
    a.textContent = errs[f];
    a.addEventListener("click", (e) => {
      e.preventDefault();
      $(f).focus();
    });
    li.appendChild(a);
    list.appendChild(li);
  }
  box.hidden = false;
  box.focus();
}
function hideFormErrors() {
  $("form-errors").hidden = true;
  $("form-errors-list").textContent = "";
}

// Tema: tek düğme, localStorage + data-theme.
function initTheme() {
  const btn = $("theme-btn");
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
}

// İstemci doğrulaması yalnızca UX içindir; asıl güvenlik sunucudadır.
function validate(values) {
  const errs = {};
  if (values.name.length < 2 || values.name.length > 80)
    errs.name = "Adınız 2-80 karakter olmalı.";
  if (values.email.length > 254 || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(values.email))
    errs.email = "Geçerli bir e-posta adresi yazın.";
  if (!["backup", "monitoring", "hardening"].includes(values.service))
    errs.service = "Geçerli bir hizmet seçin.";
  if (values.message.length < 10 || values.message.length > 1000)
    errs.message = "Mesajınız 10-1000 karakter olmalı.";
  return errs;
}

document.addEventListener("DOMContentLoaded", async () => {
  initTheme();
  const form = $("request-form");
  const btn = $("submit-btn");
  const statusBtn = $("status-btn");

  async function refreshStatus() {
    const msg = $("status-msg");
    statusBtn.disabled = true;
    try {
      const res = await fetch("/api/status");
      const data = await res.json();
      if (res.ok && data.ok === true) {
        // Sayılar/metinler textContent ile basılır; sunucudan HTML gelmez.
        $("st-time").textContent = String(data.time);
        $("st-uptime").textContent = String(data.uptime_s);
        $("st-total").textContent = String(data.requests_total);
        $("st-host").textContent = String(data.host || "—");
        $("st-docker").textContent = data.in_docker === true ? "evet" : "hayır";
        $("st-go").textContent = String(data.go || "—");
        $("st-rt").textContent = String(data.goroutines) + " / " + String(data.cpu);
        $("st-mem").textContent = String(data.mem_alloc_mb);
        msg.textContent = "";
      } else {
        msg.textContent = "Durum alınamadı.";
      }
    } catch {
      msg.textContent = "Durum alınamadı: bağlantınızı kontrol edin.";
    } finally {
      statusBtn.disabled = false;
    }
  }
  statusBtn.addEventListener("click", refreshStatus);
  refreshStatus(); // açılışta bir kez doldur

  const demoBtn = $("demo-btn");
  demoBtn.addEventListener("click", async () => {
    const msg = $("demo-msg");
    const list = $("demo-list");
    demoBtn.disabled = true;
    msg.textContent = "Deneniyor…";
    try {
      const res = await fetch("/api/demo");
      const data = await res.json();
      if (res.ok && data.ok === true && Array.isArray(data.checks)) {        list.textContent = ""; // textContent: güvenli temizlik
        let failed = 0;
        for (const c of data.checks) {
          const li = document.createElement("li");
          // Sembol + metin: yalnızca renge bırakılmaz.
          li.textContent = (c.pass === true ? "✓ " : "✗ ") + c.name + " — " + c.detail;
          li.dataset.pass = c.pass === true ? "yes" : "no";
          list.appendChild(li);
          if (c.pass !== true) failed++;
        }
        msg.textContent = failed === 0
          ? "Tüm özellikler çalışıyor (" + data.checks.length + "/" + data.checks.length + ")."
          : failed + " özellik kalmış, detaya bakın.";
      } else if (res.status === 401) {
        msg.textContent = "Demo için önce giriş yapın.";
      } else {
        msg.textContent = "Demo alınamadı.";
      }
    } catch {
      msg.textContent = "Demo alınamadı: bağlantınızı kontrol edin.";
    } finally {
      demoBtn.disabled = false;
    }
  });

  // --- Hesap: üye ol / giriş yap / çıkış. Demo giriş ister.
  async function refreshMe() {
    const msg = $("account-msg");
    let email = "";
    try {
      const res = await fetch("/api/auth/me");
      const data = await res.json();
      if (res.ok && data.ok === true) email = String(data.email || "");
    } catch { /* girişsiz say */ }
    const loggedIn = email !== "";
    $("account-logged-out").hidden = loggedIn;
    $("account-logged-in").hidden = !loggedIn;
    if (loggedIn) $("account-email").textContent = email; // textContent: güvenli
    demoBtn.disabled = !loggedIn;
    if (!loggedIn) msg.textContent = "";
    return loggedIn;
  }

  async function postAuth(path, email, password) {
    const res = await fetch(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    });
    let data = {};
    try { data = await res.json(); } catch { /* yoksay */ }
    return { res, data };
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

  $("signup-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    authClear("su-err");
    const email = $("su-email").value.trim();
    const password = $("su-pass").value;
    const { res, data } = await postAuth("/api/auth/signup", email, password);
    if (res.status === 201) {
      // Üyelik tamam: aynı bilgilerle sessizce giriş yap.
      const login = await postAuth("/api/auth/login", email, password);
      if (login.res.ok) {
        $("signup-form").reset();
        $("login-form").reset();
        await refreshMe();
        $("account-msg").textContent = "Üyelik açıldı, giriş yapıldı.";
      } else {
        $("account-msg").textContent = "Üyelik açıldı, şimdi giriş yapın.";
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

  $("login-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    authClear("li-err");
    const { res } = await postAuth("/api/auth/login", $("li-email").value.trim(), $("li-pass").value);
    if (res.ok) {
      $("login-form").reset();
      await refreshMe();
      $("account-msg").textContent = "Giriş yapıldı.";
    } else if (res.status === 401) {
      authErr("li-err", "E-posta veya parola hatalı.");
    } else if (res.status === 429) {
      authErr("li-err", "Çok fazla deneme. Bir dakika bekleyin.");
    } else {
      authErr("li-err", "Giriş yapılamadı. Daha sonra tekrar deneyin.");
    }
  });

  $("logout-btn").addEventListener("click", async () => {
    try { await fetch("/api/auth/logout", { method: "POST" }); } catch { /* yoksay */ }
    await refreshMe();
    $("account-msg").textContent = "Çıkış yapıldı.";
  });

  await refreshMe();

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (btn.disabled) return; // çift gönderimi engelle

    const values = {
      name: $("name").value.trim(),
      email: $("email").value.trim().toLowerCase(),
      service: $("service").value,
      message: $("message").value.trim(),
    };

    const errs = validate(values);
    fields.forEach((f) => setErr(f, errs[f] || ""));
    if (Object.keys(errs).length > 0) {
      showFormErrors(errs);
      status("Formda hata var: " + errs[fields.find((f) => errs[f])], "error");
      return;
    }
    hideFormErrors();

    btn.disabled = true;
    const original = btn.textContent;
    btn.textContent = "Gönderiliyor…";
    status("Talebiniz gönderiliyor…", "");

    // Başarı SADECE sunucu 201 dönerse gösterilir.
    const ctrl = new AbortController();
    const timer = setTimeout(() => ctrl.abort(), 15000);
    try {
      const res = await fetch("/api/requests", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(values),
        signal: ctrl.signal,
      });
      let data = {};
      try { data = await res.json(); } catch { /* JSON dışı gövde: hata say */ }
      if (res.status === 201 && data.ok === true) {
        status("Talebiniz alındı. Ortalama 2 iş günü içinde dönüş yaparız.", "ok");
        form.reset();
        refreshStatus(); // demoda sayaç artar görünür
      } else if (res.status === 400 && data.errors) {
        fields.forEach((f) => setErr(f, (data.errors || {})[f] || ""));
        const first = fields.find((f) => (data.errors || {})[f]);
        if (first) $(first).focus();
        status("Formda hata var, kontrol edip tekrar deneyin.", "error");
      } else if (res.status === 429) {
        status("Çok fazla istek gönderdiniz. Bir dakika sonra tekrar deneyin.", "error");
      } else {
        status("Kayıt şu anda alınamadı. Lütfen daha sonra tekrar deneyin.", "error");
      }
    } catch {
      status("Ağ hatası: bağlantınızı kontrol edip tekrar deneyin.", "error");
    } finally {
      clearTimeout(timer);
      btn.disabled = false;
      btn.textContent = original;
    }
  });
});
