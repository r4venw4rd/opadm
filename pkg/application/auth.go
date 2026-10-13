package application

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"opadm/pkg/domain"
)

// SessionDays, "beni hatırla yok": oturum ömrü 7 gün.
const SessionDays = 7

// hashToken, çerezdeki ham jetonun DB'de tutulan özetidir.
// DB sızsa bile oturumlar ele geçmez (parola özeti mantığı).
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// Signup, üyelik açar. conflict=true ise e-posta kayıtlıdır.
func (s *Service) Signup(email, password string) (fieldErrs map[string]string, conflict bool, err error) {
	email = domain.NormalizeEmail(email)
	fieldErrs = map[string]string{}
	if msg := domain.CheckEmail(email); msg != "" {
		fieldErrs["email"] = msg
	} else if len(email) > 254 {
		fieldErrs["email"] = "E-posta en fazla 254 karakter olmalı."
	}
	if msg := domain.CheckPassword(password); msg != "" {
		fieldErrs["password"] = msg
	}
	if len(fieldErrs) > 0 {
		return fieldErrs, false, nil
	}
	if _, _, err := s.Auth.FindUserByEmail(email); err == nil {
		return nil, true, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, false, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, false, err
	}
	if _, err := s.Auth.CreateUser(email, string(hash)); err != nil {
		return nil, false, err
	}
	return nil, false, nil
}

// Login, doğruysa ham oturum jetonunu döner (çereze yazılır).
// Yanlışta ok=false; hangisinin yanlış olduğu söylenmez.
func (s *Service) Login(email, password string) (token string, ok bool, err error) {
	email = domain.NormalizeEmail(email)
	_, hash, err := s.Auth.FindUserByEmail(email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", false, nil
	}
	// Kullanıcı kimliğini taze çek (yarış güvenliği için ikinci okuma).
	id, _, err := s.Auth.FindUserByEmail(email)
	if err != nil {
		return "", false, err
	}
	token, err = newToken()
	if err != nil {
		return "", false, err
	}
	if err := s.Auth.CreateSession(id, hashToken(token), time.Now().AddDate(0, 0, SessionDays)); err != nil {
		return "", false, err
	}
	return token, true, nil
}

// Me, jetonun sahibinin e-postasını döner.
func (s *Service) Me(token string) (email string, ok bool, err error) {
	email, err = s.Auth.FindSessionUser(hashToken(token))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return email, true, nil
}

// Logout, oturumu düşürür. Olmayan jeton hata değildir.
func (s *Service) Logout(token string) error {
	return s.Auth.DeleteSession(hashToken(token))
}
