package evmrpc_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmnode "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpclocal "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	receiptstore "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

type repair185Harness struct {
	s         *pendingSequencer
	tm        *rpclocal.Local
	api       *evmrpc.FilterAPI
	client    *rpc.Client
	cache     evmrpc.BlockCache
	sender    utils.TestAcct
	nonce     uint64
	blocks    map[int64]*coretypes.ResultBlock
	hashes    map[int64][]common.Hash
	receipts  map[common.Hash]*evmtypes.Receipt
	saveBlock func(int64, [][]byte, time.Time) *coretypes.ResultBlock
}

func newRepair185Harness(t *testing.T, maxLog int64) *repair185Harness {
	t.Helper()
	defaults := app.DefaultConsensusParams
	params := *defaults
	blockParams := *defaults.Block
	blockParams.MaxGasWanted = blockParams.MaxGas
	params.Block = &blockParams
	app.DefaultConsensusParams = &params
	t.Cleanup(func() { app.DefaultConsensusParams = defaults })
	sender := utils.NewSigner()
	s := newPendingSequencer(t, 1, sender)
	t.Cleanup(func() { require.NoError(t, s.app.Close()) })

	// Construct the real consensus stores and local RPC client without starting
	// consensus: each block below is produced from the executor's sealed batch.
	cfg, err := rpctest.CreateConfig(t, strings.ReplaceAll(t.Name(), "/", "_"))
	require.NoError(t, err)
	cfg.Mode = tmconfig.ModeFull
	node, err := tmnode.New(context.Background(), cfg, func() {}, abci.BaseApplication{}, nil, nil,
		tmnode.NoOpMetricsProvider(), tmtypes.DefaultConsensusPolicy())
	require.NoError(t, err)
	t.Cleanup(func() { node.(interface{ OnStop() }).OnStop() })
	tm, err := rpclocal.New(node)
	require.NoError(t, err)
	state, err := node.RPCEnvironment().StateStore.Load()
	require.NoError(t, err)
	require.NotNil(t, state.Validators)
	validator, err := privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
	require.NoError(t, err)
	lastCommit := &tmtypes.Commit{}
	h := &repair185Harness{s: s, tm: tm, sender: sender, cache: evmrpc.NewBlockCache(64),
		blocks: make(map[int64]*coretypes.ResultBlock), hashes: make(map[int64][]common.Hash),
		receipts: make(map[common.Hash]*evmtypes.Receipt)}
	h.saveBlock = func(height int64, encoded [][]byte, timestamp time.Time) *coretypes.ResultBlock {
		txs := make([]tmtypes.Tx, len(encoded))
		for i := range encoded {
			txs[i] = encoded[i]
		}
		block := state.MakeBlockAt(height, txs, lastCommit, nil,
			state.Validators.GetProposer().Address, timestamp)
		parts, err := block.MakePartSet(tmtypes.BlockPartSizeBytes)
		require.NoError(t, err)
		id := tmtypes.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
		vote := &tmproto.Vote{Type: tmproto.PrecommitType, Height: height, BlockID: id.ToProto(),
			Timestamp: timestamp, ValidatorAddress: validator.Key.Address, ValidatorIndex: 0}
		require.NoError(t, validator.SignVote(context.Background(), state.ChainID, vote))
		signed, err := tmtypes.VoteFromProto(vote)
		require.NoError(t, err)
		commit := &tmtypes.Commit{Height: height, BlockID: id, Signatures: []tmtypes.CommitSig{signed.CommitSig()}}
		require.NoError(t, commit.ValidateBasic())
		require.NoError(t, state.Validators.VerifyCommit(state.ChainID, id, height, commit))
		node.RPCEnvironment().BlockStore.SaveBlock(block, parts, commit)
		lastCommit = commit
		state.LastBlockID = id
		state.LastBlockHeight = height
		result, err := tm.Block(context.Background(), &height)
		require.NoError(t, err)
		h.blocks[height] = result
		return result
	}
	h.saveBlock(1, nil, time.Now().UTC())
	wm := evmrpc.NewWatermarkManager(tm, s.ctxProvider, nil, s.app.EvmKeeper.ReceiptStore())
	h.api = evmrpc.NewFilterAPI(tm, &s.app.EvmKeeper, s.ctxProvider,
		func(int64) client.TxConfig { return s.app.GetTxConfig() },
		evmrpc.NewFilterConfig(time.Minute, maxLog, 64), evmrpc.ConnectionTypeHTTP, "eth",
		make(chan struct{}, evmrpc.MaxDBReadConcurrency), h.cache, &sync.Mutex{}, evmrpc.NewLogSlicePool(), wm)
	t.Cleanup(func() { evmrpc.ShutdownFilterAPIForTest(h.api) })
	server := rpc.NewServer()
	require.NoError(t, server.RegisterName("eth", h.api))
	t.Cleanup(server.Stop)
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	h.client, err = rpc.DialHTTP(httpServer.URL)
	require.NoError(t, err)
	t.Cleanup(h.client.Close)
	return h
}

