package chainapi

import "context"

// Endpoint 是单个 route/profile 绑定后的上游适配实例。
// 设计约束：provider 负责兼容上游细节；业务层只看统一输出。
type Endpoint interface {
	Capabilities() []Capability
	GetUTXOsContext(ctx context.Context, address string) ([]UTXO, error)
	GetTipHeightContext(ctx context.Context) (uint32, error)
	BroadcastContext(ctx context.Context, txHex string) (string, error)
	GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error)
}

type Provider interface {
	Name() string
	NewEndpoint(cfg RouteConfig) (Endpoint, error)
}
