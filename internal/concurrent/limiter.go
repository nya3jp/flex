package concurrent

import "context"

type Limiter struct {
	tokens chan struct{}
}

func NewLimiter(n int) *Limiter {
	tokens := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		tokens <- struct{}{}
	}
	return &Limiter{tokens: tokens}
}

func (l *Limiter) Take(ctx context.Context) error {
	select {
	case <-l.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *Limiter) TryTake() bool {
	select {
	case <-l.tokens:
		return true
	default:
		return false
	}
}

func (l *Limiter) Done() {
	select {
	case l.tokens <- struct{}{}:
	default:
		panic("concurrent.Limit: Done called excessively")
	}
}
