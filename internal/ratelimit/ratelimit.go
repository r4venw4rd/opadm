package ratelimit

import (
	"sync"
	"time"
)

// Limiter, POST /api/requests'i kötüye kullanıma karşı koruyan
// basit in-memory sabit pencereli sayaçtır.
// Restart'ta sıfırlar, tek instance'ta geçerlidir (bkz. README).
type Limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	max    int
	window time.Duration
}

func New(max int, window time.Duration) *Limiter {
	return &Limiter{hits: make(map[string][]time.Time), max: max, window: window}
}

// Allow, ip bu pencerede kotanın altındaysa true döner ve sayacı artırır.
// Penceresi tamamen dolmuş IP'nin girdisini siler; hiç dönmeyen IP'ler
// janitor ile temizlenir. Böylece map sınırsız büyümez.
func (l *Limiter) Allow(ip string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	valid := l.hits[ip][:0]
	for _, t := range l.hits[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) == 0 && len(l.hits[ip]) > 0 {
		delete(l.hits, ip) // süresi dolmuş tek seferlik IP: yer kaplamasın
	}
	if len(valid) >= l.max {
		l.hits[ip] = valid
		return false
	}
	l.hits[ip] = append(valid, now)
	return true
}

// Cleanup, penceresi dolmuş tüm girdileri siler. StartJanitor periyodik
// çağırır; testler doğrudan çağırabilir.
func (l *Limiter) Cleanup() {
	cutoff := time.Now().Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	for ip, ts := range l.hits {
		kept := ts[:0]
		for _, t := range ts {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(l.hits, ip)
		} else {
			l.hits[ip] = kept
		}
	}
}

// Size, testler için map'teki IP sayısıdır.
func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.hits)
}

// StartJanitor, stop kapatılana kadar her interval'de Cleanup çalıştırır.
func (l *Limiter) StartJanitor(interval time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			l.Cleanup()
		case <-stop:
			return
		}
	}
}
