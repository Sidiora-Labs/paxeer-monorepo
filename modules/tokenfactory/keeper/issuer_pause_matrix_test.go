package keeper_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestIssuerPauseAuthorityMatrix(t *testing.T) {
	issuerDid := adminsIssuerDid(0xab)
	otherDid := adminsIssuerDid(0xcd)
	for _, tc := range []struct {
		name       string
		bound      bool
		callerDid  string
		admin      bool
		unchanged  bool
		wantErr    *sdkerrors.Error
		wantCode   uint32
		wantPrefix string
	}{
		{name: "issuer DID allowed", bound: true, callerDid: issuerDid},
		{name: "other DID refused", bound: true, callerDid: otherDid, wantErr: types.ErrIssuerMismatch, wantCode: 28, wantPrefix: "LXP_UNAUTHORIZED_DEBIT"},
		{name: "other DID refused before unchanged state", bound: true, callerDid: otherDid, unchanged: true, wantErr: types.ErrIssuerMismatch, wantCode: 28, wantPrefix: "LXP_UNAUTHORIZED_DEBIT"},
		{name: "empty caller DID refused", bound: true, wantErr: types.ErrIssuerMismatch, wantCode: 28, wantPrefix: "LXP_UNAUTHORIZED_DEBIT"},
		{name: "unbound issuer refused", callerDid: issuerDid, wantErr: types.ErrIssuerMismatch, wantCode: 28, wantPrefix: "LXP_UNAUTHORIZED_DEBIT"},
		{name: "unbound empty issuer refused", wantErr: types.ErrIssuerMismatch, wantCode: 28, wantPrefix: "LXP_UNAUTHORIZED_DEBIT"},
		{name: "unbound issuer refused before unchanged state", callerDid: issuerDid, unchanged: true, wantErr: types.ErrIssuerMismatch, wantCode: 28, wantPrefix: "LXP_UNAUTHORIZED_DEBIT"},
		{name: "issuer repeat refused", bound: true, callerDid: issuerDid, unchanged: true, wantErr: types.ErrPauseUnchanged, wantCode: 27, wantPrefix: "LXP_PAUSED_SCOPE"},
		{name: "bech32 admin refused for issuer bound denom", bound: true, admin: true, wantErr: types.ErrIssuerMismatch, wantCode: 28, wantPrefix: "LXP_UNAUTHORIZED_DEBIT"},
		{name: "bech32 admin refused before unchanged state", bound: true, admin: true, unchanged: true, wantErr: types.ErrIssuerMismatch, wantCode: 28, wantPrefix: "LXP_UNAUTHORIZED_DEBIT"},
		{name: "bech32 admin allowed for unbound denom", admin: true},
		{name: "bech32 admin repeat refused for unbound denom", admin: true, unchanged: true, wantErr: types.ErrPauseUnchanged, wantCode: 27, wantPrefix: "LXP_PAUSED_SCOPE"},
	} {
		for _, initiallyPaused := range []bool{false, true} {
			t.Run(tc.name+"/initially_paused="+strconv.FormatBool(initiallyPaused), func(t *testing.T) {
				s := denomPauseSuite(t)
				k := s.App.TokenFactoryKeeper
				admin := s.TestAccs[0].String()
				if initiallyPaused {
					response, err := s.msgServer.SetDenomPaused(sdk.WrapSDKContext(s.Ctx), types.NewMsgSetDenomPaused(admin, s.defaultDenom, true))
					require.NoError(t, err)
					require.NotNil(t, response)
				}
				if tc.bound {
					require.NoError(t, k.SetIssuerDid(s.Ctx, s.defaultDenom, issuerDid))
				}
				metadata, err := k.GetAuthorityMetadata(s.Ctx, s.defaultDenom)
				require.NoError(t, err)
				wantMetadata := types.DenomAuthorityMetadata{Admin: admin}
				if tc.bound {
					wantMetadata.IssuerDid = issuerDid
				}
				require.Equal(t, wantMetadata, metadata)
				require.Equal(t, initiallyPaused, k.IsDenomPaused(s.Ctx, s.defaultDenom))
				ctx := s.Ctx.WithEventManager(sdk.NewEventManager())
				paused := !initiallyPaused
				if tc.unchanged {
					paused = initiallyPaused
				}

				if tc.admin {
					var response *types.MsgSetDenomPausedResponse
					response, err = s.msgServer.SetDenomPaused(sdk.WrapSDKContext(ctx), types.NewMsgSetDenomPaused(admin, s.defaultDenom, paused))
					if tc.wantErr == nil {
						require.NotNil(t, response)
					} else {
						require.Nil(t, response)
					}
				} else {
					err = k.SetDenomPausedByIssuer(ctx, s.defaultDenom, tc.callerDid, paused)
				}

				if tc.wantErr == nil {
					require.NoError(t, err)
					require.Equal(t, paused, k.IsDenomPaused(ctx, s.defaultDenom))
					wantEvent := sdk.NewEvent(
						types.TypeMsgSetDenomPaused,
						sdk.NewAttribute(types.AttributeDenom, s.defaultDenom),
						sdk.NewAttribute(types.AttributePaused, strconv.FormatBool(paused)),
					)
					if !tc.admin {
						wantEvent = wantEvent.AppendAttributes(sdk.NewAttribute(types.AttributeIssuerDid, issuerDid))
					}
					require.Equal(t, sdk.Events{wantEvent}, ctx.EventManager().Events())
				} else {
					require.ErrorIs(t, err, tc.wantErr)
					codespace, code, _ := sdkerrors.ABCIInfo(err, false)
					require.Equal(t, types.ModuleName, codespace)
					require.Equal(t, tc.wantCode, code)
					require.Equal(t, initiallyPaused, k.IsDenomPaused(ctx, s.defaultDenom))
					require.Empty(t, ctx.EventManager().Events())
				}
				stored, metadataErr := k.GetAuthorityMetadata(ctx, s.defaultDenom)
				require.NoError(t, metadataErr)
				require.Equal(t, metadata, stored)
				if tc.wantErr != nil {
					if tc.admin && tc.bound {
						require.EqualError(t, err, "denom is issuer bound: LXP_UNAUTHORIZED_DEBIT: caller is not the denom issuer")
					} else {
						require.True(t, strings.HasPrefix(err.Error(), tc.wantPrefix), "error %q must begin with %q", err.Error(), tc.wantPrefix)
					}
				}
			})
		}
	}
}
