package protocol

import (
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestRoleEventsRegistered(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		signature string
		contracts []string
	}{
		{"AccountingMigrated", "AccountingMigrated(uint256)", []string{"FeeAccumulator"}},
		{"LegacyAirdropAllocated", "LegacyAirdropAllocated(address,uint256,bytes32,uint256)", []string{"FeeAccumulator"}},
		{"MirrorFailed", "MirrorFailed(bytes4)", []string{"PoolRegistry", "FeeAccumulator", "FeesRouter"}},
		{"OpticalSurplusClaimed", "OpticalSurplusClaimed(address,address,uint256)", []string{"FeeAccumulator"}},
		{"OpticalSurplusRecorded", "OpticalSurplusRecorded(address,uint256)", []string{"FeeAccumulator"}},
		{"MultihopSwap", "MultihopSwap(address,address,address,uint256,uint256,uint256)", []string{"Router"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := crypto.Keccak256Hash([]byte(tc.signature))
			def, ok := LookupEvent(want)
			if !ok || def == nil {
				t.Fatalf("LookupEvent(%s): %s is not registered", want.Hex(), tc.signature)
			}
			if def.Topic0 != want {
				t.Errorf("Topic0 = %s, want keccak256(%q) = %s", def.Topic0.Hex(), tc.signature, want.Hex())
			}
			if def.Name != tc.name || def.Sig != tc.signature {
				t.Errorf("event = %s (%s), want %s (%s)", def.Name, def.Sig, tc.name, tc.signature)
			}
			if def.Event.ID != want || def.Event.Sig != tc.signature {
				t.Errorf("ABI event = %s (%s), want %s (%s)", def.Event.ID.Hex(), def.Event.Sig, want.Hex(), tc.signature)
			}
			if !slices.Equal(def.Contracts, tc.contracts) {
				t.Errorf("declaring contracts in source order = %v, want %v", def.Contracts, tc.contracts)
			}
		})
	}
}
