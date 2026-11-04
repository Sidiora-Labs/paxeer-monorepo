package staking_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/api"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repairM690Store struct {
	ctx      sdk.Context
	ms       *rootmulti.Store
	cdc      *codec.ProtoCodec
	registry codectypes.InterfaceRegistry
	staking  keeper.Keeper
	bank     bankkeeper.BaseKeeper
	stop     func()
}

func repairM690Open(t *testing.T, dir string) repairM690Store {
	t.Helper()
	db, err := dbm.NewGoLevelDB("repair-m690", dir)
	require.NoError(t, err)
	ms := rootmulti.NewStore(db)
	keys := sdk.NewKVStoreKeys("auth", "bank", "staking", "params")
	for _, key := range keys {
		ms.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	}
	transient := sdk.NewTransientStoreKey("transient_params")
	ms.MountStoreWithDB(transient, storetypes.StoreTypeTransient, nil)
	deferred := storetypes.NewMemoryStoreKey("deferred_bank")
	ms.MountStoreWithDB(deferred, storetypes.StoreTypeMemory, nil)
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, tmproto.Header{Height: 10, Time: time.Unix(1700000000, 0).UTC()}, false)
	registry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	pk := paramskeeper.NewKeeper(cdc, codec.NewLegacyAmino(), keys["params"], transient)
	ak := authkeeper.NewAccountKeeper(cdc, keys["auth"], pk.Subspace("auth"), authtypes.ProtoBaseAccount,
		map[string][]string{"mint": {authtypes.Minter}, types.BondedPoolName: {authtypes.Staking, authtypes.Burner}, types.NotBondedPoolName: {authtypes.Staking, authtypes.Burner}})
	bk := bankkeeper.NewBaseKeeperWithDeferredCache(cdc, keys["bank"], ak, pk.Subspace("bank"), map[string]bool{}, deferred)
	sk := keeper.NewKeeper(cdc, keys["staking"], ak, bk, pk.Subspace("staking"))
	if ms.LastCommitID().Version == 0 {
		ak.SetParams(ctx, authtypes.DefaultParams())
		bk.SetParams(ctx, banktypes.DefaultParams())
		sk.SetParams(ctx, types.DefaultParams())
		for _, name := range []string{"mint", types.BondedPoolName, types.NotBondedPoolName} {
			require.NotNil(t, ak.GetModuleAccount(ctx, name))
		}
	}
	closed := false
	stop := func() {
		if !closed {
			require.NoError(t, ms.Close())
			closed = true
		}
	}
	t.Cleanup(stop)
	return repairM690Store{ctx, ms, cdc, registry, sk, bk, stop}
}

func repairM690Address(b byte) sdk.AccAddress { return sdk.AccAddress(bytes.Repeat([]byte{b}, 20)) }

func (f repairM690Store) fund(t *testing.T, addr sdk.AccAddress, amount sdk.Int) {
	t.Helper()
	coins := sdk.NewCoins(sdk.NewCoin(f.staking.BondDenom(f.ctx), amount))
	require.NoError(t, f.bank.MintCoins(f.ctx, "mint", coins))
	require.NoError(t, f.bank.SendCoinsFromModuleToAccount(f.ctx, "mint", addr, coins))
}

func (f repairM690Store) validator(t *testing.T, b byte, status types.BondStatus) types.Validator {
	t.Helper()
	key := ed25519.GenPrivKeyFromSecret([]byte{b}).PubKey()
	v, err := types.NewValidator(sdk.ValAddress(repairM690Address(b)), key, types.Description{Moniker: fmt.Sprintf("validator-%d", b)})
	require.NoError(t, err)
	v.Status = status
	v.UnbondingHeight = f.ctx.BlockHeight() - 1
	v.UnbondingTime = f.ctx.BlockTime().Add(time.Hour)
	f.staking.SetValidator(f.ctx, v)
	require.NoError(t, f.staking.SetValidatorByConsAddr(f.ctx, v))
	f.staking.SetValidatorByPowerIndex(f.ctx, v)
	return v
}

func (f repairM690Store) delegate(t *testing.T, del sdk.AccAddress, val sdk.ValAddress, power int64) sdk.Int {
	t.Helper()
	amount := f.staking.TokensFromConsensusPower(f.ctx, power)
	f.fund(t, del, amount)
	msg := types.NewMsgDelegate(del, val, sdk.NewCoin(f.staking.BondDenom(f.ctx), amount))
	require.NoError(t, msg.ValidateBasic())
	var decoded types.MsgDelegate
	require.NoError(t, f.cdc.Unmarshal(f.cdc.MustMarshal(msg), &decoded))
	_, err := keeper.NewMsgServerImpl(f.staking).Delegate(sdk.WrapSDKContext(f.ctx), &decoded)
	require.NoError(t, err)
	return amount
}

func repairM690Doc(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("spec", path))
	require.NoError(t, err)
	return string(b)
}

func repairM690JSONBlock(t *testing.T, doc, label string) []byte {
	t.Helper()
	marker := "<!-- repair-m690:" + label + " -->"
	_, rest, ok := strings.Cut(doc, marker)
	require.True(t, ok, "missing documented fixture %s", label)
	_, rest, ok = strings.Cut(rest, "```json\n")
	require.True(t, ok)
	block, _, ok := strings.Cut(rest, "```")
	require.True(t, ok)
	require.True(t, json.Valid([]byte(block)))
	return []byte(block)
}

