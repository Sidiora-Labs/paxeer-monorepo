package layerxauth

import (
	"bytes"
	"testing"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
)

func TestAuthGasTable(t *testing.T) {
	expected := []struct {
		name      string
		write     bool
		surcharge uint64
	}{
		{"grantSession", true, 36680},
		{"grantSessionSuccessor", true, 38600},
		{"grantCapability", true, 34760},
		{"revokeGrant", true, 5000},
		{"resultByIdempotency", false, 1204},
		{"getGrant", false, 1108},
		{"isLive", false, 1300},
		{"sessionKey", false, 1108},
		{"capabilityScope", false, 1108},
		{"checkActivity", false, 1396},
	}
	if len(GasTable) != len(expected) {
		t.Fatalf("gas table has %d rows, want %d", len(GasTable), len(expected))
	}

	inputs := []struct {
		name string
		data []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"one_byte", []byte{1}},
		{"word", make([]byte, 32)},
		{"four_words", make([]byte, 128)},
		{"long_zero", make([]byte, 4096)},
		{"long_nonzero", bytes.Repeat([]byte{0xff}, 4096)},
	}
	for _, want := range expected {
		t.Run(want.name, func(t *testing.T) {
			row, ok := GasTable[want.name]
			if !ok {
				t.Fatal("missing gas row")
			}
			if row.Write != want.write {
				t.Errorf("Write = %t, want %t", row.Write, want.write)
			}
			if row.Surcharge != want.surcharge {
				t.Errorf("Surcharge = %d, want %d", row.Surcharge, want.surcharge)
			}
			for _, input := range inputs {
				t.Run(input.name, func(t *testing.T) {
					wantGas := pcommon.DefaultGasCost(input.data, want.write) + want.surcharge
					if got := row.Gas(input.data); got != wantGas {
						t.Errorf("Gas(%d bytes) = %d, want %d", len(input.data), got, wantGas)
					}
				})
			}
		})
	}
}
