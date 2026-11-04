package types_test

import (
	"strings"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/ante"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repairM627Fixture struct {
	ctx      sdk.Context
	keeper   paramskeeper.Keeper
	space    types.Subspace
	amino    *codec.LegacyAmino
	cdc      *codec.ProtoCodec
	accounts authkeeper.AccountKeeper
	bank     bankkeeper.BaseKeeper
	payer    sdk.AccAddress
}

func newRepairM627Fixture(t *testing.T) repairM627Fixture {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ms := store.NewCommitMultiStore(db)
	paramsKey := sdk.NewKVStoreKey(types.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	deferredKey := sdk.NewKVStoreKey("repair_m627_deferred")
	transientKey := sdk.NewTransientStoreKey(types.TStoreKey)
	for _, key := range []*sdk.KVStoreKey{paramsKey, authKey, bankKey, deferredKey} {
		ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	ms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, db)
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, tmproto.Header{Height: 1}, false).WithTxIndex(0)
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	k := paramskeeper.NewKeeper(cdc, amino, paramsKey, transientKey)
	space, ok := k.GetSubspace(types.ModuleName)
	require.True(t, ok)
	accounts := authkeeper.NewAccountKeeper(cdc, authKey, k.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{authtypes.FeeCollectorName: {}})
	bank := bankkeeper.NewBaseKeeperWithDeferredCache(cdc, bankKey, accounts, k.Subspace(banktypes.ModuleName), map[string]bool{}, deferredKey)
	accounts.SetParams(ctx, authtypes.DefaultParams())
	bank.SetParams(ctx, banktypes.DefaultParams())
	payer := sdk.AccAddress([]byte("repair-m627-payer-01"))
	accounts.SetAccount(ctx, accounts.NewAccountWithAddress(ctx, payer))
	k.SetFeesParams(ctx, *types.DefaultFeesParams())
	return repairM627Fixture{ctx: ctx, keeper: k, space: space, amino: amino, cdc: cdc, accounts: accounts, bank: bank, payer: payer}
}

func (f repairM627Fixture) update(t *testing.T, params types.FeesParams) error {
	t.Helper()
	proposal, err := f.amino.MarshalAsJSON(&params)
	require.NoError(t, err)
	return f.space.Update(f.ctx, types.ParamStoreKeyFeesParams, proposal)
}

func (f repairM627Fixture) tx(t *testing.T, fees sdk.Coins) sdk.Tx {
	t.Helper()
	config := authtx.NewTxConfig(f.cdc, authtx.DefaultSignModes)
	builder := config.NewTxBuilder()
	builder.SetFeeAmount(fees)
	builder.SetGasLimit(10)
	require.NoError(t, builder.SetMsgs(banktypes.NewMsgSend(f.payer, f.payer, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1)))))
	wire, err := config.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	tx, err := config.TxDecoder()(wire)
	require.NoError(t, err)
	return tx
}

func (f repairM627Fixture) checkConsumers(t *testing.T, tx sdk.Tx, expected sdk.Coins) {
	t.Helper()
	for _, mode := range []struct {
		name     string
		check    bool
		simulate bool
	}{{"check", true, false}, {"deliver", false, false}, {"simulate", true, true}} {
		t.Run(mode.name, func(t *testing.T) {
			ctx := f.ctx.WithIsCheckTx(mode.check)
			var selected sdk.Coins
			var priority int64
			var err error
			require.NotPanics(t, func() {
				selected, priority, err = ante.CheckTxFeeWithValidatorMinGasPrices(ctx, tx, mode.simulate, f.keeper)
			})
			require.NoError(t, err)
			require.Equal(t, expected, selected)
			require.True(t, selected.IsValid())
			require.Equal(t, ante.GetTxPriority(expected, 10), priority)
		})
	}

	fees := tx.(sdk.FeeTx).GetFee()
	require.NoError(t, f.bank.AddCoins(f.ctx, f.payer, fees, true))
	decorator := ante.NewDeductFeeDecorator(f.accounts, f.bank, nil, f.keeper, nil)
	ctx, err := sdk.ChainAnteDecorators(decorator)(f.ctx.WithIsCheckTx(true), tx, false)
	require.NoError(t, err)
	require.Equal(t, ante.GetTxPriority(expected, 10), ctx.Priority())
	require.Equal(t, fees.Sub(expected), f.bank.GetAllBalances(f.ctx, f.payer))
	f.bank.WriteDeferredBalances(f.ctx)
	collector := f.accounts.GetModuleAddress(authtypes.FeeCollectorName)
	require.Equal(t, expected, f.bank.GetAllBalances(f.ctx, collector))
}

