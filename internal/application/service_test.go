package application

import (
	"errors"
	"testing"
	"time"

	"opadm/internal/domain"
	"opadm/internal/infrastructure/ratelimit"
)

type fakeRepo struct {
	rows []domain.Request
	fail bool
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

func TestSubmit(t *testing.T) {
	svc := New(&fakeRepo{}, time.Now())
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

	broken := New(&fakeRepo{fail: true}, time.Now())
	_, _, err = broken.Submit(domain.Request{Name: "Ada", Email: "a@b.co", Service: "backup", Message: "yeterince uzun mesaj"}, "h")
	if err == nil {
		t.Fatal("bozuk DB hata vermeli")
	}
}

func TestRunDemo(t *testing.T) {
	svc := New(&fakeRepo{}, time.Now())
	lim := ratelimit.New(10, time.Minute)
	checks := svc.RunDemo(lim)
	if len(checks) != 6 {
		t.Fatalf("%d check, 6 bekleniyordu", len(checks))
	}
	for _, c := range checks {
		if !c.Pass {
			t.Fatalf("%s kalmış: %s", c.Name, c.Detail)
		}
	}
}

func TestSnapshot(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, time.Now())
	repo.rows = append(repo.rows, domain.Request{})
	snap, err := svc.Snapshot()
	if err != nil || snap.Total != 1 || snap.Host == "" || snap.Go == "" {
		t.Fatalf("snapshot: %+v err=%v", snap, err)
	}
}
