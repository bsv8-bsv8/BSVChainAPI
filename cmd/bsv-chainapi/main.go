package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bsv8/BSVChainAPI"
)

type runtimeConfig struct {
	Listen string          `json:"listen"`
	Routes []routeFileItem `json:"routes"`
}

type routeFileItem struct {
	Provider string `json:"provider"`
	Network  string `json:"network"`
	Profile  string `json:"profile,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
	Auth     struct {
		Mode  string `json:"mode,omitempty"`
		Name  string `json:"name,omitempty"`
		Value string `json:"value,omitempty"`
	} `json:"auth,omitempty"`
	Protect struct {
		MinInterval string `json:"min_interval,omitempty"`
	} `json:"protect,omitempty"`
}

func main() {
	configPath := flag.String("config", "", "runtime config file path")
	listen := flag.String("listen", "", "listen address override")
	flag.Parse()

	if strings.TrimSpace(*configPath) == "" {
		fmt.Fprintln(os.Stderr, "config is required")
		os.Exit(2)
	}
	cfg, err := loadRuntimeConfig(strings.TrimSpace(*configPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}
	if v := strings.TrimSpace(*listen); v != "" {
		cfg.Listen = v
	}
	if strings.TrimSpace(cfg.Listen) == "" {
		cfg.Listen = "127.0.0.1:18222"
	}

	managerCfg, err := toManagerConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(1)
	}
	manager, err := chainapi.NewManager(managerCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build manager failed: %v\n", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           chainapi.NewPortServer(manager).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Printf("bsv-chainapi listening on %s\n", cfg.Listen)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server failed: %v\n", err)
		os.Exit(1)
	}
}

func loadRuntimeConfig(path string) (runtimeConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return runtimeConfig{}, err
	}
	var cfg runtimeConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return runtimeConfig{}, err
	}
	return cfg, nil
}

func toManagerConfig(cfg runtimeConfig) (chainapi.Config, error) {
	out := chainapi.Config{Routes: make([]chainapi.RouteConfig, 0, len(cfg.Routes))}
	for _, item := range cfg.Routes {
		var interval time.Duration
		if strings.TrimSpace(item.Protect.MinInterval) != "" {
			v, err := time.ParseDuration(strings.TrimSpace(item.Protect.MinInterval))
			if err != nil {
				return chainapi.Config{}, fmt.Errorf("invalid min_interval: %w", err)
			}
			interval = v
		}
		out.Routes = append(out.Routes, chainapi.RouteConfig{
			Provider: item.Provider,
			Network:  item.Network,
			Profile:  item.Profile,
			BaseURL:  item.BaseURL,
			Auth: chainapi.AuthConfig{
				Mode:  item.Auth.Mode,
				Name:  item.Auth.Name,
				Value: item.Auth.Value,
			},
			Protect: chainapi.ProtectConfig{
				MinInterval: interval,
			},
		})
	}
	return out, nil
}
