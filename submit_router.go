package chainapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type TxSubmitAttemptOutcome string

const (
	TxSubmitAttemptSuccess      TxSubmitAttemptOutcome = "success"
	TxSubmitAttemptAlreadyKnown TxSubmitAttemptOutcome = "already_known"
	TxSubmitAttemptFailed       TxSubmitAttemptOutcome = "failed"
)

type BroadcastErrorClass string

const (
	BroadcastErrorPermanent BroadcastErrorClass = "permanent"
	BroadcastErrorTemporary BroadcastErrorClass = "temporary"
	BroadcastErrorUnknown   BroadcastErrorClass = "unknown"
)

type TxSubmitAttempt struct {
	Route      Route                  `json:"route"`
	Outcome    TxSubmitAttemptOutcome `json:"outcome"`
	TxID       string                 `json:"txid,omitempty"`
	Error      string                 `json:"error,omitempty"`
	ErrorClass BroadcastErrorClass    `json:"error_class,omitempty"`
}

type TxSubmitResult struct {
	ExpectedTxID string            `json:"expected_txid"`
	TxID         string            `json:"txid,omitempty"`
	WinnerRoute  Route             `json:"winner_route,omitempty"`
	Attempts     []TxSubmitAttempt `json:"attempts"`
}

type TxSubmitRouter struct {
	api    API
	policy TxSubmitPolicy
}

