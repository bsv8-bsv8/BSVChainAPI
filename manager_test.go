package chainapi

import (
	"context"
	"sync"
	"testing"
	"time"
)

type testProvider struct {
	name     string
	endpoint Endpoint
}

func (p testProvider) Name() string { return p.name }

func (p testProvider) NewEndpoint(cfg RouteConfig) (Endpoint, error) {
	return p.endpoint, nil
}

type testEndpoint struct {
	mu    sync.Mutex
	calls []time.Time
}

func (e *testEndpoint) GetUTXOsContext(ctx context.Context, address string) ([]UTXO, error) {
	e.mu.Lock()
	e.calls = append(e.calls, time.Now())
	e.mu.Unlock()
	return []UTXO{{TxID: address, Vout: 1, Value: 2}}, nil
}

func (e *testEndpoint) GetTipHeightContext(ctx context.Context) (uint32, error) {
	return 7, nil
}

func (e *testEndpoint) BroadcastContext(ctx context.Context, txHex string) (string, error) {
	return "txid-" + txHex, nil
}

func (e *testEndpoint) GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error) {
	return TxDetail{TxID: txid}, nil
}

func TestRouteNormalizeDefaultProfile(t *testing.T) {
	route := (Route{Provider: "WhatsonChain", Network: "testnet"}).Normalize()
	if route.Provider != "whatsonchain" {
		t.Fatalf("unexpected provider: %s", route.Provider)
	}
	if route.Network != "test" {
		t.Fatalf("unexpected network: %s", route.Network)
	}
	if route.Profile != DefaultProfile {
		t.Fatalf("unexpected profile: %s", route.Profile)
	}
}

func TestManagerRejectsDuplicateRoute(t *testing.T) {
	_, err := NewManagerWithProviders(Config{
		Routes: []RouteConfig{
			{Provider: "stub", Network: "test"},
			{Provider: "stub", Network: "test", Profile: DefaultProfile},
		},
	}, testProvider{name: "stub", endpoint: &testEndpoint{}})
	if err == nil {
		t.Fatalf("expected duplicate route error")
	}
}

func TestManagerProtectsSameRouteWithoutBlockingOtherRoute(t *testing.T) {
	epA := &testEndpoint{}
	epB := &testEndpoint{}
	manager, err := NewManagerWithProviders(Config{
		Routes: []RouteConfig{
			{Provider: "stub", Network: "test", Profile: "a", Protect: ProtectConfig{MinInterval: 40 * time.Millisecond}},
			{Provider: "stub", Network: "test", Profile: "b", Protect: ProtectConfig{MinInterval: 40 * time.Millisecond}},
		},
	}, providerMux{
		items: map[string]Endpoint{
			"a": epA,
			"b": epB,
		},
	})
	if err != nil {
		t.Fatalf("new manager failed: %v", err)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		<-start
		_, _ = manager.GetUTXOsContext(context.Background(), Route{Provider: "stub", Network: "test", Profile: "a"}, "1")
	}()
	go func() {
		defer wg.Done()
		<-start
		_, _ = manager.GetUTXOsContext(context.Background(), Route{Provider: "stub", Network: "test", Profile: "a"}, "2")
	}()
	go func() {
		defer wg.Done()
		<-start
		_, _ = manager.GetUTXOsContext(context.Background(), Route{Provider: "stub", Network: "test", Profile: "b"}, "3")
	}()
	close(start)
	wg.Wait()

	if len(epA.calls) != 2 || len(epB.calls) != 1 {
		t.Fatalf("unexpected calls count: routeA=%d routeB=%d", len(epA.calls), len(epB.calls))
	}
	diff := epA.calls[1].Sub(epA.calls[0])
	if diff < 35*time.Millisecond {
		t.Fatalf("same route should be serialized, diff=%s", diff)
	}
	diffOther := epB.calls[0].Sub(epA.calls[0])
	if diffOther > 25*time.Millisecond || diffOther < -25*time.Millisecond {
		t.Fatalf("different routes should not be serialized, diff=%s", diffOther)
	}
}

type providerMux struct {
	items map[string]Endpoint
}

func (p providerMux) Name() string { return "stub" }

func (p providerMux) NewEndpoint(cfg RouteConfig) (Endpoint, error) {
	return p.items[cfg.Route().Profile], nil
}
