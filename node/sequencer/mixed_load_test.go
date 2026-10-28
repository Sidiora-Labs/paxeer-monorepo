package sequencer_test

import (
	"context"
	"crypto/sha256"
	"math"
	"math/big"
	"math/rand"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestMixedLoadSealedEqualsProcessBlock(t *testing.T) {
	const transfers, bankSends, calls = 200, 50, 20
	const total = transfers + bankSends + calls
	previousOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previousOCC })

	timestamp := time.Unix(1_800_000_000, 0).UTC()
	validator := secp256k1.GenPrivKey().PubKey()
	sender, payer, recipient := utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
	contract := common.HexToAddress("0x5678")
	newApp := func() *app.TestWrapper {
		return app.NewTestWrapper(t, timestamp, validator, false, func(ba *baseapp.BaseApp) {
			ba.SetOccEnabled(false)
			ba.SetConcurrencyWorkers(1)
		})
	}
	primary, replay := newApp(), newApp()
	replay.App.EpochKeeper.SetEpoch(replay.Ctx, primary.App.EpochKeeper.GetEpoch(primary.Ctx))
	public := publicOf(testKeySeed)
	for _, w := range []*app.TestWrapper{primary, replay} {
		a, ctx := w.App, w.Ctx
		resolutionBlockGas(ctx, a)
		for _, acct := range []utils.TestAcct{sender, payer, recipient} {
			a.EvmKeeper.SetAddressMapping(ctx, acct.AccountAddress, acct.EvmAddress)
			w.FundAcc(acct.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", fundingUhpx)))
		}
		a.EvmKeeper.SetCode(ctx, contract, common.FromHex("0x60005460010180600055600052600160206000a100"))
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
		require.True(t, a.Sequenced(a.GetCheckCtx(), openHeight))
	}
	require.Equal(t, primary.App.CommitMultiStore().LastCommitID(), replay.App.CommitMultiStore().LastCommitID())
	initialHash := append([]byte(nil), primary.App.CommitMultiStore().LastCommitID().Hash...)

	exec, err := sequencer.NewExecutor(primary.App, sequencer.Options{
		ChainID: testChainID, ProposerAddress: validator.Address().Bytes(),
	})
	require.NoError(t, err)
	h := &harness{t: t, app: primary.App, exec: exec, proposer: validator.Address().Bytes(), genesis: timestamp}
	blockTime := timestamp.Add(time.Second)
	h.openAt(blockTime)
	order := make([]int, total)
	for i := range order {
		switch {
		case i < transfers:
			order[i] = 0
		case i < transfers+bankSends:
			order[i] = 1
		default:
			order[i] = 2
		}
	}
	rand.New(rand.NewSource(5162)).Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	var nonce uint64
	var counts [3]int
	txs := make([][]byte, 0, total)
	transactions := make([]*ethtypes.Transaction, total)
	acceptedResults := make([]*abci.ExecTxResult, 0, total)
	var head [32]byte
	for i, kind := range order {
		var tx []byte
		var signed *ethtypes.Transaction
		switch kind {
		case 0:
			signed, tx = transfer(t, sender, nonce, recipient.EvmAddress, big.NewInt(onePaxInWei))
			nonce++
		case 1:
			tx = signCosmos(t, h.openCtx(), h, payer, 200_000,
				banktypes.NewMsgSend(payer.AccountAddress, recipient.AccountAddress,
					sdk.NewCoins(sdk.NewInt64Coin("uhpx", 7))))
		case 2:
			signed, tx = evmTx(t, sender, receiptLegacyTx(nonce, &contract, big.NewInt(0), callGas, nil))
			nonce++
		}
		id := sha256.Sum256(tx)
		if signed != nil {
			id = signed.Hash()
		}
		accepted, err := exec.Accept(tx)
		require.NoError(t, err, "operation %d, kind %d", i, kind)
		require.NotNil(t, accepted)
		require.Zero(t, accepted.Result.Code, accepted.Result.Log)
		require.Positive(t, accepted.Result.GasUsed)
		require.Equal(t, uint64(i+1), accepted.Seq)
		require.Equal(t, i+1, accepted.TxIndex)
		require.Equal(t, id, accepted.ActivityID)
		require.Equal(t, head, accepted.PrevSeqHash)
		head = expectedSeqHash(head, accepted.Seq, id)
		require.Equal(t, head, accepted.SeqHash)
		counts[kind]++
		txs = append(txs, tx)
		transactions[i] = signed
		acceptedResults = append(acceptedResults, accepted.Result)
	}
	require.Equal(t, [3]int{transfers, bankSends, calls}, counts)
	batch, err := exec.Seal(openHeight)
	require.NoError(t, err)
	require.Equal(t, txs, batch.Txs)
	require.Equal(t, uint64(total), batch.LastSeq)
	require.Equal(t, head, batch.LastSeqHash)
	sealed, tx0, err := sequencer.SealBatch(primary.App, batch,
		signSealBatch(t, sequencer.BatchFields(batch), testKeySeed, testKeyID))
	require.NoError(t, err)
	require.Len(t, sealed.Txs, total+1)
	require.Len(t, sealed.Results, total+1)
	require.Equal(t, tx0, sealed.Txs[0])
	require.Equal(t, txs, sealed.Txs[1:])
	require.Equal(t, acceptedResults, sealed.Results[1:])
	require.Equal(t, abci.ExecTxResult{}, *sealed.Results[0])

	ms := replay.App.CommitMultiStore().CacheMultiStore()
	ctx := sdk.NewContext(ms, batch.Ctx.BlockHeader(), false)
	ctx = ctx.WithConsensusParams(replay.App.GetConsensusParams(ctx))
	events, results, end, err := replay.App.ProcessBlock(ctx, sealed.Txs, &app.BlockProcessRequest{
		Height: openHeight, Time: blockTime,
	}, abci.CommitInfo{}, false, nil)
	require.NoError(t, err)
	require.Equal(t, sealed.Results, results)
	require.Equal(t, sealed.Events, events)
	require.Equal(t, sealed.EndBlock, end)
	requireStoredSequence(t, primary.App, sealed.Context(), total, head)
	requireStoredSequence(t, replay.App, ctx, total, head)
	var evmReceipts, callReceipts int
	for i, signed := range transactions {
		if signed == nil {
			require.Nil(t, results[i+1].EvmTxInfo)
			continue
		}
		want, err := primary.App.EvmKeeper.GetTransientReceipt(sealed.Context(), signed.Hash(), uint64(i+1))
		require.NoError(t, err)
		got, err := replay.App.EvmKeeper.GetTransientReceipt(ctx, signed.Hash(), uint64(i+1))
		require.NoError(t, err)
		require.NotNil(t, want)
		require.NotNil(t, got)
		require.Equal(t, want, got, "operation %d receipt", i)
		require.Equal(t, uint32(1), got.Status)
		evmReceipts++
		if order[i] == 2 {
			callReceipts++
			require.Len(t, got.Logs, 1)
			require.Equal(t, contract.Hex(), got.Logs[0].Address)
			require.Equal(t, common.BigToHash(big.NewInt(int64(callReceipts))).Bytes(), got.Logs[0].Data)
		} else {
			require.Empty(t, got.Logs)
		}
	}
	require.Equal(t, transfers+calls, evmReceipts)
	require.Equal(t, calls, callReceipts)
	for _, state := range []struct {
		a   *app.App
		ctx sdk.Context
	}{{primary.App, sealed.Context()}, {replay.App, ctx}} {
		require.Equal(t, common.BigToHash(big.NewInt(calls)), state.a.EvmKeeper.GetState(state.ctx, contract, common.Hash{}))
		require.Equal(t, uint64(transfers+calls), state.a.EvmKeeper.GetNonce(state.ctx, sender.EvmAddress))
		account := state.a.AccountKeeper.GetAccount(state.ctx, payer.AccountAddress)
		require.Equal(t, uint64(bankSends), account.GetSequence())
		wantBalance := sdk.NewInt(fundingUhpx).Add(sdk.NewInt(transfers * 1_000_000)).AddRaw(bankSends * 7)
		require.Equal(t, wantBalance, state.a.BankKeeper.GetBalance(state.ctx, recipient.AccountAddress, "uhpx").Amount)
	}
	require.Equal(t, initialHash, primary.App.CommitMultiStore().LastCommitID().Hash)
	require.Equal(t, initialHash, replay.App.CommitMultiStore().LastCommitID().Hash)
	batch.Store.Write()
	ms.Write()
	sealedHash := primary.App.GetWorkingHash()
	replayedHash := replay.App.GetWorkingHash()
	require.Len(t, sealedHash, 32)
	require.NotEqual(t, initialHash, sealedHash)
	require.Equal(t, sealedHash, replayedHash)
}
