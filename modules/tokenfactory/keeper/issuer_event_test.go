package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func TestSetIssuerDidEmitsEvent(t *testing.T) {
	s := adminsIssuerSuite(t)
	k := s.App.TokenFactoryKeeper
	issuerDid := adminsIssuerDid(0xab)
	ctx := s.Ctx.WithEventManager(sdk.NewEventManager())

	s.Require().ErrorIs(k.SetIssuerDid(ctx, s.defaultDenom, "invalid"), types.ErrInvalidAuthorityMetadata)
	s.Require().Empty(ctx.EventManager().Events())
	metadata, err := k.GetAuthorityMetadata(ctx, s.defaultDenom)
	s.Require().NoError(err)
	s.Require().Empty(metadata.IssuerDid)

	s.Require().NoError(k.SetIssuerDid(ctx, s.defaultDenom, issuerDid))
	events := ctx.EventManager().Events()
	s.Require().Len(events, 1)
	s.Require().Equal("set_issuer_did", events[0].Type)
	s.Require().Equal(sdk.NewEvent(
		"set_issuer_did",
		sdk.NewAttribute("denom", s.defaultDenom),
		sdk.NewAttribute("issuer_did", issuerDid),
	), events[0])
	metadata, err = k.GetAuthorityMetadata(ctx, s.defaultDenom)
	s.Require().NoError(err)
	s.Require().Equal(types.DenomAuthorityMetadata{
		Admin:     s.TestAccs[0].String(),
		IssuerDid: issuerDid,
	}, metadata)

	ctx = ctx.WithEventManager(sdk.NewEventManager())
	s.Require().NoError(k.SetIssuerDid(ctx, s.defaultDenom, issuerDid))
	s.Require().Empty(ctx.EventManager().Events())

	s.Require().ErrorIs(k.SetIssuerDid(ctx, s.defaultDenom, adminsIssuerDid(0xcd)), types.ErrInvalidAuthorityMetadata)
	s.Require().Empty(ctx.EventManager().Events())
	storedDid, err := k.IssuerDid(ctx, s.defaultDenom)
	s.Require().NoError(err)
	s.Require().Equal(issuerDid, storedDid)
}