// The node is closed and re-created over the committed database, rather than
// retaining a keeper or an Any cache across the export/restart boundary.
func TestRepairM_690_ExportErrorRecovery(t *testing.T) {
	for _, kind := range []string{"unsupported", "empty_any"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			open := func(initialize bool) (*paxapp.App, sdk.Context, func()) {
				db, err := dbm.NewGoLevelDB("export", dir)
				require.NoError(t, err)
				encoding := paxapp.MakeEncodingConfig()
				app := paxapp.New(db, nil, true, map[int64]bool{}, filepath.Join(dir, "node"), 1, false,
					config.TestConfig(), encoding, wasm.EnableAllProposals, paxapp.TestAppOpts{}, paxapp.EmptyWasmOpts, nil)
				closed := false
				stop := func() {
					if !closed {
						require.NoError(t, app.Close())
						closed = true
					}
				}
				t.Cleanup(stop)
				header := tmproto.Header{ChainID: "repair-m690", Height: 1, Time: time.Unix(1700000000, 0).UTC()}
				if initialize {
					genesis, err := json.Marshal(paxapp.NewDefaultGenesisState(encoding.Marshaler))
					require.NoError(t, err)
					_, err = app.InitChain(context.Background(), &abci.RequestInitChain{Time: header.Time, ChainId: header.ChainID,
						ConsensusParams: paxapp.DefaultConsensusParams, AppStateBytes: genesis})
					require.NoError(t, err)
					return app, app.NewContext(false, header), stop
				}
				ctx := sdk.NewContext(app.CommitMultiStore().CacheMultiStore(), header, false)
				return app, ctx, stop
			}
			app, ctx, stop := open(true)
			goodKey := ed25519.GenPrivKeyFromSecret([]byte("repair-m690-good")).PubKey()
			good, err := types.NewValidator(sdk.ValAddress(repairM690Address(1)), goodKey, types.Description{Moniker: "first"})
			require.NoError(t, err)
			good.Status = types.Bonded
			good.Tokens = app.StakingKeeper.TokensFromConsensusPower(ctx, 3)
			good.DelegatorShares = good.Tokens.ToDec()
			app.StakingKeeper.SetValidator(ctx, good)
			app.StakingKeeper.SetLastValidatorPower(ctx, good.GetOperator(), 3)
			vals, err := staking.WriteValidators(ctx, app.StakingKeeper)
			require.NoError(t, err)
			require.Len(t, vals, 1)
			bad, err := types.NewValidator(sdk.ValAddress(repairM690Address(2)), secp256k1.GenPrivKey().PubKey(), types.Description{Moniker: "unsupported"})
			require.NoError(t, err)
			bad.Status, bad.Tokens, bad.DelegatorShares = good.Status, good.Tokens, good.DelegatorShares
			if kind == "empty_any" {
				bad.ConsensusPubkey = &codectypes.Any{}
			}
			app.StakingKeeper.SetValidator(ctx, bad)
			app.StakingKeeper.SetLastValidatorPower(ctx, bad.GetOperator(), 3)
			app.StakingKeeper.SetLastTotalPower(ctx, sdk.NewInt(6))
			// Use the node commit path so its pre-commit hook persists the
			// block identity alongside genesis and the malformed validator.
			require.NoError(t, app.SetSealedStateToCommit(ctx, ctx.MultiStore().(sdk.CacheMultiStore)))
			_, err = app.Commit(context.Background())
			require.NoError(t, err)
			require.Equal(t, int64(1), app.LastBlockHeight())
			committed := app.CommitMultiStore().LastCommitID()
			stop()
			app, ctx, _ = open(false)
			require.Equal(t, committed, app.CommitMultiStore().LastCommitID())
			recovered, found := app.StakingKeeper.GetValidator(ctx, bad.GetOperator())
			require.True(t, found)
			require.Equal(t, bad.ConsensusPubkey.TypeUrl, recovered.ConsensusPubkey.TypeUrl)
			var order []string
			app.StakingKeeper.IterateLastValidators(ctx, func(_ int64, v types.ValidatorI) bool { order = append(order, v.GetOperator().String()); return false })
			require.Equal(t, []string{good.OperatorAddress, bad.OperatorAddress}, order)
			vals, err = staking.WriteValidators(ctx, app.StakingKeeper)
			if kind == "unsupported" {
				require.ErrorIs(t, err, sdkerrors.ErrNotSupported)
			} else {
				require.ErrorIs(t, err, sdkerrors.ErrInvalidType)
			}
			require.Nil(t, vals, "discard the valid prefix on either conversion error")
			// The production node consumer must return no publishable genesis artifact.
			if kind == "unsupported" {
				exported, exportErr := app.ExportAppStateAndValidators(false, nil)
				require.EqualError(t, exportErr, err.Error())
				require.Empty(t, exported.AppState)
				require.Nil(t, exported.Validators)
				require.Zero(t, exported.Height)
				file, fileErr := os.CreateTemp(t.TempDir(), "export-*.json")
				require.NoError(t, fileErr)
				defer file.Close()
				streamed, streamErr := app.ExportAppToFileStateAndValidators(false, nil, file)
				require.ErrorIs(t, streamErr, sdkerrors.ErrNotSupported)
				require.Nil(t, streamed.Validators)
				require.Zero(t, streamed.Height)
			}
			// A repaired stored key recovers the whole validator set, with real powers.
			recovered.ConsensusPubkey, err = codectypes.NewAnyWithValue(ed25519.GenPrivKeyFromSecret([]byte("repair-m690-repaired")).PubKey())
			require.NoError(t, err)
			app.StakingKeeper.SetValidator(ctx, recovered)
			vals, err = staking.WriteValidators(ctx, app.StakingKeeper)
			require.NoError(t, err)
			require.Len(t, vals, 2)
			require.Equal(t, int64(3), vals[0].Power)
			require.Equal(t, int64(3), vals[1].Power)
			require.Equal(t, "first", vals[0].Name)
			require.NotEqual(t, vals[0].Address, vals[1].Address)
		})
	}
}

