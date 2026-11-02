package keeper_test

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/types"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func setupSendEnabledCheckerTest(t *testing.T) (*keeper.BaseKeeper, sdk.Context, sdk.AccAddress, sdk.AccAddress) {
	t.Helper()
	accts := utils.NewTestAccounts(1)
	sdk.RegisterDenom(sdk.DefaultBondDenom, sdk.OneDec())
	wrapper := app.NewGigaTestWrapper(t, time.Now(), accts[0].PublicKey, true, false, func(ba *baseapp.BaseApp) {
		ba.SetOccEnabled(false)
		ba.SetConcurrencyWorkers(1)
	})
	ctx := wrapper.Ctx.WithMultiStore(wrapper.Ctx.MultiStore().CacheMultiStore())
	wrapper.App.AccountKeeper.SetParams(ctx, authtypes.DefaultParams())
	k := wrapper.App.GigaBankKeeper
	k.SetParams(ctx, banktypes.DefaultParams())
	from := sdk.AccAddress([]byte("checker_sender______"))
	to := sdk.AccAddress([]byte("checker_recipient___"))
	funds := sdk.NewCoins(sdk.NewInt64Coin("ufoo", 100), sdk.NewInt64Coin(sdk.MustGetBaseDenom(), 100))
	require.NoError(t, k.MintCoins(ctx, minttypes.ModuleName, funds))
	require.NoError(t, k.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, from, funds))
	require.NoError(t, k.AddWei(ctx, from, sdk.NewInt(100)))
	return k, ctx, from, to
}

func TestSendEnabledCheckerRefusesDisabledDenom(t *testing.T) {
	k, ctx, from, to := setupSendEnabledCheckerTest(t)
	sender := k.BaseSendKeeper
	var registrar keeper.SendKeeper = k
	registrar.RegisterSendEnabledChecker(func(sdk.Context, string) bool { return true })
	registrar.RegisterSendEnabledChecker(func(_ sdk.Context, denom string) bool { return denom != "ufoo" })
	registrar.RegisterSendEnabledChecker(func(sdk.Context, string) bool { return true })
	baseDenom := sdk.MustGetBaseDenom()
	require.False(t, sender.IsSendEnabledCoin(ctx, sdk.NewInt64Coin("ufoo", 1)))
	require.ErrorIs(t, sender.IsSendEnabledCoins(ctx, sdk.NewInt64Coin("ufoo", 1)), types.ErrSendDisabled)
	require.True(t, sender.IsSendEnabledCoin(ctx, sdk.NewInt64Coin(baseDenom, 1)))

	balances := func(addr sdk.AccAddress) []sdk.Coin {
		return []sdk.Coin{k.GetBalance(ctx, addr, "ufoo"), k.GetBalance(ctx, addr, baseDenom)}
	}
	fromBefore := balances(from)
	toBefore := balances(to)
	fromWeiBefore := k.GetWeiBalance(ctx, from)
	toWeiBefore := k.GetWeiBalance(ctx, to)
	eventsBefore := len(ctx.EventManager().Events())
	assertUnchanged := func(t *testing.T) {
		t.Helper()
		require.Equal(t, fromBefore, balances(from))
		require.Equal(t, toBefore, balances(to))
		require.Equal(t, fromWeiBefore, k.GetWeiBalance(ctx, from))
		require.Equal(t, toWeiBefore, k.GetWeiBalance(ctx, to))
		require.Len(t, ctx.EventManager().Events(), eventsBefore)
	}
	for _, coins := range []sdk.Coins{
		sdk.NewCoins(sdk.NewInt64Coin("ufoo", 1)),
		sdk.NewCoins(sdk.NewInt64Coin(baseDenom, 1), sdk.NewInt64Coin("ufoo", 1)),
	} {
		err := sender.SendCoins(ctx, from, to, coins)
		require.ErrorIs(t, err, types.ErrSendDisabled)
		require.Contains(t, err.Error(), "ufoo transfers are currently disabled")
		assertUnchanged(t)
	}

	registrar.RegisterSendEnabledChecker(func(_ sdk.Context, denom string) bool { return denom != baseDenom })
	require.False(t, sender.IsSendEnabledCoin(ctx, sdk.NewInt64Coin(baseDenom, 1)))
	require.ErrorIs(t, sender.SendCoins(ctx, from, to, sdk.NewCoins(sdk.NewInt64Coin(baseDenom, 1))), types.ErrSendDisabled)
	assertUnchanged(t)
	for _, tc := range []struct {
		name string
		amt  sdk.Int
		wei  sdk.Int
	}{
		{"coins", sdk.OneInt(), sdk.ZeroInt()},
		{"wei", sdk.ZeroInt(), sdk.OneInt()},
		{"coins_and_wei", sdk.OneInt(), sdk.OneInt()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := sender.SendCoinsAndWei(ctx, from, to, tc.amt, tc.wei)
			require.ErrorIs(t, err, types.ErrSendDisabled)
			require.Contains(t, err.Error(), baseDenom+" transfers are currently disabled")
			assertUnchanged(t)
		})
	}
}

