package chainapi

import (
	"context"
	"fmt"
)

// UTXOReader 是面向业务的 UTXO 读取能力，不再暴露 route 细节。
type UTXOReader interface {
	GetUTXOsContext(ctx context.Context, address string) ([]UTXO, error)
}

// TipHeightReader 是面向业务的链高度读取能力。
type TipHeightReader interface {
	GetTipHeightContext(ctx context.Context) (uint32, error)
}

// TxDetailReader 是面向业务的交易详情读取能力。
type TxDetailReader interface {
	GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error)
}

// TxSubmitter 是面向业务的交易提交能力。
type TxSubmitter interface {
	SubmitTxContext(ctx context.Context, txHex string) (TxSubmitResult, error)
}

// CapabilityAPI 是子项目应该依赖的统一能力入口。
// 它屏蔽 route/provider 细节，稳定承载读链与提交交易 4 个能力。
type CapabilityAPI interface {
	UTXOReader
	TipHeightReader
	TxDetailReader
	TxSubmitter
}

// CapabilityPlan 描述 4 个业务能力各自绑定到哪条 route 或策略。
// 这样未来切换供应商时，只需要在装配阶段调整这里。
type CapabilityPlan struct {
	UTXORoute      Route          `json:"utxo_route"`
	TipHeightRoute Route          `json:"tip_height_route"`
	TxDetailRoute  Route          `json:"tx_detail_route"`
	TxSubmitPolicy TxSubmitPolicy `json:"tx_submit_policy"`
}

// SingleReadRouteCapabilityPlan 用于“读链三能力共用一个 route”的常见场景。
// 费用池当前就是这种模型：UTXO / tip / tx detail 都走同一个读链上游。
func SingleReadRouteCapabilityPlan(readRoute Route, txSubmitPolicy TxSubmitPolicy) CapabilityPlan {
	return CapabilityPlan{
		UTXORoute:      readRoute,
		TipHeightRoute: readRoute,
		TxDetailRoute:  readRoute,
		TxSubmitPolicy: txSubmitPolicy,
	}
}

// CapabilityManager 是业务真正应该依赖的上层能力管理器。
// 下层 Manager 仍然保留，但它只负责“给定 route 执行一次”。
type CapabilityManager struct {
	utxos     *UTXORouter
	tipHeight *TipHeightRouter
	txDetail  *TxDetailRouter
	txSubmit  *TxSubmitRouter
	plan      CapabilityPlan
}

func NewCapabilityManager(ctx context.Context, api API, plan CapabilityPlan) (*CapabilityManager, error) {
	if api == nil {
		return nil, fmt.Errorf("api is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	normalized, err := normalizeCapabilityPlan(plan)
	if err != nil {
		return nil, err
	}
	utxos, err := NewUTXORouter(ctx, api, normalized.UTXORoute)
	if err != nil {
		return nil, err
	}
	tipHeight, err := NewTipHeightRouter(ctx, api, normalized.TipHeightRoute)
	if err != nil {
		return nil, err
	}
	txDetail, err := NewTxDetailRouter(ctx, api, normalized.TxDetailRoute)
	if err != nil {
		return nil, err
	}
	txSubmit, err := NewTxSubmitRouter(ctx, api, normalized.TxSubmitPolicy)
	if err != nil {
		return nil, err
	}
	return &CapabilityManager{
		utxos:     utxos,
		tipHeight: tipHeight,
		txDetail:  txDetail,
		txSubmit:  txSubmit,
		plan:      normalized,
	}, nil
}

func (m *CapabilityManager) GetUTXOs(address string) ([]UTXO, error) {
	return m.GetUTXOsContext(context.Background(), address)
}

func (m *CapabilityManager) GetUTXOsContext(ctx context.Context, address string) ([]UTXO, error) {
	if m == nil || m.utxos == nil {
		return nil, fmt.Errorf("capability manager is nil")
	}
	return m.utxos.GetUTXOsContext(ctx, address)
}

func (m *CapabilityManager) GetTipHeight() (uint32, error) {
	return m.GetTipHeightContext(context.Background())
}

func (m *CapabilityManager) GetTipHeightContext(ctx context.Context) (uint32, error) {
	if m == nil || m.tipHeight == nil {
		return 0, fmt.Errorf("capability manager is nil")
	}
	return m.tipHeight.GetTipHeightContext(ctx)
}

func (m *CapabilityManager) GetTxDetail(txid string) (TxDetail, error) {
	return m.GetTxDetailContext(context.Background(), txid)
}

func (m *CapabilityManager) GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error) {
	if m == nil || m.txDetail == nil {
		return TxDetail{}, fmt.Errorf("capability manager is nil")
	}
	return m.txDetail.GetTxDetailContext(ctx, txid)
}

func (m *CapabilityManager) SubmitTx(txHex string) (TxSubmitResult, error) {
	return m.SubmitTxContext(context.Background(), txHex)
}

func (m *CapabilityManager) SubmitTxContext(ctx context.Context, txHex string) (TxSubmitResult, error) {
	if m == nil || m.txSubmit == nil {
		return TxSubmitResult{}, fmt.Errorf("capability manager is nil")
	}
	return m.txSubmit.SubmitTxContext(ctx, txHex)
}

func (m *CapabilityManager) Plan() CapabilityPlan {
	if m == nil {
		return CapabilityPlan{}
	}
	return copyCapabilityPlan(m.plan)
}

func normalizeCapabilityPlan(plan CapabilityPlan) (CapabilityPlan, error) {
	utxoRoute, err := normalizeCapabilityRoute(plan.UTXORoute, "utxo")
	if err != nil {
		return CapabilityPlan{}, err
	}
	tipRoute, err := normalizeCapabilityRoute(plan.TipHeightRoute, "tip_height")
	if err != nil {
		return CapabilityPlan{}, err
	}
	txDetailRoute, err := normalizeCapabilityRoute(plan.TxDetailRoute, "tx_detail")
	if err != nil {
		return CapabilityPlan{}, err
	}
	submitRoutes, err := normalizeTxSubmitPolicyRoutes(plan.TxSubmitPolicy.Routes)
	if err != nil {
		return CapabilityPlan{}, err
	}
	return CapabilityPlan{
		UTXORoute:      utxoRoute,
		TipHeightRoute: tipRoute,
		TxDetailRoute:  txDetailRoute,
		TxSubmitPolicy: TxSubmitPolicy{Routes: submitRoutes},
	}, nil
}

func normalizeCapabilityRoute(route Route, label string) (Route, error) {
	n := route.Normalize()
	if n.Provider == "" || n.Network == "" {
		return Route{}, fmt.Errorf("%s route provider and network are required", label)
	}
	return n, nil
}

func copyCapabilityPlan(plan CapabilityPlan) CapabilityPlan {
	return CapabilityPlan{
		UTXORoute:      plan.UTXORoute,
		TipHeightRoute: plan.TipHeightRoute,
		TxDetailRoute:  plan.TxDetailRoute,
		TxSubmitPolicy: TxSubmitPolicy{
			Routes: append([]Route(nil), plan.TxSubmitPolicy.Routes...),
		},
	}
}
