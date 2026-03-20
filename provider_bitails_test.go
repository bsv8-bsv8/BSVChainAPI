package chainapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bsv8/BSVChainAPI/internal/bitails"
)

func TestBitailsAuthDefaultsAndMapping(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("apikey")
		switch r.URL.Path {
		case "/address/abc/unspent":
			if r.URL.Query().Get("from") == "0" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"address": "abc",
					"unspent": []map[string]any{
						{"txid": "mempool-only", "vout": 0, "satoshis": 50, "blockheight": -1, "confirmations": 0},
						{"txid": "tx1", "vout": 2, "satoshis": 99, "blockheight": 1, "confirmations": 2},
					},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"address": "abc", "unspent": []any{}})
		case "/network/info":
			_ = json.NewEncoder(w).Encode(map[string]any{"blocks": 123})
		case "/tx/broadcast":
			_ = json.NewEncoder(w).Encode(map[string]any{"txid": "btx1"})
		case "/tx/tx1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"txid":           "tx1",
				"partialOutputs": true,
				"inputs": []map[string]any{
					{
						"index": 0,
						"source": map[string]any{
							"txid":     "prev1",
							"index":    3,
							"script":   "76a9",
							"satoshis": 120,
						},
					},
				},
				"outputs": []map[string]any{
					{
						"index":         0,
						"satoshis":      100,
						"script":        "76a9",
						"partialScript": true,
					},
				},
			})
		case "/download/tx/tx1/output/0/hex":
			_, _ = w.Write([]byte("76a91401"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	manager, err := NewManager(Config{
		Routes: []RouteConfig{
			{
				Provider: "bitails",
				Network:  "test",
				BaseURL:  srv.URL,
				Auth: AuthConfig{
					Value: "secret",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new manager failed: %v", err)
	}

	route := Route{Provider: "bitails", Network: "test"}
	utxos, err := manager.GetUTXOsContext(context.Background(), route, "abc")
	if err != nil {
		t.Fatalf("get utxos failed: %v", err)
	}
	if seenAuth != "secret" {
		t.Fatalf("expected apikey header, got %q", seenAuth)
	}
	if len(utxos) != 1 || utxos[0].Value != 99 {
		t.Fatalf("unexpected utxos: %+v", utxos)
	}
	tip, err := manager.GetTipHeightContext(context.Background(), route)
	if err != nil || tip != 123 {
		t.Fatalf("unexpected tip result: tip=%d err=%v", tip, err)
	}
	txid, err := manager.BroadcastContext(context.Background(), route, "beef")
	if err != nil || txid != "btx1" {
		t.Fatalf("unexpected broadcast result: txid=%s err=%v", txid, err)
	}
	txj, err := manager.GetTxDetailContext(context.Background(), route, "tx1")
	if err != nil {
		t.Fatalf("get tx detail failed: %v", err)
	}
	if txj.TxID != "tx1" || len(txj.Vin) != 1 || len(txj.Vout) != 1 {
		t.Fatalf("unexpected tx detail: %+v", txj)
	}
	if txj.Vout[0].Value != 0.000001 || txj.Vout[0].ScriptPubKey.Hex != "76a91401" {
		t.Fatalf("unexpected vout value: %+v", txj.Vout[0])
	}
}

func TestBitailsBaseURLForNetwork(t *testing.T) {
	if got := bitails.BaseURLForNetwork("main"); got != bitails.MainnetBaseURL {
		t.Fatalf("unexpected mainnet url: %s", got)
	}
	if got := bitails.BaseURLForNetwork("test"); got != bitails.TestnetBaseURL {
		t.Fatalf("unexpected testnet url: %s", got)
	}
}
