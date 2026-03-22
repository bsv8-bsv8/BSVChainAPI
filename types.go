package chainapi

import (
	"context"
	"strings"
)

const (
	// DefaultProfile 避免把“无 profile”与“默认 profile”当成两条路由。
	DefaultProfile = "default"
)

// API 是库主形态与端口客户端共同遵循的最小业务语义。
// 设计约束：只保留当前系统真正需要的四个链能力，不镜像上游原始 API。
type API interface {
	GetUTXOsContext(ctx context.Context, route Route, address string) ([]UTXO, error)
	GetTipHeightContext(ctx context.Context, route Route) (uint32, error)
	BroadcastContext(ctx context.Context, route Route, txHex string) (string, error)
	GetTxDetailContext(ctx context.Context, route Route, txid string) (TxDetail, error)
	GetRouteInfoContext(ctx context.Context, route Route) (RouteInfo, error)
}

type Route struct {
	Provider string `json:"provider"`
	Network  string `json:"network"`
	Profile  string `json:"profile,omitempty"`
}

type RouteInfo struct {
	Route        Route        `json:"route"`
	Capabilities []Capability `json:"capabilities"`
}

func (r Route) Normalize() Route {
	out := Route{
		Provider: strings.ToLower(strings.TrimSpace(r.Provider)),
		Network:  strings.ToLower(strings.TrimSpace(r.Network)),
		Profile:  strings.ToLower(strings.TrimSpace(r.Profile)),
	}
	if out.Network == "mainnet" {
		out.Network = "main"
	}
	if out.Network == "testnet" {
		out.Network = "test"
	}
	if out.Profile == "" {
		out.Profile = DefaultProfile
	}
	return out
}

func (r Route) Key() string {
	n := r.Normalize()
	return n.Provider + "|" + n.Network + "|" + n.Profile
}

type UTXO struct {
	TxID  string `json:"txid"`
	Vout  uint32 `json:"vout"`
	Value uint64 `json:"value"`
}

type TxDetail struct {
	TxID string     `json:"txid"`
	Vin  []TxInput  `json:"vin"`
	Vout []TxOutput `json:"vout"`
}

type TxInput struct {
	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`
}

type TxOutput struct {
	N            uint32       `json:"n"`
	Value        float64      `json:"value"`
	ScriptPubKey ScriptPubKey `json:"script_pub_key"`
}

type ScriptPubKey struct {
	Hex string `json:"hex"`
}
