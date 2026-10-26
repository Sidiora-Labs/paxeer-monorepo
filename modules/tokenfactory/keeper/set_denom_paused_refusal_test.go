package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestSetDenomPausedMissingDenom(t *testing.T) {
	s := denomPauseSuite(t)
	admin := s.TestAccs[0].String()
	denom := "factory/" + admin + "/missing"
	k := s.App.TokenFactoryKeeper
	_, found := s.App.BankKeeper.GetDenomMetaData(s.Ctx, denom)
	require.False(t, found)
	metadata, err := k.GetAuthorityMetadata(s.Ctx, denom)
	require.NoError(t, err)
	require.Equal(t, types.DenomAuthorityMetadata{}, metadata)
	events := s.Ctx.EventManager().Events()

	for _, paused := range []bool{true, false} {
		response, err := s.msgServer.SetDenomPaused(sdk.WrapSDKContext(s.Ctx), types.NewMsgSetDenomPaused(admin, denom, paused))
		require.ErrorIs(t, err, types.ErrDenomDoesNotExist)
		require.Contains(t, err.Error(), "denom: "+denom)
		require.Nil(t, response)
		require.False(t, k.IsDenomPaused(s.Ctx, denom))
		require.Equal(t, events, s.Ctx.EventManager().Events())
	}
	_, found = s.App.BankKeeper.GetDenomMetaData(s.Ctx, denom)
	require.False(t, found)
	stored, err := k.GetAuthorityMetadata(s.Ctx, denom)
	require.NoError(t, err)
	require.Equal(t, metadata, stored)
}

func TestSetDenomPausedRefusesAdminWhenIssuerBound(t *testing.T) {
	for _, initiallyPaused := range []bool{false, true} {
		name := "unpaused"
		if initiallyPaused {
			name = "paused"
		}
		t.Run(name, func(t *testing.T) {
			s := denomPauseSuite(t)
			k := s.App.TokenFactoryKeeper
			admin := s.TestAccs[0].String()
			other := s.TestAccs[1].String()
			metadata, err := k.GetAuthorityMetadata(s.Ctx, s.defaultDenom)
			require.NoError(t, err)
			require.Equal(t, admin, metadata.Admin)
			require.Empty(t, metadata.IssuerDid)

			require.ErrorIs(t, denomPauseSet(s, other, true), types.ErrUnauthorized)
			require.False(t, k.IsDenomPaused(s.Ctx, s.defaultDenom))
			require.Empty(t, denomPauseEvents(s))
			require.NoError(t, denomPauseSet(s, admin, true))
			require.True(t, k.IsDenomPaused(s.Ctx, s.defaultDenom))
			require.Equal(t, []string{"true"}, denomPauseEvents(s))
			if !initiallyPaused {
				require.NoError(t, denomPauseSet(s, admin, false))
				require.False(t, k.IsDenomPaused(s.Ctx, s.defaultDenom))
				require.Equal(t, []string{"true", "false"}, denomPauseEvents(s))
			}

			issuer := adminsIssuerDid(0xab)
			require.NoError(t, k.SetIssuerDid(s.Ctx, s.defaultDenom, issuer))
			metadata, err = k.GetAuthorityMetadata(s.Ctx, s.defaultDenom)
			require.NoError(t, err)
			require.Equal(t, types.DenomAuthorityMetadata{Admin: admin, IssuerDid: issuer}, metadata)
			events := s.Ctx.EventManager().Events()

			for _, paused := range []bool{!initiallyPaused, initiallyPaused} {
				response, err := s.msgServer.SetDenomPaused(sdk.WrapSDKContext(s.Ctx), types.NewMsgSetDenomPaused(admin, s.defaultDenom, paused))
				require.ErrorIs(t, err, types.ErrIssuerMismatch)
				require.Contains(t, err.Error(), "denom is issuer bound")
				require.Nil(t, response)
				require.Equal(t, initiallyPaused, k.IsDenomPaused(s.Ctx, s.defaultDenom))
				require.Equal(t, events, s.Ctx.EventManager().Events())
			}
			require.ErrorIs(t, denomPauseSet(s, other, !initiallyPaused), types.ErrUnauthorized)
			require.Equal(t, initiallyPaused, k.IsDenomPaused(s.Ctx, s.defaultDenom))
			require.Equal(t, events, s.Ctx.EventManager().Events())
			stored, err := k.GetAuthorityMetadata(s.Ctx, s.defaultDenom)
			require.NoError(t, err)
			require.Equal(t, metadata, stored)
		})
	}
}
