package chainapi

import (
	"context"
	"sync"
	"time"
)

type turnGate struct {
	interval time.Duration

	mu          sync.Mutex
	nextAllowed time.Time
}

func newTurnGate(interval time.Duration) *turnGate {
	if interval <= 0 {
		return nil
	}
	return &turnGate{interval: interval}
}

func (g *turnGate) Wait(ctx context.Context) error {
	if g == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	waitUntil := g.reserveSlot()
	waitDur := time.Until(waitUntil)
	if waitDur <= 0 {
		return nil
	}
	timer := time.NewTimer(waitDur)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (g *turnGate) reserveSlot() time.Time {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	slot := now
	if !g.nextAllowed.IsZero() && g.nextAllowed.After(now) {
		slot = g.nextAllowed
	}
	g.nextAllowed = slot.Add(g.interval)
	return slot
}
