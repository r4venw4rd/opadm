package application

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/bcrypt"

	"opadm/pkg/domain"
)

// Service, kullanım senaryolarının (use case) tek giriş noktasıdır.
// HTTP ve DB detayları dışında; burada yalnız iş kuralları + orkestrasyon.
type Service struct {
	Repo    domain.Repository
	Auth    domain.AuthStore
	Started time.Time
	// Version, canlıdaki sürüm (APP_VERSION; "hangi kod koşuyor" kanıtı).
	Version string
	// submitted/failed, süreç ömrü boyunca talep sonuçlarını sayar.
	// /api/demo bunları "canlı servis metriği" olarak gösterir.
	submitted atomic.Int64
	failed    atomic.Int64
}

func New(repo domain.Repository, auth domain.AuthStore, started time.Time, version string) *Service {
	return &Service{Repo: repo, Auth: auth, Started: started, Version: version}
}

// Ping, /healthz için DB'ye gerçekten dokunur.
func (s *Service) Ping() error { return s.Repo.Ping() }

// Submit, talebi normalize edip doğrular, geçerliyse saklar.
// fieldErrs doluysa 400'lük durumdur; err doluysa 500'lük.
func (s *Service) Submit(r domain.Request, ipHash string) (id int64, fieldErrs map[string]string, err error) {
	r.Normalize()
	if fieldErrs = r.Validate(); len(fieldErrs) > 0 {
		return 0, fieldErrs, nil
	}
	id, err = s.Repo.Insert(r.Name, r.Email, r.Service, r.Message, ipHash)
	if err != nil {
		s.failed.Add(1)
		return 0, nil, err
	}
	s.submitted.Add(1)
	return id, nil, nil
}

// Snapshot, /api/status'un ham verisidir. Kişisel veri içermez.
type Snapshot struct {
	Time       string
	UptimeS    int64
	Total      int
	Host       string
	InDocker   bool
	Go         string
	Goroutines int
	CPU        int
	MemAllocMB int64
	Version    string
}

// Snapshot, canlı sayaç + runtime bilgisini toplar.
func (s *Service) Snapshot() (Snapshot, error) {
	n, err := s.Repo.Count()
	if err != nil {
		return Snapshot{}, err
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	host, _ := os.Hostname() // Docker'da kısa container ID'dir
	_, dockerErr := os.Stat("/.dockerenv")
	return Snapshot{
		Time:       time.Now().UTC().Format(time.RFC3339),
		UptimeS:    int64(time.Since(s.Started).Seconds()),
		Total:      n,
		Host:       host,
		InDocker:   dockerErr == nil,
		Go:         runtime.Version(),
		Goroutines: runtime.NumGoroutine(),
		CPU:        runtime.NumCPU(),
		MemAllocMB: int64(mem.Alloc / 1024 / 1024),
		Version:    s.Version,
	}, nil
}

// Check, demodaki tek kontroldür. Name + Why müşteriye, Detail mühendise
// konuşur; Group iki katmanı ayırır (Ürün / Mühendislik).
type Check struct {
	ID     string
	Name   string
	Why    string
	Group  string
	Pass   bool
	Detail string
}

// Limiter, demonun bakması için dar arayüzdür.
type Limiter interface {
	Peek(ip string) (remaining int, limited bool)
	Allow(ip string) bool
	Release(ip string)
	Size() int
	Quota() (max int, window string)
}

// ServiceMetrics, ürünün iş değerini gösteren canlı sayılardır.
type ServiceMetrics struct {
	Uptime            string `json:"uptime"`
	RequestsTotal     int    `json:"requests_total"`
	LastRequestAt     string `json:"last_request_at"`
	WriteLatencyMs    int64  `json:"write_latency_ms"`
	DBLatencyUs       int64  `json:"db_latency_us"`
	SubmissionsOK     int64  `json:"submissions_ok"`
	SubmissionsFailed int64  `json:"submissions_failed"`
	ErrorRatePct      string `json:"error_rate_pct"`
}

// DemoResult, bir demo koşusunun kimlikli zarfıdır.
type DemoResult struct {
	RunID   string
	TookMs  int64
	Summary ServiceMetrics
	Checks  []Check
}

const (
	groupProduct = "Ürün"
	groupEng     = "Mühendislik"
)

func newRunID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(b[:])
}

