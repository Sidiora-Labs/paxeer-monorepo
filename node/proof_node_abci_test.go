// Proves: app.App and sequencer.ConsensusSource -> exported ABCI methods and NewBlockProcessRequest -> utils.ProcessBlockDirect and sequencer.SealedLayer consumers.
//
// The batch comes from Executor.Accept and ConsensusSource.Seal, including its
// signature and transaction root. Independent application branches execute it
// twice through the OCC consumer, as the engine and fee-collection callers do;
// their results are compared with the real sequencer's sealed execution. ABCI
// proposal/finalize/commit then publish that state. The producer's application
// databases are reopened only after the producing application has stopped.
//
// Repairs node/abci.go App.CheckTx: a malformed nil request must return an ABCI
// error rather than panic. NewBlockProcessRequest also preserves finalize metadata
// for all six direct callers, including the OCC helpers that dropped block time.
// EVM signing derives chain 125 from the application configuration, including
// the unmapped sequenced test chain, without overriding the production mapping.
// node/leader_source_test.go commitLeaderSourceBlock supplies no batch after
// activation; the proof preserves the resulting ErrInvalidSequenceBatch refusal.
// Outside scope: sdk/storev2/rootmulti/store.go Store.Restore publishes the
// committed height and hash without exposing the restored EVM nonce to queries
// within five seconds. Restart recovery here reopens the actual producer stores;
// it must expose the committed state within the same five-second bound.
// Corrupt batch bytes, signatures and committed leader data must be refused by
// their real decoders, without substituting a transport or application layer.
package app_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestProofNodeAbci(t *testing.T) {
	require.Equal(t, int64(125), config.GetEVMChainID(sequencedChainID).Int64())
	oldOCC := app.EnableOCC
	t.Cleanup(func() { app.EnableOCC = oldOCC })
	// Keep the application's directories alive across the producer subtest's
	// cleanup, which closes its databases before the recovery consumer opens.
	w := newSequencerWrapper(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), false)
	upgradeInfo, err := w.App.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	home := filepath.Dir(filepath.Dir(upgradeInfo))
	var committedHash []byte
	var sender, recipient common.Address
	var lastHash [32]byte

	// The subtest's cleanup closes the producer before the recovery consumer opens.
	require.True(t, t.Run("producer to exported surface to consumers", func(t *testing.T) {
		h := newProofNodeAbciProducer(t, w)
		a := h.w.App
		require.NotNil(t, h.source, "node/sequencer.ConsensusSource producer is missing")
		require.NotNil(t, h.sealed, "node/sequencer.ConsensusSource.Seal produced no layer")
		require.Len(t, h.sealed.Txs, 2)
		sender, recipient = h.sender.EvmAddress, h.recipient
		batch, err := app.DecodeSequenceBatch(h.sealed.Txs[0])
		require.NoError(t, err)
		require.Equal(t, uint64(1), batch.ToSeq)
		copy(lastHash[:], batch.LastSeqHash)

		// CheckTx's real consumer sees an admissible user tx but never admits tx 0.
		for range 2 {
			checked := a.CheckTx(context.Background(), &abci.RequestCheckTxV2{
				Tx: h.sealed.Txs[1], Type: abci.CheckTxTypeV2New,
			})
			require.NotNil(t, checked.ResponseCheckTx)
			require.Zero(t, checked.Code, checked.Log)
			require.True(t, checked.IsEVM)
			require.Equal(t, sender, checked.EVMSenderAddress)
			refused := a.CheckTx(context.Background(), &abci.RequestCheckTxV2{Tx: h.sealed.Txs[0]})
			require.Equal(t, seqtypes.ModuleName, refused.Codespace)
			require.Equal(t, seqkeeper.ErrBatchBlockOnly.ABCICode(), refused.Code)
		}
		refused := a.CheckTx(context.Background(), nil)
		require.NotNil(t, refused.ResponseCheckTx)
		require.Equal(t, sdkerrors.ErrInvalidRequest.ABCICode(), refused.Code)
		require.Contains(t, refused.Log, "nil CheckTx request")

		fresh := func() sdk.Context {
			ctx := sdk.NewContext(a.CommitMultiStore().CacheMultiStore(), *h.req.Header, false)
			return ctx.WithConsensusParams(a.GetConsensusParams(ctx)).WithHeaderHash(h.req.Hash)
		}
		// BeginBlock consumes this height's votes and reads the previously stored votes.
		beginCtx := fresh()
		a.BeginBlock(beginCtx, 2, h.votes, nil, true)
		votes, err := a.SequencerKeeper.StoredVotes(beginCtx, 2)
		require.NoError(t, err)
		require.Equal(t, h.votes, votes)

		// Each call starts from committed height 1, so no prior execution can supply
		// the result. Compare against the independently produced sealed execution.
		var previous []*abci.ExecTxResult
		for range 2 {
			ctx := fresh()
			consumer := &utils.TestContext{Ctx: ctx, TestApp: a}
			events, results, end, err := utils.ProcessBlockDirect(consumer, h.sealed.Txs, false)
			require.NoError(t, err)
			require.Equal(t, h.sealed.Results, results)
			require.Equal(t, h.sealed.Events, events)
			require.Equal(t, h.sealed.EndBlock, end)
			if previous != nil {
				require.NotSame(t, previous[1], results[1])
			}
			previous = results
			require.Equal(t, uint64(1), a.EvmKeeper.GetNonce(ctx, sender))
			requireSeqCtxWei(t, big.NewInt(1), seqCtxWei(a, ctx, recipient))
			seq, err := a.SequencerKeeper.LastSeq(ctx)
			require.NoError(t, err)
			require.Equal(t, uint64(1), seq)
		}
		require.Zero(t, a.EvmKeeper.GetNonce(a.GetCheckCtx(), sender))

		// Malformed producer output reaches the actual batch decoder.
		broken := bytes.Clone(h.sealed.Txs[0][:len(h.sealed.Txs[0])-1])
		_, err = app.DecodeSequenceBatch(broken)
		require.Error(t, err)
		consumer := &utils.TestContext{Ctx: fresh(), TestApp: a}
		_, results, _, err := utils.ProcessBlockDirect(consumer, [][]byte{broken, h.sealed.Txs[1]}, false)
		require.Error(t, err)
		require.Nil(t, results)
		consumer.Ctx = fresh()
		_, results, _, err = utils.ProcessBlockDirect(consumer, nil, false)
		require.ErrorIs(t, err, seqtypes.ErrInvalidSequenceBatch)
		require.Nil(t, results)

		// A well-encoded producer result with a damaged signature passes decoding
		// and is refused by delivery, preserving both the fatal sentinel and ABCI code.
		batch.Sig = bytes.Clone(batch.Sig)
		batch.Sig[0] ^= 1
		wrongSigner, err := app.EncodeSequenceBatch(batch)
		require.NoError(t, err)
		consumer.Ctx = fresh()
		_, results, _, err = utils.ProcessBlockDirect(consumer, [][]byte{wrongSigner, h.sealed.Txs[1]}, false)
		require.ErrorIs(t, err, app.ErrSequencedBlockHalt)
		require.ErrorIs(t, err, seqtypes.ErrBadSignature)
		require.Nil(t, results)
		space, code, _ := sdkerrors.ABCIInfo(err, false)
		require.Equal(t, seqtypes.ModuleName, space)
		require.Equal(t, seqtypes.ErrBadSignature.ABCICode(), code)
		seq, err := a.SequencerKeeper.LastSeq(consumer.Ctx)
		require.NoError(t, err)
		require.Zero(t, seq)

		proposalHeader := *h.req.Header
		proposalHeader.ProposerAddress = crypto.AddressHash(sequencerLeader)
		proposal := &abci.RequestProcessProposal{
			Header: &proposalHeader, Hash: h.req.Hash, Txs: h.sealed.Txs,
			ProposedLastCommit: h.req.DecidedLastCommit,
		}
		proposal.Txs = [][]byte{broken, h.sealed.Txs[1]}
		rejected, err := a.ProcessProposal(context.Background(), proposal)
		require.NoError(t, err)
		require.Equal(t, abci.ResponseProcessProposal_REJECT, rejected.Status)
		proposal.Txs = h.sealed.Txs
		proposalHeader.ProposerAddress = bytes.Repeat([]byte{0xff}, 20)
		rejected, err = a.ProcessProposal(context.Background(), proposal)
		require.NoError(t, err)
		require.Equal(t, abci.ResponseProcessProposal_REJECT, rejected.Status)
		proposalHeader.ProposerAddress = crypto.AddressHash(sequencerLeader)
		accepted, err := a.ProcessProposal(context.Background(), proposal)
		require.NoError(t, err)
		require.Equal(t, abci.ResponseProcessProposal_ACCEPT, accepted.Status)
		info := a.GetOptimisticProcessingInfo()
		require.NotNil(t, info.Completion)
		select {
		case <-info.Completion:
		case <-time.After(30 * time.Second):
			t.Fatal("node/app.go App.ProcessProposalHandler producer did not finish")
		}
		info = a.GetOptimisticProcessingInfo()
		require.False(t, info.Aborted)
		require.Equal(t, h.sealed.Results, info.TxRes)

		// BaseApp.FinalizeBlock invokes the real registered App.FinalizeBlocker.
		finalized, err := a.FinalizeBlock(context.Background(), h.req)
		require.NoError(t, err)
		require.Equal(t, h.sealed.Results, finalized.TxResults)
		require.Same(t, h.sealed.Results[1], finalized.TxResults[1])
		require.NotEmpty(t, finalized.AppHash)
		_, err = a.Commit(context.Background())
		require.NoError(t, err)
		require.Equal(t, int64(2), a.LastBlockHeight())
		require.Equal(t, finalized.AppHash, a.LastCommitID().Hash)
		committedHash = bytes.Clone(finalized.AppHash)
	}))

	// New loads the same persisted SC and query databases, just as node startup
	// does. The BaseApp metadata DB is in memory in NewTestWrapper; application
	// state and its committed version belong to the persisted multistore.
	reopened := app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, home, 1, false,
		tmconfig.TestConfig(), app.MakeEncodingConfig(), nil, app.TestAppOpts{}, app.EmptyWasmOpts, nil,
		func(ba *baseapp.BaseApp) { ba.SetOccEnabled(false); ba.SetConcurrencyWorkers(4) })
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	require.Equal(t, int64(2), reopened.LastBlockHeight())
	require.Equal(t, committedHash, reopened.LastCommitID().Hash)
	// Create a new query branch on each attempt so an early missing value cannot
	// remain cached in the context used for the recovery assertions below.
	require.Eventually(t, func() bool {
		ctx, err := reopened.CreateQueryContext(0, false)
		return err == nil && reopened.EvmKeeper.GetNonce(ctx, sender) == 1
	}, 5*time.Second, 10*time.Millisecond,
		"node App.New did not recover the committed nonce from its query store")
	for range 2 {
		ctx, err := reopened.CreateQueryContext(0, false)
		require.NoError(t, err)
		require.Equal(t, uint64(1), reopened.EvmKeeper.GetNonce(ctx, sender))
		requireSeqCtxWei(t, big.NewInt(1), seqCtxWei(reopened, ctx, recipient))
		seq, err := reopened.SequencerKeeper.LastSeq(ctx)
		require.NoError(t, err)
		require.Equal(t, uint64(1), seq)
		storedHash, err := reopened.SequencerKeeper.LastSeqHash(ctx)
		require.NoError(t, err)
		require.Equal(t, lastHash, storedHash)
		var source tmtypes.LeaderSource = reopened
		activation, leaders, err := source.LeaderSchedule(3)
		require.NoError(t, err)
		require.Equal(t, int64(2), activation)
		require.Equal(t, [][32]byte{[32]byte(sequencerLeader)}, leaders)
	}
}

