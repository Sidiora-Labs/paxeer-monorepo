// Proves: sequencer.Receipter / NewAccepter -> Replicator / ReceiptSource / HTTP JSON-RPC -> SequencerReceiptAPI / SequencerReceiptResult.Verify.
// NewFollowerReceiptOption carries the real primary endpoint into the follower's
// RPC consumer, and NewLogFeedPublisher carries the same executed receipts into
// eth_getLogs. NewNode and NewAppReplayer supply real execution, WALs and standby
// acknowledgements; no receipt producer or transport is replaced.
// The receipt source and forwarding option have no persistent state of their own.
// Two fresh HTTP reads per result, followed by a malformed record and recovery,
// prove that consumers decode the current producer output without a cached view.
// ReceiptSource.Record checks the envelope; rpc/sequencer_receipt.go's
// sequencerReceiptView checks the execution digests. This proof expects malformed
// execution data to be refused there and truncated bytes to be refused by Verify.
// Repairs node/sequencer/accepter.go's Accepter.send: reloading the transient
// execution receipt lost the cumulative gas Receipter.after signed. AcceptedSend
// must carry that same cumulative gas to its consumers, including log publication.
package sequencer_test

import (
	"bytes"
	"crypto/sha256"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	bridgetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestProofSequencerReceipt(t *testing.T) {
	// Default genesis activates the root EVM chain (125). Configure the test
	// application and sign transactions for that real domain before node startup.
	const chainID = int64(125)
	admissionChainID(t, chainID)
	h := newRPCE2EHarness(t)
	n := h.node
	encode := func(data ethtypes.TxData) (*ethtypes.Transaction, []byte) {
		t.Helper()
		signed, err := ethtypes.SignNewTx(h.sender.EvmPrivateKey,
			ethtypes.LatestSignerForChainID(big.NewInt(chainID)), data)
		require.NoError(t, err)
		txData, err := ethtx.NewTxDataFromTx(signed)
		require.NoError(t, err)
		msg, err := evmtypes.NewMsgEVMTransaction(txData)
		require.NoError(t, err)
		txConfig := app.MakeEncodingConfig().TxConfig
		builder := txConfig.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		builder.SetGasLimit(signed.Gas())
		txBytes, err := txConfig.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		return signed, txBytes
	}
	// These are the exported constructors and interfaces used by node assembly.
	accepter := sequencer.NewAccepter(n.Receipter, n.Replicator, n.ReceiptSource, n.Metrics)
	publisher := sequencer.NewLogFeedPublisher(n.ReceiptLogFeed)
	var source evmrpc.SequencerReceiptSource = n.ReceiptSource
	var consumerAccepter evmrpc.SequencerAccepter = accepter

	port := rpcE2EPort(t)
	follower := newRPCE2EApp(t, h.sender, h.genesis, port)
	cfg := sequencer.DefaultConfig()
	cfg.ReceiptForwardURL = h.url
	option, enabled := sequencer.NewFollowerReceiptOption(cfg)
	require.True(t, enabled)
	require.Equal(t, h.url, option.ForwardURL.String())
	follower.SetSequencerRPC(evmrpc.SequencerOption{}, option)
	forwarded, _ := rpcE2EClient(t, h.ctx, follower, h.consensus, port)
	consumers := []*gethrpc.Client{h.client, forwarded}

	pub := n.ReceiptKey.PublicKey()
	keys := seqkeeper.ReceiptKeys{{
		KeyId: testKeyID[:], PublicKey: pub[:], FirstHeight: 1, LastHeight: 1000,
	}}
	read := func(client *gethrpc.Client, hash common.Hash) *evmrpc.SequencerReceiptResult {
		t.Helper()
		var result *evmrpc.SequencerReceiptResult
		require.NoError(t, client.CallContext(h.ctx, &result, evmrpc.SequencerReceiptMethod, hash))
		return result
	}
	assertUnknown := func(hash common.Hash) {
		t.Helper()
		for _, client := range consumers {
			require.Nil(t, read(client, hash))
		}
		got, err := source.SequencerSend(hash)
		require.NoError(t, err)
		require.Nil(t, got)
	}
	assertUnknown(common.HexToHash("0xdeadbeef"))
	refused, err := consumerAccepter.Accept(nil)
	require.Error(t, err)
	require.NotErrorIs(t, err, sequencer.ErrReplicationHalted)
	require.Equal(t, evmrpc.AcceptedSend{}, refused)

	contract := crypto.CreateAddress(h.sender.EvmAddress, 0)
	var lastHash common.Hash
	var lastSend *evmrpc.SequencerSend
	var lastResult *evmrpc.SequencerReceiptResult
	var previousLogs []publishedReceiptLog
	var cumulativeGasUsed uint64
	for nonce := uint64(0); nonce <= 2; nonce++ {
		data := receiptLegacyTx(nonce, &contract, big.NewInt(0), callGas, nil)
		if nonce == 0 {
			data = receiptLegacyTx(nonce, nil, big.NewInt(0), createGas, logInit)
		}
		tx, txBytes := encode(data)
		assertUnknown(tx.Hash())
		accepted, err := consumerAccepter.Accept(txBytes)
		require.NoError(t, err)
		require.Equal(t, openHeight, accepted.Height)
		require.NotNil(t, accepted.Exec)
		require.Equal(t, uint32(1), accepted.Exec.Status)
		cumulativeGasUsed += accepted.Exec.GasUsed
		require.Equal(t, cumulativeGasUsed, accepted.Exec.CumulativeGasUsed)
		decoded, err := codec.DecodeReceipt(accepted.Receipt)
		require.NoError(t, err)
		require.Equal(t, sourceLastSeq+nonce+1, decoded.GlobalSequence)
		require.Equal(t, [32]byte(tx.Hash()), decoded.ActivityID)
		require.Equal(t, decoded.GlobalSequence, h.standby.LastSeq())
		require.NoError(t, h.standby.Err())

		// The primary WAL and the real standby both observed this acceptance
		// before the receipt became visible to the RPC consumer.
		primaryEntries := accepterWALEntries(t, n.Config.WALDir)
		persisted, ok := primaryEntries[len(primaryEntries)-1].(*wal.Accept)
		require.True(t, ok)
		require.Equal(t, txBytes, persisted.Tx)
		require.Equal(t, accepted.Receipt, persisted.Receipt)
		require.Equal(t, decoded.GlobalSequence, persisted.Seq)
		require.Equal(t, uint64(accepted.Height), persisted.Height)

		send, err := source.SequencerSend(tx.Hash())
		require.NoError(t, err)
		require.NotNil(t, send)
		require.Equal(t, accepted.Receipt, send.Receipt)
		require.Equal(t, nonce, send.Nonce)
		require.NoError(t, publisher.Receipted(decoded.GlobalSequence, accepted.Height, tx.Hash(), accepted.Exec))
		logs := publishedLogs(t, h.client)
		if nonce == 0 {
			require.Empty(t, logs)
		} else {
			require.Len(t, accepted.Exec.Logs, 3)
			require.Len(t, logs, len(previousLogs)+3)
			require.Equal(t, previousLogs, logs[:len(previousLogs)])
			requirePublishedReceipt(t, logs[len(previousLogs):], decoded.GlobalSequence,
				accepted.Height, tx.Hash(), accepted.Exec, uint64(len(previousLogs)))
		}
		require.Error(t, publisher.Receipted(decoded.GlobalSequence, accepted.Height, tx.Hash(), accepted.Exec))
		require.Equal(t, logs, publishedLogs(t, h.client))
		previousLogs = logs

		for repeat := 0; repeat < 2; repeat++ {
			for _, client := range consumers {
				result := read(client, tx.Hash())
				require.NotNil(t, result)
				require.NotNil(t, result.SequencerReceiptView)
				require.Equal(t, evmrpc.RungReceipted, result.Rung)
				require.Equal(t, tx.Hash(), result.TxHash)
				require.Equal(t, nonce, uint64(result.Nonce))
				require.Equal(t, decoded.GlobalSequence, uint64(result.Seq))
				require.Equal(t, uint64(accepted.Height), uint64(result.Height))
				require.Equal(t, h.sender.EvmAddress, result.From)
				require.Equal(t, contract, result.To)
				require.Equal(t, uint64(accepted.Exec.Status), uint64(result.Status))
				require.Equal(t, accepted.Exec.GasUsed, uint64(result.GasUsed))
				require.Equal(t, accepted.Exec.CumulativeGasUsed, uint64(result.CumulativeGasUsed))
				require.Equal(t, uint64(accepted.Exec.TransactionIndex), uint64(result.Index))
				require.Equal(t, hexutil.Encode(send.ReturnData), hexutil.Encode(result.ReturnData))
				require.Equal(t, send.VMError, result.VMError)
				require.Len(t, result.Logs, len(send.LogData))
				for i, log := range result.Logs {
					require.Equal(t, hexutil.Encode(send.LogData[i]), hexutil.Encode(log.Data))
				}
				require.Equal(t, accepted.Receipt, []byte(result.Receipt))
				require.NoError(t, result.Verify(keys))
				rpcE2EVerifyReceipt(t, follower, result)
				lastResult = result
			}
		}
		lastHash, lastSend = tx.Hash(), send
	}
	require.Equal(t, committedTip, follower.LastBlockHeight())
	require.Zero(t, follower.EvmKeeper.GetNonce(follower.GetCheckCtx(), h.sender.EvmAddress))
	backup, err := h.backup.PendingSnapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(3), h.backup.EvmKeeper.GetNonce(backup, h.sender.EvmAddress))

	// Mutating caller-owned bytes cannot poison either the source or a later RPC read.
	original := bytes.Clone(lastSend.Receipt)
	lastSend.Receipt[0] ^= 0xff
	require.Equal(t, original, []byte(read(forwarded, lastHash).Receipt))
	lastSend.Receipt = original

	// A real producer result with truncated bytes is refused at consumer decode.
	malformed := *lastResult
	view := *lastResult.SequencerReceiptView
	malformed.SequencerReceiptView = &view
	malformed.Receipt = bytes.Clone(view.Receipt[:len(view.Receipt)-1])
	require.ErrorIs(t, malformed.Verify(keys), evmrpc.ErrSequencerReceiptInvalid)
	require.Panics(t, func() {
		bad := *lastSend
		bad.Receipt = malformed.Receipt
		n.ReceiptSource.Record(lastHash, bad)
	})

	// Preserve the signed receipt, but corrupt its associated execution output.
	// Both real RPC consumers must reject this current source record, even after
	// successfully serving the same hash, and must recover when it is restored.
	bad := *lastSend
	bad.ReturnData = append(bytes.Clone(lastSend.ReturnData), 0xff)
	n.ReceiptSource.Record(lastHash, bad)
	for _, client := range consumers {
		var result *evmrpc.SequencerReceiptResult
		err := client.CallContext(h.ctx, &result, evmrpc.SequencerReceiptMethod, lastHash)
		require.ErrorContains(t, err, evmrpc.ErrSequencerReceiptMismatch.Error())
		require.Nil(t, result)
	}
	n.ReceiptSource.Record(lastHash, *lastSend)
	for repeat := 0; repeat < 2; repeat++ {
		for _, client := range consumers {
			result := read(client, lastHash)
			require.Equal(t, lastResult, result)
			require.NoError(t, result.Verify(keys))
		}
	}

	// Replication with a closed primary WAL cannot release a receipt or publish
	// its hash. The first failure halts the accepter; the second never executes.
	n.WAL.Close()
	tx, txBytes := encode(receiptLegacyTx(3, &contract, big.NewInt(1), transferGas, nil))
	for repeat := 0; repeat < 2; repeat++ {
		refused, err := consumerAccepter.Accept(txBytes)
		require.ErrorIs(t, err, sequencer.ErrReplicationHalted)
		require.Equal(t, evmrpc.AcceptedSend{}, refused)
		assertUnknown(tx.Hash())
		require.Equal(t, sourceLastSeq+3, h.standby.LastSeq())
		require.Equal(t, previousLogs, publishedLogs(t, h.client))
		pending, err := n.App.PendingSnapshot()
		require.NoError(t, err)
		require.Equal(t, uint64(4), n.App.EvmKeeper.GetNonce(pending, h.sender.EvmAddress))
	}
	// Existing receipts remain available after replication halts.
	for _, client := range consumers {
		require.Equal(t, lastResult, read(client, lastHash))
	}
}

