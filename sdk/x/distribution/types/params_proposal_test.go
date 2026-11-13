package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	distrkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
)

type distributionParamsFixture struct {
	ctx       sdk.Context
	keeper    distrkeeper.Keeper
	space     paramtypes.Subspace
	authSpace paramtypes.Subspace
	cdc       *codec.ProtoCodec
	amino     *codec.LegacyAmino
	handler   govtypes.Handler
}

func distributionParamsSetup(t *testing.T, initial types.Params) distributionParamsFixture {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	cms := store.NewCommitMultiStore(db)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	distrKey := sdk.NewKVStoreKey(types.StoreKey)
	tkey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	for _, key := range []*sdk.KVStoreKey{paramsKey, authKey, distrKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(tkey, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{}, false)
	registry := codectypes.NewInterfaceRegistry()
	proposal.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	pk := paramskeeper.NewKeeper(cdc, amino, paramsKey, tkey)
	space := pk.Subspace(types.ModuleName)
	authSpace := pk.Subspace(authtypes.ModuleName)
	ak := authkeeper.NewAccountKeeper(cdc, authKey, authSpace, authtypes.ProtoBaseAccount,
		map[string][]string{types.ModuleName: nil})
	k := distrkeeper.NewKeeper(cdc, distrKey, space, ak, nil, nil, "fee_collector", nil)
	k.SetParams(ctx, initial)
	authParams := authtypes.DefaultParams()
	authSpace.SetParamSet(ctx, &authParams)
	cms.Commit(true)
	return distributionParamsFixture{ctx, k, space, authSpace, cdc, amino,
		k.ParamChangeProposalHandler(params.NewParamChangeProposalHandler(pk))}
}

func (f distributionParamsFixture) change(t *testing.T, key []byte, value interface{}) proposal.ParamChange {
	t.Helper()
	return f.changeInSubspace(t, types.ModuleName, key, value)
}

func (f distributionParamsFixture) changeInSubspace(t *testing.T, subspace string, key []byte, value interface{}) proposal.ParamChange {
	t.Helper()
	bz, err := f.amino.MarshalAsJSON(value)
	require.NoError(t, err)
	return proposal.NewParamChange(subspace, string(key), string(bz))
}

func (f distributionParamsFixture) content(t *testing.T, changes ...proposal.ParamChange) govtypes.Content {
	t.Helper()
	p := proposal.NewParameterChangeProposal("Distribution allocation budget", "Validate the complete prospective parameter set", changes, false)
	require.NoError(t, p.ValidateBasic())
	bz, err := f.cdc.MarshalInterface(p)
	require.NoError(t, err)
	var decoded govtypes.Content
	require.NoError(t, f.cdc.UnmarshalInterface(bz, &decoded))
	require.Equal(t, p, decoded)
	require.NoError(t, decoded.ValidateBasic())
	return decoded
}

func distributionParamsRates(tax, base, bonus string) types.Params {
	return types.Params{
		CommunityTax:        sdk.MustNewDecFromStr(tax),
		BaseProposerReward:  sdk.MustNewDecFromStr(base),
		BonusProposerReward: sdk.MustNewDecFromStr(bonus),
		WithdrawAddrEnabled: true,
	}
}

func (f distributionParamsFixture) unchanged(t *testing.T, before types.Params) {
	t.Helper()
	require.Equal(t, before, f.keeper.GetParams(f.ctx))
	for _, pair := range before.ParamSetPairs() {
		bz, err := f.amino.MarshalAsJSON(pair.Value)
		require.NoError(t, err)
		require.Equal(t, bz, f.space.GetRaw(f.ctx, pair.Key))
		require.False(t, f.space.Modified(f.ctx, pair.Key), string(pair.Key))
	}
}

func TestDistributionParamsKeeperRejectsInvalidSet(t *testing.T) {
	initial := distributionParamsRates("0.1", "0.2", "0.3")
	cases := map[string]types.Params{
		"combined rates":        distributionParamsRates("0", "0.8", "0.8"),
		"tax in combined bound": distributionParamsRates("0.3", "0.4", "0.4"),
		"negative late field":   distributionParamsRates("0.2", "0.3", "-0.1"),
		"individual bound":      distributionParamsRates("0", "0", "1.1"),
	}
	for _, field := range []string{"tax", "base", "bonus"} {
		p := initial
		switch field {
		case "tax":
			p.CommunityTax = sdk.Dec{}
		case "base":
			p.BaseProposerReward = sdk.Dec{}
		case "bonus":
			p.BonusProposerReward = sdk.Dec{}
		}
		cases["nil "+field] = p
	}
	for name, invalid := range cases {
		t.Run(name, func(t *testing.T) {
			f := distributionParamsSetup(t, initial)
			require.Error(t, invalid.ValidateBasic())
			require.Panics(t, func() { f.keeper.SetParams(f.ctx, invalid) })
			f.unchanged(t, initial)
		})
	}
}

func TestDistributionParamsProposalRejectsCombinedRates(t *testing.T) {
	for _, tc := range []struct {
		name    string
		initial types.Params
		key     []byte
		value   string
	}{
		{"base", distributionParamsRates("0.1", "0.2", "0.3"), types.ParamStoreKeyBaseProposerReward, "0.8"},
		{"bonus", distributionParamsRates("0.1", "0.3", "0.2"), types.ParamStoreKeyBonusProposerReward, "0.8"},
		{"tax", distributionParamsRates("0.1", "0.4", "0.4"), types.ParamStoreKeyCommunityTax, "0.3"},
		{"smallest excess", distributionParamsRates("0.1", "0.4", "0.4"), types.ParamStoreKeyCommunityTax, "0.200000000000000001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := distributionParamsSetup(t, tc.initial)
			p := f.content(t, f.change(t, tc.key, sdk.MustNewDecFromStr(tc.value)))
			require.ErrorContains(t, f.handler(f.ctx, p), "sum of base, bonus proposer rewards, and community tax")
			f.unchanged(t, tc.initial)
		})
	}
}

