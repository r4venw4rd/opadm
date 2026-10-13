package http

import (
	"net/http"
)

// sessionCookie, oturum çerezinin adıdır. Değer ham jetondur;
// DB'de yalnız özeti durur.
const sessionCookie = "opadm_session"

const sessionMaxAge = 7 * 24 * 3600 // application.SessionDays ile aynı

// credentialsInput, üye ol / giriş yap gövdesidir.
type credentialsInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *App) setSession(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true, // JS okuyamaz: XSS çalsa bile çerezi alamaz
		Secure:   a.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionMaxAge,
	})
}

func (a *App) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func sessionToken(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return ""
	}
	return c.Value
}

// currentUser, çerezdeki oturumun sahibini döner.
func (a *App) currentUser(r *http.Request) (email string, ok bool, err error) {
	tok := sessionToken(r)
	if tok == "" {
		return "", false, nil
	}
	return a.Service.Me(tok)
}

// Signup, POST /api/auth/signup. Başarı 201; kayıtlı e-posta 409.
func (a *App) Signup(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.guard(w, r); !ok {
		return
	}
	var in credentialsInput
	if !decode(w, r, &in) {
		return
	}
	fieldErrs, conflict, err := a.Service.Signup(in.Email, in.Password)
	if len(fieldErrs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "errors": fieldErrs})
		return
	}
	if conflict {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "Bu e-posta kayıtlı. Giriş yapın."})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Kayıt açılamadı. Daha sonra tekrar deneyin."})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

// Login, POST /api/auth/login. Doğruysa çerez yazar, 200 döner;
// yanlışsa 401. Hangisinin yanlış olduğu söylenmez.
func (a *App) Login(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.guard(w, r); !ok {
		return
	}
	var in credentialsInput
	if !decode(w, r, &in) {
		return
	}
	token, ok, err := a.Service.Login(in.Email, in.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Giriş yapılamadı. Daha sonra tekrar deneyin."})
		return
	}
	if !ok {
		// Kasıtlı gecikme yok; bcrypt zaten ~100ms sürer (kaba kuvvet freni).
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "E-posta veya parola hatalı."})
		return
	}
	a.setSession(w, token)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Logout, POST /api/auth/logout. Her durumda 200 + çerez silinir.
func (a *App) Logout(w http.ResponseWriter, r *http.Request) {
	if tok := sessionToken(r); tok != "" {
		_ = a.Service.Logout(tok)
	}
	a.clearSession(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Me, GET /api/auth/me. Giriş varsa e-postayı döner, yoksa 401.
// Frontend açılışta buraya bakıp arayüzü kurar.
func (a *App) Me(w http.ResponseWriter, r *http.Request) {
	email, ok, err := a.currentUser(r)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false})
		return
	}
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "email": email})
}
