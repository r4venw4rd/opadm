package application

import (
	"os"
	"runtime"
	"strconv"
	"time"

	"opadm/internal/domain"
)

// Service, kullanım senaryolarının (use case) tek giriş noktasıdır.
// HTTP ve DB detayları dışarıda; burada yalnız iş kuralları + orkestrasyon.
type Service struct {
	Repo    domain.Repository
	Started time.Time
}

func New(repo domain.Repository, started time.Time) *Service {
	return &Service{Repo: repo, Started: started}
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
	}, nil
}

// Check, demoluk tek özelliktir: adı, geçti/kaldı ve canlı detayı.
type Check struct {
	ID     string
	Name   string
	Pass   bool
	Detail string
}

// Limiter, demonun kota yakmadan bakması için dar arayüzdür.
type Limiter interface {
	Peek(ip string) (remaining int, limited bool)
	Size() int
	Quota() (max int, window string)
}

// RunDemo, özellikleri canlı sistemde dener; DB'ye yazmaz, kota yakmaz.
// Sahte "her şey yeşil" yok: her check gerçekten koşar.
func (s *Service) RunDemo(lim Limiter) []Check {
	bad := domain.Request{Name: "A", Email: "bozuk", Service: "vpn", Message: "kisa"}
	bad.Normalize()
	good := domain.Request{Name: "Demo Kullanıcı", Email: "Demo@Ornek.com", Service: "backup", Message: "Bu, canlı demo doğrulaması için yeterli uzunlukta bir mesajdır."}
	good.Normalize()

	badErrs, goodErrs := bad.Validate(), good.Validate()
	validationPass := len(badErrs) == 4 && len(goodErrs) == 0

	normPass := good.Email == "demo@ornek.com"

	csrfPass := !domain.AllowedOrigin("ornek.site", "https://kotusu.ornek", "") &&
		domain.AllowedOrigin("ornek.site", "https://ornek.site", "")

	remaining, limited := lim.Peek("demo-probe")
	max, window := lim.Quota()
	limitPass := !limited && remaining == max
	limitDetail := "kota açık"
	if limited {
		limitDetail = "kota dolu (sistem saldırı altında olabilir)"
	}

	dbPass, dbDetail := false, ""
	if err := s.Repo.Ping(); err != nil {
		dbDetail = "ping başarısız"
	} else if n, err := s.Repo.Count(); err != nil {
		dbDetail = "sayaç okunamadı"
	} else {
		dbPass = true
		dbDetail = "ping ok, " + strconv.Itoa(n) + " kayıt"
	}

	return []Check{
		{ID: "validation", Name: "Doğrulama", Pass: validationPass, Detail: "bozuk örnek 4 hata, sağlam örnek 0 hata"},
		{ID: "normalize", Name: "Normalizasyon", Pass: normPass, Detail: "Demo@Ornek.com → demo@ornek.com"},
		{ID: "csrf", Name: "CSRF koruması", Pass: csrfPass, Detail: "yabancı Origin reddedilir, öz Origin geçer"},
		{ID: "ratelimit", Name: "Hız limiti", Pass: limitPass, Detail: limitDetail + " (kota yakılmadı)"},
		{ID: "database", Name: "Veritabanı", Pass: dbPass, Detail: dbDetail},
		{ID: "limiter-state", Name: "Limiter durumu", Pass: true, Detail: strconv.Itoa(lim.Size()) + " izlenen IP / " + window + " pencere"},
	}
}
