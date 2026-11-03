// Proves: NewExecutor/NewFeeFloor/NewReceipter -> SendAPI, TCP Replicator/Ack and WAL -> StateAPI, BlockAPI, FilterAPI, AppReplayer and ReceiptSignature.
//
// ConfigTemplate is read by ReadConfig, and LoadReceiptKey authenticates the
// signer used by the real Node. No executor, signer, WAL or replication link is
// replaced. The producer is stopped and its WAL reopened before pending state
// is read again through the same JSON-RPC consumer.
//
// Repairs node/sequencer/receipt.go, Receipter.after: Cosmos gas must contribute
// to cumulative gas, including the following EVM transaction's receipt.
// Node.Start restores receipt lookups alongside recovered state and byte-identical
// replay receipts, including sends retained below the committed WAL watermark.
package sequencer_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmutils "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// proofExecutorConsensus hosts the parent-block producer needed by BlockAPI.
func proofExecutorConsensus(t *testing.T) *local.Local {
	t.Helper()
	cfg, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	a := kvstore.NewApplication()
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	service, closeConsensus, err := rpctest.StartTendermint(t.Context(), cfg, a, rpctest.SuppressStdout)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, closeConsensus(context.Background())) })
	node, ok := service.(local.NodeService)
	require.True(t, ok)
	consensus, err := local.New(node)
	require.NoError(t, err)
	proofExecutorWaitParent(t, consensus)
	return consensus
}

func proofExecutorWaitParent(t *testing.T, consensus *local.Local) {
	t.Helper()
	height := int64(committedTip)
	require.Eventually(t, func() bool {
		block, err := consensus.Block(t.Context(), &height)
		return err == nil && block != nil && block.Block != nil && block.Block.Height == height
	}, 5*time.Second, 10*time.Millisecond, "consensus/rpc/test: local producer did not commit the pending block's parent")
}

// proofExecutorRPC registers the production APIs on the real JSON-RPC server.
func proofExecutorRPC(t *testing.T, f *startHarness, consensus *local.Local) *gethrpc.Client {
	t.Helper()
	ctxProvider := f.app.RPCContextProvider
	wm := evmrpc.NewWatermarkManager(nil, ctxProvider, nil, nil)
	wm.SetPendingSnapshot(f.n.Executor)
	option := evmrpc.NewSequencerOption(f.n.Accepter, f.n.ReceiptLogFeed)
	cfg, err := option.SendConfig(evmrpcconfig.DefaultConfig)
	require.NoError(t, err)
	cfg.SequencerSource = f.n.ReceiptSource
	send := evmrpc.NewSendAPI(nil, func(int64) client.TxConfig { return f.app.GetTxConfig() }, cfg,
		&f.app.EvmKeeper, f.app.BeginBlockKeepers, ctxProvider, "", nil,
		f.app.BaseApp, f.app.TracerAnteHandler, evmrpc.ConnectionTypeHTTP,
		tmutils.None[time.Duration](), nil, nil, wm)
	state := evmrpc.NewStateAPI(nil, &f.app.EvmKeeper, ctxProvider, evmrpc.ConnectionTypeHTTP, wm, nil)
	server := gethrpc.NewServer()
	require.NoError(t, server.RegisterName("eth", send))
	require.NoError(t, server.RegisterName("eth", state))
	block := evmrpc.NewBlockAPI(consensus, &f.app.EvmKeeper, ctxProvider,
		func(int64) client.TxConfig { return f.app.GetTxConfig() }, evmrpc.ConnectionTypeHTTP, wm, nil, nil)
	require.NoError(t, server.RegisterName("eth", block))
	httpServer := httptest.NewServer(server)
	t.Cleanup(func() { httpServer.Close(); server.Stop() })
	rpcClient, err := gethrpc.DialContext(t.Context(), httpServer.URL)
	require.NoError(t, err)
	t.Cleanup(rpcClient.Close)
	return rpcClient
}

