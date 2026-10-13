package postgres

import (
	"os"
	"testing"
)

// TestIntegration, gerçek Postgres'e karşı koşar. TEST_DATABASE_URL yoksa
// atlanır; CI'da değil, Supabase'e karşı elle doğrulanır:
//
//	TEST_DATABASE_URL='postgres://...' SIMPLE_PROTOCOL=1 go test ./internal/store/ -run Integration -v
func TestIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL yok, atlanıyor")
	}
	simple := os.Getenv("SIMPLE_PROTOCOL") == "1"
	s, err := Open(url, simple)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	if err := s.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	id, err := s.Insert("Entegrasyon Test", "int@ornek.com", "monitoring", "Entegrasyon deneme kaydi.", "abc123")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id <= 0 {
		t.Fatalf("id = %d, pozitif bekleniyordu", id)
	}
	n, err := s.Count()
	if err != nil || n < 1 {
		t.Fatalf("count = %d err = %v", n, err)
	}
}
