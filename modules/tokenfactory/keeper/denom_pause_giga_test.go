package keeper_test

import (
	"os"
	"os/exec"
	"testing"

	gigabankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/keeper"
	gigabanktypes "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

type SendEnabledChecker = gigabankkeeper.SendEnabledChecker

func TestDenomPauseGigaBankRefuses(t *testing.T) {
	const childEnv = "PAXEER_TEST_DENOM_PAUSE_GIGA_CHILD"
	if os.Getenv(childEnv) != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestDenomPauseGigaBankRefuses$", "-test.count=1")
		cmd.Env = append(os.Environ(), childEnv+"=1")
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", output)
		return
	}

	s := denomPauseSuite(t)
	k := s.App.GigaBankKeeper
	from, to := s.TestAccs[0], s.TestAccs[1]
	admin := from.String()
	require.NoError(t, denomPauseMint(s, 100))
	response, err := s.msgServer.CreateDenom(sdk.WrapSDKContext(s.Ctx), types.NewMsgCreateDenom(admin, "giga-unpaused"))
	require.NoError(t, err)
	otherDenom := response.GetNewTokenDenom()
	_, err = s.msgServer.Mint(sdk.WrapSDKContext(s.Ctx), types.NewMsgMint(admin, sdk.NewInt64Coin(otherDenom, 100)))
	require.NoError(t, err)

	baseUnit, found := sdk.GetDenomUnit(sdk.MustGetBaseDenom())
	require.True(t, found)
	unit := baseUnit.QuoInt64(2)
	require.True(t, unit.IsPositive())
	require.NoError(t, sdk.RegisterDenom(s.defaultDenom, unit))
	require.Equal(t, s.defaultDenom, sdk.MustGetBaseDenom())
	require.NoError(t, k.AddWei(s.Ctx, from, sdk.NewInt(100)))

	coin := sdk.NewInt64Coin(s.defaultDenom, 10)
	otherCoin := sdk.NewInt64Coin(otherDenom, 3)
	var checker SendEnabledChecker = s.App.TokenFactoryKeeper.SendEnabledChecker
	require.True(t, checker(s.Ctx, s.defaultDenom))
	require.True(t, k.IsSendEnabledCoin(s.Ctx, coin))
	require.Equal(t, sdk.NewInt64Coin(s.defaultDenom, 100), k.GetBalance(s.Ctx, from, s.defaultDenom))
	require.Equal(t, sdk.NewInt64Coin(s.defaultDenom, 0), k.GetBalance(s.Ctx, to, s.defaultDenom))

	require.NoError(t, denomPauseSet(s, admin, true))
	require.False(t, checker(s.Ctx, s.defaultDenom))
	require.False(t, k.IsSendEnabledCoin(s.Ctx, coin))
	require.ErrorIs(t, k.IsSendEnabledCoins(s.Ctx, coin), gigabanktypes.ErrSendDisabled)
	require.True(t, k.IsSendEnabledCoin(s.Ctx, otherCoin))

	balances := func(addr sdk.AccAddress) []sdk.Coin {
		return []sdk.Coin{k.GetBalance(s.Ctx, addr, s.defaultDenom), k.GetBalance(s.Ctx, addr, otherDenom)}
	}
	fromBefore, toBefore := balances(from), balances(to)
	fromWeiBefore, toWeiBefore := k.GetWeiBalance(s.Ctx, from), k.GetWeiBalance(s.Ctx, to)
	eventsBefore := len(s.Ctx.EventManager().Events())
	assertUnchanged := func(t *testing.T) {
		t.Helper()
		require.Equal(t, fromBefore, balances(from))
		require.Equal(t, toBefore, balances(to))
		require.Equal(t, fromWeiBefore, k.GetWeiBalance(s.Ctx, from))
		require.Equal(t, toWeiBefore, k.GetWeiBalance(s.Ctx, to))
		require.Len(t, s.Ctx.EventManager().Events(), eventsBefore)
	}
	for _, coins := range []sdk.Coins{sdk.NewCoins(coin), sdk.NewCoins(coin, otherCoin)} {
		err := k.SendCoins(s.Ctx, from, to, coins)
		require.ErrorIs(t, err, gigabanktypes.ErrSendDisabled)
		require.Contains(t, err.Error(), s.defaultDenom+" transfers are currently disabled")
		assertUnchanged(t)
	}
	transfers := []struct {
		name   string
		amount sdk.Int
		wei    sdk.Int
	}{
		{"coins", sdk.NewInt(10), sdk.ZeroInt()},
		{"wei", sdk.ZeroInt(), sdk.NewInt(7)},
		{"coins_and_wei", sdk.NewInt(10), sdk.NewInt(7)},
	}
	for _, transfer := range transfers {
		t.Run("paused_"+transfer.name, func(t *testing.T) {
			err := k.SendCoinsAndWei(s.Ctx, from, to, transfer.amount, transfer.wei)
			require.ErrorIs(t, err, gigabanktypes.ErrSendDisabled)
			require.Contains(t, err.Error(), s.defaultDenom+" transfers are currently disabled")
			assertUnchanged(t)
		})
	}

	require.NoError(t, k.SendCoins(s.Ctx, from, to, sdk.NewCoins(otherCoin)))
	require.Equal(t, sdk.NewInt64Coin(otherDenom, 97), k.GetBalance(s.Ctx, from, otherDenom))
	require.Equal(t, otherCoin, k.GetBalance(s.Ctx, to, otherDenom))
	require.Equal(t, fromBefore[0], k.GetBalance(s.Ctx, from, s.defaultDenom))
	require.Equal(t, toBefore[0], k.GetBalance(s.Ctx, to, s.defaultDenom))
	require.Equal(t, fromWeiBefore, k.GetWeiBalance(s.Ctx, from))
	require.Equal(t, toWeiBefore, k.GetWeiBalance(s.Ctx, to))
	require.False(t, k.IsSendEnabledCoin(s.Ctx, coin))

	require.NoError(t, denomPauseSet(s, admin, false))
	require.True(t, checker(s.Ctx, s.defaultDenom))
	require.True(t, k.IsSendEnabledCoin(s.Ctx, coin))
	require.NoError(t, k.IsSendEnabledCoins(s.Ctx, coin))
	require.NoError(t, k.SendCoins(s.Ctx, from, to, sdk.NewCoins(coin)))
	require.Equal(t, sdk.NewInt64Coin(s.defaultDenom, 90), k.GetBalance(s.Ctx, from, s.defaultDenom))
	require.Equal(t, coin, k.GetBalance(s.Ctx, to, s.defaultDenom))
	for _, transfer := range transfers {
		t.Run("unpaused_"+transfer.name, func(t *testing.T) {
			fromBalance := k.GetBalance(s.Ctx, from, s.defaultDenom).Amount
			toBalance := k.GetBalance(s.Ctx, to, s.defaultDenom).Amount
			fromWei := k.GetWeiBalance(s.Ctx, from)
			toWei := k.GetWeiBalance(s.Ctx, to)
			require.NoError(t, k.SendCoinsAndWei(s.Ctx, from, to, transfer.amount, transfer.wei))
			require.Equal(t, fromBalance.Sub(transfer.amount), k.GetBalance(s.Ctx, from, s.defaultDenom).Amount)
			require.Equal(t, toBalance.Add(transfer.amount), k.GetBalance(s.Ctx, to, s.defaultDenom).Amount)
			require.Equal(t, fromWei.Sub(transfer.wei), k.GetWeiBalance(s.Ctx, from))
			require.Equal(t, toWei.Add(transfer.wei), k.GetWeiBalance(s.Ctx, to))
		})
	}
	require.True(t, k.IsSendEnabledCoin(s.Ctx, otherCoin))
	require.NoError(t, k.SendCoins(s.Ctx, from, to, sdk.NewCoins(otherCoin)))
	require.Equal(t, sdk.NewInt64Coin(otherDenom, 94), k.GetBalance(s.Ctx, from, otherDenom))
	require.Equal(t, sdk.NewInt64Coin(otherDenom, 6), k.GetBalance(s.Ctx, to, otherDenom))
}
