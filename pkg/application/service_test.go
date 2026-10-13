package application

import (
	"errors"
	"testing"
	"time"

	"opadm/pkg/domain"
	"opadm/pkg/infrastructure/ratelimit"
)

type fakeRepo struct {
	rows []domain.Request
	fail bool
	// üyelik
	users    map[string]fakeUser // email -> kayıt
	sessions map[string]string   // tokenHash -> email
}

type fakeUser struct {
	id   int64
	hash string
}

func (f *fakeRepo) usersOf() map[string]fakeUser {
	if f.users == nil {
		f.users = map[string]fakeUser{}
	}
	return f.users
}

func (f *fakeRepo) Insert(name, email, service, message, ipHash string) (int64, error) {
	if f.fail {
		return 0, errors.New("db down")
	}
	f.rows = append(f.rows, domain.Request{Name: name, Email: email, Service: service, Message: message})
	return int64(len(f.rows)), nil
}
func (f *fakeRepo) Ping() error {
	if f.fail {
		return errors.New("db down")
	}
	return nil
}
func (f *fakeRepo) Count() (int, error) {
	if f.fail {
		return 0, errors.New("db down")
	}
	return len(f.rows), nil
}
func (f *fakeRepo) Close() error { return nil }

func (f *fakeRepo) LastRequestAt() (time.Time, bool, error) {
	if f.fail {
		return time.Time{}, false, errors.New("db down")
	}
	if len(f.rows) == 0 {
		return time.Time{}, false, nil
	}
	return time.Now(), true, nil
}

func (f *fakeRepo) DBInfo() (domain.DBIdentity, error) {
	if f.fail {
		return domain.DBIdentity{}, errors.New("db down")
	}
	return domain.DBIdentity{Version: "fake-postgres 0 (test)", Database: "fake"}, nil
}

func (f *fakeRepo) FullCycle() (int64, int, int, error) {
	if f.fail {
		return 0, 0, 0, errors.New("db down")
	}
	// Tur benzetimi: yazıldı, okundu, silindi; sayaç oynamaz.
	return int64(len(f.rows) + 1), len(f.rows), len(f.rows), nil
}

func (f *fakeRepo) SchemaTables() ([]string, error) {
	if f.fail {
		return nil, errors.New("db down")
	}
	return nil, nil // eksik yok
}

func (f *fakeRepo) DBSizeBytes() (int64, error) {
	if f.fail {
		return 0, errors.New("db down")
	}
	return 8192, nil
}

func (f *fakeRepo) PoolStats() (int, int, int) { return 1, 0, 1 }

func (f *fakeRepo) CreateUser(email, hash string) (int64, error) {
	if f.fail {
		return 0, errors.New("db down")
	}
	u := f.usersOf()
	if _, ok := u[email]; ok {
		return 0, errors.New("kayıtlı")
	}
	id := int64(len(u) + 1)
	u[email] = fakeUser{id: id, hash: hash}
	return id, nil
}

func (f *fakeRepo) FindUserByEmail(email string) (int64, string, error) {
	if f.fail {
		return 0, "", errors.New("db down")
	}
	if u, ok := f.usersOf()[email]; ok {
		return u.id, u.hash, nil
	}
	return 0, "", domain.ErrNotFound
}

func (f *fakeRepo) CreateSession(userID int64, tokenHash string, _ time.Time) error {
	if f.fail {
		return errors.New("db down")
	}
	email := ""
	for e, u := range f.usersOf() {
		if u.id == userID {
			email = e
		}
	}
	if f.sessions == nil {
		f.sessions = map[string]string{}
	}
	f.sessions[tokenHash] = email
	return nil
}

func (f *fakeRepo) FindSessionUser(tokenHash string) (string, error) {
	if f.fail {
		return "", errors.New("db down")
	}
	if e, ok := f.sessions[tokenHash]; ok && e != "" {
		return e, nil
	}
	return "", domain.ErrNotFound
}

func (f *fakeRepo) DeleteSession(tokenHash string) error {
	delete(f.sessions, tokenHash)
	return nil
}

func (f *fakeRepo) DeleteExpiredSessions() error { return nil }

