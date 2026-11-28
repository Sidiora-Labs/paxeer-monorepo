package consensus

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	cstypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
)

func TestRoundStateCopyWithoutAVoteSetInTheQueryRoutine(t *testing.T) {
	r := &Reactor{state: &State{config: config.DefaultConsensusConfig()}}
	for _, height := range []int64{0, 5} {
		t.Run(fmt.Sprintf("height=%d", height), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()

			r.roundState.Store(&cstypes.RoundState{HRS: cstypes.HRS{Height: height}})
			ps := NewPeerState("")
			if height > 0 {
				ps.ApplyNewRoundStepMessage(&NewRoundStepMessage{
					HRS: cstypes.HRS{
						Height: height,
						Step:   cstypes.RoundStepNewHeight,
					},
					LastCommitRound: -1,
				})
			}
			require.Equal(t, height, ps.GetHeight())
			require.NotPanics(t, func() {
				err := r.queryMaj23Routine(ctx, ps)
				require.ErrorIs(t, err, ctx.Err())
				require.ErrorIs(t, err, context.DeadlineExceeded)
			})
		})
	}
}

func newRoundStateCopyReactor(ctx context.Context, t *testing.T) *Reactor {
	t.Helper()
	cfg := configSetup(t)
	states, cleanup := makeConsensusState(ctx, t, cfg, 1, "round_state_copy",
		func() TimeoutTicker { return NewTimeoutTicker() })
	t.Cleanup(cleanup)
	state := states[0].State
	network := p2p.MakeTestNetwork(t, p2p.TestNetworkOptions{NumNodes: 1})
	r, err := NewReactor(state, network.Nodes()[0].Router, state.eventBus, true, NopMetrics(), config.DefaultConfig())
	require.NoError(t, err)
	if state.state.LastBlockHeight == 0 {
		require.NoError(t, state.blockExec.Store().Save(state.state))
	}
	return r
}

func TestRoundStateCopyIsStoredAtTheStart(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	r := newRoundStateCopyReactor(ctx, t)
	r.roundState.Store(&cstypes.RoundState{})

	require.NoError(t, r.Start(ctx))
	t.Cleanup(r.Wait)
	rs := r.roundState.Load()
	require.NotNil(t, rs)
	require.NotNil(t, rs.Votes)
	require.Equal(t, r.state.GetRoundState().Height, rs.Height)
	msg := makeRoundStepMessage(rs)
	require.NoError(t, msg.ValidateBasic())
	require.NoError(t, msg.ValidateHeight(r.state.GetState().InitialHeight))
}

func TestRoundStateCopyIsStoredBeforeTheReadySignal(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	r := newRoundStateCopyReactor(ctx, t)
	height := r.state.GetRoundState().Height
	r.roundState.Store(&cstypes.RoundState{})
	require.True(t, r.WaitSync())

	r.SwitchToConsensus(r.state.GetState(), false)

	require.False(t, r.WaitSync())
	rs := r.roundState.Load()
	require.NotNil(t, rs)
	require.NotNil(t, rs.Votes)
	require.GreaterOrEqual(t, rs.Height, height)
}
