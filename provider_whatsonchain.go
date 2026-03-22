package chainapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/bsv8/BSVChainAPI/internal/whatsonchain"
)

const (
	WhatsOnChainProvider = "whatsonchain"
)

type whatsOnChainProvider struct{}

type whatsOnChainEndpoint struct {
	raw *whatsonchain.Client
}

func NewWhatsOnChainProvider() Provider {
	return whatsOnChainProvider{}
}

func (whatsOnChainProvider) Name() string {
	return WhatsOnChainProvider
}

func (whatsOnChainProvider) Capabilities() []Capability {
	return []Capability{
		CapabilityBroadcast,
		CapabilityGetTipHeight,
		CapabilityGetTxDetail,
		CapabilityGetUTXOs,
	}
}

func (whatsOnChainProvider) NewEndpoint(cfg RouteConfig) (Endpoint, error) {
	route := cfg.Route()
	if route.Provider != WhatsOnChainProvider {
		return nil, fmt.Errorf("unsupported provider: %s", route.Provider)
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = whatsonchain.BaseURLForNetwork(route.Network)
	}
	return &whatsOnChainEndpoint{
		raw: whatsonchain.NewClient(baseURL, whatsonchain.AuthConfig{
			Mode:  cfg.Auth.Mode,
			Name:  cfg.Auth.Name,
			Value: cfg.Auth.Value,
		}),
	}, nil
}

func (e *whatsOnChainEndpoint) Capabilities() []Capability {
	return whatsOnChainProvider{}.Capabilities()
}

func (e *whatsOnChainEndpoint) GetUTXOsContext(ctx context.Context, address string) ([]UTXO, error) {
	items, err := e.raw.GetUTXOsContext(ctx, address)
	if err != nil {
		return nil, err
	}
	out := make([]UTXO, 0, len(items))
	for _, item := range items {
		out = append(out, UTXO{
			TxID:  item.TxID,
			Vout:  item.Vout,
			Value: item.Value,
		})
	}
	return out, nil
}

func (e *whatsOnChainEndpoint) GetTipHeightContext(ctx context.Context) (uint32, error) {
	return e.raw.GetTipHeightContext(ctx)
}

func (e *whatsOnChainEndpoint) BroadcastContext(ctx context.Context, txHex string) (string, error) {
	return e.raw.BroadcastContext(ctx, txHex)
}

func (e *whatsOnChainEndpoint) GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error) {
	rawTx, err := e.raw.GetTxDetailContext(ctx, txid)
	if err != nil {
		return TxDetail{}, err
	}
	out := TxDetail{
		TxID: rawTx.TxID,
		Vin:  make([]TxInput, 0, len(rawTx.Vin)),
		Vout: make([]TxOutput, 0, len(rawTx.Vout)),
	}
	for _, item := range rawTx.Vin {
		out.Vin = append(out.Vin, TxInput{
			TxID: item.TxID,
			Vout: item.Vout,
		})
	}
	for _, item := range rawTx.Vout {
		out.Vout = append(out.Vout, TxOutput{
			N:     item.N,
			Value: item.Value,
			ScriptPubKey: ScriptPubKey{
				Hex: item.ScriptPubKey.Hex,
			},
		})
	}
	return out, nil
}
