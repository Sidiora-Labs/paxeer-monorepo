package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	appparams "github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	paramstypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// mergeHeight is H_act, the height the merge plan applies at in these tests.
const mergeHeight = int64(42)

var (
	mergePrimary = [32]byte(bytes.Repeat([]byte{0x51}, 32))
	mergeStandby = [32]byte(bytes.Repeat([]byte{0x52}, 32))
)

// mergeReceiptKey is the receipt key the plan pins, from firstHeight on.
func mergeReceiptKey(firstHeight uint64) seqkeeper.ReceiptKeyEntry {
	return seqkeeper.ReceiptKeyEntry{
		KeyId:       bytes.Repeat([]byte{0x44}, seqtypes.KeyIDLength),
		PublicKey:   ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, ed25519.SeedSize)).Public().(ed25519.PublicKey),
		FirstHeight: firstHeight,
		LastHeight:  math.MaxUint64,
	}
}

// mergePlanInfo is the sequencer genesis a merge plan carries in its info.
func mergePlanInfo(t *testing.T, genesis map[string]any) string {
	t.Helper()
	bz, err := json.Marshal(genesis)
	require.NoError(t, err)
	return string(bz)
}

// activationInfo pins L = [primary, standby] and one receipt key from H_act on.
func activationInfo(t *testing.T) string {
	return mergePlanInfo(t, map[string]any{
		"leaders":      [][]byte{mergePrimary[:], mergeStandby[:]},
		"receipt_keys": seqkeeper.ReceiptKeys{mergeReceiptKey(uint64(mergeHeight))},
	})
}

// rewindBeforeMerge turns the test application into a chain that has not applied
// the merge plan: all eight merge stores are empty, sequencer parameters are
// unset and the module version map carries none of the merged modules.
func rewindBeforeMerge(t *testing.T, a *App, ctx sdk.Context) {
	t.Helper()
	for _, name := range mergeStoreUpgrades().Added {
		store := ctx.KVStore(a.GetKey(name))
		iter := store.Iterator(nil, nil)
		var stale [][]byte
		for ; iter.Valid(); iter.Next() {
			stale = append(stale, bytes.Clone(iter.Key()))
		}
		require.NoError(t, iter.Close())
		for _, key := range stale {
			store.Delete(key)
		}
	}
	params := ctx.KVStore(a.GetKey(paramstypes.StoreKey))
	for _, key := range [][]byte{seqkeeper.KeyReceiptKeys, seqkeeper.KeyLeaders} {
		params.Delete(append([]byte(seqtypes.ModuleName+"/"), key...))
	}
	for _, name := range mergeStoreUpgrades().Added {
		ctx.KVStore(a.GetKey(upgradetypes.StoreKey)).Delete(append([]byte{upgradetypes.VersionMapByte}, name...))
		require.NotContains(t, a.UpgradeKeeper.GetModuleVersionMap(ctx), name)
	}
	require.Nil(t, seqkeeper.GetReceiptKeys(ctx, a.GetSubspace(seqtypes.ModuleName)))
	require.Nil(t, seqkeeper.GetLeadersParam(ctx, a.GetSubspace(seqtypes.ModuleName)))
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
}

// snapshotStores copies every pair of every KV store the merge plan must leave
// untouched: every store but the merge stores it adds and the upgrade store
// that records it, with the params store taken without the sequencer subspace.
func snapshotStores(t *testing.T, a *App, ctx sdk.Context) map[string]map[string][]byte {
	t.Helper()
	snapshot := make(map[string]map[string][]byte, len(kvStoreKeyNames))
	for _, name := range kvStoreKeyNames {
		if indexOf(mergeStoreUpgrades().Added, name) >= 0 || name == upgradetypes.StoreKey {
			continue
		}
		pairs := map[string][]byte{}
		iter := ctx.KVStore(a.GetKey(name)).Iterator(nil, nil)
		for ; iter.Valid(); iter.Next() {
			if name == paramstypes.StoreKey && bytes.HasPrefix(iter.Key(), []byte(seqtypes.ModuleName+"/")) {
				continue
			}
			pairs[string(iter.Key())] = bytes.Clone(iter.Value())
		}
		require.NoError(t, iter.Close())
		snapshot[name] = pairs
	}
	return snapshot
}

