package protocol

import (
	"math/big"
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestRepairM_1012_RoleSchemaPath(t *testing.T) {
	testFeeStrategyChangedRoleSchemaPath(t)
}

func TestRepairM_1012_MalformedLogs(t *testing.T) {
	for _, role := range feeStrategyChangedTestRoles() {
		t.Run(role.name, func(t *testing.T) {
			valid := feeStrategyChangedTestLog(t, role, big.NewInt(73), 1, 2)
			def, ok := LookupEvent(valid.Topics[0])
			if !ok || def == nil {
				t.Fatal("missing role event")
			}
			consumer := feeStrategyChangedTestConsumer(valid.Address, def)
			cases := []struct {
				name   string
				mutate func(*types.Log)
			}{
				{"missing_signature", func(l *types.Log) { l.Topics = nil }},
				{"unknown_signature", func(l *types.Log) { l.Topics[0] = common.HexToHash("0xdeadbeef") }},
				{"missing_nft_topic", func(l *types.Log) { l.Topics = l.Topics[:1] }},
				{"extra_topic", func(l *types.Log) { l.Topics = append(l.Topics, common.Hash{}) }},
				{"truncated_strategy", func(l *types.Log) { l.Data = l.Data[:len(l.Data)-1] }},
				{"noncanonical_uint8", func(l *types.Log) { l.Data[0] = 1 }},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					malformed := valid
					malformed.Topics = slices.Clone(valid.Topics)
					malformed.Data = slices.Clone(valid.Data)
					tc.mutate(&malformed)
					if err := consumer.UnpackLogIntoMap(make(map[string]any), def.Name, malformed); err == nil {
						t.Fatalf("production ABI consumer accepted %s", tc.name)
					}
				})
			}
			if err := consumer.UnpackLogIntoMap(make(map[string]any), def.Name, valid); err != nil {
				t.Fatalf("malformed-log cases affected valid decoding: %v", err)
			}
		})
	}
}

func TestRepairM_1012_UnknownTopic(t *testing.T) {
	if def, ok := LookupEvent(common.HexToHash("0xdeadbeef")); ok || def != nil {
		t.Fatalf("unknown producer topic entered registry: %+v", def)
	}
}
