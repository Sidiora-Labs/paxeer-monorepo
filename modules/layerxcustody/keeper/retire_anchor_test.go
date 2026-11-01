package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestCheckpointRegistrationIsRefusedFromTheMergePlanHeight(t *testing.T) {
	e := newEnvWithoutCheckpoint(t, 0)
	testkeeper.EVMTestApp.UpgradeKeeper.SetDone(e.ctx.WithBlockHeight(20), types.MergePlanUpgrade)
	stateRoot := vectorArray(t, e.withdrawal, "header_state_root")
	receiptRoot := vectorArray(t, e.withdrawal, "header_receipt_root")

	for _, tc := range []struct {
		name    string
		height  int64
		batch   uint64
		retired bool
	}{
		{name: "below plan height", height: 19, batch: 1, retired: false},
		{name: "at plan height", height: 20, batch: 2, retired: true},
		{name: "above plan height", height: 21, batch: 3, retired: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := e.ctx.WithBlockHeight(tc.height)
			_, found := e.k.GetCheckpoint(ctx, tc.batch)
			require.False(t, found)
			eventCount := len(ctx.EventManager().Events())

			err := e.k.RegisterCheckpoint(ctx, tc.batch, stateRoot, receiptRoot)
			if tc.retired {
				require.ErrorIs(t, err, types.ErrModuleRetired)
				require.Contains(t, err.Error(), "v7.0 from height")
				_, found = e.k.GetCheckpoint(ctx, tc.batch)
				require.False(t, found)
				require.Len(t, ctx.EventManager().Events(), eventCount)
				return
			}

			require.NoError(t, err)
			checkpoint, found := e.k.GetCheckpoint(ctx, tc.batch)
			require.True(t, found)
			require.Equal(t, tc.batch, checkpoint.BatchNumber)
			require.Equal(t, types.Hash32(stateRoot), checkpoint.StateRoot)
			require.Equal(t, types.Hash32(receiptRoot), checkpoint.ReceiptRoot)
		})
	}
}