func TestProofNodeAbciLeaderSchedule(t *testing.T) {
	w := app.NewTestWrapper(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a := w.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	require.NoError(t, a.SequencerKeeper.SetReceiptKeys(w.Ctx, seqkeeper.ReceiptKeys{{
		KeyId: sequencerKeyID, PublicKey: sequencerPublicKey(), FirstHeight: 1, LastHeight: ^uint64(0),
	}}))
	var consumer tmtypes.LeaderSource = a
	want := [][32]byte{{1}, {2}}
	commitProofNodeAbciSchedule(t, a, func(ctx sdk.Context) {
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, 2))
		require.NoError(t, a.SequencerKeeper.SetLeaders(ctx, want))
	})
	for range 2 {
		activation, leaders, err := consumer.LeaderSchedule(2)
		require.NoError(t, err)
		require.Equal(t, int64(2), activation)
		require.Equal(t, want, leaders)
		leaders[0][0] ^= 0xff
	}
	// A new commit must be observed by the same interface, without a cached list.
	want = [][32]byte{{3}}
	commitProofNodeAbciSchedule(t, a, func(ctx sdk.Context) {
		require.NoError(t, a.SequencerKeeper.SetLeaders(ctx, want))
	})
	for range 2 {
		_, leaders, err := consumer.LeaderSchedule(3)
		require.NoError(t, err)
		require.Equal(t, want, leaders)
	}
	commitProofNodeAbciSchedule(t, a, func(ctx sdk.Context) {
		ctx.KVStore(a.GetKey(seqtypes.StoreKey)).Set(seqtypes.LeaderListKey, []byte{1})
	})
	for range 2 {
		activation, leaders, err := consumer.LeaderSchedule(4)
		require.ErrorIs(t, err, seqkeeper.ErrCorruptStore)
		require.Zero(t, activation)
		require.Nil(t, leaders)
	}
}

