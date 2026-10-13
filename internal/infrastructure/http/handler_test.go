package http

import (
	"bytes"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"opadm/internal/application"
	"opadm/internal/domain"
	"opadm/internal/infrastructure/ratelimit"
)

// fakeStore, Repository + AuthStore'un bellek içi karşılığıdır. fail=true
// iken tüm çağrılar hata döner (DB arızası simülasyonu). `go test` DB'siz koşar.
type fakeStore struct {
	mu       sync.Mutex
	rows     []fakeRow
	fail     bool
	users    map[string]string // email -> passwordHash
	sessions map[string]string // tokenHash -> email
}

func (f *fakeStore) auth() {
	if f.users == nil {
		f.users = map[string]string{}
	}
	if f.sessions == nil {
		f.sessions = map[string]string{}
	}
}

type fakeRow struct {
	name, email, service, message, ipHash string
}

func (f *fakeStore) Insert(name, email, service, message, ipHash string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return 0, errors.New("db down")
	}
	f.rows = append(f.rows, fakeRow{name, email, service, message, ipHash})
	return int64(len(f.rows)), nil
}

func (f *fakeStore) Ping() error {
	if f.fail {
		return errors.New("db down")
	}
	return nil
}

func (f *fakeStore) Count() (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return 0, errors.New("db down")
	}
	return len(f.rows), nil
}

func (f *fakeStore) Close() error { return nil }

func (f *fakeStore) LastRequestAt() (time.Time, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return time.Time{}, false, errors.New("db down")
	}
	if len(f.rows) == 0 {
		return time.Time{}, false, nil
	}
	return time.Now(), true, nil
}

func (f *fakeStore) DBInfo() (domain.DBIdentity, error) {
	if f.fail {
		return domain.DBIdentity{}, errors.New("db down")
	}
	return domain.DBIdentity{Version: "fake-postgres 0 (test)", Database: "fake", Now: "2026-01-01T00:00:00Z"}, nil
}

func (f *fakeStore) FullCycle() (int64, int, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return 0, 0, 0, errors.New("db down")
	}
	// Tur benzetimi: yazıldı, okundu, silindi; satır eklenmez.
	return int64(len(f.rows) + 1), len(f.rows), len(f.rows), nil
}

func (f *fakeStore) SchemaTables() ([]string, error) {
	if f.fail {
		return nil, errors.New("db down")
	}
	return nil, nil
}

func (f *fakeStore) DBSizeBytes() (int64, error) {
	if f.fail {
		return 0, errors.New("db down")
	}
	return 8192, nil
}

func (f *fakeStore) PoolStats() (int, int, int) { return 1, 0, 1 }

func (f *fakeStore) CreateUser(email, hash string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return 0, errors.New("db down")
	}
	f.auth()
	if _, ok := f.users[email]; ok {
		return 0, errors.New("kayıtlı")
	}
	f.users[email] = hash
	return int64(len(f.users)), nil
}

func (f *fakeStore) FindUserByEmail(email string) (int64, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return 0, "", errors.New("db down")
	}
	f.auth()
	if h, ok := f.users[email]; ok {
		return 1, h, nil
	}
	return 0, "", domain.ErrNotFound
}

func (f *fakeStore) CreateSession(_ int64, tokenHash string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("db down")
	}
	f.auth()
	// Testte tek kullanıcı varsayımı yeterli: e-postayı geri yaz.
	for e := range f.users {
		f.sessions[tokenHash] = e
		break
	}
	return nil
}

func (f *fakeStore) FindSessionUser(tokenHash string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return "", errors.New("db down")
	}
	f.auth()
	if e, ok := f.sessions[tokenHash]; ok {
		return e, nil
	}
	return "", domain.ErrNotFound
}

func (f *fakeStore) DeleteSession(tokenHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.sessions, tokenHash)
	return nil
}

func (f *fakeStore) DeleteExpiredSessions() error { return nil }

func (f *fakeStore) last() fakeRow {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rows[len(f.rows)-1]
}

