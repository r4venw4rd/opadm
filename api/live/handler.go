package handler

import (
	"net/http"
)

// Vercel Go fonksiyonu: GET /api/live.
// Serverless soket tutamaz; istemci fetch'e düşer (bkz. README).
// Bu endpoint bilerek 501 döner, sessiz 404 değil.
func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Yöntem desteklenmiyor.", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte(`{"ok":false,"error":"Canlı soket bu platformda yok; /api/status kullanın."}`))
}
