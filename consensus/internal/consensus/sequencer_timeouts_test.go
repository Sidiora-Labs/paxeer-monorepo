package consensus

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	cstypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

const sequencerTimeoutActivation = int64(100)

func sequencerTimeoutState(t *testing.T) *State {
	t.Helper()
	cs := &State{config: config.DefaultConsensusConfig()}
	cs.config.Sequencer = config.SequencerConfig{
		Enable:              true,
		TimeoutPropose:      1700 * time.Millisecond,
		TimeoutProposeDelta: 230 * time.Millisecond,
		TimeoutVote:         41 * time.Millisecond,
		TimeoutCommit:       59 * time.Millisecond,
	}
	cs.state.ConsensusParams.Timeout = types.TimeoutParams{
		Propose:      7 * time.Second,
		ProposeDelta: 1100 * time.Millisecond,
		Vote:         3 * time.Second,
		VoteDelta:    700 * time.Millisecond,
		Commit:       2 * time.Second,
	}
	key := ed25519.TestSecretKey([]byte("sequencer-timeouts-leader")).Public()
	pin, err := cstypes.NewLeaderPin(sequencerTimeoutActivation, []crypto.PubKey{key})
	require.NoError(t, err)
	cs.SetLeaderPin(pin)
	cs.roundState.SetHeight(sequencerTimeoutActivation)
	return cs
}

func sequencerTimeoutOverrides(cfg *config.ConsensusConfig, base, delta bool) {
	if base {
		cfg.UnsafeProposeTimeoutOverride = 11 * time.Second
		cfg.UnsafeVoteTimeoutOverride = 5 * time.Second
		cfg.UnsafeCommitTimeoutOverride = 4 * time.Second
	}
	if delta {
		cfg.UnsafeProposeTimeoutDeltaOverride = 1300 * time.Millisecond
		cfg.UnsafeVoteTimeoutDeltaOverride = 900 * time.Millisecond
	}
}

func TestSequencerProposeTimeoutAtPinnedHeight(t *testing.T) {
	for _, height := range []int64{sequencerTimeoutActivation, sequencerTimeoutActivation + 1, sequencerTimeoutActivation + 100} {
		for _, overrides := range []bool{false, true} {
			t.Run(fmt.Sprintf("height=%d/overrides=%t", height, overrides), func(t *testing.T) {
				cs := sequencerTimeoutState(t)
				cs.roundState.SetHeight(height)
				sequencerTimeoutOverrides(cs.config, overrides, overrides)
				require.True(t, cs.sequencerTimeouts())
				for _, round := range []int32{0, 1, 2, 7} {
					require.Equal(t, cs.config.Sequencer.ProposeTimeout(round), cs.proposeTimeout(round), "round=%d", round)
				}
			})
		}
	}
}

func TestSequencerVoteAndCommitTimeoutsAtPinnedHeight(t *testing.T) {
	for _, height := range []int64{sequencerTimeoutActivation, sequencerTimeoutActivation + 1, sequencerTimeoutActivation + 100} {
		for _, overrides := range []bool{false, true} {
			t.Run(fmt.Sprintf("height=%d/overrides=%t", height, overrides), func(t *testing.T) {
				cs := sequencerTimeoutState(t)
				cs.roundState.SetHeight(height)
				sequencerTimeoutOverrides(cs.config, overrides, overrides)
				require.True(t, cs.sequencerTimeouts())
				for _, round := range []int32{0, 1, 2, 7} {
					require.Equal(t, cs.config.Sequencer.TimeoutVote, cs.voteTimeout(round), "round=%d", round)
				}
				for _, now := range []time.Time{{}, time.Unix(1700000000, 123456789).UTC()} {
					require.Equal(t, now.Add(cs.config.Sequencer.TimeoutCommit), cs.commitTime(now))
				}
			})
		}
	}
}

func TestSequencerTimeoutsUnusedBelowPin(t *testing.T) {
	for _, tc := range []struct {
		name   string
		height int64
		enable bool
		noPin  bool
	}{
		{name: "below activation", height: sequencerTimeoutActivation - 1, enable: true},
		{name: "disabled below activation", height: sequencerTimeoutActivation - 1},
		{name: "disabled at activation", height: sequencerTimeoutActivation},
		{name: "disabled after activation", height: sequencerTimeoutActivation + 1},
		{name: "no pin", height: sequencerTimeoutActivation, enable: true, noPin: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, timeouts := range []struct {
				name     string
				defaults bool
				base     bool
				delta    bool
			}{
				{name: "consensus params"},
				{name: "zero consensus params", defaults: true},
				{name: "base overrides", base: true},
				{name: "delta overrides", delta: true},
				{name: "all overrides", base: true, delta: true},
				{name: "overrides with zero params", defaults: true, base: true, delta: true},
			} {
				t.Run(timeouts.name, func(t *testing.T) {
					cs := sequencerTimeoutState(t)
					cs.roundState.SetHeight(tc.height)
					cs.config.Sequencer.Enable = tc.enable
					if tc.noPin {
						cs.SetLeaderPin(cstypes.LeaderPin{})
					}
					if timeouts.defaults {
						cs.state.ConsensusParams.Timeout = types.TimeoutParams{}
					}
					sequencerTimeoutOverrides(cs.config, timeouts.base, timeouts.delta)
					require.False(t, cs.sequencerTimeouts())

					params := cs.state.ConsensusParams.Timeout.TimeoutParamsOrDefaults()
					propose, proposeDelta := params.Propose, params.ProposeDelta
					vote, voteDelta := params.Vote, params.VoteDelta
					commit := cs.state.ConsensusParams.Timeout.Commit
					if timeouts.base {
						propose = 11 * time.Second
						vote = 5 * time.Second
						commit = 4 * time.Second
					}
					if timeouts.delta {
						proposeDelta = 1300 * time.Millisecond
						voteDelta = 900 * time.Millisecond
					}
					for _, round := range []int32{0, 1, 2, 7} {
						require.Equal(t, propose+proposeDelta*time.Duration(round), cs.proposeTimeout(round), "round=%d", round)
						require.Equal(t, vote+voteDelta*time.Duration(round), cs.voteTimeout(round), "round=%d", round)
					}
					now := time.Unix(1700000000, 123456789).UTC()
					require.Equal(t, now.Add(commit), cs.commitTime(now))
				})
			}
		})
	}
}
