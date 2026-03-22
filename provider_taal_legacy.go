package chainapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/bsv8/BSVChainAPI/internal/rawtx"
)

const (
	TAALLegacyProvider = "taal_legacy"

	taalLegacyBaseURL    = "https://api.taal.com"
	taalLegacySubmitPath = "/api/v1/broadcast"
)

type taalLegacyProvider struct{}

type taalLegacyEndpoint struct {
	route Route
	raw   *rawtx.Client
}

func NewTAALLegacyProvider() Provider {
	return taalLegacyProvider{}
}

func (taalLegacyProvider) Name() string {
	return TAALLegacyProvider
}

func (taalLegacyProvider) Capabilities() []Capability {
	return []Capability{CapabilityBroadcast}
}

func (taalLegacyProvider) NewEndpoint(cfg RouteConfig) (Endpoint, error) {
	route := cfg.Route()
	if route.Provider != TAALLegacyProvider {
		return nil, fmt.Errorf("unsupported provider: %s", route.Provider)
	}
	if route.Network != "main" {
		return nil, fmt.Errorf("taal_legacy only supports main network")
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = taalLegacyBaseURL
	}
	auth := rawtx.AuthConfig{
		Mode:  cfg.Auth.Mode,
		Name:  cfg.Auth.Name,
		Value: cfg.Auth.Value,
	}
	if strings.TrimSpace(auth.Value) != "" && strings.TrimSpace(auth.Mode) == "" {
		auth.Mode = "header"
		auth.Name = "Authorization"
	}
	return &taalLegacyEndpoint{
		route: route,
		raw:   rawtx.NewClient(baseURL, taalLegacySubmitPath, auth),
	}, nil
}

func (e *taalLegacyEndpoint) Capabilities() []Capability {
	return taalLegacyProvider{}.Capabilities()
}

func (e *taalLegacyEndpoint) GetUTXOsContext(ctx context.Context, address string) ([]UTXO, error) {
	return nil, unsupportedCapabilityError(e.route, CapabilityGetUTXOs)
}

func (e *taalLegacyEndpoint) GetTipHeightContext(ctx context.Context) (uint32, error) {
	return 0, unsupportedCapabilityError(e.route, CapabilityGetTipHeight)
}

func (e *taalLegacyEndpoint) BroadcastContext(ctx context.Context, txHex string) (string, error) {
	return e.raw.BroadcastContext(ctx, txHex)
}

func (e *taalLegacyEndpoint) GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error) {
	return TxDetail{}, unsupportedCapabilityError(e.route, CapabilityGetTxDetail)
}
