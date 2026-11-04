package statesync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/test/factory"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	pb "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/statesync"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func testRepairM1183BlockQueueClose(t *testing.T) {
	t.Helper()
	for _, shutdown := range []string{"close", "retry", "success"} {
		t.Run(shutdown, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			q := newBlockQueue(1, 1, 1, time.Now(), 1)
			require.Equal(t, int64(1), <-q.nextHeight())
			fetch := q.nextHeight()
			verify := q.verifyNext()
			if shutdown == "success" {
				// A real terminal block completes the pending verification first.
				q.add(mockLBResp(ctx, factory.NodeID(t, "a"), 1, time.Now()))
				resp := <-verify
				require.Equal(t, int64(1), resp.block.Height)
				// Keep an active verifier notification while success shuts down.
				verify = q.verifyNext()
			}
			err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
				s.Spawn(func() error {
					select {
					case _, ok := <-fetch:
						if ok {
							return errors.New("shutdown delivered a fetch height")
						}
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
				s.Spawn(func() error {
					select {
					case _, ok := <-verify:
						if ok {
							return errors.New("shutdown delivered a verification block")
						}
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
				switch shutdown {
				case "close":
					s.Spawn(func() error { q.close(); return nil })
				case "retry":
					q.retry(1)
				case "success":
					q.success()
				}
				// Competing shutdown paths must all finish without panicking.
				for range 8 {
					s.Spawn(func() error { q.close(); return nil })
				}
				return nil
			})
			require.NoError(t, err)
			select {
			case <-q.done():
			default:
				t.Fatal("queue did not finish")
			}
			q.close()
			_, ok := <-q.nextHeight()
			require.False(t, ok)
			_, ok = <-q.verifyNext()
			require.False(t, ok)
			if shutdown == "retry" {
				require.Error(t, q.error())
			} else {
				require.NoError(t, q.error())
			}
		})
	}
}

func TestRepairM_1183_BackfillClose(t *testing.T) {
	testRepairM1183BlockQueueClose(t)
}

// This application is a committed snapshot announcement fixture. Both the wire
// response and its decoding/consumption are produced by the real reactors.
type repairM1183SnapshotApp struct {
	abci.BaseApplication
	snapshots []*abci.Snapshot
}

func (app repairM1183SnapshotApp) ListSnapshots(context.Context, *abci.RequestListSnapshots) (*abci.ResponseListSnapshots, error) {
	return &abci.ResponseListSnapshots{Snapshots: app.snapshots}, nil
}

func repairM1183SnapshotPair(t *testing.T, snapshots []*abci.Snapshot) (*Reactor, *Reactor, types.NodeID) {
	t.Helper()
	network := p2p.MakeTestNetwork(t, p2p.TestNetworkOptions{NumNodes: 2})
	nodes := network.Nodes()
	reactors := make([]*Reactor, 2)
	for i, node := range nodes {
		stateDB, blockDB := dbm.NewMemDB(), dbm.NewMemDB()
		t.Cleanup(func() {
			require.NoError(t, stateDB.Close())
			require.NoError(t, blockDB.Close())
		})
		r, err := NewReactor(factory.DefaultTestChainID, 1, *config.DefaultStateSyncConfig(),
			proxy.New(repairM1183SnapshotApp{snapshots: snapshots}, proxy.NopMetrics()), node.Router,
			sm.NewStore(stateDB), store.NewBlockStore(blockDB), t.TempDir(), NopMetrics(),
			nil, nil, false, func() {}, config.DefaultSelfRemediationConfig())
		require.NoError(t, err)
		reactors[i] = r
	}
	reactors[0].syncer = &syncer{snapshots: newSnapshotPool(), metrics: NopMetrics()}
	network.Start(t)
	return reactors[0], reactors[1], nodes[1].NodeID
}

func TestRepairM_1183_SnapshotAllocation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	counts := []uint32{maxSnapshotChunks - 1, maxSnapshotChunks, maxSnapshotChunks + 1, ^uint32(0)}
	var announcements []*abci.Snapshot
	for i, count := range counts {
		announcements = append(announcements, &abci.Snapshot{Height: uint64(i + 1), Format: 1, Chunks: count, Hash: []byte{byte(i + 1)}})
	}
	client, producer, producerID := repairM1183SnapshotPair(t, announcements)
	client.snapshotChannel.Send(wrap(&pb.SnapshotsRequest{}), producerID)
	request, err := producer.snapshotChannel.Recv(ctx)
	require.NoError(t, err)
	require.NoError(t, producer.handleSnapshotMessage(ctx, request))
	for range announcements {
		response, err := client.snapshotChannel.Recv(ctx)
		require.NoError(t, err)
		require.NotNil(t, response.Message.GetSnapshotsResponse())
		require.NoError(t, client.handleSnapshotMessage(ctx, response))
	}
	ranked := client.syncer.snapshots.Ranked()
	require.Len(t, ranked, 2)
	require.Equal(t, maxSnapshotChunks, ranked[0].Chunks)
	require.Equal(t, maxSnapshotChunks-1, ranked[1].Chunks)
	q, err := newChunkQueue(ranked[0], t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, q.Close()) })
	require.Equal(t, maxSnapshotChunks, q.Size())
	index, err := q.Allocate()
	require.NoError(t, err)
	require.Zero(t, index)
	payload := []byte("bounded snapshot data")
	added, err := q.Add(&chunk{Height: ranked[0].Height, Format: 1, Index: index, Chunk: payload, Sender: producerID})
	require.NoError(t, err)
	require.True(t, added)
	got, err := q.Next()
	require.NoError(t, err)
	require.Equal(t, payload, got.Chunk)
	require.Equal(t, producerID, q.GetSender(index))
	last := q.WaitFor(maxSnapshotChunks - 1)
	added, err = q.Add(&chunk{Height: ranked[0].Height, Format: 1, Index: maxSnapshotChunks - 1, Chunk: payload})
	require.NoError(t, err)
	require.True(t, added)
	require.Equal(t, maxSnapshotChunks-1, <-last)
	_, err = q.Add(&chunk{Height: ranked[0].Height, Format: 1, Index: maxSnapshotChunks, Chunk: payload})
	require.Error(t, err)
	// Invalid metadata must fail before creating a temporary directory, including
	// direct/local queue construction that does not use the network pool.
	for _, count := range []uint32{0, maxSnapshotChunks + 1, ^uint32(0)} {
		dir := t.TempDir()
		invalid, err := newChunkQueue(&snapshot{Chunks: count}, dir)
		require.Error(t, err)
		require.Nil(t, invalid)
		files, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, files)
	}
}

