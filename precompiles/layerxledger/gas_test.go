package layerxledger_test

import (
	"bytes"
	"fmt"
	"testing"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
)

func TestLedgerGasTable(t *testing.T) {
	var table pcommon.GasTable = layerxledger.GasTable
	expected := []struct {
		name      string
		write     bool
		surcharge uint64
	}{
		{"register", true, 18440},
		{"pause", true, 3080},
		{"unpause", true, 3080},
		{"openAccount", true, 3080},
		{"send", true, 17480},
		{"draw", true, 8840},
		{"payExact", true, 7880},
		{"issueGrant", true, 16520},
		{"revokeGrant", true, 4040},
		{"mint", true, 5000},
		{"burn", true, 5000},
		{"getAsset", false, 1108},
		{"balanceOf", false, 1204},
		{"totalUnits", false, 1108},
		{"accountId", false, 1204},
		{"accountOwner", false, 1108},
		{"nextSequence", false, 1204},
		{"getPayerGrant", false, 1108},
	}
	if len(table) != len(expected) {
		t.Fatalf("GasTable has %d methods, want %d", len(table), len(expected))
	}
	for _, want := range expected {
		t.Run(want.name, func(t *testing.T) {
			row, ok := table[want.name]
			if !ok {
				t.Fatalf("GasTable is missing method %q", want.name)
			}
			if row.Write != want.write {
				t.Errorf("Write = %t, want %t", row.Write, want.write)
			}
			if row.Surcharge != want.surcharge {
				t.Errorf("Surcharge = %d, want %d", row.Surcharge, want.surcharge)
			}
			flat, perByte := uint64(1000), uint64(3)
			if want.write {
				flat, perByte = 2000, 30
			}
			if got := row.Gas(nil); got != flat+want.surcharge {
				t.Errorf("Gas(nil) = %d, want %d", got, flat+want.surcharge)
			}
			for _, length := range []int{0, 1, 4, 32, 36, 68, 256, 420, 4096} {
				t.Run(fmt.Sprintf("input_bytes_%d", length), func(t *testing.T) {
					wantGas := flat + perByte*uint64(length) + want.surcharge
					for _, fill := range []byte{0x00, 0xff} {
						input := bytes.Repeat([]byte{fill}, length)
						if got := row.Gas(input); got != wantGas {
							t.Errorf("Gas(%d bytes of %#x) = %d, want %d", length, fill, got, wantGas)
						}
					}
				})
			}
		})
	}
}
