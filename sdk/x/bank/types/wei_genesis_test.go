package types_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func weiGenesisCodec() *codec.ProtoCodec {
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	return codec.NewProtoCodec(registry)
}

func weiGenesisAddress(n byte) sdk.AccAddress {
	return sdk.AccAddress(bytes.Repeat([]byte{n}, 20))
}

func weiGenesisRoundTrip(t *testing.T, gs *banktypes.GenesisState) banktypes.GenesisState {
	t.Helper()
	cdc := weiGenesisCodec()
	raw, err := cdc.MarshalAsJSON(gs)
	require.NoError(t, err)
	return *banktypes.GetGenesisStateFromAppState(cdc, map[string]json.RawMessage{banktypes.ModuleName: raw})
}

func weiGenesisReject(t *testing.T, balances []banktypes.WeiBalance, message string) {
	t.Helper()
	gs := banktypes.DefaultGenesisState()
	gs.WeiBalances = balances
	decoded := weiGenesisRoundTrip(t, gs)
	var err error
	require.NotPanics(t, func() { err = decoded.Validate() })
	require.ErrorContains(t, err, message)
}

func TestWeiGenesisInvalidAddress(t *testing.T) {
	for _, address := range []string{"", "not-an-address", weiGenesisAddress(1).String() + "x"} {
		t.Run(address, func(t *testing.T) {
			weiGenesisReject(t, []banktypes.WeiBalance{{Address: address, Amount: sdk.ZeroInt()}}, "invalid wei balance address")
		})
	}
}

func TestWeiGenesisNegativeAmount(t *testing.T) {
	// Offsetting entries conserve aggregate supply but remain invalid individually.
	weiGenesisReject(t, []banktypes.WeiBalance{
		{Address: weiGenesisAddress(1).String(), Amount: sdk.NewInt(-1)},
		{Address: weiGenesisAddress(2).String(), Amount: sdk.OneInt()},
	}, "must be in [0,")
}

func TestWeiGenesisUninitializedAmount(t *testing.T) {
	gs := banktypes.DefaultGenesisState()
	gs.WeiBalances = []banktypes.WeiBalance{{Address: weiGenesisAddress(1).String()}}
	var err error
	require.NotPanics(t, func() { err = gs.Validate() })
	require.ErrorContains(t, err, "uninitialized wei balance")

	// Omission on the genesis JSON boundary must retain the same validation error.
	gs.WeiBalances[0].Amount = sdk.ZeroInt()
	cdc := weiGenesisCodec()
	raw, err := cdc.MarshalAsJSON(gs)
	require.NoError(t, err)
	var document map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &document))
	var balances []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(document["wei_balances"], &balances))
	delete(balances[0], "amount")
	document["wei_balances"], err = json.Marshal(balances)
	require.NoError(t, err)
	raw, err = json.Marshal(document)
	require.NoError(t, err)
	decoded := banktypes.GetGenesisStateFromAppState(cdc, map[string]json.RawMessage{banktypes.ModuleName: raw})
	require.True(t, decoded.WeiBalances[0].Amount.IsNil())
	require.NotPanics(t, func() { err = decoded.Validate() })
	require.ErrorContains(t, err, "uninitialized wei balance")
}

func TestWeiGenesisDuplicateAddress(t *testing.T) {
	a := weiGenesisAddress(1).String()
	for _, duplicate := range []string{a, strings.ToUpper(a)} {
		t.Run(duplicate, func(t *testing.T) {
			addr, err := sdk.AccAddressFromBech32(duplicate)
			require.NoError(t, err)
			require.Equal(t, weiGenesisAddress(1), addr)
			weiGenesisReject(t, []banktypes.WeiBalance{
				{Address: a, Amount: sdk.ZeroInt()},
				{Address: duplicate, Amount: sdk.ZeroInt()},
			}, "duplicate wei balance")
		})
	}
}

func TestWeiGenesisFractionalBounds(t *testing.T) {
	for _, amount := range []sdk.Int{banktypes.OneUhpxInWei, banktypes.OneUhpxInWei.MulRaw(2)} {
		t.Run(amount.String(), func(t *testing.T) {
			weiGenesisReject(t, []banktypes.WeiBalance{
				{Address: weiGenesisAddress(1).String(), Amount: amount},
			}, "must be in [0,")
		})
	}
	weiGenesisReject(t, []banktypes.WeiBalance{
		{Address: weiGenesisAddress(1).String(), Amount: sdk.OneInt()},
	}, "non-zero wei remainder")
}

func weiGenesisKeeper(t *testing.T) (keeper.BaseKeeper, sdk.Context) {
	t.Helper()
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	for _, key := range []*sdk.KVStoreKey{bankKey, authKey, paramsKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	cdc := weiGenesisCodec()
	amino := codec.NewLegacyAmino()
	bankParams := paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, banktypes.ModuleName)
	authParams := paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, authtypes.ModuleName)
	ak := authkeeper.NewAccountKeeper(cdc, authKey, authParams, authtypes.ProtoBaseAccount, map[string][]string{})
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1}, false)
	ak.SetParams(ctx, authtypes.DefaultParams())
	return keeper.NewBaseKeeper(cdc, bankKey, ak, bankParams, map[string]bool{}), ctx
}

func TestWeiGenesisValidAggregateCarry(t *testing.T) {
	baseDenom, err := sdk.GetBaseDenom()
	if err != nil {
		require.NoError(t, sdk.RegisterDenom("uhpx", sdk.OneDec()))
		baseDenom, err = sdk.GetBaseDenom()
	}
	require.NoError(t, err)
	a, b, c := weiGenesisAddress(1), weiGenesisAddress(2), weiGenesisAddress(3)
	gs := banktypes.NewGenesisState(banktypes.DefaultParams(), []banktypes.Balance{
		{Address: a.String(), Coins: sdk.NewCoins(sdk.NewInt64Coin(baseDenom, 7))},
	}, sdk.NewCoins(sdk.NewInt64Coin(baseDenom, 8)), nil, []banktypes.WeiBalance{
		{Address: a.String(), Amount: banktypes.OneUhpxInWei.Sub(sdk.OneInt())},
		{Address: b.String(), Amount: sdk.OneInt()},
		{Address: c.String(), Amount: sdk.ZeroInt()},
	})
	decoded := weiGenesisRoundTrip(t, gs)
	require.NoError(t, decoded.Validate())
	k, ctx := weiGenesisKeeper(t)
	require.NotPanics(t, func() { k.InitGenesis(ctx, &decoded) })
	require.True(t, k.GetWeiBalance(ctx, a).Equal(banktypes.OneUhpxInWei.Sub(sdk.OneInt())))
	require.True(t, k.GetWeiBalance(ctx, b).Equal(sdk.OneInt()))
	require.True(t, k.GetWeiBalance(ctx, c).IsZero())
	require.Equal(t, sdk.NewInt64Coin(baseDenom, 7), k.GetBalance(ctx, a, baseDenom))
	require.True(t, k.GetBalance(ctx, b, baseDenom).IsZero())
	require.Equal(t, sdk.NewInt64Coin(baseDenom, 8), k.GetSupply(ctx, baseDenom))
	exported := weiGenesisRoundTrip(t, k.ExportGenesis(ctx))
	require.NoError(t, exported.Validate())
	require.Equal(t, gs.Supply, exported.Supply)
	require.Equal(t, gs.WeiBalances[:2], exported.WeiBalances)

	decoded.Supply = sdk.NewCoins(sdk.NewInt64Coin(baseDenom, 7))
	require.ErrorContains(t, decoded.Validate(), "genesis supply is incorrect")
}
