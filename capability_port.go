package chainapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	capabilityGetUTXOsPath     = "/v1/cap/get-utxos"
	capabilityGetTipHeightPath = "/v1/cap/get-tip-height"
	capabilityGetTxDetailPath  = "/v1/cap/get-tx-detail"
	capabilitySubmitTxPath     = "/v1/cap/submit-tx"
)

// CapabilityPortServer 把 CapabilityAPI 暴露为共享 HTTP 服务。
// 业务子项目应优先使用这一层，而不是直接调用 raw Port 的 route API。
type CapabilityPortServer struct {
	api CapabilityAPI
}

func NewCapabilityPortServer(api CapabilityAPI) *CapabilityPortServer {
	return &CapabilityPortServer{api: api}
}

func (s *CapabilityPortServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleCapabilityHealth)
	mux.HandleFunc(capabilityGetUTXOsPath, s.handleGetUTXOs)
	mux.HandleFunc(capabilityGetTipHeightPath, s.handleGetTipHeight)
	mux.HandleFunc(capabilityGetTxDetailPath, s.handleGetTxDetail)
	mux.HandleFunc(capabilitySubmitTxPath, s.handleSubmitTx)
	return mux
}

type CapabilityPortClient struct {
	baseURL string
	http    *http.Client
}

func NewCapabilityPortClient(baseURL string) *CapabilityPortClient {
	u := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &CapabilityPortClient{
		baseURL: u,
		http: &http.Client{
			Timeout: defaultPortTimeout,
		},
	}
}

func (c *CapabilityPortClient) GetUTXOsContext(ctx context.Context, address string) ([]UTXO, error) {
	var resp struct {
		UTXOs []UTXO `json:"utxos"`
	}
	if err := c.postJSON(ctx, capabilityGetUTXOsPath, map[string]any{
		"address": strings.TrimSpace(address),
	}, &resp); err != nil {
		return nil, err
	}
	return resp.UTXOs, nil
}

func (c *CapabilityPortClient) GetTipHeightContext(ctx context.Context) (uint32, error) {
	var resp struct {
		TipHeight uint32 `json:"tip_height"`
	}
	if err := c.postJSON(ctx, capabilityGetTipHeightPath, map[string]any{}, &resp); err != nil {
		return 0, err
	}
	return resp.TipHeight, nil
}

func (c *CapabilityPortClient) GetTxDetailContext(ctx context.Context, txid string) (TxDetail, error) {
	var resp struct {
		Tx TxDetail `json:"tx"`
	}
	if err := c.postJSON(ctx, capabilityGetTxDetailPath, map[string]any{
		"txid": strings.TrimSpace(txid),
	}, &resp); err != nil {
		return TxDetail{}, err
	}
	if strings.TrimSpace(resp.Tx.TxID) == "" {
		return TxDetail{}, fmt.Errorf("response missing txid")
	}
	return resp.Tx, nil
}

func (c *CapabilityPortClient) SubmitTxContext(ctx context.Context, txHex string) (TxSubmitResult, error) {
	var resp struct {
		Result TxSubmitResult `json:"result"`
	}
	if err := c.postJSON(ctx, capabilitySubmitTxPath, map[string]any{
		"tx_hex": strings.TrimSpace(txHex),
	}, &resp); err != nil {
		return TxSubmitResult{}, err
	}
	if strings.TrimSpace(resp.Result.ExpectedTxID) == "" {
		return TxSubmitResult{}, fmt.Errorf("response missing expected_txid")
	}
	return resp.Result, nil
}

func (c *CapabilityPortClient) postJSON(ctx context.Context, path string, payload any, out any) error {
	if c == nil {
		return fmt.Errorf("capability port client is nil")
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
	body, err := readBody(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &portHTTPError{
			statusCode: resp.StatusCode,
			body:       strings.TrimSpace(string(body)),
		}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (s *CapabilityPortServer) handleGetUTXOs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Address string `json:"address"`
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	utxos, err := s.api.GetUTXOsContext(r.Context(), req.Address)
	if err != nil {
		writeJSON(w, classifyStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"utxos": utxos})
}

func (s *CapabilityPortServer) handleGetTipHeight(w http.ResponseWriter, r *http.Request) {
	var req struct{}
	if !decodeRequest(w, r, &req) {
		return
	}
	tip, err := s.api.GetTipHeightContext(r.Context())
	if err != nil {
		writeJSON(w, classifyStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tip_height": tip})
}

func (s *CapabilityPortServer) handleGetTxDetail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TxID string `json:"txid"`
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	txj, err := s.api.GetTxDetailContext(r.Context(), req.TxID)
	if err != nil {
		writeJSON(w, classifyStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tx": txj})
}

func (s *CapabilityPortServer) handleSubmitTx(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TxHex string `json:"tx_hex"`
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	result, err := s.api.SubmitTxContext(r.Context(), req.TxHex)
	if err != nil {
		writeJSON(w, classifyStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

func handleCapabilityHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": ServiceName + "-capability",
		"version": ServiceVersion,
	})
}
