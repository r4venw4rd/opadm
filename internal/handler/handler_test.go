package handler

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"opadm/internal/ratelimit"
	"opadm/internal/store"
)

func newTestApp(t *testing.T, max int) *App {
	t.Helper()
	return newTestAppTrust(t, max, false)
}

func newTestAppTrust(t *testing.T, max int, trustProxy bool) *App {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st, ratelimit.New(max, time.Minute), trustProxy, time.Now())
}

func postJSON(t *testing.T, app *App, body string, ctype string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/requests", strings.NewReader(body))
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	rec := httptest.NewRecorder()
	app.Create(rec, req)
	return rec
}

func validBody() string {
	return `{"name":"Test Kullanicisi","email":"Test@Ornek.com","service":"backup","message":"Sunucum icin yedekleme kurulumu istiyorum."}`
}

func TestValidInsert(t *testing.T) {
	app := newTestApp(t, 100)
	rec := postJSON(t, app, validBody(), "application/json")
	if rec.Code != 201 {
		t.Fatalf("kod = %d, istendi 201 (%s)", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out["ok"] != true {
		t.Fatalf("201 yanıtında ok:true yok: %s", rec.Body.String())
	}
	n, err := app.Store.Count()
	if err != nil || n != 1 {
		t.Fatalf("DB'de 1 kayıt bekleniyordu, n=%d err=%v", n, err)
	}
}

func TestInvalidEmail(t *testing.T) {
	app := newTestApp(t, 100)
	rec := postJSON(t, app, `{"name":"Test Adi","email":"gecersiz","service":"backup","message":"Gecerli bir mesaj icerigi burada."}`, "application/json")
	if rec.Code != 400 {
		t.Fatalf("kod = %d, istendi 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "email") {
		t.Fatalf("alan bazlı email hatası yok: %s", rec.Body.String())
	}
	if n, _ := app.Store.Count(); n != 0 {
		t.Fatalf("hatalı kayıtta DB boş olmalı, n=%d", n)
	}
}

func TestFieldLengths(t *testing.T) {
	cases := []struct {
		name, body, field string
	}{
		{"bos ad", `{"name":"","email":"a@b.co","service":"backup","message":"Yeterince uzun mesaj."}`, "name"},
		{"kisa ad", `{"name":"A","email":"a@b.co","service":"backup","message":"Yeterince uzun mesaj."}`, "name"},
		{"uzun ad", `{"name":"` + strings.Repeat("x", 81) + `","email":"a@b.co","service":"backup","message":"Yeterince uzun mesaj."}`, "name"},
		{"kisa mesaj", `{"name":"Test Adi","email":"a@b.co","service":"backup","message":"kisa"}`, "message"},
		{"uzun mesaj", `{"name":"Test Adi","email":"a@b.co","service":"backup","message":"` + strings.Repeat("y", 1001) + `"}`, "message"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app := newTestApp(t, 100)
			rec := postJSON(t, app, c.body, "application/json")
			if rec.Code != 400 {
				t.Fatalf("kod = %d, istendi 400", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), c.field) {
				t.Fatalf("%s alan hatası yok: %s", c.field, rec.Body.String())
			}
			if n, _ := app.Store.Count(); n != 0 {
				t.Fatalf("DB boş olmalı, n=%d", n)
			}
		})
	}
}

func TestInvalidService(t *testing.T) {
	app := newTestApp(t, 100)
	rec := postJSON(t, app, `{"name":"Test Adi","email":"a@b.co","service":"vpn","message":"Yeterince uzun bir mesaj."}`, "application/json")
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "service") {
		t.Fatalf("service 400 bekleniyordu: %d %s", rec.Code, rec.Body.String())
	}
}

func TestBadEnvelope(t *testing.T) {
	app := newTestApp(t, 100)
	// bilinmeyen alan
	rec := postJSON(t, app, `{"name":"Test Adi","email":"a@b.co","service":"backup","message":"Yeterince uzun mesaj.","admin":true}`, "application/json")
	if rec.Code != 400 {
		t.Fatalf("bilinmeyen alan: kod = %d, istendi 400", rec.Code)
	}
	// yanlış Content-Type
	rec = postJSON(t, app, validBody(), "text/plain")
	if rec.Code != 415 {
		t.Fatalf("yanlış Content-Type: kod = %d, istendi 415", rec.Code)
	}
	// aşırı büyük body
	big := `{"name":"Test Adi","email":"a@b.co","service":"backup","message":"` + strings.Repeat("z", 20*1024) + `"}`
	rec = postJSON(t, app, big, "application/json")
	if rec.Code < 400 || rec.Code > 499 {
		t.Fatalf("büyük body: kod = %d, 4xx istendi", rec.Code)
	}
	if n, _ := app.Store.Count(); n != 0 {
		t.Fatalf("DB boş olmalı, n=%d", n)
	}
}

func TestDBFailure(t *testing.T) {
	app := newTestApp(t, 100)
	app.Store.Close() // DB'yi boz: INSERT imkansız
	rec := postJSON(t, app, validBody(), "application/json")
	if rec.Code != 500 {
		t.Fatalf("kod = %d, istendi 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("500 yanıtında başarı işareti olmamalı: %s", rec.Body.String())
	}
}

func TestRateLimit(t *testing.T) {
	app := newTestApp(t, 2) // 2 istek/dakika
	for i := 0; i < 2; i++ {
		if rec := postJSON(t, app, validBody(), "application/json"); rec.Code != 201 {
			t.Fatalf("istek %d: kod = %d, istendi 201", i+1, rec.Code)
		}
	}
	if rec := postJSON(t, app, validBody(), "application/json"); rec.Code != 429 {
		t.Fatalf("limit aşımı: kod = %d, istendi 429", rec.Code)
	}
}

func TestHealthz(t *testing.T) {
	app := newTestApp(t, 100)
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	app.Health(rec, req)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(`"ok":true`)) {
		t.Fatalf("healthz 200 bekleniyordu: %d %s", rec.Code, rec.Body.String())
	}
	app.Store.Close()
	rec = httptest.NewRecorder()
	app.Health(rec, req)
	if rec.Code != 500 {
		t.Fatalf("kapalı DB'de healthz 500 bekleniyordu: %d", rec.Code)
	}
}

func postWithOrigin(t *testing.T, app *App, origin string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/requests", strings.NewReader(validBody()))
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	app.Create(rec, req)
	return rec
}

func TestCSRFOrigin(t *testing.T) {
	app := newTestApp(t, 100)
	// Saldırgan origin'i -> 403, kayıt yok.
	rec := postWithOrigin(t, app, "https://kotusu.ornek")
	if rec.Code != 403 {
		t.Fatalf("yabancı Origin: kod = %d, istendi 403", rec.Code)
	}
	// Origin yok (curl) -> 201.
	if rec := postWithOrigin(t, app, ""); rec.Code != 201 {
		t.Fatalf("originsiz: kod = %d, istendi 201", rec.Code)
	}
	// Eşleşen Origin -> 201. httptest isteği example.com host'undadır.
	if rec := postWithOrigin(t, app, "http://example.com"); rec.Code != 201 {
		t.Fatalf("eşleşen Origin: kod = %d, istendi 201", rec.Code)
	}
	if n, _ := app.Store.Count(); n != 2 {
		t.Fatalf("DB'de 2 kayıt bekleniyordu, n=%d", n)
	}
}

func TestTrustProxyDefaultOff(t *testing.T) {
	app := newTestApp(t, 100) // TrustProxy kapalı
	req := httptest.NewRequest("POST", "/api/requests", strings.NewReader(validBody()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "6.6.6.6")
	rec := httptest.NewRecorder()
	app.Create(rec, req)
	if rec.Code != 201 {
		t.Fatalf("kod = %d, istendi 201", rec.Code)
	}
	// Başlık yoksayıldı: hash, RemoteAddr'ın (192.0.2.1) olmalı.
	if got := clientIP(req, false); got != "192.0.2.1" {
		t.Fatalf("clientIP = %q, istendi 192.0.2.1", got)
	}
	if got := clientIP(req, true); got != "6.6.6.6" {
		t.Fatalf("trust açıkken clientIP = %q, istendi 6.6.6.6", got)
	}
}

func TestStatus(t *testing.T) {
	app := newTestApp(t, 100)
	get := func() (int, map[string]any) {
		req := httptest.NewRequest("GET", "/api/status", nil)
		rec := httptest.NewRecorder()
		app.Status(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	code, out := get()
	if code != 200 || out["ok"] != true || out["requests_total"] != float64(0) {
		t.Fatalf("boş status bekleniyordu: %d %v", code, out)
	}
	for _, k := range []string{"uptime_s", "time", "host", "in_docker", "go", "goroutines", "cpu", "mem_alloc_mb"} {
		if _, ok := out[k]; !ok {
			t.Fatalf("%s alanı yok: %v", k, out)
		}
	}
	if h, _ := out["host"].(string); h == "" {
		t.Fatalf("host boş olmamalı: %v", out)
	}
	if rec := postJSON(t, app, validBody(), "application/json"); rec.Code != 201 {
		t.Fatalf("kayıt 201 bekleniyordu: %d", rec.Code)
	}
	if code, out := get(); code != 200 || out["requests_total"] != float64(1) {
		t.Fatalf("kayıt sonrası total=1 bekleniyordu: %d %v", code, out)
	}
}
