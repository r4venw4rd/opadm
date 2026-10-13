package store

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store, taleplerin SQLite kalıcılığıdır.
type Store struct {
	db *sql.DB
}

// Open, dizini oluşturur, bağlanır ve şemayı hazırlar.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite yazma kilitlenmesini basit tutar
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS requests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		email TEXT NOT NULL,
		service TEXT NOT NULL,
		message TEXT NOT NULL,
		ip_hash TEXT,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`)
	return err
}

// Insert, doğrulanmış bir talebi tek parametrik sorguyla yazar.
// Tablo yoksa/kapalıysa hata döner; çağıran bunu 500 yapar, asla başarı göstermez.
func (s *Store) Insert(name, email, service, message, ipHash string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO requests (name, email, service, message, ip_hash) VALUES (?, ?, ?, ?, ?)`,
		name, email, service, message, ipHash,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Ping, /healthz için DB'ye gerçekten dokunur.
func (s *Store) Ping() error { return s.db.Ping() }

// Count, testlerin kaydı doğrulaması içindir.
func (s *Store) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM requests`).Scan(&n)
	return n, err
}

func (s *Store) Close() error { return s.db.Close() }
