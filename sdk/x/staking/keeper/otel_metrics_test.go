package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
)

const stakingMeterName = "paxcosmos_x_staking_keeper"

func delegateTotal(reader *oteltest.Reader) int64 {
	return reader.CounterSum(stakingMeterName, "staking_keeper_delegate")
}

func delegateAmount(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt(stakingMeterName, "staking_keeper_last_delegate_amount")
}

func redelegateTotal(reader *oteltest.Reader) int64 {
	return reader.CounterSum(stakingMeterName, "staking_keeper_redelegate")
}

func redelegateAmount(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt(stakingMeterName, "staking_keeper_last_redelegate_amount")
}

func undelegateTotal(reader *oteltest.Reader) int64 {
	return reader.CounterSum(stakingMeterName, "staking_keeper_undelegate")
}

func undelegateAmount(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt(stakingMeterName, "staking_keeper_last_undelegate_amount")
}

func newStakingMetricsFixture(t *testing.T) (*paxapp.App, sdk.Context, sdk.AccAddress, []sdk.ValAddress) {
	t.Helper()
	_, app, ctx := createTestInput(t)
	require.Equal(t, "uhpx", app.StakingKeeper.BondDenom(ctx))
	delegators, validators := generateAddresses(app, ctx, 3)
	for i, addr := range validators[1:] {
		validator := teststaking.NewValidator(t, addr, PKs[i])
		app.StakingKeeper.SetValidator(ctx, validator)
		app.StakingKeeper.SetValidatorByConsAddr(ctx, validator)
		app.StakingKeeper.SetNewValidatorByPowerIndex(ctx, validator)
	}
	return app, ctx, delegators[0], validators[1:]
}

func TestDelegateRecordsDelegateTotalAndAmount(t *testing.T) {
	reader := oteltest.Install()
	app, ctx, delegator, validators := newStakingMetricsFixture(t)
	server := keeper.NewMsgServerImpl(app.StakingKeeper)
	goCtx := sdk.WrapSDKContext(ctx)

	_, err := server.Delegate(goCtx, types.NewMsgDelegate(delegator, validators[0], sdk.NewInt64Coin("uhpx", 1)))
	require.NoError(t, err)
	beforeTotal := delegateTotal(reader)
	beforeAmount, found := delegateAmount(reader)
	require.True(t, found)
	require.Equal(t, int64(1), beforeAmount)
	beforeBalance := app.BankKeeper.GetBalance(ctx, delegator, "uhpx").Amount

	response, err := server.Delegate(goCtx, types.NewMsgDelegate(delegator, validators[0], sdk.NewInt64Coin("uhpx", 10)))
	require.NoError(t, err)
	require.NotNil(t, response)

	require.Equal(t, int64(1), delegateTotal(reader)-beforeTotal)
	afterAmount, found := delegateAmount(reader)
	require.True(t, found)
	require.Equal(t, int64(10), afterAmount)
	require.Equal(t, int64(9), afterAmount-beforeAmount)
	require.Equal(t, sdk.NewInt(10), beforeBalance.Sub(app.BankKeeper.GetBalance(ctx, delegator, "uhpx").Amount))
	delegation, found := app.StakingKeeper.GetDelegation(ctx, delegator, validators[0])
	require.True(t, found)
	require.Equal(t, sdk.NewDec(11), delegation.Shares)
}

func TestBeginRedelegateRecordsRedelegateTotalAndAmount(t *testing.T) {
	reader := oteltest.Install()
	app, ctx, delegator, validators := newStakingMetricsFixture(t)
	server := keeper.NewMsgServerImpl(app.StakingKeeper)
	goCtx := sdk.WrapSDKContext(ctx)

	_, err := server.Delegate(goCtx, types.NewMsgDelegate(delegator, validators[0], sdk.NewInt64Coin("uhpx", 10)))
	require.NoError(t, err)
	_, err = server.BeginRedelegate(goCtx, types.NewMsgBeginRedelegate(delegator, validators[0], validators[1], sdk.NewInt64Coin("uhpx", 1)))
	require.NoError(t, err)
	beforeTotal := redelegateTotal(reader)
	beforeAmount, found := redelegateAmount(reader)
	require.True(t, found)
	require.Equal(t, int64(1), beforeAmount)
	sourceBefore, found := app.StakingKeeper.GetDelegation(ctx, delegator, validators[0])
	require.True(t, found)
	destinationBefore, found := app.StakingKeeper.GetDelegation(ctx, delegator, validators[1])
	require.True(t, found)

	response, err := server.BeginRedelegate(goCtx, types.NewMsgBeginRedelegate(delegator, validators[0], validators[1], sdk.NewInt64Coin("uhpx", 5)))
	require.NoError(t, err)
	require.NotNil(t, response)

	require.Equal(t, int64(1), redelegateTotal(reader)-beforeTotal)
	afterAmount, found := redelegateAmount(reader)
	require.True(t, found)
	require.Equal(t, int64(5), afterAmount)
	require.Equal(t, int64(4), afterAmount-beforeAmount)
	sourceAfter, found := app.StakingKeeper.GetDelegation(ctx, delegator, validators[0])
	require.True(t, found)
	destinationAfter, found := app.StakingKeeper.GetDelegation(ctx, delegator, validators[1])
	require.True(t, found)
	require.Equal(t, sdk.NewDec(5), sourceBefore.Shares.Sub(sourceAfter.Shares))
	require.Equal(t, sdk.NewDec(5), destinationAfter.Shares.Sub(destinationBefore.Shares))
}

func TestUndelegateRecordsUndelegateTotalAndAmount(t *testing.T) {
	reader := oteltest.Install()
	app, ctx, delegator, validators := newStakingMetricsFixture(t)
	server := keeper.NewMsgServerImpl(app.StakingKeeper)
	goCtx := sdk.WrapSDKContext(ctx)

	_, err := server.Delegate(goCtx, types.NewMsgDelegate(delegator, validators[0], sdk.NewInt64Coin("uhpx", 10)))
	require.NoError(t, err)
	_, err = server.Undelegate(goCtx, types.NewMsgUndelegate(delegator, validators[0], sdk.NewInt64Coin("uhpx", 1)))
	require.NoError(t, err)
	beforeTotal := undelegateTotal(reader)
	beforeAmount, found := undelegateAmount(reader)
	require.True(t, found)
	require.Equal(t, int64(1), beforeAmount)
	beforeDelegation, found := app.StakingKeeper.GetDelegation(ctx, delegator, validators[0])
	require.True(t, found)

	response, err := server.Undelegate(goCtx, types.NewMsgUndelegate(delegator, validators[0], sdk.NewInt64Coin("uhpx", 5)))
	require.NoError(t, err)
	require.NotNil(t, response)

	require.Equal(t, int64(1), undelegateTotal(reader)-beforeTotal)
	afterAmount, found := undelegateAmount(reader)
	require.True(t, found)
	require.Equal(t, int64(5), afterAmount)
	require.Equal(t, int64(4), afterAmount-beforeAmount)
	afterDelegation, found := app.StakingKeeper.GetDelegation(ctx, delegator, validators[0])
	require.True(t, found)
	require.Equal(t, sdk.NewDec(5), beforeDelegation.Shares.Sub(afterDelegation.Shares))
	unbonding, found := app.StakingKeeper.GetUnbondingDelegation(ctx, delegator, validators[0])
	require.True(t, found)
	require.Len(t, unbonding.Entries, 2)
	require.Equal(t, sdk.NewInt(5), unbonding.Entries[1].Balance)
	require.Equal(t, response.CompletionTime, unbonding.Entries[1].CompletionTime)
}
