package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramstypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func TestMergeRehearsalFirstSequencedBlock(t *testing.T) {
	const chainID = "pax-test"
	const mergeAt = int64(2)
	const firstSequenced = mergeAt + 1
	genesisTime := time.Now().UTC().Truncate(time.Millisecond)
	valPub := secp256k1.GenPrivKey().PubKey()
	previousOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previousOCC })
	w := app.NewTestWrapper(t, genesisTime, valPub, true, func(ba *baseapp.BaseApp) {
		ba.SetOccEnabled(false)
		ba.SetConcurrencyWorkers(1)
	})
	a := w.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	params := a.GetConsensusParams(w.Ctx)
	params.Block.MaxGasWanted = seqCtxTransferGas
	a.StoreConsensusParams(w.Ctx, params)

	for _, name := range []string{
		keys.SequencerStoreKey, keys.LayerXLedgerStoreKey, keys.LayerXAuthStoreKey,
		keys.LayerXPayStoreKey, keys.LayerXProgramsStoreKey, keys.LayerXAgentStoreKey, keys.LayerXHostStoreKey,
	} {
		store := w.Ctx.KVStore(a.GetKey(name))
		iter := store.Iterator(nil, nil)
		var stale [][]byte
		for ; iter.Valid(); iter.Next() {
			stale = append(stale, bytes.Clone(iter.Key()))
		}
		require.NoError(t, iter.Close())
		for _, key := range stale {
			store.Delete(key)
		}
		w.Ctx.KVStore(a.GetKey(upgradetypes.StoreKey)).Delete(append([]byte{upgradetypes.VersionMapByte}, name...))
	}
	w.Ctx.KVStore(a.GetKey(paramstypes.StoreKey)).Delete(append([]byte(seqtypes.ModuleName+"/"), seqkeeper.KeyReceiptKeys...))
	for _, name := range []string{app.ActivationUpgrade, app.V610Upgrade, app.V6120Upgrade} {
		a.UpgradeKeeper.SetDone(w.Ctx.WithBlockHeight(1), name)
	}
	a.RefreshPrecompiles(w.Ctx)
	require.Empty(t, a.EvmKeeper.ActivationAddressesWithoutPrecompile())
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(w.Ctx, app.MergeUpgrade))
	require.NotContains(t, a.UpgradeKeeper.GetModuleVersionMap(w.Ctx), seqtypes.ModuleName)
	require.False(t, a.Sequenced(w.Ctx, mergeAt))
	require.False(t, a.Sequenced(w.Ctx, firstSequenced))

	sender, recipient := utils.NewSigner(), utils.NewSigner()
	a.EvmKeeper.SetAddressMapping(w.Ctx, sender.AccountAddress, sender.EvmAddress)
	w.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", seqCtxFundingUhpx)))
	seed := sha256.Sum256([]byte("merge rehearsal receipt key"))
	public := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
	keyID := sha256.Sum256([]byte("merge rehearsal receipt key id"))
	leader := sha256.Sum256([]byte("merge rehearsal leader"))
	info, err := json.Marshal(map[string]any{
		"leaders": [][]byte{leader[:]},
		"receipt_keys": seqkeeper.ReceiptKeys{{
			KeyId: keyID[:], PublicKey: public, FirstHeight: uint64(firstSequenced), LastHeight: math.MaxUint64,
		}},
	})
	require.NoError(t, err)
	plan := upgradetypes.Plan{Name: app.MergeUpgrade, Height: mergeAt, Info: string(info)}
	header := func(height int64, at time.Time) *tmproto.Header {
		return &tmproto.Header{ChainID: chainID, Height: height, Time: at, ProposerAddress: valPub.Address().Bytes()}
	}
	_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Header: header(1, genesisTime)})
	require.NoError(t, err)
	require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(a.GetContextForDeliverTx(nil), plan))
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), a.LastBlockHeight())
	before := a.GetCheckCtx()
	require.Equal(t, int64(1), a.UpgradeKeeper.GetDoneHeight(before, app.V6120Upgrade))
	scheduled, found := a.UpgradeKeeper.GetUpgradePlan(before)
	require.True(t, found)
	require.Equal(t, plan, scheduled)
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(before, app.MergeUpgrade))
	require.False(t, a.Sequenced(before, firstSequenced))

	response, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: header(mergeAt, genesisTime.Add(time.Second)),
	})
	require.NoError(t, err)
	require.Empty(t, response.TxResults)
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, mergeAt, a.LastBlockHeight())
	merged := a.GetCheckCtx()
	require.Equal(t, mergeAt, a.UpgradeKeeper.GetDoneHeight(merged, app.MergeUpgrade))
	_, found = a.UpgradeKeeper.GetUpgradePlan(merged)
	require.False(t, found)
	require.Contains(t, a.UpgradeKeeper.GetModuleVersionMap(merged), seqtypes.ModuleName)
	require.False(t, a.Sequenced(merged, mergeAt))
	require.True(t, a.Sequenced(merged, firstSequenced))
	state, err := a.SequencerKeeper.ExportGenesis(merged)
	require.NoError(t, err)
	require.Equal(t, [][32]byte{leader}, state.Leaders)
	require.Equal(t, uint64(firstSequenced), state.ActivationHeight)
	require.Zero(t, state.LastSeq)
	require.Equal(t, [32]byte{}, state.LastSeqHash)

	dir := t.TempDir()
	consensusPath := filepath.Join(dir, "priv_validator_key.json")
	consensusKey, err := privval.GenFilePV(consensusPath, filepath.Join(dir, "priv_validator_state.json"), "")
	require.NoError(t, err)
	require.NoError(t, consensusKey.Key.Save())
	receiptPath := filepath.Join(dir, "receipt_key")
	require.NoError(t, os.WriteFile(receiptPath, []byte(hex.EncodeToString(seed[:])), 0o600))
	pins, err := a.SequencerKeeper.ReceiptKeyPins(merged)
	require.NoError(t, err)
	key, err := sequencer.LoadReceiptKey(receiptPath, consensusPath, pins, firstSequenced)
	require.NoError(t, err)
	exec, err := sequencer.NewExecutor(a, sequencer.Options{
		ChainID: chainID, ProposerAddress: valPub.Address().Bytes(), LastSeq: state.LastSeq, LastSeqHash: state.LastSeqHash,
	})
	require.NoError(t, err)
	source, err := sequencer.NewConsensusSource(a, exec, key)
	require.NoError(t, err)
	a.SetSequencerSource(source)
	require.False(t, source.Sequenced(mergeAt))
	require.True(t, source.Sequenced(firstSequenced))
	openTime := genesisTime.Add(2 * time.Second)
	require.NoError(t, exec.Open(firstSequenced, openTime.UnixMilli()))
	value := big.NewInt(1_000_000_000_000_000_000)
	tx := seqCtxTransfer(t, sender, 0, recipient.EvmAddress, value)
	accepted, err := exec.Accept(tx)
	require.NoError(t, err)
	require.Equal(t, abci.CodeTypeOK, accepted.Result.Code, accepted.Result.Log)
	require.Equal(t, uint64(1), accepted.Seq)
	block, err := source.Seal(context.Background(), firstSequenced)
	require.NoError(t, err)
	require.Equal(t, openTime, block.OpenTime)
	require.Len(t, block.Txs, 1)
	require.Equal(t, tx, []byte(block.Txs[0]))
	batch, err := app.DecodeSequenceBatch(block.Batch)
	require.NoError(t, err)
	require.Equal(t, uint64(firstSequenced), batch.Height)
	require.Equal(t, uint64(1), batch.FromSeq)
	require.Equal(t, uint64(1), batch.ToSeq)
	require.Equal(t, state.LastSeqHash[:], batch.PrevSeqHash)
	require.NotEqual(t, state.LastSeqHash[:], batch.LastSeqHash)
	require.NoError(t, batch.VerifySignature(chainID, key.PublicKey()))
	require.NotNil(t, source.Sealed())
	txs := append([][]byte{block.Batch}, block.Txs.ToSliceOfBytes()...)
	require.Equal(t, txs, source.Sealed().Txs)
	lastSeq, err := a.SequencerKeeper.LastSeq(a.GetCheckCtx())
	require.NoError(t, err)
	require.Zero(t, lastSeq)
	response, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: header(firstSequenced, block.OpenTime), Txs: txs,
	})
	require.NoError(t, err)
	require.Len(t, response.TxResults, 2)
	for _, result := range response.TxResults {
		require.Equal(t, abci.CodeTypeOK, result.Code, result.Log)
	}
	require.Same(t, source.Sealed().Results[1], response.TxResults[1])
	require.NotEmpty(t, response.AppHash)
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, firstSequenced, a.LastBlockHeight())
	committed := a.GetCheckCtx()
	require.False(t, a.Sequenced(committed, mergeAt))
	require.True(t, a.Sequenced(committed, firstSequenced))
	lastSeq, err = a.SequencerKeeper.LastSeq(committed)
	require.NoError(t, err)
	require.Equal(t, uint64(1), lastSeq)
	require.Equal(t, batch.ToSeq, lastSeq)
	require.Greater(t, lastSeq, state.LastSeq)
	lastHash, err := a.SequencerKeeper.LastSeqHash(committed)
	require.NoError(t, err)
	require.Equal(t, batch.LastSeqHash, lastHash[:])
	require.Equal(t, response.AppHash, a.LastCommitID().Hash)
	require.Equal(t, uint64(1), a.EvmKeeper.GetNonce(committed, sender.EvmAddress))
	require.Zero(t, value.Cmp(seqCtxWei(a, committed, recipient.EvmAddress)))
}