func snapshotMergeStores(t *testing.T, a *App, ctx sdk.Context) map[string]map[string][]byte {
	t.Helper()
	snapshot := make(map[string]map[string][]byte, len(kvStoreKeyNames))
	for _, name := range kvStoreKeyNames {
		pairs := map[string][]byte{}
		iter := ctx.KVStore(a.GetKey(name)).Iterator(nil, nil)
		for ; iter.Valid(); iter.Next() {
			pairs[string(iter.Key())] = bytes.Clone(iter.Value())
		}
		require.NoError(t, iter.Close())
		snapshot[name] = pairs
	}
	return snapshot
}

func TestUpgradesSequencerInitialisesAllMergeStores(t *testing.T) {
	upgrades, ok := layerxStoreUpgrades(MergeUpgrade)
	require.True(t, ok)
	require.Equal(t, []string{keys.SequencerStoreKey, keys.LayerXLedgerStoreKey, keys.LayerXAuthStoreKey,
		keys.LayerXPayStoreKey, keys.LayerXChainsStoreKey, keys.LayerXProgramsStoreKey,
		keys.LayerXAgentStoreKey, keys.LayerXHostStoreKey}, upgrades.Added)
	require.Empty(t, upgrades.Deleted)
	require.Empty(t, upgrades.Renamed)
	require.Contains(t, knownUpgradePlans(), MergeUpgrade)

	testWrapper := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := testWrapper.App, testWrapper.Ctx.WithBlockHeight(mergeHeight)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	require.True(t, a.UpgradeKeeper.HasHandler(MergeUpgrade))
	rewindBeforeMerge(t, a, ctx)

	// An account with a balance, a nonce and code, so the comparison covers each.
	contract := common.HexToAddress("0x00000000000000000000000000000000000c0de1")
	code := []byte{0x60, 0x00, 0x60, 0x00, 0xf3}
	a.EvmKeeper.SetCode(ctx, contract, code)
	a.EvmKeeper.SetNonce(ctx, contract, 7)
	funding := sdk.NewCoins(sdk.NewInt64Coin(appparams.BaseCoinUnit, 1_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, "evm", funding))
	holder := a.AccountKeeper.GetModuleAddress("evm")
	balances := a.BankKeeper.GetAllBalances(ctx, holder)
	require.True(t, balances.IsAllGTE(funding))
	before := snapshotStores(t, a, ctx)

	versions := a.UpgradeKeeper.GetModuleVersionMap(ctx)
	a.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: activationInfo(t)})

	require.Equal(t, mergeHeight, a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
	after := a.UpgradeKeeper.GetModuleVersionMap(ctx)
	for _, name := range upgrades.Added {
		if name == keys.LayerXProgramsStoreKey {
			require.NotContains(t, a.mm.Modules, name)
			require.NotContains(t, after, name)
			continue
		}
		require.Equal(t, a.mm.Modules[name].ConsensusVersion(), after[name], name)
		delete(after, name)
	}
	require.Equal(t, versions, after)

	require.Equal(t, before, snapshotStores(t, a, ctx))
	require.Equal(t, balances, a.BankKeeper.GetAllBalances(ctx, holder))
	require.Equal(t, uint64(7), a.EvmKeeper.GetNonce(ctx, contract))
	require.Equal(t, crypto.Keccak256Hash(code), a.EvmKeeper.GetCodeHash(ctx, contract))

	// A replay cannot refund or reinitialize already migrated state.
	committed := snapshotMergeStores(t, a, ctx)
	currentVersions := a.UpgradeKeeper.GetModuleVersionMap(ctx)
	_, err := a.mergeUpgradePlan(ctx, upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: activationInfo(t)}, currentVersions)
	require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
	require.ErrorContains(t, err, "upgrade with name v7.0 has already been completed")
	require.Equal(t, committed, snapshotMergeStores(t, a, ctx))
	require.Equal(t, currentVersions, a.UpgradeKeeper.GetModuleVersionMap(ctx))
	require.Equal(t, mergeHeight, a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
}

