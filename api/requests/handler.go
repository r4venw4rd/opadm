package handler

import (
	"net/http"
	"sync"

	"opadm/internal/handler"
)

// Vercel Go fonksiyonu: POST /api/requests.
// Soğuk başlatmada bir kez kurulur, sonraki çağrılar havuzu tekrar kullanır.
var (
	app     *handler.App
	appErr  error
	appOnce sync.Once
)

func Handler(w http.ResponseWriter, r *http.Request) {
	appOnce.Do(func() { app, appErr = handler.NewFromEnv() })
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