func TestDistributionParamsProposalRollsBackAllChanges(t *testing.T) {
	initial := types.DefaultParams()
	f := distributionParamsSetup(t, initial)
	p := f.content(t,
		f.changeInSubspace(t, authtypes.ModuleName, authtypes.KeyMaxMemoCharacters, uint64(42)),
		f.change(t, types.ParamStoreKeyWithdrawAddrEnabled, false),
		f.change(t, types.ParamStoreKeyBaseProposerReward, sdk.MustNewDecFromStr("0.8")),
		f.change(t, types.ParamStoreKeyBonusProposerReward, sdk.MustNewDecFromStr("0.8")),
	)
	require.ErrorContains(t, f.handler(f.ctx, p), "sum of base, bonus proposer rewards, and community tax")
	f.unchanged(t, initial)
	var memo uint64
	f.authSpace.Get(f.ctx, authtypes.KeyMaxMemoCharacters, &memo)
	require.Equal(t, authtypes.DefaultMaxMemoCharacters, memo)
	require.False(t, f.authSpace.Modified(f.ctx, authtypes.KeyMaxMemoCharacters))
}

func TestDistributionParamsProposalAcceptsFinalSet(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		name := "increase first"
		if reverse {
			name = "decrease first"
		}
		t.Run(name, func(t *testing.T) {
			f := distributionParamsSetup(t, distributionParamsRates("0.1", "0.2", "0.7"))
			changes := []proposal.ParamChange{
				f.change(t, types.ParamStoreKeyBaseProposerReward, sdk.MustNewDecFromStr("0.8")),
				f.change(t, types.ParamStoreKeyBonusProposerReward, sdk.MustNewDecFromStr("0.1")),
			}
			if reverse {
				changes[0], changes[1] = changes[1], changes[0]
			}
			changes = append(changes, f.change(t, types.ParamStoreKeyWithdrawAddrEnabled, false))
			require.NoError(t, f.handler(f.ctx, f.content(t, changes...)))
			expected := distributionParamsRates("0.1", "0.8", "0.1")
			expected.WithdrawAddrEnabled = false
			require.Equal(t, expected, f.keeper.GetParams(f.ctx))
			require.NoError(t, f.keeper.GetParams(f.ctx).ValidateBasic())
			require.Equal(t, expected.CommunityTax, f.keeper.GetCommunityTax(f.ctx))
			require.Equal(t, expected.BaseProposerReward, f.keeper.GetBaseProposerReward(f.ctx))
			require.Equal(t, expected.BonusProposerReward, f.keeper.GetBonusProposerReward(f.ctx))
			for _, key := range [][]byte{types.ParamStoreKeyBaseProposerReward, types.ParamStoreKeyBonusProposerReward, types.ParamStoreKeyWithdrawAddrEnabled} {
				require.True(t, f.space.Modified(f.ctx, key))
			}
		})
	}
	for _, p := range []types.Params{types.DefaultParams(), distributionParamsRates("1", "0", "0"), distributionParamsRates("0", "1", "0"), distributionParamsRates("0", "0", "1")} {
		f := distributionParamsSetup(t, types.DefaultParams())
		require.NotPanics(t, func() { f.keeper.SetParams(f.ctx, p) })
		require.Equal(t, p, f.keeper.GetParams(f.ctx))
	}
}

func TestDistributionParamsProposalPreservesIndividualValidation(t *testing.T) {
	for _, value := range []string{"-0.1", "1.1"} {
		t.Run(value, func(t *testing.T) {
			initial := types.DefaultParams()
			f := distributionParamsSetup(t, initial)
			p := f.content(t,
				f.change(t, types.ParamStoreKeyCommunityTax, sdk.MustNewDecFromStr("0.1")),
				f.change(t, types.ParamStoreKeyBonusProposerReward, sdk.MustNewDecFromStr(value)),
			)
			require.ErrorContains(t, f.handler(f.ctx, p), "bonus proposer reward")
			f.unchanged(t, initial)
		})
	}
}

func TestDistributionParamsProposalPreservesOtherSubspaces(t *testing.T) {
	f := distributionParamsSetup(t, types.DefaultParams())
	change := f.changeInSubspace(t, authtypes.ModuleName, authtypes.KeyMaxMemoCharacters, uint64(42))
	require.NoError(t, f.handler(f.ctx, f.content(t, change)))
	var memo uint64
	f.authSpace.Get(f.ctx, authtypes.KeyMaxMemoCharacters, &memo)
	require.Equal(t, uint64(42), memo)
	f.unchanged(t, types.DefaultParams())
}
