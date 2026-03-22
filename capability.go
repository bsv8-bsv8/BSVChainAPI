package chainapi

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type Capability string

const (
	CapabilityGetUTXOs     Capability = "get_utxos"
	CapabilityGetTipHeight Capability = "get_tip_height"
	CapabilityBroadcast    Capability = "broadcast"
	CapabilityGetTxDetail  Capability = "get_tx_detail"
)

var ErrCapabilityUnsupported = errors.New("capability unsupported")

type CapabilityError struct {
	Route      Route
	Capability Capability
}

func (e *CapabilityError) Error() string {
	if e == nil {
		return ErrCapabilityUnsupported.Error()
	}
	return fmt.Sprintf("capability unsupported: %s on route %s", e.Capability, e.Route.Key())
}

func (e *CapabilityError) Unwrap() error {
	return ErrCapabilityUnsupported
}

type statusError interface {
	error
	HTTPStatus() int
	HTTPBody() string
}

func unsupportedCapabilityError(route Route, cap Capability) error {
	return &CapabilityError{
		Route:      route.Normalize(),
		Capability: cap,
	}
}

func normalizeCapabilities(in []Capability) []Capability {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[Capability]struct{}, len(in))
	out := make([]Capability, 0, len(in))
	for _, cap := range in {
		cap = Capability(strings.TrimSpace(string(cap)))
		if cap == "" {
			continue
		}
		if _, exists := seen[cap]; exists {
			continue
		}
		seen[cap] = struct{}{}
		out = append(out, cap)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i] < out[j]
	})
	return out
}

func hasCapability(caps []Capability, want Capability) bool {
	for _, cap := range caps {
		if cap == want {
			return true
		}
	}
	return false
}