func TestUpgradesSequencerGenesisAtActivation(t *testing.T) {
	testWrapper := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := testWrapper.App, testWrapper.Ctx.WithBlockHeight(mergeHeight)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	rewindBeforeMerge(t, a, ctx)
	versions := a.UpgradeKeeper.GetModuleVersionMap(ctx)
	activationOnlyKey := mergeReceiptKey(uint64(mergeHeight))
	activationOnlyKey.LastHeight = uint64(mergeHeight)

	refusals := []struct {
		name           string
		info           string
		invalidGenesis bool
		wantText       string
	}{
		{"not a genesis", "v7.0", false, "sequencer genesis"},
		{"no plan info", "", false, "sequencer genesis"},
		{"no leader list", mergePlanInfo(t, map[string]any{
			"receipt_keys": seqkeeper.ReceiptKeys{mergeReceiptKey(uint64(mergeHeight))},
		}), true, "carries no leader list"},
		{"lastSeq above 0", mergePlanInfo(t, map[string]any{
			"leaders":       [][]byte{mergePrimary[:]},
			"receipt_keys":  seqkeeper.ReceiptKeys{mergeReceiptKey(uint64(mergeHeight))},
			"last_seq":      5,
			"last_seq_hash": bytes.Repeat([]byte{0x77}, seqtypes.HashLength),
		}), true, "want 0"},
		{"no receipt key for H_act+1", mergePlanInfo(t, map[string]any{
			"leaders":      [][]byte{mergePrimary[:]},
			"receipt_keys": seqkeeper.ReceiptKeys{mergeReceiptKey(uint64(mergeHeight) + 2)},
		}), true, "pins no receipt key for first sequenced height"},
		{"explicit activation is uncovered", mergePlanInfo(t, map[string]any{
			"leaders":           [][]byte{mergePrimary[:]},
			"receipt_keys":      seqkeeper.ReceiptKeys{mergeReceiptKey(uint64(mergeHeight) + 1)},
			"activation_height": mergeHeight,
		}), true, "no receipt key covers activation height"},
		{"explicit activation key misses first sequenced height", mergePlanInfo(t, map[string]any{
			"leaders":           [][]byte{mergePrimary[:]},
			"receipt_keys":      seqkeeper.ReceiptKeys{activationOnlyKey},
			"activation_height": mergeHeight,
		}), true, "pins no receipt key for first sequenced height"},
		{"zero leader", mergePlanInfo(t, map[string]any{
			"leaders":      [][]byte{make([]byte, 32)},
			"receipt_keys": seqkeeper.ReceiptKeys{mergeReceiptKey(uint64(mergeHeight))},
		}), true, "sequencer genesis"},
	}
	for _, refusal := range refusals {
		t.Run(refusal.name, func(t *testing.T) {
			cacheCtx, _ := ctx.CacheContext()
			before := snapshotMergeStores(t, a, cacheCtx)
			plan := upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: refusal.info}
			require.NotPanics(t, func() {
				returned, err := a.mergeUpgradePlan(cacheCtx, plan, versions)
				require.Error(t, err, refusal.name)
				require.Equal(t, refusal.invalidGenesis, errors.Is(err, seqtypes.ErrInvalidGenesis), refusal.name)
				require.ErrorContains(t, err, refusal.wantText)
				require.Equal(t, versions, returned)
			})
			require.Zero(t, a.UpgradeKeeper.GetDoneHeight(cacheCtx, MergeUpgrade), refusal.name)
			require.Equal(t, before, snapshotMergeStores(t, a, cacheCtx))
		})
	}

	info := activationInfo(t)
	a.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: info})

	state, err := a.SequencerKeeper.ExportGenesis(ctx)
	require.NoError(t, err)
	require.Equal(t, [][32]byte{mergePrimary, mergeStandby}, state.Leaders)
	pinned := mergeReceiptKey(uint64(mergeHeight))
	want := seqtypes.ReceiptKey{
		KeyId:       [seqtypes.KeyIDLength]byte(pinned.KeyId),
		PublicKey:   [32]byte(pinned.PublicKey),
		FirstHeight: uint64(mergeHeight),
		LastHeight:  math.MaxUint64,
	}
	require.Equal(t, []seqtypes.ReceiptKey{want}, state.ReceiptKeys)
	require.Zero(t, state.LastSeq)
	require.Equal(t, [seqtypes.HashLength]byte{}, state.LastSeqHash)
	require.Equal(t, uint64(mergeHeight+1), state.ActivationHeight)
	require.Equal(t, mergeHeight, a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
	require.Equal(t, seqkeeper.ReceiptKeys{pinned}, seqkeeper.GetReceiptKeys(ctx, a.GetSubspace(seqtypes.ModuleName)))

	exported := a.mm.Modules[seqtypes.ModuleName].ExportGenesis(ctx, a.appCodec)
	var resolved map[string]any
	require.NoError(t, json.Unmarshal([]byte(info), &resolved))
	resolved["activation_height"] = uint64(mergeHeight + 1)
	require.JSONEq(t, mergePlanInfo(t, resolved), string(exported))
}

