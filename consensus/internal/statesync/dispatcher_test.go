package statesync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/test/factory"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/require"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	pb "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/statesync"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

type dispatcherTestSuite struct {
	network    *p2p.TestNetwork
	node       *p2p.TestNode
	dispatcher *Dispatcher
}

func dispatcherSetup(t *testing.T) *dispatcherTestSuite {
	t.Helper()

	network := p2p.MakeTestNetwork(t, p2p.TestNetworkOptions{
		NumNodes: 1,
		NodeOpts: p2p.TestNodeOptions{
			MaxConnected: utils.Some(100),
		},
	})
	n := network.Nodes()[0]
	ch, err := p2p.OpenChannel(n.Router, GetLightBlockChannelDescriptor())
	if err != nil {
		panic(err)
	}
	network.Start(t)
	t.Cleanup(leaktest.Check(t))

	return &dispatcherTestSuite{
		network:    network,
		node:       n,
		dispatcher: NewDispatcher(ch),
	}
}

type DispatcherNode struct {
	*p2p.TestNode
	blockCh *p2p.Channel[*pb.Message]
}

func (ts *dispatcherTestSuite) AddPeer(t *testing.T) *DispatcherNode {
	testNode := ts.network.MakeNode(t, p2p.TestNodeOptions{
		MaxConnected: utils.Some(1),
	})
	blockCh, err := p2p.OpenChannel(testNode.Router, GetLightBlockChannelDescriptor())
	if err != nil {
		panic(err)
	}
	n := &DispatcherNode{
		TestNode: testNode,
		blockCh:  blockCh,
	}
	require.NoError(t, ts.node.Connect(t.Context(), testNode))
	return n
}

// handleRequests is a helper function usually run in a separate go routine to
// imitate the expected responses of the reactor wired to the dispatcher
func (n *DispatcherNode) handleRequests(ctx context.Context, d *Dispatcher) error {
	for {
		req, err := n.blockCh.Recv(ctx)
		if err != nil {
			return nil
		}
		height := req.Message.Sum.(*pb.Message_LightBlockRequest).LightBlockRequest.Height
		resp := mockLBResp(ctx, n.NodeID, int64(height), time.Now())
		block, _ := resp.block.ToProto()
		if err := d.Respond(ctx, block, n.NodeID); err != nil {
			return fmt.Errorf("d.Respond(): %w", err)
		}
	}
}

