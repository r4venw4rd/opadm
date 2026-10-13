package domain

import "time"

// AuthStore, üyelik kalıcılığının sözleşmesidir.
type AuthStore interface {
	CreateUser(email, passwordHash string) (int64, error)
	FindUserByEmail(email string) (id int64, passwordHash string, err error)
	CreateSession(userID int64, tokenHash string, expiresAt time.Time) error
	// FindSessionUser, süresi geçmemiş oturumun e-postasını döner.
	FindSessionUser(tokenHash string) (email string, err error)
	DeleteSession(tokenHash string) error
	DeleteExpiredSessions() error
}
