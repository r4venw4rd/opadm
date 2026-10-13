package handler

import (
	"net/http"
	"sync"

	httphandler "opadm/internal/infrastructure/http"
)

// Vercel Go fonksiyonu: POST /api/requests.
// Soğuk başlatmada bir kez kurulur, sonraki çağrılar havuzu tekrar kullanır.
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
	if r.Method != http.MethodPost {
		http.Error(w, "Yöntem desteklenmiyor.", http.StatusMethodNotAllowed)
		return
	}
	app.Create(w, r)
}
