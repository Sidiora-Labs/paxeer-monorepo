package types

import (
	"bytes"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
)

func TestAgentGenesisValidateJobs(t *testing.T) {
	validJob := Job{
		Id:             1,
		Creator:        "0x1234567890abcdef1234567890abcdef12345678",
		Target:         "0xABCDEF1234567890ABCDEF1234567890ABCDEF12",
		CallData:       []byte{0xde, 0xad, 0xbe, 0xef},
		ExecuteAtBlock: 42,
		GasLimit:       21000,
		Deposit:        "1000000000000000000",
		Active:         true,
	}
	for _, tc := range []struct {
		name      string
		mutate    func(*GenesisState)
		wantError string
	}{
		{"valid job", func(*GenesisState) {}, ""},
		{"zero deposit", func(gs *GenesisState) { gs.Jobs[0].Deposit = "0" }, ""},
		{"maximum deposit", func(gs *GenesisState) {
			gs.Jobs[0].Deposit = "115792089237316195423570985008687907853269984665640564039457584007913129639935"
		}, ""},
		{"distinct unordered ids", func(gs *GenesisState) {
			gs.NextJobId = 4
			other := validJob
			other.Id = 3
			gs.Jobs = []Job{other, validJob}
		}, ""},
		{"maximum next id", func(gs *GenesisState) { gs.NextJobId = math.MaxUint64; gs.Jobs[0].Id = math.MaxUint64 - 1 }, ""},
		{"duplicate id", func(gs *GenesisState) { gs.Jobs = append(gs.Jobs, validJob) }, "duplicate job id"},
		{"id equals next", func(gs *GenesisState) { gs.Jobs[0].Id = gs.NextJobId }, "below next job id"},
		{"id exceeds next", func(gs *GenesisState) { gs.Jobs[0].Id = gs.NextJobId + 1 }, "below next job id"},
		{"maximum id equals next", func(gs *GenesisState) { gs.NextJobId = math.MaxUint64; gs.Jobs[0].Id = math.MaxUint64 }, "below next job id"},
		{"zero job id", func(gs *GenesisState) { gs.Jobs[0].Id = 0 }, "must be positive"},
		{"zero next id", func(gs *GenesisState) { gs.NextJobId = 0; gs.Jobs = nil }, "next job id must be positive"},
		{"zero target", func(gs *GenesisState) { gs.Jobs[0].Target = "0x0000000000000000000000000000000000000000" }, "target must be nonzero"},
		{"inactive", func(gs *GenesisState) { gs.Jobs[0].Active = false }, "must be active"},
		{"invalid params", func(gs *GenesisState) { gs.Params.MaxJobsPerCreator = 0 }, "max jobs per creator must be positive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gs := GenesisState{Params: DefaultParams(), Jobs: []Job{validJob}, NextJobId: 2}
			tc.mutate(&gs)
			err := gs.Validate()
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("valid genesis refused: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("validation = %v, want error containing %q", err, tc.wantError)
			}
		})
	}
	for _, address := range []string{"", "pax1invalid", validJob.Creator[2:], "0x1234", "0x" + strings.Repeat("1", 42), "0x" + strings.Repeat("z", 40), "0X" + validJob.Creator[2:]} {
		for _, field := range []string{"creator", "target"} {
			t.Run(field+"/"+address, func(t *testing.T) {
				gs := GenesisState{Params: DefaultParams(), Jobs: []Job{validJob}, NextJobId: 2}
				if field == "creator" {
					gs.Jobs[0].Creator = address
				} else {
					gs.Jobs[0].Target = address
				}
				if err := gs.Validate(); err == nil || !strings.Contains(err.Error(), field+" must be a 0x hex address") {
					t.Fatalf("invalid %s %q: validation = %v", field, address, err)
				}
			})
		}
	}
	for _, deposit := range []string{"", "-1", "1.5", "1e3", "not-an-integer", " 1", "1 ", "115792089237316195423570985008687907853269984665640564039457584007913129639936"} {
		t.Run("deposit/"+deposit, func(t *testing.T) {
			gs := GenesisState{Params: DefaultParams(), Jobs: []Job{validJob}, NextJobId: 2}
			gs.Jobs[0].Deposit = deposit
			if err := gs.Validate(); err == nil || !strings.Contains(err.Error(), "deposit must be a nonnegative integer") {
				t.Fatalf("invalid deposit %q: validation = %v", deposit, err)
			}
		})
	}
}

func TestAgentDefaultGenesisRoundTrip(t *testing.T) {
	gs := DefaultGenesis()
	if gs == nil {
		t.Fatal("DefaultGenesis returned nil")
	}
	if gs.Params != DefaultParams() || len(gs.Jobs) != 0 || gs.NextJobId != 1 {
		t.Fatalf("unexpected default genesis: %+v", gs)
	}
	if err := gs.Validate(); err != nil {
		t.Fatalf("default genesis invalid: %v", err)
	}
	encoded, err := ModuleCdc.MarshalAsJSON(gs)
	if err != nil {
		t.Fatalf("marshal default genesis: %v", err)
	}
	var decoded GenesisState
	if err := ModuleCdc.UnmarshalAsJSON(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal default genesis: %v", err)
	}
	if decoded.Params != gs.Params || len(decoded.Jobs) != 0 || decoded.NextJobId != 1 {
		t.Fatalf("default genesis changed after JSON round-trip: %+v", decoded)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("decoded default genesis invalid: %v", err)
	}
	reencoded, err := ModuleCdc.MarshalAsJSON(&decoded)
	if err != nil {
		t.Fatalf("reencode default genesis: %v", err)
	}
	if !bytes.Equal(encoded, reencoded) {
		t.Fatalf("default JSON changed: %s -> %s", encoded, reencoded)
	}

	gs.Jobs = []Job{{Id: 3, Creator: "0x1234567890abcdef1234567890abcdef12345678", Target: "0xabcdef1234567890abcdef1234567890abcdef12", CallData: []byte{0, 1, 0xff}, ExecuteAtBlock: 42, GasLimit: 21000, Deposit: "1000000000000000000", Active: true}}
	gs.NextJobId = 4
	encoded, err = ModuleCdc.MarshalAsJSON(gs)
	if err != nil {
		t.Fatalf("marshal populated genesis: %v", err)
	}
	if err := ModuleCdc.UnmarshalAsJSON(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal populated genesis: %v", err)
	}
	if !reflect.DeepEqual(*gs, decoded) {
		t.Fatalf("populated genesis changed after JSON round-trip: got %+v, want %+v", decoded, *gs)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("decoded populated genesis invalid: %v", err)
	}
	wire, err := proto.Marshal(gs)
	if err != nil {
		t.Fatalf("marshal genesis protobuf: %v", err)
	}
	var fromWire GenesisState
	if err := proto.Unmarshal(wire, &fromWire); err != nil {
		t.Fatalf("unmarshal genesis protobuf: %v", err)
	}
	if !reflect.DeepEqual(*gs, fromWire) {
		t.Fatalf("populated genesis changed after protobuf round-trip: got %+v, want %+v", fromWire, *gs)
	}
	if fresh := DefaultGenesis(); fresh == gs || fresh.Params != DefaultParams() || len(fresh.Jobs) != 0 || fresh.NextJobId != 1 {
		t.Fatalf("DefaultGenesis did not return independent defaults: %+v", fresh)
	}
}