func TestMergePlanRequiresKeyForFirstSequencedHeight(t *testing.T) {
	testWrapper := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := testWrapper.App, testWrapper.Ctx.WithBlockHeight(mergeHeight)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	rewindBeforeMerge(t, a, ctx)

	key := mergeReceiptKey(uint64(mergeHeight) + 1)
	key.LastHeight = key.FirstHeight
	info := mergePlanInfo(t, map[string]any{
		"leaders":      [][]byte{mergePrimary[:], mergeStandby[:]},
		"receipt_keys": seqkeeper.ReceiptKeys{key},
	})
	a.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: info})

	require.Equal(t, mergeHeight, a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
	require.False(t, a.Sequenced(ctx, mergeHeight))
	require.True(t, a.Sequenced(ctx, mergeHeight+1))
	state, err := a.SequencerKeeper.ExportGenesis(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(mergeHeight+1), state.ActivationHeight)
	require.Len(t, state.ReceiptKeys, 1)
	require.False(t, state.ReceiptKeys[0].Covers(uint64(mergeHeight)))
	require.True(t, state.ReceiptKeys[0].Covers(uint64(mergeHeight)+1))
	require.Equal(t, seqkeeper.ReceiptKeys{key}, seqkeeper.GetReceiptKeys(ctx, a.GetSubspace(seqtypes.ModuleName)))
}

func TestMergePlanRefusesKeyCoveringOnlyActivationHeight(t *testing.T) {
	testWrapper := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := testWrapper.App, testWrapper.Ctx.WithBlockHeight(mergeHeight)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	rewindBeforeMerge(t, a, ctx)
	versions := a.UpgradeKeeper.GetModuleVersionMap(ctx)

	key := mergeReceiptKey(uint64(mergeHeight))
	key.LastHeight = uint64(mergeHeight)
	info := mergePlanInfo(t, map[string]any{
		"leaders":      [][]byte{mergePrimary[:], mergeStandby[:]},
		"receipt_keys": seqkeeper.ReceiptKeys{key},
	})
	plan := upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: info}
	cacheCtx, _ := ctx.CacheContext()
	before := snapshotMergeStores(t, a, cacheCtx)
	var refusal error
	require.NotPanics(t, func() {
		returned, err := a.mergeUpgradePlan(cacheCtx, plan, versions)
		refusal = err
		require.Equal(t, versions, returned)
	})
	err := refusal
	require.ErrorIs(t, err, seqtypes.ErrInvalidGenesis)
	want := fmt.Errorf("upgrade %s pins no receipt key for first sequenced height %d: sequencer genesis: %w", MergeUpgrade, mergeHeight+1,
		seqtypes.ErrInvalidGenesis.Wrapf("no receipt key covers activation height %d", mergeHeight+1))
	require.EqualError(t, err, want.Error())
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(cacheCtx, MergeUpgrade))
	require.False(t, a.Sequenced(cacheCtx, mergeHeight+1))
	require.Equal(t, before, snapshotMergeStores(t, a, cacheCtx))
	state, err := a.SequencerKeeper.ExportGenesis(cacheCtx)
	require.NoError(t, err)
	require.Empty(t, state.ReceiptKeys)
	require.Empty(t, state.Leaders)
	require.Zero(t, state.ActivationHeight)
	require.Nil(t, seqkeeper.GetReceiptKeys(cacheCtx, a.GetSubspace(seqtypes.ModuleName)))
	require.Equal(t, versions, a.UpgradeKeeper.GetModuleVersionMap(cacheCtx))

	migrationCtx, _ := ctx.CacheContext()
	require.NotPanics(t, func() {
		returned, err := a.migrateMergeState(migrationCtx, plan, versions)
		require.ErrorIs(t, err, seqtypes.ErrInvalidGenesis)
		require.EqualError(t, err, want.Error())
		require.Equal(t, versions, returned)
	})
	require.Equal(t, before, snapshotMergeStores(t, a, migrationCtx))
	require.Empty(t, migrationCtx.EventManager().Events())
}

