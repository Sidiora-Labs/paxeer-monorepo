package keeper_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repair302Fixture struct {
	bank   keeper.BaseKeeper
	auth   authkeeper.AccountKeeper
	ctx    sdk.Context
	store  storetypes.CommitMultiStore
	cdc    *codec.ProtoCodec
	closed bool
}

func repair302Open(t *testing.T, dir string) *repair302Fixture {
	t.Helper()
	db, err := dbm.NewDB("bank", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	cms := store.NewCommitMultiStore(db)
	bankKey := sdk.NewKVStoreKey(types.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	for _, key := range []*sdk.KVStoreKey{bankKey, authKey, paramsKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	bankParams := paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, types.ModuleName)
	authParams := paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, authtypes.ModuleName)
	ak := authkeeper.NewAccountKeeper(cdc, authKey, authParams, authtypes.ProtoBaseAccount, map[string][]string{})
	f := &repair302Fixture{
		bank:  keeper.NewBaseKeeper(cdc, bankKey, ak, bankParams, map[string]bool{}),
		auth:  ak,
		ctx:   sdk.NewContext(cms, tmproto.Header{Height: 1, Time: time.Unix(1, 0)}, false),
		store: cms, cdc: cdc,
	}
	t.Cleanup(func() { f.close(t) })
	return f
}

func (f *repair302Fixture) close(t *testing.T) {
	t.Helper()
	if !f.closed {
		require.NoError(t, f.store.Close())
		f.closed = true
	}
}

func repair302Addresses() (sdk.AccAddress, sdk.AccAddress, sdk.AccAddress) {
	return sdk.AccAddress(bytes.Repeat([]byte{1}, 20)), sdk.AccAddress(bytes.Repeat([]byte{2}, 20)), sdk.AccAddress(bytes.Repeat([]byte{3}, 20))
}

func repair302Seed(t *testing.T, f *repair302Fixture) (string, string, string) {
	t.Helper()
	a, b, denied := repair302Addresses()
	restricted := "factory/" + a.String() + "/restricted"
	empty := "factory/" + a.String() + "/empty"
	absent := "factory/" + a.String() + "/absent"
	gs := types.DefaultGenesisState()
	f.auth.SetParams(f.ctx, authtypes.DefaultParams())
	for _, address := range []sdk.AccAddress{a, b, denied} {
		f.auth.SetAccount(f.ctx, f.auth.NewAccountWithAddress(f.ctx, address))
		gs.Balances = append(gs.Balances, types.Balance{Address: address.String(), Coins: sdk.NewCoins(
			sdk.NewInt64Coin(restricted, 100), sdk.NewInt64Coin(empty, 100), sdk.NewInt64Coin(absent, 100), sdk.NewInt64Coin("ordinary", 100),
		)})
	}
	f.bank.InitGenesis(f.ctx, gs)
	f.bank.SetDenomAllowList(f.ctx, restricted, types.AllowList{Addresses: []string{b.String(), a.String()}})
	f.bank.SetDenomAllowList(f.ctx, empty, types.AllowList{})
	// A policy with no balance/supply/metadata must also survive export.
	f.bank.SetDenomAllowList(f.ctx, "factory/"+a.String()+"/unused", types.AllowList{Addresses: []string{a.String()}})
	return restricted, empty, absent
}

func repair302Transfers(t *testing.T, f *repair302Fixture, restricted, empty, absent string) {
	t.Helper()
	a, b, denied := repair302Addresses()
	for _, tc := range []struct {
		name     string
		from, to sdk.AccAddress
		denom    string
		allowed  bool
	}{
		{"allowed", a, b, restricted, true},
		{"denied_sender", denied, a, restricted, false},
		{"denied_recipient", a, denied, restricted, false},
		{"empty_list", a, denied, empty, true},
		{"absent_list", denied, a, absent, true},
		{"ordinary_denom", denied, a, "ordinary", true},
	} {
		for _, multi := range []bool{false, true} {
			method := "/send"
			if multi {
				method = "/multisend"
			}
			t.Run(tc.name+method, func(t *testing.T) {
				ctx, _ := f.ctx.CacheContext()
				server := keeper.NewMsgServerImpl(f.bank)
				fromBefore := f.bank.GetAllBalances(ctx, tc.from)
				toBefore := f.bank.GetAllBalances(ctx, tc.to)
				amount := sdk.NewCoins(sdk.NewInt64Coin(tc.denom, 1))
				var err error
				if multi {
					_, err = server.MultiSend(sdk.WrapSDKContext(ctx), &types.MsgMultiSend{
						Inputs:  []types.Input{{Address: tc.from.String(), Coins: amount}},
						Outputs: []types.Output{{Address: tc.to.String(), Coins: amount}},
					})
				} else {
					_, err = server.Send(sdk.WrapSDKContext(ctx), &types.MsgSend{FromAddress: tc.from.String(), ToAddress: tc.to.String(), Amount: amount})
				}
				if tc.allowed {
					require.NoError(t, err)
					require.Equal(t, fromBefore.Sub(amount), f.bank.GetAllBalances(ctx, tc.from))
					require.Equal(t, toBefore.Add(amount...), f.bank.GetAllBalances(ctx, tc.to))
				} else {
					require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
					require.Equal(t, fromBefore, f.bank.GetAllBalances(ctx, tc.from))
					require.Equal(t, toBefore, f.bank.GetAllBalances(ctx, tc.to))
				}
			})
		}
	}
}

func TestRepair_302_AllowlistGenesisRecovery(t *testing.T) {
	dir := t.TempDir()
	source := repair302Open(t, dir)
	restricted, empty, absent := repair302Seed(t, source)
	repair302Transfers(t, source, restricted, empty, absent)
	expected := source.bank.ExportGenesisWithAllowLists(source.ctx)
	require.Equal(t, keeper.DenomAllowListsVersion, expected.DenomAllowListsVersion)
	require.Len(t, expected.DenomAllowLists, 3)
	encoded, err := keeper.MarshalGenesisJSON(source.cdc, expected)
	require.NoError(t, err)
	transport := filepath.Join(t.TempDir(), "bank-genesis.json")
	require.NoError(t, os.WriteFile(transport, encoded, 0600))
	require.EqualValues(t, 1, source.store.Commit(true).Version)
	source.close(t)

	// Reopen the original component to prove the producer's durable KV policy.
	reopened := repair302Open(t, dir)
	require.Equal(t, expected, reopened.bank.ExportGenesisWithAllowLists(reopened.ctx))
	repair302Transfers(t, reopened, restricted, empty, absent)
	reopened.close(t)

	// Recover through JSON into a different database, with no producer KV state.
	freshDir := t.TempDir()
	fresh := repair302Open(t, freshDir)
	require.Empty(t, fresh.bank.GetDenomAllowList(fresh.ctx, restricted).Addresses)
	data, err := os.ReadFile(transport)
	require.NoError(t, err)
	recovered, err := keeper.UnmarshalGenesisJSON(fresh.cdc, data)
	require.NoError(t, err)
	a, b, denied := repair302Addresses()
	for _, address := range []sdk.AccAddress{a, b, denied} {
		fresh.auth.SetAccount(fresh.ctx, fresh.auth.NewAccountWithAddress(fresh.ctx, address))
	}
	fresh.bank.InitGenesisWithAllowLists(fresh.ctx, recovered)
	require.Equal(t, expected, fresh.bank.ExportGenesisWithAllowLists(fresh.ctx))
	for _, entry := range expected.DenomAllowLists {
		require.Equal(t, entry.AllowList, fresh.bank.GetDenomAllowList(fresh.ctx, entry.Denom))
	}
	repair302Transfers(t, fresh, restricted, empty, absent)
	fresh.store.Commit(true)
	fresh.close(t)
	restarted := repair302Open(t, freshDir)
	require.Equal(t, expected, restarted.bank.ExportGenesisWithAllowLists(restarted.ctx))
	repair302Transfers(t, restarted, restricted, empty, absent)
}

func TestRepair_302_RejectInvalidPoliciesBeforeMutation(t *testing.T) {
	f := repair302Open(t, t.TempDir())
	repair302Seed(t, f)
	before := f.bank.ExportGenesisWithAllowLists(f.ctx)
	encoded, err := keeper.MarshalGenesisJSON(f.cdc, before)
	require.NoError(t, err)
	for _, tc := range []struct {
		name    string
		change  func(*keeper.GenesisState)
		message string
	}{
		{"unknown_version", func(gs *keeper.GenesisState) { gs.DenomAllowListsVersion = 2 }, "unsupported denom allowlists version"},
		{"unversioned_policy", func(gs *keeper.GenesisState) { gs.DenomAllowListsVersion = 0 }, "unsupported denom allowlists version"},
		{"duplicate_denom", func(gs *keeper.GenesisState) { gs.DenomAllowLists = append(gs.DenomAllowLists, gs.DenomAllowLists[0]) }, "duplicate allowlist denom"},
		{"invalid_denom", func(gs *keeper.GenesisState) { gs.DenomAllowLists[0].Denom = "x" }, "invalid allowlist denom"},
		{"invalid_address", func(gs *keeper.GenesisState) { gs.DenomAllowLists[0].AllowList.Addresses = []string{"not-an-address"} }, "invalid allowlist address"},
		{"duplicate_address", func(gs *keeper.GenesisState) {
			a, _, _ := repair302Addresses()
			gs.DenomAllowLists[0].AllowList.Addresses = []string{a.String(), a.String()}
		}, "duplicate allowlist address"},
		{"noncanonical_address", func(gs *keeper.GenesisState) {
			a, _, _ := repair302Addresses()
			gs.DenomAllowLists[0].AllowList.Addresses = []string{strings.ToUpper(a.String())}
		}, "non-canonical allowlist address"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gs, err := keeper.UnmarshalGenesisJSON(f.cdc, encoded)
			require.NoError(t, err)
			tc.change(gs)
			gs.Bank.Params.DefaultSendEnabled = false
			require.ErrorContains(t, gs.Validate(), tc.message)
			require.PanicsWithError(t, gs.Validate().Error(), func() { f.bank.InitGenesisWithAllowLists(f.ctx, gs) })
			_, err = keeper.MarshalGenesisJSON(f.cdc, gs)
			require.ErrorContains(t, err, tc.message)
			require.Equal(t, before, f.bank.ExportGenesisWithAllowLists(f.ctx))
		})
	}
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &fields))
	fields["denom_allow_lists_version"] = json.RawMessage("9")
	invalid, err := json.Marshal(fields)
	require.NoError(t, err)
	_, err = keeper.UnmarshalGenesisJSON(f.cdc, invalid)
	require.ErrorContains(t, err, "unsupported denom allowlists version")
	require.Equal(t, before, f.bank.ExportGenesisWithAllowLists(f.ctx))
}

