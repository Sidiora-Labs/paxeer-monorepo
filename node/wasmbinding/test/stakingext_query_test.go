package wasmbinding

import (
	"encoding/json"
	"testing"
	"time"

	epochwasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/epoch/client/wasm"
	evmwasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/client/wasm"
	oraclewasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/client/wasm"
	tokenfactorywasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/wasm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/wasmbinding"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestWasmStakingExtUnbondingInitialBalance(t *testing.T) {
	testWrapper := app.NewTestWrapper(t, time.Now().UTC(), ed25519.GenPrivKey().PubKey(), false)
	oh := oraclewasm.NewOracleWasmQueryHandler(&testWrapper.App.OracleKeeper)
	eh := epochwasm.NewEpochWasmQueryHandler(&testWrapper.App.EpochKeeper)
	th := tokenfactorywasm.NewTokenFactoryWasmQueryHandler(&testWrapper.App.TokenFactoryKeeper)
	evmh := evmwasm.NewEVMQueryHandler(&testWrapper.App.EvmKeeper)
	customQuerier := wasmbinding.CustomQuerier(wasmbinding.NewQueryPlugin(oh, eh, th, evmh, testWrapper.App.StakingKeeper))
	keeper := testWrapper.App.StakingKeeper
	ctx := testWrapper.Ctx
	validators := keeper.GetAllValidators(ctx)
	require.Len(t, validators, 1)
	validator := validators[0]
	valAddr := validator.GetOperator()
	delegator := sdk.MustAccAddressFromBech32(app.TestUser)
	bondAmount := keeper.TokensFromConsensusPower(ctx, 10)
	testWrapper.FundAcc(delegator, sdk.NewCoins(sdk.NewCoin(keeper.BondDenom(ctx), bondAmount)))
	_, err := keeper.Delegate(ctx, delegator, bondAmount, stakingtypes.Unbonded, validator, true)
	require.NoError(t, err)
	_, err = keeper.ApplyAndReturnValidatorSetUpdates(ctx)
	require.NoError(t, err)
	validator, found := keeper.GetValidator(ctx, valAddr)
	require.True(t, found)
	require.True(t, validator.IsBonded())
	consAddr, err := validator.GetConsAddr()
	require.NoError(t, err)
	power := validator.GetConsensusPower(keeper.PowerReduction(ctx))

	ctx = ctx.WithBlockHeight(10)
	unbondAmount := keeper.TokensFromConsensusPower(ctx, 4)
	shares, err := keeper.ValidateUnbondAmount(ctx, delegator, valAddr, unbondAmount)
	require.NoError(t, err)
	completionTime, err := keeper.Undelegate(ctx, delegator, valAddr, shares)
	require.NoError(t, err)

	queryData, err := json.Marshal(wasmbinding.StakingExtQuery{
		UnbondingDelegations: &wasmbinding.UnbondingDelegationsRequest{Delegator: delegator.String()},
	})
	require.NoError(t, err)
	rawQuery, err := json.Marshal(wasmbinding.PaxQueryWrapper{
		Route: wasmbinding.StakingExtRoute, QueryData: queryData,
	})
	require.NoError(t, err)
	query := func() wasmbinding.UnbondingDelegationsResponse {
		res, err := customQuerier(ctx, rawQuery)
		require.NoError(t, err)
		var response wasmbinding.UnbondingDelegationsResponse
		require.NoError(t, json.Unmarshal(res, &response))
		return response
	}
	expected := wasmbinding.UnbondingDelegationsResponse{
		Entries: []wasmbinding.UnbondingDelegationEntry{{
			CreationHeight: 10,
			CompletionTime: completionTime.String(),
			InitialBalance: unbondAmount,
			Balance:        unbondAmount,
		}},
	}
	require.Equal(t, expected, query())

	ctx = ctx.WithBlockHeight(11).WithBlockTime(ctx.BlockTime().Add(time.Second))
	require.True(t, ctx.BlockTime().Before(completionTime))
	slashFraction := sdk.NewDecWithPrec(1, 1)
	keeper.Slash(ctx, consAddr, 9, power, slashFraction)
	remainingBalance := unbondAmount.Sub(slashFraction.MulInt(unbondAmount).TruncateInt())
	unbonding, found := keeper.GetUnbondingDelegation(ctx, delegator, valAddr)
	require.True(t, found)
	require.Len(t, unbonding.Entries, 1)
	require.Equal(t, unbondAmount, unbonding.Entries[0].InitialBalance)
	require.Equal(t, remainingBalance, unbonding.Entries[0].Balance)
	require.True(t, remainingBalance.LT(unbondAmount))
	expected.Entries[0].Balance = remainingBalance
	require.Equal(t, expected, query())
}
