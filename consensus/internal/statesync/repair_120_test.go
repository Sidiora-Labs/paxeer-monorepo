package statesync

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	dbm "github.com/tendermint/tm-db"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/test/factory"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/require"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/provider"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	pb "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/statesync"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

type repair120Network struct {
	client    *Reactor
	witness   *Reactor
	clientID  types.NodeID
	witnessID types.NodeID
	provider  StateProvider
	blocks    map[int64]*types.LightBlock
	states    map[int64]sm.State
}

func newRepair120Network(t *testing.T, ctx context.Context, s scope.Scope) *repair120Network {
	t.Helper()
	keyDir := t.TempDir()
	pv, err := privval.GenFilePV(filepath.Join(keyDir, "key.json"), filepath.Join(keyDir, "state.json"), types.ABCIPubKeyTypeEd25519)
	require.NoError(t, err)
	pubKey, err := pv.GetPubKey(ctx)
	require.NoError(t, err)
	vals := types.NewValidatorSet([]*types.Validator{types.NewValidator(pubKey, 10)})
	blocks := make(map[int64]*types.LightBlock)
	states := make(map[int64]sm.State)
	lastID := factory.MakeBlockID()
	start := time.Now().Add(-time.Minute)
	for height := int64(1); height <= 3; height++ {
		params := types.DefaultConsensusParams()
		if height == 3 {
			params.Block.MaxGas = 123456
		}
		require.NoError(t, params.ValidateConsensusParams())
		header := factory.MakeHeader(&types.Header{
			Height:             height,
			Time:               start.Add(time.Duration(height) * time.Second),
			LastBlockID:        lastID,
			ValidatorsHash:     vals.Hash(),
			NextValidatorsHash: vals.Hash(),
			ConsensusHash:      params.HashConsensusParams(),
		})
		blockID := factory.MakeBlockIDWithHash(header.Hash())
		votes := types.NewVoteSet(header.ChainID, height, 0, tmproto.PrecommitType, vals)
		commit, err := factory.MakeCommit(ctx, blockID, height, 0, votes, []types.PrivValidator{pv}, header.Time)
		require.NoError(t, err)
		blocks[height] = &types.LightBlock{
			SignedHeader: &types.SignedHeader{Header: header, Commit: commit},
			ValidatorSet: vals,
		}
		require.NoError(t, blocks[height].ValidateBasic(header.ChainID))
		states[height] = sm.State{
			Version:                          sm.InitStateVersion,
			ChainID:                          header.ChainID,
			InitialHeight:                    1,
			LastBlockHeight:                  height - 1,
			LastBlockID:                      lastID,
			LastBlockTime:                    header.Time.Add(-time.Second),
			LastValidators:                   vals,
			Validators:                       vals,
			NextValidators:                   vals,
			LastHeightValidatorsChanged:      height,
			ConsensusParams:                  *params,
			LastHeightConsensusParamsChanged: height,
		}
		lastID = blockID
	}
	require.NotEqual(t, blocks[2].ConsensusHash, blocks[3].ConsensusHash)

	network := p2p.MakeTestNetwork(t, p2p.TestNetworkOptions{NumNodes: 3})
	nodes := network.Nodes()
	reactors := make([]*Reactor, len(nodes))
	for i, node := range nodes {
		stateDB, blockDB := dbm.NewMemDB(), dbm.NewMemDB()
		t.Cleanup(func() {
			require.NoError(t, stateDB.Close())
			require.NoError(t, blockDB.Close())
		})
		stateStore := sm.NewStore(stateDB)
		blockStore := store.NewBlockStore(blockDB)
		for height := int64(1); height <= 3; height++ {
			require.NoError(t, stateStore.Bootstrap(states[height]))
			require.NoError(t, blockStore.SaveSignedHeader(blocks[height].SignedHeader, blocks[height].Commit.BlockID))
		}
		r, err := NewReactor(factory.DefaultTestChainID, 1, *config.DefaultStateSyncConfig(),
			proxy.New(abci.BaseApplication{}, proxy.NopMetrics()), node.Router, stateStore, blockStore,
			t.TempDir(), NopMetrics(), nil, nil, false, func() {}, config.DefaultSelfRemediationConfig())
		require.NoError(t, err)
		r.dispatcher = NewDispatcher(r.lightBlockChannel)
		reactors[i] = r
		s.SpawnBg(func() error {
			r.processLightBlockCh(ctx)
			return nil
		})
	}
	network.Start(t)
	client := reactors[0]
	sp, err := NewP2PStateProvider(ctx, factory.DefaultTestChainID, 1, 5*time.Second,
		[]provider.Provider{
			NewBlockProvider(nodes[1].NodeID, factory.DefaultTestChainID, client.dispatcher),
			NewBlockProvider(nodes[2].NodeID, factory.DefaultTestChainID, client.dispatcher),
		}, light.TrustOptions{Period: time.Hour, Height: 1, Hash: blocks[1].Hash()}, client.paramsChannel, time.Minute)
	require.NoError(t, err)
	client.stateProvider = sp
	s.SpawnBg(func() error {
		client.processParamsCh(ctx)
		return nil
	})
	return &repair120Network{
		client: client, witness: reactors[2], clientID: nodes[0].NodeID, witnessID: nodes[2].NodeID,
		provider: sp, blocks: blocks, states: states,
	}
}

