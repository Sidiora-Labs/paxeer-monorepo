package sequencer_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	rpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type logFeedReceipter struct {
	r *sequencer.Receipter
}

func (a logFeedReceipter) Accept(txBytes []byte) ([]byte, error) {
	_, signed, err := a.r.Accept(txBytes)
	return signed, err
}

type publishedReceiptLog struct {
	Address   common.Address  `json:"address"`
	Topics    []common.Hash   `json:"topics"`
	Data      hexutil.Bytes   `json:"data"`
	Height    hexutil.Uint64  `json:"blockNumber"`
	Hash      common.Hash     `json:"transactionHash"`
	TxIndex   hexutil.Uint64  `json:"transactionIndex"`
	Index     hexutil.Uint64  `json:"logIndex"`
	BlockHash *common.Hash    `json:"blockHash"`
	Seq       *hexutil.Uint64 `json:"seq"`
	Removed   bool            `json:"removed"`
}

func logFeedRPC(t *testing.T, rh *receiptHarness, feed *evmrpc.ReceiptLogFeed) *ethrpc.Client {
	t.Helper()
	cfg := rpcconfig.DefaultConfig
	cfg.HTTPAddress, cfg.HTTPPort = "127.0.0.1", 0
	ctxProvider := func(int64) sdk.Context { return rh.openCtx() }
	server, err := evmrpc.NewEVMHTTPServer(
		cfg, &local.Local{}, &rh.app.EvmKeeper, rh.app.BeginBlockKeepers,
		rh.app.BaseApp, rh.app.TracerAnteHandler, ctxProvider,
		func(int64) client.TxConfig { return app.MakeEncodingConfig().TxConfig },
		t.TempDir(), nil,
		evmrpc.NewSequencerOption(logFeedReceipter{rh.r}, feed),
	)
	require.NoError(t, err)
	require.NoError(t, server.Start())
	t.Cleanup(server.Stop)
	httpServer, ok := server.(*evmrpc.HTTPServer)
	require.True(t, ok)
	rpc, err := ethrpc.DialHTTP("http://" + httpServer.ListenAddr())
	require.NoError(t, err)
	t.Cleanup(rpc.Close)
	return rpc
}

func publishedLogs(t *testing.T, rpc *ethrpc.Client) []publishedReceiptLog {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var logs []publishedReceiptLog
	require.NoError(t, rpc.CallContext(ctx, &logs, "eth_getLogs", map[string]string{
		"fromBlock": "pending", "toBlock": "pending",
	}))
	return logs
}

func requirePublishedReceipt(t *testing.T, logs []publishedReceiptLog, seq uint64, height int64, hash common.Hash, rcpt *evmtypes.Receipt, firstIndex uint64) {
	t.Helper()
	require.Len(t, logs, len(rcpt.Logs))
	for i, want := range rcpt.Logs {
		got := logs[i]
		require.NotNil(t, got.Seq)
		require.Equal(t, seq, uint64(*got.Seq))
		require.Equal(t, uint64(height), uint64(got.Height))
		require.Equal(t, hash, got.Hash)
		require.Equal(t, uint64(rcpt.TransactionIndex), uint64(got.TxIndex))
		require.Equal(t, firstIndex+uint64(i), uint64(got.Index))
		require.Equal(t, common.HexToAddress(want.Address), got.Address)
		require.Equal(t, hexutil.Encode(want.Data), hexutil.Encode(got.Data))
		require.Len(t, got.Topics, len(want.Topics))
		for j, topic := range want.Topics {
			require.Equal(t, common.HexToHash(topic), got.Topics[j])
		}
		require.Nil(t, got.BlockHash)
		require.False(t, got.Removed)
	}
}