func TestRepairM_690_StateLayoutAndHistory(t *testing.T) {
	f := repairM690Open(t, t.TempDir())
	del := repairM690Address(90)
	vals := []types.Validator{f.validator(t, 11, types.Unbonded), f.validator(t, 12, types.Unbonded), f.validator(t, 13, types.Unbonded)}
	for i, v := range vals {
		f.delegate(t, del, v.GetOperator(), []int64{3, 5, 5}[i])
	}
	staking.EndBlocker(f.ctx, f.staking)
	before, found := f.staking.GetDelegation(f.ctx, del, vals[0].GetOperator())
	require.True(t, found)
	f.ctx = f.ctx.WithBlockHeight(11)
	staking.BeginBlocker(f.ctx, f.staking)
	hist, found := f.staking.GetHistoricalInfo(f.ctx, 11)
	require.True(t, found)
	require.Len(t, hist.Valset, 3)
	require.NoError(t, types.ValidateBasic(hist, f.staking.PowerReduction(f.ctx)))
	require.Equal(t, int64(3), hist.Valset[2].GetConsensusPower(f.staking.PowerReduction(f.ctx)))
	a, err := hist.Valset[0].GetConsAddr()
	require.NoError(t, err)
	b, err := hist.Valset[1].GetConsAddr()
	require.NoError(t, err)
	require.Negative(t, bytes.Compare(a, b), "equal-power historical ties use consensus address")
	require.True(t, f.ctx.KVStore(f.staking.GetStoreKey()).Has(append([]byte{0x50}, []byte("11")...)))
	cons, err := vals[0].GetConsAddr()
	require.NoError(t, err)
	f.staking.Jail(f.ctx, cons)
	staking.EndBlocker(f.ctx, f.staking)
	leaving, found := f.staking.GetValidator(f.ctx, vals[0].GetOperator())
	require.True(t, found)
	require.Equal(t, types.Unbonding, leaving.Status)
	after, found := f.staking.GetDelegation(f.ctx, del, vals[0].GetOperator())
	require.True(t, found)
	require.Equal(t, before, after)
	_, found = f.staking.GetUnbondingDelegation(f.ctx, del, vals[0].GetOperator())
	require.False(t, found)
	timeBytes := sdk.FormatTimeBytes(leaving.UnbondingTime)
	expected := append([]byte{0x43}, sdk.Uint64ToBigEndian(uint64(len(timeBytes)))...)
	expected = append(expected, timeBytes...)
	expected = append(expected, sdk.Uint64ToBigEndian(uint64(leaving.UnbondingHeight))...)
	require.Equal(t, expected, types.GetValidatorQueueKey(leaving.UnbondingTime, leaving.UnbondingHeight))
	require.Equal(t, uint64(29), binary.BigEndian.Uint64(expected[1:9]))
	raw := f.ctx.KVStore(f.staking.GetStoreKey()).Get(expected)
	var queued types.ValAddresses
	require.NoError(t, f.cdc.Unmarshal(raw, &queued))
	require.Equal(t, []string{leaving.OperatorAddress}, queued.Addresses)
	when, height, err := types.ParseValidatorQueueKey(expected)
	require.NoError(t, err)
	require.Equal(t, leaving.UnbondingTime, when)
	require.Equal(t, leaving.UnbondingHeight, height)
	f.ctx = f.ctx.WithBlockHeight(12).WithBlockTime(leaving.UnbondingTime)
	staking.EndBlocker(f.ctx, f.staking)
	left, found := f.staking.GetValidator(f.ctx, vals[0].GetOperator())
	require.True(t, found)
	require.Equal(t, types.Unbonded, left.Status)
	after, found = f.staking.GetDelegation(f.ctx, del, vals[0].GetOperator())
	require.True(t, found)
	require.Equal(t, before, after)
	require.False(t, f.ctx.KVStore(f.staking.GetStoreKey()).Has(expected))
	doc := repairM690Doc(t, "01_state.md")
	for _, claim := range []string{"BE64(len(timeBytes)) | timeBytes | BE64(UnbondingHeight)", "ASCII(decimal block height)", "consensus voting power **descending**", "consensus address **ascending**", "Delegation shares remain recorded", "02_state_transitions.md#begin-unbonding"} {
		require.Contains(t, doc, claim)
	}
}

