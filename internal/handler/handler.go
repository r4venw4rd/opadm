package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/mail"
	"strings"

	"opadm/internal/ratelimit"
	"opadm/internal/store"
)

const maxBodyBytes = 16 * 1024 // 16 KB: form için fazlasıyla yeter, şişkin body'yi keser

// App, bağımlılıkları tek struct'ta toplar (mülakatta anlatması kolay olsun diye).
type App struct {
	Store   *store.Store
	Limiter *ratelimit.Limiter
}

func New(s *store.Store, l *ratelimit.Limiter) *App { return &App{Store: s, Limiter: l } }

// request, forma birebir uyar; bilinmeyen alan json.Decoder ile reddedilir.
type request struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Service string `json:"service"`
	Message string `json:"message"`
}

// validate, istemciyle aynı kuralları uygular. İstemciye güvenilmez:
// asıl güvenlik burasıdır. Hatalar alan bazlı döner.
func validate(r *request) map[string]string {
	errs := map[string]string{}
	r.Name = strings.TrimSpace(r.Name)
	if len([]rune(r.Name)) < 2 || len([]rune(r.Name)) > 80 {
		errs["name"] = "Adınız 2-80 karakter olmalı."
	}
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	if len(r.Email) > 254 {
		errs["email"] = "E-posta en fazla 254 karakter olmalı."
	} else if _, err := mail.ParseAddress(r.Email); err != nil || !strings.Contains(r.Email, ".") {
		// mail.ParseAddress makul bir RFC kontrolüdür; tek parça host'u ele.
		errs["email"] = "Geçerli bir e-posta adresi yazın."
	}
	switch r.Service {
	case "backup", "monitoring", "hardening":
	default:
		errs["service"] = "Geçerli bir hizmet seçin."
	}
	r.Message = strings.TrimSpace(r.Message)
	if n := len([]rune(r.Message)); n < 10 || n > 1000 {
		errs["message"] = "Mesajınız 10-1000 karakter olmalı."
	}
	return errs
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// clientIP, proxy başlıklarını okur. Cloudflare Tunnel gibi güvenilir
// proxy varsayımına dayanır; sahte başlık riski README'de not edildi.
// Yalnızca rate limit + ip_hash içindir, kimlik doğrulama için kullanılmaz.
func clientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, _ := strings.Cut(xff, ","); strings.TrimSpace(first) != "" {
			return strings.TrimSpace(first)
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

// Create, POST /api/requests. Başarı SADECE INSERT commit edildikten
// sonra 201 ile bildirilir; DB hatasında 500 + genel mesaj döner.
func (a *App) Create(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{
			"ok": false, "error": "Content-Type application/json olmalı.",
		})
		return
	}
	ip := clientIP(r)
	if !a.Limiter.Allow(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"ok": false, "error": "Çok fazla istek gönderdiniz. Bir dakika sonra tekrar deneyin.",
		})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req request
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // bilinmeyen alan = 400, şema kaymasını erken yakalar
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "error": "Geçersiz istek gövdesi. Tüm alanları kontrol edip tekrar deneyin.",
		})
		return
	}
	if errs := validate(&req); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "errors": errs,
		})
		return
	}
	if _, err := a.Store.Insert(req.Name, req.Email, req.Service, req.Message, hashIP(ip)); err != nil {
		// İç detay sızdırılmaz; log'a yazılır, istemciye genel mesaj.
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"ok": false, "error": "Kayıt şu anda alınamadı. Lütfen daha sonra tekrar deneyin.",
		})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

// Health, GET /healthz. DB'ye ping atar; DB kapalıysa 500 verir.
func (a *App) Health(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.Ping(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
