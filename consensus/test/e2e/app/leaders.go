package app

import (
	"slices"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

var _ types.LeaderSource = (*Application)(nil)

func (app *Application) LeaderSchedule(_ int64) (int64, [][32]byte, error) {
	return app.leaderPinHeight, slices.Clone(app.leaderPin), nil
}