func TestRepairM_690_MessageWaitingPeriod(t *testing.T) {
	for _, status := range []types.BondStatus{types.Bonded, types.Unbonding, types.Unbonded} {
		t.Run(status.String(), func(t *testing.T) {
			f := repairM690Open(t, t.TempDir())
			del := repairM690Address(90)
			v := f.validator(t, 21, status)
			amount := f.delegate(t, del, v.GetOperator(), 10).QuoRaw(2)
			now := f.ctx.BlockTime()
			want := now.Add(f.staking.UnbondingTime(f.ctx))
			msg := types.NewMsgUndelegate(del, v.GetOperator(), sdk.NewCoin(f.staking.BondDenom(f.ctx), amount))
			require.NoError(t, msg.ValidateBasic())
			var decoded types.MsgUndelegate
			require.NoError(t, f.cdc.Unmarshal(f.cdc.MustMarshal(msg), &decoded))
			response, err := keeper.NewMsgServerImpl(f.staking).Undelegate(sdk.WrapSDKContext(f.ctx), &decoded)
			require.NoError(t, err)
			var transported types.MsgUndelegateResponse
			require.NoError(t, f.cdc.Unmarshal(f.cdc.MustMarshal(response), &transported))
			require.Equal(t, want, transported.CompletionTime)
			require.NotEqual(t, v.UnbondingTime, transported.CompletionTime)
			router := baseapp.NewQueryServerTestHelper(f.ctx, f.registry)
			types.RegisterQueryServer(router, keeper.Querier{Keeper: f.staking})
			result, err := types.NewQueryClient(router).UnbondingDelegation(context.Background(), &types.QueryUnbondingDelegationRequest{DelegatorAddr: del.String(), ValidatorAddr: v.OperatorAddress})
			require.NoError(t, err)
			require.Len(t, result.Unbond.Entries, 1)
			require.Equal(t, want, result.Unbond.Entries[0].CompletionTime)
			require.Equal(t, amount, result.Unbond.Entries[0].Balance)
			pair := types.DVPair{DelegatorAddress: del.String(), ValidatorAddress: v.OperatorAddress}
			require.Equal(t, []types.DVPair{pair}, f.staking.GetUBDQueueTimeSlice(f.ctx, want))
			denom := f.staking.BondDenom(f.ctx)
			balance := f.bank.GetBalance(f.ctx, del, denom)
			require.True(t, balance.IsZero(), "request must never pay immediately")
			for _, early := range []time.Time{now, v.UnbondingTime, want.Add(-time.Nanosecond)} {
				_, err := f.staking.CompleteUnbonding(f.ctx.WithBlockTime(early), del, v.GetOperator())
				require.NoError(t, err)
				require.Equal(t, balance, f.bank.GetBalance(f.ctx, del, denom))
				ubd, found := f.staking.GetUnbondingDelegation(f.ctx, del, v.GetOperator())
				require.True(t, found)
				require.Len(t, ubd.Entries, 1)
			}
			paid, err := f.staking.CompleteUnbonding(f.ctx.WithBlockTime(want), del, v.GetOperator())
			require.NoError(t, err)
			require.Equal(t, sdk.NewCoins(sdk.NewCoin(denom, amount)), paid)
			require.Equal(t, balance.Add(sdk.NewCoin(denom, amount)), f.bank.GetBalance(f.ctx, del, denom))
			_, found := f.staking.GetUnbondingDelegation(f.ctx, del, v.GetOperator())
			require.False(t, found)
		})
	}
	doc := repairM690Doc(t, "03_messages.md")
	for _, status := range []string{"Bonded", "Unbonding", "Unbonded"} {
		require.Contains(t, doc, "| "+status+" | Current block time plus `UnbondingTime` |")
	}
	require.Contains(t, doc, "sdkerrors.ErrInvalidRecipient")
	require.Contains(t, doc, "operator's own delegation")
}

