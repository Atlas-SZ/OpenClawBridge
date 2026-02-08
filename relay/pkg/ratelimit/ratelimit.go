package ratelimit

// Limiter is a v0.1 stub.
type Limiter struct{}

func New() *Limiter {
	return &Limiter{}
}

func (l *Limiter) Allow(_ string) bool {
	return true
}