func NewTxSubmitRouter(ctx context.Context, api API, policy TxSubmitPolicy) (*TxSubmitRouter, error) {
	if api == nil {
		return nil, fmt.Errorf("api is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	routes, err := normalizeTxSubmitPolicyRoutes(policy.Routes)
	if err != nil {
		return nil, err
	}
	for _, route := range routes {
		info, err := api.GetRouteInfoContext(ctx, route)
		if err != nil {
			return nil, fmt.Errorf("get route info failed for %s: %w", route.Key(), err)
		}
		if !hasCapability(info.Capabilities, CapabilityBroadcast) {
			return nil, fmt.Errorf("route does not support broadcast: %s", route.Key())
		}
	}
	return &TxSubmitRouter{
		api:    api,
		policy: TxSubmitPolicy{Routes: routes},
	}, nil
}

func (r *TxSubmitRouter) SubmitTxContext(ctx context.Context, txHex string) (TxSubmitResult, error) {
	if r == nil || r.api == nil {
		return TxSubmitResult{}, fmt.Errorf("tx submit router is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	txHex = strings.TrimSpace(txHex)
	if txHex == "" {
		return TxSubmitResult{}, fmt.Errorf("tx_hex is required")
	}
	expectedTxID, err := computeTxID(txHex)
	if err != nil {
		return TxSubmitResult{}, err
	}
	result := TxSubmitResult{
		ExpectedTxID: expectedTxID,
		Attempts:     make([]TxSubmitAttempt, 0, len(r.policy.Routes)),
	}
	var lastErr error
	for _, route := range r.policy.Routes {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		attempt := TxSubmitAttempt{Route: route}
		txid, err := r.api.BroadcastContext(ctx, route, txHex)
		if err == nil {
			normalized := normalizeTxID(txid)
			if normalized == "" {
				attempt.Outcome = TxSubmitAttemptFailed
				attempt.Error = "broadcast response missing txid"
				attempt.ErrorClass = BroadcastErrorUnknown
				result.Attempts = append(result.Attempts, attempt)
				lastErr = fmt.Errorf("broadcast response missing txid")
				continue
			}
			if normalized != expectedTxID {
				attempt.Outcome = TxSubmitAttemptFailed
				attempt.TxID = normalized
				attempt.Error = fmt.Sprintf("broadcast response txid mismatch: got=%s want=%s", normalized, expectedTxID)
				attempt.ErrorClass = BroadcastErrorPermanent
				result.Attempts = append(result.Attempts, attempt)
				return result, fmt.Errorf("broadcast response txid mismatch on %s: got=%s want=%s", route.Key(), normalized, expectedTxID)
			}
			attempt.Outcome = TxSubmitAttemptSuccess
			attempt.TxID = expectedTxID
			result.Attempts = append(result.Attempts, attempt)
			result.TxID = expectedTxID
			result.WinnerRoute = route
			return result, nil
		}
		if isAlreadyKnownBroadcastError(err) {
			attempt.Outcome = TxSubmitAttemptAlreadyKnown
			attempt.TxID = expectedTxID
			result.Attempts = append(result.Attempts, attempt)
			result.TxID = expectedTxID
			result.WinnerRoute = route
			return result, nil
		}
		attempt.Outcome = TxSubmitAttemptFailed
		attempt.Error = err.Error()
		attempt.ErrorClass = classifyBroadcastError(err)
		result.Attempts = append(result.Attempts, attempt)
		lastErr = err
		if attempt.ErrorClass == BroadcastErrorPermanent {
			return result, fmt.Errorf("broadcast failed on %s: %w", route.Key(), err)
		}
	}
	if lastErr == nil {
		return result, fmt.Errorf("tx submit failed: no routes attempted")
	}
	return result, fmt.Errorf("tx submit failed after %d attempt(s): %w", len(result.Attempts), lastErr)
}

func normalizeTxSubmitPolicyRoutes(in []Route) ([]Route, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("tx submit policy routes are required")
	}
	out := make([]Route, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, route := range in {
		n := route.Normalize()
		if n.Provider == "" || n.Network == "" {
			return nil, fmt.Errorf("route provider and network are required")
		}
		key := n.Key()
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate tx submit route: %s", key)
		}
		seen[key] = struct{}{}
		out = append(out, n)
	}
	return out, nil
}

func computeTxID(txHex string) (string, error) {
	raw, err := hex.DecodeString(strings.TrimSpace(txHex))
	if err != nil {
		return "", fmt.Errorf("decode tx_hex: %w", err)
	}
	first := sha256.Sum256(raw)
	second := sha256.Sum256(first[:])
	for i := 0; i < len(second)/2; i++ {
		second[i], second[len(second)-1-i] = second[len(second)-1-i], second[i]
	}
	return hex.EncodeToString(second[:]), nil
}

func normalizeTxID(in string) string {
	v := strings.ToLower(strings.TrimSpace(in))
	if len(v) != 64 {
		return ""
	}
	if _, err := hex.DecodeString(v); err != nil {
		return ""
	}
	return v
}

func isAlreadyKnownBroadcastError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, key := range []string{
		"already known",
		"already in mempool",
		"transaction already known",
		"duplicate transaction",
		"txn-already-known",
		"tx already known",
		"already broadcast",
		"already submitted",
	} {
		if strings.Contains(msg, key) {
			return true
		}
	}
	return false
}

func classifyBroadcastError(err error) BroadcastErrorClass {
	if err == nil {
		return BroadcastErrorUnknown
	}
	if errors.Is(err, context.Canceled) {
		return BroadcastErrorTemporary
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return BroadcastErrorTemporary
	}
	var se statusError
	if errors.As(err, &se) {
		status := se.HTTPStatus()
		switch {
		case status == http.StatusTooManyRequests,
			status == http.StatusRequestTimeout,
			status >= 500:
			return BroadcastErrorTemporary
		case status >= 400 && status < 500:
			return BroadcastErrorPermanent
		}
	}
	if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
		return BroadcastErrorTemporary
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, key := range []string{
		"timeout",
		"temporarily unavailable",
		"connection refused",
		"connection reset",
		"unexpected eof",
		"eof",
		"429",
		"502",
		"503",
		"504",
	} {
		if strings.Contains(msg, key) {
			return BroadcastErrorTemporary
		}
	}
	for _, key := range []string{
		"invalid",
		"rejected",
		"fee",
		"script",
		"authentication",
		"authorization",
		"unsupported",
		"bad request",
	} {
		if strings.Contains(msg, key) {
			return BroadcastErrorPermanent
		}
	}
	if strings.Contains(msg, "unexpected broadcast response") ||
		strings.Contains(msg, "broadcast response missing txid") ||
		strings.Contains(msg, "decode broadcast response") {
		return BroadcastErrorUnknown
	}
	return BroadcastErrorUnknown
}
