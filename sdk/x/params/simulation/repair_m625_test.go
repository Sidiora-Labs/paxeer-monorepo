package simulation_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	simtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/simulation"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	paramssimulation "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/simulation"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	xsimulation "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/simulation"
)

func TestRepairM_625_SingleEntry(t *testing.T) {
	for seed := int64(0); seed < 16; seed++ {
		repairM625CheckProposal(t, 1, seed)
	}
}

func TestRepairM_625_FullPool(t *testing.T) {
	for _, size := range []int{2, 7, 1000} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			for _, count := range []int{1, size} {
				seed := repairM625SeedForCount(t, size, count)
				repairM625CheckProposal(t, size, seed)
			}
		})
	}
}

func TestRepairM_625_PoolLimit(t *testing.T) {
	for _, size := range []int{1001, 1100} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			seed := repairM625SeedForCount(t, 1000, 1000)
			repairM625CheckProposal(t, size, seed)
			for seed := int64(0); seed < 64; seed++ {
				repairM625CheckProposal(t, size, seed)
			}
		})
	}
}

func repairM625SeedForCount(t *testing.T, limit, count int) int64 {
	t.Helper()
	for seed := int64(0); seed < 20000; seed++ {
		r := rand.New(rand.NewSource(seed))
		if simtypes.RandIntBetween(r, 1, limit+1) == count {
			return seed
		}
	}
	t.Fatalf("no seed selected endpoint %d for limit %d", count, limit)
	return 0
}

func repairM625CheckProposal(t *testing.T, size int, seed int64) {
	t.Helper()
	pool := make([]simtypes.ParamChange, size)
	keys := make(map[string]bool, size)
	for i := range pool {
		key := fmt.Sprintf("Param%d", i)
		keys[key] = true
		pool[i] = xsimulation.NewSimParamChange("repair625", key, func(r *rand.Rand) string {
			return strconv.Itoa(simtypes.RandIntBetween(r, 1, 1001))
		})
	}

	limit := size
	if limit > 1000 {
		limit = 1000
	}
	expected := simtypes.RandIntBetween(rand.New(rand.NewSource(seed)), 1, limit+1)
	weighted := paramssimulation.ProposalContents(pool)
	if len(weighted) != 1 {
		t.Fatalf("got %d weighted proposal contents, want 1", len(weighted))
	}
	content := weighted[0].ContentSimulatorFn()(rand.New(rand.NewSource(seed)), sdk.Context{}, nil)
	generated, ok := content.(*proposal.ParameterChangeProposal)
	if !ok {
		t.Fatalf("generated content has type %T", content)
	}
	if len(generated.Changes) != expected {
		t.Fatalf("pool %d, seed %d: got %d changes, want %d", size, seed, len(generated.Changes), expected)
	}
	if err := generated.ValidateBasic(); err != nil {
		t.Fatalf("generated proposal is invalid: %v", err)
	}

	registry := codectypes.NewInterfaceRegistry()
	govtypes.RegisterInterfaces(registry)
	proposal.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	bz, err := cdc.MarshalInterface(generated)
	if err != nil {
		t.Fatalf("marshal generated proposal: %v", err)
	}
	var decoded govtypes.Content
	if err := cdc.UnmarshalInterface(bz, &decoded); err != nil {
		t.Fatalf("unmarshal governance content: %v", err)
	}
	if !reflect.DeepEqual(generated, decoded) {
		t.Fatal("proposal changed across the governance codec")
	}
	if err := decoded.ValidateBasic(); err != nil {
		t.Fatalf("decoded proposal is invalid: %v", err)
	}

	seen := make(map[string]bool, expected)
	for _, change := range decoded.(*proposal.ParameterChangeProposal).Changes {
		if change.Subspace != "repair625" || !keys[change.Key] {
			t.Fatalf("change is outside the input pool: %+v", change)
		}
		if seen[change.Key] {
			t.Fatalf("parameter %s selected twice", change.Key)
		}
		seen[change.Key] = true
		value, err := strconv.Atoi(change.Value)
		if err != nil || value < 1 || value > 1000 {
			t.Fatalf("invalid simulated value %q", change.Value)
		}
	}
}