func proofExecutorReceiptRPC(t *testing.T, n *sequencer.Node, consensus *local.Local) *gethrpc.Client {
	t.Helper()
	cfg := evmrpcconfig.DefaultConfig
	cfg.HTTPAddress, cfg.HTTPPort = "127.0.0.1", rpcE2EPort(t)
	server, err := evmrpc.NewEVMHTTPServer(cfg, consensus, &n.App.EvmKeeper, n.App.BeginBlockKeepers,
		n.App.BaseApp, n.App.TracerAnteHandler, n.App.RPCContextProvider,
		func(int64) client.TxConfig { return n.App.GetTxConfig() }, "", n.App.GetStateStore(),
		evmrpc.SequencerReceiptOption{Source: n.ReceiptSource, Keeper: &n.App.SequencerKeeper})
	require.NoError(t, err)
	require.NoError(t, server.Start())
	t.Cleanup(server.Stop)
	c, err := gethrpc.DialHTTP(fmt.Sprintf("http://127.0.0.1:%d", cfg.HTTPPort))
	require.NoError(t, err)
	t.Cleanup(c.Close)
	return c
}

func TestProofSequencerExecutorTransportRecovery(t *testing.T) {
	sender, recipient := utils.NewSigner(), utils.NewSigner()
	f := newStartHarness(t, sequencer.ModeSequential, sender)

	// Read the actual shipped template through the same options surface as startup.
	v := viper.New()
	v.SetConfigType("toml")
	require.NoError(t, v.ReadConfig(bytes.NewBufferString(sequencer.ConfigTemplate)))
	defaults, err := sequencer.ReadConfig(v)
	require.NoError(t, err)
	require.Equal(t, sequencer.DefaultConfig(), defaults)
	v.Set(sequencer.FlagEnable, true)
	v.Set(sequencer.FlagWALDir, f.cfg.WALDir)
	v.Set(sequencer.FlagReceiptKeyFile, f.cfg.ReceiptKeyFile)
	v.Set(sequencer.FlagStandbyAddress, f.cfg.StandbyAddress)
	v.Set(sequencer.FlagReplicationTimeout, f.cfg.ReplicationTimeout)
	v.Set(sequencer.FlagLeaseDuration, f.cfg.LeaseDuration)
	v.Set(sequencer.FlagNonceQueueMaxPerSender, f.cfg.NonceQueueMaxPerSender)
	v.Set(sequencer.FlagNonceQueueMaxTotal, f.cfg.NonceQueueMaxTotal)
	f.cfg, err = sequencer.ReadConfig(v)
	require.NoError(t, err)
	v.Set(sequencer.FlagMode, "unknown-executor")
	_, err = sequencer.ReadConfig(v)
	require.ErrorIs(t, err, sequencer.ErrConfigMode)
	sb := f.restart(t, nil)
	require.NoError(t, f.n.Start(t.Context()))
	consensus := proofExecutorConsensus(t)
	rpcClient := proofExecutorRPC(t, f, consensus)
	initial, err := f.app.PendingSnapshot()
	require.NoError(t, err)
	fee, err := f.n.FeeFloor.BaseFeePerGas()
	require.NoError(t, err)
	require.False(t, fee.IsNegative())
	pin, err := f.n.ReceiptKey.KeyID(openHeight)
	require.NoError(t, err)
	require.Equal(t, testKeyID, pin)
	_, err = f.n.ReceiptKey.KeyID(0)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyNotPinned)

	// Both transactions and receipts are emitted by the production send path.
	var receipts [][]byte
	var txs [][]byte
	var hashes []common.Hash
	prev := sourcePrevSeqHash
	for nonce := uint64(0); nonce < 2; nonce++ {
		if nonce == 1 {
			// A fresh TCP link resends the retained OPEN and first ACCEPT;
			// the standby must not execute either a second time.
			f.n.Replicator.Close()
		}
		signed, _ := transfer(t, sender, nonce, recipient.EvmAddress, big.NewInt(5))
		raw, err := signed.MarshalBinary()
		require.NoError(t, err)
		var hash common.Hash
		require.NoError(t, rpcClient.CallContext(t.Context(), &hash, "eth_sendRawTransaction", hexutil.Bytes(raw)))
		require.Equal(t, signed.Hash(), hash)
		send, err := f.n.ReceiptSource.SequencerSend(hash)
		require.NoError(t, err)
		require.NotNil(t, send)
		verified, err := verify.ReceiptSignature(send.Receipt, f.n.ReceiptKey.PublicKey())
		require.NoError(t, err)
		rec := verified.Receipt
		require.Equal(t, sourceLastSeq+nonce+1, rec.GlobalSequence)
		require.Equal(t, prev, rec.PreviousStateRoot)
		require.Equal(t, [32]byte(hash), rec.ActivityID)
		// Independent encoding pins seqHashDomain, not a producer helper's answer.
		hashInput := append([]byte("PAXEER/v1/sequence\x00"), prev[:]...)
		hashInput = binary.BigEndian.AppendUint64(hashInput, rec.GlobalSequence)
		hashInput = append(hashInput, hash[:]...)
		require.Equal(t, sha256.Sum256(hashInput), rec.ResultingStateRoot)
		require.Equal(t, uint64((nonce+1)*transferGas), binary.BigEndian.Uint64(rec.Effects[0].Body[8:16]))
		require.Equal(t, testKeyID, rec.ContextHash)
		_, err = verify.ReceiptSignature(send.Receipt, publicOf(consensusKeySeed))
		require.Error(t, err, "an unrelated signer must not authenticate the receipt")
		pending, ok := f.n.Executor.PendingTx(hash)
		require.True(t, ok)
		require.Equal(t, openHeight, pending.Height)
		require.Equal(t, int(nonce+1), pending.TxIndex)
		require.Zero(t, pending.Result.Code, pending.Result.Log)
		txs = append(txs, bytes.Clone(pending.TxBytes))
		receipts = append(receipts, bytes.Clone(send.Receipt))
		hashes = append(hashes, hash)
		prev = rec.ResultingStateRoot
	}
	require.Equal(t, sourceLastSeq+2, sb.LastSeq())
	require.NoError(t, sb.Err())
	// Ack traverses the real socket when the producer resends an existing frame.
	entries := driverEntries(t, f.cfg.WALDir)
	require.Len(t, entries, 3)
	require.IsType(t, &wal.Open{}, entries[0])
	for i := range txs {
		logged := entries[i+1].(*wal.Accept)
		require.Equal(t, txs[i], logged.Tx)
		require.Equal(t, receipts[i], logged.Receipt)
	}
	ack, err := f.n.Replicator.Release(t.Context())
	require.EqualError(t, err, "sequencer: empty release group")
	require.Equal(t, sequencer.Ack{}, ack)

	assertPending := func(c *gethrpc.Client) {
		t.Helper()
		var balance hexutil.Big
		require.NoError(t, c.CallContext(t.Context(), &balance, "eth_getBalance", recipient.EvmAddress, "pending"))
		require.Equal(t, 0, big.NewInt(10).Cmp(balance.ToInt()))
		snap, err := f.app.PendingSnapshot()
		require.NoError(t, err)
		require.Equal(t, initial.BlockTime(), snap.BlockTime())
		require.Equal(t, uint64(2), f.app.EvmKeeper.GetNonce(snap, sender.EvmAddress))
		require.Zero(t, f.app.EvmKeeper.GetNonce(f.app.GetCheckCtx(), sender.EvmAddress))
		var block struct {
			Number       hexutil.Uint64 `json:"number"`
			Transactions []common.Hash  `json:"transactions"`
		}
		require.NoError(t, c.CallContext(t.Context(), &block, "eth_getBlockByNumber", "pending", false))
		require.Equal(t, uint64(openHeight), uint64(block.Number))
		require.Equal(t, hashes, block.Transactions)
	}
	assertPending(rpcClient)
	beforeRestart := driverEntries(t, f.cfg.WALDir)
	committed := f.app.CommitMultiStore().LastCommitID()
	old := f.n
	old.Replicator.Close()
	old.WAL.Close()
	f.app.SetOpenBlock(nil)
	f.app.SetSequencerSource(nil)
	f.app.SetSequencerRPC(evmrpc.SequencerOption{}, evmrpc.SequencerReceiptOption{})
	f.app.SetCommitListener(nil)
	require.NoError(t, old.Executor.Fence())
	f.n, err = sequencer.NewNode(f.cfg, f.app)
	require.NoError(t, err)
	require.NotSame(t, old.Executor, f.n.Executor)
	require.NoError(t, f.n.Start(t.Context()))
	require.NotSame(t, old.ReceiptKey, f.n.ReceiptKey)
	require.Equal(t, old.ReceiptKey.PublicKey(), f.n.ReceiptKey.PublicKey())
	recoveredFee, err := f.n.FeeFloor.BaseFeePerGas()
	require.NoError(t, err)
	require.Equal(t, fee, recoveredFee)
	recoveredClient := proofExecutorRPC(t, f, consensus)
	recoveredReceipts := proofExecutorReceiptRPC(t, f.n, consensus)
	assertPending(recoveredClient)
	require.Equal(t, committed, f.app.CommitMultiStore().LastCommitID())
	require.Equal(t, beforeRestart, driverEntries(t, f.cfg.WALDir), "replay must not append producer records")
	for i, hash := range hashes {
		_, err := verify.ReceiptSignature(receipts[i], f.n.ReceiptKey.PublicKey())
		require.NoError(t, err, "the reloaded signer must authenticate persisted receipts")
		recovered, ok := f.n.Executor.PendingTx(hash)
		require.True(t, ok)
		require.Equal(t, txs[i], recovered.TxBytes)
		// Node.Start uses AppReplayer.Execute, which checks the regenerated receipt
		// byte for byte against the WAL before installing this recovered executor.
		require.Equal(t, int(i+1), recovered.TxIndex)
		restored, err := f.n.ReceiptSource.SequencerSend(hash)
		require.NoError(t, err)
		require.NotNil(t, restored)
		require.Equal(t, receipts[i], restored.Receipt)
		require.Equal(t, uint64(i), restored.Nonce)
		var consumed *evmrpc.SequencerReceiptResult
		require.NoError(t, recoveredReceipts.CallContext(t.Context(), &consumed, evmrpc.SequencerReceiptMethod, hash))
		require.NotNil(t, consumed)
		require.Equal(t, restored.Receipt, []byte(consumed.Receipt))
		require.Equal(t, evmrpc.RungReceipted, consumed.Rung)
		rpcE2EVerifyReceipt(t, f.app, consumed)
	}
	signed, _ := transfer(t, sender, 0, recipient.EvmAddress, big.NewInt(5))
	raw, err := signed.MarshalBinary()
	require.NoError(t, err)
	var duplicate common.Hash
	err = recoveredClient.CallContext(t.Context(), &duplicate, "eth_sendRawTransaction", hexutil.Bytes(raw))
	require.NoError(t, err)
	require.Equal(t, signed.Hash(), duplicate)
	conflict, _ := transfer(t, sender, 0, recipient.EvmAddress, big.NewInt(6))
	conflictRaw, err := conflict.MarshalBinary()
	require.NoError(t, err)
	err = recoveredClient.CallContext(t.Context(), &duplicate, "eth_sendRawTransaction", hexutil.Bytes(conflictRaw))
	require.ErrorContains(t, err, sdkerrors.ErrWrongSequence.Error())
	assertPending(recoveredClient)
	require.Equal(t, beforeRestart, driverEntries(t, f.cfg.WALDir))
	require.NoError(t, sb.Err())
	_, ok := f.n.Executor.PendingTx(common.HexToHash("0xff"))
	require.False(t, ok)
	var block any
	require.ErrorContains(t, recoveredClient.CallContext(t.Context(), &block, "eth_getBlockByNumber", "pending", true), "pending block lists transaction hashes only")
	var malformed common.Hash
	require.Error(t, recoveredClient.CallContext(t.Context(), &malformed, "eth_sendRawTransaction", hexutil.Bytes{0xff}))
	require.Equal(t, beforeRestart, driverEntries(t, f.cfg.WALDir))
	var proof any
	require.ErrorContains(t, recoveredClient.CallContext(t.Context(), &proof, "eth_getProof", recipient.EvmAddress, []string{}, "pending"), "pending state has no committed root")
}

