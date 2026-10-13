package http

import "net/http"

// RegisterRoutes, API rotalarının TEK kaynağıdır. Hem main.go (gömülü statik
// dosyalarla) hem de Vercel fonksiyonu (api/index.go) bu haritayı kullanır;
// böylece rota tanımları iki ortamda ayrışamaz.
func (a *App) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/requests", a.Create)
	mux.HandleFunc("GET /healthz", a.Health)
	mux.HandleFunc("GET /api/status", a.Status)
	mux.HandleFunc("GET /api/live", a.Live)
	mux.HandleFunc("GET /api/demo", a.Demo)
	mux.HandleFunc("POST /api/auth/signup", a.Signup)
	mux.HandleFunc("POST /api/auth/login", a.Login)
	mux.HandleFunc("POST /api/auth/logout", a.Logout)
	mux.HandleFunc("GET /api/auth/me", a.Me)
}
