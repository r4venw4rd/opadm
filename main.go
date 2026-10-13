package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	httphandler "opadm/internal/infrastructure/http"
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
		// CSP sıkı: her şey same-origin. Tek istisna, tema kırpışmasını önleyen
		// minik inline script; sha256 hash'i sabit olduğu için allowlist'te.
		// index.html'deki script değişirse bu hash YENİDEN hesaplanmalı.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self' 'sha256-V/E0ZGUezEoOvey9qly2gDBIJn5K3JVU/Qm5IykSSkk='; img-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
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

	app, err := httphandler.NewFromEnv()
	if err != nil {
		log.Fatalf("setup: %v", err)
	}
	defer app.Service.Repo.Close()

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
	log.Printf("opadm listening on :%s (trustProxy=%v)", port, app.TrustProxy)
	log.Fatal(srv.ListenAndServe())
}
