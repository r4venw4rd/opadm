package handler

import (
	"net/http"
	"sync"

	httphandler "opadm/internal/infrastructure/http"
)

// Vercel Go fonksiyonu: /api/* ve /healthz istekleri vercel.json'daki rewrite
// ile buraya gelir. Rotalar main.go ile AYNI haritadan (App.RegisterRoutes)
// çözülür; iki ortamda rota farkı olmaz.
//
// Serverless'ta kalıcı süreç yoktur: App + mux ilk istekte bir kez kurulur,
// sonraki çağrılar (aynı instance içinde) havuzu yeniden kullanır.
var (
	once   sync.Once
	app    *httphandler.App
	appErr error
	mux    *http.ServeMux
)

func Handler(w http.ResponseWriter, r *http.Request) {
	// WebSocket serverless fonksiyonda tutulamaz; net 501 dön ki istemci
	// hızla fetch'e düşsün (sessiz 404/hang yerine).
	if r.URL.Path == "/api/live" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"ok":false,"error":"Canlı soket bu platformda yok; /api/status kullanın."}`))
		return
	}

	once.Do(func() {
		app, appErr = httphandler.NewFromEnv()
		if appErr != nil {
			return
		}
		mux = http.NewServeMux()
		app.RegisterRoutes(mux)
	})
	if appErr != nil || mux == nil {
		http.Error(w, "Sunucu hatası.", http.StatusInternalServerError)
		return
	}
	mux.ServeHTTP(w, r)
}
