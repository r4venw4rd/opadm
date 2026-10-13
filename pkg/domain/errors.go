package domain

import "errors"

// ErrNotFound, kaydın bulunamadığını anlatır (örn. yanlış giriş,
// süresi dolmuş oturum). Kimlik avına ipucu vermemek için istemciye
// "e-posta mı parola mı yanlış" ayrımı yapılmaz.
var ErrNotFound = errors.New("bulunamadı")
