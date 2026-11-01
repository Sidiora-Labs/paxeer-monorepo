package consensus

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	cstypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/consensus/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

type persistedLeaderSchedule struct {
	ActivationHeight int64
	Leaders          [][32]byte
}

type leaderScheduleDirectory string

func (dir leaderScheduleDirectory) path(height int64) string {
	return filepath.Join(string(dir), strconv.FormatInt(height, 10)+".json")
}

func (dir leaderScheduleDirectory) LeaderSchedule(height int64) (int64, [][32]byte, error) {
	data, err := os.ReadFile(dir.path(height))
	if err != nil {
		return 0, nil, err
	}
	var schedule persistedLeaderSchedule
	if err := json.Unmarshal(data, &schedule); err != nil {
		return 0, nil, err
	}
	return schedule.ActivationHeight, schedule.Leaders, nil
}

func writeLeaderSchedule(t *testing.T, dir leaderScheduleDirectory, height, activationHeight int64, keys ...crypto.PubKey) cstypes.LeaderPin {
	t.Helper()
	schedule := persistedLeaderSchedule{ActivationHeight: activationHeight}
	for _, key := range keys {
		schedule.Leaders = append(schedule.Leaders, [32]byte(key.Bytes()))
	}
	data, err := json.Marshal(schedule)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dir.path(height), data, 0600))
	pin, err := cstypes.LeaderPinFromSchedule(activationHeight, schedule.Leaders)
	require.NoError(t, err)
	return pin
}

func leaderSourceProposal(ctx context.Context, t *testing.T, cs *testState) types.BlockID {
	t.Helper()
	height := cs.roundState.Height()
	cs.enterNewRound(ctx, height, 0, "")
	require.NoError(t, cs.receiveRoutine(ctx, cs.nSteps+1))
	rs := cs.GetRoundState()
	require.Equal(t, height, rs.Height)
	require.Equal(t, cstypes.RoundStepPrevote, rs.Step)
	require.NotNil(t, rs.Proposal)
	require.NotNil(t, rs.ProposalBlock)
	require.Equal(t, cs.address(ctx), rs.Proposal.ProposerAddress)
	return types.BlockID{Hash: rs.ProposalBlock.Hash(), PartSetHeader: rs.ProposalBlockParts.Header()}
}

func TestSetLeaderSourceInstallsPinForCurrentHeight(t *testing.T) {
	for _, tc := range []struct {
		name             string
		activationOffset int64
		disabled         bool
	}{
		{name: "active"},
		{name: "future activation", activationOffset: 5},
		{name: "disabled", disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			cs, vss := makeState(ctx, t, makeStateArgs{})
			remote, local := sequencerSourcePin(ctx, t, cs, vss)
			height := cs.roundState.Height()
			dir := leaderScheduleDirectory(t.TempDir())
			activation := height + tc.activationOffset
			if tc.disabled {
				activation = 0
			}
			want := writeLeaderSchedule(t, dir, height, activation, local, remote)
			cs.SetLeaderSource(dir)
			rs := cs.GetRoundState()
			require.Equal(t, want, rs.LeaderPin)
			require.Equal(t, height, rs.Height)
			for round := range int32(sequencerSourceRounds) {
				rs.Round = round
				leader, pinned := want.Leader(height, round)
				if !pinned {
					weighted := *rs
					weighted.LeaderPin = cstypes.LeaderPin{}
					leader = weighted.Leader()
				}
				require.Equal(t, leader, rs.Leader())
			}
			if activation == height {
				require.True(t, cs.isProposer(local))
				require.False(t, cs.isProposer(remote))
			}
		})
	}
}

func TestLeaderSourceRefreshedOnNewHeight(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), ensureTimeout)
	defer cancel()
	cs, vss := makeState(ctx, t, makeStateArgs{})
	remote, local := sequencerSourcePin(ctx, t, cs, vss)
	height := cs.roundState.Height()
	dir := leaderScheduleDirectory(t.TempDir())
	initial := writeLeaderSchedule(t, dir, height, height, local, remote)
	cs.SetLeaderSource(dir)
	blockID := leaderSourceProposal(ctx, t, cs)

	next := writeLeaderSchedule(t, dir, height+1, height+1, remote, local)
	require.Equal(t, initial, cs.GetRoundState().LeaderPin)
	cs.signAddVotes(ctx, t, tmproto.PrecommitType, cs.GetState().ChainID, blockID, vss[1:]...)
	require.NoError(t, cs.receiveRoutine(ctx, cs.nSteps+1))

	require.NoError(t, ctx.Err())
	rs := cs.GetRoundState()
	require.Equal(t, height+1, rs.Height)
	require.Equal(t, cstypes.RoundStepNewHeight, rs.Step)
	require.Equal(t, height, cs.GetState().LastBlockHeight)
	require.Equal(t, height, cs.blockStore.Height())
	require.Equal(t, next, rs.LeaderPin)
	require.Equal(t, remote, rs.Leader())
	require.True(t, cs.isProposer(remote))
	require.False(t, cs.isProposer(local))
	require.Nil(t, rs.Proposal)
	require.False(t, rs.StartTime.IsZero())
}

func TestLeaderSourceErrorStopsState(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), ensureTimeout)
	defer cancel()
	cs, vss := makeState(ctx, t, makeStateArgs{})
	remote, local := sequencerSourcePin(ctx, t, cs, vss)
	height := cs.roundState.Height()
	dir := leaderScheduleDirectory(t.TempDir())
	initial := writeLeaderSchedule(t, dir, height, height, local, remote)
	cs.SetLeaderSource(dir)
	blockID := leaderSourceProposal(ctx, t, cs)
	startTime := cs.GetRoundState().StartTime
	cs.signAddVotes(ctx, t, tmproto.PrecommitType, cs.GetState().ChainID, blockID, vss[1:]...)

	var failure any
	func() {
		defer func() { failure = recover() }()
		require.NoError(t, cs.receiveRoutine(ctx, 0))
	}()
	err, ok := failure.(error)
	require.True(t, ok, "receiveRoutine must propagate the schedule error as a panic")
	require.ErrorIs(t, err, os.ErrNotExist)
	var pathErr *os.PathError
	require.ErrorAs(t, err, &pathErr)
	require.Equal(t, dir.path(height+1), pathErr.Path)
	require.NoError(t, ctx.Err())
	committed, err := cs.stateStore.Load()
	require.NoError(t, err)
	require.Equal(t, height, committed.LastBlockHeight)
	require.Equal(t, height, cs.blockStore.Height())
	rs := cs.GetRoundState()
	require.Equal(t, height, rs.Height)
	require.Equal(t, cstypes.RoundStepCommit, rs.Step)
	require.Equal(t, initial, rs.LeaderPin)
	require.Equal(t, startTime, rs.StartTime)
	require.Equal(t, height, rs.Proposal.Height)
}
