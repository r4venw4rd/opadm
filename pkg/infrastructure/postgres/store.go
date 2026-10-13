package postgres

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"opadm/pkg/domain"
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

// LastRequestAt, en son kalıcı kaydın zamanıdır (yoksa ok=false, hata yok).
func (s *Store) LastRequestAt() (time.Time, bool, error) {
	var t time.Time
	err := s.db.QueryRow(`SELECT created_at FROM requests ORDER BY id DESC LIMIT 1`).Scan(&t)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return t, true, nil
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

// DBInfo, SELECT ile DB kimliğini okur + gecikmeyi ölçer.
// Demo "gerçekten bağlı" kanıtıdır; sahteye yazılamaz.
func (s *Store) DBInfo() (domain.DBIdentity, error) {
	start := time.Now()
	var v, db, now string
	err := s.db.QueryRow(`SELECT version(), current_database(), now()::text`).Scan(&v, &db, &now)
	if err != nil {
		return domain.DBIdentity{}, err
	}
	return domain.DBIdentity{Version: v, Database: db, Now: now, PingUs: time.Since(start).Microseconds()}, nil
}

// FullCycle, yaz → geri oku → sil. Her adım doğrulanır; sayaç başa döner.
func (s *Store) FullCycle() (int64, int, int, error) {
	var n0 int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM requests`).Scan(&n0); err != nil {
		return 0, 0, 0, err
	}
	var id int64
	var email string
	err := s.db.QueryRow(
		`INSERT INTO requests (name, email, service, message, ip_hash)
		 VALUES ('__demo__', 'demo@ornek.com', 'backup', 'yazma kaniti', 'demo') RETURNING id`,
	).Scan(&id)
	if err != nil || id <= 0 {
		return 0, n0, n0, err
	}
	if err := s.db.QueryRow(`SELECT email FROM requests WHERE id = $1`, id).Scan(&email); err != nil {
		return 0, n0, n0, err
	}
	if email != "demo@ornek.com" {
		return 0, n0, n0, errors.New("geri okuma tutmadı")
	}
	if _, err := s.db.Exec(`DELETE FROM requests WHERE id = $1`, id); err != nil {
		return 0, n0, n0, err
	}
	var n1 int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM requests`).Scan(&n1); err != nil {
		return 0, n0, n0, err
	}
	return id, n0, n1, nil
}

// SchemaTables, beklenen tabloların eksiklerini döner.
func (s *Store) SchemaTables() ([]string, error) {
	want := []string{"requests", "users", "sessions"}
	rows, err := s.db.Query(
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_name = ANY ($1)`, want)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	have := map[string]bool{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		have[t] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var missing []string
	for _, t := range want {
		if !have[t] {
			missing = append(missing, t)
		}
	}
	return missing, nil
}

// DBSizeBytes, veritabanı boyutudur.
func (s *Store) DBSizeBytes() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT pg_database_size(current_database())`).Scan(&n)
	return n, err
}

// PoolStats, havuzun canlı sayılarıdır.
func (s *Store) PoolStats() (int, int, int) {
	st := s.db.Stats()
	return st.OpenConnections, st.InUse, st.Idle
}