func TestProofSequencerExecutorMixedCumulativeGas(t *testing.T) {
	sender, recipient, cosmos := utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
	rh := newReceiptHarness(t, 0, resolutionBlockGas, sender, cosmos)
	var cumulative uint64
	check := func(tx []byte, operation uint8) {
		t.Helper()
		acc, rec, signed := rh.accept(t, tx)
		require.Equal(t, operation, rec.Operation)
		require.Greater(t, acc.Result.GasUsed, int64(0))
		cumulative += uint64(acc.Result.GasUsed)
		require.Equal(t, cumulative, binary.BigEndian.Uint64(rec.Effects[0].Body[8:16]))
		require.Equal(t, signed, rec.CanonicalBytes())
	}
	_, first := transfer(t, sender, 0, recipient.EvmAddress, big.NewInt(5))
	check(first, ethtypes.LegacyTxType)
	for i := 0; i < 2; i++ {
		tx := signCosmos(t, rh.openCtx(), rh.harness, cosmos, 200_000,
			banktypes.NewMsgSend(cosmos.AccountAddress, recipient.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(5)))))
		check(tx, sequencer.CosmosOperation)
	}
	_, last := transfer(t, sender, 1, recipient.EvmAddress, big.NewInt(5))
	check(last, ethtypes.LegacyTxType)
	// Refusals must not add gas to the next successfully receipted transaction.
	_, _, err := rh.r.Accept(last)
	require.ErrorIs(t, err, sdkerrors.ErrWrongSequence)
	_, next := transfer(t, sender, 2, recipient.EvmAddress, big.NewInt(5))
	check(next, ethtypes.LegacyTxType)
	_, err = rh.exec.Seal(openHeight)
	require.NoError(t, err)
}

