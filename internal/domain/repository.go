package domain

// Repository, taleplerin kalıcılık sözleşmesidir. Gerçek: Postgres,
// testler: fake. Domain ne SQL ne HTTP bilir.
type Repository interface {
	Insert(name, email, service, message, ipHash string) (int64, error)
	Ping() error
	Count() (int, error)
	Close() error
	// DBInfo, canlı kanıt için DB'nin kendi ağzından kimlik verir.
	DBInfo() (DBIdentity, error)
	// ProveWrite, yazma yolunu uçtan uca dener ve GERİ ALIR (kayıt kalmaz).
	// Başarılıysa yazılan satırın id'sini döner.
	ProveWrite() (int64, error)
}

// DBIdentity, "hangi DB, ne zaman" sorusunun DB'den gelen cevabıdır.
type DBIdentity struct {
	Version  string
	Database string
	Now      string
	PingMs   int64
}