func TestRepairM_1183_DispatcherClose(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		client, server, serverID := repairM1183SnapshotPair(t, nil)
		d := NewDispatcher(client.lightBlockChannel)
		result := make(chan error, 1)
		s.Spawn(func() error {
			lb, err := d.LightBlock(ctx, 1, serverID)
			if err == nil && lb != nil {
				err = errors.New("closed request returned a block")
			}
			result <- err
			return nil
		})
		req, err := server.lightBlockChannel.Recv(ctx)
		if err != nil {
			return err
		}
		require.Equal(t, uint64(1), req.Message.GetLightBlockRequest().Height)
		d.Close()
		d.Close()
		select {
		case err := <-result:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("Close did not promptly complete live request")
		}
		// The real producer has no block at this height and emits the late nil reply.
		require.NoError(t, server.handleLightBlockMessage(ctx, req))
		late, err := client.lightBlockChannel.Recv(ctx)
		if err != nil {
			return err
		}
		require.ErrorIs(t, d.Respond(ctx, late.Message.GetLightBlockResponse().LightBlock, late.From), ErrUnsolicitedResponse)
		_, err = d.LightBlock(ctx, 1, serverID)
		require.ErrorIs(t, err, ErrDisconnected)
		for calls := range d.calls.Lock() {
			require.Empty(t, calls)
		}
		return nil
	})
	require.NoError(t, err)
}

func TestRepairM_1183_DispatcherCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	client, server, serverID := repairM1183SnapshotPair(t, nil)
	d := NewDispatcher(client.lightBlockChannel)
	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		requestCtx, stop := context.WithCancel(ctx)
		defer stop()
		result := make(chan error, 1)
		s.Spawn(func() error { _, err := d.LightBlock(requestCtx, 1, serverID); result <- err; return nil })
		req, err := server.lightBlockChannel.Recv(ctx)
		if err != nil {
			return err
		}
		stop()
		select {
		case err := <-result:
			require.ErrorIs(t, err, context.Canceled)
		case <-ctx.Done():
			return ctx.Err()
		}
		require.NoError(t, server.handleLightBlockMessage(ctx, req))
		late, err := client.lightBlockChannel.Recv(ctx)
		if err != nil {
			return err
		}
		require.ErrorIs(t, d.Respond(ctx, late.Message.GetLightBlockResponse().LightBlock, late.From), ErrUnsolicitedResponse)
		return nil
	})
	require.NoError(t, err)
	d.Close()
}

