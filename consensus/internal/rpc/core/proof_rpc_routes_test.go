// Proves: core.Environment RPC producers -> core.NewRoutesMap and server.RegisterRPCFuncs JSON-RPC over HTTP -> http.HTTP and rpctest.WaitForRPC.
//
// The classic node proves the changed routes with real consensus,
// signed evidence, block storage and indexes; reopening it proves historical
// results survive shutdown. Every repeated read writes another HTTP request.
// The malformed-status proof uses Environment.Status itself to emit a result
// with no chain identity, without modifying the response or replacing transport.
// Repairs: http.go accepted that malformed status, including in batches;
// helpers.go waited forever after cancellation and could not preserve node data.
// Outside scope: server/http_server.go:writeRPCResponse collapses a single-item
// batch to an object; client/decode.go:unmarshalResponseBytesArray refuses it.
package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"golang.org/x/time/rate"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	atypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	aconsensus "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/consensus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/producer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p/conn"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	core "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/tcp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	jsonclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/client"
	rpc "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestProofRpcRoutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	conf.Mode = config.ModeValidator
	// CreateConfig defaults to memdb; the restart must reopen the actual
	// consensus stores while the application retains its committed state.
	conf.DBBackend = string(dbm.GoLevelDBBackend)
	conf.Consensus.CreateEmptyBlocks = false
	app := kvstore.NewApplication()
	defer func() { require.NoError(t, app.Close()) }()
	n, closeNode, err := rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout, rpctest.PreserveRoot)
	require.NoError(t, err)
	defer func() { require.NoError(t, closeNode(ctx)) }()
	env := n.(interface{ RPCEnvironment() *core.Environment }).RPCEnvironment()
	c, err := rpchttp.New(conf.RPC.ListenAddress)
	require.NoError(t, err)

	// The consumer submits transactions to real consensus. Distinct status
	// heights also prove a previous successful response is not cached.
	first, err := c.BroadcastTxCommit(ctx, types.Tx("proof=first"))
	require.NoError(t, err)
	require.Zero(t, first.CheckTx.Code)
	require.Zero(t, first.TxResult.Code)
	before, err := c.Status(ctx)
	require.NoError(t, err)
	tx := types.Tx("proof=second")
	second, err := c.BroadcastTxCommit(ctx, tx)
	require.NoError(t, err)
	require.Zero(t, second.CheckTx.Code)
	require.Zero(t, second.TxResult.Code)
	require.Greater(t, second.Height, before.SyncInfo.LatestBlockHeight)
	height := second.Height
	block, err := c.Block(ctx, &height)
	require.NoError(t, err)
	require.NotNil(t, block.Block)
	require.Contains(t, block.Block.Txs, tx)
	genesis, err := types.GenesisDocFromFile(conf.GenesisFile())
	require.NoError(t, err)
	pv, err := privval.LoadFilePV(conf.PrivValidator.KeyFile(), conf.PrivValidator.StateFile())
	require.NoError(t, err)
	vals, err := env.StateStore.LoadValidators(height)
	require.NoError(t, err)
	evidence := proofRpcEvidence(t, pv.Key.PrivKey, genesis.ChainID, height, block.Block.Time, vals)

	var writes atomic.Int64
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{WroteRequest: func(info httptrace.WroteRequestInfo) {
		if info.Err == nil {
			writes.Add(1)
		}
	}})
	for _, route := range []struct {
		name  string
		call  func() (any, error)
		check func(*testing.T, any)
	}{
		{"status", func() (any, error) { return c.Status(ctx) }, func(t *testing.T, got any) {
			r := got.(*coretypes.ResultStatus)
			require.Equal(t, genesis.ChainID, r.NodeInfo.Network)
			require.GreaterOrEqual(t, r.SyncInfo.LatestBlockHeight, height)
			meta := env.BlockStore.LoadBlockMeta(r.SyncInfo.LatestBlockHeight)
			require.NotNil(t, meta)
			require.Equal(t, meta.BlockID.Hash, r.SyncInfo.LatestBlockHash)
		}},
		{"block_search", func() (any, error) {
			return c.BlockSearch(ctx, fmt.Sprintf("block.height=%d", height), nil, nil, "asc")
		}, func(t *testing.T, got any) {
			r := got.(*coretypes.ResultBlockSearch)
			require.Equal(t, 1, r.TotalCount)
			require.Equal(t, []*coretypes.ResultBlock{block}, r.Blocks)
		}},
		{"blockchain", func() (any, error) { return c.BlockchainInfo(ctx, height, height) }, func(t *testing.T, got any) {
			r := got.(*coretypes.ResultBlockchainInfo)
			require.GreaterOrEqual(t, r.LastHeight, height)
			require.Len(t, r.BlockMetas, 1)
			require.Equal(t, block.BlockID, r.BlockMetas[0].BlockID)
			require.Equal(t, block.Block.Header, r.BlockMetas[0].Header)
		}},
		{"broadcast_evidence", func() (any, error) { return c.BroadcastEvidence(ctx, evidence) }, func(t *testing.T, got any) {
			require.Equal(t, evidence.Hash(), []byte(got.(*coretypes.ResultBroadcastEvidence).Hash))
		}},
		{"commit", func() (any, error) { return c.Commit(ctx, &height) }, func(t *testing.T, got any) {
			r := got.(*coretypes.ResultCommit)
			require.NotNil(t, r.Header)
			require.NotNil(t, r.Commit)
			require.Equal(t, block.Block.Header, *r.Header)
			require.NoError(t, vals.VerifyCommitLight(genesis.ChainID, block.BlockID, height, r.Commit))
		}},
		{"consensus_params", func() (any, error) { return c.ConsensusParams(ctx, &height) }, func(t *testing.T, got any) {
			r := got.(*coretypes.ResultConsensusParams)
			want, err := env.ConsensusParams(ctx, &coretypes.RequestConsensusParams{Height: (*coretypes.Int64)(&height)})
			require.NoError(t, err)
			require.Equal(t, want, r)
		}},
		{"consensus_state", func() (any, error) { return c.ConsensusState(ctx) }, func(t *testing.T, got any) {
			var round struct {
				HeightRoundStep string `json:"height/round/step"`
			}
			require.NoError(t, json.Unmarshal(got.(*coretypes.ResultConsensusState).RoundState, &round))
			var h int64
			var roundNum, step int
			count, err := fmt.Sscanf(round.HeightRoundStep, "%d/%d/%d", &h, &roundNum, &step)
			require.NoError(t, err)
			require.Equal(t, 3, count)
			require.Greater(t, h, height)
		}},
		{"dump_consensus_state", func() (any, error) { return c.DumpConsensusState(ctx) }, func(t *testing.T, got any) {
			var round struct {
				Height int64 `json:"Height"`
			}
			r := got.(*coretypes.ResultDumpConsensusState)
			require.NoError(t, json.Unmarshal(r.RoundState, &round))
			require.Greater(t, round.Height, height)
			require.Empty(t, r.Peers)
		}},
		{"header", func() (any, error) { return c.Header(ctx, &height) }, func(t *testing.T, got any) {
			require.Equal(t, &block.Block.Header, got.(*coretypes.ResultHeader).Header)
		}},
		{"header_by_hash", func() (any, error) { return c.HeaderByHash(ctx, block.BlockID.Hash) }, func(t *testing.T, got any) {
			require.Equal(t, &block.Block.Header, got.(*coretypes.ResultHeader).Header)
		}},
		{"lag_status", func() (any, error) { return c.LagStatus(ctx) }, func(t *testing.T, got any) {
			r := got.(*coretypes.ResultLagStatus)
			require.GreaterOrEqual(t, r.CurrentHeight, height)
			require.Zero(t, r.Lag)
		}},
		{"tx", func() (any, error) { return c.Tx(ctx, tx.Hash().Bytes(), true) }, func(t *testing.T, got any) {
			r := got.(*coretypes.ResultTx)
			require.Equal(t, tx, r.Tx)
			require.Equal(t, height, r.Height)
			require.NoError(t, r.Proof.Validate(block.Block.DataHash))
		}},
		{"tx_search", func() (any, error) {
			return c.TxSearch(ctx, fmt.Sprintf("tx.height=%d", height), true, nil, nil, "asc")
		}, func(t *testing.T, got any) {
			r := got.(*coretypes.ResultTxSearch)
			require.Equal(t, 1, r.TotalCount)
			require.Len(t, r.Txs, 1)
			require.Equal(t, tx, r.Txs[0].Tx)
			require.NoError(t, r.Txs[0].Proof.Validate(block.Block.DataHash))
		}},
	} {
		t.Run(route.name, func(t *testing.T) {
			for range 2 {
				before := writes.Load()
				result, err := route.call()
				require.NoError(t, err)
				require.Equal(t, before+1, writes.Load(), "each observation must cross HTTP")
				route.check(t, result)
			}
		})
	}

	// Refusals go through the public consumer and the same real route map.
	t.Run("refusals", func(t *testing.T) {
		badHeight := int64(-1)
		_, err := c.Commit(ctx, &badHeight)
		proofRpcRefusal(t, err)
		_, err = c.ConsensusParams(ctx, &badHeight)
		proofRpcRefusal(t, err)
		_, err = c.BlockchainInfo(ctx, -1, height)
		proofRpcRefusal(t, err)
		_, err = c.BlockSearch(ctx, "block.height=", nil, nil, "asc")
		proofRpcRefusal(t, err)
		_, err = c.BroadcastEvidence(ctx, nil)
		proofRpcRefusal(t, err)
		_, err = c.Tx(ctx, crypto.Checksum([]byte("unknown transaction")).Bytes(), true)
		proofRpcRefusal(t, err)
	})

	// Batch decoding must enforce the same status contract while preserving
	// the public result types returned by BatchHTTP.Send.
	batch := c.NewBatch()
	pending, err := batch.Status(ctx)
	require.NoError(t, err)
	health, err := batch.Health(ctx)
	require.NoError(t, err)
	results, err := batch.Send(ctx)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Same(t, pending, results[0])
	require.Same(t, health, results[1])
	require.Equal(t, genesis.ChainID, pending.NodeInfo.Network)

	// consensus/rpc/jsonrpc/server/http_server.go: writeRPCResponse emits an
	// object for one response, even to a batch request. The array decoder in
	// consensus/rpc/jsonrpc/client/decode.go: unmarshalResponseBytesArray
	// currently refuses that shape; these files are outside this task's scope.
	single := c.NewBatch()
	_, err = single.Status(ctx)
	require.NoError(t, err)
	_, err = single.Send(ctx)
	var shapeError *json.UnmarshalTypeError
	require.ErrorAs(t, err, &shapeError)
	require.Equal(t, reflect.TypeOf([]rpctypes.RPCResponse{}), shapeError.Type)

	// Stop and reopen real storage before a new consumer observes history.
	require.NoError(t, closeNode(ctx))
	_, closeNode, err = rpctest.StartTendermint(ctx, conf, app, rpctest.SuppressStdout, rpctest.PreserveRoot)
	require.NoError(t, err)
	reopened, err := rpchttp.New(conf.RPC.ListenAddress)
	require.NoError(t, err)
	for range 2 {
		got, err := reopened.Block(ctx, &height)
		require.NoError(t, err)
		require.Equal(t, block, got)
		indexed, err := reopened.Tx(ctx, tx.Hash().Bytes(), true)
		require.NoError(t, err)
		require.Equal(t, tx, indexed.Tx)
		require.NoError(t, indexed.Proof.Validate(block.Block.DataHash))
	}
}

