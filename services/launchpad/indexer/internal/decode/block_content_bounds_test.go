package decode

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func TestBlockContentBoundsRawBatch(t *testing.T) {
	d, a := deployment(t, "EventEmitter", common.HexToAddress("0x1"))
	base, _ := syntheticLog(t, a.Events["Swap"], d.Address)
	logs := make([]ethtypes.Log, MaxRawBatchLogs+1)
	for i := range logs {
		logs[i] = base
		logs[i].Index = uint(i)
	}
	unique, err := DeduplicateRaw(logs)
	if err != nil || len(unique) != len(logs) {
		t.Fatalf("raw batch count: %d: %v", len(unique), err)
	}
	shared := make([]byte, 1<<20)
	logs = make([]ethtypes.Log, MaxRawBatchBytes/(256+len(shared))+1)
	for i := range logs {
		logs[i] = base
		logs[i].Data = shared
	}
	if unique, err := DeduplicateRaw(logs); unique != nil || err == nil {
		t.Fatalf("raw batch bytes: %d: %v", len(unique), err)
	}
}

func TestBlockContentBoundsDecode(t *testing.T) {
	d, a := deployment(t, "EventEmitter", common.HexToAddress("0x1"))
	d.From = Order{Block: 100}
	r := registry(t, d)
	for _, topics := range [][]common.Hash{{a.Events["Swap"].ID}, {common.HexToHash("0x999")}, nil, make([]common.Hash, 5)} {
		log, _ := syntheticLog(t, a.Events["Swap"], common.HexToAddress("0x2"))
		log.Data, log.Topics = make([]byte, MaxEnvelopeBytes+1), topics
		if rawSizeAllowed(log) {
			t.Fatal("oversized envelope required")
		}
		env, err := r.Decode(log)
		if err != nil || env.Status != "opaque" || !env.Opaque || env.FailureReason != "untrusted_emitter" || env.Event != nil || env.Family != nil || !reflect.DeepEqual(env.Raw, rawMetadata(log)) || !reflect.DeepEqual(env.RawSummary, summarizeRaw(log)) {
			t.Fatalf("unbound bounds envelope: %+v: %v", env, err)
		}
		encoded, err := json.Marshal(env)
		if err != nil || len(encoded) > 2048 {
			t.Fatalf("opaque summary size: %d: %v", len(encoded), err)
		}
		for _, height := range []uint64{0, 100} {
			log.Address, log.BlockNumber = d.Address, height
			env, err = r.Decode(log)
			if err != nil || env.Status != "quarantined" || env.FailureReason != "bounds" || env.Event != nil || !reflect.DeepEqual(env.RawSummary, summarizeRaw(log)) {
				t.Fatalf("deployment bounds envelope: %+v: %v", env, err)
			}
		}
	}
}
