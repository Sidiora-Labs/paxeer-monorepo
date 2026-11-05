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

func TestVestingEmitsNoLegacyTelemetry(t *testing.T) {
	reader := oteltest.Install()
	for _, tc := range []struct {
		name    string
		delayed bool
		amount  int64
	}{
		{name: "delayed", delayed: true, amount: 100},
		{name: "continuous", delayed: false, amount: 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := app.Setup(t, false, false, false)
			ctx := app.BaseApp.NewContext(false, tmproto.Header{Height: app.LastBlockHeight() + 1})
			from := sdk.AccAddress([]byte("addr1_______________"))
			to := sdk.AccAddress([]byte("addr2_______________"))
			amount := sdk.NewCoins(sdk.NewInt64Coin("uhpx", tc.amount))
			app.AccountKeeper.SetAccount(ctx, app.AccountKeeper.NewAccountWithAddress(ctx, from))
			require.NoError(t, apptesting.FundAccount(app.BankKeeper, ctx, from, amount))
			require.Nil(t, app.AccountKeeper.GetAccount(ctx, to))

			sink := oteltest.InstallLegacySink(t)
			require.Empty(t, sink.Keys())
			beforeNewAccount := newAccount(reader)
			beforeAmount, _ := accountAmount(reader)
			require.NotEqual(t, tc.amount, beforeAmount)
			server := vesting.NewMsgServerImpl(app.AccountKeeper, app.BankKeeper)
			msg := types.NewMsgCreateVestingAccount(from, to, amount, ctx.BlockTime().Unix()+10000, tc.delayed, nil)
			response, err := server.CreateVestingAccount(sdk.WrapSDKContext(ctx), msg)
			require.NoError(t, err)
			require.NotNil(t, response)

			account := app.AccountKeeper.GetAccount(ctx, to)
			if tc.delayed {
				require.IsType(t, &types.DelayedVestingAccount{}, account)
				require.Equal(t, amount, account.(*types.DelayedVestingAccount).GetVestingCoins(ctx.BlockTime()))
			} else {
				require.IsType(t, &types.ContinuousVestingAccount{}, account)
				require.Equal(t, amount, account.(*types.ContinuousVestingAccount).GetVestingCoins(ctx.BlockTime()))
			}
			require.Equal(t, amount, app.BankKeeper.GetAllBalances(ctx, to))
			keys := sink.Keys()
			require.NotContains(t, keys, "oteltest.new.account")
			require.NotContains(t, keys, "oteltest.tx.msg.create_vesting_account")
			require.Equal(t, int64(1), newAccount(reader)-beforeNewAccount)
			afterAmount, found := accountAmount(reader)
			require.True(t, found)
			require.Equal(t, tc.amount-beforeAmount, afterAmount-beforeAmount)
			require.Equal(t, tc.amount, afterAmount)
		})
	}
}