func proofRpcRefusal(t *testing.T, err error) {
	t.Helper()
	var rpcError *rpctypes.RPCError
	require.ErrorAs(t, err, &rpcError)
	require.NotZero(t, rpcError.Code)
}

func proofRpcEvidence(t *testing.T, key crypto.PrivKey, chainID string, height int64, at time.Time, vals *types.ValidatorSet) *types.DuplicateVoteEvidence {
	t.Helper()
	votes := make([]*types.Vote, 2)
	for i := range votes {
		vote := &types.Vote{
			ValidatorAddress: key.Public().Address(), ValidatorIndex: 0,
			Height: height, Round: 0, Type: tmproto.PrevoteType, Timestamp: at,
			BlockID: types.BlockID{
				Hash:          crypto.Checksum([]byte(fmt.Sprintf("equivocation-%d", i))).Bytes(),
				PartSetHeader: types.PartSetHeader{Total: 1, Hash: crypto.Checksum([]byte("part")).Bytes()},
			},
		}
		vote.Signature = utils.Some(key.Sign(types.VoteSignBytes(chainID, vote.ToProto())))
		votes[i] = vote
	}
	ev, err := types.NewDuplicateVoteEvidence(votes[0], votes[1], at, vals)
	require.NoError(t, err)
	require.NoError(t, ev.ValidateBasic())
	return ev
}

