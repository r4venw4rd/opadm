package domain

import (
	"net/url"
	"strings"
)

// Request, bir hizmet talebidir. Kurallar burada yaşar; ne HTTP ne DB bilir.
type Request struct {
	Name    string
	Email   string
	Service string
	Message string
}

// Normalize, kayda geçmeden önce biçimi düzeltir: kırp + e-posta küçük harf.
func (r *Request) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	r.Message = strings.TrimSpace(r.Message)
}

// Validate, alan bazlı hataları döner. Boş map = geçerli.
// Önce Normalize çağrılmış olmalı (Submit garanti eder).
func (r *Request) Validate() map[string]string {
	errs := map[string]string{}
	if len([]rune(r.Name)) < 2 || len([]rune(r.Name)) > 80 {
		errs["name"] = "Adınız 2-80 karakter olmalı."
	}
	if len(r.Email) > 254 {
		errs["email"] = "E-posta en fazla 254 karakter olmalı."
	} else if msg := CheckEmail(r.Email); msg != "" {
		errs["email"] = msg
	}
	switch r.Service {
	case "backup", "monitoring", "hardening":
	default:
		errs["service"] = "Geçerli bir hizmet seçin."
	}
	if n := len([]rune(r.Message)); n < 10 || n > 1000 {
		errs["message"] = "Mesajınız 10-1000 karakter olmalı."
	}
	return errs
}

// Services, form dropdown'u ve allowlist'in tek kaynağıdır.
var Services = []string{"backup", "monitoring", "hardening"}

// AllowedOrigin, tarayıcı tabanlı CSRF'yi keser: Origin/Referer Host ile
// eşleşmiyorsa false. Başlıksız istek (curl) true döner; tarayıcı POST'ta
// Origin'i her zaman gönderir, o yüzden gerçek CSRF yakalanır.
func AllowedOrigin(host, origin, referer string) bool {
	for _, v := range []string{strings.TrimSpace(origin), strings.TrimSpace(referer)} {
		if v == "" {
			continue
		}
		u, err := url.Parse(v)
		if err != nil || u.Host == "" {
			return false
		}
		if !strings.EqualFold(u.Host, host) {
			return false
		}
	}
	return true
}
