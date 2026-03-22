package chainapi

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBroadcastOnlyProviders(t *testing.T) {
	txHex := "01020304"
	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		t.Fatalf("decode sample tx failed: %v", err)
	}
	expectedTxID, err := computeTxID(txHex)
	if err != nil {
		t.Fatalf("compute txid failed: %v", err)
	}

	t.Run("gorillapool_arc", func(t *testing.T) {
		var seenAuth string
		var seenBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seenAuth = r.Header.Get("Authorization")
			if r.URL.Path != "/v1/tx" {
				http.NotFound(w, r)
				return
			}
			seenBody, _ = io.ReadAll(r.Body)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":   200,
				"title":    "OK",
				"txid":     expectedTxID,
				"txStatus": "SEEN_ON_NETWORK",
			})
		}))
		defer srv.Close()

		endpoint, err := gorillaPoolARCProvider{}.NewEndpoint(RouteConfig{
			Provider: GorillaPoolARCProvider,
			Network:  "main",
			BaseURL:  srv.URL,
			Auth: AuthConfig{
				Value: "secret",
			},
		})
		if err != nil {
			t.Fatalf("new endpoint failed: %v", err)
		}
		txid, err := endpoint.BroadcastContext(context.Background(), txHex)
		if err != nil {
			t.Fatalf("broadcast failed: %v", err)
		}
		if txid != expectedTxID {
			t.Fatalf("unexpected txid: %s", txid)
		}
		if seenAuth != "Bearer secret" {
			t.Fatalf("unexpected auth header: %q", seenAuth)
		}
		if string(seenBody) != string(txBytes) {
			t.Fatalf("unexpected body bytes")
		}
	})

	t.Run("taal_arc", func(t *testing.T) {
		var seenAuth string
		var seenBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seenAuth = r.Header.Get("Authorization")
			if r.URL.Path != "/v1/tx" {
				http.NotFound(w, r)
				return
			}
			seenBody, _ = io.ReadAll(r.Body)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":   200,
				"title":    "OK",
				"txid":     expectedTxID,
				"txStatus": "SEEN_ON_NETWORK",
			})
		}))
		defer srv.Close()

		endpoint, err := taalARCProvider{}.NewEndpoint(RouteConfig{
			Provider: TAALARCProvider,
			Network:  "main",
			BaseURL:  srv.URL,
			Auth: AuthConfig{
				Value: "secret",
			},
		})
		if err != nil {
			t.Fatalf("new endpoint failed: %v", err)
		}
		txid, err := endpoint.BroadcastContext(context.Background(), txHex)
		if err != nil {
			t.Fatalf("broadcast failed: %v", err)
		}
		if txid != expectedTxID {
			t.Fatalf("unexpected txid: %s", txid)
		}
		if seenAuth != "Bearer secret" {
			t.Fatalf("unexpected auth header: %q", seenAuth)
		}
		if string(seenBody) != string(txBytes) {
			t.Fatalf("unexpected body bytes")
		}
	})

	t.Run("taal_legacy", func(t *testing.T) {
		var seenAuth string
		var seenBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seenAuth = r.Header.Get("Authorization")
			if r.URL.Path != "/api/v1/broadcast" {
				http.NotFound(w, r)
				return
			}
			seenBody, _ = io.ReadAll(r.Body)
			_, _ = w.Write([]byte(`"` + expectedTxID + `"`))
		}))
		defer srv.Close()

		endpoint, err := taalLegacyProvider{}.NewEndpoint(RouteConfig{
			Provider: TAALLegacyProvider,
			Network:  "main",
			BaseURL:  srv.URL,
			Auth: AuthConfig{
				Value: "legacy-key",
			},
		})
		if err != nil {
			t.Fatalf("new endpoint failed: %v", err)
		}
		txid, err := endpoint.BroadcastContext(context.Background(), txHex)
		if err != nil {
			t.Fatalf("broadcast failed: %v", err)
		}
		if txid != expectedTxID {
			t.Fatalf("unexpected txid: %s", txid)
		}
		if seenAuth != "legacy-key" {
			t.Fatalf("unexpected auth header: %q", seenAuth)
		}
		if string(seenBody) != string(txBytes) {
			t.Fatalf("unexpected body bytes")
		}
	})
}

func TestBroadcastOnlyProvidersRejectNonMain(t *testing.T) {
	if _, err := (gorillaPoolARCProvider{}).NewEndpoint(RouteConfig{Provider: GorillaPoolARCProvider, Network: "test"}); err == nil {
		t.Fatalf("expected network rejection")
	}
	if _, err := (taalARCProvider{}).NewEndpoint(RouteConfig{Provider: TAALARCProvider, Network: "test"}); err == nil {
		t.Fatalf("expected network rejection")
	}
	if _, err := (taalLegacyProvider{}).NewEndpoint(RouteConfig{Provider: TAALLegacyProvider, Network: "test"}); err == nil {
		t.Fatalf("expected network rejection")
	}
}
