package data

import (
	"context"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/require"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
)

func TestStateFirstBlockAdvancesOnPrune(t *testing.T) {
	ctx := t.Context()
	rng := utils.TestRng()
	committee, keys := types.GenCommittee(rng, 3)
	require.NoError(t, scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		wal, err := NewDataWAL(utils.None[string](), committee)
		if err != nil {
			return err
		}
		defer func() { require.NoError(t, wal.Close()) }()
		state, err := NewState(&Config{Committee: committee}, wal)
		if err != nil {
			return err
		}
		s.SpawnBgNamed("state.Run()", func() error {
			return utils.IgnoreCancel(state.Run(ctx))
		})

		require.Equal(t, committee.FirstBlock(), state.FirstBlock())
		require.NoError(t, state.PruneBefore(committee.FirstBlock()))
		require.Equal(t, committee.FirstBlock(), state.FirstBlock())

		prev := utils.None[*types.CommitQC]()
		for range 3 {
			qc, blocks := TestCommitQC(rng, committee, keys, prev)
			if err := state.PushQC(ctx, qc, blocks); err != nil {
				return err
			}
			prev = utils.Some(qc.QC())
			gr := qc.QC().GlobalRange(committee)
			first := state.FirstBlock()
			require.NoError(t, state.PruneBefore(gr.Next))
			require.Equal(t, first, state.FirstBlock())

			for n := gr.First; n < gr.Next; n++ {
				if err := state.PushAppHash(ctx, n, types.GenAppHash(rng)); err != nil {
					return err
				}
			}
			require.Equal(t, first, state.FirstBlock())
			require.NoError(t, state.PruneBefore(gr.Next))
			require.Equal(t, gr.Next-1, state.FirstBlock())
			retained, err := state.TryBlock(gr.Next - 1)
			require.NoError(t, err)
			require.Equal(t, blocks[len(blocks)-1], retained)
			_, err = state.TryBlock(gr.Next - 2)
			require.ErrorIs(t, err, ErrPruned)

			require.NoError(t, state.PruneBefore(gr.First))
			require.Equal(t, gr.Next-1, state.FirstBlock())
		}
		return nil
	}))
}