func TestReceiptAssetIsPaymentAsset(t *testing.T) {
	transport := newRPCE2EHarness(t)
	payer, recipient := utils.NewSigner(), utils.NewSigner()
	feeDenom := bridgetypes.SidioraDenom()
	producer := newReceiptHarness(t, 0, func(ctx sdk.Context, a *app.App) {
		a.EvmKeeper.SetAddressMapping(ctx, recipient.AccountAddress, recipient.EvmAddress)
		params := a.EvmKeeper.GetParams(ctx)
		params.FeeTokenEnabled = true
		params.MaxFeeTokenRateAge = evmtypes.DefaultMaxFeeTokenRateAge
		params.AllowedFeeDenoms = []evmtypes.AllowedFeeDenom{{
			Denom: feeDenom, Rate: sdk.NewDec(evmtypes.InitialSidioraBaseUnitsPerPax), RateUpdateHeight: committedTip,
		}}
		a.EvmKeeper.SetParams(ctx, params)
		coins := sdk.NewCoins(sdk.NewInt64Coin(feeDenom, sidFunding))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", coins))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", payer.AccountAddress, coins))
		require.NoError(t, a.EvmKeeper.SetAccountFeeDenom(ctx, payer.EvmAddress, feeDenom))
		require.NoError(t, a.SequencerKeeper.InitGenesis(ctx, seqtypes.GenesisState{
			Leaders:     [][32]byte{[32]byte(label("payment receipt leader"))},
			ReceiptKeys: []seqtypes.ReceiptKey{{KeyId: testKeyID, PublicKey: publicOf(testKeySeed), FirstHeight: 1, LastHeight: 1000}},
		}))
	}, payer)
	before := producer.app.BankKeeper.GetBalance(producer.openCtx(), payer.AccountAddress, feeDenom).Amount
	requireWei(t, new(big.Int), producer.weiBalance(producer.openCtx(), recipient.EvmAddress))
	tx, txBytes := transfer(t, payer, 0, recipient.EvmAddress, big.NewInt(onePaxInWei))
	accepted, receipt, signed := producer.accept(t, txBytes)
	paymentAsset := sha256.Sum256([]byte(evmkeeper.BaseDenom))
	feeAsset := sha256.Sum256([]byte(feeDenom))
	require.NotEqual(t, paymentAsset, feeAsset)
	require.Equal(t, paymentAsset, receipt.Asset)
	require.Equal(t, u128(big.NewInt(onePaxInWei)), receipt.Amount)
	require.Equal(t, feeDenom, string(receipt.Effects[0].Body[109:]))
	charged := before.Sub(producer.app.BankKeeper.GetBalance(producer.openCtx(), payer.AccountAddress, feeDenom).Amount)
	require.Equal(t, sdk.NewInt(sidConverted), charged)
	require.Equal(t, u128(charged.BigInt()), receipt.FeeCharged)
	requireWei(t, big.NewInt(onePaxInWei), producer.weiBalance(producer.openCtx(), recipient.EvmAddress))
	source := sequencer.NewReceiptSource(1)
	send := evmrpc.SequencerSend{Receipt: signed, LogData: [][]byte{}}
	source.Record(tx.Hash(), send)
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress, config.HTTPPort = "127.0.0.1", rpcE2EPort(t)
	server, err := evmrpc.NewEVMHTTPServer(config, transport.consensus, &producer.app.EvmKeeper,
		producer.app.BeginBlockKeepers, producer.app.BaseApp, producer.app.TracerAnteHandler,
		func(int64) sdk.Context { return producer.app.GetCheckCtx() },
		func(int64) client.TxConfig { return producer.app.GetTxConfig() }, "", nil,
		evmrpc.SequencerReceiptOption{Source: source, Keeper: &producer.app.SequencerKeeper})
	require.NoError(t, err)
	require.NoError(t, server.Start())
	t.Cleanup(server.Stop)
	consumer, err := gethrpc.DialHTTP("http://127.0.0.1:" + big.NewInt(int64(config.HTTPPort)).String())
	require.NoError(t, err)
	t.Cleanup(consumer.Close)
	var result *evmrpc.SequencerReceiptResult
	require.NoError(t, consumer.CallContext(t.Context(), &result, evmrpc.SequencerReceiptMethod, tx.Hash()))
	require.NotNil(t, result)
	require.Equal(t, evmkeeper.BaseDenom, result.PaymentDenom)
	require.Equal(t, feeDenom, result.FeeDenom)
	require.Equal(t, uint64(accepted.Seq), uint64(result.Seq))
	rpcE2EVerifyReceipt(t, producer.app, result)
	require.Equal(t, 1, bytes.Count(signed, paymentAsset[:]))
	malformed := bytes.Replace(signed, paymentAsset[:], feeAsset[:], 1)
	decoded, err := codec.DecodeReceipt(malformed)
	require.NoError(t, err)
	require.Equal(t, feeAsset, decoded.Asset)
	result.Receipt = malformed
	require.ErrorContains(t, result.Verify(), "asset is not the payment asset digest")
	send.Receipt = malformed
	source.Record(tx.Hash(), send)
	result = nil
	err = consumer.CallContext(t.Context(), &result, evmrpc.SequencerReceiptMethod, tx.Hash())
	require.ErrorContains(t, err, "asset is not the payment asset digest")
	require.Nil(t, result)
}
