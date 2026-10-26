package api_test

import (
	"testing"

	oracletypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Params = oracletypes.Params
type MsgAggregateExchangeRateVote = oracletypes.MsgAggregateExchangeRateVote

func TestOracleRemovedFieldsReserved(t *testing.T) {
	for _, tc := range []struct {
		file    string
		message string
		value   interface{ Descriptor() ([]byte, []int) }
		number  int32
		name    string
	}{
		{file: "oracle/oracle.proto", message: "Params", value: &Params{}, number: 8},
		{file: "oracle/tx.proto", message: "MsgAggregateExchangeRateVote", value: &MsgAggregateExchangeRateVote{}, number: 1, name: "salt"},
		{file: "oracle/genesis.proto", message: "GenesisState", value: &oracletypes.GenesisState{}, number: 5},
	} {
		t.Run(tc.message, func(t *testing.T) {
			for _, source := range []struct {
				name string
				read func(*testing.T) *descriptorpb.FileDescriptorProto
			}{
				{name: "source", read: func(t *testing.T) *descriptorpb.FileDescriptorProto { return bufFile(t, tc.file) }},
				{name: "generated", read: func(t *testing.T) *descriptorpb.FileDescriptorProto { return generatedFile(t, tc.value) }},
				{name: "registered", read: func(t *testing.T) *descriptorpb.FileDescriptorProto { return registeredFile(t, tc.file) }},
			} {
				t.Run(source.name, func(t *testing.T) {
					file := source.read(t)
					if file.GetName() != tc.file {
						t.Fatalf("descriptor file = %q, want %q", file.GetName(), tc.file)
					}
					var message *descriptorpb.DescriptorProto
					for _, candidate := range file.GetMessageType() {
						if candidate.GetName() == tc.message {
							if message != nil {
								t.Fatalf("%s: duplicate message %s", tc.file, tc.message)
							}
							message = candidate
						}
					}
					if message == nil {
						t.Fatalf("%s: missing message %s", tc.file, tc.message)
					}
					ranges := message.GetReservedRange()
					if len(ranges) != 1 || ranges[0].GetStart() != tc.number || ranges[0].GetEnd() != tc.number+1 {
						t.Errorf("reserved ranges = %v, want [%d, %d)", ranges, tc.number, tc.number+1)
					}
					if tc.name != "" {
						names := message.GetReservedName()
						if len(names) != 1 || names[0] != tc.name {
							t.Errorf("reserved names = %v, want [%s]", names, tc.name)
						}
					}
					for _, field := range message.GetField() {
						if field.GetNumber() == tc.number {
							t.Errorf("field %q reuses reserved number %d", field.GetName(), tc.number)
						}
						if tc.name != "" && field.GetName() == tc.name {
							t.Errorf("field %q reuses reserved name", tc.name)
						}
					}
				})
			}
		})
	}
}
