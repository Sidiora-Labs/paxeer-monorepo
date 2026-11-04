package rpc_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventbus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/mempool"
	appproxy "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer"
	kvsink "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink/kv"
	blockstore "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/provider"
	lighthttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/provider/http"
	lightproxy "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/proxy"
	lrpc "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/rpc"
	lightstore "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/store"
	lightdb "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/store/db"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/commitment"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/memiavl"
)

const repair84ChainID = "repair-84"

type repair84Chain struct {
	env    *core.Environment
	blocks []*types.Block
	roots  []*types.LightBlock
}

func repair84Genesis(t *testing.T) (sm.State, *privval.FilePV) {
	t.Helper()
	key := ed25519.GenerateSecretKey()
	state, err := sm.MakeGenesisState(&types.GenesisDoc{
		ChainID: repair84ChainID, GenesisTime: time.Now().Add(-time.Minute).UTC(),
		Validators: []types.GenesisValidator{{Address: key.Public().Address(), PubKey: key.Public(), Power: 10}},
	})
	require.NoError(t, err)
	dir := t.TempDir()
	return state, privval.NewFilePV(key, filepath.Join(dir, "key.json"), filepath.Join(dir, "state.json"))
}

func repair84Sign(t *testing.T, state sm.State, signer *privval.FilePV, block *types.Block) (*types.PartSet, *types.LightBlock) {
	t.Helper()
	parts, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	id := types.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
	votes := types.NewVoteSet(repair84ChainID, block.Height, 0, tmproto.PrecommitType, state.Validators)
	commit, err := types.MakeCommit(t.Context(), id, block.Height, 0, votes, []types.PrivValidator{signer}, block.Time)
	require.NoError(t, err)
	lb := &types.LightBlock{SignedHeader: &types.SignedHeader{Header: &block.Header, Commit: commit}, ValidatorSet: state.Validators.Copy()}
	require.NoError(t, lb.ValidateBasic(repair84ChainID))
	require.NoError(t, lb.ValidatorSet.VerifyCommitLight(repair84ChainID, id, block.Height, commit))
	return parts, lb
}

func repair84Execute(t *testing.T) *repair84Chain {
	t.Helper()
	state, signer := repair84Genesis(t)
	app := kvstore.NewApplication()
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	ap := appproxy.New(app, appproxy.NopMetrics())
	ss := sm.NewStore(dbm.NewMemDB())
	bs := blockstore.NewBlockStore(dbm.NewMemDB())
	sink := kvsink.NewEventSink(dbm.NewMemDB())
	t.Cleanup(func() {
		require.NoError(t, ss.Close())
		require.NoError(t, bs.Close())
		require.NoError(t, sink.Stop())
	})
	require.NoError(t, ss.Save(state))
	bus := eventbus.NewDefault()
	require.NoError(t, bus.Start(t.Context()))
	t.Cleanup(bus.Stop)
	mp := mempool.NewTxMempool(mempool.TestConfig(), ap, mempool.NopMetrics(), mempool.NopTxConstraintsFetcher)
	executor := sm.NewBlockExecutor(ss, ap, mp, sm.EmptyEvidencePool{}, bs, bus, sm.NopMetrics(), types.DefaultConsensusPolicy())
	t.Cleanup(executor.Stop)
	chain := &repair84Chain{env: &core.Environment{App: ap, BlockStore: bs, StateStore: ss, EventSinks: []indexer.EventSink{sink}, NodeInfo: types.NodeInfo{Network: repair84ChainID}, EventBus: bus}}
	lastCommit := &types.Commit{}
	for h := int64(1); h <= 5; h++ {
		txs := types.Txs{types.Tx(fmt.Sprintf("key%d=value%d", h, h)), types.Tx(fmt.Sprintf("other%d=second%d", h, h))}
		if h == 2 {
			txs = nil
		}
		block := state.MakeBlockAt(h, txs, lastCommit, nil, state.Validators.GetProposer().Address, state.LastBlockTime.Add(time.Second))
		parts, lb := repair84Sign(t, state, signer, block)
		next, err := executor.ApplyBlock(t.Context(), state, lb.Commit.BlockID, block, nil)
		require.NoError(t, err)
		bs.SaveBlock(block, parts, lb.Commit)
		response, err := ss.LoadFinalizeBlockResponses(h)
		require.NoError(t, err)
		for i, tx := range txs {
			require.NoError(t, sink.IndexTxEvents([]*abci.TxResultV2{{Height: h, Index: uint32(i), Tx: tx, Result: *response.TxResults[i]}}))
		}
		chain.blocks = append(chain.blocks, block)
		chain.roots = append(chain.roots, lb)
		state = next
		lastCommit = lb.Commit
	}
	return chain
}

