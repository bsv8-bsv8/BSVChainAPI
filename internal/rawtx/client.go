package rawtx

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type AuthConfig struct {
	Mode  string
	Name  string
	Value string
}

type Client struct {
	baseURL    string
	submitPath string
	auth       AuthConfig
	http       *http.Client
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e == nil {
		return "http error"
	}
	return fmt.Sprintf("http %d: %s", e.StatusCode, e.Body)
}

func (e *HTTPError) HTTPStatus() int {
	if e == nil {
		return 0
	}
	return e.StatusCode
}

func (e *HTTPError) HTTPBody() string {
	if e == nil {
		return ""
	}
	return e.Body
}

func NewClient(baseURL, submitPath string, auth AuthConfig) *Client {
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		submitPath: normalizePath(submitPath),
		auth:       auth,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) BroadcastContext(ctx context.Context, txHex string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("client is nil")
	}
	txHex = strings.TrimSpace(txHex)
	if txHex == "" {
		return "", fmt.Errorf("tx_hex is required")
	}
	raw, err := hex.DecodeString(txHex)
	if err != nil {
		return "", fmt.Errorf("decode tx_hex: %w", err)
	}
	req, err := http.NewRequestWithContext(ctxOrBackground(ctx), http.MethodPost, c.baseURL+c.submitPath, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if err := c.auth.Apply(req); err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", &HTTPError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	txid, err := parseTxID(body)
	if err != nil {
		return "", err
	}
	return txid, nil
}

func (a AuthConfig) Apply(req *http.Request) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}
	mode := strings.ToLower(strings.TrimSpace(a.Mode))
	switch mode {
	case "", "none":
		return nil
	case "header":
		name := strings.TrimSpace(a.Name)
		if name == "" {
			return fmt.Errorf("auth name is required for mode header")
		}
		if strings.TrimSpace(a.Value) == "" {
			return fmt.Errorf("auth value is required for mode header")
		}
		req.Header.Set(name, strings.TrimSpace(a.Value))
		return nil
	case "query":
		name := strings.TrimSpace(a.Name)
		if name == "" {
			return fmt.Errorf("auth name is required for mode query")
		}
		if strings.TrimSpace(a.Value) == "" {
			return fmt.Errorf("auth value is required for mode query")
		}
		q := req.URL.Query()
		q.Set(name, strings.TrimSpace(a.Value))
		req.URL.RawQuery = q.Encode()
		return nil
	case "bearer":
		if strings.TrimSpace(a.Value) == "" {
			return fmt.Errorf("auth value is required for mode bearer")
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(a.Value))
		return nil
	default:
		return fmt.Errorf("unsupported auth mode: %s", mode)
	}
}

func normalizePath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return "/v1/tx"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func parseTxID(body []byte) (string, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "", fmt.Errorf("broadcast response missing txid")
	}
	var txid string
	if err := json.Unmarshal(body, &txid); err == nil {
		if normalized := normalizeHexID(txid); normalized != "" {
			return normalized, nil
		}
	}
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err == nil {
		for _, key := range []string{"txid", "txId", "hash", "data", "result"} {
			if v, ok := obj[key].(string); ok {
				if normalized := normalizeHexID(v); normalized != "" {
					return normalized, nil
				}
			}
		}
	}
	if normalized := normalizeHexID(trimmed); normalized != "" {
		return normalized, nil
	}
	return "", fmt.Errorf("unexpected broadcast response: %s", trimmed)
}

func normalizeHexID(in string) string {
	v := strings.ToLower(strings.TrimSpace(in))
	if len(v) != 64 {
		return ""
	}
	if _, err := hex.DecodeString(v); err != nil {
		return ""
	}
	return v
}

func ctxOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
