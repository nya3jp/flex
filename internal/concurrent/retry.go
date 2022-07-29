package concurrent

import (
	"context"
	"sync"
	"time"

	"github.com/nya3jp/flex/internal/ctxutil"
)

type Retry struct {
	min time.Duration
	max time.Duration

	mu  sync.Mutex
	cur time.Duration
}

func NewRetry(min, max time.Duration) *Retry {
	return &Retry{min: min, max: max, cur: min}
}

func (r *Retry) Wait(ctx context.Context) error {
	r.mu.Lock()
	wait := r.cur
	r.cur *= 2
	if r.cur > r.max {
		r.cur = r.max
	}
	r.mu.Unlock()

	return ctxutil.Sleep(ctx, wait)
}

func (r *Retry) Clear() {
	r.mu.Lock()
	r.cur = r.min
	r.mu.Unlock()
}
