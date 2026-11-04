package types_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/transient"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repairM1126Fixture struct {
	bank keeper.BaseKeeper
	ctx  sdk.Context
	cdc  *codec.ProtoCodec
	keys []sdk.StoreKey
}

func repairM1126Keeper(t *testing.T) repairM1126Fixture {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	bankKey := sdk.NewKVStoreKey(types.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	stores := make(map[storetypes.StoreKey]storetypes.CacheWrapper)
	keysByName := make(map[string]storetypes.StoreKey)
	keys := []sdk.StoreKey{bankKey, authKey, paramsKey, transientKey}
	for _, key := range keys {
		db := dbm.NewMemDB()
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		stores[key] = dbadapter.Store{DB: db}
		keysByName[key.Name()] = key
	}
	transientStore := transient.NewStore()
	t.Cleanup(func() { require.NoError(t, transientStore.DB.Close()) })
	stores[transientKey] = transientStore
	rootDB := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, rootDB.Close()) })
	ms := cachemulti.NewFromKVStore(dbadapter.Store{DB: rootDB}, stores, nil,
		keysByName, []storetypes.StoreKey{bankKey}, nil, nil, 0)
	ak := authkeeper.NewAccountKeeper(cdc, authKey,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, authtypes.ModuleName),
		authtypes.ProtoBaseAccount, map[string][]string{})
	bk := keeper.NewBaseKeeper(cdc, bankKey, ak,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, types.ModuleName).WithKeyTable(banktypes.ParamKeyTable()), nil)
	return repairM1126Fixture{bank: bk, ctx: sdk.NewContext(ms, tmproto.Header{Height: 1}, false), cdc: cdc, keys: keys}
}

func (f repairM1126Fixture) transport(gs *types.GenesisState) *types.GenesisState {
	return types.GetGenesisStateFromAppState(f.cdc, map[string]json.RawMessage{
		types.ModuleName: f.cdc.MustMarshalJSON(gs),
	})
}

func (f repairM1126Fixture) initialize(gs *types.GenesisState) error {
	if err := gs.Validate(); err != nil {
		return err
	}
	f.bank.InitGenesis(f.ctx, gs)
	return nil
}

func repairM1126Addresses() (sdk.AccAddress, sdk.AccAddress, sdk.AccAddress) {
	return sdk.AccAddress(bytes.Repeat([]byte{1}, 20)),
		sdk.AccAddress(bytes.Repeat([]byte{2}, 20)),
		sdk.AccAddress(bytes.Repeat([]byte{3}, 20))
}

func repairM1126BaseDenom(t *testing.T) string {
	t.Helper()
	if _, err := sdk.GetBaseDenom(); err != nil {
		require.NoError(t, sdk.RegisterDenom(sdk.DefaultBondDenom, sdk.OneDec()))
	}
	denom, err := sdk.GetBaseDenom()
	require.NoError(t, err)
	return denom
}

func TestRepairM_1126_InvalidEntriesBeforeInitGenesis(t *testing.T) {
	repairM1126BaseDenom(t)
	a, b, _ := repairM1126Addresses()
	unit := types.OneUhpxInWei
	for _, tc := range []struct {
		name      string
		balances  []types.WeiBalance
		errorText string
		direct    bool
	}{
		{"empty_address", []types.WeiBalance{{Address: "", Amount: sdk.ZeroInt()}}, "invalid wei balance address", false},
		{"malformed_address", []types.WeiBalance{{Address: "not-bech32", Amount: unit}}, "invalid wei balance address", false},
		{"bad_checksum", []types.WeiBalance{{Address: a.String() + "q", Amount: unit}}, "invalid wei balance address", false},
		{"wrong_prefix", []types.WeiBalance{{Address: sdk.ValAddress(a).String(), Amount: unit}}, "invalid wei balance address", false},
		{"uninitialized_amount", []types.WeiBalance{{Address: a.String()}}, "uninitialized wei balance amount", true},
		{"negative_amount_cancelled_by_positive", []types.WeiBalance{{Address: a.String(), Amount: unit.Neg()}, {Address: b.String(), Amount: unit}}, "negative wei balance amount", false},
		{"duplicate_address", []types.WeiBalance{{Address: a.String(), Amount: unit}, {Address: a.String(), Amount: unit}}, "duplicate wei balance", false},
		{"duplicate_zero", []types.WeiBalance{{Address: a.String(), Amount: sdk.ZeroInt()}, {Address: a.String(), Amount: sdk.ZeroInt()}}, "duplicate wei balance", false},
		{"duplicate_decoded_address", []types.WeiBalance{{Address: a.String(), Amount: unit}, {Address: strings.ToUpper(a.String()), Amount: unit}}, "duplicate wei balance", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := repairM1126Keeper(t)
			gs := types.NewGenesisState(types.DefaultParams(), nil, nil, nil, tc.balances)
			if !tc.direct {
				gs = f.transport(gs)
			}
			var err error
			require.NotPanics(t, func() { err = f.initialize(gs) })
			require.ErrorContains(t, err, tc.errorText)
			require.Empty(t, f.ctx.GigaKVStore(f.bank.GetStoreKey()).GetAllKeyStrsInRange(nil, nil), "invalid genesis wrote Giga bank state")
			for _, key := range f.keys {
				require.Empty(t, f.ctx.MultiStore().GetKVStore(key).GetAllKeyStrsInRange(nil, nil), "invalid genesis wrote %s", key.Name())
			}
			require.Empty(t, f.ctx.EventManager().Events())
		})
	}
}

