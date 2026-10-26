package app

import tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"

var _ tmtypes.LeaderSource = (*App)(nil)

func (app *App) LeaderSchedule(height int64) (int64, [][32]byte, error) {
	ctx, err := app.CreateQueryContext(0, false)
	if err != nil {
		return 0, nil, err
	}
	return app.SequencerKeeper.LeaderSchedule(ctx)
}
