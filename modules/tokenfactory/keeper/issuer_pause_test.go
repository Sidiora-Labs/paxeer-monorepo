package keeper_test

import (
	"strconv"
	"testing"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
)

func TestSetDenomPausedByIssuerMatches(t *testing.T) {
	s := denomPauseSuite(t)
	k := s.App.TokenFactoryKeeper
	issuerDid := adminsIssuerDid(0xab)
	require.NoError(t, k.SetIssuerDid(s.Ctx, s.defaultDenom, issuerDid))
	ctx := s.Ctx.WithEventManager(sdk.NewEventManager())

	require.Equal(t, types.ErrPauseUnchanged, k.SetDenomPausedByIssuer(ctx, s.defaultDenom, issuerDid, false))
	require.False(t, k.IsDenomPaused(ctx, s.defaultDenom))
	require.Empty(t, ctx.EventManager().Events())

	for _, paused := range []bool{true, false} {
		ctx = ctx.WithEventManager(sdk.NewEventManager())
		require.NoError(t, k.SetDenomPausedByIssuer(ctx, s.defaultDenom, issuerDid, paused))
		require.Equal(t, paused, k.IsDenomPaused(ctx, s.defaultDenom))
		events := ctx.EventManager().Events()
		require.Len(t, events, 1)
		require.Equal(t, sdk.NewEvent(
			"set_denom_paused",
			sdk.NewAttribute("denom", s.defaultDenom),
			sdk.NewAttribute("paused", strconv.FormatBool(paused)),
			sdk.NewAttribute("issuer_did", issuerDid),
		), events[0])

		require.Equal(t, types.ErrPauseUnchanged, k.SetDenomPausedByIssuer(ctx, s.defaultDenom, issuerDid, paused))
		require.Equal(t, paused, k.IsDenomPaused(ctx, s.defaultDenom))
		require.Equal(t, events, ctx.EventManager().Events())
		storedDid, err := k.IssuerDid(ctx, s.defaultDenom)
		require.NoError(t, err)
		require.Equal(t, issuerDid, storedDid)
	}
}

func TestSetDenomPausedByIssuerRefusesOtherDid(t *testing.T) {
	for _, bound := range []bool{true, false} {
		for _, initiallyPaused := range []bool{false, true} {
			t.Run("bound="+strconv.FormatBool(bound)+"/paused="+strconv.FormatBool(initiallyPaused), func(t *testing.T) {
				s := denomPauseSuite(t)
				k := s.App.TokenFactoryKeeper
				issuerDid := adminsIssuerDid(0xab)
				if bound {
					require.NoError(t, k.SetIssuerDid(s.Ctx, s.defaultDenom, issuerDid))
				}
				if initiallyPaused {
					require.NoError(t, denomPauseSet(s, s.TestAccs[0].String(), true))
				}
				ctx := s.Ctx.WithEventManager(sdk.NewEventManager())
				metadata, err := k.GetAuthorityMetadata(ctx, s.defaultDenom)
				require.NoError(t, err)
				callers := []string{adminsIssuerDid(0xcd), "", s.TestAccs[0].String()}
				if !bound {
					callers = append(callers, issuerDid)
				}
				for _, callerDid := range callers {
					for _, paused := range []bool{true, false} {
						err := k.SetDenomPausedByIssuer(ctx, s.defaultDenom, callerDid, paused)
						require.Equal(t, types.ErrIssuerMismatch, err)
						require.Equal(t, initiallyPaused, k.IsDenomPaused(ctx, s.defaultDenom))
						require.Empty(t, ctx.EventManager().Events())
						storedMetadata, err := k.GetAuthorityMetadata(ctx, s.defaultDenom)
						require.NoError(t, err)
						require.Equal(t, metadata, storedMetadata)
					}
				}
			})
		}
	}
}
