package tests

import (
	"encoding/json"
	"os"
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/stretchr/testify/require"
)

func TestRecordedValidatorPagination(t *testing.T) {
	data, err := os.ReadFile("mock_data/blocks/169638844.json")
	require.NoError(t, err)
	var fixture RpcResponse
	require.NoError(t, json.Unmarshal(data, &fixture))
	client := &MockClient{}
	mockTendermintStateFromJson(fixture.Result.TendermintState, client)
	height := int64(169638843)
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
