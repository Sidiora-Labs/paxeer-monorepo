package keeper_test

import (
	"testing"

	gigabanktypes "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/stretchr/testify/require"
)

func TestDenomPauseBankPathConformance(t *testing.T) {
	s := denomPauseSuite(t)
	ctx := s.Ctx
	bank := s.App.BankKeeper
	tokenfactory := s.App.TokenFactoryKeeper
	bankMessages := bankkeeper.NewMsgServerImpl(bank)
	from, to := s.TestAccs[0], s.TestAccs[1]
	admin := from.String()
	issuerDid := adminsIssuerDid(0xab)
	module := s.App.AccountKeeper.GetModuleAddress(types.ModuleName)
	pausedDenom := s.defaultDenom

	response, err := s.msgServer.CreateDenom(sdk.WrapSDKContext(ctx), types.NewMsgCreateDenom(admin, "conformance-unpaused"))
	require.NoError(t, err)
	unpausedDenom := response.GetNewTokenDenom()
	require.NotEqual(t, pausedDenom, unpausedDenom)
	for _, denom := range []string{pausedDenom, unpausedDenom} {
		_, err := s.msgServer.Mint(sdk.WrapSDKContext(ctx), types.NewMsgMint(admin, sdk.NewInt64Coin(denom, 100)))
		require.NoError(t, err)
		require.NoError(t, tokenfactory.SetIssuerDid(ctx, denom, issuerDid))
	}
	require.NoError(t, tokenfactory.SetDenomPausedByIssuer(ctx, pausedDenom, issuerDid, true))
	require.True(t, tokenfactory.IsDenomPaused(ctx, pausedDenom))
	require.False(t, tokenfactory.IsDenomPaused(ctx, unpausedDenom))

	paths := []struct {
		name        string
		invoke      func(sdk.Context, sdk.Coin) error
		balance     func(sdk.Context, sdk.AccAddress, string) sdk.Coin
		pausedError error
		fromDelta   int64
		toDelta     int64
		supplyDelta int64
	}{
		{
			name: "MsgSend",
			invoke: func(ctx sdk.Context, coin sdk.Coin) error {
				_, err := bankMessages.Send(sdk.WrapSDKContext(ctx), banktypes.NewMsgSend(from, to, sdk.NewCoins(coin)))
				return err
			},
			balance: bank.GetBalance, pausedError: banktypes.ErrSendDisabled,
			fromDelta: -10, toDelta: 10,
		},
		{
			name: "MsgMultiSend",
			invoke: func(ctx sdk.Context, coin sdk.Coin) error {
				coins := sdk.NewCoins(coin)
				msg := banktypes.NewMsgMultiSend(
					[]banktypes.Input{banktypes.NewInput(from, coins)},
					[]banktypes.Output{banktypes.NewOutput(to, coins)},
				)
				_, err := bankMessages.MultiSend(sdk.WrapSDKContext(ctx), msg)
				return err
			},
			balance: bank.GetBalance, pausedError: banktypes.ErrSendDisabled,
			fromDelta: -10, toDelta: 10,
		},
		{
			name: "GigaBankKeeper.SendCoins",
			invoke: func(ctx sdk.Context, coin sdk.Coin) error {
				return s.App.GigaBankKeeper.SendCoins(ctx, from, to, sdk.NewCoins(coin))
			},
			balance: s.App.GigaBankKeeper.GetBalance, pausedError: gigabanktypes.ErrSendDisabled,
			fromDelta: -10, toDelta: 10,
		},
		{
			name: "Mint",
			invoke: func(ctx sdk.Context, coin sdk.Coin) error {
				_, err := s.msgServer.Mint(sdk.WrapSDKContext(ctx), types.NewMsgMint(admin, coin))
				return err
			},
			balance: bank.GetBalance, pausedError: types.ErrDenomPaused,
			fromDelta: 10, supplyDelta: 10,
		},
		{
			name: "Burn",
			invoke: func(ctx sdk.Context, coin sdk.Coin) error {
				_, err := s.msgServer.Burn(sdk.WrapSDKContext(ctx), types.NewMsgBurn(admin, coin))
				return err
			},
			balance: bank.GetBalance, pausedError: types.ErrDenomPaused,
			fromDelta: -10, supplyDelta: -10,
		},
		{
			name: "MintByIssuer",
			invoke: func(ctx sdk.Context, coin sdk.Coin) error {
				return tokenfactory.MintByIssuer(ctx, issuerDid, coin, to.String())
			},
			balance: bank.GetBalance, pausedError: types.ErrDenomPaused,
			toDelta: 10, supplyDelta: 10,
		},
	}
	require.Len(t, paths, 6)
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			for _, testCase := range []struct {
				name   string
				denom  string
				paused bool
			}{
				{"paused", pausedDenom, true},
				{"unpaused", unpausedDenom, false},
			} {
				t.Run(testCase.name, func(t *testing.T) {
					callCtx := ctx.WithEventManager(sdk.NewEventManager())
					coin := sdk.NewInt64Coin(testCase.denom, 10)
					fromBefore := path.balance(callCtx, from, coin.Denom)
					toBefore := path.balance(callCtx, to, coin.Denom)
					moduleBefore := path.balance(callCtx, module, coin.Denom)
					supplyBefore := bank.GetSupply(callCtx, coin.Denom)
					require.True(t, fromBefore.Amount.GTE(coin.Amount), "sender must have funds before testing a refusal")
					require.Equal(t, testCase.paused, tokenfactory.IsDenomPaused(callCtx, coin.Denom))

					err := path.invoke(callCtx, coin)
					if testCase.paused {
						require.ErrorIs(t, err, path.pausedError)
						require.Equal(t, fromBefore, path.balance(callCtx, from, coin.Denom))
						require.Equal(t, toBefore, path.balance(callCtx, to, coin.Denom))
						require.Equal(t, supplyBefore, bank.GetSupply(callCtx, coin.Denom))
						require.Empty(t, callCtx.EventManager().Events())
					} else {
						require.NoError(t, err)
						require.Equal(t, fromBefore.Amount.AddRaw(path.fromDelta), path.balance(callCtx, from, coin.Denom).Amount)
						require.Equal(t, toBefore.Amount.AddRaw(path.toDelta), path.balance(callCtx, to, coin.Denom).Amount)
						require.Equal(t, supplyBefore.Amount.AddRaw(path.supplyDelta), bank.GetSupply(callCtx, coin.Denom).Amount)
					}
					require.Equal(t, moduleBefore, path.balance(callCtx, module, coin.Denom))
					require.True(t, tokenfactory.IsDenomPaused(callCtx, pausedDenom))
					require.False(t, tokenfactory.IsDenomPaused(callCtx, unpausedDenom))
				})
			}
		})
	}

	// Task 6.25.2's decision.623 boundary: keeper-level x/bank SendCoins
	// serves module flows and stays outside the pause-gated user-path table.
	t.Run("module_flow_SendCoins_outside_user_paths", func(t *testing.T) {
		for _, path := range paths {
			require.NotEqual(t, "BankKeeper.SendCoins", path.name)
		}
		coin := sdk.NewInt64Coin(pausedDenom, 10)
		fromBefore := bank.GetBalance(ctx, from, pausedDenom)
		toBefore := bank.GetBalance(ctx, to, pausedDenom)
		supplyBefore := bank.GetSupply(ctx, pausedDenom)
		require.True(t, tokenfactory.IsDenomPaused(ctx, pausedDenom))
		require.ErrorIs(t, bank.IsSendEnabledCoins(ctx, coin), banktypes.ErrSendDisabled)
		require.NoError(t, bank.SendCoins(ctx, from, to, sdk.NewCoins(coin)))
		require.Equal(t, fromBefore.Sub(coin), bank.GetBalance(ctx, from, pausedDenom))
		require.Equal(t, toBefore.Add(coin), bank.GetBalance(ctx, to, pausedDenom))
		require.Equal(t, supplyBefore, bank.GetSupply(ctx, pausedDenom))
		require.True(t, tokenfactory.IsDenomPaused(ctx, pausedDenom))
	})
}