func TestRepairM_627_RejectInvalidProposals(t *testing.T) {
	for _, tc := range []struct {
		name      string
		denoms    []string
		errorText string
	}{
		{"empty", []string{""}, "invalid allowed fee denom"},
		{"short", []string{"ab"}, "invalid allowed fee denom"},
		{"numeric_prefix", []string{"1coin"}, "invalid allowed fee denom"},
		{"whitespace", []string{"u other"}, "invalid allowed fee denom"},
		{"punctuation", []string{"uother!"}, "invalid allowed fee denom"},
		{"long", []string{strings.Repeat("u", 129)}, "invalid allowed fee denom"},
		{"malformed_after_valid", []string{"uother", "?"}, "invalid allowed fee denom"},
		{"duplicate", []string{"uother", "uother"}, "duplicate allowed fee denom"},
		{"nonadjacent_duplicate", []string{"uother", "uzzz", "uother"}, "duplicate allowed fee denom"},
		{"bond", []string{sdk.DefaultBondDenom}, "duplicate allowed fee denom"},
		{"bond_after_valid", []string{"uother", sdk.DefaultBondDenom}, "duplicate allowed fee denom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRepairM627Fixture(t)
			baseline := *types.DefaultFeesParams()
			baseline.AllowedFeeDenoms = []string{"uother"}
			require.NoError(t, f.update(t, baseline))
			before := append([]byte(nil), f.space.GetRaw(f.ctx, types.ParamStoreKeyFeesParams)...)
			proposed := *types.DefaultFeesParams()
			proposed.AllowedFeeDenoms = tc.denoms
			require.ErrorContains(t, proposed.Validate(), tc.errorText)
			require.ErrorContains(t, types.NewGenesisState(proposed, *types.DefaultCosmosGasParams()).Validate(), tc.errorText)
			require.ErrorContains(t, f.update(t, proposed), tc.errorText)
			require.Panics(t, func() { f.keeper.SetFeesParams(f.ctx, proposed) })
			require.Equal(t, before, f.space.GetRaw(f.ctx, types.ParamStoreKeyFeesParams))
			require.Equal(t, baseline, f.keeper.GetFeesParams(f.ctx))
			fees := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 100), sdk.NewInt64Coin("uother", 20), sdk.NewInt64Coin("uzzz", 30))
			f.checkConsumers(t, f.tx(t, fees), sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 100), sdk.NewInt64Coin("uother", 20)))
		})
	}
}

func TestRepairM_627_ValidProposalsReachAnte(t *testing.T) {
	for _, tc := range []struct {
		name   string
		denoms []string
	}{
		{"nil_allowlist", nil},
		{"empty_allowlist", []string{}},
		{"one_denom", []string{"uother"}},
		{"distinct_denoms", []string{"uother", "uzzz"}},
		{"valid_syntax", []string{"ibc/ABC123", "factory/creator/token", "ABC", strings.Repeat("u", 128)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRepairM627Fixture(t)
			old := *types.DefaultFeesParams()
			if len(tc.denoms) > 0 {
				old.AllowedFeeDenoms = []string{"uold"}
			}
			require.NoError(t, f.update(t, old))
			params := *types.DefaultFeesParams()
			params.AllowedFeeDenoms = tc.denoms
			require.NoError(t, params.Validate())
			require.NoError(t, f.update(t, params))
			stored := f.keeper.GetFeesParams(f.ctx)
			require.Equal(t, params.GlobalMinimumGasPrices, stored.GlobalMinimumGasPrices)
			require.EqualValues(t, len(tc.denoms), len(stored.AllowedFeeDenoms))
			for i, denom := range tc.denoms {
				require.Equal(t, denom, stored.AllowedFeeDenoms[i])
			}
			fees := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 100), sdk.NewInt64Coin("uold", 40), sdk.NewInt64Coin("uother", 20), sdk.NewInt64Coin("uzzz", 30))
			expected := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 100))
			for _, denom := range tc.denoms {
				if amount := fees.AmountOf(denom); amount.IsPositive() {
					expected = expected.Add(sdk.NewCoin(denom, amount))
				}
			}
			f.checkConsumers(t, f.tx(t, fees), expected)
		})
	}
}

func TestRepairM_627_MinimumGasPriceValidationPreserved(t *testing.T) {
	f := newRepairM627Fixture(t)
	before := append([]byte(nil), f.space.GetRaw(f.ctx, types.ParamStoreKeyFeesParams)...)
	params := types.FeesParams{
		GlobalMinimumGasPrices: sdk.DecCoins{{Denom: sdk.DefaultBondDenom, Amount: sdk.NewDec(-1)}},
		AllowedFeeDenoms:       []string{"uother"},
	}
	require.Error(t, params.Validate())
	require.Error(t, f.update(t, params))
	require.Equal(t, before, f.space.GetRaw(f.ctx, types.ParamStoreKeyFeesParams))
}
