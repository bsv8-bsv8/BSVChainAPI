package chainapi

import (
	"context"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
)

type testHTTPStatusError struct {
	status int
	body   string
}

func (e *testHTTPStatusError) Error() string {
	return fmt.Sprintf("http %d: %s", e.status, e.body)
}

func (e *testHTTPStatusError) HTTPStatus() int {
	return e.status
}

func (e *testHTTPStatusError) HTTPBody() string {
	return e.body
}

type submitRouterTestAPI struct {
	mu         sync.Mutex
	routeInfos map[string]RouteInfo
	broadcasts map[string]func(string) (string, error)
	calls      []string
}

func (a *submitRouterTestAPI) GetUTXOsContext(ctx context.Context, route Route, address string) ([]UTXO, error) {
	return nil, fmt.Errorf("unsupported")
}

func (a *submitRouterTestAPI) GetTipHeightContext(ctx context.Context, route Route) (uint32, error) {
	return 0, fmt.Errorf("unsupported")
}

func (a *submitRouterTestAPI) BroadcastContext(ctx context.Context, route Route, txHex string) (string, error) {
	a.mu.Lock()
	a.calls = append(a.calls, route.Key())
	fn := a.broadcasts[route.Key()]
	a.mu.Unlock()
	if fn == nil {
		return "", fmt.Errorf("missing broadcast handler for %s", route.Key())
	}
	return fn(txHex)
}

func (a *submitRouterTestAPI) GetTxDetailContext(ctx context.Context, route Route, txid string) (TxDetail, error) {
	return TxDetail{}, fmt.Errorf("unsupported")
}

func (a *submitRouterTestAPI) GetRouteInfoContext(ctx context.Context, route Route) (RouteInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	info, ok := a.routeInfos[route.Key()]
	if !ok {
		return RouteInfo{}, fmt.Errorf("route not found: %s", route.Key())
	}
	return info, nil
}

func TestTxSubmitRouterSuccessAndFallback(t *testing.T) {
	txHex := "01020304"
	expectedTxID, err := computeTxID(txHex)
	if err != nil {
		t.Fatalf("compute txid failed: %v", err)
	}
	routeA := Route{Provider: "stub", Network: "test", Profile: "a"}.Normalize()
	routeB := Route{Provider: "stub", Network: "test", Profile: "b"}.Normalize()
	api := &submitRouterTestAPI{
		routeInfos: map[string]RouteInfo{
			routeA.Key(): {Route: routeA, Capabilities: []Capability{CapabilityBroadcast}},
			routeB.Key(): {Route: routeB, Capabilities: []Capability{CapabilityBroadcast}},
		},
		broadcasts: map[string]func(string) (string, error){
			routeA.Key(): func(string) (string, error) {
				return "", &testHTTPStatusError{status: 503, body: "unavailable"}
			},
			routeB.Key(): func(txHex string) (string, error) {
				got, err := computeTxID(txHex)
				if err != nil {
					return "", err
				}
				return got, nil
			},
		},
	}
	router, err := NewTxSubmitRouter(context.Background(), api, TxSubmitPolicy{Routes: []Route{routeA, routeB}})
	if err != nil {
		t.Fatalf("new router failed: %v", err)
	}
	result, err := router.SubmitTxContext(context.Background(), txHex)
	if err != nil {
		t.Fatalf("submit tx failed: %v", err)
	}
	if result.TxID != expectedTxID || result.ExpectedTxID != expectedTxID {
		t.Fatalf("unexpected txid result: %+v", result)
	}
	if result.WinnerRoute.Key() != routeB.Key() {
		t.Fatalf("unexpected winner route: %+v", result.WinnerRoute)
	}
	if len(result.Attempts) != 2 {
		t.Fatalf("unexpected attempts: %+v", result.Attempts)
	}
	if result.Attempts[0].ErrorClass != BroadcastErrorTemporary || result.Attempts[1].Outcome != TxSubmitAttemptSuccess {
		t.Fatalf("unexpected attempts detail: %+v", result.Attempts)
	}
}

func TestTxSubmitRouterPermanentStopsFallback(t *testing.T) {
	txHex := "01020304"
	routeA := Route{Provider: "stub", Network: "test", Profile: "a"}.Normalize()
	routeB := Route{Provider: "stub", Network: "test", Profile: "b"}.Normalize()
	api := &submitRouterTestAPI{
		routeInfos: map[string]RouteInfo{
			routeA.Key(): {Route: routeA, Capabilities: []Capability{CapabilityBroadcast}},
			routeB.Key(): {Route: routeB, Capabilities: []Capability{CapabilityBroadcast}},
		},
		broadcasts: map[string]func(string) (string, error){
			routeA.Key(): func(string) (string, error) {
				return "", &testHTTPStatusError{status: 400, body: "invalid tx"}
			},
			routeB.Key(): func(string) (string, error) {
				return "should-not-run", nil
			},
		},
	}
	router, err := NewTxSubmitRouter(context.Background(), api, TxSubmitPolicy{Routes: []Route{routeA, routeB}})
	if err != nil {
		t.Fatalf("new router failed: %v", err)
	}
	result, err := router.SubmitTxContext(context.Background(), txHex)
	if err == nil {
		t.Fatalf("expected submit error")
	}
	if len(result.Attempts) != 1 {
		t.Fatalf("unexpected attempts: %+v", result.Attempts)
	}
	if result.Attempts[0].ErrorClass != BroadcastErrorPermanent {
		t.Fatalf("unexpected error class: %+v", result.Attempts[0])
	}
	if got := len(api.calls); got != 1 {
		t.Fatalf("fallback should not run, calls=%d", got)
	}
}

