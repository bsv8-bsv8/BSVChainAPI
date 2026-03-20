package chainapi

import (
	"context"
	"fmt"
	"strings"
)

type Manager struct {
	routes map[string]*routeRuntime
}

type routeRuntime struct {
	route    Route
	endpoint Endpoint
	gate     *turnGate
}

func NewManager(cfg Config, extraProviders ...Provider) (*Manager, error) {
	providers := append([]Provider{
		NewWhatsOnChainProvider(),
		NewBitailsProvider(),
	}, extraProviders...)
	return NewManagerWithProviders(cfg, providers...)
}

func NewManagerWithProviders(cfg Config, providers ...Provider) (*Manager, error) {
	if len(cfg.Routes) == 0 {
		return nil, fmt.Errorf("at least one route is required")
	}
	registry := map[string]Provider{}
	for _, p := range providers {
		if p == nil {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(p.Name()))
		if name == "" {
			return nil, fmt.Errorf("provider name is required")
		}
		registry[name] = p
	}
	out := &Manager{routes: map[string]*routeRuntime{}}
	for _, rc := range cfg.Routes {
		if err := rc.Validate(); err != nil {
			return nil, err
		}
		route := rc.Route()
		key := route.Key()
		if _, exists := out.routes[key]; exists {
			return nil, fmt.Errorf("duplicate route: %s", key)
		}
		provider := registry[route.Provider]
		if provider == nil {
			return nil, fmt.Errorf("provider not registered: %s", route.Provider)
		}
		endpoint, err := provider.NewEndpoint(rc)
		if err != nil {
			return nil, err
		}
		out.routes[key] = &routeRuntime{
			route:    route,
			endpoint: endpoint,
			gate:     newTurnGate(rc.Protect.MinInterval),
		}
	}
	return out, nil
}

func (m *Manager) GetUTXOs(route Route, address string) ([]UTXO, error) {
	return m.GetUTXOsContext(context.Background(), route, address)
}

func (m *Manager) GetUTXOsContext(ctx context.Context, route Route, address string) ([]UTXO, error) {
	rt, err := m.resolve(route)
	if err != nil {
		return nil, err
	}
	if err := rt.gate.Wait(ctx); err != nil {
		return nil, err
	}
	return rt.endpoint.GetUTXOsContext(ctx, address)
}

func (m *Manager) GetTipHeight(route Route) (uint32, error) {
	return m.GetTipHeightContext(context.Background(), route)
}

func (m *Manager) GetTipHeightContext(ctx context.Context, route Route) (uint32, error) {
	rt, err := m.resolve(route)
	if err != nil {
		return 0, err
	}
	if err := rt.gate.Wait(ctx); err != nil {
		return 0, err
	}
	return rt.endpoint.GetTipHeightContext(ctx)
}

func (m *Manager) Broadcast(route Route, txHex string) (string, error) {
	return m.BroadcastContext(context.Background(), route, txHex)
}

func (m *Manager) BroadcastContext(ctx context.Context, route Route, txHex string) (string, error) {
	rt, err := m.resolve(route)
	if err != nil {
		return "", err
	}
	if err := rt.gate.Wait(ctx); err != nil {
		return "", err
	}
	return rt.endpoint.BroadcastContext(ctx, txHex)
}

func (m *Manager) GetTxDetail(route Route, txid string) (TxDetail, error) {
	return m.GetTxDetailContext(context.Background(), route, txid)
}

func (m *Manager) GetTxDetailContext(ctx context.Context, route Route, txid string) (TxDetail, error) {
	rt, err := m.resolve(route)
	if err != nil {
		return TxDetail{}, err
	}
	if err := rt.gate.Wait(ctx); err != nil {
		return TxDetail{}, err
	}
	return rt.endpoint.GetTxDetailContext(ctx, txid)
}

func (m *Manager) resolve(route Route) (*routeRuntime, error) {
	if m == nil {
		return nil, fmt.Errorf("manager is nil")
	}
	key := route.Key()
	rt := m.routes[key]
	if rt == nil {
		return nil, fmt.Errorf("route not found: %s", key)
	}
	return rt, nil
}
