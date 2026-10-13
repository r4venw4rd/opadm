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
	if len(valid) >= l.max {
		l.hits[ip] = valid
		return false
	}
	l.hits[ip] = append(valid, now)
	return true
}
