package app

import (
	"bytes"

	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (app *App) checkSequencerProposer(ctx sdk.Context, proposer []byte) error {
	leaders, err := app.SequencerKeeper.Leaders(ctx)
	if err != nil {
		return err
	}
	for _, leader := range leaders {
		if bytes.Equal(proposer, leader[:]) {
			return nil
		}
	}
	return seqtypes.ErrInvalidSequenceBatch.Wrapf("proposer %x is outside the pinned leader list", proposer)
}

func (w *TestWrapper) CheckSequencerProposer(ctx sdk.Context, proposer []byte) error {
	return w.App.checkSequencerProposer(ctx, proposer)
}
