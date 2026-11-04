package types_test

import (
	"sort"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
)

var header = tmproto.Header{
	ChainID: "hello",
	Height:  5,
}

func createValidators(t *testing.T) []types.Validator {
	return []types.Validator{
		newValidator(t, valAddr1, pk1),
		newValidator(t, valAddr2, pk2),
		newValidator(t, valAddr3, pk3),
	}
}

func TestHistoricalInfo(t *testing.T) {
	validators := createValidators(t)
	hi := types.NewHistoricalInfo(header, validators, sdk.DefaultPowerReduction)
	require.True(t, sort.SliceIsSorted(hi.Valset, func(i, j int) bool {
		return types.ValidatorsByVotingPower(hi.Valset).Less(i, j, sdk.DefaultPowerReduction)
	}), "Validators are not sorted")

	var value []byte
	require.NotPanics(t, func() {
		value = types.ModuleCdc.MustMarshal(&hi)
	})
	require.NotNil(t, value, "Marshalled HistoricalInfo is nil")

	recv, err := types.UnmarshalHistoricalInfo(types.ModuleCdc, value)
	require.Nil(t, err, "Unmarshalling HistoricalInfo failed")
	require.Equal(t, hi.Header, recv.Header)
	for i := range hi.Valset {
		require.True(t, hi.Valset[i].Equal(&recv.Valset[i]))
	}
	require.True(t, sort.SliceIsSorted(hi.Valset, func(i, j int) bool {
		return types.ValidatorsByVotingPower(hi.Valset).Less(i, j, sdk.DefaultPowerReduction)
	}), "Validators are not sorted")
}

func TestValidateBasic(t *testing.T) {
	validators := createValidators(t)
	hi := types.HistoricalInfo{
		Header: header,
	}
	err := types.ValidateBasic(hi, sdk.DefaultPowerReduction)
	require.Error(t, err, "ValidateBasic passed on nil ValSet")

	// Reverse the producer's consensus ordering to make an invalid set.
	validators = types.NewHistoricalInfo(header, validators, sdk.DefaultPowerReduction).Valset
	validators[0], validators[len(validators)-1] = validators[len(validators)-1], validators[0]

	hi = types.HistoricalInfo{
		Header: header,
		Valset: validators,
	}
	err = types.ValidateBasic(hi, sdk.DefaultPowerReduction)
	require.Error(t, err, "ValidateBasic passed on unsorted ValSet")

	hi = types.NewHistoricalInfo(header, validators, sdk.DefaultPowerReduction)
	err = types.ValidateBasic(hi, sdk.DefaultPowerReduction)
	require.NoError(t, err, "ValidateBasic failed on valid HistoricalInfo")
}
