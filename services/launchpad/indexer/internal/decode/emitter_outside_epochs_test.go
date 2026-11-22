package decode

import (
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestEmitterOutsideEpochsStaysOpaque(t *testing.T) {
	for _, c := range []struct{ role, event string }{
		{"SidioraERC20", "Transfer"},
		{"EventEmitter", "Swap"},
	} {
		t.Run(c.role+"/"+c.event, func(t *testing.T) {
			d, a := deployment(t, c.role, common.HexToAddress("0x1"))
			l, _ := syntheticLog(t, a.Events[c.event], common.HexToAddress("0x2"))
			r := registry(t, d)
			if !r.known[l.Topics[0]] {
				t.Fatal("catalog signature required")
			}
			e, err := r.Decode(l)
			if err != nil || e.Status != "opaque" || !e.Opaque || e.FailureReason != "untrusted_emitter" || e.Event != nil || e.Family != nil || e.Actors != nil || e.Operational != nil || e.EmitterRole != "" || e.ABIRevision != "" || !reflect.DeepEqual(e.Raw, rawLog(l)) {
				t.Fatalf("unbound emitter: %+v, %v", e, err)
			}
		})
	}
}

func TestEmitterOutsideEpochsKeepsBoundEmitterRefusals(t *testing.T) {
	emitter, a := deployment(t, "EventEmitter", common.HexToAddress("0x1"))
	early := emitter
	early.From = Order{Block: 71}
	other, _ := deployment(t, "SidioraERC20", emitter.Address)
	for _, c := range []struct {
		name, reason string
		d            Deployment
	}{
		{"before activation", "untrusted_emitter", early},
		{"outside role", "unauthorized_signature", other},
	} {
		t.Run(c.name, func(t *testing.T) {
			l, _ := syntheticLog(t, a.Events["Swap"], emitter.Address)
			e, err := registry(t, c.d).Decode(l)
			if err == nil || e.Status != "quarantined" || e.FailureReason != c.reason || e.Event != nil || e.Family != nil || !reflect.DeepEqual(e.Raw, rawLog(l)) {
				t.Fatalf("bound emitter: %+v, %v", e, err)
			}
		})
	}
	for _, topics := range [][]common.Hash{nil, {common.HexToHash("0xdead")}} {
		l, _ := syntheticLog(t, a.Events["Swap"], emitter.Address)
		l.Topics = topics
		e, err := registry(t, early).Decode(l)
		if err != nil || e.Status != "opaque" || !e.Opaque || e.FailureReason != "untrusted_emitter" || e.Event != nil || !reflect.DeepEqual(e.Raw, rawLog(l)) {
			t.Fatalf("unknown signature before activation: %+v, %v", e, err)
		}
	}
}