func TestRepairM_690_TransitionsAndDurableRetry(t *testing.T) {
	for _, status := range []types.BondStatus{types.Bonded, types.Unbonding, types.Unbonded} {
		t.Run("redelegation_"+status.String(), func(t *testing.T) {
			f := repairM690Open(t, t.TempDir())
			src := f.validator(t, 31, status)
			dst := f.validator(t, 32, types.Unbonded)
			del := repairM690Address(90)
			amount := f.delegate(t, del, src.GetOperator(), 10).QuoRaw(2)
			msg := types.NewMsgBeginRedelegate(del, src.GetOperator(), dst.GetOperator(), sdk.NewCoin(f.staking.BondDenom(f.ctx), amount))
			response, err := keeper.NewMsgServerImpl(f.staking).BeginRedelegate(sdk.WrapSDKContext(f.ctx), msg)
			require.NoError(t, err)
			expected := f.ctx.BlockTime().Add(f.staking.UnbondingTime(f.ctx))
			if status == types.Unbonding {
				expected = src.UnbondingTime
			}
			if status == types.Unbonded {
				expected = time.Time{}
			}
			require.Equal(t, expected, response.CompletionTime)
			red, found := f.staking.GetRedelegation(f.ctx, del, src.GetOperator(), dst.GetOperator())
			require.Equal(t, status != types.Unbonded, found)
			if found {
				require.Len(t, red.Entries, 1)
				require.Equal(t, expected, red.Entries[0].CompletionTime)
			}
			dstDel, found := f.staking.GetDelegation(f.ctx, del, dst.GetOperator())
			require.True(t, found)
			require.Equal(t, amount.ToDec(), dstDel.Shares)
			require.True(t, f.bank.GetBalance(f.ctx, del, f.staking.BondDenom(f.ctx)).IsZero())
		})
	}
	t.Run("redelegation_removed_source", func(t *testing.T) {
		f := repairM690Open(t, t.TempDir())
		src := f.validator(t, 35, types.Unbonded)
		dst := f.validator(t, 36, types.Unbonded)
		del := repairM690Address(90)
		amount := f.delegate(t, del, src.GetOperator(), 10)
		response, err := keeper.NewMsgServerImpl(f.staking).BeginRedelegate(sdk.WrapSDKContext(f.ctx), types.NewMsgBeginRedelegate(del, src.GetOperator(), dst.GetOperator(), sdk.NewCoin(f.staking.BondDenom(f.ctx), amount)))
		require.NoError(t, err)
		require.Equal(t, f.ctx.BlockTime().Add(f.staking.UnbondingTime(f.ctx)), response.CompletionTime)
		_, found := f.staking.GetValidator(f.ctx, src.GetOperator())
		require.False(t, found)
		red, found := f.staking.GetRedelegation(f.ctx, del, src.GetOperator(), dst.GetOperator())
		require.True(t, found)
		require.Len(t, red.Entries, 1)
		require.Equal(t, f.ctx.BlockHeight(), red.Entries[0].CreationHeight)
		require.Equal(t, response.CompletionTime, red.Entries[0].CompletionTime)
	})
	dir := t.TempDir()
	f := repairM690Open(t, dir)
	v := f.validator(t, 41, types.Unbonded)
	del := repairM690Address(90)
	amount := f.delegate(t, del, v.GetOperator(), 10).QuoRaw(2)
	// Keep another validator's backing in the not-bonded pool: the retry
	// must preserve that stake as well as paying exactly the mature entry.
	reserve := f.validator(t, 42, types.Unbonded)
	reserveAmount := f.delegate(t, repairM690Address(91), reserve.GetOperator(), 1)
	reserveCons, err := reserve.GetConsAddr()
	require.NoError(t, err)
	f.staking.Jail(f.ctx, reserveCons)
	staking.EndBlocker(f.ctx, f.staking)
	response, err := keeper.NewMsgServerImpl(f.staking).Undelegate(sdk.WrapSDKContext(f.ctx), types.NewMsgUndelegate(del, v.GetOperator(), sdk.NewCoin(f.staking.BondDenom(f.ctx), amount)))
	require.NoError(t, err)
	maturity := response.CompletionTime
	f.ctx = f.ctx.WithBlockTime(maturity.Add(time.Second)).WithBlockHeight(11)
	denom := f.staking.BondDenom(f.ctx)
	pool := f.staking.GetNotBondedPool(f.ctx).GetAddress()
	poolBefore := f.bank.GetBalance(f.ctx, pool, denom)
	before := f.bank.GetBalance(f.ctx, del, denom)
	supply := f.bank.GetSupply(f.ctx, denom)
	ubd, found := f.staking.GetUnbondingDelegation(f.ctx, del, v.GetOperator())
	require.True(t, found)
	f.bank.RegisterRecipientChecker(func(_ sdk.Context, addr sdk.AccAddress) bool { return !addr.Equals(del) })
	_, err = f.staking.CompleteUnbonding(f.ctx, del, v.GetOperator())
	require.ErrorIs(t, err, sdkerrors.ErrInvalidRecipient)
	staking.EndBlocker(f.ctx, f.staking)
	retryTime := f.ctx.BlockTime()
	pair := types.DVPair{DelegatorAddress: del.String(), ValidatorAddress: v.OperatorAddress}
	require.Empty(t, f.staking.GetUBDQueueTimeSlice(f.ctx, maturity))
	require.Equal(t, []types.DVPair{pair}, f.staking.GetUBDQueueTimeSlice(f.ctx, retryTime))
	recovered, found := f.staking.GetUnbondingDelegation(f.ctx, del, v.GetOperator())
	require.True(t, found)
	require.Equal(t, ubd, recovered)
	require.Equal(t, poolBefore, f.bank.GetBalance(f.ctx, pool, denom))
	require.Equal(t, before, f.bank.GetBalance(f.ctx, del, denom))
	committed := f.ms.Commit(true)
	f.stop()
	f = repairM690Open(t, dir)
	require.Equal(t, committed, f.ms.LastCommitID())
	recovered, found = f.staking.GetUnbondingDelegation(f.ctx, del, v.GetOperator())
	require.True(t, found)
	require.Equal(t, ubd, recovered)
	require.Equal(t, []types.DVPair{pair}, f.staking.GetUBDQueueTimeSlice(f.ctx, retryTime))
	f.ctx = f.ctx.WithBlockTime(retryTime.Add(time.Second)).WithBlockHeight(12)
	staking.EndBlocker(f.ctx, f.staking)
	payout := sdk.NewCoin(denom, amount)
	require.Equal(t, before.Add(payout), f.bank.GetBalance(f.ctx, del, denom))
	require.Equal(t, poolBefore.Sub(payout), f.bank.GetBalance(f.ctx, pool, denom))
	require.Equal(t, reserveAmount, f.bank.GetBalance(f.ctx, pool, denom).Amount)
	reserveAfter, found := f.staking.GetValidator(f.ctx, reserve.GetOperator())
	require.True(t, found)
	require.Equal(t, types.Unbonded, reserveAfter.Status)
	require.True(t, reserveAfter.Jailed)
	require.Equal(t, reserveAmount, reserveAfter.Tokens)
	require.Equal(t, supply, f.bank.GetSupply(f.ctx, denom))
	_, found = f.staking.GetUnbondingDelegation(f.ctx, del, v.GetOperator())
	require.False(t, found)
	require.False(t, f.ctx.KVStore(f.staking.GetStoreKey()).Has(types.GetUBDByValIndexKey(del, v.GetOperator())))
	iterator := sdk.KVStorePrefixIterator(f.ctx.KVStore(f.staking.GetStoreKey()), types.UnbondingQueueKey)
	require.False(t, iterator.Valid())
	require.NoError(t, iterator.Close())
	f.ctx = f.ctx.WithBlockTime(retryTime.Add(2 * time.Second)).WithBlockHeight(13)
	staking.EndBlocker(f.ctx, f.staking)
	require.Equal(t, before.Add(payout), f.bank.GetBalance(f.ctx, del, denom))
	doc := repairM690Doc(t, "02_state_transitions.md")
	for _, claim := range []string{"For **Bonded, Unbonding and Unbonded**", "getBeginInfo", "after a restart", "without a partial payment", "current block time"} {
		require.Contains(t, doc, claim)
	}
}

