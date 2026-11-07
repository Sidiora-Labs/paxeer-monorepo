// Proves: keeper.Keeper.SetIssuerDid/MintByIssuer/BurnByIssuer/SetDenomPausedByIssuer -> types.IssuerKeeper and the app gRPC query router -> adminsRequireIssuerDid and bank/query consumers.
//
// The proof uses the real app, authority protobuf, bank keeper and registered
// query service. ExportGenesis, app.Close and InitGenesis stop and reopen the
// consumer with producer-emitted state; each observation makes two fresh calls.
// Repairs: admins.go GetAuthorityMetadata accepted a decoded but invalid DID;
// issuer.go and issuer_supply.go now share validation of the stored authority.
// issuer_supply.go MintByIssuer also rejects malformed recipients before mintTo
// (bankactions.go) can mint coins and then return an address-decoding error.
package keeper_test

import (
	"bytes"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/stretchr/testify/require"
)

func TestProofIssuerDid(t *testing.T) {
	s := adminsIssuerSuite(t)
	closed := false
	t.Cleanup(func() {
		if !closed {
			require.NoError(t, s.App.Close())
		}
	})
	var issuer types.IssuerKeeper = s.App.TokenFactoryKeeper
	denom := s.defaultDenom
	admin := s.TestAccs[0].String()
	recipient := s.TestAccs[1]
	did := adminsIssuerDid(0xab)
	other := adminsIssuerDid(0xcd)
	coin := func(amount int64) sdk.Coin { return sdk.NewInt64Coin(denom, amount) }

	observe := func(expected int64, paused bool) {
		t.Helper()
		for read := 0; read < 2; read++ {
			adminsRequireIssuerDid(s, issuer, admin, did)
			require.Equal(t, paused, issuer.IsDenomPaused(s.Ctx, denom))
			state, err := s.queryClient.DenomPaused(s.Ctx.Context(), &types.QueryDenomPausedRequest{Denom: denom})
			require.NoError(t, err)
			require.Equal(t, &types.QueryDenomPausedResponse{Paused: paused, IssuerDid: did}, state)
			require.Equal(t, coin(expected), s.App.BankKeeper.GetBalance(s.Ctx, recipient, denom))
			require.Equal(t, coin(expected), s.App.BankKeeper.GetSupply(s.Ctx, denom))
			module := s.App.AccountKeeper.GetModuleAddress(types.ModuleName)
			require.True(t, s.App.BankKeeper.GetBalance(s.Ctx, module, denom).IsZero())
		}
	}

	s.Ctx = s.Ctx.WithEventManager(sdk.NewEventManager())
	require.NoError(t, issuer.SetIssuerDid(s.Ctx, denom, did))
	require.Equal(t, sdk.Events{sdk.NewEvent(types.EventTypeSetIssuerDid,
		sdk.NewAttribute(types.AttributeDenom, denom),
		sdk.NewAttribute(types.AttributeIssuerDid, did))}, s.Ctx.EventManager().Events())
	s.Ctx = s.Ctx.WithEventManager(sdk.NewEventManager())
	require.NoError(t, issuer.SetIssuerDid(s.Ctx, denom, did))
	require.Empty(t, s.Ctx.EventManager().Events())
	observe(0, false)
	require.ErrorIs(t, issuer.SetIssuerDid(s.Ctx, denom, other), types.ErrInvalidAuthorityMetadata)
	require.ErrorIs(t, issuer.SetIssuerDid(s.Ctx, denom+"-missing", did), types.ErrDenomDoesNotExist)

	require.NoError(t, issuer.MintByIssuer(s.Ctx, did, coin(50), recipient.String()))
	observe(50, false)
	require.NoError(t, issuer.BurnByIssuer(s.Ctx, did, coin(20), recipient.String()))
	observe(30, false)
	s.Ctx = s.Ctx.WithEventManager(sdk.NewEventManager())
	require.ErrorIs(t, issuer.MintByIssuer(s.Ctx, other, coin(10), recipient.String()), types.ErrIssuerMismatch)
	require.ErrorIs(t, issuer.BurnByIssuer(s.Ctx, other, coin(10), recipient.String()), types.ErrIssuerMismatch)
	require.ErrorIs(t, issuer.SetDenomPausedByIssuer(s.Ctx, denom, other, true), types.ErrIssuerMismatch)
	require.Error(t, issuer.MintByIssuer(s.Ctx, did, coin(10), "invalid-recipient"))
	require.Empty(t, s.Ctx.EventManager().Events())
	observe(30, false)
	require.NoError(t, issuer.SetDenomPausedByIssuer(s.Ctx, denom, did, true))
	require.Equal(t, sdk.Events{sdk.NewEvent(types.TypeMsgSetDenomPaused,
		sdk.NewAttribute(types.AttributeDenom, denom), sdk.NewAttribute(types.AttributePaused, "true"),
		sdk.NewAttribute(types.AttributeIssuerDid, did))}, s.Ctx.EventManager().Events())
	observe(30, true)

	exported := ExportGenesis(t, &s.KeeperTestHelper)
	header := s.Ctx.BlockHeader()
	require.NoError(t, s.App.Close())
	closed = true
	restored := InitGenesis(t, exported, header)
	s.App = restored.App
	s.Ctx = restored.Ctx
	s.QueryHelper = &baseapp.QueryServiceTestHelper{GRPCQueryRouter: s.App.GRPCQueryRouter(), Ctx: s.Ctx}
	s.queryClient = types.NewQueryClient(s.QueryHelper)
	s.msgServer = keeper.NewMsgServerImpl(s.App.TokenFactoryKeeper)
	issuer = s.App.TokenFactoryKeeper
	observe(30, true)
	s.Ctx = s.Ctx.WithEventManager(sdk.NewEventManager())
	require.ErrorIs(t, issuer.MintByIssuer(s.Ctx, did, coin(10), recipient.String()), types.ErrDenomPaused)
	require.ErrorIs(t, issuer.BurnByIssuer(s.Ctx, did, coin(10), recipient.String()), types.ErrDenomPaused)
	require.ErrorIs(t, issuer.SetDenomPausedByIssuer(s.Ctx, denom, did, true), types.ErrPauseUnchanged)
	require.Empty(t, s.Ctx.EventManager().Events())
	observe(30, true)

	for round := 0; round < 2; round++ {
		require.NoError(t, issuer.SetDenomPausedByIssuer(s.Ctx, denom, did, false))
		require.NoError(t, issuer.MintByIssuer(s.Ctx, did, coin(10), recipient.String()))
		require.NoError(t, issuer.BurnByIssuer(s.Ctx, did, coin(5), recipient.String()))
		observe(35+int64(round)*5, false)
		require.NoError(t, issuer.SetDenomPausedByIssuer(s.Ctx, denom, did, true))
		observe(35+int64(round)*5, true)
	}

	// Corrupt bytes actually emitted by SetIssuerDid, rather than substituting
	// a transport or manufacturing an authority message for the happy path.
	store := s.App.TokenFactoryKeeper.GetDenomPrefixStore(s.Ctx, denom)
	key := []byte(types.DenomAuthorityMetadataKey)
	canonical := bytes.Clone(store.Get(key))
	position := bytes.Index(canonical, []byte(did))
	require.NotEqual(t, -1, position)
	malformed := bytes.Clone(canonical)
	malformed[position+len(did)-1] = 'g'
	badDid := did[:len(did)-1] + "g"
	for _, damaged := range [][]byte{canonical[:len(canonical)-1], malformed} {
		store.Set(key, damaged)
		for read := 0; read < 2; read++ {
			value, err := issuer.IssuerDid(s.Ctx, denom)
			require.Error(t, err)
			require.Empty(t, value)
			response, err := s.queryClient.DenomAuthorityMetadata(s.Ctx.Context(), &types.QueryDenomAuthorityMetadataRequest{Denom: denom})
			require.Error(t, err)
			require.Nil(t, response)
			s.Ctx = s.Ctx.WithEventManager(sdk.NewEventManager())
			require.Error(t, issuer.SetIssuerDid(s.Ctx, denom, badDid))
			require.Error(t, issuer.MintByIssuer(s.Ctx, badDid, coin(10), recipient.String()))
			require.Error(t, issuer.BurnByIssuer(s.Ctx, badDid, coin(10), recipient.String()))
			require.Error(t, issuer.SetDenomPausedByIssuer(s.Ctx, denom, badDid, false))
			require.True(t, issuer.IsDenomPaused(s.Ctx, denom))
			require.Equal(t, coin(40), s.App.BankKeeper.GetBalance(s.Ctx, recipient, denom))
			require.Equal(t, coin(40), s.App.BankKeeper.GetSupply(s.Ctx, denom))
			require.Equal(t, damaged, store.Get(key))
			require.Empty(t, s.Ctx.EventManager().Events())
		}
	}
	store.Set(key, canonical)
	observe(40, true)
}

