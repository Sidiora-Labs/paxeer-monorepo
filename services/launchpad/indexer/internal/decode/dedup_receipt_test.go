package decode

import (
	"encoding/json"
	"os"
	"testing"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// TestDeduplicateFrozenReceipts keeps the first occurrence of each committed
// receipt log while preserving the order in which distinct logs arrived.
func TestDeduplicateFrozenReceipts(t *testing.T) {
	raw, err := os.ReadFile("../../../../../sdk/launchpad/protocol/contracts/v1/chain/golden-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		DecodedLogs []struct {
			Raw goldenRaw
		}
	}
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	var distinct []ethtypes.Log
	for _, entry := range fixtures.DecodedLogs {
		log := entry.Raw.log()
		if validIdentity(log) {
			distinct = append(distinct, log)
		}
		if len(distinct) == 2 {
			break
		}
	}
	if len(distinct) != 2 {
		t.Fatal("fixture contains fewer than two committed logs")
	}
	input := []ethtypes.Log{distinct[0], distinct[0], distinct[1]}
	got, err := DeduplicateRaw(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].TxHash != distinct[0].TxHash || got[1].TxHash != distinct[1].TxHash || got[0].Index != distinct[0].Index || got[1].Index != distinct[1].Index {
		t.Fatalf("deduplicated log identities: %+v", got)
	}
	if _, err := DeduplicateRaw([]ethtypes.Log{distinct[0], {}}); err == nil {
		t.Fatal("accepted a log without committed identity")
	}
}
