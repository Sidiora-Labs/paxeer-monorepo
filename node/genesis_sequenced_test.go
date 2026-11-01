package app_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func InitChainer(t *testing.T, w *app.TestWrapper) {
	t.Helper()
	var genesis map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(activationGenesis(t), &genesis))
	genesis["activation_height"] = json.RawMessage(`0`)
	sequencerGenesis, err := json.Marshal(genesis)
	require.NoError(t, err)
	state, err := json.Marshal(app.GenesisState{seqtypes.ModuleName: sequencerGenesis})
	require.NoError(t, err)
	w.App.InitChainer(w.Ctx, abci.RequestInitChain{AppStateBytes: state})
	activation, err := w.App.SequencerKeeper.ActivationHeight(w.Ctx)
	require.NoError(t, err)
	require.Equal(t, w.Ctx.BlockHeight()+1, activation)
}

func Sequenced(t *testing.T, w *app.TestWrapper, initialHeight int64) {
	t.Helper()
	require.False(t, w.App.Sequenced(w.Ctx, initialHeight))
	for _, height := range []int64{initialHeight + 1, initialHeight + 2, initialHeight + 100} {
		require.True(t, w.App.Sequenced(w.Ctx, height), "height %d", height)
	}
}

func TestGenesisBornChainSequencedFromSecondBlock(t *testing.T) {
	at := time.Now().UTC()
	validator := secp256k1.GenPrivKey().PubKey()
	w := newSequencerWrapper(t, at, validator, false)
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	initialHeight := w.Ctx.BlockHeight()
	require.Equal(t, int64(1), initialHeight)
	InitChainer(t, w)
	Sequenced(t, w, initialHeight)
	params := w.App.GetConsensusParams(w.Ctx)
	params.Block.MaxGasWanted = 21000
	w.App.StoreConsensusParams(w.Ctx, params)

	sender := utils.NewSigner()
	w.App.EvmKeeper.SetAddressMapping(w.Ctx, sender.AccountAddress, sender.EvmAddress)
	tx, hash := evmTransfer(t, w, sender, utils.NewSigner().EvmAddress)
	first, err := w.App.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: sequencedChainID, Height: initialHeight, Time: at},
	})
	require.NoError(t, err)
	require.Empty(t, first.TxResults)
	w.App.SetDeliverStateToCommit()
	_, err = w.App.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, initialHeight, w.App.LastBlockHeight())
	w.Ctx = w.App.GetCheckCtx()
	Sequenced(t, w, initialHeight)

	height := initialHeight + 1
	blockTime := at.Add(2 * time.Second).Truncate(time.Millisecond)
	executor, err := sequencer.NewExecutor(w.App, sequencer.Options{
		ChainID: sequencedChainID, ProposerAddress: validator.Address().Bytes(),
	})
	require.NoError(t, err)
	require.NoError(t, executor.Open(height, blockTime.UnixMilli()))
	accepted, err := executor.Accept(tx)
	require.NoError(t, err)
	require.Equal(t, uint32(0), accepted.Result.Code, accepted.Result.Log)
	require.Equal(t, uint64(1), accepted.Seq)
	require.Equal(t, [32]byte(hash), accepted.ActivityID)
	batch, err := executor.Seal(height)
	require.NoError(t, err)
	fields := sequencer.BatchFields(batch)
	tx0 := signedBatchAt(t, fields.Height, sequencerKeySeed, fields.FromSeq, fields.ToSeq,
		fields.PrevSeqHash[:], fields.LastSeqHash[:], fields.TxRoot[:], sequencerKeyID)
	msg, err := app.DecodeSequenceBatch(tx0)
	require.NoError(t, err)
	sealed, encoded, err := sequencer.SealBatch(w.App, batch, msg)
	require.NoError(t, err)
	require.Equal(t, tx0, encoded)
	require.Equal(t, height, sealed.Height)
	require.Equal(t, [][]byte{tx0, tx}, sealed.Txs)

	second, err := w.App.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: sequencedChainID, Height: height, Time: blockTime},
		Txs:    sealed.Txs,
	})
	require.NoError(t, err)
	require.Len(t, second.TxResults, 2)
	require.Equal(t, abci.ExecTxResult{}, *second.TxResults[0])
	require.Equal(t, uint32(0), second.TxResults[1].Code, second.TxResults[1].Log)
	require.Equal(t, accepted.Result.GasUsed, second.TxResults[1].GasUsed)
	w.Ctx = w.App.GetContextForDeliverTx(nil)
	require.Equal(t, height, w.Ctx.BlockHeight())
	Sequenced(t, w, initialHeight)
	requireSequencerState(t, w, 1, fields.LastSeqHash[:])
	requireReceiptIndex(t, w, hash, 1)
}