func TestRepairM_1183_PeerListConcurrent(t *testing.T) { TestPeerListConcurrent(t) }
func TestRepairM_1183_BackfillRecoveredInterval(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client, server, serverID := repairM1183SnapshotPair(t, nil)
	client.dispatcher = NewDispatcher(client.lightBlockChannel)
	client.peers.Append(serverID)
	const startHeight, stopHeight int64 = 20, 10
	stopTime := factory.DefaultTestTime
	chain := buildLightBlockChain(ctx, t, stopHeight-1, startHeight+1, stopTime)
	for height := stopHeight - 1; height <= startHeight; height++ {
		require.NoError(t, server.stateStore.SaveValidatorSets(height, height, chain[height].ValidatorSet))
		require.NoError(t, server.blockStore.SaveSignedHeader(chain[height].SignedHeader, chain[height].Commit.BlockID))
	}
	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBg(func() error {
			for {
				req, err := server.lightBlockChannel.Recv(ctx)
				if err != nil {
					return nil
				}
				if err := server.handleLightBlockMessage(ctx, req); err != nil {
					return err
				}
			}
		})
		s.SpawnBg(func() error { client.processLightBlockCh(ctx); return nil })
		return client.backfill(ctx, factory.DefaultTestChainID, startHeight, stopHeight, 1,
			chain[startHeight].Commit.BlockID, stopTime)
	})
	require.NoError(t, err)
	var visited int64
	for height := startHeight; height >= stopHeight; height-- {
		meta := client.blockStore.LoadBlockMeta(height)
		require.NotNil(t, meta)
		require.Equal(t, chain[height].Header, &meta.Header)
		require.Equal(t, chain[height].Commit, client.blockStore.LoadBlockCommit(height))
		vals, err := client.stateStore.LoadValidators(height)
		require.NoError(t, err)
		require.Equal(t, chain[height].ValidatorSet.Hash(), vals.Hash())
		visited++
	}
	require.Equal(t, startHeight-stopHeight+1, visited)
	require.Nil(t, client.blockStore.LoadBlockMeta(stopHeight-1))
	require.Nil(t, client.blockStore.LoadBlockMeta(startHeight+1))
	require.Equal(t, visited, client.BackFilledBlocks())
	require.Equal(t, visited, client.BackFillBlocksTotal())
}
func TestRepairM_1183_ParamsTransition(t *testing.T) {
	t.Run("restored", TestRepair_120_ParamsHeightRestoration)
	t.Run("wrong-height", TestRepair_120_ParamsWrongHeightRefused)
	t.Run("wrong-hash", TestRepair_120_ParamsWrongHashRefused)
}

func TestRepairM_1183_SnapshotComparator(t *testing.T) {
	candidates := []*snapshot{
		{Height: 10, Format: 1, Chunks: 1, Hash: []byte{1}},
		{Height: 10, Format: 9, Chunks: 1, Hash: []byte{2}},
		{Height: 10, Format: 8, Chunks: 1, Hash: []byte{3}},
		{Height: 10, Format: 20, Chunks: 1, Hash: []byte{4}},
	}
	peerCounts := []int{3, 2, 2, 1}
	var permute func([]int, []int)
	permute = func(prefix, remaining []int) {
		if len(remaining) > 0 {
			for i, index := range remaining {
				next := append([]int{}, remaining[:i]...)
				next = append(next, remaining[i+1:]...)
				permute(append(append([]int{}, prefix...), index), next)
			}
			return
		}
		t.Run(fmt.Sprint(prefix), func(t *testing.T) {
			pool := newSnapshotPool()
			for _, index := range prefix {
				for peer := range peerCounts[index] {
					_, err := pool.Add(types.NodeID(fmt.Sprintf("peer-%d", peer)), candidates[index])
					require.NoError(t, err)
				}
			}
			less := pool.sorterFactory(candidates)
			for i := range candidates {
				require.False(t, less(i, i))
				for j := range candidates {
					if i < j {
						require.True(t, less(i, j))
						require.False(t, less(j, i))
					}
				}
			}
			require.Equal(t, candidates, pool.Ranked())
			require.Equal(t, candidates[0], pool.Best())
		})
	}
	permute(nil, []int{0, 1, 2, 3})
}

// Observe wait registration without replacing the context's cancellation or
// deadline behavior, so both discovery branches can be canceled deterministically.
type repairM1183DiscoveryContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (ctx *repairM1183DiscoveryContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.waiting) })
	return ctx.Context.Done()
}

func TestRepairM_1183_DiscoveryCancellation(t *testing.T) {
	for _, local := range []bool{false, true} {
		t.Run(fmt.Sprintf("local=%t", local), func(t *testing.T) {
			client, _, _ := repairM1183SnapshotPair(t, nil)
			syncer := client.syncer
			syncer.useLocalSnapshot = local // initial discovery and repeated empty discovery
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			observed := &repairM1183DiscoveryContext{Context: ctx, waiting: make(chan struct{})}
			completed := make(chan error, 1)
			err := scope.Run(t.Context(), func(_ context.Context, s scope.Scope) error {
				s.Spawn(func() error {
					_, _, err := syncer.SyncAny(observed, minimumDiscoveryTime, func() error {
						client.snapshotChannel.Broadcast(wrap(&pb.SnapshotsRequest{}))
						return nil
					})
					completed <- err
					return nil
				})
				select {
				case <-observed.waiting:
				case <-time.After(time.Second):
					t.Error("discovery did not register a cancellable wait")
				}
				cancel()
				select {
				case err := <-completed:
					assert.ErrorIs(t, err, context.Canceled)
				case <-time.After(time.Second):
					t.Error("empty discovery did not finish on cancellation")
				}
				return nil
			}) // joining proves the discovery goroutine has actually exited
			require.NoError(t, err)
		})
	}
}