func TestRepairM_690_RegisteredParameters(t *testing.T) {
	f := repairM690Open(t, t.TempDir())
	params := f.staking.GetParams(f.ctx)
	require.Equal(t, types.DefaultParams(), params)
	pairs := params.ParamSetPairs()
	require.Len(t, pairs, 8)
	table := repairM690Doc(t, "08_params.md")
	expectedTypes := map[string]reflect.Type{
		"UnbondingTime": reflect.TypeOf(time.Duration(0)), "MaxValidators": reflect.TypeOf(uint32(0)), "MaxEntries": reflect.TypeOf(uint32(0)), "HistoricalEntries": reflect.TypeOf(uint32(0)),
		"BondDenom": reflect.TypeOf(""), "MinCommissionRate": reflect.TypeOf(sdk.Dec{}), "MaxVotingPower": reflect.TypeOf(sdk.Dec{}), "MaxVotingPowerEnforcementThreshold": reflect.TypeOf(sdk.Int{}),
	}
	for _, pair := range pairs {
		key := string(pair.Key)
		require.NotEqual(t, "PowerReduction", key)
		require.Equal(t, expectedTypes[key], reflect.TypeOf(pair.Value).Elem())
		require.NoError(t, pair.ValidatorFn(reflect.ValueOf(pair.Value).Elem().Interface()))
		require.Contains(t, table, "| `"+key+"` |")
		if expectedTypes[key] == reflect.TypeOf(uint32(0)) {
			require.Error(t, pair.ValidatorFn(uint16(1)))
			require.Regexp(t, regexp.MustCompile("(?m)^\\|[^\\n]*\\| `"+key+"` \\| `uint32` \\|"), table)
		}
	}
	require.Contains(t, table, params.MinCommissionRate.String())
	require.Contains(t, table, params.MaxVotingPowerRatio.String())
	require.Contains(t, table, "not\nregistered in `ParamSetPairs`")
	require.Equal(t, sdk.DefaultPowerReduction, f.staking.PowerReduction(f.ctx))
	// The source reference describes the application default, while this package's
	// older tests deliberately override that global to 10^18 in common_test.go.
	require.Contains(t, table, "1000000")
	// Drive governed values through store, generated query transport and codec.
	params.MaxValidators = 37
	params.MaxEntries = 9
	params.HistoricalEntries = 12
	params.MinCommissionRate = sdk.NewDecWithPrec(7, 2)
	params.MaxVotingPowerRatio = sdk.NewDecWithPrec(3, 1)
	params.MaxVotingPowerEnforcementThreshold = sdk.NewInt(12345)
	f.staking.SetParams(f.ctx, params)
	router := baseapp.NewQueryServerTestHelper(f.ctx, f.registry)
	types.RegisterQueryServer(router, keeper.Querier{Keeper: f.staking})
	result, err := types.NewQueryClient(router).Params(context.Background(), &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, params, result.Params)
}

