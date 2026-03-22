package chainapi

import (
	"context"
	"fmt"
)

// UTXORouter 当前是“单 route 读 UTXO”的最小实现。
// 以后如果需要多上游编排，可以在这里扩展而不影响业务调用方。
type UTXORouter struct {
	api   API
	route Route
}

func NewUTXORouter(ctx context.Context, api API, route Route) (*UTXORouter, error) {
	normalized, err := requireRouteCapability(ctx, api, route, CapabilityGetUTXOs)
	if err != nil {
		return nil, err
	}
	return &UTXORouter{api: api, route: normalized}, nil
}

func (r *UTXORouter) GetUTXOsContext(ctx context.Context, address string) ([]UTXO, error) {
	if r == nil || r.api == nil {
		return nil, fmt.Errorf("utxo router is nil")
	}
	return r.api.GetUTXOsContext(ctxOrBackground(ctx), r.route, address)
}

func (r *UTXORouter) Route() Route {
	if r == nil {
		return Route{}
	}
	return r.route
}

// TipHeightRouter 当前是“单 route 读 tip height”的最小实现。
type TipHeightRouter struct {
	api   API
	route Route
}

func NewTipHeightRouter(ctx context.Context, api API, route Route) (*TipHeightRouter, error) {
	normalized, err := requireRouteCapability(ctx, api, route, CapabilityGetTipHeight)
	if err != nil {
		return nil, err
	}
	return &TipHeightRouter{api: api, route: normalized}, nil
}

func (r *TipHeightRouter) GetTipHeightContext(ctx context.Context) (uint32, error) {
	if r == nil || r.api == nil {
		return 0, fmt.Errorf("tip height router is nil")
	}
	return r.api.GetTipHeightContext(ctxOrBackground(ctx), r.route)
}

func (r *TipHeightRouter) Route() Route {
	if r == nil {
		return Route{}
	}
	return r.route
}

// TxDetailRouter 当前是“单 route 查 tx detail”的最小实现。
type TxDetailRouter struct {
	api   API
	route Route
}

func NewTxDetailRouter(ctx context.Context, api API, route Route) (*TxDetailRouter, error) {
	normalized, err := requireRouteCapability(ctx, api, route, CapabilityGetTxDetail)
	if err != nil {
		return nil, err
	}
	return &TxDetailRouter{api: api, route: normalized}, nil
}

func (r *TxDetailRouter) GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error) {
	if r == nil || r.api == nil {
		return TxDetail{}, fmt.Errorf("tx detail router is nil")
	}
	return r.api.GetTxDetailContext(ctxOrBackground(ctx), r.route, txid)
}

func (r *TxDetailRouter) Route() Route {
	if r == nil {
		return Route{}
	}
	return r.route
}

func requireRouteCapability(ctx context.Context, api API, route Route, cap Capability) (Route, error) {
	if api == nil {
		return Route{}, fmt.Errorf("api is nil")
	}
	normalized := route.Normalize()
	if normalized.Provider == "" || normalized.Network == "" {
		return Route{}, fmt.Errorf("route provider and network are required")
	}
	info, err := api.GetRouteInfoContext(ctxOrBackground(ctx), normalized)
	if err != nil {
		return Route{}, fmt.Errorf("get route info failed for %s: %w", normalized.Key(), err)
	}
	if !hasCapability(info.Capabilities, cap) {
		return Route{}, unsupportedCapabilityError(normalized, cap)
	}
	return normalized, nil
}