func newTestApp(t *testing.T, max int) *App {
	t.Helper()
	return newTestAppTrust(t, max, false)
}

func newTestAppTrust(t *testing.T, max int, trustProxy bool) *App {
	t.Helper()
	f := &fakeStore{}
	svc := application.New(f, f, time.Now(), "test")
	return New(svc, ratelimit.New(max, time.Minute), trustProxy)
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
	// Yanıt, kalıcı satırın id'sini taşımalı (başarının kanıtı).
	if id, ok := out["id"].(float64); !ok || id <= 0 {
		t.Fatalf("201 yanıtında pozitif id yok: %s", rec.Body.String())
	}
	n, err := app.Service.Repo.Count()
	if err != nil || n != 1 {
		t.Fatalf("DB'de 1 kayıt bekleniyordu, n=%d err=%v", n, err)
	}
	if got := app.Service.Repo.(*fakeStore).last(); got.email != "test@ornek.com" {
		t.Fatalf("e-posta normalize edilmedi: %q", got.email)
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
	if n, _ := app.Service.Repo.Count(); n != 0 {
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
			if n, _ := app.Service.Repo.Count(); n != 0 {
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
	if n, _ := app.Service.Repo.Count(); n != 0 {
		t.Fatalf("DB boş olmalı, n=%d", n)
	}
}

func TestDBFailure(t *testing.T) {
	app := newTestApp(t, 100)
	app.Service.Repo.(*fakeStore).fail = true // DB arızası simülasyonu
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
	app.Service.Repo.(*fakeStore).fail = true
	rec = httptest.NewRecorder()
	app.Health(rec, req)
	if rec.Code != 500 {
		t.Fatalf("bozuk DB'de healthz 500 bekleniyordu: %d", rec.Code)
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
	if n, _ := app.Service.Repo.Count(); n != 2 {
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
	get := func(cookie string) (int, map[string]any) {
		req := httptest.NewRequest("GET", "/api/status", nil)
		if cookie != "" {
			req.AddCookie(&stdhttp.Cookie{Name: sessionCookie, Value: cookie})
		}
		rec := httptest.NewRecorder()
		app.Status(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	// Girişsiz 401.
	if code, _ := get(""); code != 401 {
		t.Fatalf("girişsiz status: kod = %d, istendi 401", code)
	}
	if rec, _ := doAuth(t, app, "/api/auth/signup", `{"email":"s@e.co","password":"gizli-parola-1"}`, ""); rec.Code != 201 {
		t.Fatalf("signup: %d", rec.Code)
	}
	_, ck := doAuth(t, app, "/api/auth/login", `{"email":"s@e.co","password":"gizli-parola-1"}`, "")
	if ck == "" {
		t.Fatal("login çerez vermedi")
	}
	code, out := get(ck)
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
	if code, out := get(ck); code != 200 || out["requests_total"] != float64(1) {
		t.Fatalf("kayıt sonrası total=1 bekleniyordu: %d %v", code, out)
	}
}

func TestDemo(t *testing.T) {
	app := newTestApp(t, 100)
	// Demo giriş ister: önce üye ol + giriş yap.
	if rec, _ := doAuth(t, app, "/api/auth/signup", `{"email":"d@e.co","password":"gizli-parola-1"}`, ""); rec.Code != 201 {
		t.Fatalf("signup: %d", rec.Code)
	}
	_, ck := doAuth(t, app, "/api/auth/login", `{"email":"d@e.co","password":"gizli-parola-1"}`, "")
	if ck == "" {
		t.Fatal("login çerez vermedi")
	}
	req := httptest.NewRequest("GET", "/api/demo", nil)
	req.AddCookie(&stdhttp.Cookie{Name: sessionCookie, Value: ck})
	rec := httptest.NewRecorder()
	app.Demo(rec, req)
	if rec.Code != 200 {
		t.Fatalf("kod = %d, istendi 200", rec.Code)
	}
	var out struct {
		OK     bool   `json:"ok"`
		RunID  string `json:"run_id"`
		TookMs int64  `json:"took_ms"`
		Sum    struct {
			Uptime        string `json:"uptime"`
			RequestsTotal int    `json:"requests_total"`
			LastRequestAt string `json:"last_request_at"`
			ErrorRatePct  string `json:"error_rate_pct"`
		} `json:"summary"`
		Checks []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Why    string `json:"why"`
			Group  string `json:"group"`
			Pass   bool   `json:"pass"`
			Detail string `json:"detail"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || !out.OK {
		t.Fatalf("demo yanıtı bozuk: %s", rec.Body.String())
	}
	if out.RunID == "" {
		t.Fatal("run_id boş olmamalı (kanıtsız koşu)")
	}
	// İş değeri özeti dolu gelmeli (canlı servis metrikleri).
	if out.Sum.Uptime == "" || out.Sum.LastRequestAt == "" || out.Sum.ErrorRatePct == "" {
		t.Fatalf("özet eksik: %+v", out.Sum)
	}
	if len(out.Checks) != 8 {
		t.Fatalf("%d check, 8 bekleniyordu", len(out.Checks))
	}
	ids := map[string]bool{}
	for _, c := range out.Checks {
		if !c.Pass || c.Detail == "" {
			t.Fatalf("check kalmış/boş: %+v", c)
		}
		if c.Name == "" || c.Why == "" || c.Group == "" {
			t.Fatalf("check adı/açıklaması/grup eksik: %+v", c)
		}
		ids[c.ID] = true
	}
	for _, want := range []string{"validation", "write-cycle", "database", "csrf", "ratelimit", "bcrypt", "session", "version"} {
		if !ids[want] {
			t.Fatalf("%s check'i yok", want)
		}
	}
	// Demo kalıcı kayıt yazmamalı (yazma kanıtı geri alınır).
	if n, _ := app.Service.Repo.Count(); n != 0 {
		t.Fatalf("demo kayıt bırakmış, n=%d", n)
	}
}

func doAuth(t *testing.T, app *App, path, body, cookie string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.AddCookie(&stdhttp.Cookie{Name: sessionCookie, Value: cookie})
	}
	rec := httptest.NewRecorder()
	switch path {
	case "/api/auth/signup":
		app.Signup(rec, req)
	case "/api/auth/login":
		app.Login(rec, req)
	case "/api/auth/logout":
		app.Logout(rec, req)
	default:
		t.Fatalf("bilinmeyen path: %s", path)
	}
	setCookie := ""
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			setCookie = c.Value
		}
	}
	return rec, setCookie
}

func TestAuthFlow(t *testing.T) {
	app := newTestApp(t, 100)
	good := `{"email":"Uye@Ornek.com","password":"gizli-parola-1"}`

	// Bozuk girdi -> 400.
	if rec, _ := doAuth(t, app, "/api/auth/signup", `{"email":"x","password":"k"}`, ""); rec.Code != 400 {
		t.Fatalf("bozuk signup: kod = %d, istendi 400", rec.Code)
	}
	// Üyelik -> 201.
	if rec, _ := doAuth(t, app, "/api/auth/signup", good, ""); rec.Code != 201 {
		t.Fatalf("signup: kod = %d, istendi 201 (%s)", rec.Code, rec.Body.String())
	}
	// Aynı e-posta -> 409.
	if rec, _ := doAuth(t, app, "/api/auth/signup", good, ""); rec.Code != 409 {
		t.Fatalf("çifte signup: kod = %d, istendi 409", rec.Code)
	}
	// Yanlış parola -> 401, çerez yok.
	if rec, ck := doAuth(t, app, "/api/auth/login", `{"email":"uye@ornek.com","password":"yanlis"}`, ""); rec.Code != 401 || ck != "" {
		t.Fatalf("yanlış login: kod = %d ck = %q", rec.Code, ck)
	}
	// Doğru giriş -> 200 + çerez.
	rec, ck := doAuth(t, app, "/api/auth/login", good, "")
	if rec.Code != 200 || ck == "" {
		t.Fatalf("login: kod = %d ck boş mu: %v", rec.Code, ck == "")
	}

	// Me: çerezsiz 401, çerezli 200 + normalize e-posta.
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	r2 := httptest.NewRecorder()
	app.Me(r2, req)
	if r2.Code != 401 {
		t.Fatalf("çerezsiz me: kod = %d, istendi 401", r2.Code)
	}
	req.AddCookie(&stdhttp.Cookie{Name: sessionCookie, Value: ck})
	r2 = httptest.NewRecorder()
	app.Me(r2, req)
	if r2.Code != 200 || !strings.Contains(r2.Body.String(), "uye@ornek.com") {
		t.Fatalf("me: %d %s", r2.Code, r2.Body.String())
	}

	// Demo: girişsiz 401.
	dreq := httptest.NewRequest("GET", "/api/demo", nil)
	drec := httptest.NewRecorder()
	app.Demo(drec, dreq)
	if drec.Code != 401 {
		t.Fatalf("girişsiz demo: kod = %d, istendi 401", drec.Code)
	}
	// Demo: girişli 200.
	dreq.AddCookie(&stdhttp.Cookie{Name: sessionCookie, Value: ck})
	drec = httptest.NewRecorder()
	app.Demo(drec, dreq)
	if drec.Code != 200 {
		t.Fatalf("girişli demo: kod = %d, istendi 200 (%s)", drec.Code, drec.Body.String())
	}

	// Çıkış -> me artık 401.
	if rec, _ := doAuth(t, app, "/api/auth/logout", `{}`, ck); rec.Code != 200 {
		t.Fatalf("logout: kod = %d", rec.Code)
	}
	r2 = httptest.NewRecorder()
	app.Me(r2, req) // req hâlâ çerezli ama oturum düştü
	if r2.Code != 401 {
		t.Fatalf("çıkış sonrası me: kod = %d, istendi 401", r2.Code)
	}
}

// TestRoutes, main.go ile api/index.go'nun paylaştığı rota haritasını kilitler.
func TestRoutes(t *testing.T) {
	app := newTestApp(t, 100)
	mux := stdhttp.NewServeMux()
	app.RegisterRoutes(mux)

	call := func(method, path, body string) int {
		t.Helper()
		var r *stdhttp.Request
		if body == "" {
			r = httptest.NewRequest(method, path, nil)
		} else {
			r = httptest.NewRequest(method, path, strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, r)
		return rec.Code
	}

	if code := call("POST", "/api/requests", validBody()); code != 201 {
		t.Fatalf("POST /api/requests → %d, 201 bekleniyordu", code)
	}
	if code := call("GET", "/healthz", ""); code != 200 {
		t.Fatalf("GET /healthz → %d, 200 bekleniyordu", code)
	}
	if code := call("GET", "/api/status", ""); code != 401 {
		t.Fatalf("GET /api/status (girişsiz) → %d, 401 bekleniyordu", code)
	}
	if code := call("GET", "/api/demo", ""); code != 401 {
		t.Fatalf("GET /api/demo (girişsiz) → %d, 401 bekleniyordu", code)
	}
	if code := call("POST", "/api/auth/signup", `{"email":"r@e.co","password":"gizli-parola-1"}`); code != 201 {
		t.Fatalf("POST /api/auth/signup → %d, 201 bekleniyordu", code)
	}
	if code := call("POST", "/api/auth/login", `{"email":"r@e.co","password":"gizli-parola-1"}`); code != 200 {
		t.Fatalf("POST /api/auth/login → %d, 200 bekleniyordu", code)
	}
	if code := call("GET", "/api/yok", ""); code != 404 {
		t.Fatalf("bilinmeyen rota → %d, 404 bekleniyordu", code)
	}
}

func TestLiveOriginGate(t *testing.T) {
	app := newTestApp(t, 100)
	req := httptest.NewRequest("GET", "/api/live", nil)
	req.Header.Set("Origin", "https://kotusu.ornek")
	rec := httptest.NewRecorder()
	app.Live(rec, req)
	if rec.Code != 403 {
		t.Fatalf("yabancı Origin: kod = %d, istendi 403", rec.Code)
	}
}
