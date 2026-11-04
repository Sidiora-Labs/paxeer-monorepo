package params_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
)

func repairM628Components(table paramtypes.KeyTable) (sdk.Context, keeper.Keeper, paramtypes.Subspace, *codec.ProtoCodec, *codec.LegacyAmino) {
	key := sdk.NewKVStoreKey(paramtypes.StoreKey)
	tkey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	ctx := testutil.DefaultContext(key, tkey)
	registry := codectypes.NewInterfaceRegistry()
	proposal.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	k := keeper.NewKeeper(cdc, amino, key, tkey)
	ss := k.Subspace("repairM628").WithKeyTable(table)
	return ctx, k, ss, cdc, amino
}

func repairM628Proposal(t *testing.T, cdc *codec.ProtoCodec, amino *codec.LegacyAmino, key string, value interface{}) govtypes.Content {
	t.Helper()
	valueJSON, err := amino.MarshalAsJSON(value)
	require.NoError(t, err)
	p := proposal.NewParameterChangeProposal("Exact key registration", "Exercise the registered parameter contract", []proposal.ParamChange{
		proposal.NewParamChange("repairM628", key, string(valueJSON)),
	}, false)
	require.NoError(t, p.ValidateBasic())
	wire, err := cdc.MarshalInterface(p)
	require.NoError(t, err)
	var decoded govtypes.Content
	require.NoError(t, cdc.UnmarshalInterface(wire, &decoded))
	require.Equal(t, p, decoded)
	require.NoError(t, decoded.ValidateBasic())
	return decoded
}

func repairM628Positive(value interface{}) error {
	v, ok := value.(uint64)
	if !ok || v == 0 {
		return fmt.Errorf("expected a positive uint64, got %v", value)
	}
	return nil
}

func TestRepairM_628_ExactRegisteredKeys(t *testing.T) {
	doc, err := os.ReadFile("spec/02_subspace.md")
	require.NoError(t, err)
	text := strings.Join(strings.Fields(string(doc)), " ")
	require.Contains(t, text, "`ExampleParameterChild` must be registered separately with its own type and validator")
	require.Contains(t, text, "executing an update for an unregistered key panics")

	primary := []byte("ExampleParameter")
	table := paramtypes.NewKeyTable(paramtypes.NewParamSetPair(primary, new(uint64), repairM628Positive))
	ctx, k, ss, cdc, amino := repairM628Components(table)
	handler := params.NewParamChangeProposalHandler(k)
	initial := uint64(7)
	require.NoError(t, handler(ctx, repairM628Proposal(t, cdc, amino, string(primary), initial)))
	var got uint64
	ss.Get(ctx, primary, &got)
	require.Equal(t, initial, got)
	require.True(t, ss.Modified(ctx, primary))
	encoded, err := amino.MarshalAsJSON(initial)
	require.NoError(t, err)
	require.Equal(t, encoded, ss.GetRaw(ctx, primary))

	for _, key := range []string{"ExampleParameterChild", "ExampleParameter/Child", "exampleParameter", "Example"} {
		t.Run(key, func(t *testing.T) {
			unregistered := []byte(key)
			expected := "parameter " + key + " not registered"
			require.EqualError(t, ss.Validate(ctx, unregistered, initial), expected)
			require.PanicsWithValue(t, expected, func() { ss.Set(ctx, unregistered, initial) })
			require.PanicsWithValue(t, expected, func() { ss.Get(ctx, unregistered, &got) })
			content := repairM628Proposal(t, cdc, amino, key, initial)
			require.PanicsWithValue(t, expected, func() { _ = handler(ctx, content) })
			require.False(t, ss.Has(ctx, unregistered))
			require.False(t, ss.Modified(ctx, unregistered))
			require.Equal(t, encoded, ss.GetRaw(ctx, primary))

			// Raw storage does not grant registration to typed reads or updates.
			ss.SetRaw(ctx, unregistered, encoded)
			require.Equal(t, encoded, ss.GetRaw(ctx, unregistered))
			require.PanicsWithValue(t, expected, func() { ss.GetIfExists(ctx, unregistered, &got) })
			require.PanicsWithValue(t, expected, func() { _ = handler(ctx, content) })
			require.Equal(t, encoded, ss.GetRaw(ctx, unregistered))
		})
	}
	ss.Get(ctx, primary, &got)
	require.Equal(t, initial, got)
}

func TestRepairM_628_IndependentAttributes(t *testing.T) {
	primary := []byte("ExampleParameter")
	child := []byte("ExampleParameterChild")
	validateChild := func(value interface{}) error {
		if value != "enabled" && value != "disabled" {
			return fmt.Errorf("expected enabled or disabled, got %v", value)
		}
		return nil
	}
	table := paramtypes.NewKeyTable(
		paramtypes.NewParamSetPair(primary, new(uint64), repairM628Positive),
		paramtypes.NewParamSetPair(child, new(string), validateChild),
	)
	ctx, k, ss, cdc, amino := repairM628Components(table)
	handler := params.NewParamChangeProposalHandler(k)
	require.NoError(t, handler(ctx, repairM628Proposal(t, cdc, amino, string(primary), uint64(7))))
	require.NoError(t, handler(ctx, repairM628Proposal(t, cdc, amino, string(child), "enabled")))
	var parentValue uint64
	var childValue string
	ss.Get(ctx, primary, &parentValue)
	ss.Get(ctx, child, &childValue)
	require.Equal(t, uint64(7), parentValue)
	require.Equal(t, "enabled", childValue)
	require.True(t, ss.Modified(ctx, primary))
	require.True(t, ss.Modified(ctx, child))
	require.PanicsWithValue(t, "type mismatch with registered table", func() { ss.Set(ctx, child, uint64(9)) })

	beforePrimary, beforeChild := ss.GetRaw(ctx, primary), ss.GetRaw(ctx, child)
	require.ErrorContains(t, handler(ctx, repairM628Proposal(t, cdc, amino, string(primary), uint64(0))), "expected a positive uint64")
	require.ErrorContains(t, handler(ctx, repairM628Proposal(t, cdc, amino, string(child), "invalid")), "expected enabled or disabled")
	require.Equal(t, beforePrimary, ss.GetRaw(ctx, primary))
	require.Equal(t, beforeChild, ss.GetRaw(ctx, child))
	require.NoError(t, handler(ctx, repairM628Proposal(t, cdc, amino, string(child), "disabled")))
	ss.Get(ctx, child, &childValue)
	require.Equal(t, "disabled", childValue)
	ss.Get(ctx, primary, &parentValue)
	require.Equal(t, uint64(7), parentValue)
}