func (h *repair185Harness) commit(t *testing.T, count int) []common.Hash {
	t.Helper()
	height := h.s.committed + 1
	var hashes []common.Hash
	for i := 0; i < count; i++ {
		encoded, hash := pendingFilterDeploy(t, h.sender, h.nonce)
		h.nonce++
		accepted, err := h.s.exec.Accept(encoded)
		require.NoError(t, err)
		require.Equal(t, abci.CodeTypeOK, accepted.Result.Code, accepted.Result.Log)
		snapshot, err := h.s.exec.Snapshot()
		require.NoError(t, err)
		rcpt, err := h.s.app.EvmKeeper.GetTransientReceipt(snapshot, hash, uint64(accepted.TxIndex))
		require.NoError(t, err)
		require.Len(t, rcpt.Logs, 1)
		h.receipts[hash] = rcpt
		hashes = append(hashes, hash)
	}
	batch, err := h.s.exec.Seal(height)
	require.NoError(t, err)
	require.NoError(t, h.s.app.EvmKeeper.FlushTransientReceipts(batch.Ctx))
	batch.Store.Write()
	require.Equal(t, height, h.s.app.CommitMultiStore().Commit(true).Version)
	h.s.committed = height
	for _, hash := range hashes {
		require.Eventually(t, func() bool {
			rcpt, err := h.s.app.EvmKeeper.GetReceipt(h.s.ctxProvider(height), hash)
			return err == nil && rcpt.BlockNumber == uint64(height)
		}, 5*time.Second, time.Millisecond)
	}
	require.NoError(t, h.s.app.EvmKeeper.ReceiptStore().SetLatestVersion(height))
	h.saveBlock(height, batch.Txs, batch.Ctx.BlockTime())
	h.hashes[height] = hashes
	require.NoError(t, h.s.exec.Open(height+1, batch.TimestampMs+1000))
	return hashes
}

func (h *repair185Harness) install(t *testing.T, criteria map[string]any) rpc.ID {
	t.Helper()
	var id rpc.ID
	require.NoError(t, h.client.CallContext(context.Background(), &id, "eth_newFilter", criteria))
	require.NotEmpty(t, id)
	return id
}

func (h *repair185Harness) read(t *testing.T, method string, id rpc.ID) []ethtypes.Log {
	t.Helper()
	var logs []ethtypes.Log
	require.NoError(t, h.client.CallContext(context.Background(), &logs, method, id))
	require.NotNil(t, logs)
	return logs
}

func (h *repair185Harness) assertLogs(t *testing.T, logs []ethtypes.Log, heights ...int64) {
	t.Helper()
	var expected []common.Hash
	for _, height := range heights {
		expected = append(expected, h.hashes[height]...)
	}
	require.Len(t, logs, len(expected))
	position := 0
	for _, height := range heights {
		for i, hash := range h.hashes[height] {
			got := logs[position]
			position++
			require.Equal(t, hash, got.TxHash)
			require.Equal(t, uint64(height), got.BlockNumber)
			require.Equal(t, uint(i), got.Index)
			require.Equal(t, uint(i), got.TxIndex)
			require.Equal(t, common.BytesToHash(h.blocks[height].BlockID.Hash), got.BlockHash)
			require.Equal(t, []common.Hash{pendingFilterTopic}, got.Topics)
			require.False(t, got.Removed)
		}
	}
}