func repair84Node(t *testing.T, env *core.Environment, amend func(map[string]*rpcserver.RPCFunc)) (*rpchttp.HTTP, string) {
	t.Helper()
	routes := core.NewRoutesMap(env, nil)
	if amend != nil {
		amend(routes)
	}
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, routes)
	wm := rpcserver.NewWebsocketManager(routes)
	mux.HandleFunc("/websocket", wm.WebsocketHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client, err := rpchttp.New(server.URL)
	require.NoError(t, err)
	return client, server.URL
}

func repair84Light(t *testing.T, url string, trusted lightstore.Store, root *types.LightBlock) *light.Client {
	t.Helper()
	p, err := lighthttp.New(repair84ChainID, url)
	require.NoError(t, err)
	client, err := light.NewClient(t.Context(), repair84ChainID, light.TrustOptions{Period: time.Hour, Height: root.Height, Hash: root.Hash()}, p, []provider.Provider{p}, trusted, time.Hour)
	require.NoError(t, err)
	return client
}

func repair84Proxy(t *testing.T, lc *light.Client, url string) (*rpchttp.HTTP, *lightproxy.Proxy) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	proxy, err := lightproxy.NewProxy(lc, "tcp://"+address, url, rpcserver.DefaultConfig(), lrpc.KeyPathFn(lrpc.DefaultMerkleKeyPathFn()))
	require.NoError(t, err)
	proxy.Client.RegisterOpDecoder(storetypes.ProofOpIAVLCommitment, storetypes.CommitmentOpDecoder)
	proxy.Client.RegisterOpDecoder(storetypes.ProofOpSimpleMerkleCommitment, storetypes.CommitmentOpDecoder)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- proxy.ListenAndServe(ctx) }()
	t.Cleanup(func() {
		cancel()
		proxy.Client.Stop()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("proxy did not stop")
		}
	})
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		conn, err := net.DialTimeout("tcp", address, 50*time.Millisecond)
		if err == nil {
			require.NoError(t, conn.Close())
			break
		}
		select {
		case err := <-done:
			t.Fatalf("proxy startup: %v", err)
		case <-deadline.C:
			t.Fatal("proxy did not listen")
		case <-time.After(5 * time.Millisecond):
		}
	}
	client, err := rpchttp.New("http://" + address)
	require.NoError(t, err)
	return client, proxy
}

func TestRepair_84_ResultCommitment(t *testing.T) {
	require.False(t, types.SkipLastResultsHashValidation.Load())
	chain := repair84Execute(t)
	var corrupt atomic.Bool
	node, url := repair84Node(t, chain.env, func(routes map[string]*rpcserver.RPCFunc) {
		routes["block_results"] = rpcserver.NewRPCFunc(func(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultBlockResults, error) {
			res, err := chain.env.BlockResults(ctx, req)
			if err == nil && corrupt.Load() && len(res.TxsResults) > 0 {
				res.TxsResults[0].GasUsed++
			}
			return res, err
		})
	})
	lc := repair84Light(t, url, lightdb.New(dbm.NewMemDB()), chain.roots[0])
	consumer, _ := repair84Proxy(t, lc, url)
	for _, height := range []int64{1, 2, 3} {
		expected, err := node.BlockResults(t.Context(), &height)
		require.NoError(t, err)
		leaves, err := abci.MarshalTxResults(expected.TxsResults)
		require.NoError(t, err)
		require.Equal(t, merkle.HashFromByteSlices(leaves), []byte(chain.blocks[height].LastResultsHash))
		actual, err := consumer.BlockResults(t.Context(), &height)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
	corrupt.Store(true)
	height := int64(1)
	_, err := consumer.BlockResults(t.Context(), &height)
	require.ErrorContains(t, err, "last results")
}

func TestRepair_84_Status(t *testing.T) {
	chain := repair84Execute(t)
	_, url := repair84Node(t, chain.env, nil)
	lc := repair84Light(t, url, lightdb.New(dbm.NewMemDB()), chain.roots[0])
	consumer, _ := repair84Proxy(t, lc, url)
	status, err := consumer.Status(t.Context())
	require.NoError(t, err)
	require.Equal(t, repair84ChainID, status.NodeInfo.Network)
	require.Equal(t, int64(1), status.LightClientInfo.LastTrustedHeight)
	require.Equal(t, chain.roots[0].Hash(), status.LightClientInfo.LastTrustedHash)
	batch := consumer.NewBatch()
	deferred, err := batch.Status(t.Context())
	require.NoError(t, err)
	results, err := batch.Send(t.Context())
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, status, deferred)
	require.Equal(t, status, results[0])
}

