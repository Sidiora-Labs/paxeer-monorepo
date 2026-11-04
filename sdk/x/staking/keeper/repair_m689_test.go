package keeper_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
)

func TestRepairM_689_UnbondingRetryRestart(t *testing.T) {
	testUnbondingRetryRestart(t)
}

func TestRepairM_689_RedelegationPartialFilters(t *testing.T) {
	testRedelegationPartialFilters(t)
}

func TestRepairM_689_MinimumSelfBacking(t *testing.T) {
	for _, tc := range []struct {
		name        string
		selfPower   int64
		slash       bool
		newMinPower int64
		accepted    bool
	}{
		{"no_operator_delegation", 0, false, 1, false},
		{"third_party_stake_is_not_self_backing", 100, false, 101, false},
		{"slashed_shares_use_current_token_value", 100, true, 60, false},
		{"minimum_at_current_token_value", 100, true, 50, true},
		{"minimum_at_unslashed_self_stake", 100, false, 100, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := repair310Open(t, t.TempDir())
			t.Cleanup(func() { require.NoError(t, f.store.Close()) })
			f.ctx = f.ctx.WithBlockHeight(1)
			operator := sdk.AccAddress(PKs[260].Address())
			address := sdk.ValAddress(operator)
			thirdParty := sdk.AccAddress(PKs[261].Address())
			denom := f.staking.BondDenom(f.ctx)
			server := keeper.NewMsgServerImpl(f.staking)
			if tc.selfPower == 0 {
				validator := repair310Validator(t, f.staking, f.ctx, 260)
				validator, err := validator.SetInitialCommission(types.NewCommissionWithTime(f.staking.MinCommissionRate(f.ctx), sdk.OneDec(), sdk.OneDec(), f.ctx.BlockTime()))
				require.NoError(t, err)
				f.staking.SetValidator(f.ctx, validator)
			} else {
				self := f.staking.TokensFromConsensusPower(f.ctx, tc.selfPower)
				f.fund(t, operator, self)
				message, err := types.NewMsgCreateValidator(address, PKs[260], sdk.NewCoin(denom, self),
					types.Description{Moniker: "original"},
					types.NewCommissionRates(f.staking.MinCommissionRate(f.ctx), sdk.OneDec(), sdk.OneDec()), sdk.OneInt())
				require.NoError(t, err)
				require.NoError(t, message.ValidateBasic())
				response, err := server.CreateValidator(sdk.WrapSDKContext(f.ctx), message)
				require.NoError(t, err)
				require.NotNil(t, response)
			}
			thirdPartyStake := f.staking.TokensFromConsensusPower(f.ctx, 10_000)
			f.fund(t, thirdParty, thirdPartyStake)
			_, err := server.Delegate(sdk.WrapSDKContext(f.ctx), types.NewMsgDelegate(thirdParty, address, sdk.NewCoin(denom, thirdPartyStake)))
			require.NoError(t, err)
			staking.EndBlocker(f.ctx, f.staking)
			f.ctx = f.ctx.WithBlockHeight(2).WithBlockTime(f.ctx.BlockTime().Add(48 * time.Hour))
			if tc.slash {
				f.staking.Slash(f.ctx, sdk.GetConsAddress(PKs[260]), f.ctx.BlockHeight(), f.staking.GetLastValidatorPower(f.ctx, address), sdk.NewDecWithPrec(5, 1))
			}
			before, found := f.staking.GetValidator(f.ctx, address)
			require.True(t, found)
			minimum := f.staking.TokensFromConsensusPower(f.ctx, tc.newMinPower)
			require.True(t, before.Tokens.GT(minimum), "large third-party stake makes the old aggregate-token check pass")
			selfDelegation, hasSelf := f.staking.GetDelegation(f.ctx, operator, address)
			if tc.selfPower == 0 {
				require.False(t, hasSelf)
			} else {
				require.True(t, hasSelf)
				selfValue := before.TokensFromShares(selfDelegation.Shares).TruncateInt()
				if tc.accepted {
					require.True(t, selfValue.Equal(minimum))
				} else {
					require.True(t, selfValue.LT(minimum))
				}
				if tc.slash && !tc.accepted {
					require.True(t, selfDelegation.Shares.GT(minimum.ToDec()), "shares alone also overstate the slashed self-backing")
				}
			}
			store := f.ctx.KVStore(f.staking.GetStoreKey())
			validatorBytes := append([]byte(nil), store.Get(types.GetValidatorKey(address))...)
			eventsBefore := len(f.ctx.EventManager().Events())
			commission := before.Commission.Rate.Add(sdk.NewDecWithPrec(1, 2))
			message := types.NewMsgEditValidator(address, types.Description{Moniker: "edited"}, &commission, &minimum)
			require.NoError(t, message.ValidateBasic())
			response, err := server.EditValidator(sdk.WrapSDKContext(f.ctx), message)
			after, found := f.staking.GetValidator(f.ctx, address)
			require.True(t, found)
			if tc.accepted {
				require.NoError(t, err)
				require.NotNil(t, response)
				require.True(t, minimum.Equal(after.MinSelfDelegation))
				require.Equal(t, "edited", after.Description.Moniker)
				require.Equal(t, commission, after.Commission.Rate)
				require.True(t, before.Tokens.Equal(after.Tokens))
				require.Equal(t, before.DelegatorShares, after.DelegatorShares)
			} else {
				require.ErrorIs(t, err, types.ErrSelfDelegationBelowMinimum)
				require.Nil(t, response)
				require.Equal(t, before, after)
				require.Equal(t, validatorBytes, store.Get(types.GetValidatorKey(address)))
				require.Len(t, f.ctx.EventManager().Events(), eventsBefore)
			}
			if hasSelf {
				afterSelf, found := f.staking.GetDelegation(f.ctx, operator, address)
				require.True(t, found)
				require.Equal(t, selfDelegation, afterSelf)
			}
			messageText, broken := bankkeeper.TotalSupply(f.bank)(f.ctx)
			require.False(t, broken, messageText)
		})
	}
}

func TestRepairM_689_LargeAmountMetrics(t *testing.T) {
	testStakingMetricsLargeAmounts(t)
}

func TestRepairM_689_BenchmarkContext(t *testing.T) {
	result := testing.Benchmark(BenchmarkGetValidator)
	require.Positive(t, result.N)
	require.Positive(t, result.T)
	t.Logf("BenchmarkGetValidator: %s", result.String())
}