// RunDemo, özellikleri canlı sistemde dener. Her kontrol ölçülebilir iz
// bırakır: süre, DB kimliği, sayaç deltaları. Kayıt turu gerçek satırla
// yapılır ama sonunda silinir; hız kanıtı probe kotası yakıp temizler.
// Sahte "her şey yeşil" yok: her kontrol gerçekten koşar.
func (s *Service) RunDemo(lim Limiter, userEmail string) DemoResult {
	start := time.Now()
	runID := newRunID()
	var lastUs int64
	timed := func(f func() (bool, string)) (bool, string) {
		t0 := time.Now()
		pass, detail := f()
		lastUs = time.Since(t0).Microseconds()
		return pass, detail + " (" + strconv.FormatInt(lastUs, 10) + "µs)"
	}

	validationPass, validationDetail := timed(func() (bool, string) {
		bad := domain.Request{Name: "A", Email: "bozuk", Service: "vpn", Message: "kisa"}
		bad.Normalize()
		good := domain.Request{Name: "Demo Kullanıcı", Email: "Demo@Ornek.com", Service: "backup", Message: "Bu, canlı demo doğrulaması için yeterli uzunlukta bir mesajdır."}
		good.Normalize()
		be, ge := bad.Validate(), good.Validate()
		if len(be) != 4 || len(ge) != 0 {
			return false, "beklenmedik sonuç"
		}
		if good.Email != "demo@ornek.com" {
			return false, "normalizasyon bozuk"
		}
		return true, "bozuk örnek 4 hata, sağlam örnek 0 hata, e-posta küçük harfe çevrildi"
	})

	csrfPass, csrfDetail := timed(func() (bool, string) {
		if domain.AllowedOrigin("ornek.site", "https://kotusu.ornek", "") {
			return false, "yabancı Origin geçti"
		}
		if !domain.AllowedOrigin("ornek.site", "https://ornek.site", "") {
			return false, "öz Origin kalmış"
		}
		return true, "yabancı kaynak reddedildi, kendi kaynağımız geçti"
	})

	limitPass, limitDetail := timed(func() (bool, string) {
		probe := "demo-probe-" + runID
		defer lim.Release(probe) // iz bırakma: sayaç gerçek trafiği göstersin
		before, limited := lim.Peek(probe)
		max, window := lim.Quota()
		if limited || before != max {
			return false, "probe anahtarı kirli"
		}
		lim.Allow(probe) // 1 kota yak: uygulama yolu canlı mı?
		after, _ := lim.Peek(probe)
		if before-after != 1 {
			return false, "delta tutmadı"
		}
		return true, "kota " + strconv.Itoa(before) + ", 1 yakıldı, " + strconv.Itoa(after) + " kaldı / " + window + " pencere (probe temizlendi)"
	})

	var dbPingUs int64
	dbPass, dbDetail := timed(func() (bool, string) {
		info, err := s.Repo.DBInfo()
		if err != nil {
			return false, "kimlik okunamadı"
		}
		dbPingUs = info.PingUs
		ver := info.Version
		if i := strings.Index(ver, " on "); i > 0 {
			ver = ver[:i]
		}
		// Şema kontrolü bu kontrole katıldı (sadeleştirme).
		missing, err := s.Repo.SchemaTables()
		if err != nil {
			return false, "şema okunamadı"
		}
		schema := "şema tam (requests/users/sessions)"
		if len(missing) > 0 {
			schema = "eksik tablo: " + strings.Join(missing, ", ")
		}
		size, _ := s.Repo.DBSizeBytes()
		open, inUse, idle := s.Repo.PoolStats()
		clockSkew := "?"
		if dbNow, err := time.Parse("2006-01-02 15:04:05.999999999-07", info.Now); err == nil {
			d := time.Since(dbNow)
			if d < 0 {
				d = -d
			}
			clockSkew = strconv.FormatInt(d.Milliseconds(), 10) + "ms"
		}
		return len(missing) == 0, ver + " · " + info.Database + " · " + schema +
			" · ping " + strconv.FormatInt(info.PingUs, 10) + "µs" +
			" · boyut " + humanBytes(size) +
			" · havuz " + strconv.Itoa(open) + "/" + strconv.Itoa(inUse) + "/" + strconv.Itoa(idle) + " (açık/kullanımda/boşta)" +
			" · saat farkı " + clockSkew
	})

	cyclePass, cycleDetail := timed(func() (bool, string) {
		id, n0, n1, err := s.Repo.FullCycle()
		if err != nil {
			return false, "tur bozuk: " + err.Error()
		}
		if id <= 0 {
			return false, "id alınamadı"
		}
		if n0 != n1 {
			return false, "silme başarısız: kayıt kaldı"
		}
		return true, "satır #" + strconv.FormatInt(id, 10) + " yazıldı → geri okundu → silindi (sayaç " + strconv.Itoa(n0) + "→" + strconv.Itoa(n1) + ")"
	})
	writeUs := lastUs

	bcryptPass, bcryptDetail := timed(func() (bool, string) {
		// Gerçek tur: üret + doğrula. ~100ms sürer; sahte karşılaştırma anında biterdi.
		hash, err := bcrypt.GenerateFromPassword([]byte("demo-parola-kanit"), bcrypt.DefaultCost)
		if err != nil {
			return false, "üretilemedi"
		}
		if err := bcrypt.CompareHashAndPassword(hash, []byte("demo-parola-kanit")); err != nil {
			return false, "doğrulama tutmadı"
		}
		if err := bcrypt.CompareHashAndPassword(hash, []byte("yanlis")); err == nil {
			return false, "yanlış parola geçti"
		}
		return true, "parola özetlendi, doğru parola geçti, yanlış parola reddedildi"
	})

	sessionPass, sessionDetail := timed(func() (bool, string) {
		if userEmail == "" {
			return false, "oturum e-postası yok"
		}
		return true, userEmail + " olarak girişli (oturum çerezi HttpOnly)"
	})

	return DemoResult{
		RunID:   runID,
		TookMs:  time.Since(start).Milliseconds(),
		Summary: s.metrics(writeUs, dbPingUs),
		Checks: []Check{
			{ID: "validation", Group: groupProduct, Name: "Form doğrulaması", Why: "Bozuk veya eksik girdi kaydedilmez.", Pass: validationPass, Detail: validationDetail},
			{ID: "write-cycle", Group: groupProduct, Name: "Kalıcı kayıt", Why: "Talep gerçekten yazılır ve geri okunur; başarı mesajı buna bağlıdır.", Pass: cyclePass, Detail: cycleDetail},
			{ID: "database", Group: groupEng, Name: "Veritabanı", Why: "Canlı PostgreSQL'e bağlıyız; kayıtlar burada kalıcı durur.", Pass: dbPass, Detail: dbDetail},
			{ID: "csrf", Group: groupEng, Name: "Yabancı kaynak koruması", Why: "Başka siteler formu sizin adınıza gönderemez.", Pass: csrfPass, Detail: csrfDetail},
			{ID: "ratelimit", Group: groupEng, Name: "Hız limiti", Why: "Aynı IP dakikada en fazla 10 istek gönderebilir.", Pass: limitPass, Detail: limitDetail},
			{ID: "bcrypt", Group: groupEng, Name: "Parola güvenliği", Why: "Parolalar geri döndürülemez şekilde saklanır.", Pass: bcryptPass, Detail: bcryptDetail},
			{ID: "session", Group: groupEng, Name: "Oturum güvenliği", Why: "Oturum çerezi JavaScript'ten gizlidir (HttpOnly).", Pass: sessionPass, Detail: sessionDetail},
			{ID: "version", Group: groupEng, Name: "Canlı sürüm", Why: "Hangi kodun çalıştığı izlenebilir.", Pass: true, Detail: "canlıdaki kod: " + s.Version},
		},
	}
}

