package keeper_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
)

const stakingMeterName = "paxcosmos_x_staking_keeper"

var stakingMetricsReader = oteltest.Install()

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

func TestStakingMetricsLargeAmounts(t *testing.T) {
	testStakingMetricsLargeAmounts(t)
}

func testStakingMetricsLargeAmounts(t *testing.T) {
	t.Helper()
	reader := stakingMetricsReader
	for _, operation := range []struct {
		name   string
		total  func(*oteltest.Reader) int64
		amount func(*oteltest.Reader) (int64, bool)
	}{
		{"delegate", delegateTotal, delegateAmount},
		{"redelegate", redelegateTotal, redelegateAmount},
		{"undelegate", undelegateTotal, undelegateAmount},
	} {
		t.Run(operation.name, func(t *testing.T) {
			f := repair310Open(t, t.TempDir())
			t.Cleanup(func() { require.NoError(t, f.store.Close()) })
			f.ctx = f.ctx.WithBlockHeight(1)
			source := repair310Validator(t, f.staking, f.ctx, 250).UpdateStatus(types.Unbonding)
			source.UnbondingTime = f.ctx.BlockTime().Add(24 * time.Hour)
			f.staking.SetValidator(f.ctx, source)
			destination := repair310Validator(t, f.staking, f.ctx, 251)
			delegator := sdk.AccAddress(PKs[252].Address())
			denom := f.staking.BondDenom(f.ctx)
			max := sdk.NewInt(math.MaxInt64)
			amounts := []sdk.Int{sdk.OneInt(), max, max.AddRaw(1), max.MulRaw(2), sdk.OneInt()}
			funds := sdk.NewInt(100)
			for _, amount := range amounts {
				funds = funds.Add(amount)
			}
			f.fund(t, delegator, funds)
			server := keeper.NewMsgServerImpl(f.staking)
			goCtx := sdk.WrapSDKContext(f.ctx)
			if operation.name != "delegate" {
				_, err := server.Delegate(goCtx, types.NewMsgDelegate(delegator, source.GetOperator(), sdk.NewCoin(denom, funds)))
				require.NoError(t, err)
			}
			for index, amount := range amounts {
				t.Run(fmt.Sprintf("amount_%s", amount), func(t *testing.T) {
					beforeTotal := operation.total(reader)
					beforeOverflow := reader.CounterSum(stakingMeterName, "staking_keeper_amount_overflow")
					balanceBefore := f.bank.GetBalance(f.ctx, delegator, denom)
					pool := f.staking.GetNotBondedPool(f.ctx).GetAddress()
					poolBefore := f.bank.GetBalance(f.ctx, pool, denom)
					validatorBefore, found := f.staking.GetValidator(f.ctx, source.GetOperator())
					require.True(t, found)
					coin := sdk.NewCoin(denom, amount)
					switch operation.name {
					case "delegate":
						response, err := server.Delegate(goCtx, types.NewMsgDelegate(delegator, source.GetOperator(), coin))
						require.NoError(t, err)
						require.NotNil(t, response)
						require.True(t, balanceBefore.Sub(coin).IsEqual(f.bank.GetBalance(f.ctx, delegator, denom)))
						require.True(t, poolBefore.Add(coin).IsEqual(f.bank.GetBalance(f.ctx, pool, denom)))
						delegation, found := f.staking.GetDelegation(f.ctx, delegator, source.GetOperator())
						require.True(t, found)
						require.Equal(t, validatorBefore.DelegatorShares.Add(amount.ToDec()), delegation.Shares)
					case "redelegate":
						destinationBefore, found := f.staking.GetValidator(f.ctx, destination.GetOperator())
						require.True(t, found)
						response, err := server.BeginRedelegate(goCtx, types.NewMsgBeginRedelegate(delegator, source.GetOperator(), destination.GetOperator(), coin))
						require.NoError(t, err)
						require.NotNil(t, response)
						require.Equal(t, source.UnbondingTime, response.CompletionTime)
						redelegation, found := f.staking.GetRedelegation(f.ctx, delegator, source.GetOperator(), destination.GetOperator())
						require.True(t, found)
						require.Len(t, redelegation.Entries, index+1)
						require.True(t, amount.Equal(redelegation.Entries[index].InitialBalance))
						destinationAfter, found := f.staking.GetValidator(f.ctx, destination.GetOperator())
						require.True(t, found)
						require.True(t, destinationBefore.Tokens.Add(amount).Equal(destinationAfter.Tokens))
					case "undelegate":
						response, err := server.Undelegate(goCtx, types.NewMsgUndelegate(delegator, source.GetOperator(), coin))
						require.NoError(t, err)
						require.NotNil(t, response)
						unbonding, found := f.staking.GetUnbondingDelegation(f.ctx, delegator, source.GetOperator())
						require.True(t, found)
						require.Len(t, unbonding.Entries, index+1)
						require.True(t, amount.Equal(unbonding.Entries[index].Balance))
						require.Equal(t, response.CompletionTime, unbonding.Entries[index].CompletionTime)
					}
					validatorAfter, found := f.staking.GetValidator(f.ctx, source.GetOperator())
					require.True(t, found)
					if operation.name == "delegate" {
						require.True(t, validatorBefore.Tokens.Add(amount).Equal(validatorAfter.Tokens))
					} else {
						require.True(t, validatorBefore.Tokens.Sub(amount).Equal(validatorAfter.Tokens))
						require.True(t, balanceBefore.IsEqual(f.bank.GetBalance(f.ctx, delegator, denom)))
						require.True(t, poolBefore.IsEqual(f.bank.GetBalance(f.ctx, pool, denom)))
					}
					require.Equal(t, int64(1), operation.total(reader)-beforeTotal)
					gauge, found := operation.amount(reader)
					require.True(t, found)
					if amount.IsInt64() {
						require.Equal(t, amount.Int64(), gauge)
						require.Equal(t, beforeOverflow, reader.CounterSum(stakingMeterName, "staking_keeper_amount_overflow"))
					} else {
						require.Equal(t, int64(math.MaxInt64), gauge)
						require.Equal(t, int64(1), reader.CounterSum(stakingMeterName, "staking_keeper_amount_overflow")-beforeOverflow)
					}
				})
			}
			beforeTotal := operation.total(reader)
			beforeOverflow := reader.CounterSum(stakingMeterName, "staking_keeper_amount_overflow")
			beforeValidator, found := f.staking.GetValidator(f.ctx, source.GetOperator())
			require.True(t, found)
			badCoin := sdk.NewInt64Coin("wrong", 1)
			var err error
			switch operation.name {
			case "delegate":
				_, err = server.Delegate(goCtx, types.NewMsgDelegate(delegator, source.GetOperator(), badCoin))
			case "redelegate":
				_, err = server.BeginRedelegate(goCtx, types.NewMsgBeginRedelegate(delegator, source.GetOperator(), destination.GetOperator(), badCoin))
			case "undelegate":
				_, err = server.Undelegate(goCtx, types.NewMsgUndelegate(delegator, source.GetOperator(), badCoin))
			}
			require.Error(t, err)
			require.Equal(t, beforeTotal, operation.total(reader))
			require.Equal(t, beforeOverflow, reader.CounterSum(stakingMeterName, "staking_keeper_amount_overflow"))
			gauge, found := operation.amount(reader)
			require.True(t, found)
			require.Equal(t, int64(1), gauge)
			afterValidator, found := f.staking.GetValidator(f.ctx, source.GetOperator())
			require.True(t, found)
			require.Equal(t, beforeValidator, afterValidator)
		})
	}
}
