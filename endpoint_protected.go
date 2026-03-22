package chainapi

import (
	"context"
	"time"
)

// ProtectedEndpoint 把访问频率保护包装在上游 endpoint 外面。
// 设计说明：
// - provider 只负责协议适配；
// - 频率控制属于上游实例的外层壳，不侵入 provider 内部；
// - Manager 只持有包装后的 endpoint，不再自己管理节流细节。
type ProtectedEndpoint struct {
	inner Endpoint
	gate  *turnGate
}

func NewProtectedEndpoint(inner Endpoint, interval time.Duration) Endpoint {
	if inner == nil {
		return nil
	}
	if interval <= 0 {
		return inner
	}
	return &ProtectedEndpoint{
		inner: inner,
		gate:  newTurnGate(interval),
	}
}

func (e *ProtectedEndpoint) Capabilities() []Capability {
	if e == nil || e.inner == nil {
		return nil
	}
	return append([]Capability(nil), e.inner.Capabilities()...)
}

func (e *ProtectedEndpoint) GetUTXOsContext(ctx context.Context, address string) ([]UTXO, error) {
	if err := e.wait(ctx); err != nil {
		return nil, err
	}
	return e.inner.GetUTXOsContext(ctx, address)
}

func (e *ProtectedEndpoint) GetTipHeightContext(ctx context.Context) (uint32, error) {
	if err := e.wait(ctx); err != nil {
		return 0, err
	}
	return e.inner.GetTipHeightContext(ctx)
}

func (e *ProtectedEndpoint) BroadcastContext(ctx context.Context, txHex string) (string, error) {
	if err := e.wait(ctx); err != nil {
		return "", err
	}
	return e.inner.BroadcastContext(ctx, txHex)
}

func (e *ProtectedEndpoint) GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error) {
	if err := e.wait(ctx); err != nil {
		return TxDetail{}, err
	}
	return e.inner.GetTxDetailContext(ctx, txid)
}

func (e *ProtectedEndpoint) wait(ctx context.Context) error {
	if e == nil || e.gate == nil {
		return nil
	}
	return e.gate.Wait(ctx)
}
