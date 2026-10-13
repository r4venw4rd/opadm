package application

import (
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
