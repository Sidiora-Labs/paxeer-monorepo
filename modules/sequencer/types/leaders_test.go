package types_test

import (
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/stretchr/testify/require"
)

func TestValidateLeadersRefusesEmpty(t *testing.T) {
	for _, tc := range []struct {
		name    string
		leaders [][32]byte
	}{
		{name: "nil"},
		{name: "empty", leaders: [][32]byte{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := types.ValidateLeaders(tc.leaders)
			require.ErrorIs(t, err, types.ErrInvalidGenesis)
			require.Contains(t, err.Error(), "leader list is empty")
		})
	}
}

func TestValidateLeadersRefusesZeroKey(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run(fmt.Sprintf("index_%d", i), func(t *testing.T) {
			leaders := [][32]byte{{1}, {2}, {3}}
			leaders[i] = [32]byte{}

			err := types.ValidateLeaders(leaders)
			require.ErrorIs(t, err, types.ErrInvalidGenesis)
			require.Contains(t, err.Error(), fmt.Sprintf("leader %d is the zero key", i))
		})
	}
}

func TestValidateLeadersRefusesDuplicate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		leaders [][32]byte
		index   int
	}{
		{name: "adjacent", leaders: [][32]byte{{1}, {1}}, index: 1},
		{name: "separated", leaders: [][32]byte{{1}, {2}, {1}}, index: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := types.ValidateLeaders(tc.leaders)
			require.ErrorIs(t, err, types.ErrInvalidGenesis)
			require.Contains(t, err.Error(), fmt.Sprintf("leader %d repeats an earlier leader", tc.index))
		})
	}

	t.Run("single_nonzero", func(t *testing.T) {
		require.NoError(t, types.ValidateLeaders([][32]byte{{1}}))
	})
	t.Run("distinct_nonzero", func(t *testing.T) {
		require.NoError(t, types.ValidateLeaders([][32]byte{{1}, {2}, {0: 1, 31: 1}, {31: 1}}))
	})
}
