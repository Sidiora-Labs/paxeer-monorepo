package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
)

func TestStakingMsgEmitsNoLegacyTelemetry(t *testing.T) {
	reader := oteltest.Install()
	app, ctx, delegator, validators := newStakingMetricsFixture(t)
	server := keeper.NewMsgServerImpl(app.StakingKeeper)
	goCtx := sdk.WrapSDKContext(ctx)
	sink := oteltest.InstallLegacySink(t)

	beforeDelegate := delegateTotal(reader)
	delegateResponse, err := server.Delegate(goCtx, types.NewMsgDelegate(delegator, validators[0], sdk.NewInt64Coin("uhpx", 10)))
	require.NoError(t, err)
	require.NotNil(t, delegateResponse)
	require.Equal(t, int64(1), delegateTotal(reader)-beforeDelegate)
	amount, found := delegateAmount(reader)
	require.True(t, found)
	require.Equal(t, int64(10), amount)
	delegation, found := app.StakingKeeper.GetDelegation(ctx, delegator, validators[0])
	require.True(t, found)
	require.Equal(t, sdk.NewDec(10), delegation.Shares)

	beforeRedelegate := redelegateTotal(reader)
	redelegateResponse, err := server.BeginRedelegate(goCtx, types.NewMsgBeginRedelegate(delegator, validators[0], validators[1], sdk.NewInt64Coin("uhpx", 5)))
	require.NoError(t, err)
	require.NotNil(t, redelegateResponse)
	require.Equal(t, int64(1), redelegateTotal(reader)-beforeRedelegate)
	amount, found = redelegateAmount(reader)
	require.True(t, found)
	require.Equal(t, int64(5), amount)
	delegation, found = app.StakingKeeper.GetDelegation(ctx, delegator, validators[0])
	require.True(t, found)
	require.Equal(t, sdk.NewDec(5), delegation.Shares)
	delegation, found = app.StakingKeeper.GetDelegation(ctx, delegator, validators[1])
	require.True(t, found)
	require.Equal(t, sdk.NewDec(5), delegation.Shares)

	beforeUndelegate := undelegateTotal(reader)
	undelegateResponse, err := server.Undelegate(goCtx, types.NewMsgUndelegate(delegator, validators[0], sdk.NewInt64Coin("uhpx", 5)))
	require.NoError(t, err)
	require.NotNil(t, undelegateResponse)
	require.Equal(t, int64(1), undelegateTotal(reader)-beforeUndelegate)
	amount, found = undelegateAmount(reader)
	require.True(t, found)
	require.Equal(t, int64(5), amount)
	_, found = app.StakingKeeper.GetDelegation(ctx, delegator, validators[0])
	require.False(t, found)
	unbonding, found := app.StakingKeeper.GetUnbondingDelegation(ctx, delegator, validators[0])
	require.True(t, found)
	require.Len(t, unbonding.Entries, 1)
	require.Equal(t, sdk.NewInt(5), unbonding.Entries[0].Balance)
	require.Equal(t, undelegateResponse.CompletionTime, unbonding.Entries[0].CompletionTime)

	keys := sink.Keys()
	for _, key := range []string{
		"oteltest.staking.delegate",
		"oteltest.staking.redelegate",
		"oteltest.staking.undelegate",
		"oteltest.tx.msg." + types.TypeMsgDelegate,
		"oteltest.tx.msg." + types.TypeMsgBeginRedelegate,
		"oteltest.tx.msg." + types.TypeMsgUndelegate,
	} {
		require.NotContains(t, keys, key, "staking message emitted legacy telemetry")
	}
}
