package postgres

import (
	"database/sql"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"opadm/internal/domain"
)

// Store, taleplerin Postgres (Supabase) kalıcılığıdır.
// Şema repo'daki migration ile kurulur; burada DDL yok.
type Store struct {
	db *sql.DB
}

// Open, connStr ile bağlanır. Supabase pooler (6543, transaction mode)
// server-side prepare'a izin vermez; o yüzden simpleProtocol=1 ile
// simple protocol zorlanır (Vercel'de şart, doğrudan bağlantıda zararsız).
func Open(connStr string, simpleProtocol bool) (*Store, error) {
	cfg, err := pgx.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}
	if simpleProtocol {
		cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	}
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(5) // serverless + pooler dostu küçük havuz
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Insert, doğrulanmış bir talebi tek parametrik sorguyla yazar.
// Hata döner; çağıran bunu 500 yapar, asla başarı göstermez.
func (s *Store) Insert(name, email, service, message, ipHash string) (int64, error) {
	var id int64
	err := s.db.QueryRow(
		`INSERT INTO requests (name, email, service, message, ip_hash)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		name, email, service, message, ipHash,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Ping, /healthz için DB'ye gerçekten dokunur.
func (s *Store) Ping() error { return s.db.Ping() }

// Count, testlerin ve /api/status sayacının kaydı doğrulaması içindir.
func (s *Store) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM requests`).Scan(&n)
	return n, err
}

func (s *Store) Close() error { return s.db.Close() }

// CreateUser, üyelik açar. E-posta çakışırsa unique index hatası döner;
// çağıran (Signup) önce FindUserByEmail ile bakar, yarışta bu hatayı
// da çakışma sayar.
func (s *Store) CreateUser(email, passwordHash string) (int64, error) {
	var id int64
	err := s.db.QueryRow(
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		email, passwordHash,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// FindUserByEmail, giriş için kimlik + parola özetini döner.
// Yoksa domain.ErrNotFound (istemciye "hangisi yanlış" denmez).
func (s *Store) FindUserByEmail(email string) (int64, string, error) {
	var id int64
	var hash string
	err := s.db.QueryRow(
		`SELECT id, password_hash FROM users WHERE lower(email) = lower($1)`,
		email,
	).Scan(&id, &hash)
	if err == sql.ErrNoRows {
		return 0, "", domain.ErrNotFound
	}
	if err != nil {
		return 0, "", err
	}
	return id, hash, nil
}

// CreateSession, oturum açar. Ham jeton değil özeti saklanır.
func (s *Store) CreateSession(userID int64, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, userID, expiresAt,
	)
	return err
}

// FindSessionUser, süresi geçmemiş oturumun e-postasını döner.
func (s *Store) FindSessionUser(tokenHash string) (string, error) {
	var email string
	err := s.db.QueryRow(
		`SELECT u.email FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1 AND s.expires_at > now()`,
		tokenHash,
	).Scan(&email)
	if err == sql.ErrNoRows {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return email, nil
}

// DeleteSession, çıkıştır. Olmayan jeton hata değildir.
func (s *Store) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

// DeleteExpiredSessions, janitor süpürür (saatte bir).
func (s *Store) DeleteExpiredSessions() error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= now()`)
	return err
}
