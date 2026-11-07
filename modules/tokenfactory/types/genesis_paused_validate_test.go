package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
)

func TestGenesisPausedDenomsMustBeKnown(t *testing.T) {
	const creator = "pax1y3pxq5dp900czh0mkudhjdqjq5m8cpmmsnt288"
	const known = "factory/" + creator + "/bitcoin"
	const unknown = "factory/" + creator + "/unknown"

	t.Run("default genesis", func(t *testing.T) {
		genesis := types.DefaultGenesis()
		require.Empty(t, genesis.GetPausedDenoms())
		require.NoError(t, genesis.Validate())
	})

	for _, tc := range []struct {
		name          string
		factoryDenoms []types.GenesisDenom
		pausedDenoms  []string
		unknownDenom  string
		invalid       bool
	}{
		{
			name:          "known denom",
			factoryDenoms: []types.GenesisDenom{{Denom: known}},
			pausedDenoms:  []string{known},
		},
		{
			name:          "no paused denoms",
			factoryDenoms: []types.GenesisDenom{{Denom: known}},
		},
		{
			name:         "no factory denoms",
			pausedDenoms: []string{unknown},
			unknownDenom: unknown,
			invalid:      true,
		},
		{
			name:          "unknown after known denom",
			factoryDenoms: []types.GenesisDenom{{Denom: known}},
			pausedDenoms:  []string{known, unknown},
			unknownDenom:  unknown,
			invalid:       true,
		},
		{
			name:          "empty paused denom",
			factoryDenoms: []types.GenesisDenom{{Denom: known}},
			pausedDenoms:  []string{""},
			invalid:       true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			genesis := types.DefaultGenesis()
			genesis.FactoryDenoms = tc.factoryDenoms
			genesis.PausedDenoms = tc.pausedDenoms

			err := genesis.Validate()
			if tc.invalid {
				require.ErrorIs(t, err, types.ErrInvalidGenesis)
				require.ErrorContains(t, err, "paused denom "+tc.unknownDenom+" is not a factory denom")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGenesisPausedDenomsUnique(t *testing.T) {
	const creator = "pax1y3pxq5dp900czh0mkudhjdqjq5m8cpmmsnt288"
	const first = "factory/" + creator + "/bitcoin"
	const second = "factory/" + creator + "/litecoin"

	for _, tc := range []struct {
		name         string
		pausedDenoms []string
		duplicate    string
	}{
		{name: "nil list"},
		{name: "empty list", pausedDenoms: []string{}},
		{name: "single denom", pausedDenoms: []string{first}},
		{name: "distinct denoms", pausedDenoms: []string{second, first}},
		{name: "adjacent duplicate", pausedDenoms: []string{first, first}, duplicate: first},
		{name: "non-adjacent duplicate", pausedDenoms: []string{first, second, first}, duplicate: first},
		{name: "second denom duplicate", pausedDenoms: []string{first, second, second}, duplicate: second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			genesis := types.DefaultGenesis()
			genesis.FactoryDenoms = []types.GenesisDenom{{Denom: first}, {Denom: second}}
			genesis.PausedDenoms = tc.pausedDenoms

			err := genesis.Validate()
			if tc.duplicate != "" {
				require.ErrorIs(t, err, types.ErrInvalidGenesis)
				require.ErrorContains(t, err, "duplicate paused denom: "+tc.duplicate)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