func TestRepair_185_ReceiptReadFailureAndRecovery(t *testing.T) {
	h := newRepair185Harness(t, 20)
	hashes := h.commit(t, 3)
	store := h.s.app.EvmKeeper.ReceiptStore()
	ctx := h.s.ctxProvider(evmrpc.LatestCtxHeight)
	_, err := store.GetReceipt(ctx, common.HexToHash("0xdead"))
	require.ErrorIs(t, err, receiptstore.ErrNotFound)
	id := h.install(t, map[string]any{"fromBlock": "0x2", "toBlock": "0x2"})
	produced := h.read(t, "eth_getFilterLogs", id)
	h.assertLogs(t, produced, 2)
	candidates := make([]*ethtypes.Log, len(produced))
	for i := range produced {
		candidates[i] = &produced[i]
	}
	h.cache.Remove(2)
	record := receiptstore.ReceiptRecord{TxHash: hashes[1], Receipt: h.receipts[hashes[1]], ReceiptBytes: []byte{0xff}}
	require.NoError(t, store.SetReceipts(ctx.WithBlockHeight(3), []receiptstore.ReceiptRecord{record}))
	require.Eventually(t, func() bool {
		_, err := store.GetReceipt(ctx, hashes[1])
		return err != nil && !errors.Is(err, receiptstore.ErrNotFound)
	}, 5*time.Second, time.Millisecond)
	var logs []ethtypes.Log
	for _, method := range []string{"eth_getFilterChanges", "eth_getFilterLogs"} {
		err := h.client.CallContext(context.Background(), &logs, method, id)
		require.Error(t, err)
		require.ErrorContains(t, err, hashes[1].Hex())
		require.Nil(t, logs)
	}
	_, err = evmrpc.NormalizeRepair185LogsForTest(h.api, candidates, filters.FilterCriteria{})
	require.ErrorContains(t, err, hashes[1].Hex())
	record.ReceiptBytes = nil
	require.NoError(t, store.SetReceipts(ctx.WithBlockHeight(4), []receiptstore.ReceiptRecord{record}))
	require.Eventually(t, func() bool {
		rcpt, err := store.GetReceipt(ctx, hashes[1])
		return err == nil && len(rcpt.Logs) == 1
	}, 5*time.Second, time.Millisecond)
	h.assertLogs(t, h.read(t, "eth_getFilterChanges", id), 2)
	require.Empty(t, h.read(t, "eth_getFilterChanges", id))
	rebuilt, err := evmrpc.NormalizeRepair185LogsForTest(h.api, candidates, filters.FilterCriteria{})
	require.NoError(t, err)
	values := make([]ethtypes.Log, len(rebuilt))
	for i, log := range rebuilt {
		values[i] = *log
	}
	h.assertLogs(t, values, 2)
}

func TestRepair_185_LogLimitIsExplicitAndDoesNotConsume(t *testing.T) {
	for _, perBlock := range []int{1, 3} {
		t.Run(hexutil.EncodeUint64(uint64(perBlock)), func(t *testing.T) {
			h := newRepair185Harness(t, 2)
			h.commit(t, perBlock)
			if perBlock == 1 {
				h.commit(t, 1)
			}
			id := h.install(t, map[string]any{"fromBlock": "0x2"})
			if perBlock == 1 {
				h.assertLogs(t, h.read(t, "eth_getFilterLogs", id), 2, 3)
				h.commit(t, 1)
			}
			cursor := evmrpc.Repair185CursorForTest(h.api, id)
			for _, method := range []string{"eth_getFilterChanges", "eth_getFilterLogs", "eth_getFilterChanges"} {
				var logs []ethtypes.Log
				err := h.client.CallContext(context.Background(), &logs, method, id)
				require.ErrorContains(t, err, "log limit exceeded (3), maximum allowed is 2 logs")
				require.Nil(t, logs)
				require.Equal(t, cursor, evmrpc.Repair185CursorForTest(h.api, id))
			}
			var logs []ethtypes.Log
			for _, criteria := range []map[string]any{
				{"fromBlock": "0x2", "toBlock": hexutil.EncodeUint64(uint64(h.s.committed))},
				{"fromBlock": "0x2"},
			} {
				err := h.client.CallContext(context.Background(), &logs, "eth_getLogs", criteria)
				require.ErrorContains(t, err, "log limit exceeded")
				require.Nil(t, logs)
			}
			if perBlock == 3 {
				err := h.client.CallContext(context.Background(), &logs, "eth_getLogs", map[string]any{
					"blockHash": common.BytesToHash(h.blocks[2].BlockID.Hash),
				})
				require.ErrorContains(t, err, "log limit exceeded")
			}
			var candidates []*ethtypes.Log
			for height, hashes := range h.hashes {
				var produced []ethtypes.Log
				require.NoError(t, h.client.CallContext(context.Background(), &produced, "eth_getLogs", map[string]any{
					"fromBlock": hexutil.EncodeUint64(uint64(height)), "toBlock": hexutil.EncodeUint64(uint64(height)),
					"address": common.HexToAddress(h.receipts[hashes[0]].ContractAddress),
				}))
				require.Len(t, produced, 1)
				candidates = append(candidates, &produced[0])
			}
			_, err := evmrpc.NormalizeRepair185LogsForTest(h.api, candidates, filters.FilterCriteria{})
			require.ErrorContains(t, err, "log limit exceeded (3), maximum allowed is 2 logs")
		})
	}
}

