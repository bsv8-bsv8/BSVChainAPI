package chainapi

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestCapabilityPortServerAndClientRoundTrip(t *testing.T) {
	manager, err := NewManagerWithProviders(Config{
		Routes: []RouteConfig{
			{Provider: "stub", Network: "test", Profile: "read"},
			{Provider: "stub", Network: "test", Profile: "submit"},
		},
	}, providerMux{
		items: map[string]Endpoint{
			"read": &testEndpoint{},
			"submit": &submitRouterEndpoint{broadcast: func(string) (string, error) {
				return computeTxID("01020304")
			}},
		},
	})
	if err != nil {
		t.Fatalf("new manager failed: %v", err)
	}
	capManager, err := NewCapabilityManager(context.Background(), manager, CapabilityPlan{
		UTXORoute:      Route{Provider: "stub", Network: "test", Profile: "read"},
		TipHeightRoute: Route{Provider: "stub", Network: "test", Profile: "read"},
		TxDetailRoute:  Route{Provider: "stub", Network: "test", Profile: "read"},
		TxSubmitPolicy: TxSubmitPolicy{Routes: []Route{{Provider: "stub", Network: "test", Profile: "submit"}}},
	})
	if err != nil {
		t.Fatalf("new capability manager failed: %v", err)
	}
	srv := httptest.NewServer(NewCapabilityPortServer(capManager).Handler())
	defer srv.Close()

	client := NewCapabilityPortClient(srv.URL)
	utxos, err := client.GetUTXOsContext(context.Background(), "abc")
	if err != nil {
		t.Fatalf("get utxos failed: %v", err)
	}
	if len(utxos) != 1 || utxos[0].TxID != "abc" {
		t.Fatalf("unexpected utxos: %+v", utxos)
	}
	tip, err := client.GetTipHeightContext(context.Background())
	if err != nil {
		t.Fatalf("get tip failed: %v", err)
	}
	if tip != 7 {
		t.Fatalf("unexpected tip: %d", tip)
	}
	txj, err := client.GetTxDetailContext(context.Background(), "tx1")
	if err != nil {
		t.Fatalf("get tx detail failed: %v", err)
	}
	if txj.TxID != "tx1" {
		t.Fatalf("unexpected tx detail: %+v", txj)
	}
	result, err := client.SubmitTxContext(context.Background(), "01020304")
	if err != nil {
		t.Fatalf("submit tx failed: %v", err)
	}
	expectedTxID, err := computeTxID("01020304")
	if err != nil {
		t.Fatalf("compute txid failed: %v", err)
	}
	if result.TxID != expectedTxID || result.ExpectedTxID != expectedTxID {
		t.Fatalf("unexpected submit result: %+v", result)
	}
}
