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
	// FullCycle, yazma yolunu uçtan uca dener: yaz → geri oku → sil.
	// Kalıcı iz bırakmaz; sayaç başa döner. Dönen id kanıttır.
	FullCycle() (id int64, before, after int, err error)
	// SchemaTables, beklenen tablolardan eksikleri döner (boş = tam).
	SchemaTables() (missing []string, err error)
	// DBSizeBytes, veritabanı boyutudur.
	DBSizeBytes() (int64, error)
	// PoolStats, havuzun canlı sayılarıdır.
	PoolStats() (open, inUse, idle int)
}

// DBIdentity, "hangi DB, ne zaman" sorusunun DB'den gelen cevabıdır.
type DBIdentity struct {
	Version  string
	Database string
	Now      string
	PingUs   int64 // mikrosaniye: "0ms" gibi yuvarlak görünen kanıt olmasın
}
