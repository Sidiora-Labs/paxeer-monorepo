package core

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
)

func TestCommitUnderAutobahnReturnsErrCommitNotAvailable(t *testing.T) {
	rng := utils.TestRng()
	app := proxy.New(kvstore.NewApplication(), proxy.NopMetrics())
	stateStore := sm.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, stateStore.Close()) })
	router, _ := newAutobahnTestRouter(t, rng, "autobahn-commit-test", app, stateStore, 1, 1_000)
	env := &Environment{
		App:        app,
		StateStore: stateStore,
		BlockStore: store.NewBlockStore(dbm.NewMemDB()),
		Router:     router,
	}

	for _, tc := range []struct {
		name   string
		height coretypes.Int64
		latest bool
	}{
		{name: "latest", latest: true},
		{name: "zero", height: 0},
		{name: "negative", height: -1},
		{name: "minimum", height: math.MinInt64},
		{name: "first", height: 1},
		{name: "above_head", height: 42},
		{name: "maximum", height: math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := &coretypes.RequestBlockInfo{}
			if !tc.latest {
				req.Height = &tc.height
			}
			result, err := env.Commit(t.Context(), req)
			require.ErrorIs(t, err, coretypes.ErrCommitNotAvailable)
			require.Nil(t, result)
		})
	}
}