func TestUpgradesSequencerSwitchAtActivation(t *testing.T) {
	testWrapper := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a := testWrapper.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	const mergeAt = int64(2)
	const firstSequenced = mergeAt + 1
	commit := func(write func(sdk.Context), txs ...[]byte) {
		t.Helper()
		height := a.LastBlockHeight() + 1
		_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{ChainID: "pax-test", Height: height},
			Txs:    txs,
		})
		require.NoError(t, err)
		write(a.GetContextForDeliverTx(nil))
		a.SetDeliverStateToCommit()
		_, err = a.Commit(context.Background())
		require.NoError(t, err)
		require.Equal(t, height, a.LastBlockHeight())
	}
	commit(func(ctx sdk.Context) { rewindBeforeMerge(t, a, ctx) })

	for _, height := range []int64{mergeAt - 1, mergeAt, mergeAt + 1000} {
		activation, leaders, err := a.LeaderSchedule(height)
		require.NoError(t, err)
		require.Zero(t, activation)
		require.Empty(t, leaders)
		ctx, err := a.CreateQueryContext(0, false)
		require.NoError(t, err)
		require.False(t, a.Sequenced(ctx, height))
	}

	commit(func(ctx sdk.Context) {
		require.Equal(t, mergeAt, ctx.BlockHeight())
		info := mergePlanInfo(t, map[string]any{
			"leaders":      [][]byte{mergePrimary[:], mergeStandby[:]},
			"receipt_keys": seqkeeper.ReceiptKeys{mergeReceiptKey(uint64(firstSequenced))},
		})
		a.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: MergeUpgrade, Height: mergeAt, Info: info})
		require.Equal(t, mergeAt, a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
		require.False(t, a.Sequenced(ctx, mergeAt))
		require.True(t, a.Sequenced(ctx, firstSequenced))
	})

	for _, height := range []int64{mergeAt - 1, mergeAt, firstSequenced, mergeAt + 1000} {
		activation, leaders, err := a.LeaderSchedule(height)
		require.NoError(t, err)
		require.Equal(t, firstSequenced, activation)
		require.Equal(t, [][32]byte{mergePrimary, mergeStandby}, leaders)
		require.Equal(t, height > mergeAt, height >= activation)
	}

	cases := []struct {
		height int64
		round  int32
		want   [32]byte
	}{
		{firstSequenced, 0, mergePrimary},
		{firstSequenced, 1, mergeStandby},
		{firstSequenced, 2, mergePrimary},
		{firstSequenced, 5, mergeStandby},
		{firstSequenced + 10, 0, mergePrimary},
		{firstSequenced + 10, 3, mergeStandby},
	}
	for _, c := range cases {
		activation, leaders, err := a.LeaderSchedule(c.height)
		require.NoError(t, err)
		require.Equal(t, firstSequenced, activation)
		require.Equal(t, [][32]byte{mergePrimary, mergeStandby}, leaders)
		require.Equal(t, c.want, leaders[int(c.round)%len(leaders)], "height %d round %d", c.height, c.round)
	}

	batch := seqtypes.MsgSequenceBatch{
		Height:      uint64(firstSequenced),
		FromSeq:     1,
		ToSeq:       0,
		PrevSeqHash: make([]byte, seqtypes.HashLength),
		LastSeqHash: make([]byte, seqtypes.HashLength),
		TxRoot:      seqtypes.EmptyTxRoot[:],
		KeyId:       mergeReceiptKey(uint64(firstSequenced)).KeyId,
	}
	digest, err := batch.SignBytes(testWrapper.Ctx.ChainID())
	require.NoError(t, err)
	batch.Sig = ed25519.Sign(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, ed25519.SeedSize)), digest[:])
	tx, err := EncodeSequenceBatch(batch)
	require.NoError(t, err)
	commit(func(ctx sdk.Context) {
		ctx.KVStore(a.GetKey(keys.SequencerStoreKey)).Set(seqtypes.LeaderListKey, []byte{1})
	}, tx)
	activation, leaders, err := a.LeaderSchedule(firstSequenced)
	require.ErrorIs(t, err, seqkeeper.ErrCorruptStore)
	require.Zero(t, activation)
	require.Nil(t, leaders)
}
