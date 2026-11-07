package keeper

import (
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
)

func TestFulfilProgramDisabledBeforeTheMergePlan(t *testing.T) {
	s := newProgramFulfilSuite(t, 4)
	s.record(programFulfilP, 3, types.KindSearch, programFulfilPayload, paxProgramAsset, sdk.NewInt(75))
	observation := s.observation(programFulfilP, 3, []byte("answer"), 6)
	valid := s.signed(observation, s.keys[:3]...)
	decoded, err := types.DecodeObservation(valid)
	require.NoError(t, err)
	_, err = verifyProgramSignatures(s.k.GetAttestorSet(s.ctx), decoded)
	require.NoError(t, err)
	require.False(t, s.pending(programFulfilP, 3).Fulfilled)

	upgrades, ok := s.k.upgrades.(*upgradekeeper.Keeper)
	require.True(t, ok)
	for _, key := range s.ctx.MultiStore().StoreKeys() {
		if key.Name() == upgradetypes.StoreKey {
			s.ctx.KVStore(key).Delete(append([]byte{upgradetypes.DoneByte}, []byte(ProgramOriginPlan)...))
		}
	}
	require.Zero(t, upgrades.GetDoneHeight(s.ctx, ProgramOriginPlan))
	for _, planDone := range []bool{false, true} {
		if planDone {
			upgrades.SetDone(s.ctx.WithBlockHeight(100), ProgramOriginPlan)
		}
		for _, height := range []int64{1, 8, 99} {
			s.ctx = s.ctx.WithBlockHeight(height)
			for _, paused := range []bool{false, true} {
				t.Run(fmt.Sprintf("done_%t/height_%d/paused_%t", planDone, height, paused), func(t *testing.T) {
					if paused {
						require.NoError(t, s.k.Pause(s.ctx, types.MsgPause{Authority: types.DefaultAuthority()}))
					} else {
						require.NoError(t, s.k.Unpause(s.ctx, types.MsgUnpause{Authority: types.DefaultAuthority()}))
					}
					require.False(t, s.k.ProgramOriginActive(s.ctx))
					for _, raw := range [][]byte{valid, {1}, nil} {
						before := s.state()
						err := s.k.FulfilProgram(s.ctx, raw)
						require.EqualError(t, err, "LXP_MODULE_DISABLED")
						require.ErrorIs(t, err, ErrProgramModuleDisabled)
						require.Equal(t, before, s.state())
					}
				})
			}
		}
	}
}

func TestFulfilProgramPausedAfterTheMergePlanIsPaused(t *testing.T) {
	s := newProgramFulfilSuite(t, 4)
	s.record(programFulfilP, 3, types.KindSearch, programFulfilPayload, paxProgramAsset, sdk.NewInt(75))
	valid := s.signed(s.observation(programFulfilP, 3, []byte("answer"), 6), s.keys[:3]...)
	require.NoError(t, s.k.Pause(s.ctx, types.MsgPause{Authority: types.DefaultAuthority()}))
	for _, height := range []int64{8, 9, 100} {
		t.Run(fmt.Sprintf("height_%d", height), func(t *testing.T) {
			s.ctx = s.ctx.WithBlockHeight(height)
			require.True(t, s.k.ProgramOriginActive(s.ctx))
			require.True(t, s.k.IsPaused(s.ctx))
			for _, raw := range [][]byte{valid, {1}, nil} {
				before := s.state()
				err := s.k.FulfilProgram(s.ctx, raw)
				require.Equal(t, "LXP_XWEB_PAUSED", programFulfilCode(err))
				require.ErrorIs(t, err, types.ErrPaused)
				require.Equal(t, before, s.state())
			}
		})
	}
}
