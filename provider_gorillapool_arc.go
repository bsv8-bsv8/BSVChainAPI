package chainapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/bsv8/BSVChainAPI/internal/rawtx"
)

const (
	GorillaPoolARCProvider = "gorillapool_arc"

	gorillaPoolARCBaseURL    = "https://arc.gorillapool.io"
	gorillaPoolARCSubmitPath = "/v1/tx"
)

type gorillaPoolARCProvider struct{}

type gorillaPoolARCEndpoint struct {
	route Route
	raw   *rawtx.Client
}

func NewGorillaPoolARCProvider() Provider {
	return gorillaPoolARCProvider{}
}

func (gorillaPoolARCProvider) Name() string {
	return GorillaPoolARCProvider
}

func (gorillaPoolARCProvider) Capabilities() []Capability {
	return []Capability{CapabilityBroadcast}
}

func (gorillaPoolARCProvider) NewEndpoint(cfg RouteConfig) (Endpoint, error) {
	route := cfg.Route()
	if route.Provider != GorillaPoolARCProvider {
		return nil, fmt.Errorf("unsupported provider: %s", route.Provider)
	}
	if route.Network != "main" {
		return nil, fmt.Errorf("gorillapool_arc only supports main network")
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = gorillaPoolARCBaseURL
	}
	auth := rawtx.AuthConfig{
		Mode:  cfg.Auth.Mode,
		Name:  cfg.Auth.Name,
		Value: cfg.Auth.Value,
	}
	if strings.TrimSpace(auth.Value) != "" && strings.TrimSpace(auth.Mode) == "" {
		auth.Mode = "bearer"
	}
	return &gorillaPoolARCEndpoint{
		route: route,
		raw:   rawtx.NewClient(baseURL, gorillaPoolARCSubmitPath, auth),
	}, nil
}

func (e *gorillaPoolARCEndpoint) Capabilities() []Capability {
	return gorillaPoolARCProvider{}.Capabilities()
}

func (e *gorillaPoolARCEndpoint) GetUTXOsContext(ctx context.Context, address string) ([]UTXO, error) {
	return nil, unsupportedCapabilityError(e.route, CapabilityGetUTXOs)
}

func (e *gorillaPoolARCEndpoint) GetTipHeightContext(ctx context.Context) (uint32, error) {
	return 0, unsupportedCapabilityError(e.route, CapabilityGetTipHeight)
}

func (e *gorillaPoolARCEndpoint) BroadcastContext(ctx context.Context, txHex string) (string, error) {
	return e.raw.BroadcastContext(ctx, txHex)
}

func (e *gorillaPoolARCEndpoint) GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error) {
	return TxDetail{}, unsupportedCapabilityError(e.route, CapabilityGetTxDetail)
}
