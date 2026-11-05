package keeper_test

import (
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/stretchr/testify/require"
)

const bankKeeperMeterName = "paxcosmos_x_bank_keeper"

func newAccountMetricsFixture(t *testing.T) (*IntegrationTestSuite, sdk.AccAddress) {
	t.Helper()
	suite := new(IntegrationTestSuite)
	suite.SetT(t)
	suite.SetupTest()
	sender := sdk.AccAddress("addr1_______________")
	ak := suite.app.AccountKeeper
	ak.SetAccount(suite.ctx, ak.NewAccountWithAddress(suite.ctx, sender))
	require.NoError(t, apptesting.FundAccount(suite.app.BankKeeper, suite.ctx, sender, sdk.NewCoins(newFooCoin(100))))
	return suite, sender
}

func TestSendCoinsToNewAccountRecordsNewAccount(t *testing.T) {
	reader := oteltest.Install()
	suite, sender := newAccountMetricsFixture(t)
	ctx, bk, ak := suite.ctx, suite.app.BankKeeper, suite.app.AccountKeeper
	recipient := sdk.AccAddress("addr2_______________")
	amount := sdk.NewCoins(newFooCoin(10))
	before := reader.CounterSum(bankKeeperMeterName, "new_account")

	require.False(t, ak.HasAccount(ctx, recipient))
	require.NoError(t, bk.SendCoins(ctx, sender, recipient, amount))
	require.True(t, ak.HasAccount(ctx, recipient))
	require.Equal(t, newFooCoin(10), bk.GetBalance(ctx, recipient, fooDenom))
	require.Equal(t, before+1, reader.CounterSum(bankKeeperMeterName, "new_account"))

	require.NoError(t, bk.SendCoins(ctx, sender, recipient, amount))
	require.Equal(t, newFooCoin(20), bk.GetBalance(ctx, recipient, fooDenom))
	require.Equal(t, before+1, reader.CounterSum(bankKeeperMeterName, "new_account"))

	unfundedRecipient := sdk.AccAddress("addr3_______________")
	require.Error(t, bk.SendCoins(ctx, sender, unfundedRecipient, sdk.NewCoins(newFooCoin(100))))
	require.False(t, ak.HasAccount(ctx, unfundedRecipient))
	require.Equal(t, before+1, reader.CounterSum(bankKeeperMeterName, "new_account"))
}

func TestInputOutputCoinsToNewAccountRecordsNewAccount(t *testing.T) {
	reader := oteltest.Install()
	suite, sender := newAccountMetricsFixture(t)
	ctx, bk, ak := suite.ctx, suite.app.BankKeeper, suite.app.AccountKeeper
	first := sdk.AccAddress("addr2_______________")
	second := sdk.AccAddress("addr3_______________")
	inputs := []types.Input{{Address: sender.String(), Coins: sdk.NewCoins(newFooCoin(40))}}
	outputs := []types.Output{
		{Address: first.String(), Coins: sdk.NewCoins(newFooCoin(10))},
		{Address: second.String(), Coins: sdk.NewCoins(newFooCoin(10))},
		{Address: sender.String(), Coins: sdk.NewCoins(newFooCoin(10))},
		{Address: first.String(), Coins: sdk.NewCoins(newFooCoin(10))},
	}
	before := reader.CounterSum(bankKeeperMeterName, "new_account")

	require.False(t, ak.HasAccount(ctx, first))
	require.False(t, ak.HasAccount(ctx, second))
	require.NoError(t, bk.InputOutputCoins(ctx, inputs, outputs))
	require.True(t, ak.HasAccount(ctx, first))
	require.True(t, ak.HasAccount(ctx, second))
	require.Equal(t, newFooCoin(20), bk.GetBalance(ctx, first, fooDenom))
	require.Equal(t, newFooCoin(10), bk.GetBalance(ctx, second, fooDenom))
	require.Equal(t, newFooCoin(70), bk.GetBalance(ctx, sender, fooDenom))
	require.Equal(t, before+2, reader.CounterSum(bankKeeperMeterName, "new_account"))

	require.NoError(t, bk.InputOutputCoins(ctx, inputs, outputs))
	require.Equal(t, newFooCoin(40), bk.GetBalance(ctx, first, fooDenom))
	require.Equal(t, newFooCoin(20), bk.GetBalance(ctx, second, fooDenom))
	require.Equal(t, before+2, reader.CounterSum(bankKeeperMeterName, "new_account"))

	unfundedRecipient := sdk.AccAddress("addr4_______________")
	require.Error(t, bk.InputOutputCoins(ctx,
		[]types.Input{{Address: sender.String(), Coins: sdk.NewCoins(newFooCoin(100))}},
		[]types.Output{{Address: unfundedRecipient.String(), Coins: sdk.NewCoins(newFooCoin(100))}},
	))
	require.False(t, ak.HasAccount(ctx, unfundedRecipient))
	require.Equal(t, before+2, reader.CounterSum(bankKeeperMeterName, "new_account"))
}

func TestSendCoinsToNewAccountEmitsNoLegacyNewAccount(t *testing.T) {
	reader := oteltest.Install()
	suite, sender := newAccountMetricsFixture(t)
	ctx, bk, ak := suite.ctx, suite.app.BankKeeper, suite.app.AccountKeeper
	sink := oteltest.InstallLegacySink(t)
	telemetry.IncrCounter(1, "bank_test", "sink_check")
	require.Contains(t, sink.Keys(), "oteltest.bank_test.sink_check")
	before := reader.CounterSum(bankKeeperMeterName, "new_account")
	first := sdk.AccAddress("addr2_______________")
	second := sdk.AccAddress("addr3_______________")
	amount := sdk.NewCoins(newFooCoin(10))

	require.False(t, ak.HasAccount(ctx, first))
	require.NoError(t, bk.SendCoins(ctx, sender, first, amount))
	require.True(t, ak.HasAccount(ctx, first))
	require.Equal(t, before+1, reader.CounterSum(bankKeeperMeterName, "new_account"))
	for _, key := range sink.Keys() {
		require.NotEqual(t, "new.account", key)
		require.False(t, strings.HasSuffix(key, ".new.account"), "legacy counter emitted: %s", key)
	}

	require.False(t, ak.HasAccount(ctx, second))
	require.NoError(t, bk.InputOutputCoins(ctx,
		[]types.Input{{Address: sender.String(), Coins: amount}},
		[]types.Output{{Address: second.String(), Coins: amount}},
	))
	require.True(t, ak.HasAccount(ctx, second))
	require.Equal(t, before+2, reader.CounterSum(bankKeeperMeterName, "new_account"))
	for _, key := range sink.Keys() {
		require.NotEqual(t, "new.account", key)
		require.False(t, strings.HasSuffix(key, ".new.account"), "legacy counter emitted: %s", key)
	}
}