func TestRepairM_690_ClientExamples(t *testing.T) {
	f := repairM690Open(t, t.TempDir())
	router := baseapp.NewQueryServerTestHelper(f.ctx, f.registry)
	types.RegisterQueryServer(router, keeper.Querier{Keeper: f.staking})
	grpcClient := types.NewQueryClient(router)
	mux := api.New(client.Context{InterfaceRegistry: f.registry}).GRPCGatewayRouter
	require.NoError(t, types.RegisterQueryHandlerClient(context.Background(), mux, grpcClient))
	server := httptest.NewServer(mux)
	defer server.Close()
	get := func(path string) []byte {
		response, err := http.Get(server.URL + path)
		require.NoError(t, err)
		defer response.Body.Close()
		var raw json.RawMessage
		require.NoError(t, json.NewDecoder(response.Body).Decode(&raw))
		require.Equal(t, http.StatusOK, response.StatusCode, string(raw))
		return raw
	}
	doc := repairM690Doc(t, "09_client.md")
	actual := get("/cosmos/staking/v1beta1/params")
	require.JSONEq(t, string(repairM690JSONBlock(t, doc, "params")), string(actual))
	var params types.QueryParamsResponse
	require.NoError(t, f.cdc.UnmarshalAsJSON(actual, &params))
	require.Equal(t, f.staking.GetParams(f.ctx), params.Params)
	var shape map[string]map[string]interface{}
	require.NoError(t, json.Unmarshal(actual, &shape))
	require.Len(t, shape["params"], 8)
	// CLI queries use the production JSON-RPC transport and protobuf router.
	rpcMux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(rpcMux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, request *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			handler := router.Route(request.Path)
			if handler == nil {
				return nil, fmt.Errorf("unregistered query %q", request.Path)
			}
			result, err := handler(f.ctx, abci.RequestQuery{Path: request.Path, Data: request.Data, Height: int64(request.Height), Prove: request.Prove})
			if err != nil {
				return nil, err
			}
			result.Height = f.ctx.BlockHeight()
			return &coretypes.ResultABCIQuery{Response: result}, nil
		}),
	})
	rpcHTTP := httptest.NewServer(rpcMux)
	defer rpcHTTP.Close()
	rpc, err := rpchttp.New(rpcHTTP.URL)
	require.NoError(t, err)
	clientCtx := client.Context{}.WithCodec(f.cdc).WithLegacyAmino(codec.NewLegacyAmino()).WithInterfaceRegistry(f.registry).WithClient(rpc).WithOutputFormat("json")
	out, err := clitestutil.ExecTestCLICmd(clientCtx, cli.GetCmdQueryParams(), []string{"--output=json"})
	require.NoError(t, err, out.String())
	t.Logf("staking params CLI capture: %s", out.String())
	require.JSONEq(t, string(repairM690JSONBlock(t, doc, "params-cli")), out.String())
	var cliParams types.Params
	require.NoError(t, f.cdc.UnmarshalAsJSON(out.Bytes(), &cliParams))
	require.Equal(t, params.Params, cliParams)
	// Produce the documented redelegation through the real message handler.
	del := repairM690Address(90)
	src := f.validator(t, 51, types.Unbonding)
	dst := f.validator(t, 52, types.Unbonded)
	initial := sdk.NewInt(10_000_000)
	f.fund(t, del, initial)
	_, err = keeper.NewMsgServerImpl(f.staking).Delegate(sdk.WrapSDKContext(f.ctx), types.NewMsgDelegate(del, src.GetOperator(), sdk.NewCoin(f.staking.BondDenom(f.ctx), initial)))
	require.NoError(t, err)
	amount := initial.QuoRaw(2)
	_, err = keeper.NewMsgServerImpl(f.staking).BeginRedelegate(sdk.WrapSDKContext(f.ctx), types.NewMsgBeginRedelegate(del, src.GetOperator(), dst.GetOperator(), sdk.NewCoin(f.staking.BondDenom(f.ctx), amount)))
	require.NoError(t, err)
	// A second real record proves that the destination filter actually constrains results.
	other := f.validator(t, 53, types.Unbonded)
	_, err = keeper.NewMsgServerImpl(f.staking).BeginRedelegate(sdk.WrapSDKContext(f.ctx), types.NewMsgBeginRedelegate(del, src.GetOperator(), other.GetOperator(), sdk.NewCoin(f.staking.BondDenom(f.ctx), amount.QuoRaw(2))))
	require.NoError(t, err)
	args := url.Values{"src_validator_addr": {src.OperatorAddress}, "dst_validator_addr": {dst.OperatorAddress}, "pagination.limit": {"1"}, "pagination.count_total": {"true"}}
	path := "/cosmos/staking/v1beta1/delegators/" + del.String() + "/redelegations?" + args.Encode()
	actual = get(path)
	require.JSONEq(t, string(repairM690JSONBlock(t, doc, "redelegations")), string(actual))
	var gatewayResponse types.QueryRedelegationsResponse
	require.NoError(t, f.cdc.UnmarshalAsJSON(actual, &gatewayResponse))
	// RedelegationResponse.MarshalJSON is used even inside protobuf JSON.
	// It preserves the producer's nil nested entries as null on both clients.
	var response types.QueryRedelegationsResponse
	require.NoError(t, f.cdc.UnmarshalAsJSON(f.cdc.MustMarshalJSON(&gatewayResponse), &response))
	require.Len(t, response.RedelegationResponses, 1)
	require.Equal(t, uint64(1), response.Pagination.Total)
	stored, found := f.staking.GetRedelegation(f.ctx, del, src.GetOperator(), dst.GetOperator())
	require.True(t, found)
	identity := stored
	identity.Entries = nil // The producer transports entries only in the sibling response field.
	require.Equal(t, identity, response.RedelegationResponses[0].Redelegation)
	require.Len(t, response.RedelegationResponses[0].Entries, 1)
	require.Equal(t, stored.Entries[0], response.RedelegationResponses[0].Entries[0].RedelegationEntry)
	require.Equal(t, amount, response.RedelegationResponses[0].Entries[0].Balance)
	var documentedRequest types.QueryRedelegationsRequest
	require.NoError(t, f.cdc.UnmarshalAsJSON(repairM690JSONBlock(t, doc, "redelegations-request"), &documentedRequest))
	require.Equal(t, &types.QueryRedelegationsRequest{DelegatorAddr: del.String(), SrcValidatorAddr: src.OperatorAddress, DstValidatorAddr: dst.OperatorAddress, Pagination: &query.PageRequest{Limit: 1, CountTotal: true}}, &documentedRequest)
	grpcResponse, err := grpcClient.Redelegations(context.Background(), &documentedRequest)
	require.NoError(t, err)
	keeperResponse, err := (keeper.Querier{Keeper: f.staking}).Redelegations(sdk.WrapSDKContext(f.ctx), &documentedRequest)
	require.NoError(t, err)
	require.Equal(t, keeperResponse, grpcResponse)
	require.Nil(t, grpcResponse.RedelegationResponses[0].Redelegation.Entries)
	var grpcJSONResponse types.QueryRedelegationsResponse
	require.NoError(t, f.cdc.UnmarshalAsJSON(f.cdc.MustMarshalJSON(grpcResponse), &grpcJSONResponse))
	require.Equal(t, response, grpcJSONResponse)
	out, err = clitestutil.ExecTestCLICmd(clientCtx, cli.GetCmdQueryRedelegation(), []string{del.String(), src.OperatorAddress, dst.OperatorAddress, "--output=json"})
	require.NoError(t, err, out.String())
	t.Logf("staking redelegation CLI capture: %s", out.String())
	require.JSONEq(t, string(repairM690JSONBlock(t, doc, "redelegations-cli")), out.String())
	var cliResult types.QueryRedelegationsResponse
	require.NoError(t, f.cdc.UnmarshalAsJSON(out.Bytes(), &cliResult))
	require.Len(t, cliResult.RedelegationResponses, 1)
	require.Equal(t, identity, cliResult.RedelegationResponses[0].Redelegation)
	require.Equal(t, response.RedelegationResponses[0].Entries, cliResult.RedelegationResponses[0].Entries)
	require.Equal(t, &query.PageResponse{Total: 0}, cliResult.Pagination, "the exact-triplet CLI does not request a total")
	for _, address := range []string{del.String(), src.OperatorAddress, dst.OperatorAddress} {
		require.Contains(t, doc, address)
	}
	commands := cli.GetQueryCmd().Commands()
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		names = append(names, command.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		require.Contains(t, doc, "#### "+name)
	}
}

