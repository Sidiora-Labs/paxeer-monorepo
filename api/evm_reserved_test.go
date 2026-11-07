package api_test

import (
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestEvmParamsRemovedFieldsReserved(t *testing.T) {
	type Params = evmtypes.Params

	for _, source := range []struct {
		name string
		read func(*testing.T) *descriptorpb.FileDescriptorProto
	}{
		{name: "source", read: func(t *testing.T) *descriptorpb.FileDescriptorProto {
			return bufFile(t, "evm/params.proto")
		}},
		{name: "generated", read: func(t *testing.T) *descriptorpb.FileDescriptorProto {
			return generatedFile(t, &Params{})
		}},
		{name: "registered", read: func(t *testing.T) *descriptorpb.FileDescriptorProto {
			return registeredFile(t, "evm/params.proto")
		}},
	} {
		t.Run(source.name, func(t *testing.T) {
			file := source.read(t)
			if file.GetName() != "evm/params.proto" {
				t.Fatalf("descriptor file = %q, want evm/params.proto", file.GetName())
			}
			for _, tc := range []struct {
				message string
				numbers []int32
				names   []string
			}{
				{
					message: "Params",
					numbers: []int32{1, 5, 6, 7},
					names:   []string{"base_denom", "chain_config", "chain_id", "whitelisted_codehashes_bank_send"},
				},
				{
					message: "AllowedFeeDenom",
					numbers: []int32{2},
					names:   []string{"oracle_pair"},
				},
			} {
				t.Run(tc.message, func(t *testing.T) {
					var message *descriptorpb.DescriptorProto
					for _, candidate := range file.GetMessageType() {
						if candidate.GetName() == tc.message {
							if message != nil {
								t.Fatalf("duplicate message %s", tc.message)
							}
							message = candidate
						}
					}
					if message == nil {
						t.Fatalf("missing message %s", tc.message)
					}
					for _, number := range tc.numbers {
						reserved := false
						for _, interval := range message.GetReservedRange() {
							if interval.GetStart() <= number && number < interval.GetEnd() {
								reserved = true
								break
							}
						}
						if !reserved {
							t.Errorf("%s does not reserve field number %d", tc.message, number)
						}
						for _, field := range message.GetField() {
							if field.GetNumber() == number {
								t.Errorf("%s field %q reuses removed field number %d", tc.message, field.GetName(), number)
							}
						}
					}
					for _, name := range tc.names {
						reserved := false
						for _, candidate := range message.GetReservedName() {
							if candidate == name {
								reserved = true
								break
							}
						}
						if !reserved {
							t.Errorf("%s does not reserve field name %q", tc.message, name)
						}
						for _, field := range message.GetField() {
							if field.GetName() == name {
								t.Errorf("%s field %q reuses a removed field name", tc.message, name)
							}
						}
					}
					for _, field := range message.GetField() {
						for _, interval := range message.GetReservedRange() {
							if interval.GetStart() <= field.GetNumber() && field.GetNumber() < interval.GetEnd() {
								t.Errorf("%s field %q reuses reserved number %d", tc.message, field.GetName(), field.GetNumber())
							}
						}
					}
				})
			}
		})
	}
}
