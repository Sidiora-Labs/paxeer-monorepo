// Proves: types.DefaultGenesis and types.GenesisState.Validate -> proto.Marshal/Unmarshal of types.GenesisState -> TestProofLayerxauthGenesis consumer.
// The genesis inventory lists no external caller, so this external test package
// consumes only the exported Go surface and the real generated protobuf codec.
// Genesis carries only Params, with no persistent store to reopen. Each result
// is encoded and consumed twice into independent states; malformed producer
// bytes are refused by the decoder before semantic validation. Params.Validate
// accepts both pause values and every uint64 retention, so pause is preserved,
// not treated as an invalid genesis. No implementation defect is exposed here.
package types_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
)

func TestProofLayerxauthGenesisRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		want types.Params
		wire []byte
	}{
		{name: "default", wire: []byte{0x0a, 0x00}},
		{name: "paused with retention", want: types.Params{Paused: true, IdempotencyRetention: 42}, wire: []byte{0x0a, 0x04, 0x08, 0x01, 0x10, 0x2a}},
		{name: "maximum retention", want: types.Params{IdempotencyRetention: math.MaxUint64}, wire: []byte{0x0a, 0x0b, 0x10, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			producer := types.DefaultGenesis()
			if producer == nil || producer.Params != (types.Params{Paused: false, IdempotencyRetention: 0}) {
				t.Fatalf("DefaultGenesis = %v, want unpaused with zero retention", producer)
			}
			producer.Params = tc.want
			for round := 0; round < 2; round++ {
				if err := producer.Validate(); err != nil {
					t.Fatalf("round %d: producer validation: %v", round, err)
				}
				encoded, err := proto.Marshal(producer)
				if err != nil {
					t.Fatalf("round %d: marshal producer: %v", round, err)
				}
				if !bytes.Equal(encoded, tc.wire) {
					t.Fatalf("round %d: wire = %x, want %x", round, encoded, tc.wire)
				}
				var consumer types.GenesisState
				if err := proto.Unmarshal(encoded, &consumer); err != nil {
					t.Fatalf("round %d: consumer decode: %v", round, err)
				}
				if name := proto.MessageName(&consumer); name != "paxprotocol.paxchain.layerxauth.GenesisState" {
					t.Fatalf("consumer message = %q", name)
				}
				if err := consumer.Validate(); err != nil {
					t.Fatalf("round %d: consumer validation: %v", round, err)
				}
				if got := consumer.GetParams(); got != tc.want {
					t.Fatalf("round %d: consumer observed %+v, want %+v", round, got, tc.want)
				}
				consumer.Params.Paused = !tc.want.Paused
				consumer.Params.IdempotencyRetention = tc.want.IdempotencyRetention ^ 1
				if producer.Params != tc.want {
					t.Fatal("consumer mutation changed the producer")
				}
			}
			fresh := types.DefaultGenesis()
			if fresh == nil || fresh == producer || fresh.Params != (types.Params{}) {
				t.Fatalf("new default shares producer state: %v", fresh)
			}
		})
	}
}

func TestProofLayerxauthGenesisRejectsMalformed(t *testing.T) {
	producer := types.DefaultGenesis()
	producer.Params.Paused = true
	producer.Params.IdempotencyRetention = 42
	if err := producer.Validate(); err != nil {
		t.Fatalf("producer validation: %v", err)
	}
	encoded, err := proto.Marshal(producer)
	if err != nil {
		t.Fatalf("marshal producer: %v", err)
	}
	wrongType := append([]byte(nil), encoded...)
	wrongType[0] = 0x08 // Params field 1 must be length-delimited, not a varint.
	badParams := append([]byte(nil), encoded...)
	badParams[len(badParams)-1] |= 0x80 // Nested retention varint never terminates.
	for _, tc := range []struct {
		name string
		wire []byte
		want error
	}{
		{name: "truncated envelope", wire: encoded[:len(encoded)-1], want: io.ErrUnexpectedEOF},
		{name: "wrong Params wire type", wire: wrongType},
		{name: "malformed nested Params", wire: badParams, want: io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for round := 0; round < 2; round++ {
				var consumer types.GenesisState
				err := proto.Unmarshal(tc.wire, &consumer)
				if err == nil {
					t.Fatalf("round %d: consumer accepted malformed producer bytes %x", round, tc.wire)
				}
				if tc.want != nil {
					if !errors.Is(err, tc.want) {
						t.Fatalf("round %d: decode error = %v, want %v", round, err, tc.want)
					}
				} else if !strings.Contains(err.Error(), "wrong wireType = 0 for field Params") {
					t.Fatalf("round %d: expected Params wire-type refusal, got %v", round, err)
				}
			}
		})
	}
}
