package chainapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	ServiceName    = "bsv-chainapi"
	ServiceVersion = "0.1.0"
)

type PortServer struct {
	api API
}

func NewPortServer(api API) *PortServer {
	return &PortServer{api: api}
}

func (s *PortServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/get-utxos", s.handleGetUTXOs)
	mux.HandleFunc("/v1/get-tip-height", s.handleGetTipHeight)
	mux.HandleFunc("/v1/broadcast", s.handleBroadcast)
	mux.HandleFunc("/v1/get-tx-detail", s.handleGetTxDetail)
	return mux
}

type PortClient struct {
	baseURL string
	http    *http.Client
}

func NewPortClient(baseURL string) *PortClient {
	u := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &PortClient{
		baseURL: u,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *PortClient) GetUTXOsContext(ctx context.Context, route Route, address string) ([]UTXO, error) {
	var resp struct {
		UTXOs []UTXO `json:"utxos"`
	}
	if err := c.postJSON(ctx, "/v1/get-utxos", map[string]any{
		"route":   route.Normalize(),
		"address": strings.TrimSpace(address),
	}, &resp); err != nil {
		return nil, err
	}
	return resp.UTXOs, nil
}

func (c *PortClient) GetTipHeightContext(ctx context.Context, route Route) (uint32, error) {
	var resp struct {
		TipHeight uint32 `json:"tip_height"`
	}
	if err := c.postJSON(ctx, "/v1/get-tip-height", map[string]any{
		"route": route.Normalize(),
	}, &resp); err != nil {
		return 0, err
	}
	return resp.TipHeight, nil
}

func (c *PortClient) BroadcastContext(ctx context.Context, route Route, txHex string) (string, error) {
	var resp struct {
		TxID string `json:"txid"`
	}
	if err := c.postJSON(ctx, "/v1/broadcast", map[string]any{
		"route":  route.Normalize(),
		"tx_hex": strings.TrimSpace(txHex),
	}, &resp); err != nil {
		return "", err
	}
	if strings.TrimSpace(resp.TxID) == "" {
		return "", fmt.Errorf("response missing txid")
	}
	return strings.TrimSpace(resp.TxID), nil
}

func (c *PortClient) GetTxDetailContext(ctx context.Context, route Route, txid string) (TxDetail, error) {
	var resp struct {
		Tx TxDetail `json:"tx"`
	}
	if err := c.postJSON(ctx, "/v1/get-tx-detail", map[string]any{
		"route": route.Normalize(),
		"txid":  strings.TrimSpace(txid),
	}, &resp); err != nil {
		return TxDetail{}, err
	}
	if strings.TrimSpace(resp.Tx.TxID) == "" {
		return TxDetail{}, fmt.Errorf("response missing txid")
	}
	return resp.Tx, nil
}

func (c *PortClient) postJSON(ctx context.Context, path string, payload any, out any) error {
	if c == nil {
		return fmt.Errorf("port client is nil")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctxOrBackground(ctx), http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (s *PortServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": ServiceName,
		"version": ServiceVersion,
	})
}

func (s *PortServer) handleGetUTXOs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Route   Route  `json:"route"`
		Address string `json:"address"`
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	utxos, err := s.api.GetUTXOsContext(r.Context(), req.Route, req.Address)
	if err != nil {
		writeJSON(w, classifyStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"utxos": utxos})
}

func (s *PortServer) handleGetTipHeight(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Route Route `json:"route"`
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	tip, err := s.api.GetTipHeightContext(r.Context(), req.Route)
	if err != nil {
		writeJSON(w, classifyStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tip_height": tip})
}

func (s *PortServer) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Route Route  `json:"route"`
		TxHex string `json:"tx_hex"`
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	txid, err := s.api.BroadcastContext(r.Context(), req.Route, req.TxHex)
	if err != nil {
		writeJSON(w, classifyStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"txid": txid})
}

func (s *PortServer) handleGetTxDetail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Route Route  `json:"route"`
		TxID  string `json:"txid"`
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	txj, err := s.api.GetTxDetailContext(r.Context(), req.Route, req.TxID)
	if err != nil {
		writeJSON(w, classifyStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tx": txj})
}

func decodeRequest(w http.ResponseWriter, r *http.Request, out any) bool {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return false
	}
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid json: %v", err)})
		return false
	}
	return true
}

func classifyStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "route not found"),
		strings.Contains(msg, "provider not registered"),
		strings.Contains(msg, "required"),
		strings.Contains(msg, "unsupported"),
		strings.Contains(msg, "invalid"):
		return http.StatusBadRequest
	default:
		return http.StatusBadGateway
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func ctxOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
