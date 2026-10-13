package domain

import (
	"net/mail"
	"strings"
)

// CheckEmail, e-posta biçim kuralının tek kaynağıdır (talep + üyelik ortak).
// Boş string = geçerli.
func CheckEmail(email string) string {
	if _, err := mail.ParseAddress(email); err != nil || !strings.Contains(email, ".") {
		return "Geçerli bir e-posta adresi yazın."
	}
	return ""
}

// CheckPassword, parola kuralıdır: 8-128 karakter. Boş string = geçerli.
func CheckPassword(password string) string {
	if n := len([]rune(password)); n < 8 || n > 128 {
		return "Parola 8-128 karakter olmalı."
	}
	return ""
}

// NormalizeEmail, karşılaştırma/kayıt öncesi biçimi düzeltir.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