func TestRepair_84_MissingHashes(t *testing.T) {
	chain := repair84Execute(t)
	node, url := repair84Node(t, chain.env, nil)
	lc := repair84Light(t, url, lightdb.New(dbm.NewMemDB()), chain.roots[0])
	consumer, _ := repair84Proxy(t, lc, url)
	for _, tc := range []struct {
		name  string
		hash  []byte
		found bool
	}{
		{"known", chain.blocks[2].Hash(), true}, {"unknown", make([]byte, 32), false}, {"nil", nil, false}, {"empty", []byte{}, false}, {"short", []byte{1}, false}, {"long", make([]byte, 33), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodeBlock, err := node.BlockByHash(t.Context(), tc.hash)
			require.NoError(t, err)
			nodeHeader, err := node.HeaderByHash(t.Context(), tc.hash)
			require.NoError(t, err)
			block, err := consumer.BlockByHash(t.Context(), tc.hash)
			require.NoError(t, err)
			header, err := consumer.HeaderByHash(t.Context(), tc.hash)
			require.NoError(t, err)
			require.Equal(t, nodeBlock, block)
			require.Equal(t, nodeHeader, header)
			if tc.found {
				require.NotNil(t, block.Block)
				require.Equal(t, block.Block.Header, *header.Header)
			} else {
				require.Nil(t, block.Block)
				require.True(t, block.BlockID.IsNil())
				require.Nil(t, header.Header)
			}
		})
	}
}

func TestRepair_84_TransactionBindings(t *testing.T) {
	chain := repair84Execute(t)
	var attack atomic.Int32
	node, url := repair84Node(t, chain.env, func(routes map[string]*rpcserver.RPCFunc) {
		routes["tx"] = rpcserver.NewRPCFunc(func(ctx context.Context, req *coretypes.RequestTx) (*coretypes.ResultTx, error) {
			if attack.Load() == 1 {
				req.Hash = chain.blocks[0].Txs[1].Hash().Bytes()
			}
			res, err := chain.env.Tx(ctx, req)
			if err != nil {
				return res, err
			}
			switch attack.Load() {
			case 2:
				res.Tx = chain.blocks[0].Txs[1]
			case 3:
				res.Hash = chain.blocks[0].Txs[1].Hash().Bytes()
			case 4:
				res.Index++
			case 5:
				res.Tx = chain.blocks[0].Txs[1]
				res.Hash = res.Tx.Hash().Bytes()
			case 6:
				res.Proof = chain.blocks[0].Txs.Proof(1)
			}
			return res, nil
		})
	})
	lc := repair84Light(t, url, lightdb.New(dbm.NewMemDB()), chain.roots[0])
	consumer, _ := repair84Proxy(t, lc, url)
	hash := chain.blocks[0].Txs[0].Hash().Bytes()
	valid, err := node.Tx(t.Context(), hash, true)
	require.NoError(t, err)
	require.NoError(t, valid.Proof.Validate(chain.roots[0].DataHash))
	result, err := consumer.Tx(t.Context(), hash, true)
	require.NoError(t, err)
	require.Equal(t, valid, result)
	for mode := int32(1); mode <= 6; mode++ {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			attack.Store(mode)
			supplied, err := node.Tx(t.Context(), hash, true)
			require.NoError(t, err)
			require.NoError(t, supplied.Proof.Validate(chain.roots[0].DataHash), "attack must carry a real valid inclusion proof")
			_, err = consumer.Tx(t.Context(), hash, true)
			require.Error(t, err)
		})
	}
}