func TestRepair_302_LegacyCompatibilityAndLossPrevention(t *testing.T) {
	f := repair302Open(t, t.TempDir())
	f.bank.InitGenesis(f.ctx, types.DefaultGenesisState())
	legacy := f.bank.ExportGenesis(f.ctx)
	data := f.cdc.MustMarshalJSON(legacy)
	recovered, err := keeper.UnmarshalGenesisJSON(f.cdc, data)
	require.NoError(t, err)
	require.Zero(t, recovered.DenomAllowListsVersion)
	require.Empty(t, recovered.DenomAllowLists)
	fresh := repair302Open(t, t.TempDir())
	fresh.bank.InitGenesisWithAllowLists(fresh.ctx, recovered)
	require.Equal(t, legacy, fresh.bank.ExportGenesis(fresh.ctx))
	a, _, _ := repair302Addresses()
	// Even an explicitly empty policy is never silently dropped by the old API.
	f.bank.SetDenomAllowList(f.ctx, "factory/"+a.String()+"/empty", types.AllowList{})
	require.PanicsWithValue(t, "bank genesis contains denom allowlists: use ExportGenesisWithAllowLists and InitGenesisWithAllowLists", func() { f.bank.ExportGenesis(f.ctx) })
	complete := f.bank.ExportGenesisWithAllowLists(f.ctx)
	data, err = keeper.MarshalGenesisJSON(f.cdc, complete)
	require.NoError(t, err)
	// The unwired legacy module decoder must fail, rather than erase policy.
	var old types.GenesisState
	require.Error(t, f.cdc.UnmarshalAsJSON(data, &old))
	_, err = keeper.UnmarshalGenesisJSON(f.cdc, []byte(`{"denom_allow_lists_version":1,"denom_allow_lists":[{"denom":"valid","allow_list":{},"typo":true}]}`))
	require.ErrorContains(t, err, "unknown field")
}
