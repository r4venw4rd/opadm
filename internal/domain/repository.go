package domain

// Repository, taleplerin kalıcılık sözleşmesidir. Gerçek: Postgres,
// testler: fake. Domain ne SQL ne HTTP bilir.
type Repository interface {
	Insert(name, email, service, message, ipHash string) (int64, error)
	Ping() error
	Count() (int, error)
	Close() error
}
