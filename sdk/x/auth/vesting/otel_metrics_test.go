package vesting_test

import (
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
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

func TestCreateVestingAccountFailedFundingRecordsNoMetrics(t *testing.T) {
	testCreateVestingAccountFailedFundingMetrics(t)
}

func testCreateVestingAccountFailedFundingMetrics(t *testing.T) {
	t.Helper()
	reader := oteltest.Install()
	a := app.Setup(t, false, false, false)
	root := a.NewContext(false, tmproto.Header{Height: a.LastBlockHeight() + 1, Time: time.Unix(1700000000, 0)})
	from := sdk.AccAddress([]byte("repair658_sender____"))
	to := sdk.AccAddress([]byte("repair658_recipient_"))
	amount := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100))
	tooMuch := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 101))
	for _, delayed := range []bool{false, true} {
		name := "continuous"
		if delayed {
			name = "delayed"
		}
		t.Run(name, func(t *testing.T) {
			ctx, _ := root.CacheContext()
			a.AccountKeeper.SetAccount(ctx, a.AccountKeeper.NewAccountWithAddress(ctx, from))
			require.NoError(t, apptesting.FundAccount(a.BankKeeper, ctx, from, amount))
			accounts := repair301StoreSnapshot(ctx.KVStore(a.GetKey(authtypes.StoreKey)))
			bank := repair301StoreSnapshot(ctx.KVStore(a.GetKey(banktypes.StoreKey)))
			beforeNewAccount := newAccount(reader)
			beforeAmount, beforeFound := accountAmount(reader)

			txCtx, _ := ctx.CacheContext()
			msg := types.NewMsgCreateVestingAccount(from, to, tooMuch, ctx.BlockTime().Unix()+10000, delayed, nil)
			require.NoError(t, msg.ValidateBasic())
			result, err := repair301Dispatch(t, a, txCtx, msg)
			require.ErrorIs(t, err, sdkerrors.ErrInsufficientFunds)
			require.Nil(t, result)
			require.NotNil(t, a.AccountKeeper.GetAccount(txCtx, to), "the real handler must reach bank funding after constructing the account")
			if delayed {
				require.IsType(t, &types.DelayedVestingAccount{}, a.AccountKeeper.GetAccount(txCtx, to))
			} else {
				require.IsType(t, &types.ContinuousVestingAccount{}, a.AccountKeeper.GetAccount(txCtx, to))
			}
			require.Nil(t, a.AccountKeeper.GetAccount(ctx, to), "discard the failed transaction's cache")
			require.Equal(t, accounts, repair301StoreSnapshot(ctx.KVStore(a.GetKey(authtypes.StoreKey))))
			require.Equal(t, bank, repair301StoreSnapshot(ctx.KVStore(a.GetKey(banktypes.StoreKey))))
			require.Equal(t, beforeNewAccount, newAccount(reader), "failed funding must not increment new_account")
			afterAmount, afterFound := accountAmount(reader)
			require.Equal(t, beforeFound, afterFound, "failed funding must not introduce an account_amount observation")
			require.Equal(t, beforeAmount, afterAmount, "failed funding must not replace the last funded amount")

			msg = types.NewMsgCreateVestingAccount(from, to, amount, ctx.BlockTime().Unix()+10000, delayed, nil)
			result, err = repair301Dispatch(t, a, ctx, msg)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, amount, a.BankKeeper.GetAllBalances(ctx, to))
			require.True(t, a.BankKeeper.GetAllBalances(ctx, from).IsZero())
			require.Equal(t, beforeNewAccount+1, newAccount(reader), "successful funding must increment new_account exactly once")
			afterAmount, afterFound = accountAmount(reader)
			require.True(t, afterFound)
			require.Equal(t, int64(100), afterAmount, "successful funding must record account_amount")
		})
	}
}
