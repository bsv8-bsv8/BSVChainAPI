package chainapi

import (
	"context"
	"testing"
)

type capabilityManagerTestAPI struct {
	routeInfos map[string]RouteInfo
	utxos      map[string]func(string) ([]UTXO, error)
	tips       map[string]func() (uint32, error)
	txDetails  map[string]func(string) (TxDetail, error)
	broadcasts map[string]func(string) (string, error)
}

func (a *capabilityManagerTestAPI) GetUTXOsContext(ctx context.Context, route Route, address string) ([]UTXO, error) {
	fn := a.utxos[route.Key()]
	if fn == nil {
		return nil, unsupportedCapabilityError(route, CapabilityGetUTXOs)
	}
	return fn(address)
}

func (a *capabilityManagerTestAPI) GetTipHeightContext(ctx context.Context, route Route) (uint32, error) {
	fn := a.tips[route.Key()]
	if fn == nil {
		return 0, unsupportedCapabilityError(route, CapabilityGetTipHeight)
	}
	return fn()
}

func (a *capabilityManagerTestAPI) BroadcastContext(ctx context.Context, route Route, txHex string) (string, error) {
	fn := a.broadcasts[route.Key()]
	if fn == nil {
		return "", unsupportedCapabilityError(route, CapabilityBroadcast)
	}
	return fn(txHex)
}

func (a *capabilityManagerTestAPI) GetTxDetailContext(ctx context.Context, route Route, txid string) (TxDetail, error) {
	fn := a.txDetails[route.Key()]
	if fn == nil {
		return TxDetail{}, unsupportedCapabilityError(route, CapabilityGetTxDetail)
	}
	return fn(txid)
}

func (a *capabilityManagerTestAPI) GetRouteInfoContext(ctx context.Context, route Route) (RouteInfo, error) {
	info, ok := a.routeInfos[route.Key()]
	if !ok {
		return RouteInfo{}, unsupportedCapabilityError(route, CapabilityBroadcast)
	}
	return info, nil
}

func TestCapabilityManagerDelegatesByPlan(t *testing.T) {
	txHex := "01020304"
	expectedTxID, err := computeTxID(txHex)
	if err != nil {
		t.Fatalf("compute txid failed: %v", err)
	}
	utxoRoute := Route{Provider: "stub", Network: "test", Profile: "utxo"}.Normalize()
	tipRoute := Route{Provider: "stub", Network: "test", Profile: "tip"}.Normalize()
	txDetailRoute := Route{Provider: "stub", Network: "test", Profile: "detail"}.Normalize()
	submitRoute := Route{Provider: "stub", Network: "test", Profile: "submit"}.Normalize()
	api := &capabilityManagerTestAPI{
		routeInfos: map[string]RouteInfo{
			utxoRoute.Key():     {Route: utxoRoute, Capabilities: []Capability{CapabilityGetUTXOs}},
			tipRoute.Key():      {Route: tipRoute, Capabilities: []Capability{CapabilityGetTipHeight}},
			txDetailRoute.Key(): {Route: txDetailRoute, Capabilities: []Capability{CapabilityGetTxDetail}},
			submitRoute.Key():   {Route: submitRoute, Capabilities: []Capability{CapabilityBroadcast}},
		},
		utxos: map[string]func(string) ([]UTXO, error){
			utxoRoute.Key(): func(address string) ([]UTXO, error) {
				return []UTXO{{TxID: address, Vout: 2, Value: 99}}, nil
			},
		},
		tips: map[string]func() (uint32, error){
			tipRoute.Key(): func() (uint32, error) { return 88, nil },
		},
		txDetails: map[string]func(string) (TxDetail, error){
			txDetailRoute.Key(): func(txid string) (TxDetail, error) {
				return TxDetail{TxID: txid}, nil
			},
		},
		broadcasts: map[string]func(string) (string, error){
			submitRoute.Key(): func(string) (string, error) { return expectedTxID, nil },
		},
	}
	manager, err := NewCapabilityManager(context.Background(), api, CapabilityPlan{
		UTXORoute:      utxoRoute,
		TipHeightRoute: tipRoute,
		TxDetailRoute:  txDetailRoute,
		TxSubmitPolicy: TxSubmitPolicy{Routes: []Route{submitRoute}},
	})
	if err != nil {
		t.Fatalf("new capability manager failed: %v", err)
	}
	utxos, err := manager.GetUTXOsContext(context.Background(), "addr1")
	if err != nil {
		t.Fatalf("get utxos failed: %v", err)
	}
	if len(utxos) != 1 || utxos[0].TxID != "addr1" {
		t.Fatalf("unexpected utxos: %+v", utxos)
	}
	tip, err := manager.GetTipHeightContext(context.Background())
	if err != nil {
		t.Fatalf("get tip failed: %v", err)
	}
	if tip != 88 {
		t.Fatalf("unexpected tip: %d", tip)
	}
	txj, err := manager.GetTxDetailContext(context.Background(), "tx1")
	if err != nil {
		t.Fatalf("get tx detail failed: %v", err)
	}
	if txj.TxID != "tx1" {
		t.Fatalf("unexpected tx detail: %+v", txj)
	}
	result, err := manager.SubmitTxContext(context.Background(), txHex)
	if err != nil {
		t.Fatalf("submit tx failed: %v", err)
	}
	if result.TxID != expectedTxID || result.WinnerRoute.Key() != submitRoute.Key() {
		t.Fatalf("unexpected submit result: %+v", result)
	}
	gotPlan := manager.Plan()
	if gotPlan.UTXORoute != utxoRoute || gotPlan.TipHeightRoute != tipRoute || gotPlan.TxDetailRoute != txDetailRoute {
		t.Fatalf("unexpected plan: %+v", gotPlan)
	}
}

func TestCapabilityManagerRejectsUnsupportedReadRoute(t *testing.T) {
	route := Route{Provider: "stub", Network: "test", Profile: "only-submit"}.Normalize()
	api := &capabilityManagerTestAPI{
		routeInfos: map[string]RouteInfo{
			route.Key(): {Route: route, Capabilities: []Capability{CapabilityBroadcast}},
		},
		broadcasts: map[string]func(string) (string, error){
			route.Key(): func(string) (string, error) { return "", nil },
		},
	}
	_, err := NewCapabilityManager(context.Background(), api, CapabilityPlan{
		UTXORoute:      route,
		TipHeightRoute: route,
		TxDetailRoute:  route,
		TxSubmitPolicy: TxSubmitPolicy{Routes: []Route{route}},
	})
	if err == nil {
		t.Fatalf("expected unsupported route error")
	}
}