func TestTxSubmitRouterAlreadyKnownCountsAsSuccess(t *testing.T) {
	txHex := "01020304"
	expectedTxID, err := computeTxID(txHex)
	if err != nil {
		t.Fatalf("compute txid failed: %v", err)
	}
	route := Route{Provider: "stub", Network: "test", Profile: "a"}.Normalize()
	api := &submitRouterTestAPI{
		routeInfos: map[string]RouteInfo{
			route.Key(): {Route: route, Capabilities: []Capability{CapabilityBroadcast}},
		},
		broadcasts: map[string]func(string) (string, error){
			route.Key(): func(string) (string, error) {
				return "", &testHTTPStatusError{status: 409, body: "txn-already-known"}
			},
		},
	}
	router, err := NewTxSubmitRouter(context.Background(), api, TxSubmitPolicy{Routes: []Route{route}})
	if err != nil {
		t.Fatalf("new router failed: %v", err)
	}
	result, err := router.SubmitTxContext(context.Background(), txHex)
	if err != nil {
		t.Fatalf("submit tx failed: %v", err)
	}
	if result.TxID != expectedTxID || result.Attempts[0].Outcome != TxSubmitAttemptAlreadyKnown {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestTxSubmitRouterRejectsUnsupportedBroadcastRoute(t *testing.T) {
	route := Route{Provider: "stub", Network: "test", Profile: "a"}.Normalize()
	api := &submitRouterTestAPI{
		routeInfos: map[string]RouteInfo{
			route.Key(): {Route: route, Capabilities: []Capability{CapabilityGetTipHeight}},
		},
	}
	if _, err := NewTxSubmitRouter(context.Background(), api, TxSubmitPolicy{Routes: []Route{route}}); err == nil {
		t.Fatalf("expected unsupported broadcast route error")
	}
}

func TestTxSubmitRouterRejectsDuplicateRoutes(t *testing.T) {
	route := Route{Provider: "stub", Network: "test", Profile: "a"}.Normalize()
	api := &submitRouterTestAPI{}
	if _, err := NewTxSubmitRouter(context.Background(), api, TxSubmitPolicy{Routes: []Route{route, route}}); err == nil {
		t.Fatalf("expected duplicate route error")
	}
}

type submitRouterEndpoint struct {
	broadcast func(string) (string, error)
}

func (e *submitRouterEndpoint) Capabilities() []Capability {
	return []Capability{CapabilityBroadcast}
}

func (e *submitRouterEndpoint) GetUTXOsContext(ctx context.Context, address string) ([]UTXO, error) {
	return nil, unsupportedCapabilityError(Route{Provider: "stub", Network: "test"}, CapabilityGetUTXOs)
}

func (e *submitRouterEndpoint) GetTipHeightContext(ctx context.Context) (uint32, error) {
	return 0, unsupportedCapabilityError(Route{Provider: "stub", Network: "test"}, CapabilityGetTipHeight)
}

func (e *submitRouterEndpoint) BroadcastContext(ctx context.Context, txHex string) (string, error) {
	return e.broadcast(txHex)
}

func (e *submitRouterEndpoint) GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error) {
	return TxDetail{}, unsupportedCapabilityError(Route{Provider: "stub", Network: "test"}, CapabilityGetTxDetail)
}

func TestTxSubmitRouterPortClientKeepsHTTPStatusForFallback(t *testing.T) {
	txHex := "01020304"
	expectedTxID, err := computeTxID(txHex)
	if err != nil {
		t.Fatalf("compute txid failed: %v", err)
	}
	manager, err := NewManagerWithProviders(Config{
		Routes: []RouteConfig{
			{Provider: "stub", Network: "test", Profile: "a"},
			{Provider: "stub", Network: "test", Profile: "b"},
		},
	}, providerMux{
		items: map[string]Endpoint{
			"a": &submitRouterEndpoint{
				broadcast: func(string) (string, error) {
					return "", &testHTTPStatusError{status: 503, body: "unavailable"}
				},
			},
			"b": &submitRouterEndpoint{
				broadcast: func(string) (string, error) {
					return expectedTxID, nil
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new manager failed: %v", err)
	}
	srv := httptest.NewServer(NewPortServer(manager).Handler())
	defer srv.Close()

	client := NewPortClient(srv.URL)
	routeA := Route{Provider: "stub", Network: "test", Profile: "a"}.Normalize()
	routeB := Route{Provider: "stub", Network: "test", Profile: "b"}.Normalize()
	router, err := NewTxSubmitRouter(context.Background(), client, TxSubmitPolicy{Routes: []Route{routeA, routeB}})
	if err != nil {
		t.Fatalf("new router failed: %v", err)
	}
	result, err := router.SubmitTxContext(context.Background(), txHex)
	if err != nil {
		t.Fatalf("submit tx failed: %v", err)
	}
	if result.TxID != expectedTxID {
		t.Fatalf("unexpected txid: %+v", result)
	}
	if len(result.Attempts) != 2 || result.Attempts[0].ErrorClass != BroadcastErrorTemporary {
		t.Fatalf("unexpected attempts: %+v", result.Attempts)
	}
	if result.WinnerRoute.Key() != routeB.Key() {
		t.Fatalf("unexpected winner route: %+v", result.WinnerRoute)
	}
}