func TestRepairM_690_MessageRestrictions(t *testing.T) {
	f := repairM690Open(t, t.TempDir())
	operator := repairM690Address(61)
	address := sdk.ValAddress(operator)
	amount := f.staking.TokensFromConsensusPower(f.ctx, 10)
	f.fund(t, operator, amount)
	key := ed25519.GenPrivKeyFromSecret([]byte("commission-fixture")).PubKey()
	create := func(rate sdk.Dec) *types.MsgCreateValidator {
		msg, err := types.NewMsgCreateValidator(address, key, sdk.NewCoin(f.staking.BondDenom(f.ctx), amount), types.Description{Moniker: "commission-fixture"}, types.NewCommissionRates(rate, sdk.OneDec(), sdk.OneDec()), sdk.OneInt())
		require.NoError(t, err)
		require.NoError(t, msg.ValidateBasic())
		return msg
	}
	server := keeper.NewMsgServerImpl(f.staking)
	_, err := server.CreateValidator(sdk.WrapSDKContext(f.ctx), create(sdk.NewDecWithPrec(4, 2)))
	require.ErrorIs(t, err, types.ErrCommissionLTMinRate)
	_, found := f.staking.GetValidator(f.ctx, address)
	require.False(t, found)
	_, err = server.CreateValidator(sdk.WrapSDKContext(f.ctx), create(f.staking.MinCommissionRate(f.ctx)))
	require.NoError(t, err)
	// Third-party tokens cannot back an operator's requested minimum.
	f.delegate(t, repairM690Address(62), address, 20)
	validator, found := f.staking.GetValidator(f.ctx, address)
	require.True(t, found)
	minimum := f.staking.TokensFromConsensusPower(f.ctx, 11)
	require.True(t, validator.Tokens.GT(minimum))
	_, err = server.EditValidator(sdk.WrapSDKContext(f.ctx), types.NewMsgEditValidator(address, types.Description{}, nil, &minimum))
	require.ErrorIs(t, err, types.ErrSelfDelegationBelowMinimum)
	unchanged, found := f.staking.GetValidator(f.ctx, address)
	require.True(t, found)
	require.Equal(t, validator, unchanged)
	// Drive the default 20% cap at the default projected-power threshold.
	del := repairM690Address(63)
	addition := f.staking.TokensFromConsensusPower(f.ctx, 1_000_000-30)
	f.fund(t, del, addition)
	before := f.bank.GetBalance(f.ctx, del, f.staking.BondDenom(f.ctx))
	_, err = server.Delegate(sdk.WrapSDKContext(f.ctx), types.NewMsgDelegate(del, address, sdk.NewCoin(before.Denom, addition)))
	require.ErrorIs(t, err, types.ErrExceedMaxVotingPowerRatio)
	require.Equal(t, before, f.bank.GetBalance(f.ctx, del, before.Denom))
	unchanged, found = f.staking.GetValidator(f.ctx, address)
	require.True(t, found)
	require.Equal(t, validator, unchanged)
	doc := repairM690Doc(t, "03_messages.md")
	for _, claim := range []string{"20%", "1000000", "5%", "ErrSelfDelegationBelowMinimum", "operator's own delegation"} {
		require.Contains(t, doc, claim)
	}
}
