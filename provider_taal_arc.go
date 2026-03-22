package chainapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/bsv8/BSVChainAPI/internal/rawtx"
)

const (
	TAALARCProvider = "taal_arc"

	taalARCBaseURL    = "https://arc.taal.com"
	taalARCSubmitPath = "/v1/tx"
)

type taalARCProvider struct{}

type taalARCEndpoint struct {
	route Route
	raw   *rawtx.Client
}

func NewTAALARCProvider() Provider {
	return taalARCProvider{}
}

func (taalARCProvider) Name() string {
	return TAALARCProvider
}

func (taalARCProvider) Capabilities() []Capability {
	return []Capability{CapabilityBroadcast}
}

func (taalARCProvider) NewEndpoint(cfg RouteConfig) (Endpoint, error) {
	route := cfg.Route()
	if route.Provider != TAALARCProvider {
		return nil, fmt.Errorf("unsupported provider: %s", route.Provider)
	}
	if route.Network != "main" {
		return nil, fmt.Errorf("taal_arc only supports main network")
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = taalARCBaseURL
	}
	auth := rawtx.AuthConfig{
		Mode:  cfg.Auth.Mode,
		Name:  cfg.Auth.Name,
		Value: cfg.Auth.Value,
	}
	if strings.TrimSpace(auth.Value) != "" && strings.TrimSpace(auth.Mode) == "" {
		auth.Mode = "bearer"
	}
	return &taalARCEndpoint{
		route: route,
		raw:   rawtx.NewClient(baseURL, taalARCSubmitPath, auth),
	}, nil
}

func (e *taalARCEndpoint) Capabilities() []Capability {
	return taalARCProvider{}.Capabilities()
}

func (e *taalARCEndpoint) GetUTXOsContext(ctx context.Context, address string) ([]UTXO, error) {
	return nil, unsupportedCapabilityError(e.route, CapabilityGetUTXOs)
}

func (e *taalARCEndpoint) GetTipHeightContext(ctx context.Context) (uint32, error) {
	return 0, unsupportedCapabilityError(e.route, CapabilityGetTipHeight)
}

func (e *taalARCEndpoint) BroadcastContext(ctx context.Context, txHex string) (string, error) {
	return e.raw.BroadcastContext(ctx, txHex)
}

func (e *taalARCEndpoint) GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error) {
	return TxDetail{}, unsupportedCapabilityError(e.route, CapabilityGetTxDetail)
}
