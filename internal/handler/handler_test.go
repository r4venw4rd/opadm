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
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st, ratelimit.New(max, time.Minute))
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
