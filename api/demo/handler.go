package handler

import (
	"net/http"
	"sync"

	httphandler "opadm/internal/infrastructure/http"
)

// Vercel Go fonksiyonu: GET /api/demo.
var (
	demoApp  *httphandler.App
	demoErr  error
	demoOnce sync.Once
)

func Handler(w http.ResponseWriter, r *http.Request) {
	demoOnce.Do(func() { demoApp, demoErr = httphandler.NewFromEnv() })
	if demoErr != nil || demoApp == nil {
		http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Yöntem desteklenmiyor.", http.StatusMethodNotAllowed)
		return
	}
	demoApp.Demo(w, r)
}
