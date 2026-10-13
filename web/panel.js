"use strict";
// Panel (giriş arkası): kapı + canlı durum (soket) + demo + çıkış.
// Girişsiz açılırsa /login/'e yollanır.
const $ = (id) => document.getElementById(id);

async function meEmail() {
  try {
    const res = await fetch("/api/auth/me");
    const data = await res.json();
    if (res.ok && data.ok === true) return String(data.email || "");
  } catch { /* girişsiz say */ }
  return "";
}

// Sayılar/metinler textContent ile basılır; sunucudan HTML gelmez.
function paintStatus(data) {
  $("st-time").textContent = String(data.time);
  $("st-uptime").textContent = String(data.uptime_s);
  $("st-total").textContent = String(data.requests_total);
  $("st-host").textContent = String(data.host || "—");
  $("st-docker").textContent = data.in_docker === true ? "evet" : "hayır";
  $("st-go").textContent = String(data.go || "—");
  $("st-rt").textContent = String(data.goroutines) + " / " + String(data.cpu);
  $("st-mem").textContent = String(data.mem_alloc_mb);
}

document.addEventListener("DOMContentLoaded", async () => {
  const email = await meEmail();
  if (!email) {
    location.replace("/login/");
    return;
  }
  $("panel-user").textContent = email; // textContent: güvenli

  $("logout-btn").addEventListener("click", async () => {
    try { await fetch("/api/auth/logout", { method: "POST" }); } catch { /* yoksay */ }
    location.replace("/login/");
  });

  const statusBtn = $("status-btn");
  let liveOn = false;

  async function refreshStatus() {
    const msg = $("status-msg");
    statusBtn.disabled = true;
    try {
      const res = await fetch("/api/status");
      const data = await res.json();
      if (res.ok && data.ok === true) {
        paintStatus(data);
        if (!liveOn) msg.textContent = "";
      } else if (res.status === 401) {
        location.replace("/login/");
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

  // Canlı soket: 5 sn'de bir + her kayıtta iter. 3 denemeden sonra bırakır.
  let liveTries = 0;
  function connectLive() {
    if (liveTries >= 3) return;
    liveTries++;
    let ws;
    try {
      const proto = location.protocol === "https:" ? "wss" : "ws";
      ws = new WebSocket(proto + "://" + location.host + "/api/live");
    } catch {
      return;
    }
    ws.onopen = () => { liveOn = true; };
    ws.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data);
        if (data && data.ok === true) {
          paintStatus(data);
          $("status-msg").textContent = "";
        }
      } catch { /* bozuk kare yoksay */ }
    };
    ws.onclose = () => {
      liveOn = false;
      setTimeout(connectLive, 3000);
    };
    ws.onerror = () => { try { ws.close(); } catch { /* yoksay */ } };
  }
  refreshStatus();
  connectLive();

  const demoBtn = $("demo-btn");
  demoBtn.addEventListener("click", async () => {
    const msg = $("demo-msg");
    const list = $("demo-list");
    demoBtn.disabled = true;
    msg.textContent = "Deneniyor…";
    try {
      const res = await fetch("/api/demo");
      const data = await res.json();
      if (res.ok && data.ok === true && Array.isArray(data.checks)) {
        list.textContent = ""; // textContent: güvenli temizlik
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
          ? "Tüm özellikler çalışıyor (" + data.checks.length + "/" + data.checks.length + ", koşu " + data.run_id + ", " + data.took_ms + "ms)."
          : failed + " özellik kalmış, detaya bakın (koşu " + data.run_id + ").";
      } else if (res.status === 401) {
        location.replace("/login/");
      } else {
        msg.textContent = "Demo alınamadı.";
      }
    } catch {
      msg.textContent = "Demo alınamadı: bağlantınızı kontrol edin.";
    } finally {
      demoBtn.disabled = false;
    }
  });
});