func TestSendEnabledCheckerAllowsOtherDenoms(t *testing.T) {
	k, ctx, from, to := setupSendEnabledCheckerTest(t)
	k.RegisterSendEnabledChecker(func(_ sdk.Context, denom string) bool { return denom != "ufoo" })
	baseDenom := sdk.MustGetBaseDenom()
	for _, tc := range []struct {
		name   string
		params banktypes.Params
	}{
		{"default_enabled", banktypes.DefaultParams()},
		{"denom_disabled", banktypes.DefaultParams().SetSendEnabledParam(baseDenom, false)},
		{"default_disabled", banktypes.NewParams(false, nil)},
		{"denom_enabled", banktypes.NewParams(false, nil).SetSendEnabledParam(baseDenom, true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k.SetParams(ctx, tc.params)
			for _, denom := range []string{baseDenom, "ubar"} {
				coin := sdk.NewInt64Coin(denom, 1)
				enabled := tc.params.SendEnabledDenom(denom)
				require.Equal(t, enabled, k.IsSendEnabledCoin(ctx, coin))
				if enabled {
					require.NoError(t, k.IsSendEnabledCoins(ctx, coin))
				} else {
					require.ErrorIs(t, k.IsSendEnabledCoins(ctx, coin), types.ErrSendDisabled)
				}
			}
			require.False(t, k.IsSendEnabledCoin(ctx, sdk.NewInt64Coin("ufoo", 1)))
			if tc.params.SendEnabledDenom(baseDenom) {
				fromBefore := k.GetBalance(ctx, from, baseDenom).Amount
				toBefore := k.GetBalance(ctx, to, baseDenom).Amount
				require.NoError(t, k.SendCoins(ctx, from, to, sdk.NewCoins(sdk.NewInt64Coin(baseDenom, 3))))
				require.Equal(t, fromBefore.SubRaw(3), k.GetBalance(ctx, from, baseDenom).Amount)
				require.Equal(t, toBefore.AddRaw(3), k.GetBalance(ctx, to, baseDenom).Amount)
				fromWeiBefore := k.GetWeiBalance(ctx, from)
				toWeiBefore := k.GetWeiBalance(ctx, to)
				require.NoError(t, k.SendCoinsAndWei(ctx, from, to, sdk.OneInt(), sdk.OneInt()))
				require.Equal(t, fromBefore.SubRaw(4), k.GetBalance(ctx, from, baseDenom).Amount)
				require.Equal(t, toBefore.AddRaw(4), k.GetBalance(ctx, to, baseDenom).Amount)
				require.Equal(t, fromWeiBefore.SubRaw(1), k.GetWeiBalance(ctx, from))
				require.Equal(t, toWeiBefore.AddRaw(1), k.GetWeiBalance(ctx, to))
			}
		})
	}
}
