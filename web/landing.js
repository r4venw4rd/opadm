"use strict";
// Talep formu (açık sayfa): istemci doğrulama UX içindir, asıl güvenlik sunucuda.
// Başarı SADECE sunucu 201 dönerse gösterilir.
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

document.addEventListener("DOMContentLoaded", () => {
  const form = $("request-form");
  if (!form) return;
  const btn = $("submit-btn");

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
        hideFormErrors();
        status("Talebiniz alındı. Ortalama 2 iş günü içinde dönüş yaparız.", "ok");
        form.reset();
      } else if (res.status === 400 && data.errors) {
        fields.forEach((f) => setErr(f, (data.errors || {})[f] || ""));
        showFormErrors(data.errors || {});
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