func TestProofNodeAbciRequestMetadata(t *testing.T) {
	req := &abci.RequestFinalizeBlock{
		Hash:                []byte("decided block"),
		Header:              &tmproto.Header{Height: 7, Time: time.Unix(1_800_000_007, 0).UTC()},
		ByzantineValidators: []abci.Misbehavior{{Height: 6, TotalVotingPower: 10}},
	}
	for range 2 {
		got := app.NewBlockProcessRequest(req)
		require.Equal(t, req.Hash, got.Hash)
		require.Equal(t, req.Header.Height, got.Height)
		require.Equal(t, req.Header.Time, got.Time)
		require.Equal(t, req.ByzantineValidators, got.ByzantineValidators)
	}
}

func newProofNodeAbciProducer(t *testing.T, w *app.TestWrapper) *finalizeSealedHarness {
	t.Helper()
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	require.NoError(t, w.App.SequencerKeeper.SetActivationHeight(w.Ctx, 2))
	activate(t, w)
	params := w.App.GetConsensusParams(w.Ctx)
	params.Block.MaxGasWanted = params.Block.MaxGas
	w.App.StoreConsensusParams(w.Ctx, params)
	sender := utils.NewSigner()
	recipient := utils.NewSigner().EvmAddress
	w.App.EvmKeeper.SetAddressMapping(w.Ctx, sender.AccountAddress, sender.EvmAddress)
	w.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", seqCtxFundingUhpx)))
	validators := w.App.StakingKeeper.GetAllValidators(w.Ctx)
	require.Len(t, validators, 1)
	consAddr, err := validators[0].GetConsAddr()
	require.NoError(t, err)
	votes := []abci.VoteInfo{{Validator: abci.Validator{Address: consAddr, Power: 100}, SignedLastBlock: true}}
	require.NoError(t, w.App.SequencerKeeper.StoreVotes(w.Ctx, 1, votes))
	// Keep the inactive-domain refusal independent of the root chain default.
	chainsGenesis := chainstypes.DefaultGenesis()
	chainsGenesis.Domains = []chainstypes.Domain{{
		Id: 7, Name: "paused-proof", Status: chainstypes.StatusPaused,
		GasAllowance: 1_000_000, ByteAllowance: 200_000,
		TargetGasPerBlock: 500_000, Bond: sdk.ZeroInt(),
	}}
	chainsGenesis.NextID = 8
	chainsGenesis.NextBaseFees[7] = sdk.ZeroDec()
	require.NoError(t, w.App.LayerXChainsKeeper.InitGenesis(w.Ctx, *chainsGenesis))
	commitBeforeSequencedProposal(t, w)
	exec, err := sequencer.NewExecutor(w.App, sequencer.Options{ChainID: sequencedChainID, ProposerAddress: consAddr})
	require.NoError(t, err)
	dir := t.TempDir()
	consensusKey := filepath.Join(dir, "consensus.json")
	pv, err := privval.GenFilePV(consensusKey, filepath.Join(dir, "consensus-state.json"), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	receiptKey := filepath.Join(dir, "receipt-key")
	require.NoError(t, os.WriteFile(receiptKey, []byte(hex.EncodeToString(sequencerKeySeed)), 0o600))
	pins, err := w.App.SequencerKeeper.ReceiptKeyPins(w.App.GetCheckCtx())
	require.NoError(t, err)
	key, err := sequencer.LoadReceiptKey(receiptKey, consensusKey, pins, 2)
	require.NoError(t, err)
	source, err := sequencer.NewConsensusSource(w.App, exec, key)
	require.NoError(t, err)
	w.App.SetSequencerSource(source)
	at := w.Ctx.BlockTime().Add(2 * time.Second).Truncate(time.Millisecond)
	require.NoError(t, exec.Open(2, at.UnixMilli()))
	encodeTransfer := func(chainID *big.Int) []byte {
		signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce: 0, GasPrice: big.NewInt(seqCtxGasPrice), Gas: seqCtxTransferGas,
			To: &recipient, Value: big.NewInt(1),
		}), ethtypes.LatestSignerForChainID(chainID), sender.EvmPrivateKey)
		require.NoError(t, err)
		data, err := ethtx.NewTxDataFromTx(signed)
		require.NoError(t, err)
		msg, err := evmtypes.NewMsgEVMTransaction(data)
		require.NoError(t, err)
		txConfig := w.App.GetTxConfig()
		builder := txConfig.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		builder.SetGasLimit(seqCtxTransferGas)
		tx, err := txConfig.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		return tx
	}
	refused, err := exec.Accept(encodeTransfer(chainstypes.ChainIDOf(7)))
	require.Nil(t, refused)
	require.ErrorIs(t, err, sequencer.ErrNotAdmitted)
	require.ErrorIs(t, err, chainstypes.ErrDomainNotActive)
	tx := encodeTransfer(w.App.EvmKeeper.ChainID(w.Ctx))
	accepted, err := exec.Accept(tx)
	require.NoError(t, err)
	require.Zero(t, accepted.Result.Code, accepted.Result.Log)
	_, err = source.Seal(context.Background(), 2)
	require.NoError(t, err)
	sealed := source.Sealed()
	require.NotNil(t, sealed)
	return &finalizeSealedHarness{
		w: w, source: source, sealed: sealed, sender: sender, recipient: recipient, votes: votes,
		req: &abci.RequestFinalizeBlock{
			Hash: []byte(t.Name()), Txs: sealed.Txs,
			Header:            &tmproto.Header{ChainID: sequencedChainID, Height: 2, Time: at, ProposerAddress: consAddr},
			DecidedLastCommit: abci.CommitInfo{Votes: votes},
		},
	}
}

