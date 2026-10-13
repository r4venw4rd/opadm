package domain

import (
	"net/mail"
	"strings"
)

// CheckEmail, e-posta biçim kuralının tek kaynağıdır.
// Boş string = geçerli.
func CheckEmail(email string) string {
	if _, err := mail.ParseAddress(email); err != nil || !strings.Contains(email, ".") {
		return "Geçerli bir e-posta adresi yazın."
	}
	return ""
}
