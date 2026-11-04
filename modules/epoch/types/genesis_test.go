package types_test

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/epoch/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
	for _, tc := range []struct {
		desc     string
		genState *types.GenesisState
		valid    bool
	}{
		{
			desc:     "default is valid",
			genState: types.DefaultGenesis(),
			valid:    true,
		},
		{
			desc:     "invalid genesis state",
			genState: &types.GenesisState{},
			valid:    false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestGenesisState_DurationValidation(t *testing.T) {
	for _, duration := range []time.Duration{
		time.Duration(-1 << 63),
		-time.Minute,
		-time.Nanosecond,
		0,
		time.Nanosecond,
		time.Minute,
	} {
		t.Run(duration.String(), func(t *testing.T) {
			genesis := types.DefaultGenesis()
			genesis.Epoch.EpochDuration = duration
			if duration <= 0 {
				require.Error(t, genesis.Epoch.Validate())
				require.Error(t, genesis.Validate())
			} else {
				require.NoError(t, genesis.Epoch.Validate())
				require.NoError(t, genesis.Validate())
			}
		})
	}
}

func TestDefaultGenesisIsDeterministic(t *testing.T) {
	first := types.DefaultGenesis()
	second := types.DefaultGenesis()

	require.Equal(t, first, second)
	require.Equal(t, types.DefaultGenesisTime(), first.Epoch.GenesisTime)
	require.Equal(t, types.DefaultGenesisTime(), first.Epoch.CurrentEpochStartTime)
}
