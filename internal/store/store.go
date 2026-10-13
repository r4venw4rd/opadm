package store

import (
	"database/sql"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
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