func TestRepair_185_DynamicEndsAndInclusiveCursor(t *testing.T) {
	for _, tag := range []string{"latest", "safe", "finalized"} {
		t.Run(tag, func(t *testing.T) {
			h := newRepair185Harness(t, 20)
			h.commit(t, 1)
			id := h.install(t, map[string]any{"fromBlock": "0x2", "toBlock": tag})
			h.assertLogs(t, h.read(t, "eth_getFilterChanges", id), 2)
			require.Empty(t, h.read(t, "eth_getFilterChanges", id))
			h.commit(t, 1)
			h.commit(t, 1)
			h.assertLogs(t, h.read(t, "eth_getFilterChanges", id), 3, 4)
			require.Empty(t, h.read(t, "eth_getFilterChanges", id))
		})
	}
	t.Run("inclusive_numeric_end", func(t *testing.T) {
		h := newRepair185Harness(t, 20)
		h.commit(t, 1)
		id := h.install(t, map[string]any{"toBlock": "0x2"})
		require.Equal(t, int64(2), evmrpc.Repair185CursorForTest(h.api, id))
		h.assertLogs(t, h.read(t, "eth_getFilterChanges", id), 2)
		require.Empty(t, h.read(t, "eth_getFilterChanges", id))
		h.commit(t, 1)
		require.Empty(t, h.read(t, "eth_getFilterChanges", id))
	})
}

func TestRepair_185_InstalledStartDoesNotMove(t *testing.T) {
	for _, start := range []string{"", "latest", "safe", "finalized"} {
		t.Run("from_"+start, func(t *testing.T) {
			h := newRepair185Harness(t, 20)
			h.commit(t, 1)
			criteria := map[string]any{}
			if start != "" {
				criteria["fromBlock"] = start
			}
			id := h.install(t, criteria)
			h.commit(t, 1)
			h.commit(t, 1)
			h.commit(t, 1)
			h.assertLogs(t, h.read(t, "eth_getFilterChanges", id), 2, 3, 4, 5)
			require.Empty(t, h.read(t, "eth_getFilterChanges", id))
			h.commit(t, 1)
			h.commit(t, 1)
			h.assertLogs(t, h.read(t, "eth_getFilterChanges", id), 6, 7)
			require.Empty(t, h.read(t, "eth_getFilterChanges", id))
		})
	}
}

func TestRepair_185_SnapshotsDoNotConsumeChanges(t *testing.T) {
	h := newRepair185Harness(t, 20)
	h.commit(t, 1)
	id := h.install(t, map[string]any{"fromBlock": "latest"})
	h.assertLogs(t, h.read(t, "eth_getFilterChanges", id), 2)
	h.commit(t, 1)
	h.commit(t, 1)
	for i := 0; i < 2; i++ {
		h.assertLogs(t, h.read(t, "eth_getFilterLogs", id), 2, 3, 4)
	}
	h.commit(t, 1)
	h.assertLogs(t, h.read(t, "eth_getFilterChanges", id), 3, 4, 5)
	h.assertLogs(t, h.read(t, "eth_getFilterLogs", id), 2, 3, 4, 5)
	require.Empty(t, h.read(t, "eth_getFilterChanges", id))
	h.commit(t, 1)
	h.commit(t, 1)
	h.assertLogs(t, h.read(t, "eth_getFilterLogs", id), 2, 3, 4, 5, 6, 7)
	h.assertLogs(t, h.read(t, "eth_getFilterChanges", id), 6, 7)
	require.Empty(t, h.read(t, "eth_getFilterChanges", id))
}
