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
  $("st-ver").textContent = String(data.version || "—");
}

// Servis metrikleri: ürünün iş değerini gösteren canlı sayılar.
function paintSummary(sum) {
  const s = sum || {};
  $("ds-uptime").textContent = String(s.uptime ?? "—");
  $("ds-total").textContent = String(s.requests_total ?? 0);
  $("ds-last").textContent = String(s.last_request_at ?? "—");
  $("ds-write").textContent = String(s.write_latency_ms ?? "—");
  $("ds-db").textContent = String(s.db_latency_us ?? "—");
  $("ds-ok").textContent = String(s.submissions_ok ?? 0);
  $("ds-fail").textContent = String(s.submissions_failed ?? 0);
  $("ds-rate").textContent = String(s.error_rate_pct ?? "0.0");
}

// Bir kontrol satırı: ✓/✗ + ad + "ne işe yarar" + teknik detay.
function checkItem(c) {
  const li = document.createElement("li");
  li.dataset.pass = c.pass === true ? "yes" : "no";

  const mark = document.createElement("span");
  mark.className = "demo-mark";
  mark.setAttribute("aria-hidden", "true");
  mark.textContent = c.pass === true ? "✓" : "✗";

  const body = document.createElement("div");

  const title = document.createElement("strong");
  title.textContent = String(c.name || c.id || "");
  // Renk tek başına anlam taşımasın: durumu metinle de söyle.
  const sr = document.createElement("span");
  sr.className = "sr-only";
  sr.textContent = (c.pass === true ? "Geçti: " : "Kaldı: ");
  title.prepend(sr);

  const why = document.createElement("p");
  why.className = "demo-why";
  why.textContent = String(c.why || "");

  const detail = document.createElement("p");
  detail.className = "demo-detail";
  detail.textContent = String(c.detail || "");

  body.append(title, why, detail);
  li.append(mark, body);
  return li;
}

// Kontrolleri sunucudan gelen sırayı koruyarak gruplara ayır.
function paintChecks(checks) {
  const host = $("demo-list");
  host.textContent = "";
  const groups = [];
  const byGroup = new Map();
  for (const c of checks) {
    const g = String(c.group || "Kontroller");
    if (!byGroup.has(g)) {
      byGroup.set(g, []);
      groups.push(g);
    }
    byGroup.get(g).push(c);
  }
  let failed = 0;
  for (const g of groups) {
    const h = document.createElement("h4");
    h.className = "demo-group";
    h.textContent = g;
    host.appendChild(h);

    const ul = document.createElement("ul");
    ul.className = "demo-items";
    for (const c of byGroup.get(g)) {
      ul.appendChild(checkItem(c));
      if (c.pass !== true) failed++;
    }
    host.appendChild(ul);
  }
  return failed;
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
    demoBtn.disabled = true;
    msg.textContent = "Deneniyor…";
    try {
      const res = await fetch("/api/demo");
      const data = await res.json();
      if (res.ok && data.ok === true && Array.isArray(data.checks)) {
        paintSummary(data.summary);
        const failed = paintChecks(data.checks);
        const n = data.checks.length;
        msg.textContent = failed === 0
          ? "Tüm özellikler çalışıyor (" + n + "/" + n + ", koşu " + data.run_id + ", " + data.took_ms + "ms)."
          : failed + " kontrol kalmış, detaya bakın (koşu " + data.run_id + ").";
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
