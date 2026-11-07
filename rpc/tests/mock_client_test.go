package tests

import (
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/stretchr/testify/require"
)

func TestValidatorPagination(t *testing.T) {
	height := int64(1)
	set := make([]*tmtypes.Validator, 30)
	for i := range set {
		key := ed25519.TestSecretKey([]byte(fmt.Sprintf("rpc-test-validator-%d", i)))
		set[i] = tmtypes.NewValidator(key.Public(), 1)
	}
	client := &MockClient{mockedValidators: map[int64]*coretypes.ResultValidators{
		height: {BlockHeight: height, Validators: set, Count: 30, Total: 40},
	}}
	captured := client.mockedValidators[height]
	require.NotNil(t, captured)
	require.Len(t, captured.Validators, 30)
	require.Equal(t, 40, captured.Total)
	originalCount, originalTotal := captured.Count, captured.Total

	whole, err := client.Validators(t.Context(), &height, nil, nil)
	require.NoError(t, err)
	require.Equal(t, captured.Validators, whole.Validators)
	require.Equal(t, 30, whole.Count)
	require.Equal(t, 30, whole.Total)

	pageSize := 7
	var validators []*tmtypes.Validator
	for page := 1; page <= 5; page++ {
		result, err := client.Validators(t.Context(), &height, &page, &pageSize)
		require.NoError(t, err)
		require.Equal(t, height, result.BlockHeight)
		require.Equal(t, 30, result.Count)
		require.Equal(t, 30, result.Total)
		start := (page - 1) * pageSize
		require.Equal(t, captured.Validators[start:min(start+pageSize, 30)], result.Validators)
		validators = append(validators, result.Validators...)
	}
	require.Equal(t, captured.Validators, validators)
	for _, page := range []int{-1, 0, 6} {
		result, err := client.Validators(t.Context(), &height, &page, &pageSize)
		require.ErrorContains(t, err, "outside range")
		require.Nil(t, result)
	}
	require.Equal(t, originalCount, captured.Count)
	require.Equal(t, originalTotal, captured.Total)
	require.Len(t, captured.Validators, 30)
}
