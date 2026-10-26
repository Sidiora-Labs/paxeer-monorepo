package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestConsensusParamsUnderAutobahnReturnsGenesisParams(t *testing.T) {
	ctx := t.Context()
	rng := utils.TestRng()
	kv := kvstore.NewApplication()
	t.Cleanup(func() { require.NoError(t, kv.Close()) })
	app := proxy.New(kv, proxy.NopMetrics())
	const head int64 = 3
	for height := int64(1); height <= head; height++ {
		_, err := app.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{Header: &tmproto.Header{Height: height}})
		require.NoError(t, err)
		_, err = app.Commit(ctx)
		require.NoError(t, err)
	}

	stateStore := sm.NewStore(dbm.NewMemDB())
	router, _ := newAutobahnTestRouter(t, rng, "autobahn-consensus-params-test", app, stateStore, 1, 12345)
	params := types.DefaultConsensusParams()
	params.Block.MaxGas = 12345
	params.Synchrony = types.SynchronyParams{}
	params.Timeout = types.TimeoutParams{}
	env := &Environment{
		App:        app,
		StateStore: stateStore,
		BlockStore: store.NewBlockStore(dbm.NewMemDB()),
		Router:     router,
		GenDoc:     &types.GenesisDoc{ConsensusParams: params},
	}
	info, err := env.ABCIInfo(ctx)
	require.NoError(t, err)
	require.Equal(t, head, info.Response.LastBlockHeight)
	require.Zero(t, env.BlockStore.Height())
	_, err = stateStore.LoadConsensusParams(head)
	require.Error(t, err, "Autobahn does not populate classic consensus parameters")

	want := *params
	want.Synchrony = types.DefaultSynchronyParams()
	want.Timeout = types.DefaultTimeoutParams()
	first, latest := coretypes.Int64(1), coretypes.Int64(head)
	for _, height := range []*coretypes.Int64{nil, &first, &latest} {
		result, err := env.ConsensusParams(ctx, &coretypes.RequestConsensusParams{Height: height})
		require.NoError(t, err)
		wantHeight := info.Response.LastBlockHeight
		if height != nil {
			wantHeight = int64(*height)
		}
		require.Equal(t, wantHeight, result.BlockHeight)
		require.Equal(t, want, result.ConsensusParams)
	}
	require.Equal(t, types.SynchronyParams{}, params.Synchrony)
	require.Equal(t, types.TimeoutParams{}, params.Timeout)

	params.Synchrony.Precision = 2 * time.Second
	params.Timeout.Propose = 3 * time.Second
	params.Timeout.BypassCommitTimeout = true
	want.Synchrony.Precision = params.Synchrony.Precision
	want.Timeout.Propose = params.Timeout.Propose
	want.Timeout.BypassCommitTimeout = true
	result, err := env.ConsensusParams(ctx, &coretypes.RequestConsensusParams{})
	require.NoError(t, err)
	require.Equal(t, want, result.ConsensusParams)

	for _, tc := range []struct {
		name   string
		height coretypes.Int64
		err    error
	}{
		{name: "zero", height: 0, err: coretypes.ErrZeroOrNegativeHeight},
		{name: "negative", height: -1, err: coretypes.ErrZeroOrNegativeHeight},
		{name: "future", height: coretypes.Int64(head + 1), err: coretypes.ErrHeightExceedsChainHead},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := env.ConsensusParams(t.Context(), &coretypes.RequestConsensusParams{Height: &tc.height})
			require.ErrorIs(t, err, tc.err)
			require.Nil(t, result)
		})
	}
}