func TestProofSequencerExecutorRPCConsumers(t *testing.T) {
	// Configure the genesis block's gas-wanted budget before either real App
	// starts, as resolutionBlockGas does for the recovery proof's harness.
	previous := app.DefaultConsensusParams
	params, blockParams := *previous, *previous.Block
	blockParams.MaxGasWanted = blockParams.MaxGas
	params.Block = &blockParams
	app.DefaultConsensusParams = &params
	t.Cleanup(func() { app.DefaultConsensusParams = previous })
	h := newRPCE2EHarness(t)
	proofExecutorWaitParent(t, h.consensus)

	var filter gethrpc.ID
	require.NoError(t, h.client.CallContext(h.ctx, &filter, "eth_newPendingTransactionFilter"))
	changes := func() []common.Hash {
		t.Helper()
		var hashes []common.Hash
		require.NoError(t, h.client.CallContext(h.ctx, &hashes, "eth_getFilterChanges", filter))
		return hashes
	}
	require.Empty(t, changes())
	to := utils.NewSigner().EvmAddress
	first, receipt := h.send(t, 0, to, big.NewInt(5))
	rpcE2EVerifyReceipt(t, h.node.App, receipt)
	require.Equal(t, uint64(transferGas), uint64(receipt.GasUsed))
	require.Equal(t, uint64(transferGas), uint64(receipt.CumulativeGasUsed))

	// Emit an actual EVM log during creation, then consume it through the
	// production FilterAPI installed by RegisterLocalServices.
	topic := common.HexToHash("0xfeed")
	initCode := append(append([]byte{0x7f}, topic.Bytes()...), 0x60, 0x00, 0x60, 0x00, 0xa1, 0x00)
	deployment, _ := evmTx(t, h.sender, receiptLegacyTx(1, nil, new(big.Int), createGas, initCode))
	raw, err := deployment.MarshalBinary()
	require.NoError(t, err)
	var hash common.Hash
	require.NoError(t, h.client.CallContext(h.ctx, &hash, "eth_sendRawTransaction", hexutil.Bytes(raw)))
	require.Equal(t, deployment.Hash(), hash)
	var deployed *evmrpc.SequencerReceiptResult
	require.NoError(t, h.client.CallContext(h.ctx, &deployed, evmrpc.SequencerReceiptMethod, hash))
	require.NotNil(t, deployed)
	rpcE2EVerifyReceipt(t, h.node.App, deployed)
	require.Equal(t, sourceLastSeq+2, uint64(deployed.Seq))
	require.Equal(t, uint64(receipt.GasUsed)+uint64(deployed.GasUsed), uint64(deployed.CumulativeGasUsed))
	require.Len(t, deployed.Logs, 1)
	require.Equal(t, []common.Hash{topic}, deployed.Logs[0].Topics)
	require.Equal(t, []common.Hash{first.Hash(), hash}, changes())
	require.Empty(t, changes(), "a pending filter must drain each receipted hash once")

	var logs []struct {
		TxHash      common.Hash     `json:"transactionHash"`
		Address     common.Address  `json:"address"`
		Topics      []common.Hash   `json:"topics"`
		Seq         *hexutil.Uint64 `json:"seq"`
		BlockNumber hexutil.Uint64  `json:"blockNumber"`
		TxIndex     hexutil.Uint    `json:"transactionIndex"`
	}
	criteria := map[string]any{"fromBlock": "pending", "toBlock": "pending", "topics": []common.Hash{topic}}
	require.NoError(t, h.client.CallContext(h.ctx, &logs, "eth_getLogs", criteria))
	require.Len(t, logs, 1)
	require.Equal(t, hash, logs[0].TxHash)
	require.Equal(t, deployed.Logs[0].Address, logs[0].Address)
	require.Equal(t, []common.Hash{topic}, logs[0].Topics)
	require.NotNil(t, logs[0].Seq)
	require.Equal(t, uint64(deployed.Seq), uint64(*logs[0].Seq))
	require.Equal(t, uint64(openHeight), uint64(logs[0].BlockNumber))
	require.Equal(t, uint(deployed.Index), uint(logs[0].TxIndex))

	var block struct {
		Number       hexutil.Uint64 `json:"number"`
		Transactions []common.Hash  `json:"transactions"`
		Hash         *common.Hash   `json:"hash"`
		StateRoot    *common.Hash   `json:"stateRoot"`
	}
	require.NoError(t, h.client.CallContext(h.ctx, &block, "eth_getBlockByNumber", "pending", false))
	require.Equal(t, uint64(openHeight), uint64(block.Number))
	require.Equal(t, []common.Hash{first.Hash(), hash}, block.Transactions)
	require.Nil(t, block.Hash)
	require.Nil(t, block.StateRoot)
	var balance hexutil.Big
	require.NoError(t, h.client.CallContext(h.ctx, &balance, "eth_getBalance", to, "pending"))
	require.Equal(t, 0, big.NewInt(5).Cmp(balance.ToInt()))

	// The same JSON-RPC crossing returns the recorded hash on resubmission and refuses
	// an unsupported full transaction filter without publishing another pending hash.
	err = h.client.CallContext(h.ctx, &hash, "eth_sendRawTransaction", hexutil.Bytes(raw))
	require.NoError(t, err)
	require.Equal(t, deployment.Hash(), hash)
	var unsupported gethrpc.ID
	require.ErrorContains(t, h.client.CallContext(h.ctx, &unsupported, "eth_newPendingTransactionFilter", true), "serves transaction hashes only")
	require.Empty(t, changes())
	var unknown *evmrpc.SequencerReceiptResult
	require.NoError(t, h.client.CallContext(h.ctx, &unknown, evmrpc.SequencerReceiptMethod, common.HexToHash("0xff")))
	require.Nil(t, unknown)

	pending, err := h.node.Executor.Snapshot()
	require.NoError(t, err)
	sealed, err := h.node.ConsensusSource.Seal(h.ctx, openHeight)
	require.NoError(t, err)
	response, err := h.node.App.FinalizeBlock(h.ctx, &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: testChainID, Height: openHeight, Time: sealed.OpenTime, ProposerAddress: pending.BlockHeader().ProposerAddress},
		Txs:    append([][]byte{sealed.Batch}, sealed.Txs.ToSliceOfBytes()...),
	})
	require.NoError(t, err)
	require.Len(t, response.TxResults, 3)
	for _, result := range response.TxResults {
		require.Zero(t, result.Code, result.Log)
	}
	h.node.App.SetDeliverStateToCommit()
	_, err = h.node.App.Commit(h.ctx)
	require.NoError(t, err)
	commitID := h.node.App.CommitMultiStore().LastCommitID()
	tip := h.node.WAL.LastCommitted()
	require.NotNil(t, tip)
	require.Equal(t, uint64(openHeight), tip.Height)
	require.Equal(t, uint64(deployed.Seq), tip.LastSeq)
	require.NoError(t, h.node.Stop())
	h.node.App.SetOpenBlock(nil)
	h.node.App.SetSequencerSource(nil)
	h.node.App.SetSequencerRPC(evmrpc.SequencerOption{}, evmrpc.SequencerReceiptOption{})
	h.node.App.SetCommitListener(nil)
	h.node, err = sequencer.NewNode(h.node.Config, h.node.App)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, h.node.Stop()) })
	replay, err := h.node.WAL.Replay(sequencer.NewAppReplayer(h.node.Executor, h.node.Receipter))
	require.NoError(t, err)
	require.Equal(t, tip, replay.From)
	require.Zero(t, replay.Accepts)
	require.Nil(t, replay.LastAccept)
	require.NoError(t, h.node.Stop())
	h.node, err = sequencer.NewNode(h.node.Config, h.node.App)
	require.NoError(t, err)
	require.NoError(t, h.node.Start(h.ctx))
	require.Equal(t, commitID, h.node.App.CommitMultiStore().LastCommitID())
	require.Eventually(t, func() bool {
		height := int64(openHeight)
		block, err := h.consensus.Block(h.ctx, &height)
		return err == nil && block != nil && block.Block != nil && block.Block.Height == height
	}, 5*time.Second, 10*time.Millisecond)
	recovered := proofExecutorReceiptRPC(t, h.node, h.consensus)
	for _, original := range []*evmrpc.SequencerReceiptResult{receipt, deployed} {
		restored, err := h.node.ReceiptSource.SequencerSend(original.TxHash)
		require.NoError(t, err)
		require.NotNil(t, restored)
		require.Equal(t, []byte(original.Receipt), restored.Receipt)
		var consumed *evmrpc.SequencerReceiptResult
		require.NoError(t, recovered.CallContext(h.ctx, &consumed, evmrpc.SequencerReceiptMethod, original.TxHash))
		require.NotNil(t, consumed)
		require.Equal(t, evmrpc.RungCommitted, consumed.Rung)
		require.Equal(t, original.TxHash, consumed.TxHash)
		require.Equal(t, original.SequencerReceiptView, consumed.SequencerReceiptView)
		require.NotNil(t, consumed.BlockNumber)
		require.Equal(t, uint64(openHeight), uint64(*consumed.BlockNumber))
		rpcE2EVerifyReceipt(t, h.node.App, consumed)
	}
}