func (n *repair120Network) serveParams(ctx context.Context, s scope.Scope) {
	s.SpawnBg(func() error {
		request, err := n.witness.paramsChannel.Recv(ctx)
		if err != nil {
			return err
		}
		if req := request.Message.GetParamsRequest(); req == nil || req.Height != 2 {
			return fmt.Errorf("expected parameter request for height 2, got %v", req)
		}
		return n.witness.handleParamsMessage(ctx, request)
	})
}

func TestRepair_120_ParamsHeightRestoration(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		n := newRepair120Network(t, ctx, s)
		n.serveParams(ctx, s)
		restored, err := n.provider.State(ctx, 1)
		if err != nil {
			return err
		}
		require.Equal(t, n.states[2].ConsensusParams, restored.ConsensusParams)
		require.Equal(t, int64(2), restored.LastHeightConsensusParamsChanged)
		require.Equal(t, int64(1), restored.LastBlockHeight)
		require.Equal(t, n.blocks[1].Commit.BlockID, restored.LastBlockID)
		require.Equal(t, n.blocks[2].AppHash, restored.AppHash)
		require.Equal(t, n.blocks[2].ConsensusHash, restored.ConsensusParams.HashConsensusParams())
		require.NotEqual(t, n.blocks[3].ConsensusHash, restored.ConsensusParams.HashConsensusParams())
		db := dbm.NewMemDB()
		defer db.Close()
		stateStore := sm.NewStore(db)
		require.NoError(t, stateStore.Bootstrap(restored))
		loaded, err := stateStore.Load()
		if err != nil {
			return err
		}
		equal, err := restored.Equals(loaded)
		require.NoError(t, err)
		require.True(t, equal)
		return nil
	})
	require.NoError(t, err)
}

func TestRepair_120_ParamsWrongHeightRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		n := newRepair120Network(t, ctx, s)
		require.Equal(t, n.blocks[1].ConsensusHash, n.blocks[2].ConsensusHash)
		s.SpawnBg(func() error {
			pending, err := n.witness.paramsChannel.Recv(ctx)
			if err != nil {
				return err
			}
			if req := pending.Message.GetParamsRequest(); req == nil || req.Height != 2 {
				return fmt.Errorf("expected pending request for height 2, got %v", req)
			}
			n.client.paramsChannel.Send(wrap(&pb.ParamsRequest{Height: 1}), n.witnessID)
			other, err := n.witness.paramsChannel.Recv(ctx)
			if err != nil {
				return err
			}
			if req := other.Message.GetParamsRequest(); req == nil || req.Height != 1 || other.From != n.clientID {
				return fmt.Errorf("expected request from the restoring client for height 1, got %v", req)
			}
			return n.witness.handleParamsMessage(ctx, other)
		})
		restored, err := n.provider.State(ctx, 1)
		require.Error(t, err)
		require.NoError(t, ctx.Err())
		require.True(t, restored.IsEmpty())
		return nil
	})
	require.NoError(t, err)
}

func TestRepair_120_ParamsWrongHashRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		n := newRepair120Network(t, ctx, s)
		wrong := n.states[2]
		wrong.ConsensusParams = n.states[3].ConsensusParams
		require.NoError(t, n.witness.stateStore.Bootstrap(wrong))
		n.serveParams(ctx, s)
		restored, err := n.provider.State(ctx, 1)
		require.Error(t, err)
		require.NoError(t, ctx.Err())
		require.True(t, restored.IsEmpty())
		return nil
	})
	require.NoError(t, err)
}