func TestRepair_84_DescendingRangeReopenedSparseStore(t *testing.T) {
	chain := repair84Execute(t)
	node, url := repair84Node(t, chain.env, nil)
	dir := t.TempDir()
	db, err := dbm.NewDB("trusted", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	trusted := lightdb.New(db)
	require.NoError(t, trusted.SaveLightBlock(chain.roots[0]))
	require.NoError(t, db.Close())
	db, err = dbm.NewDB("trusted", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	trusted = lightdb.New(db)
	require.EqualValues(t, 1, trusted.Size())
	lc := repair84Light(t, url, trusted, chain.roots[0])
	consumer, proxy := repair84Proxy(t, lc, url)
	expected, err := node.BlockchainInfo(t.Context(), 2, 5)
	require.NoError(t, err)
	actual, err := consumer.BlockchainInfo(t.Context(), 2, 5)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
	require.Len(t, actual.BlockMetas, 4)
	for i, meta := range actual.BlockMetas {
		require.Equal(t, int64(5-i), meta.Header.Height)
	}
	proxy.Client.Stop()
	require.NoError(t, db.Close())
	db, err = dbm.NewDB("trusted", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	trusted = lightdb.New(db)
	recovered := repair84Light(t, url, trusted, chain.roots[0])
	for h := int64(1); h <= 5; h++ {
		stored, err := recovered.TrustedLightBlock(h)
		require.NoError(t, err)
		require.Equal(t, chain.roots[h-1].Hash(), stored.Hash())
	}
	verified := lrpc.NewClient(node, recovered)
	again, err := verified.BlockchainInfo(t.Context(), 2, 5)
	require.NoError(t, err)
	require.Equal(t, expected, again)
}

func TestRepair_84_QueryKeyAndSnapshot(t *testing.T) {
	// Produce genuine SDK store queries and ICS23 proofs for two committed versions.
	tree := memiavl.New(0)
	t.Cleanup(func() { require.NoError(t, tree.Close()) })
	stores := make([]*commitment.Store, 2)
	infos := make([]storetypes.CommitInfo, 2)
	for version := int64(1); version <= 2; version++ {
		tree.Set([]byte("requested"), []byte(fmt.Sprintf("value%d", version)))
		tree.Set([]byte("substituted"), []byte("other value"))
		_, v, err := tree.SaveVersion(true)
		require.NoError(t, err)
		require.Equal(t, version, v)
		snapshot := tree.Copy()
		t.Cleanup(func() { require.NoError(t, snapshot.Close()) })
		stores[version-1] = commitment.NewStore(snapshot)
		infos[version-1] = storetypes.CommitInfo{Version: version, StoreInfos: []storetypes.StoreInfo{{Name: "bank", CommitId: storetypes.CommitID{Version: version, Hash: snapshot.RootHash()}}}}
	}
	// The application roots become header AppHash at the following height.
	state, signer := repair84Genesis(t)
	ss := sm.NewStore(dbm.NewMemDB())
	bs := blockstore.NewBlockStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, ss.Close()); require.NoError(t, bs.Close()) })
	require.NoError(t, ss.Save(state))
	env := &core.Environment{StateStore: ss, BlockStore: bs}
	var roots []*types.LightBlock
	last := &types.Commit{}
	for height := int64(1); height <= 3; height++ {
		if height > 1 {
			state.AppHash = infos[height-2].Hash()
		}
		block := state.MakeBlockAt(height, nil, last, nil, state.Validators.GetProposer().Address, state.LastBlockTime.Add(time.Second))
		parts, lb := repair84Sign(t, state, signer, block)
		bs.SaveBlock(block, parts, lb.Commit)
		next, err := state.Update(lb.Commit.BlockID, &block.Header, nil, nil, nil)
		require.NoError(t, err)
		require.NoError(t, ss.Save(next))
		state = next
		roots = append(roots, lb)
		last = lb.Commit
	}
	var attack atomic.Int32
	node, url := repair84Node(t, env, func(routes map[string]*rpcserver.RPCFunc) {
		routes["abci_query"] = rpcserver.NewRPCFunc(func(ctx context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			height := int64(req.Height)
			key := req.Data
			switch attack.Load() {
			case 1:
				key = []byte("substituted")
			case 2:
				height = 2
			}
			response := stores[height-1].Query(ctx, abci.RequestQuery{Path: "/key", Data: key, Height: height, Prove: req.Prove})
			if response.IsOK() {
				response.ProofOps.Ops = append(response.ProofOps.Ops, infos[height-1].ProofOp("bank"))
			}
			return &coretypes.ResultABCIQuery{Response: response}, nil
		})
		// Latest-height selection also goes through the node's real Status handler.
		app := kvstore.NewApplication()
		t.Cleanup(func() { require.NoError(t, app.Close()) })
		env.App = appproxy.New(app, appproxy.NopMetrics())
		env.NodeInfo = types.NodeInfo{Network: repair84ChainID}
	})
	lc := repair84Light(t, url, lightdb.New(dbm.NewMemDB()), roots[0])
	consumer, proxy := repair84Proxy(t, lc, url)
	options := rpcclient.ABCIQueryOptions{Height: 1, Prove: true}
	expected, err := node.ABCIQueryWithOptions(t.Context(), "/store/bank/key", []byte("requested"), options)
	require.NoError(t, err)
	result, err := consumer.ABCIQueryWithOptions(t.Context(), "/store/bank/key", []byte("requested"), options)
	require.NoError(t, err)
	require.Equal(t, expected, result)
	for _, mode := range []int32{1, 2} {
		attack.Store(mode)
		supplied, err := node.ABCIQueryWithOptions(t.Context(), "/store/bank/key", []byte("requested"), options)
		require.NoError(t, err)
		proofRuntime := merkle.NewProofRuntime()
		proofRuntime.RegisterOpDecoder(storetypes.ProofOpIAVLCommitment, storetypes.CommitmentOpDecoder)
		proofRuntime.RegisterOpDecoder(storetypes.ProofOpSimpleMerkleCommitment, storetypes.CommitmentOpDecoder)
		path, err := lrpc.DefaultMerkleKeyPathFn()("/store/bank/key", supplied.Response.Key)
		require.NoError(t, err)
		require.NoError(t, proofRuntime.VerifyValue(supplied.Response.ProofOps, infos[supplied.Response.Height-1].Hash(), path.String(), supplied.Response.Value))
		_, err = consumer.ABCIQueryWithOptions(t.Context(), "/store/bank/key", []byte("requested"), options)
		require.Error(t, err)
	}
	attack.Store(0)
	for _, key := range []string{"substituted", "absent"} {
		result, err := consumer.ABCIQueryWithOptions(t.Context(), "/store/bank/key", []byte(key), options)
		require.NoError(t, err)
		require.Equal(t, []byte(key), result.Response.Key)
		if key == "absent" {
			require.Nil(t, result.Response.Value)
		}
	}
	// Zero height resolves to a concrete snapshot before requesting or checking a proof.
	latest, err := proxy.Client.ABCIQuery(t.Context(), "/store/bank/key", []byte("requested"))
	require.NoError(t, err)
	require.Equal(t, int64(2), latest.Response.Height)
	attack.Store(1)
	_, err = proxy.Client.ABCIQuery(t.Context(), "/store/bank/key", []byte("requested"))
	require.Error(t, err)
	for _, badPath := range []string{"prefix/store/bank/key", "/store/bank/key/suffix", "/store/bank/extra/key"} {
		_, err := lrpc.DefaultMerkleKeyPathFn()(badPath, []byte("requested"))
		require.Error(t, err)
	}
}
