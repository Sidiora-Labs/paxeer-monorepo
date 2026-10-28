package sequencer_test

import (
	"context"
	"math"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestSealBatchEqualsGigaProcessBlock(t *testing.T) {
	previousOCC := app.EnableOCC
	previousSkipResultsHash := tmtypes.SkipLastResultsHashValidation.Load()
	app.EnableOCC = false
	t.Cleanup(func() {
		app.EnableOCC = previousOCC
		tmtypes.SkipLastResultsHashValidation.Store(previousSkipResultsHash)
	})

	timestamp := time.Unix(1_800_000_000, 0).UTC()
	validator := secp256k1.GenPrivKey().PubKey()
	senders := []utils.TestAcct{utils.NewSigner(), utils.NewSigner(), utils.NewSigner(), utils.NewSigner()}
	newApp := func() *app.TestWrapper {
		w := app.NewGigaTestWrapper(t, timestamp, validator, false, false, func(ba *baseapp.BaseApp) {
			ba.SetOccEnabled(false)
			ba.SetConcurrencyWorkers(1)
		})
		w.App.ChainID = testChainID
		return w
	}
	primary, replay := newApp(), newApp()
	tmtypes.SkipLastResultsHashValidation.Store(previousSkipResultsHash)
	primary.App.GigaExecutorEnabled = false
	require.True(t, replay.App.GigaExecutorEnabled)
	require.NotNil(t, replay.App.GigaEvmKeeper.EvmoneVM)
	replay.App.EpochKeeper.SetEpoch(replay.Ctx, primary.App.EpochKeeper.GetEpoch(primary.Ctx))
	public := publicOf(testKeySeed)
	for _, w := range []*app.TestWrapper{primary, replay} {
		a, ctx := w.App, w.Ctx
		resolutionBlockGas(ctx, a)
		for _, sender := range senders {
			a.EvmKeeper.SetAddressMapping(ctx, sender.AccountAddress, sender.EvmAddress)
			w.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", fundingUhpx)))
		}
		require.NoError(t, a.SequencerKeeper.SetReceiptKeys(ctx, seqkeeper.ReceiptKeys{{
			KeyId: testKeyID[:], PublicKey: public[:], FirstHeight: 1, LastHeight: math.MaxUint64,
		}}))
		a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(committedTip), app.MergeUpgrade)
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, openHeight))
		_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{
				ChainID: testChainID, Height: committedTip, Time: timestamp,
				ProposerAddress: validator.Address().Bytes(),
			},
		})
		require.NoError(t, err)
		a.SetDeliverStateToCommit()
		_, err = a.Commit(context.Background())
		require.NoError(t, err)
		require.Equal(t, committedTip, a.LastBlockHeight())
		require.True(t, a.Sequenced(a.GetCheckCtx(), openHeight))
	}
	require.Equal(t, primary.App.CommitMultiStore().LastCommitID(), replay.App.CommitMultiStore().LastCommitID())
	initialHash := append([]byte(nil), primary.App.CommitMultiStore().LastCommitID().Hash...)

	exec, err := sequencer.NewExecutor(primary.App, sequencer.Options{
		ChainID: testChainID, ProposerAddress: validator.Address().Bytes(),
	})
	require.NoError(t, err)
	blockTime := timestamp.Add(time.Second)
	require.NoError(t, exec.Open(openHeight, blockTime.UnixMilli()))
	var transactions []*ethtypes.Transaction
	for nonce := range uint64(25) {
		for i, sender := range senders {
			contract := crypto.CreateAddress(sender.EvmAddress, 0)
			peer := senders[(i+1)%len(senders)].EvmAddress
			var data ethtypes.TxData
			switch {
			case nonce == 0:
				data = receiptLegacyTx(nonce, nil, big.NewInt(0), createGas, logInit)
			case nonce%2 == 0:
				data = receiptLegacyTx(nonce, &contract, big.NewInt(0), callGas, nil)
			default:
				data = receiptLegacyTx(nonce, &peer, big.NewInt(int64(nonce)), transferGas, nil)
			}
			signed, tx := evmTx(t, sender, data)
			accepted, err := exec.Accept(tx)
			require.NoError(t, err, "tx %d", len(transactions)+1)
			require.Zero(t, accepted.Result.Code, accepted.Result.Log)
			require.Equal(t, len(transactions)+1, accepted.TxIndex)
			require.Equal(t, [32]byte(signed.Hash()), accepted.ActivityID)
			transactions = append(transactions, signed)
		}
	}
	require.Len(t, transactions, 100)
	batch, err := exec.Seal(openHeight)
	require.NoError(t, err)
	msg := signSealBatch(t, sequencer.BatchFields(batch), testKeySeed, testKeyID)
	sealed, tx0, err := sequencer.SealBatch(primary.App, batch, msg)
	require.NoError(t, err)
	require.Len(t, sealed.Txs, 101)
	require.Len(t, sealed.Results, 101)
	require.Equal(t, tx0, sealed.Txs[0])
	require.Equal(t, abci.ExecTxResult{}, *sealed.Results[0])
	requireStoredSequence(t, primary.App, sealed.Context(), 100, batch.LastSeqHash)

	ms := replay.App.CommitMultiStore().CacheMultiStore()
	ctx := sdk.NewContext(ms, batch.Ctx.BlockHeader(), false)
	ctx = ctx.WithConsensusParams(replay.App.GetConsensusParams(ctx))
	_, results, _, err := replay.App.ProcessBlock(ctx, sealed.Txs, &app.BlockProcessRequest{
		Height: openHeight, Time: blockTime,
	}, abci.CommitInfo{}, false, nil)
	require.NoError(t, err)
	require.Len(t, results, len(sealed.Results))
	require.Equal(t, abci.ExecTxResult{}, *results[0])
	requireStoredSequence(t, replay.App, ctx, 100, batch.LastSeqHash)
	for i, want := range sealed.Results {
		got := results[i]
		require.NotNil(t, got, "tx %d", i)
		require.Equal(t, want.Code, got.Code, "tx %d: %s", i, got.Log)
		require.Equal(t, want.Data, got.Data, "tx %d result data", i)
		require.Equal(t, want.GasWanted, got.GasWanted, "tx %d gas wanted", i)
		require.Equal(t, want.GasUsed, got.GasUsed, "tx %d gas used", i)
		require.Equal(t, want.Codespace, got.Codespace, "tx %d codespace", i)
		if i == 0 {
			continue
		}
		require.Positive(t, got.GasUsed, "tx %d", i)
		require.NotNil(t, want.EvmTxInfo, "tx %d", i)
		require.NotNil(t, got.EvmTxInfo, "tx %d", i)
		require.Equal(t, want.EvmTxInfo.TxHash, got.EvmTxInfo.TxHash, "tx %d hash", i)
		require.Equal(t, want.EvmTxInfo.Nonce, got.EvmTxInfo.Nonce, "tx %d nonce", i)
		require.Equal(t, want.EvmTxInfo.VmError, got.EvmTxInfo.VmError, "tx %d VM error", i)
		hash := transactions[i-1].Hash()
		wantReceipt, err := primary.App.EvmKeeper.GetTransientReceipt(sealed.Context(), hash, uint64(i))
		require.NoError(t, err)
		gotReceipt, err := replay.App.EvmKeeper.GetTransientReceipt(ctx, hash, uint64(i))
		require.NoError(t, err)
		require.Equal(t, wantReceipt, gotReceipt, "tx %d receipt", i)
		if transactions[i-1].To() != nil && transactions[i-1].Nonce()%2 == 0 {
			require.Len(t, gotReceipt.Logs, 3, "tx %d contract call", i)
		}
	}
	for _, sender := range senders {
		require.Equal(t, uint64(25), primary.App.EvmKeeper.GetNonce(sealed.Context(), sender.EvmAddress))
		require.Equal(t, uint64(25), replay.App.EvmKeeper.GetNonce(ctx, sender.EvmAddress))
	}
	require.Equal(t, initialHash, replay.App.CommitMultiStore().LastCommitID().Hash)
	batch.Store.Write()
	ms.Write()
	sealedHash := primary.App.GetWorkingHash()
	gigaHash := replay.App.GetWorkingHash()
	require.NotEmpty(t, sealedHash)
	require.NotEqual(t, initialHash, sealedHash)
	require.Equal(t, sealedHash, gigaHash)
}
