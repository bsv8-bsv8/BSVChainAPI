package chainapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	Routes []RouteConfig `json:"routes"`
}

type RouteConfig struct {
	Provider string        `json:"provider"`
	Network  string        `json:"network"`
	Profile  string        `json:"profile,omitempty"`
	BaseURL  string        `json:"base_url,omitempty"`
	Auth     AuthConfig    `json:"auth,omitempty"`
	Protect  ProtectConfig `json:"protect,omitempty"`
}

func (c RouteConfig) Route() Route {
	return Route{
		Provider: c.Provider,
		Network:  c.Network,
		Profile:  c.Profile,
	}.Normalize()
}

func (c RouteConfig) Validate() error {
	route := c.Route()
	if route.Provider == "" {
		return fmt.Errorf("route provider is required")
	}
	if route.Network == "" {
		return fmt.Errorf("route network is required")
	}
	if err := c.Auth.Validate(); err != nil {
		return err
	}
	return nil
}

type ProtectConfig struct {
	// MinInterval 定义同一路由对同一上游实例的最小调用间隔。
	// 保护挂在 provider endpoint 外层壳上，以 route/profile 为桶，
	// 不做全局串行，避免不同上游彼此阻塞。
	MinInterval time.Duration `json:"min_interval,omitempty"`
}

type AuthConfig struct {
	Mode  string `json:"mode,omitempty"`
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

func (c AuthConfig) Validate() error {
	mode := strings.ToLower(strings.TrimSpace(c.Mode))
	switch mode {
	case "", "none":
		return nil
	case "header", "query":
		if strings.TrimSpace(c.Name) == "" {
			return fmt.Errorf("auth name is required for mode %s", mode)
		}
		if strings.TrimSpace(c.Value) == "" {
			return fmt.Errorf("auth value is required for mode %s", mode)
		}
		return nil
	case "bearer":
		if strings.TrimSpace(c.Value) == "" {
			return fmt.Errorf("auth value is required for mode bearer")
		}
		return nil
	default:
		return fmt.Errorf("unsupported auth mode: %s", mode)
	}
}

func (c AuthConfig) Apply(req *http.Request) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}
	mode := strings.ToLower(strings.TrimSpace(c.Mode))
	switch mode {
	case "", "none":
		return nil
	case "header":
		req.Header.Set(strings.TrimSpace(c.Name), strings.TrimSpace(c.Value))
		return nil
	case "query":
		q := req.URL.Query()
		q.Set(strings.TrimSpace(c.Name), strings.TrimSpace(c.Value))
		req.URL.RawQuery = q.Encode()
		return nil
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Value))
		return nil
	default:
		return fmt.Errorf("unsupported auth mode: %s", mode)
	}
}
