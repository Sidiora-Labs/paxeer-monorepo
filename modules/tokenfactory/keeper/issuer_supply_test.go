package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/stretchr/testify/require"
)

func requireIssuerSupplyEvent(t *testing.T, ctx sdk.Context, eventType, addressKey, address string, amount sdk.Coin, bankEvents ...sdk.Event) {
	t.Helper()
	var events sdk.Events
	for _, event := range ctx.EventManager().Events() {
		if event.Type == eventType {
			events = append(events, event)
		}
	}
	expected := append(sdk.Events(bankEvents), sdk.NewEvent(
		eventType,
		sdk.NewAttribute(addressKey, address),
		sdk.NewAttribute(types.AttributeAmount, amount.String()),
	))
	require.Equal(t, expected, events)
}

func TestMintByIssuerRespectsPause(t *testing.T) {
	s := denomPauseSuite(t)
	k := s.App.TokenFactoryKeeper
	bank := s.App.BankKeeper
	issuerDid := adminsIssuerDid(0xab)
	otherDid := adminsIssuerDid(0xcd)
	to := s.TestAccs[1]
	amount := sdk.NewInt64Coin(s.defaultDenom, 50)
	moduleAddress := s.App.AccountKeeper.GetModuleAddress(types.ModuleName)
	ctx := s.Ctx.WithEventManager(sdk.NewEventManager())
	assertSupply := func(expected int64) {
		t.Helper()
		require.Equal(t, expected, bank.GetBalance(ctx, to, amount.Denom).Amount.Int64())
		require.Equal(t, expected, bank.GetSupply(ctx, amount.Denom).Amount.Int64())
		require.True(t, bank.GetBalance(ctx, moduleAddress, amount.Denom).IsZero())
	}

	for _, callerDid := range []string{"", issuerDid} {
		require.ErrorIs(t, k.MintByIssuer(ctx, callerDid, amount, to.String()), types.ErrIssuerMismatch)
		assertSupply(0)
		require.Empty(t, ctx.EventManager().Events())
	}
	require.NoError(t, k.SetIssuerDid(ctx, amount.Denom, issuerDid))
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	for _, callerDid := range []string{"", otherDid, s.TestAccs[0].String()} {
		require.ErrorIs(t, k.MintByIssuer(ctx, callerDid, amount, to.String()), types.ErrIssuerMismatch)
		assertSupply(0)
		require.Empty(t, ctx.EventManager().Events())
	}

	require.NoError(t, k.MintByIssuer(ctx, issuerDid, amount, to.String()))
	assertSupply(50)
	requireIssuerSupplyEvent(t, ctx, types.TypeMsgMint, types.AttributeMintToAddress, to.String(), amount)

	require.NoError(t, k.SetDenomPausedByIssuer(ctx, amount.Denom, issuerDid, true))
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	require.ErrorIs(t, k.MintByIssuer(ctx, issuerDid, amount, to.String()), types.ErrDenomPaused)
	assertSupply(50)
	require.Empty(t, ctx.EventManager().Events())
	require.ErrorIs(t, k.MintByIssuer(ctx, otherDid, amount, to.String()), types.ErrIssuerMismatch)
	assertSupply(50)
	require.Empty(t, ctx.EventManager().Events())

	require.NoError(t, k.SetDenomPausedByIssuer(ctx, amount.Denom, issuerDid, false))
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.MintByIssuer(ctx, issuerDid, amount, to.String()))
	assertSupply(100)
	requireIssuerSupplyEvent(t, ctx, types.TypeMsgMint, types.AttributeMintToAddress, to.String(), amount)
}

func TestBurnByIssuerRefusesOtherDid(t *testing.T) {
	s := denomPauseSuite(t)
	k := s.App.TokenFactoryKeeper
	bank := s.App.BankKeeper
	issuerDid := adminsIssuerDid(0xab)
	otherDid := adminsIssuerDid(0xcd)
	from := s.TestAccs[1]
	amount := sdk.NewInt64Coin(s.defaultDenom, 20)
	moduleAddress := s.App.AccountKeeper.GetModuleAddress(types.ModuleName)
	ctx := s.Ctx.WithEventManager(sdk.NewEventManager())
	assertSupply := func(expected int64) {
		t.Helper()
		require.Equal(t, expected, bank.GetBalance(ctx, from, amount.Denom).Amount.Int64())
		require.Equal(t, expected, bank.GetSupply(ctx, amount.Denom).Amount.Int64())
		require.True(t, bank.GetBalance(ctx, moduleAddress, amount.Denom).IsZero())
	}

	for _, callerDid := range []string{"", issuerDid} {
		require.ErrorIs(t, k.BurnByIssuer(ctx, callerDid, amount, from.String()), types.ErrIssuerMismatch)
		assertSupply(0)
		require.Empty(t, ctx.EventManager().Events())
	}
	require.NoError(t, k.SetIssuerDid(ctx, amount.Denom, issuerDid))
	require.NoError(t, k.MintByIssuer(ctx, issuerDid, sdk.NewInt64Coin(amount.Denom, 100), from.String()))
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	for _, callerDid := range []string{otherDid, "", s.TestAccs[0].String()} {
		require.ErrorIs(t, k.BurnByIssuer(ctx, callerDid, amount, from.String()), types.ErrIssuerMismatch)
		assertSupply(100)
		require.Empty(t, ctx.EventManager().Events())
	}

	require.NoError(t, k.SetDenomPausedByIssuer(ctx, amount.Denom, issuerDid, true))
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	require.ErrorIs(t, k.BurnByIssuer(ctx, otherDid, amount, from.String()), types.ErrIssuerMismatch)
	assertSupply(100)
	require.Empty(t, ctx.EventManager().Events())
	require.ErrorIs(t, k.BurnByIssuer(ctx, issuerDid, amount, from.String()), types.ErrDenomPaused)
	assertSupply(100)
	require.Empty(t, ctx.EventManager().Events())

	require.NoError(t, k.SetDenomPausedByIssuer(ctx, amount.Denom, issuerDid, false))
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BurnByIssuer(ctx, issuerDid, amount, from.String()))
	assertSupply(80)
	requireIssuerSupplyEvent(t, ctx, types.TypeMsgBurn, types.AttributeBurnFromAddress, from.String(), amount,
		sdk.NewEvent(
			banktypes.EventTypeCoinBurn,
			sdk.NewAttribute(banktypes.AttributeKeyBurner, moduleAddress.String()),
			sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
		))

	ctx = ctx.WithEventManager(sdk.NewEventManager())
	require.Error(t, k.BurnByIssuer(ctx, issuerDid, sdk.NewInt64Coin(amount.Denom, 81), from.String()))
	assertSupply(80)
	require.Empty(t, ctx.EventManager().Events())
}
