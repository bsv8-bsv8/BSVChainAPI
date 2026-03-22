package whatsonchain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	TestnetBaseURL = "https://api.whatsonchain.com/v1/bsv/test"
	MainnetBaseURL = "https://api.whatsonchain.com/v1/bsv/main"
)

func BaseURLForNetwork(network string) string {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "main":
		return MainnetBaseURL
	default:
		return TestnetBaseURL
	}
}

type Client struct {
	baseURL string
	auth    AuthConfig
	http    *http.Client
}

type AuthConfig struct {
	Mode  string
	Name  string
	Value string
}

type UTXO struct {
	TxID  string
	Vout  uint32
	Value uint64
}

type TxDetail struct {
	TxID string
	Vin  []TxInput
	Vout []TxOutput
}

type TxInput struct {
	TxID string
	Vout uint32
}

type TxOutput struct {
	N            uint32
	Value        float64
	ScriptPubKey ScriptPubKey
}

type ScriptPubKey struct {
	Hex string
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

func NewClient(baseURL string, auth AuthConfig) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = TestnetBaseURL
	}
	return &Client{
		baseURL: baseURL,
		auth:    auth,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) BaseURL() string {
	if c == nil {
		return ""
	}
	return c.baseURL
}

func (c *Client) GetUTXOsContext(ctx context.Context, address string) ([]UTXO, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, fmt.Errorf("address is required")
	}
	body, err := c.get(ctx, "/address/"+address+"/confirmed/unspent")
	if err != nil {
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusNotFound {
			return nil, err
		}
		body, err = c.get(ctx, "/address/"+address+"/unspent")
		if err != nil {
			return nil, err
		}
	}
	var raw []struct {
		TxID               string `json:"tx_hash"`
		Vout               uint32 `json:"tx_pos"`
		Value              uint64 `json:"value"`
		IsSpentInMempoolTx bool   `json:"isSpentInMempoolTx"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		var wrapped struct {
			Result []struct {
				TxID               string `json:"tx_hash"`
				Vout               uint32 `json:"tx_pos"`
				Value              uint64 `json:"value"`
				IsSpentInMempoolTx bool   `json:"isSpentInMempoolTx"`
			} `json:"result"`
		}
		if wrapErr := json.Unmarshal(body, &wrapped); wrapErr != nil {
			return nil, fmt.Errorf("decode utxos: %w", err)
		}
		raw = wrapped.Result
	}
	out := make([]UTXO, 0, len(raw))
	for _, item := range raw {
		if item.IsSpentInMempoolTx {
			continue
		}
		out = append(out, UTXO{
			TxID:  strings.TrimSpace(item.TxID),
			Vout:  item.Vout,
			Value: item.Value,
		})
	}
	return out, nil
}

func (c *Client) GetTipHeightContext(ctx context.Context) (uint32, error) {
	body, err := c.get(ctx, "/chain/info")
	if err != nil {
		return 0, err
	}
	var info struct {
		Blocks uint32 `json:"blocks"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return 0, fmt.Errorf("decode chain info: %w", err)
	}
	return info.Blocks, nil
}

func (c *Client) BroadcastContext(ctx context.Context, txHex string) (string, error) {
	txHex = strings.TrimSpace(txHex)
	if txHex == "" {
		return "", fmt.Errorf("tx_hex is required")
	}
	body, err := c.postJSON(ctx, "/tx/raw", map[string]string{"txhex": txHex})
	if err != nil {
		return "", err
	}
	var txid string
	if err := json.Unmarshal(body, &txid); err == nil && strings.TrimSpace(txid) != "" {
		return strings.TrimSpace(txid), nil
	}
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err == nil {
		if v, ok := obj["txid"].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), nil
		}
		if v, ok := obj["data"].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), nil
		}
	}
	return "", fmt.Errorf("unexpected broadcast response: %s", string(body))
}

func (c *Client) GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error) {
	txid = strings.TrimSpace(txid)
	if txid == "" {
		return TxDetail{}, fmt.Errorf("txid is required")
	}
	body, err := c.get(ctx, "/tx/hash/"+txid)
	if err != nil {
		return TxDetail{}, err
	}
	var raw struct {
		TxID string `json:"txid"`
		Vin  []struct {
			TxID string `json:"txid"`
			Vout uint32 `json:"vout"`
		} `json:"vin"`
		Vout []struct {
			N            uint32  `json:"n"`
			Value        float64 `json:"value"`
			ScriptPubKey struct {
				Hex string `json:"hex"`
			} `json:"scriptPubKey"`
		} `json:"vout"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return TxDetail{}, fmt.Errorf("decode tx detail: %w", err)
	}
	out := TxDetail{
		TxID: strings.TrimSpace(raw.TxID),
		Vin:  make([]TxInput, 0, len(raw.Vin)),
		Vout: make([]TxOutput, 0, len(raw.Vout)),
	}
	for _, item := range raw.Vin {
		out.Vin = append(out.Vin, TxInput{
			TxID: strings.TrimSpace(item.TxID),
			Vout: item.Vout,
		})
	}
	for _, item := range raw.Vout {
		out.Vout = append(out.Vout, TxOutput{
			N:     item.N,
			Value: item.Value,
			ScriptPubKey: ScriptPubKey{
				Hex: strings.TrimSpace(item.ScriptPubKey.Hex),
			},
		})
	}
	return out, nil
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctxOrBackground(ctx), http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if err := c.auth.Apply(req); err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	return body, nil
}

func (c *Client) postJSON(ctx context.Context, path string, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctxOrBackground(ctx), http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.auth.Apply(req); err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	return body, nil
}

func ctxOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (c AuthConfig) Apply(req *http.Request) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}
	switch strings.ToLower(strings.TrimSpace(c.Mode)) {
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
		return fmt.Errorf("unsupported auth mode: %s", strings.TrimSpace(c.Mode))
	}
}
