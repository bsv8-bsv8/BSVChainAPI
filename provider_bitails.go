package chainapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/bsv8/BSVChainAPI/internal/bitails"
)

const (
	BitailsProvider = "bitails"
)

type bitailsProvider struct{}

type bitailsEndpoint struct {
	raw *bitails.Client
}

func NewBitailsProvider() Provider {
	return bitailsProvider{}
}

func (bitailsProvider) Name() string {
	return BitailsProvider
}

func (bitailsProvider) NewEndpoint(cfg RouteConfig) (Endpoint, error) {
	route := cfg.Route()
	if route.Provider != BitailsProvider {
		return nil, fmt.Errorf("unsupported provider: %s", route.Provider)
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = bitails.BaseURLForNetwork(route.Network)
	}
	auth := bitails.AuthConfig{
		Mode:  cfg.Auth.Mode,
		Name:  cfg.Auth.Name,
		Value: cfg.Auth.Value,
	}
	if strings.TrimSpace(auth.Value) != "" && strings.TrimSpace(auth.Mode) == "" {
		auth.Mode = "header"
	}
	if strings.EqualFold(strings.TrimSpace(auth.Mode), "header") && strings.TrimSpace(auth.Name) == "" {
		auth.Name = "apikey"
	}
	return &bitailsEndpoint{
		raw: bitails.NewClient(baseURL, auth),
	}, nil
}

func (e *bitailsEndpoint) GetUTXOsContext(ctx context.Context, address string) ([]UTXO, error) {
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

func (e *bitailsEndpoint) GetTipHeightContext(ctx context.Context) (uint32, error) {
	return e.raw.GetTipHeightContext(ctx)
}

func (e *bitailsEndpoint) BroadcastContext(ctx context.Context, txHex string) (string, error) {
	return e.raw.BroadcastContext(ctx, txHex)
}

func (e *bitailsEndpoint) GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error) {
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
