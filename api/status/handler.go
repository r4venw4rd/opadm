package handler

import (
	"net/http"
	"sync"

	"opadm/internal/handler"
)

// Vercel Go fonksiyonu: GET /api/status.
var (
	statusApp  *handler.App
	statusErr  error
	statusOnce sync.Once
)

func Handler(w http.ResponseWriter, r *http.Request) {
	statusOnce.Do(func() { statusApp, statusErr = handler.NewFromEnv() })
	if statusErr != nil || statusApp == nil {
		http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Yöntem desteklenmiyor.", http.StatusMethodNotAllowed)
		return
	}
	statusApp.Status(w, r)
}
