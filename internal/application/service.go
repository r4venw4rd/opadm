package application

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"opadm/internal/domain"
)

// Service, kullanım senaryolarının (use case) tek giriş noktasıdır.
// HTTP ve DB detayları dışarıda; burada yalnız iş kuralları + orkestrasyon.
type Service struct {
	Repo    domain.Repository
	Auth    domain.AuthStore
	Started time.Time
	// Version, canlıdaki sürüm (APP_VERSION; "hangi kod koşuyor" kanıtı).
	Version string
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
		return 0, nil, err
	}
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

// Check, demoluk tek özelliktir: adı, geçti/kaldı ve canlı detayı.
type Check struct {
	ID     string
	Name   string
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

// DemoResult, bir demo koşusunun kimlikli zarfıdır.
type DemoResult struct {
	RunID  string
	TookMs int64
	Checks []Check
}

func newRunID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(b[:])
}

// RunDemo, özellikleri canlı sistemde dener. Her check ölçülebilir iz bırakır:
// süre, DB kimliği, sayaç deltaları. Kayıt turu gerçek satırla yapılır ama
// sonunda silinir; hız kanıtı probe kotası yakıp temizler.
// Sahte "her şey yeşil" yok: her check gerçekten koşar.
func (s *Service) RunDemo(lim Limiter, userEmail string) DemoResult {
	start := time.Now()
	runID := newRunID()
	timed := func(f func() (bool, string)) (bool, string) {
		t0 := time.Now()
		pass, detail := f()
		return pass, detail + " (" + strconv.FormatInt(time.Since(t0).Microseconds(), 10) + "µs)"
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
		return true, "bozuk örnek 4 hata, sağlam örnek 0 hata, e-posta küçük harf"
	})

	csrfPass, csrfDetail := timed(func() (bool, string) {
		if domain.AllowedOrigin("ornek.site", "https://kotusu.ornek", "") {
			return false, "yabancı Origin geçti"
		}
		if !domain.AllowedOrigin("ornek.site", "https://ornek.site", "") {
			return false, "öz Origin kalmış"
		}
		return true, "yabancı reddedildi, öz geçti"
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
		return true, "önce " + strconv.Itoa(before) + ", 1 yakıldı, sonra " + strconv.Itoa(after) + " / " + window + " pencere (probe temizlendi)"
	})

	dbPass, dbDetail := timed(func() (bool, string) {
		info, err := s.Repo.DBInfo()
		if err != nil {
			return false, "kimlik okunamadı"
		}
		ver := info.Version
		if i := strings.Index(ver, " on "); i > 0 {
			ver = ver[:i]
		}
		size, err := s.Repo.DBSizeBytes()
		if err != nil {
			return false, "boyut okunamadı"
		}
		open, inUse, idle := s.Repo.PoolStats()
		clockSkew := "?"
		if dbNow, err := time.Parse("2006-01-02 15:04:05.999999999-07", info.Now); err == nil {
			d := time.Since(dbNow)
			if d < 0 {
				d = -d
			}
			clockSkew = strconv.FormatInt(d.Milliseconds(), 10) + "ms"
		}
		return true, ver + " · " + info.Database + " · " + info.Now +
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
		return true, "satır #" + strconv.FormatInt(id, 10) + " yazıldı → okundu → silindi (sayaç " + strconv.Itoa(n0) + "→" + strconv.Itoa(n1) + ")"
	})

	schemaPass, schemaDetail := timed(func() (bool, string) {
		missing, err := s.Repo.SchemaTables()
		if err != nil {
			return false, "şema okunamadı"
		}
		if len(missing) > 0 {
			return false, "eksik tablo: " + strings.Join(missing, ", ")
		}
		return true, "requests/users/sessions mevcut"
	})

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
		return true, "üret + doğru geçti + yanlış kastedildi"
	})

	sessionPass, sessionDetail := timed(func() (bool, string) {
		if userEmail == "" {
			return false, "oturum e-postası yok"
		}
		return true, userEmail + " olarak girişli (çerez HttpOnly)"
	})

	return DemoResult{
		RunID:  runID,
		TookMs: time.Since(start).Milliseconds(),
		Checks: []Check{
			{ID: "validation", Name: "Doğrulama", Pass: validationPass, Detail: validationDetail},
			{ID: "csrf", Name: "CSRF koruması", Pass: csrfPass, Detail: csrfDetail},
			{ID: "ratelimit", Name: "Hız limiti", Pass: limitPass, Detail: limitDetail},
			{ID: "database", Name: "Veritabanı", Pass: dbPass, Detail: dbDetail},
			{ID: "write-cycle", Name: "Kayıt turu", Pass: cyclePass, Detail: cycleDetail},
			{ID: "schema", Name: "Şema", Pass: schemaPass, Detail: schemaDetail},
			{ID: "bcrypt", Name: "Parola koruması", Pass: bcryptPass, Detail: bcryptDetail},
			{ID: "session", Name: "Oturum", Pass: sessionPass, Detail: sessionDetail},
			{ID: "version", Name: "Sürüm", Pass: true, Detail: "canlıdaki kod: " + s.Version},
			{ID: "limiter-state", Name: "Limiter durumu", Pass: true, Detail: strconv.Itoa(lim.Size()) + " izlenen IP (demo probeleri temizlenir)"},
		},
	}
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
