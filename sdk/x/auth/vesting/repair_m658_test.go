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
	vestingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/stretchr/testify/require"
)

func TestRepairM_658_ContinuousEndTime(t *testing.T) {
	oteltest.Install()
	a := app.Setup(t, false, false, false)
	root := a.NewContext(false, tmproto.Header{Height: a.LastBlockHeight() + 1, Time: time.Unix(1700000000, 123)})
	from := sdk.AccAddress([]byte("repair658_sender____"))
	to := sdk.AccAddress([]byte("repair658_recipient_"))
	amount := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100))
	for _, tc := range []struct {
		name   string
		offset int64
	}{
		{name: "past", offset: -1},
		{name: "equal", offset: 0},
		{name: "future", offset: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := root.CacheContext()
			a.AccountKeeper.SetAccount(ctx, a.AccountKeeper.NewAccountWithAddress(ctx, from))
			require.NoError(t, apptesting.FundAccount(a.BankKeeper, ctx, from, amount))
			require.Nil(t, a.AccountKeeper.GetAccount(ctx, to))
			accounts := repair301StoreSnapshot(ctx.KVStore(a.GetKey(authtypes.StoreKey)))
			bank := repair301StoreSnapshot(ctx.KVStore(a.GetKey(banktypes.StoreKey)))
			msg := vestingtypes.NewMsgCreateVestingAccount(from, to, amount, ctx.BlockTime().Unix()+tc.offset, false, nil)
			require.NoError(t, msg.ValidateBasic(), "all boundaries pass stateless message validation")
			result, err := repair301Dispatch(t, a, ctx, msg)
			if tc.offset <= 0 {
				require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
				require.Nil(t, result)
				require.Nil(t, a.AccountKeeper.GetAccount(ctx, to))
				require.Equal(t, amount, a.BankKeeper.GetAllBalances(ctx, from))
				require.True(t, a.BankKeeper.GetAllBalances(ctx, to).IsZero())
				require.Equal(t, accounts, repair301StoreSnapshot(ctx.KVStore(a.GetKey(authtypes.StoreKey))), "rejection must preserve accounts and the account number counter")
				require.Equal(t, bank, repair301StoreSnapshot(ctx.KVStore(a.GetKey(banktypes.StoreKey))), "rejection must preserve the complete bank store")
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			account := a.AccountKeeper.GetAccount(ctx, to)
			require.IsType(t, &vestingtypes.ContinuousVestingAccount{}, account)
			continuous := account.(*vestingtypes.ContinuousVestingAccount)
			require.NoError(t, continuous.Validate())
			require.Equal(t, ctx.BlockTime().Unix(), continuous.GetStartTime())
			require.Equal(t, msg.EndTime, continuous.GetEndTime())
			require.Equal(t, amount, continuous.GetVestingCoins(ctx.BlockTime()))
			require.Equal(t, amount, a.BankKeeper.GetAllBalances(ctx, to))
			require.True(t, a.BankKeeper.GetAllBalances(ctx, from).IsZero())
			require.True(t, a.BankKeeper.SpendableCoins(ctx, to).IsZero())
			require.True(t, continuous.GetVestingCoins(time.Unix(msg.EndTime, 0)).IsZero())
			require.Equal(t, amount, a.BankKeeper.SpendableCoins(ctx.WithBlockTime(time.Unix(msg.EndTime, 0)), to))
		})
	}
}

func TestRepairM_658_FailedFundingMetrics(t *testing.T) {
	testCreateVestingAccountFailedFundingMetrics(t)
}