func TestDispatcherBasic(t *testing.T) {
	const numPeers = 5
	ctx := t.Context()
	ts := dispatcherSetup(t)

	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		var peers []*DispatcherNode
		for range numPeers {
			n := ts.AddPeer(t)
			s.SpawnBg(func() error { return n.handleRequests(ctx, ts.dispatcher) })
			peers = append(peers, n)
		}

		// make a bunch of async requests and require that the correct responses are
		// given
		for i, peer := range peers {
			s.Spawn(func() error {
				height := int64(i + 1)
				lb, err := ts.dispatcher.LightBlock(ctx, height, peer.NodeID)
				if err != nil {
					return fmt.Errorf("LightBlock(%v): %w", height, err)
				}
				if lb.Height != height {
					return fmt.Errorf("expected height %v, got %v", height, lb.Height)
				}
				return nil
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// assert that all calls were responded to
	for calls := range ts.dispatcher.calls.Lock() {
		assert.Empty(t, calls)
	}
}

func TestDispatcherReturnsNoBlock(t *testing.T) {
	ts := dispatcherSetup(t)
	peer := ts.AddPeer(t)
	err := scope.Run(t.Context(), func(ctx context.Context, s scope.Scope) error {
		s.SpawnBg(func() error {
			for {
				_, err := peer.blockCh.Recv(ctx)
				if err != nil {
					return nil
				}
				if err := ts.dispatcher.Respond(ctx, nil, peer.NodeID); err != nil {
					return fmt.Errorf("d.Respond(): %w", err)
				}
			}
		})
		lb, err := ts.dispatcher.LightBlock(ctx, 1, peer.NodeID)
		if err != nil {
			return fmt.Errorf("LightBlock: %w", err)
		}
		if lb != nil {
			return fmt.Errorf("expected no light block, got %v", lb)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDispatcherTimeOutWaitingOnLightBlock(t *testing.T) {
	ts := dispatcherSetup(t)
	peer := factory.NodeID(t, "a")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := ts.dispatcher.LightBlock(ctx, 1, peer)
	if !errors.Is(err, ErrDisconnected) {
		t.Fatalf("expected context.Canceled error, got %v", err)
	}
}

func TestDispatcherProviders(t *testing.T) {
	ctx := t.Context()
	ts := dispatcherSetup(t)
	var peers []*DispatcherNode
	for range 5 {
		n := ts.AddPeer(t)
		go n.handleRequests(ctx, ts.dispatcher)
		peers = append(peers, n)
	}
	chainID := "test-chain"
	for _, peer := range peers {
		p := NewBlockProvider(peer.NodeID, chainID, ts.dispatcher)
		assert.Equal(t, string(peer.NodeID), p.String())
		lb, err := p.LightBlock(ctx, 10)
		assert.NoError(t, err)
		assert.NotNil(t, lb)
	}
}

func TestPeerListBasic(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ctx := t.Context()

	peerList := NewPeerList()
	assert.Zero(t, peerList.Len())
	numPeers := 10
	peerSet := createPeerSet(numPeers)

	for _, peer := range peerSet {
		peerList.Append(peer)
	}

	for idx, peer := range peerList.All() {
		assert.Equal(t, peer, peerSet[idx])
	}

	assert.Equal(t, numPeers, peerList.Len())

	half := numPeers / 2
	for i := range half {
		assert.Equal(t, peerSet[i], peerList.Pop(ctx))
	}
	assert.Equal(t, half, peerList.Len())

	// removing a peer that doesn't exist should not change the list
	peerList.Remove(types.NodeID("lp"))
	assert.Equal(t, half, peerList.Len())

	// removing a peer that exists should decrease the list size by one
	peerList.Remove(peerSet[half])
	assert.Equal(t, numPeers-half-1, peerList.Len())

	// popping the next peer should work as expected
	assert.Equal(t, peerSet[half+1], peerList.Pop(ctx))
	assert.Equal(t, numPeers-half-2, peerList.Len())

	// append the two peers back
	peerList.Append(peerSet[half])
	peerList.Append(peerSet[half+1])
	assert.Equal(t, half, peerList.Len())
}

func TestPeerListAllReturnsSnapshot(t *testing.T) {
	peerList := NewPeerList()
	peerSet := createPeerSet(2)
	for _, peer := range peerSet {
		peerList.Append(peer)
	}

	peers := peerList.All()
	require.Len(t, peers, 2)

	// Mutating the returned list must not mutate the internal peer list.
	peers[0] = peerSet[1]
	peers = peerList.All()
	require.Equal(t, peerSet[0], peers[0])
	require.Equal(t, peerSet[1], peers[1])
}

func TestPeerListBlocksWhenEmpty(t *testing.T) {
	t.Cleanup(leaktest.Check(t))
	peerList := NewPeerList()
	require.Zero(t, peerList.Len())
	doneCh := make(chan struct{})
	ctx := t.Context()
	go func() {
		peerList.Pop(ctx)
		close(doneCh)
	}()
	select {
	case <-doneCh:
		t.Error("empty peer list should not have returned result")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestEmptyPeerListReturnsWhenContextCanceled(t *testing.T) {
	t.Cleanup(leaktest.Check(t))
	peerList := NewPeerList()
	require.Zero(t, peerList.Len())
	doneCh := make(chan struct{})

	ctx := t.Context()

	wrapped, cancel := context.WithCancel(ctx)
	go func() {
		peerList.Pop(wrapped)
		close(doneCh)
	}()
	select {
	case <-doneCh:
		t.Error("empty peer list should not have returned result")
	case <-time.After(100 * time.Millisecond):
	}

	cancel()

	select {
	case <-doneCh:
	case <-time.After(100 * time.Millisecond):
		t.Error("peer list should have returned after context canceled")
	}
}

func TestPeerListConcurrent(t *testing.T) {
	t.Cleanup(leaktest.Check(t))
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	peerList := NewPeerList()
	const numPeers = 10
	completed := make(chan types.NodeID, numPeers)
	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		pop := func() error {
			peer := peerList.Pop(ctx)
			if peer == "" {
				return ctx.Err()
			}
			completed <- peer
			return nil
		}
		// Start consumers before populating the list, then a second wave.
		for range numPeers / 2 {
			s.Spawn(pop)
		}
		for _, peer := range createPeerSet(numPeers) {
			peerList.Append(peer)
		}
		for range numPeers / 2 {
			s.Spawn(pop)
		}
		return nil
	}) // cancellation terminates and joins every blocked Pop on failure too.
	assert.NoError(t, err, "peer-list consumers did not complete before the deadline")
	assert.Len(t, completed, numPeers)
	assert.Zero(t, peerList.Len())
	close(completed)
	var peers []types.NodeID
	for peer := range completed {
		peers = append(peers, peer)
	}
	assert.ElementsMatch(t, createPeerSet(numPeers), peers)
}

func TestPeerListRemove(t *testing.T) {
	peerList := NewPeerList()
	numPeers := 10

	peerSet := createPeerSet(numPeers)
	for _, peer := range peerSet {
		peerList.Append(peer)
	}

	for _, peer := range peerSet {
		peerList.Remove(peer)
		for _, p := range peerList.All() {
			require.NotEqual(t, p, peer)
		}
		numPeers--
		require.Equal(t, numPeers, peerList.Len())
	}
}

func createPeerSet(num int) []types.NodeID {
	peers := make([]types.NodeID, num)
	for i := range num {
		peers[i], _ = types.NewNodeID(strings.Repeat(fmt.Sprintf("%d", i), 2*types.NodeIDByteLength))
	}
	return peers
}

const testAppVersion = 9

func mockLBResp(ctx context.Context, peer types.NodeID, height int64, time time.Time) lightBlockResponse {
	vals, pv := factory.ValidatorSet(ctx, 3, 10)
	_, _, lb := mockLB(ctx, height, time, factory.MakeBlockID(), vals, pv)
	return lightBlockResponse{
		block: lb,
		peer:  peer,
	}
}

func mockLB(ctx context.Context, height int64, time time.Time, lastBlockID types.BlockID,
	currentVals *types.ValidatorSet, currentPrivVals []types.PrivValidator,
	consensusParams ...*types.ConsensusParams,
) (*types.ValidatorSet, []types.PrivValidator, *types.LightBlock) {
	header := factory.MakeHeader(&types.Header{
		Height:      height,
		LastBlockID: lastBlockID,
		Time:        time,
	})
	header.Version.App = testAppVersion

	nextVals, nextPrivVals := factory.ValidatorSet(ctx, 3, 10)
	header.ValidatorsHash = currentVals.Hash()
	header.NextValidatorsHash = nextVals.Hash()
	params := types.DefaultConsensusParams()
	if len(consensusParams) > 0 {
		params = consensusParams[0]
	}
	header.ConsensusHash = params.HashConsensusParams()
	lastBlockID = factory.MakeBlockIDWithHash(header.Hash())
	voteSet := types.NewVoteSet(factory.DefaultTestChainID, height, 0, tmproto.PrecommitType, currentVals)
	commit, err := factory.MakeCommit(ctx, lastBlockID, height, 0, voteSet, currentPrivVals, time)
	if err != nil {
		panic(fmt.Errorf("factory.MakeCommit(): %w", err))
	}
	return nextVals, nextPrivVals, &types.LightBlock{
		SignedHeader: &types.SignedHeader{
			Header: header,
			Commit: commit,
		},
		ValidatorSet: currentVals,
	}
}