func TestProofIssuerDidAdminConsumers(t *testing.T) {
	t.Run("validation", TestIssuerDidValidate)
	t.Run("change_admin", TestIssuerDidSurvivesChangeAdmin)
}

func TestProofIssuerMintBlockedDestinationAtomic(t *testing.T) {
	s := adminsIssuerSuite(t)
	t.Cleanup(func() { require.NoError(t, s.App.Close()) })
	var issuer types.IssuerKeeper = s.App.TokenFactoryKeeper
	did := adminsIssuerDid(0xab)
	denom := s.defaultDenom
	bank := s.App.BankKeeper
	recipient := s.TestAccs[1]
	module := s.App.AccountKeeper.GetModuleAddress(types.ModuleName)
	blocked := s.App.AccountKeeper.GetModuleAddress(govtypes.ModuleName)
	require.NotEmpty(t, blocked)
	decoded, err := sdk.AccAddressFromBech32(blocked.String())
	require.NoError(t, err)
	require.Equal(t, blocked, decoded)
	require.True(t, bank.BlockedAddr(blocked))
	require.NoError(t, issuer.SetIssuerDid(s.Ctx, denom, did))
	require.NoError(t, issuer.MintByIssuer(s.Ctx, did, sdk.NewInt64Coin(denom, 50), recipient.String()))
	// Keep a nonzero module balance so refusal must preserve existing funds too.
	require.NoError(t, bank.MintCoins(s.Ctx, types.ModuleName, sdk.NewCoins(sdk.NewInt64Coin(denom, 7))))
	ctx := s.Ctx.WithEventManager(sdk.NewEventManager())
	ctx.EventManager().EmitEvent(sdk.NewEvent("before_issuer_mint"))
	events := append(sdk.Events(nil), ctx.EventManager().Events()...)
	evmEvents := ctx.EVMEventManager().Events()
	supply := bank.GetSupply(ctx, denom)
	moduleBalance := bank.GetBalance(ctx, module, denom)
	recipientBalance := bank.GetBalance(ctx, recipient, denom)
	blockedBalance := bank.GetBalance(ctx, blocked, denom)
	for attempt := 0; attempt < 2; attempt++ {
		err := issuer.MintByIssuer(ctx, did, sdk.NewInt64Coin(denom, 11), blocked.String())
		require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
		require.Equal(t, supply, bank.GetSupply(ctx, denom))
		require.Equal(t, moduleBalance, bank.GetBalance(ctx, module, denom))
		require.Equal(t, recipientBalance, bank.GetBalance(ctx, recipient, denom))
		require.Equal(t, blockedBalance, bank.GetBalance(ctx, blocked, denom))
		require.Equal(t, events, ctx.EventManager().Events())
		require.Equal(t, evmEvents, ctx.EVMEventManager().Events())
		adminsRequireIssuerDid(s, issuer, s.TestAccs[0].String(), did)
	}
	successCtx := ctx.WithEventManager(sdk.NewEventManager())
	amount := sdk.NewInt64Coin(denom, 13)
	require.NoError(t, issuer.MintByIssuer(successCtx, did, amount, recipient.String()))
	require.Equal(t, supply.Add(amount), bank.GetSupply(successCtx, denom))
	require.Equal(t, recipientBalance.Add(amount), bank.GetBalance(successCtx, recipient, denom))
	require.Equal(t, moduleBalance, bank.GetBalance(successCtx, module, denom))
	require.Equal(t, blockedBalance, bank.GetBalance(successCtx, blocked, denom))
	requireIssuerSupplyEvent(t, successCtx, types.TypeMsgMint, types.AttributeMintToAddress, recipient.String(), amount)
	require.Equal(t, events, ctx.EventManager().Events())
}
