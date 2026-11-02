package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
)

// V65StoreUpgrades exposes the v6.5 store upgrades to the external test package.
var V65StoreUpgrades = v65StoreUpgrades

// V66StoreUpgrades exposes the v6.6 store upgrades to the external test package.
var V66StoreUpgrades = v66StoreUpgrades

// LayerXStoreUpgrades exposes the plan-name lookup of SetStoreUpgradeHandlers.
var LayerXStoreUpgrades = layerxStoreUpgrades

func ExportAppStateAndValidators(t *testing.T, a *App) servertypes.ExportedApp {
	t.Helper()
	exported, err := a.ExportAppStateAndValidators(false, nil)
	require.NoError(t, err)
	return exported
}

func TestExportAfterMergeKeepsSequencerState(t *testing.T) {
	a := Setup(t, true, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	at := time.Unix(1_700_000_000, 0).UTC()
	const chainID = "pax-test"
	genesis, err := json.Marshal(NewDefaultGenesisState(a.AppCodec()))
	require.NoError(t, err)
	_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
		Time: at, ChainId: chainID, InitialHeight: mergeHeight,
		ConsensusParams: DefaultConsensusParams, AppStateBytes: genesis,
	})
	require.NoError(t, err)
	ctx := a.NewContext(false, tmproto.Header{ChainID: chainID, Height: mergeHeight, Time: at})
	rewindBeforeMerge(t, a, ctx)

	firstKey := mergeReceiptKey(uint64(mergeHeight))
	firstKey.LastHeight = uint64(mergeHeight + 1)
	nextKey := mergeReceiptKey(uint64(mergeHeight + 2))
	nextKey.KeyId = bytes.Repeat([]byte{0x45}, seqtypes.KeyIDLength)
	nextKey.PublicKey = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	keys := seqkeeper.ReceiptKeys{firstKey, nextKey}
	plan := upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: mergePlanInfo(t, map[string]any{
		"leaders": [][]byte{mergePrimary[:], mergeStandby[:]}, "receipt_keys": keys,
	})}
	require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(ctx, plan))
	versions, err := a.continueMergeUpgradePlan(ctx, plan, a.UpgradeKeeper.GetModuleVersionMap(ctx))
	require.NoError(t, err)
	a.UpgradeKeeper.SetModuleVersionMap(ctx, versions)
	require.True(t, a.Sequenced(ctx, mergeHeight+1))
	require.NoError(t, a.SetSealedStateToCommit(ctx, ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, mergeHeight, a.LastBlockHeight())

	header := tmproto.Header{ChainID: chainID, Height: mergeHeight + 1, Time: at.Add(time.Second)}
	ctx = sdk.NewContext(a.CommitMultiStore().CacheMultiStore(), header, false).WithTxIndex(0)
	activityIDs := [][32]byte{sha256.Sum256([]byte("first activity")), sha256.Sum256([]byte("second activity"))}
	var lastHash [seqtypes.HashLength]byte
	for i, id := range activityIDs {
		lastHash = seqtypes.SeqHash(lastHash, uint64(i+1), id)
	}
	root := seqtypes.TxRoot(activityIDs)
	batch := seqtypes.MsgSequenceBatch{
		Height: uint64(header.Height), FromSeq: 1, ToSeq: uint64(len(activityIDs)),
		PrevSeqHash: make([]byte, seqtypes.HashLength), LastSeqHash: lastHash[:],
		TxRoot: root[:], KeyId: firstKey.KeyId,
	}
	digest, err := batch.SignBytes(chainID)
	require.NoError(t, err)
	batch.Sig = ed25519.Sign(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, ed25519.SeedSize)), digest[:])
	require.NoError(t, a.SequencerKeeper.DeliverSequenceBatch(ctx, batch))
	want, err := a.SequencerKeeper.ExportGenesis(ctx)
	require.NoError(t, err)
	require.Equal(t, [][32]byte{mergePrimary, mergeStandby}, want.Leaders)
	require.Len(t, want.ReceiptKeys, len(keys))
	for i, key := range keys {
		require.Equal(t, seqtypes.ReceiptKey{
			KeyId: [seqtypes.KeyIDLength]byte(key.KeyId), PublicKey: [32]byte(key.PublicKey),
			FirstHeight: key.FirstHeight, LastHeight: key.LastHeight,
		}, want.ReceiptKeys[i])
	}
	require.Equal(t, uint64(len(activityIDs)), want.LastSeq)
	require.Equal(t, lastHash, want.LastSeqHash)
	require.Equal(t, uint64(mergeHeight+1), want.ActivationHeight)
	require.NoError(t, a.SetSealedStateToCommit(ctx, ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, header.Height, a.LastBlockHeight())

	exported := ExportAppStateAndValidators(t, a)
	require.Equal(t, header.Height+1, exported.Height)
	var exportedGenesis GenesisState
	require.NoError(t, json.Unmarshal(exported.AppState, &exportedGenesis))
	require.Contains(t, exportedGenesis, seqtypes.ModuleName)
	require.JSONEq(t, string(a.mm.Modules[seqtypes.ModuleName].ExportGenesis(ctx, a.AppCodec())), string(exportedGenesis[seqtypes.ModuleName]))

	restored := Setup(t, true, true, false)
	t.Cleanup(func() { require.NoError(t, restored.Close()) })
	_, err = restored.InitChain(context.Background(), &abci.RequestInitChain{
		Time: header.Time, ChainId: chainID, InitialHeight: exported.Height,
		ConsensusParams: exported.ConsensusParams, AppStateBytes: exported.AppState,
	})
	require.NoError(t, err)
	importCtx := restored.NewContext(false, tmproto.Header{ChainID: chainID, Height: exported.Height, Time: header.Time})
	got, err := restored.SequencerKeeper.ExportGenesis(importCtx)
	require.NoError(t, err)
	require.Equal(t, want.Leaders, got.Leaders)
	require.Equal(t, want.ReceiptKeys, got.ReceiptKeys)
	require.Equal(t, want.LastSeq, got.LastSeq)
	require.Equal(t, want.LastSeqHash, got.LastSeqHash)
	require.Equal(t, want, got)
	require.Equal(t, keys, seqkeeper.GetReceiptKeys(importCtx, restored.GetSubspace(seqtypes.ModuleName)))
}
