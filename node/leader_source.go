package app

import (
	"slices"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

var _ tmtypes.LeaderSource = (*App)(nil)

type committedLeaderSchedule struct {
	activationHeight int64
	leaders          [][32]byte
	err              error
}

// publishLeaderSchedule runs after loading or committing the application root,
// before another block can change its working state.
func (app *App) publishLeaderSchedule() {
	ctx := app.NewUncachedContext(true, tmproto.Header{Height: app.LastBlockHeight()})
	height, leaders, err := app.SequencerKeeper.LeaderSchedule(ctx)
	app.committedLeaders.Store(&committedLeaderSchedule{height, leaders, err})
}

// LeaderSchedule reads the latest committed schedule on every call. The requested
// height does not select an uncommitted proposal or executor branch.
func (app *App) LeaderSchedule(_ int64) (int64, [][32]byte, error) {
	if schedule := app.committedLeaders.Load(); schedule != nil {
		return schedule.activationHeight, slices.Clone(schedule.leaders), schedule.err
	}
	// Before the first commit, retain the query context's empty-state behavior.
	ctx, err := app.CreateQueryContext(0, false)
	if err != nil {
		return 0, nil, err
	}
	return app.SequencerKeeper.LeaderSchedule(ctx)
}
