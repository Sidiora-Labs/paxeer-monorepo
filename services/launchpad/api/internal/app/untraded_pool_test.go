package app

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestUntradedPoolAuthorityPolicy(t *testing.T) {
	a, reader, head := newAuthorityReadinessState(t)
	commit := productionAuthorityHeads(t, a)
	commit(reader.header, "healthy")
	config := common.HexToAddress(a.config.Manifest.Roles["ProtocolConfig"].Address)
	for slot, value := range map[int64]int64{4: 10, 5: 300, 6: 30, 8: 199} {
		reader.state.SetState(config, common.BigToHash(big.NewInt(slot)), common.BigToHash(big.NewInt(value)))
	}
	if _, ok := a.poolFeePolicy().FeeBps(time.Now().Unix()); ok {
		t.Fatal("policy available before authority publication")
	}
	if err := a.refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	if a.lastEnvelope.CommittedHead != head {
		t.Fatal("fee policy differs from published head")
	}
	before := reader.reads.Load()
	policy := a.poolFeePolicy()
	for _, tc := range []struct {
		age  int64
		want int
	}{{0, 229}, {3599, 229}, {3600, 129}, {7201, 96}} {
		if got, ok := policy.FeeBps(a.lastEnvelope.Attestation.BlockTime.Unix() - tc.age); !ok || got != tc.want {
			t.Fatalf("age %d: fee %d available %t, want %d", tc.age, got, ok, tc.want)
		}
	}
	raw, err := json.Marshal(a.lastEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err = json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire) != 4 {
		t.Fatalf("authority fields: %v", wire)
	}
	if reader.reads.Load() != before {
		t.Fatal("pool fee calculation read the chain")
	}
	a.mu.Lock()
	a.builtAt = time.Now().Add(-authorityEnvelopeMaxAge - time.Second)
	a.mu.Unlock()
	if _, ok := a.poolFeePolicy().FeeBps(a.lastEnvelope.Attestation.BlockTime.Unix()); ok {
		t.Fatal("expired authority policy available")
	}
	var unavailable *networkAuthority
	if _, ok := unavailable.poolFeePolicy().FeeBps(time.Now().Unix()); ok {
		t.Fatal("absent authority policy available")
	}
}