// commitProofNodeAbciSchedule produces tx 0 for every sequenced height instead
// of relying on leader_source_test.go commitLeaderSourceBlock's empty legacy block.
func commitProofNodeAbciSchedule(t *testing.T, a *app.App, change func(sdk.Context)) {
	t.Helper()
	height := a.LastBlockHeight() + 1
	header := &tmproto.Header{ChainID: sequencedChainID, Height: height, Time: time.Unix(1_800_000_000+height, 0).UTC()}
	var txs [][]byte
	if a.Sequenced(a.GetCheckCtx(), height) {
		validators := a.StakingKeeper.GetAllValidators(a.GetCheckCtx())
		require.Len(t, validators, 1)
		proposer, err := validators[0].GetConsAddr()
		require.NoError(t, err)
		header.ProposerAddress = proposer
		exec, err := sequencer.NewExecutor(a, sequencer.Options{ChainID: sequencedChainID, ProposerAddress: proposer})
		require.NoError(t, err)
		require.NoError(t, exec.Open(height, header.Time.UnixMilli()))
		batch, err := exec.Seal(height)
		require.NoError(t, err)
		fields := sequencer.BatchFields(batch)
		encoded := signedBatchAt(t, fields.Height, sequencerKeySeed, fields.FromSeq, fields.ToSeq,
			fields.PrevSeqHash[:], fields.LastSeqHash[:], fields.TxRoot[:], sequencerKeyID)
		msg, err := app.DecodeSequenceBatch(encoded)
		require.NoError(t, err)
		sealed, tx0, err := sequencer.SealBatch(a, batch, msg)
		require.NoError(t, err)
		require.Equal(t, sealed.Txs[0], tx0)
		txs = sealed.Txs
	}
	_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Header: header, Txs: txs})
	require.NoError(t, err)
	change(a.GetContextForDeliverTx(nil))
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, height, a.LastBlockHeight())
}
