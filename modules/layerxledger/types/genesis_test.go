package types

import (
	"bytes"
	"testing"

	"github.com/gogo/protobuf/proto"
)

func TestLedgerGenesisValidate(t *testing.T) {
	gs := DefaultGenesis()
	if gs == nil {
		t.Fatal("default genesis is nil")
	}
	if gs.Params != DefaultParams() {
		t.Fatalf("default genesis params = %+v, want %+v", gs.Params, DefaultParams())
	}
	if err := gs.Validate(); err != nil {
		t.Fatalf("default genesis invalid: %v", err)
	}

	for _, tc := range []struct {
		name   string
		paused bool
		wire   []byte
	}{
		{name: "unpaused", paused: false, wire: []byte{0x0a, 0x00}},
		{name: "paused", paused: true, wire: []byte{0x0a, 0x02, 0x08, 0x01}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := GenesisState{Params: Params{Paused: tc.paused}}
			if err := state.Params.Validate(); err != nil {
				t.Fatalf("params invalid: %v", err)
			}
			if err := state.Validate(); err != nil {
				t.Fatalf("genesis validation disagrees with params validation: %v", err)
			}
			encoded, err := proto.Marshal(&state)
			if err != nil {
				t.Fatalf("marshal genesis: %v", err)
			}
			if !bytes.Equal(encoded, tc.wire) {
				t.Fatalf("genesis encoding = %x, want Params at field 1: %x", encoded, tc.wire)
			}
			var decoded GenesisState
			if err := proto.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("unmarshal genesis: %v", err)
			}
			if decoded.Params != state.Params {
				t.Fatalf("decoded params = %+v, want %+v", decoded.Params, state.Params)
			}
			if err := decoded.Validate(); err != nil {
				t.Fatalf("decoded genesis invalid: %v", err)
			}
		})
	}
}
