package types_test

import (
	"bytes"
	"sort"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

var header = tmproto.Header{ChainID: "hello", Height: 5}

func createValidators(t *testing.T) []types.Validator {
	keys := []cryptotypes.PubKey{
		ed25519.GenPrivKeyFromSecret([]byte("historical-1")).PubKey(),
		ed25519.GenPrivKeyFromSecret([]byte("historical-2")).PubKey(),
		ed25519.GenPrivKeyFromSecret([]byte("historical-3")).PubKey(),
		ed25519.GenPrivKeyFromSecret([]byte("historical-4")).PubKey(),
	}
	sort.Slice(keys, func(i, j int) bool { return bytes.Compare(keys[i].Address(), keys[j].Address()) < 0 })
	powers := []int64{30, 30, 20, 10}
	validators := make([]types.Validator, len(keys))
	for i, key := range keys {
		validator := newValidator(t, sdk.ValAddress(bytes.Repeat([]byte{byte(4 - i)}, 20)), key)
		validator.Status = types.Bonded
		validator.Tokens = sdk.TokensFromConsensusPower(powers[i], sdk.DefaultPowerReduction)
		validator.DelegatorShares = validator.Tokens.ToDec()
		validators[i] = validator
	}
	return []types.Validator{validators[3], validators[1], validators[2], validators[0]}
}

func repairM688AssertHistoricalOrder(t *testing.T, hi types.HistoricalInfo) {
	t.Helper()
	require.Len(t, hi.Valset, 4)
	for i, power := range []int64{30, 30, 20, 10} {
		require.Equal(t, power, hi.Valset[i].ConsensusPower(sdk.DefaultPowerReduction))
		require.Equal(t, sdk.ValAddress(bytes.Repeat([]byte{byte(4 - i)}, 20)).String(), hi.Valset[i].OperatorAddress)
		require.Equal(t, types.Bonded, hi.Valset[i].Status)
		if i > 0 {
			previous, err := hi.Valset[i-1].GetConsAddr()
			require.NoError(t, err)
			current, err := hi.Valset[i].GetConsAddr()
			require.NoError(t, err)
			require.Less(t, bytes.Compare(previous, current), 0)
			require.Greater(t, bytes.Compare(hi.Valset[i-1].GetOperator(), hi.Valset[i].GetOperator()), 0)
		}
	}
	require.True(t, sort.SliceIsSorted(hi.Valset, func(i, j int) bool {
		return types.ValidatorsByVotingPower(hi.Valset).Less(i, j, sdk.DefaultPowerReduction)
	}), "Validators are not sorted")
}

func TestHistoricalInfo(t *testing.T) {
	hi := types.NewHistoricalInfo(header, createValidators(t), sdk.DefaultPowerReduction)
	repairM688AssertHistoricalOrder(t, hi)
	require.NoError(t, types.ValidateBasic(hi, sdk.DefaultPowerReduction))
	h := repairM688Open(t, t.TempDir(), true)
	h.staking.SetHistoricalInfo(h.ctx, header.Height, &hi)
	recv, found := h.staking.GetHistoricalInfo(h.ctx, header.Height)
	require.True(t, found)
	require.True(t, hi.Equal(&recv))
	require.Equal(t, hi.Header, recv.Header)
	repairM688AssertHistoricalOrder(t, recv)
	require.NoError(t, types.ValidateBasic(recv, sdk.DefaultPowerReduction))
}

func TestValidateBasic(t *testing.T) {
	validators := createValidators(t)
	hi := types.HistoricalInfo{Header: header}
	require.Error(t, types.ValidateBasic(hi, sdk.DefaultPowerReduction), "ValidateBasic passed on nil ValSet")
	validators = types.NewHistoricalInfo(header, validators, sdk.DefaultPowerReduction).Valset
	validators[0], validators[len(validators)-1] = validators[len(validators)-1], validators[0]
	hi.Valset = validators
	require.Error(t, types.ValidateBasic(hi, sdk.DefaultPowerReduction), "ValidateBasic passed on unsorted ValSet")
	hi = types.NewHistoricalInfo(header, validators, sdk.DefaultPowerReduction)
	require.NoError(t, types.ValidateBasic(hi, sdk.DefaultPowerReduction), "ValidateBasic failed on valid HistoricalInfo")
	// Equal-power validators must follow consensus addresses even when operators sort oppositely.
	hi.Valset[0], hi.Valset[1] = hi.Valset[1], hi.Valset[0]
	require.Error(t, types.ValidateBasic(hi, sdk.DefaultPowerReduction), "ValidateBasic accepted reversed consensus-address tie")
	sort.Sort(types.Validators(hi.Valset))
	require.Error(t, types.ValidateBasic(hi, sdk.DefaultPowerReduction), "ValidateBasic accepted operator ordering")
}
