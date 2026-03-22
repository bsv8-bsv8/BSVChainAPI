package bitails

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	MainnetBaseURL = "https://api.bitails.io"
	TestnetBaseURL = "https://test-api.bitails.io"
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
	TxID          string
	Vout          uint32
	Value         uint64
	BlockHeight   int64
	Confirmations int64
}

type TxDetail struct {
	TxID string
	Vin  []TxInput
	Vout []TxOutput
}

type TxInput struct {
	TxID        string
	Vout        uint32
	ScriptHex   string
	SourceValue uint64
}

type TxOutput struct {
	N            uint32
	Value        float64
	ValueSatoshi uint64
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
	const pageSize = 1000
	out := make([]UTXO, 0)
	for from := 0; ; from += pageSize {
		body, err := c.get(ctx, fmt.Sprintf("/address/%s/unspent?from=%d&limit=%d", address, from, pageSize))
		if err != nil {
			return nil, err
		}
		var resp struct {
			Address string `json:"address"`
			Unspent []struct {
				TxID          string `json:"txid"`
				Vout          uint32 `json:"vout"`
				Satoshis      uint64 `json:"satoshis"`
				BlockHeight   int64  `json:"blockheight"`
				Confirmations int64  `json:"confirmations"`
			} `json:"unspent"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("decode utxos: %w", err)
		}
		for _, item := range resp.Unspent {
			if item.BlockHeight <= 0 || item.Confirmations <= 0 {
				continue
			}
			out = append(out, UTXO{
				TxID:          strings.TrimSpace(item.TxID),
				Vout:          item.Vout,
				Value:         item.Satoshis,
				BlockHeight:   item.BlockHeight,
				Confirmations: item.Confirmations,
			})
		}
		if len(resp.Unspent) < pageSize {
			return out, nil
		}
	}
}

func (c *Client) GetTipHeightContext(ctx context.Context) (uint32, error) {
	body, err := c.get(ctx, "/network/info")
	if err != nil {
		return 0, err
	}
	var resp struct {
		Blocks uint32 `json:"blocks"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("decode network info: %w", err)
	}
	return resp.Blocks, nil
}

func (c *Client) BroadcastContext(ctx context.Context, txHex string) (string, error) {
	txHex = strings.TrimSpace(txHex)
	if txHex == "" {
		return "", fmt.Errorf("tx_hex is required")
	}
	body, err := c.postJSON(ctx, "/tx/broadcast", map[string]string{"raw": txHex})
	if err != nil {
		return "", err
	}
	var resp struct {
		TxID string `json:"txid"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("decode broadcast response: %w", err)
	}
	if strings.TrimSpace(resp.TxID) == "" {
		return "", fmt.Errorf("broadcast response missing txid")
	}
	return strings.TrimSpace(resp.TxID), nil
}

func (c *Client) GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error) {
	txid = strings.TrimSpace(txid)
	if txid == "" {
		return TxDetail{}, fmt.Errorf("txid is required")
	}
	body, err := c.get(ctx, "/tx/"+txid)
	if err != nil {
		return TxDetail{}, err
	}
	var resp struct {
		TxID           string `json:"txid"`
		PartialOutputs bool   `json:"partialOutputs"`
		Inputs         []struct {
			Index  uint32 `json:"index"`
			Source struct {
				TxID     string      `json:"txid"`
				Index    uint32      `json:"index"`
				Script   string      `json:"script"`
				Satoshis json.Number `json:"satoshis"`
			} `json:"source"`
		} `json:"inputs"`
		Outputs []struct {
			Index         uint32      `json:"index"`
			Satoshis      json.Number `json:"satoshis"`
			Script        string      `json:"script"`
			PartialScript bool        `json:"partialScript"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return TxDetail{}, fmt.Errorf("decode tx detail: %w", err)
	}
	out := TxDetail{
		TxID: strings.TrimSpace(resp.TxID),
		Vin:  make([]TxInput, 0, len(resp.Inputs)),
		Vout: make([]TxOutput, 0, len(resp.Outputs)),
	}
	for _, item := range resp.Inputs {
		value, err := parseUint(item.Source.Satoshis)
		if err != nil {
			return TxDetail{}, err
		}
		out.Vin = append(out.Vin, TxInput{
			TxID:        strings.TrimSpace(item.Source.TxID),
			Vout:        item.Source.Index,
			ScriptHex:   strings.TrimSpace(item.Source.Script),
			SourceValue: value,
		})
	}
	for _, item := range resp.Outputs {
		value, err := parseUint(item.Satoshis)
		if err != nil {
			return TxDetail{}, err
		}
		scriptHex := strings.TrimSpace(item.Script)
		if resp.PartialOutputs || item.PartialScript {
			fullHex, err := c.getOutputHex(ctx, txid, item.Index)
			if err != nil {
				return TxDetail{}, err
			}
			scriptHex = fullHex
		}
		out.Vout = append(out.Vout, TxOutput{
			N:            item.Index,
			ValueSatoshi: value,
			Value:        float64(value) / 1e8,
			ScriptPubKey: ScriptPubKey{
				Hex: scriptHex,
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

func parseUint(v json.Number) (uint64, error) {
	s := strings.TrimSpace(v.String())
	if s == "" {
		return 0, nil
	}
	out, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse uint: %w", err)
	}
	return out, nil
}

func (c *Client) getOutputHex(ctx context.Context, txid string, outputIndex uint32) (string, error) {
	body, err := c.get(ctx, fmt.Sprintf("/download/tx/%s/output/%d/hex", strings.TrimSpace(txid), outputIndex))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}