func TestLogFeedPublishesReceiptedLogs(t *testing.T) {
	sender := utils.NewSigner()
	rh := newReceiptHarness(t, 0, admissionParams(createGas+2*callGas, createGas+2*callGas, 200000), sender)
	feed := evmrpc.NewReceiptLogFeed(rh.app.LastBlockHeight())
	publisher := sequencer.NewLogFeedPublisher(feed)
	rpc := logFeedRPC(t, rh, feed)
	require.Empty(t, publishedLogs(t, rpc))
	require.Panics(t, func() { sequencer.NewLogFeedPublisher(nil) })

	deploy, bz := evmTx(t, sender, receiptLegacyTx(0, nil, big.NewInt(0), createGas, logInit))
	acc, _, _ := rh.accept(t, bz)
	require.NoError(t, publisher.Receipted(acc.Seq, acc.Height, deploy.Hash(), rh.transient(t, deploy, acc)))
	require.Empty(t, publishedLogs(t, rpc))
	contract := crypto.CreateAddress(sender.EvmAddress, 0)
	var logIndex uint64
	var previous []publishedReceiptLog
	for nonce := uint64(1); nonce <= 2; nonce++ {
		tx, bz := evmTx(t, sender, receiptLegacyTx(nonce, &contract, big.NewInt(0), callGas, nil))
		acc, _, _ := rh.accept(t, bz)
		rcpt := rh.transient(t, tx, acc)
		require.Len(t, rcpt.Logs, 3)
		require.NoError(t, publisher.Receipted(acc.Seq, acc.Height, tx.Hash(), rcpt))
		logs := publishedLogs(t, rpc)
		require.Len(t, logs, int(logIndex)+len(rcpt.Logs))
		if logIndex > 0 {
			require.Equal(t, previous, logs[:logIndex])
		}
		requirePublishedReceipt(t, logs[logIndex:], acc.Seq, acc.Height, tx.Hash(), rcpt, logIndex)
		before := logs
		require.Error(t, publisher.Receipted(acc.Seq, acc.Height, tx.Hash(), rcpt))
		require.Error(t, publisher.Receipted(acc.Seq-1, acc.Height, tx.Hash(), rcpt))
		require.Error(t, publisher.Receipted(acc.Seq+1, acc.Height, tx.Hash(), nil))
		require.Equal(t, before, publishedLogs(t, rpc))
		previous = logs
		logIndex += uint64(len(rcpt.Logs))
	}
}

func TestLogFeedCommittedAdvancesHeight(t *testing.T) {
	rh := newReceiptHarness(t, 0, nil)
	committed := rh.app.LastBlockHeight()
	feed := evmrpc.NewReceiptLogFeed(committed)
	publisher := sequencer.NewLogFeedPublisher(feed)
	rpc := logFeedRPC(t, rh, feed)
	rcpt := &evmtypes.Receipt{TransactionIndex: 1, Logs: []*evmtypes.Log{{
		Address: common.HexToAddress("0x1234").Hex(),
		Topics:  []string{common.HexToHash("0x5678").Hex()}, Data: []byte{1, 2, 3},
	}}}
	require.Error(t, publisher.Receipted(1, committed, common.HexToHash("0x1"), rcpt))
	require.Empty(t, publishedLogs(t, rpc))
	for seq := uint64(1); seq <= 3; seq++ {
		require.NoError(t, publisher.Receipted(seq, committed+int64(seq), common.BigToHash(new(big.Int).SetUint64(seq)), rcpt))
	}
	require.Len(t, publishedLogs(t, rpc), 3)
	for seq := uint64(1); seq <= 3; seq++ {
		height := committed + int64(seq)
		require.NoError(t, publisher.Committed(height))
		logs := publishedLogs(t, rpc)
		require.Len(t, logs, 3-int(seq))
		for i, log := range logs {
			next := seq + uint64(i) + 1
			requirePublishedReceipt(t, []publishedReceiptLog{log}, next, committed+int64(next), common.BigToHash(new(big.Int).SetUint64(next)), rcpt, 0)
		}
		require.NoError(t, publisher.Committed(height))
		require.Error(t, publisher.Committed(height-1))
		require.Error(t, publisher.Receipted(4, height, common.HexToHash("0x4"), rcpt))
		require.Equal(t, logs, publishedLogs(t, rpc))
	}
	require.NoError(t, publisher.Committed(committed+4))
	require.Error(t, publisher.Receipted(4, committed+4, common.HexToHash("0x4"), rcpt))
	require.NoError(t, publisher.Receipted(4, committed+5, common.HexToHash("0x4"), rcpt))
	logs := publishedLogs(t, rpc)
	requirePublishedReceipt(t, logs, 4, committed+5, common.HexToHash("0x4"), rcpt, 0)
}
