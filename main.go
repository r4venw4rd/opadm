package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"opadm/internal/handler"
	"opadm/internal/ratelimit"
	"opadm/internal/store"
)

//go:embed web
var webFS embed.FS

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// securityHeaders, her yanıta temel başlıkları ekler.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CSP sıkı: her şey same-origin; form fetch'i de same-origin.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// recoverPanic, handler paniğini 500'e çevirir; süreç ayakta kalır.
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func main() {
	port := env("PORT", "8080")
	dbPath := env("DB_PATH", "./data/requests.db")
	// TRUST_PROXY=1 yalnızca başlıkları ezen bir proxy (Cloudflare Tunnel)
	// arkasındayken açılır; yoksa sahte X-Forwarded-For'a kanılır.
	trustProxy := env("TRUST_PROXY", "0") == "1"

	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer st.Close()

	// 10 istek/dakika/IP; restart'ta sıfırlanır, tek instance (bkz. README).
	limiter := ratelimit.New(10, time.Minute)
	never := make(chan struct{}) // süreç ömrü: janitor kapanmaz
	go limiter.StartJanitor(5*time.Minute, never)
	app := handler.New(st, limiter, trustProxy)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/requests", app.Create)
	mux.HandleFunc("GET /healthz", app.Health)

	// Gömülü statik dosyalar (tek binary).
	web, _ := fs.Sub(webFS, "web")
	mux.Handle("GET /", http.FileServerFS(web))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           recoverPanic(securityHeaders(mux)),
		ReadHeaderTimeout: 5 * time.Second,  // Slowloris'e karşı
		ReadTimeout:       10 * time.Second, // gövde okuma üst sınırı
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("opadm listening on :%s (db=%s)", port, dbPath)
	log.Fatal(srv.ListenAndServe())
}
