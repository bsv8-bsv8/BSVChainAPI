package chainapi

import (
	"context"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"
)

const liveTestEnv = "BSV_CHAINAPI_LIVE"

// 这些样本都来自已经上链的老交易，适合做跨 provider 的真实读取对比。
// 默认 live 测试使用中等大小样本，避免大交易触发过多补全请求导致回归过慢。
// 设计约束：只做 get 系列的公网测试，不做真实广播。
var liveTxSamples = []string{
	"5faff1d2b58877aef9a4942222a579304b73a28cdaa9e0491f3fe9e83d85a427",
	"c37f30f5b9c616641e51d6bf70bbffc08b15db857f203788e53959f0292ee426",
}

const liveUTXOAddress = "18C5qX7sM98gKtgGqTAXTa53dnbyrSkx6e"
const liveHeavyTxSample = "edd0a4e6cb73255230016604568920f10ca2cb3b67f0b9f966bb00595faf53ea"

func TestLiveGetTipHeightParity(t *testing.T) {
	manager := mustNewLiveManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	wocTip, err := manager.GetTipHeightContext(ctx, Route{Provider: WhatsOnChainProvider, Network: "main"})
	if err != nil {
		t.Fatalf("whatsonchain get tip failed: %v", err)
	}
	bitailsTip, err := manager.GetTipHeightContext(ctx, Route{Provider: BitailsProvider, Network: "main"})
	if err != nil {
		t.Fatalf("bitails get tip failed: %v", err)
	}
	if wocTip == 0 || bitailsTip == 0 {
		t.Fatalf("unexpected zero tip: woc=%d bitails=%d", wocTip, bitailsTip)
	}
	if wocTip != bitailsTip {
		t.Fatalf("tip mismatch: woc=%d bitails=%d", wocTip, bitailsTip)
	}
}

func TestLiveGetTxDetailParity(t *testing.T) {
	manager := mustNewLiveManager(t)
	for _, txid := range liveTxSamples {
		txid := txid
		t.Run(txid, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()

			wocTx, err := manager.GetTxDetailContext(ctx, Route{Provider: WhatsOnChainProvider, Network: "main"}, txid)
			if err != nil {
				t.Fatalf("whatsonchain get tx failed: %v", err)
			}
			bitailsTx, err := manager.GetTxDetailContext(ctx, Route{Provider: BitailsProvider, Network: "main"}, txid)
			if err != nil {
				t.Fatalf("bitails get tx failed: %v", err)
			}
			if wocTx.TxID != bitailsTx.TxID || wocTx.TxID != txid {
				t.Fatalf("txid mismatch: woc=%s bitails=%s want=%s", wocTx.TxID, bitailsTx.TxID, txid)
			}
			if len(wocTx.Vin) != len(bitailsTx.Vin) {
				t.Fatalf("vin count mismatch: woc=%d bitails=%d", len(wocTx.Vin), len(bitailsTx.Vin))
			}
			if len(wocTx.Vout) != len(bitailsTx.Vout) {
				t.Fatalf("vout count mismatch: woc=%d bitails=%d", len(wocTx.Vout), len(bitailsTx.Vout))
			}
		})
	}
}

func TestLiveGetTxDetailHeavyPartialBitails(t *testing.T) {
	if os.Getenv(liveTestEnv+"_HEAVY") != "1" {
		t.Skipf("set %s_HEAVY=1 to run heavy partial-output tx parity test", liveTestEnv)
	}
	manager := mustNewLiveManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	wocTx, err := manager.GetTxDetailContext(ctx, Route{Provider: WhatsOnChainProvider, Network: "main"}, liveHeavyTxSample)
	if err != nil {
		t.Fatalf("whatsonchain get heavy tx failed: %v", err)
	}
	bitailsTx, err := manager.GetTxDetailContext(ctx, Route{Provider: BitailsProvider, Network: "main"}, liveHeavyTxSample)
	if err != nil {
		t.Fatalf("bitails get heavy tx failed: %v", err)
	}
	if len(wocTx.Vin) != len(bitailsTx.Vin) {
		t.Fatalf("heavy tx vin count mismatch: woc=%d bitails=%d", len(wocTx.Vin), len(bitailsTx.Vin))
	}
	if len(wocTx.Vout) != len(bitailsTx.Vout) {
		t.Fatalf("heavy tx vout count mismatch: woc=%d bitails=%d", len(wocTx.Vout), len(bitailsTx.Vout))
	}
}

func TestLiveGetUTXOsParity(t *testing.T) {
	manager := mustNewLiveManager(t)
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		wocUTXOs, err := manager.GetUTXOsContext(ctx, Route{Provider: WhatsOnChainProvider, Network: "main"}, liveUTXOAddress)
		cancel()
		if err != nil {
			t.Fatalf("whatsonchain get utxos failed: %v", err)
		}

		ctx, cancel = context.WithTimeout(context.Background(), 25*time.Second)
		bitailsUTXOs, err := manager.GetUTXOsContext(ctx, Route{Provider: BitailsProvider, Network: "main"}, liveUTXOAddress)
		cancel()
		if err != nil {
			t.Fatalf("bitails get utxos failed: %v", err)
		}

		wocKeys := utxoKeys(wocUTXOs)
		bitailsKeys := utxoKeys(bitailsUTXOs)
		if slices.Equal(wocKeys, bitailsKeys) {
			return
		}
		lastErr = &liveMismatchError{
			wocCount:     len(wocKeys),
			bitailsCount: len(bitailsKeys),
			wocKeys:      wocKeys,
			bitailsKeys:  bitailsKeys,
		}
		time.Sleep(1500 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatal(lastErr)
	}
}

func mustNewLiveManager(t *testing.T) *Manager {
	t.Helper()
	if os.Getenv(liveTestEnv) != "1" {
		t.Skipf("set %s=1 to run live provider parity tests", liveTestEnv)
	}
	manager, err := NewManager(Config{
		Routes: []RouteConfig{
			{
				Provider: WhatsOnChainProvider,
				Network:  "main",
				Profile:  DefaultProfile,
				Protect: ProtectConfig{
					MinInterval: 200 * time.Millisecond,
				},
			},
			{
				Provider: BitailsProvider,
				Network:  "main",
				Profile:  DefaultProfile,
				Protect: ProtectConfig{
					MinInterval: 200 * time.Millisecond,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("new live manager failed: %v", err)
	}
	return manager
}

func utxoKeys(items []UTXO) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.TxID+":"+itoa(item.Vout)+":"+itoa64(item.Value))
	}
	slices.Sort(out)
	return out
}

type liveMismatchError struct {
	wocCount     int
	bitailsCount int
	wocKeys      []string
	bitailsKeys  []string
}

func (e *liveMismatchError) Error() string {
	return "live utxo mismatch after retries: woc_count=" + strconv.Itoa(e.wocCount) + " bitails_count=" + strconv.Itoa(e.bitailsCount)
}

func itoa(v uint32) string {
	return itoa64(uint64(v))
}

func itoa64(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + (v % 10))
		v /= 10
	}
	return string(buf[i:])
}
