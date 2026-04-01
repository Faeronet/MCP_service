package ratelimit

import (
	"context"
	"sync"
	"time"
)

// InFlight limits concurrent operations (e.g. LLM calls).
type InFlight struct {
	mu    sync.Mutex
	max   int
	current int
	wait  chan struct{}
}

func NewInFlight(max int) *InFlight {
	if max <= 0 {
		max = 10
	}
	return &InFlight{max: max, wait: make(chan struct{}, max)}
}

func (l *InFlight) Acquire(ctx context.Context) error {
	select {
	case l.wait <- struct{}{}:
		l.mu.Lock()
		l.current++
		l.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *InFlight) Release() {
	l.mu.Lock()
	l.current--
	l.mu.Unlock()
	<-l.wait
}

func (l *InFlight) Current() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.current
}

// PerKey limits rate per key (e.g. per user or per chat). Simple in-memory sliding window.
type PerKey struct {
	mu       sync.Mutex
	maxCount int
	window   time.Duration
	keys     map[string][]time.Time
}

func NewPerKey(maxCount int, window time.Duration) *PerKey {
	return &PerKey{maxCount: maxCount, window: window, keys: make(map[string][]time.Time)}
}

func (p *PerKey) Allow(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-p.window)
	times := p.keys[key]
	var valid []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= p.maxCount {
		return false
	}
	valid = append(valid, now)
	p.keys[key] = valid
	return true
}
