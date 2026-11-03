package app

import tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"

var _ tmtypes.LeaderSource = (*App)(nil)

// LeaderSchedule reads the latest committed schedule on every call. The requested
// height does not select an uncommitted proposal or executor branch.
func (app *App) LeaderSchedule(_ int64) (int64, [][32]byte, error) {
	ctx, err := app.CreateQueryContext(0, false)
	if err != nil {
		return 0, nil, err
	}
	return app.SequencerKeeper.LeaderSchedule(ctx)
}