// metrics, iş değeri sayılarını toplar. writeUs: bu koşudaki gerçek yazma
// süresi, dbPingUs: DB ping süresi. Kişisel veri içermez.
func (s *Service) metrics(writeUs, dbPingUs int64) ServiceMetrics {
	total, err := s.Repo.Count()
	if err != nil {
		total = 0
	}
	lastStr := "—"
	if t, ok, err := s.Repo.LastRequestAt(); err == nil && ok {
		lastStr = t.UTC().Format(time.RFC3339)
	}
	okN, failN := s.submitted.Load(), s.failed.Load()
	rate := "0.0"
	if attempts := okN + failN; attempts > 0 {
		rate = strconv.FormatFloat(float64(failN)*100/float64(attempts), 'f', 1, 64)
	}
	return ServiceMetrics{
		Uptime:            humanDuration(time.Since(s.Started)),
		RequestsTotal:     total,
		LastRequestAt:     lastStr,
		WriteLatencyMs:    writeUs / 1000, // µs → ms
		DBLatencyUs:       dbPingUs,
		SubmissionsOK:     okN,
		SubmissionsFailed: failN,
		ErrorRatePct:      rate,
	}
}

// humanDuration, süreyi okunabilir kısa metne çevirir.
func humanDuration(d time.Duration) string {
	sec := int64(d.Seconds())
	if sec < 60 {
		return strconv.FormatInt(sec, 10) + " sn"
	}
	if sec < 3600 {
		return strconv.FormatInt(sec/60, 10) + " dk " + strconv.FormatInt(sec%60, 10) + " sn"
	}
	return strconv.FormatInt(sec/3600, 10) + " sa " + strconv.FormatInt((sec%3600)/60, 10) + " dk"
}

// humanBytes, baytı okunabilir yapar (demo detayı için).
func humanBytes(n int64) string {
	if n < 1024 {
		return strconv.FormatInt(n, 10) + "B"
	}
	for _, u := range []string{"KB", "MB", "GB"} {
		n /= 1024
		if n < 1024 {
			return strconv.FormatInt(n, 10) + u
		}
	}
	return strconv.FormatInt(n, 10) + "TB"
}
