package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"

	"opadm/internal/application"
	"opadm/internal/domain"
	"opadm/internal/infrastructure/live"
	"opadm/internal/infrastructure/postgres"
	"opadm/internal/infrastructure/ratelimit"
)

const maxBodyBytes = 16 * 1024 // 16 KB: form için fazlasıyla yeter, şişkin body'yi keser

var errNoDatabaseURL = errors.New("DATABASE_URL yok")

// App, HTTP adaptörüdür: başlık/gövde/JSON ile uğraşır, iş kuralı bilmez.
// Kurallar application.Service'te, doğrulama domain'dedir.
type App struct {
	Service *application.Service
	Limiter *ratelimit.Limiter
	// TrustProxy kapalıyken proxy başlıkları okunmaz (güvenli varsayılan).
	// Yalnızca başlıkları ezen bir proxy (Vercel, Cloudflare Tunnel) arkasında
	// TRUST_PROXY=1 ile açılır (bkz. README).
	TrustProxy bool
	// SecureCookies=1 (Vercel/HTTPS) çereze Secure ekler; yerelde 0 kalır
	// yoksa tarayıcı http:// üzerinde çerezi saklamaz.
	SecureCookies bool
	// LiveHub, /api/live izleyicilerine anlık görüntü iter.
	LiveHub *live.Hub
}

func New(svc *application.Service, l *ratelimit.Limiter, trustProxy bool) *App {
	hub := live.NewHub()
	never := make(chan struct{}) // süreç ömrü: hub kapanmaz
	go hub.Run(never)
	return &App{Service: svc, Limiter: l, TrustProxy: trustProxy, LiveHub: hub}
}

// NewFromEnv, main.go ve Vercel fonksiyonlarının ortak kurulumudur:
// env'den DB'ye bağlanır, limiter + janitor hazırlar.
func NewFromEnv() (*App, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, errNoDatabaseURL
	}
	st, err := postgres.Open(url, os.Getenv("SIMPLE_PROTOCOL") == "1")
	if err != nil {
		return nil, err
	}
	limiter := ratelimit.New(10, time.Minute)
	never := make(chan struct{}) // süreç ömrü: janitor kapanmaz
	go limiter.StartJanitor(5*time.Minute, never)
	go func() { // ölü oturum süpürme (saatte bir; hata sessiz geçilir)
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for range t.C {
			_ = st.DeleteExpiredSessions()
		}
	}()
	svc := application.New(st, st, time.Now())
	app := New(svc, limiter, os.Getenv("TRUST_PROXY") == "1")
	app.SecureCookies = os.Getenv("SESSION_SECURE") == "1"
	return app, nil
}

