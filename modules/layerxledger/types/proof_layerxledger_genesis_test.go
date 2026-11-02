// Proves: types.DefaultGenesis and GenesisState.Validate -> protobuf GenesisState on disk -> proto.Unmarshal and GenesisState.Validate.
//
// The layerxledger-genesis inventory lists no external caller, so this test is
// the first consumer of the exported Go surface and its generated protobuf.
// The persisted component here is the genesis artifact: the producer syncs and
// closes a real file before a fresh reader and GenesisState recover it. Repeating
// production preserves its bytes and the consumer's observed params. Kernel
// records are outside this params-only contract and remain in the host store.
// Both pause values are valid; refusal belongs to the real protobuf decoder for
// malformed input, rather than inventing an invalid value for a boolean param.
//
// Repairs modules/layerxledger/types/params_test.go: TestParamsDefaultAndValidate
// referenced the retired KernelKVPrefix and prevented this proof from compiling.
// ParamsKey remains exactly 0x01; kernel storage stays with the shared host store
// under task 2.3.1 (decision.201), without restoring the retired prefix.
package types_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/gogo/protobuf/proto"
)

func TestProofLayerxledgerGenesisRestart(t *testing.T) {
	for _, tc := range []struct {
		name   string
		paused bool
		wire   []byte
	}{
		{name: "default", paused: false, wire: []byte{0x0a, 0x00}},
		{name: "paused", paused: true, wire: []byte{0x0a, 0x02, 0x08, 0x01}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "genesis.pb")
			produce := func() {
				state := types.DefaultGenesis()
				if state == nil || state.Params.Paused {
					t.Fatal("DefaultGenesis must produce an unpaused ledger")
				}
				if err := state.Validate(); err != nil {
					t.Fatalf("validate default producer state: %v", err)
				}
				state.Params.Paused = tc.paused
				if err := state.Validate(); err != nil {
					t.Fatalf("validate configured producer state: %v", err)
				}
				wire, err := proto.Marshal(state)
				if err != nil {
					t.Fatalf("encode producer genesis: %v", err)
				}
				if !bytes.Equal(wire, tc.wire) {
					t.Fatalf("producer encoding = %x, want params-only field 1: %x", wire, tc.wire)
				}
				proofLayerxledgerGenesisWrite(t, path, wire)
			}

			produce() // The producer state and its file handle end before consumption.
			first := proofLayerxledgerGenesisRead(t, path)
			var observed types.GenesisState
			if err := proto.Unmarshal(first, &observed); err != nil {
				t.Fatalf("decode reopened genesis: %v", err)
			}
			if err := observed.Validate(); err != nil {
				t.Fatalf("consumer rejected reopened genesis: %v", err)
			}
			if observed.GetParams().Paused != tc.paused {
				t.Fatalf("consumer paused = %v, want %v", observed.GetParams().Paused, tc.paused)
			}

			produce()
			repeated := proofLayerxledgerGenesisRead(t, path)
			if !bytes.Equal(first, repeated) {
				t.Fatalf("repeated production changed persisted genesis: %x -> %x", first, repeated)
			}
			var recovered types.GenesisState
			if err := proto.Unmarshal(repeated, &recovered); err != nil {
				t.Fatalf("decode repeated genesis: %v", err)
			}
			if err := recovered.Validate(); err != nil {
				t.Fatalf("consumer rejected repeated genesis: %v", err)
			}
			if recovered != observed {
				t.Fatalf("repeated consumer state = %+v, want %+v", recovered, observed)
			}
		})
	}
}

func TestProofLayerxledgerGenesisIndependentDefaults(t *testing.T) {
	first := types.DefaultGenesis()
	second := types.DefaultGenesis()
	if first == nil || second == nil || first == second {
		t.Fatal("default producers must return independent genesis states")
	}
	first.Params.Paused = true
	if second.Params != types.DefaultParams() || second.Params.Paused {
		t.Fatalf("changing one genesis altered another: %+v", second.Params)
	}
	if err := first.Validate(); err != nil {
		t.Fatalf("paused genesis is valid: %v", err)
	}
	if err := second.Validate(); err != nil {
		t.Fatalf("independent default genesis is valid: %v", err)
	}
}

func TestProofLayerxledgerGenesisMalformed(t *testing.T) {
	for _, paused := range []bool{false, true} {
		state := types.DefaultGenesis()
		state.Params.Paused = paused
		if err := state.Validate(); err != nil {
			t.Fatalf("validate producer before corruption: %v", err)
		}
		wire, err := proto.Marshal(state)
		if err != nil {
			t.Fatalf("encode producer before corruption: %v", err)
		}
		for _, corruption := range []string{"truncated", "wrong_wire_type"} {
			name := "unpaused/" + corruption
			if paused {
				name = "paused/" + corruption
			}
			t.Run(name, func(t *testing.T) {
				malformed := append([]byte(nil), wire...)
				if corruption == "truncated" {
					malformed = malformed[:len(malformed)-1]
				} else {
					malformed[0] = 0x08 // Params field 1 incorrectly claims varint wire type.
				}
				path := filepath.Join(t.TempDir(), "genesis.pb")
				proofLayerxledgerGenesisWrite(t, path, malformed)
				var observed types.GenesisState
				err := proto.Unmarshal(proofLayerxledgerGenesisRead(t, path), &observed)
				if err == nil {
					t.Fatal("consumer accepted malformed persisted genesis")
				}
				if corruption == "truncated" && !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatalf("truncation error = %v, want unexpected EOF", err)
				}
				if corruption == "wrong_wire_type" && !strings.Contains(err.Error(), "wrong wireType = 0 for field Params") {
					t.Fatalf("wrong wire type error = %v, want Params wire-type refusal", err)
				}
			})
		}
	}
}

func proofLayerxledgerGenesisWrite(t *testing.T, path string, wire []byte) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open producer genesis artifact: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	if n, err := f.Write(wire); err != nil || n != len(wire) {
		t.Fatalf("persist genesis: wrote %d of %d bytes: %v", n, len(wire), err)
	}
	if err := f.Sync(); err != nil {
		t.Fatalf("sync genesis artifact: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("stop producer by closing genesis artifact: %v", err)
	}
}

func proofLayerxledgerGenesisRead(t *testing.T, path string) []byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("reopen consumer genesis artifact: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	wire, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read reopened genesis: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close consumer genesis artifact: %v", err)
	}
	return wire
}
