package handler

import (
	"net/http"
	"sync"

	httphandler "opadm/internal/infrastructure/http"
)

// Vercel Go fonksiyonu: Get /api/auth/me.
var (
	app     *httphandler.App
	appErr  error
	appOnce sync.Once
)

func Handler(w http.ResponseWriter, r *http.Request) {
	appOnce.Do(func() { app, appErr = httphandler.NewFromEnv() })
	if appErr != nil || app == nil {
		http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Yöntem desteklenmiyor.", http.StatusMethodNotAllowed)
		return
	}
	app.Me(w, r)
}
