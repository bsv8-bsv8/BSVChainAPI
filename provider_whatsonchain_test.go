package chainapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsv8/BSVChainAPI/internal/whatsonchain"
)

func TestWhatsOnChainAuthAndFallback(t *testing.T) {
	var seenAuth string
	var seenQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("X-API-Key")
		seenQuery = r.URL.Query().Get("token")
		switch r.URL.Path {
		case "/address/abc/confirmed/unspent":
			http.NotFound(w, r)
		case "/address/abc/unspent":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"tx_hash": "tx0", "tx_pos": 0, "value": 7, "isSpentInMempoolTx": true},
				{"tx_hash": "tx1", "tx_pos": 1, "value": 9},
			})
		case "/chain/info":
			_ = json.NewEncoder(w).Encode(map[string]any{"blocks": 11})
		case "/tx/raw":
			_ = json.NewEncoder(w).Encode("txid-1")
		case "/tx/hash/tx1":
			_ = json.NewEncoder(w).Encode(map[string]any{"txid": "tx1"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := whatsonchain.NewClient(srv.URL, whatsonchain.AuthConfig{
		Mode:  "header",
		Name:  "X-API-Key",
		Value: "secret",
	})
	utxos, err := client.GetUTXOsContext(context.Background(), "abc")
	if err != nil {
		t.Fatalf("get utxos failed: %v", err)
	}
	if len(utxos) != 1 || utxos[0].TxID != "tx1" {
		t.Fatalf("unexpected utxos: %+v", utxos)
	}
	if seenAuth != "secret" {
		t.Fatalf("missing auth header: %q", seenAuth)
	}

	client = whatsonchain.NewClient(srv.URL, whatsonchain.AuthConfig{
		Mode:  "query",
		Name:  "token",
		Value: "abc123",
	})
	if _, err := client.GetTipHeightContext(context.Background()); err != nil {
		t.Fatalf("get tip failed: %v", err)
	}
	if seenQuery != "abc123" {
		t.Fatalf("missing auth query: %q", seenQuery)
	}
}

func TestWhatsOnChainProviderDefaultBaseURL(t *testing.T) {
	if got := whatsonchain.BaseURLForNetwork("main"); got != whatsonchain.MainnetBaseURL {
		t.Fatalf("unexpected mainnet url: %s", got)
	}
	if got := whatsonchain.BaseURLForNetwork("test"); got != whatsonchain.TestnetBaseURL {
		t.Fatalf("unexpected testnet url: %s", got)
	}
}
