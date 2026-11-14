package evmrpc_test

import (
	"context"
	"net/url"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/bytes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/mock"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/export"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type traceEntriesClient struct {
	mock.Client
	blocks map[int64]*coretypes.ResultBlock
}

func (*traceEntriesClient) EvmNextPendingNonce(common.Address) uint64 { return 0 }

func (*traceEntriesClient) EvmTxByHash(common.Hash) (tmtypes.Tx, bool) { return nil, false }

func (*traceEntriesClient) EvmProxy(common.Address) (*url.URL, bool) { return nil, false }

func (*traceEntriesClient) Validators(_ context.Context, height *int64, _, _ *int) (*coretypes.ResultValidators, error) {
	validator := tmtypes.NewValidator(ed25519.TestSecretKey([]byte("trace-entries-validator")).Public(), 1)
	result := &coretypes.ResultValidators{Validators: []*tmtypes.Validator{validator}, Count: 1, Total: 1}
	if height != nil {
		result.BlockHeight = *height
	}
	return result, nil
}

func (c *traceEntriesClient) Block(_ context.Context, height *int64) (*coretypes.ResultBlock, error) {
	if height == nil {
		height = new(int64)
		*height = 2
	}
	return c.blocks[*height], nil
}

func (c *traceEntriesClient) BlockByHash(_ context.Context, hash bytes.HexBytes) (*coretypes.ResultBlock, error) {
	for _, block := range c.blocks {
		if block != nil && block.BlockID.Hash.String() == hash.String() {
			return block, nil
		}
	}
	return &coretypes.ResultBlock{}, nil
}

func (c *traceEntriesClient) BlockResults(_ context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	block := c.blocks[*height]
	results := make([]*abci.ExecTxResult, len(block.Block.Txs))
	for i := range results {
		results[i] = &abci.ExecTxResult{}
	}
	return &coretypes.ResultBlockResults{Height: *height, TxsResults: results}, nil
}

func (c *traceEntriesClient) Status(context.Context) (*coretypes.ResultStatus, error) {
	return &coretypes.ResultStatus{SyncInfo: coretypes.SyncInfo{LatestBlockHeight: 2, EarliestBlockHeight: 1}}, nil
}

func traceEntriesBlock(height int64, blockHash byte, txs []tmtypes.Tx) *coretypes.ResultBlock {
	header := mockBlockHeader(height)
	hash := make([]byte, 32)
	hash[31] = blockHash
	return &coretypes.ResultBlock{
		BlockID: tmtypes.BlockID{Hash: bytes.HexBytes(hash)},
		Block: &tmtypes.Block{
			Header:     header,
			Data:       tmtypes.Data{Txs: txs},
			LastCommit: &tmtypes.Commit{Height: height - 1},
		},
	}
}

func newTraceEntriesAPI(t *testing.T) (*evmrpc.DebugAPI, *traceEntriesClient, *gethtypes.Transaction) {
	t.Helper()
	a := app.Setup(t, false, false, false)
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(2).WithBlockTime(mockBlockHeader(2).Time)
	a.EvmKeeper.InitGenesis(ctx, *types.DefaultGenesis())

	txBytes, err := Encoder(MultiTxBlockTx1)
	require.NoError(t, err)
	tx, _ := MultiTxBlockTx1.GetMsgs()[0].(*types.MsgEVMTransaction).AsTransaction()
	require.NotNil(t, tx)
	require.NoError(t, a.EvmKeeper.MockReceipt(ctx, tx.Hash(), &types.Receipt{
		BlockNumber:       2,
		TransactionIndex:  1,
		TxHashHex:         tx.Hash().Hex(),
		GasUsed:           21000,
		EffectiveGasPrice: 1,
	}))

	batchEntry := tmtypes.Tx([]byte("PAXEER/v1/sequence-batch\x00not-an-sdk-transaction"))
	parent := traceEntriesBlock(1, 0x06, nil)
	target := traceEntriesBlock(2, 0x42, []tmtypes.Tx{batchEntry, txBytes})
	tmClient := &traceEntriesClient{blocks: map[int64]*coretypes.ResultBlock{1: parent, 2: target}}
	ctxProvider := func(height int64) sdk.Context {
		if height == evmrpc.LatestCtxHeight {
			return ctx
		}
		return ctx.WithBlockHeight(height)
	}
	watermarks := evmrpc.NewWatermarkManager(tmClient, ctxProvider, nil, a.EvmKeeper.ReceiptStore())
	debugConfig := evmrpcconfig.DefaultConfig
	debugConfig.TraceTimeout = time.Minute
	debugConfig.MaxTraceLookbackBlocks = -1
	api := evmrpc.NewDebugAPI(
		tmClient,
		&a.EvmKeeper,
		a.BeginBlockKeepers,
		ctxProvider,
		func(int64) client.TxConfig { return TxConfig },
		&evmrpc.SimulateConfig{GasCap: 10000000},
		a.BaseApp,
		a.TracerAnteHandler,
		evmrpc.ConnectionTypeHTTP,
		debugConfig,
		evmrpc.NewBlockCache(32),
		&sync.Mutex{},
		watermarks,
	)
	return api, tmClient, tx
}

func TestTraceBlockEntriesBlockReadShape(t *testing.T) {
	api, _, tx := newTraceEntriesAPI(t)

	name := "callTracer"
	config := &tracers.TraceConfig{Tracer: &name}
	tracesResult, err := api.TraceBlockByNumber(context.Background(), 2, config)
	require.NoError(t, err)
	traces, ok := tracesResult.([]*tracers.TxTraceResult)
	require.True(t, ok)
	require.Len(t, traces, 1)
	require.Equal(t, tx.Hash(), traces[0].TxHash)
	require.NotContains(t, traces[0].Error, "tx parse error")

	byHashResult, err := api.TraceBlockByHash(context.Background(), common.BytesToHash(append(make([]byte, 31), 0x42)), config)
	require.NoError(t, err)
	byHash, ok := byHashResult.([]*tracers.TxTraceResult)
	require.True(t, ok)
	require.Len(t, byHash, 1)
	require.Equal(t, traces[0].TxHash, byHash[0].TxHash)

	_, err = api.TraceTransaction(context.Background(), tx.Hash(), config)
	require.NoError(t, err)

	_, err = api.TraceCall(context.Background(), export.TransactionArgs{}, gethrpc.BlockNumberOrHashWithNumber(2), nil)
	require.NoError(t, err)
}