func TestProofRpcRoutesMalformedStatus(t *testing.T) {
	// Environment.Status has no requirement that NodeInfo be configured. This
	// real producer therefore emits a semantically malformed status document.
	app := kvstore.NewApplication()
	defer func() { require.NoError(t, app.Close()) }()
	states := sm.NewStore(dbm.NewMemDB())
	defer func() { require.NoError(t, states.Close()) }()
	blocks := store.NewBlockStore(dbm.NewMemDB())
	defer func() { require.NoError(t, blocks.Close()) }()
	env := &core.Environment{App: proxy.New(app, proxy.NopMetrics()), StateStore: states, BlockStore: blocks}
	produced, err := env.Status(t.Context())
	require.NoError(t, err)
	require.Empty(t, produced.NodeInfo.Network)
	mux := http.NewServeMux()
	rpc.RegisterRPCFuncs(mux, core.NewRoutesMap(env, nil))
	server := httptest.NewServer(mux)
	defer server.Close()

	// Confirm the actual producer emitted a successful JSON-RPC response;
	// the refusal belongs to the consumer's result decoder.
	raw, err := jsonclient.New(server.URL)
	require.NoError(t, err)
	var emitted coretypes.ResultStatus
	require.NoError(t, raw.Call(t.Context(), "status", nil, &emitted))
	producerJSON, err := tmjson.Marshal(produced)
	require.NoError(t, err)
	emittedJSON, err := tmjson.Marshal(&emitted)
	require.NoError(t, err)
	require.Equal(t, producerJSON, emittedJSON)
	consumer, err := rpchttp.New(server.URL)
	require.NoError(t, err)
	for range 2 {
		result, err := consumer.Status(t.Context())
		require.ErrorIs(t, err, rpchttp.ErrInvalidStatusResponse)
		require.Nil(t, result)
		batch := consumer.NewBatch()
		_, err = batch.Status(t.Context())
		require.NoError(t, err)
		_, err = batch.Health(t.Context())
		require.NoError(t, err)
		_, err = batch.Send(t.Context())
		require.ErrorIs(t, err, rpchttp.ErrInvalidStatusResponse)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	conf := config.DefaultConfig()
	conf.RPC.ListenAddress = server.URL
	require.ErrorIs(t, rpctest.WaitForRPC(ctx, conf), context.Canceled)
	require.ErrorIs(t, rpctest.WaitForRPC(t.Context(), conf), rpchttp.ErrInvalidStatusResponse)

	// Once configured, the same real producer is accepted by both consumers.
	env.NodeInfo.Network = "proof-status"
	for range 2 {
		require.NoError(t, rpctest.WaitForRPC(t.Context(), conf))
		status, err := consumer.Status(t.Context())
		require.NoError(t, err)
		require.Equal(t, "proof-status", status.NodeInfo.Network)
	}
}

// consensus/rpc/jsonrpc/types/types.go: RPCRequest.MakeError currently maps
// Autobahn's unavailable sentinels to CodeInternalError. The HTTP consumer
// observes that code, while the producer retains the precise sentinel; changing
// that wire policy is outside this task's touches.
func TestProofRpcRoutesAutobahnRefusals(t *testing.T) {
	rng := utils.TestRng()
	app := kvstore.NewApplication()
	defer func() { require.NoError(t, app.Close()) }()
	states := sm.NewStore(dbm.NewMemDB())
	defer func() { require.NoError(t, states.Close()) }()
	blocks := store.NewBlockStore(dbm.NewMemDB())
	defer func() { require.NoError(t, blocks.Close()) }()
	proxyApp := proxy.New(app, proxy.NopMetrics())
	_, keys := atypes.GenCommittee(rng, 1)
	key := keys[0]
	nodeKey := p2p.NodeSecretKey(ed25519.TestSecretKey(utils.GenBytes(rng, 32)))
	addr := tcp.TestReserveAddr()
	genesis := &types.GenesisDoc{ChainID: "proof-autobahn", InitialHeight: 1}
	require.NoError(t, genesis.ValidateAndComplete())
	info := types.NodeInfo{NodeID: nodeKey.Public().NodeID(), ListenAddr: addr.String(), Network: genesis.ChainID}
	endpoint := p2p.Endpoint{AddrPort: addr}
	peers := dbm.NewMemDB()
	defer func() { require.NoError(t, peers.Close()) }()
	router, err := p2p.NewRouter(p2p.NopMetrics(), nodeKey, func() *types.NodeInfo { return &info }, peers, &p2p.RouterOptions{
		SelfAddress: utils.Some(endpoint.NodeAddress(nodeKey.Public().NodeID())), Endpoint: endpoint,
		Connection: conn.DefaultMConnConfig(), IncomingConnectionWindow: utils.Some(time.Duration(0)),
		MaxAcceptRate: utils.Some(rate.Inf), MaxDialRate: utils.Some(rate.Limit(30)),
		Giga: utils.Some(&p2p.GigaRouterConfig{
			DialInterval: time.Second,
			ValidatorAddrs: map[atypes.PublicKey]p2p.GigaNodeAddr{key.Public(): {
				Key: nodeKey.Public(), HostPort: tcp.HostPort{Hostname: addr.Addr().String(), Port: addr.Port()},
			}},
			Consensus: &aconsensus.Config{
				Key: key, ViewTimeout: func(atypes.View) time.Duration { return time.Hour },
				PersistentStateDir: utils.None[string](),
			},
			Producer: &producer.Config{
				App: proxyApp, MaxGasWantedPerBlock: 1000, MaxGasEstimatedPerBlock: 1000,
				MaxTxsPerBlock: 1, MaxTxsPerSecond: utils.None[uint64](), BlockInterval: time.Hour,
			},
			GenDoc: genesis, StateStore: states,
		}),
	})
	require.NoError(t, err)
	env := &core.Environment{App: proxyApp, StateStore: states, BlockStore: blocks, Router: router, GenDoc: genesis, NodeInfo: info}
	mux := http.NewServeMux()
	rpc.RegisterRPCFuncs(mux, core.NewRoutesMap(env, nil))
	server := httptest.NewServer(mux)
	defer server.Close()
	c, err := rpchttp.New(server.URL)
	require.NoError(t, err)
	ctx := t.Context()
	for _, route := range []struct {
		name     string
		producer func() (any, error)
		consumer func() (any, error)
		sentinel error
	}{
		{"commit", func() (any, error) { return env.Commit(ctx, &coretypes.RequestBlockInfo{}) }, func() (any, error) { return c.Commit(ctx, nil) }, coretypes.ErrCommitNotAvailable},
		{"consensus_state", func() (any, error) { return env.GetConsensusState(ctx) }, func() (any, error) { return c.ConsensusState(ctx) }, coretypes.ErrConsensusStateNotAvailable},
		{"dump_consensus_state", func() (any, error) { return env.DumpConsensusState(ctx) }, func() (any, error) { return c.DumpConsensusState(ctx) }, coretypes.ErrConsensusStateNotAvailable},
		{"broadcast_evidence", func() (any, error) { return env.BroadcastEvidence(ctx, &coretypes.RequestBroadcastEvidence{}) }, func() (any, error) { return c.BroadcastEvidence(ctx, nil) }, coretypes.ErrEvidenceNotAccepted},
	} {
		t.Run(route.name, func(t *testing.T) {
			for range 2 {
				result, err := route.producer()
				require.ErrorIs(t, err, route.sentinel)
				require.Nil(t, result)
				result, err = route.consumer()
				var wireError *rpctypes.RPCError
				require.ErrorAs(t, err, &wireError)
				require.Equal(t, int(rpctypes.CodeInternalError), wireError.Code)
				require.Nil(t, result)
			}
		})
	}
}
