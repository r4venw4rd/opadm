package application

import (
	"errors"
	"testing"
	"time"

	"opadm/internal/domain"
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

func newSvc() (*Service, *fakeRepo) {
	r := &fakeRepo{}
	return New(r, time.Now()), r
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
