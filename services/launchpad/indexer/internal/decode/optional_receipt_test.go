package decode

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func optionalLayoutLog(t *testing.T, event abi.Event, address common.Address, overrides map[string]any) types.Log {
	t.Helper()
	log, _ := syntheticLog(t, event, address)
	log.Topics = []common.Hash{event.ID}
	values := []any{}
	for _, argument := range event.Inputs {
		value := sample(argument.Type)
		if changed, ok := overrides[argument.Name]; ok {
			value = changed
		}
		if argument.Indexed {
			encoded, err := (abi.Arguments{{Type: argument.Type}}).Pack(value)
			if err != nil {
				t.Fatal(err)
			}
			log.Topics = append(log.Topics, common.BytesToHash(encoded))
		} else {
			values = append(values, value)
		}
	}
	var err error
	log.Data, err = event.Inputs.NonIndexed().Pack(values...)
	if err != nil {
		t.Fatal(err)
	}
	return log
}

func TestOptionalReceiptCorrelationRequiresUniqueBoundCompanions(t *testing.T) {
	native, na := deployment(t, "PECOR", common.HexToAddress("0x1251"))
	native.Optional = true
	native.Enabled = true
	mirror, ma := deployment(t, "EventEmitter", common.HexToAddress("0x1252"))
	mirror.EnabledOptionalEvents = []string{"PecorSwap"}
	r := registry(t, native, mirror)
	n := optionalLayoutLog(t, na.Events["SimpleSwap"], native.Address, nil)
	c := optionalLayoutLog(t, ma.Events["PecorSwap"], mirror.Address, map[string]any{"swapKind": uint8(0)})
	c.Index = n.Index + 1
	binding := CorrelationBinding{NativeEmitter: native.Address, NativeRole: native.Role, CompanionEmitter: mirror.Address, CompanionRole: mirror.Role}
	for _, mode := range []string{"bound", "unbound", "duplicate mirror", "duplicate native", "quarantined native", "comparison bound"} {
		t.Run(mode, func(t *testing.T) {
			logs := []*types.Log{&n, &c}
			bindings := []CorrelationBinding{binding}
			switch mode {
			case "unbound":
				bindings = nil
			case "duplicate mirror":
				extra := c
				extra.Index++
				logs = append(logs, &extra)
			case "duplicate native":
				extra := n
				extra.Index = c.Index + 1
				logs = append(logs, &extra)
			case "quarantined native":
				bad := n
				bad.Data = []byte{1}
				logs[0] = &bad
			case "comparison bound":
				logs = nil
				for i := 0; i < 1025; i++ {
					a, b := n, c
					a.Index = uint(2 * i)
					b.Index = uint(2*i + 1)
					logs = append(logs, &a, &b)
				}
			}
			receipt := &types.Receipt{Status: 1, BlockNumber: new(big.Int).SetUint64(n.BlockNumber), BlockHash: n.BlockHash, TxHash: n.TxHash, TransactionIndex: n.TxIndex, Logs: logs}
			out, err := r.CorrelateOptionalReceipt(receipt, bindings)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "bound":
				if len(out.Effects) != 1 || !out.Effects[0].Economic || len(out.Effects[0].Companions) != 1 || out.Effects[0].Companions[0].LogIndex != c.Index || len(out.Unmatched) != 0 {
					t.Fatalf("unique optional companion %+v", out)
				}
			case "unbound":
				if len(out.Effects) != 1 || len(out.Effects[0].Companions) != 0 || len(out.Unmatched) != 1 || len(out.Diagnostics) != 1 {
					t.Fatalf("unbound companion %+v", out)
				}
			case "duplicate mirror":
				if len(out.Effects) != 1 || len(out.Unmatched) != 2 || len(out.Effects[0].Companions) != 0 {
					t.Fatalf("ambiguous mirrors %+v", out)
				}
			case "duplicate native":
				if len(out.Effects) != 2 || len(out.Unmatched) != 1 {
					t.Fatalf("ambiguous native actions %+v", out)
				}
			case "quarantined native":
				if len(out.Effects) != 0 || len(out.Unmatched) != 1 {
					t.Fatalf("quarantined action %+v", out)
				}
			case "comparison bound":
				if len(out.Effects) != 0 || len(out.Unmatched) != 2050 || len(out.Diagnostics) != 1 || out.Diagnostics[0] != "unavailable_correlation:comparison_bound" {
					t.Fatalf("correlation comparison bound effects=%d unmatched=%d diagnostics=%v", len(out.Effects), len(out.Unmatched), out.Diagnostics)
				}
			}
		})
	}
	receipt := &types.Receipt{Status: 1, BlockNumber: new(big.Int).SetUint64(n.BlockNumber), BlockHash: n.BlockHash, TxHash: n.TxHash, TransactionIndex: n.TxIndex, Logs: []*types.Log{&n, &c}}
	for _, bindings := range [][]CorrelationBinding{make([]CorrelationBinding, 65), {{}}, {binding, binding}} {
		if _, err := r.CorrelateOptionalReceipt(receipt, bindings); err == nil {
			t.Fatal("invalid optional binding admitted")
		}
	}
	bad := *receipt
	bad.Logs = []*types.Log{nil}
	if _, err := r.CorrelateOptionalReceipt(&bad, nil); err == nil {
		t.Fatal("nil receipt log admitted")
	}
	if _, err := (*Registry)(nil).CorrelateOptionalReceipt(receipt, nil); err == nil {
		t.Fatal("absent correlation registry admitted")
	}
}
