package chainapi

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestPortServerAndClientRoundTrip(t *testing.T) {
	manager, err := NewManagerWithProviders(Config{
		Routes: []RouteConfig{
			{Provider: "stub", Network: "test"},
		},
	}, testProvider{name: "stub", endpoint: &testEndpoint{}})
	if err != nil {
		t.Fatalf("new manager failed: %v", err)
	}
	srv := httptest.NewServer(NewPortServer(manager).Handler())
	defer srv.Close()

	client := NewPortClient(srv.URL)
	route := Route{Provider: "stub", Network: "test"}

	utxos, err := client.GetUTXOsContext(context.Background(), route, "abc")
	if err != nil {
		t.Fatalf("get utxos failed: %v", err)
	}
	if len(utxos) != 1 || utxos[0].TxID != "abc" {
		t.Fatalf("unexpected utxos: %+v", utxos)
	}

	tip, err := client.GetTipHeightContext(context.Background(), route)
	if err != nil {
		t.Fatalf("get tip failed: %v", err)
	}
	if tip != 7 {
		t.Fatalf("unexpected tip: %d", tip)
	}

	txid, err := client.BroadcastContext(context.Background(), route, "beef")
	if err != nil {
		t.Fatalf("broadcast failed: %v", err)
	}
	if txid != "txid-beef" {
		t.Fatalf("unexpected txid: %s", txid)
	}

	txj, err := client.GetTxDetailContext(context.Background(), route, "tx1")
	if err != nil {
		t.Fatalf("get tx detail failed: %v", err)
	}
	if txj.TxID != "tx1" {
		t.Fatalf("unexpected tx detail: %+v", txj)
	}
}
