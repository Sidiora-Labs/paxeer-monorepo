package vesting_test

import (
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/types"
	"github.com/stretchr/testify/require"
)

const vestingMeterName = "paxcosmos_x_auth_vesting"

func newAccount(reader *oteltest.Reader) int64 {
	return reader.CounterSum(vestingMeterName, "new_account")
}

func accountAmount(reader *oteltest.Reader) (int64, bool) {
	return reader.GaugeInt(vestingMeterName, "account_amount")
}

func TestCreateVestingAccountRecordsNewAccountAndAmount(t *testing.T) {
	// Bind the package instruments before app setup installs its metrics provider.
	reader := oteltest.Install()
	app := app.Setup(t, false, false, false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{Height: app.LastBlockHeight() + 1})
	from := sdk.AccAddress([]byte("addr1_______________"))
	to := sdk.AccAddress([]byte("addr2_______________"))
	amount := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100))
	app.AccountKeeper.SetAccount(ctx, app.AccountKeeper.NewAccountWithAddress(ctx, from))
	require.NoError(t, apptesting.FundAccount(app.BankKeeper, ctx, from, amount))
	require.Nil(t, app.AccountKeeper.GetAccount(ctx, to))

	beforeNewAccount := newAccount(reader)
	beforeAmount, found := accountAmount(reader)
	require.False(t, found, "account_amount must have no observation before account creation")
	require.Zero(t, beforeAmount)

	server := vesting.NewMsgServerImpl(app.AccountKeeper, app.BankKeeper)
	msg := types.NewMsgCreateVestingAccount(from, to, amount, ctx.BlockTime().Unix()+10000, true, nil)
	response, err := server.CreateVestingAccount(sdk.WrapSDKContext(ctx), msg)
	require.NoError(t, err)
	require.NotNil(t, response)

	account := app.AccountKeeper.GetAccount(ctx, to)
	require.IsType(t, &types.DelayedVestingAccount{}, account)
	require.Equal(t, amount, account.(*types.DelayedVestingAccount).GetVestingCoins(ctx.BlockTime()))
	require.Equal(t, amount, app.BankKeeper.GetAllBalances(ctx, to))

	require.Equal(t, int64(1), newAccount(reader)-beforeNewAccount)
	afterAmount, found := accountAmount(reader)
	require.True(t, found, "account_amount must record the new vesting account's funding")
	require.Equal(t, int64(100), afterAmount-beforeAmount)
	require.Equal(t, int64(100), afterAmount)
}