// requestInput, HTTP gövdesinin DTO'sudur; domain.Request'e çevrilir.
type requestInput struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Service string `json:"service"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// clientIP, güvenilir proxy yoksa RemoteAddr'ı kullanır. Başlıklara güvenmek
// sahte başlıkla limit atlatmaya izin verirdi; bu yüzden TRUST_PROXY=1
// olmadan CF-Connecting-IP/X-Forwarded-For okunmaz.
// Yalnızca rate limit + ip_hash içindir, kimlik doğrulama için kullanılmaz.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
			return ip
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if first, _, _ := strings.Cut(xff, ","); strings.TrimSpace(first) != "" {
				return strings.TrimSpace(first)
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// hashIP, ham IP'yi saklamak yerine SHA-256 özetini saklar (KVKK temkinliliği).
func hashIP(ip string) string {
	sum := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(sum[:])
}

// guard, yazan endpoint'lerin ortak kapısıdır: JSON zarfı + Origin +
// hız limiti. Başarısızsa yanıtı yazar, ok=false döner.
func (a *App) guard(w http.ResponseWriter, r *http.Request) (ip string, ok bool) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{
			"ok": false, "error": "Content-Type application/json olmalı.",
		})
		return "", false
	}
	ip = clientIP(r, a.TrustProxy)
	if !domain.AllowedOrigin(r.Host, r.Header.Get("Origin"), r.Header.Get("Referer")) {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"ok": false, "error": "Geçersiz istek kaynağı.",
		})
		return "", false
	}
	if !a.Limiter.Allow(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"ok": false, "error": "Çok fazla istek gönderdiniz. Bir dakika sonra tekrar deneyin.",
		})
		return "", false
	}
	return ip, true
}

// decode, gövdeyi okur; bozuksa 400 yazar.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // bilinmeyen alan = 400, şema kaymasını erken yakalar
	if err := dec.Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "error": "Geçersiz istek gövdesi. Tüm alanları kontrol edip tekrar deneyin.",
		})
		return false
	}
	return true
}

// Create, POST /api/requests. Başarı SADECE INSERT commit edildikten
// sonra 201 ile bildirilir; DB hatasında 500 + genel mesaj döner.
func (a *App) Create(w http.ResponseWriter, r *http.Request) {
	ip, ok := a.guard(w, r)
	if !ok {
		return
	}
	var in requestInput
	if !decode(w, r, &in) {
		return
	}
	_, fieldErrs, err := a.Service.Submit(domain.Request{
		Name: in.Name, Email: in.Email, Service: in.Service, Message: in.Message,
	}, hashIP(ip))
	if len(fieldErrs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "errors": fieldErrs,
		})
		return
	}
	if err != nil {
		// İç detay sızdırılmaz; istemciye genel mesaj.
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"ok": false, "error": "Kayıt şu anda alınamadı. Lütfen daha sonra tekrar deneyin.",
		})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
	a.pushLive() // canlı izleyiciler sayacı hemen görür
}

// snapshotMap, status + live akışının ortak gövdesidir.
func (a *App) snapshotMap() (map[string]any, error) {
	snap, err := a.Service.Snapshot()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"ok":             true,
		"time":           snap.Time,
		"uptime_s":       snap.UptimeS,
		"requests_total": snap.Total,
		"host":           snap.Host,
		"in_docker":      snap.InDocker,
		"go":             snap.Go,
		"goroutines":     snap.Goroutines,
		"cpu":            snap.CPU,
		"mem_alloc_mb":   snap.MemAllocMB,
	}, nil
}

// pushLive, izleyicilere taze sayacı iter. Hub doluysa düşer;
// kayıt yolunu asla bekletmez.
func (a *App) pushLive() {
	m, err := a.snapshotMap()
	if err != nil {
		return
	}
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	a.LiveHub.Push(b)
}

// Status, GET /api/status. Giriş ister (panel sayfası arkası);
// kişisel veri içermez ama sistem bilgisi girişe özeldir.
func (a *App) Status(w http.ResponseWriter, r *http.Request) {
	if _, ok, _ := a.currentUser(r); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"ok": false, "error": "Durum için giriş yapın.",
		})
		return
	}
	m, err := a.snapshotMap()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false})
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// Live, GET /api/live. WebSocket: bağlan, ilk görüntüyü al, sonra 5 sn'de
// bir + her kayıtta itilir. Uzun ömürlü süreçte çalışır (binary/Docker);
// Vercel fonksiyonunda yaşamaz (bkz. README) — istemci fetch'e düşer.
func (a *App) Live(w http.ResponseWriter, r *http.Request) {
	if !domain.AllowedOrigin(r.Host, r.Header.Get("Origin"), "") {
		http.Error(w, "Geçersiz istek kaynağı.", http.StatusForbidden)
		return
	}
	if _, ok, _ := a.currentUser(r); !ok {
		http.Error(w, "Durum için giriş yapın.", http.StatusUnauthorized)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	a.LiveHub.Attach(r.Context(), conn, func() []byte {
		m, err := a.snapshotMap()
		if err != nil {
			return nil
		}
		b, err := json.Marshal(m)
		if err != nil {
			return nil
		}
		return b
	}, 5*time.Second)
}

// Demo, GET /api/demo. Giriş ister: işveren üye olup girer, özellikleri
// canlı dener. DB'ye yazmaz, kota yakmaz. Her check gerçekten koşar.
func (a *App) Demo(w http.ResponseWriter, r *http.Request) {
	if _, ok, _ := a.currentUser(r); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"ok": false, "error": "Demo için giriş yapın.",
		})
		return
	}
	checks := a.Service.RunDemo(a.Limiter)
	out := make([]map[string]any, 0, len(checks))
	for _, c := range checks {
		out = append(out, map[string]any{
			"id": c.ID, "name": c.Name, "pass": c.Pass, "detail": c.Detail,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "checks": out})
}

// Health, GET /healthz. DB'ye ping atar; DB kapalıysa 500 verir.
func (a *App) Health(w http.ResponseWriter, r *http.Request) {
	if err := a.Service.Ping(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