func TestRepairM_1126_AcceptedTotalsPreserveSupply(t *testing.T) {
	denom := repairM1126BaseDenom(t)
	a, b, c := repairM1126Addresses()
	unit := types.OneUhpxInWei
	for _, explicitSupply := range []bool{false, true} {
		name := "computed_supply"
		if explicitSupply {
			name = "explicit_supply"
		}
		t.Run(name, func(t *testing.T) {
			f := repairM1126Keeper(t)
			supply := sdk.NewCoins(sdk.NewInt64Coin(denom, 12), sdk.NewInt64Coin("other", 7))
			gs := types.NewGenesisState(types.DefaultParams(), []types.Balance{
				{Address: a.String(), Coins: sdk.NewCoins(sdk.NewInt64Coin(denom, 5), sdk.NewInt64Coin("other", 7))},
				{Address: b.String(), Coins: sdk.NewCoins(sdk.NewInt64Coin(denom, 4))},
			}, nil, nil, []types.WeiBalance{
				{Address: a.String(), Amount: unit.MulRaw(2).AddRaw(1)},
				{Address: b.String(), Amount: unit.SubRaw(1)},
				{Address: c.String(), Amount: sdk.ZeroInt()},
			})
			if explicitSupply {
				gs.Supply = supply
			}
			gs = f.transport(gs)
			require.False(t, f.bank.UseRegularStore)
			require.NoError(t, f.initialize(gs))
			require.Equal(t, sdk.NewInt64Coin(denom, 7), f.bank.GetBalance(f.ctx, a, denom))
			require.Equal(t, sdk.NewInt64Coin(denom, 4), f.bank.GetBalance(f.ctx, b, denom))
			require.Equal(t, sdk.NewInt64Coin("other", 7), f.bank.GetBalance(f.ctx, a, "other"))
			require.True(t, f.bank.GetWeiBalance(f.ctx, a).Equal(sdk.OneInt()))
			require.True(t, f.bank.GetWeiBalance(f.ctx, b).Equal(unit.SubRaw(1)))
			require.True(t, f.bank.GetWeiBalance(f.ctx, c).IsZero())
			total := sdk.ZeroInt()
			for _, addr := range []sdk.AccAddress{a, b, c} {
				total = total.Add(f.bank.GetBalance(f.ctx, addr, denom).Amount.Mul(unit)).Add(f.bank.GetWeiBalance(f.ctx, addr))
			}
			require.True(t, total.Equal(supply.AmountOf(denom).Mul(unit)))
			for _, coin := range supply {
				require.Equal(t, coin, f.bank.GetSupply(f.ctx, coin.Denom))
			}
		})
	}
}

func TestRepairM_1126_SupplyChecksRemainEnforced(t *testing.T) {
	denom := repairM1126BaseDenom(t)
	a, _, _ := repairM1126Addresses()
	for _, tc := range []struct {
		name      string
		amount    sdk.Int
		supply    sdk.Coins
		errorText string
	}{
		{"fractional_total", sdk.OneInt(), nil, "non-zero wei remainder"},
		{"incorrect_supply", types.OneUhpxInWei, sdk.NewCoins(sdk.NewInt64Coin(denom, 2)), "genesis supply is incorrect"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := repairM1126Keeper(t)
			gs := f.transport(types.NewGenesisState(types.DefaultParams(), nil, tc.supply, nil,
				[]types.WeiBalance{{Address: a.String(), Amount: tc.amount}}))
			require.ErrorContains(t, gs.Validate(), tc.errorText)
		})
	}
}
