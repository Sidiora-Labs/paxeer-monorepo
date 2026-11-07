package types

import (
	"bytes"
	"math"
	"testing"

	"github.com/gogo/protobuf/proto"
)

func TestAuthGenesisValidate(t *testing.T) {
	gs := DefaultGenesis()
	if gs == nil {
		t.Fatal("DefaultGenesis returned nil")
	}
	if want := DefaultParams(); gs.Params != want {
		t.Fatalf("default genesis params = %+v, want %+v", gs.Params, want)
	}
	if err := gs.Validate(); err != nil {
		t.Fatalf("default genesis validation: %v", err)
	}
	gs.Params.Paused = true
	gs.Params.IdempotencyRetention = 42
	if fresh := DefaultGenesis(); fresh == gs || fresh.Params != DefaultParams() {
		t.Fatalf("DefaultGenesis did not return independent defaults: %+v", fresh)
	}

	for _, tc := range []struct {
		name   string
		params Params
		wire   []byte
	}{
		{name: "zero", wire: []byte{0x0a, 0x00}},
		{name: "paused", params: Params{Paused: true}, wire: []byte{0x0a, 0x02, 0x08, 0x01}},
		{name: "retention", params: Params{IdempotencyRetention: 1}, wire: []byte{0x0a, 0x02, 0x10, 0x01}},
		{name: "paused with retention", params: Params{Paused: true, IdempotencyRetention: 42}, wire: []byte{0x0a, 0x04, 0x08, 0x01, 0x10, 0x2a}},
		{name: "maximum retention", params: Params{Paused: true, IdempotencyRetention: math.MaxUint64}, wire: []byte{0x0a, 0x0d, 0x08, 0x01, 0x10, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := GenesisState{Params: tc.params}
			gotErr, wantErr := state.Validate(), tc.params.Validate()
			if (gotErr == nil) != (wantErr == nil) || (gotErr != nil && gotErr.Error() != wantErr.Error()) {
				t.Fatalf("genesis validation = %v, params validation = %v", gotErr, wantErr)
			}
			if state.Params != tc.params {
				t.Fatalf("validation changed params: %+v", state.Params)
			}
			if name := proto.MessageName(&state); name != "paxprotocol.paxchain.layerxauth.GenesisState" {
				t.Fatalf("protobuf message name = %q", name)
			}
			encoded, err := proto.Marshal(&state)
			if err != nil {
				t.Fatalf("marshal genesis: %v", err)
			}
			if !bytes.Equal(encoded, tc.wire) {
				t.Fatalf("genesis encoding = %x, want %x", encoded, tc.wire)
			}
			var decoded GenesisState
			if err := proto.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("unmarshal genesis: %v", err)
			}
			if decoded.Params != tc.params {
				t.Fatalf("round-trip params = %+v, want %+v", decoded.Params, tc.params)
			}
		})
	}
}
