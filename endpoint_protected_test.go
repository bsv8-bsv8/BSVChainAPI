package chainapi

import (
	"context"
	"testing"
	"time"
)

func TestNewProtectedEndpointPreservesBehaviorAndCapabilities(t *testing.T) {
	inner := &testEndpoint{}
	wrapped := NewProtectedEndpoint(inner, 40*time.Millisecond)
	if wrapped == nil {
		t.Fatalf("expected wrapped endpoint")
	}
	if len(wrapped.Capabilities()) != len(inner.Capabilities()) {
		t.Fatalf("unexpected capabilities: %+v", wrapped.Capabilities())
	}
	start := time.Now()
	if _, err := wrapped.GetUTXOsContext(context.Background(), "a"); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if _, err := wrapped.GetUTXOsContext(context.Background(), "b"); err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if len(inner.calls) != 2 {
		t.Fatalf("unexpected inner call count: %d", len(inner.calls))
	}
	if elapsed := time.Since(start); elapsed < 35*time.Millisecond {
		t.Fatalf("protected endpoint should delay second call, elapsed=%s", elapsed)
	}
}

func TestNewProtectedEndpointWithoutIntervalReturnsInner(t *testing.T) {
	inner := &testEndpoint{}
	if got := NewProtectedEndpoint(inner, 0); got != inner {
		t.Fatalf("expected inner endpoint passthrough")
	}
}