func newSvc() (*Service, *fakeRepo) {
	r := &fakeRepo{}
	return New(r, r, time.Now(), "test"), r
}

func TestSubmit(t *testing.T) {
	svc, _ := newSvc()
	id, fieldErrs, err := svc.Submit(domain.Request{
		Name: "Ada", Email: "ADA@B.CO", Service: "backup", Message: "yeterince uzun mesaj",
	}, "hash")
	if err != nil || len(fieldErrs) != 0 || id != 1 {
		t.Fatalf("submit: id=%d errs=%v err=%v", id, fieldErrs, err)
	}

	_, fieldErrs, err = svc.Submit(domain.Request{Name: "A", Email: "x", Service: "y", Message: "z"}, "h")
	if err != nil || len(fieldErrs) == 0 {
		t.Fatalf("bozuk kayıt alan hatası vermeli: %v %v", fieldErrs, err)
	}

	broken, _ := newSvc()
	broken.Repo.(*fakeRepo).fail = true
	_, _, err = broken.Submit(domain.Request{Name: "Ada", Email: "a@b.co", Service: "backup", Message: "yeterince uzun mesaj"}, "h")
	if err == nil {
		t.Fatal("bozuk DB hata vermeli")
	}
}

func TestRunDemo(t *testing.T) {
	svc, _ := newSvc()
	lim := ratelimit.New(10, time.Minute)
	sizeBefore := lim.Size()
	checks := svc.RunDemo(lim, "demo@ornek.com")
	if lim.Size() != sizeBefore {
		t.Fatalf("demo iz bıraktı: önce %d, sonra %d", sizeBefore, lim.Size())
	}
	if len(checks.Checks) != 8 {
		t.Fatalf("%d check, 8 bekleniyordu", len(checks.Checks))
	}
	if checks.RunID == "" {
		t.Fatal("run_id boş olmamalı")
	}
	if checks.Summary.Uptime == "" || checks.Summary.LastRequestAt == "" {
		t.Fatalf("özet eksik: %+v", checks.Summary)
	}
	for _, c := range checks.Checks {
		if !c.Pass {
			t.Fatalf("%s kalmış: %s", c.Name, c.Detail)
		}
		if c.Why == "" || c.Group == "" {
			t.Fatalf("%s: açıklama/grup eksik", c.Name)
		}
	}
}

func TestSnapshot(t *testing.T) {
	svc, repo := newSvc()
	repo.rows = append(repo.rows, domain.Request{})
	snap, err := svc.Snapshot()
	if err != nil || snap.Total != 1 || snap.Host == "" || snap.Go == "" {
		t.Fatalf("snapshot: %+v err=%v", snap, err)
	}
}

func TestAuth(t *testing.T) {
	svc, _ := newSvc()

	if _, _, err := svc.Signup("x", "kisa"); err != nil {
		t.Fatalf("bozuk üyelik err vermemeli: %v", err)
	}
	fieldErrs, conflict, err := svc.Signup("Uye@Ornek.com", "gizli-parola-1")
	if err != nil || len(fieldErrs) != 0 || conflict {
		t.Fatalf("signup: %v %v %v", fieldErrs, conflict, err)
	}
	if _, conflict, _ = svc.Signup("uye@ornek.com", "baska-parola-2"); !conflict {
		t.Fatal("aynı e-posta çakışmalı")
	}

	token, ok, err := svc.Login("UYE@ornek.com", "gizli-parola-1")
	if err != nil || !ok || token == "" {
		t.Fatalf("login: %v %v", ok, err)
	}
	if _, ok, _ = svc.Login("uye@ornek.com", "yanlis"); ok {
		t.Fatal("yanlış parola geçmemeli")
	}
	if _, ok, _ = svc.Login("yok@ornek.com", "gizli-parola-1"); ok {
		t.Fatal("kayıtsız e-posta geçmemeli")
	}

	email, ok, err := svc.Me(token)
	if err != nil || !ok || email != "uye@ornek.com" {
		t.Fatalf("me: %q %v %v", email, ok, err)
	}
	if err := svc.Logout(token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, ok, _ := svc.Me(token); ok {
		t.Fatal("çıkış sonrası oturum geçmemeli")
	}
}
