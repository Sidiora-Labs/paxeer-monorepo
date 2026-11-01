package app

import (
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (app *App) checkSequencerOrder(ctx sdk.Context, batch seqtypes.MsgSequenceBatch) error {
	branch, _ := ctx.CacheContext()
	if err := app.SequencerKeeper.DeliverSequenceBatch(branch, batch); err != nil {
		return seqtypes.ErrInvalidSequenceBatch.Wrap(err.Error())
	}
	return nil
}

func (app *App) CheckSequencerOrder(ctx sdk.Context, batch seqtypes.MsgSequenceBatch) error {
	return app.checkSequencerOrder(ctx, batch)
}
