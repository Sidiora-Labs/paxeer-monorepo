package types

import (
	"strings"
	"testing"

	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestGovernancePauseConformance(t *testing.T) {
	expectedExits := map[string]map[string]bool{
		"layerxpay": {
			"escrowRelease": true, "escrowTimeout": true, "budgetClose": true,
			"budgetDefund": true, "budgetRevoke": true, "streamPause": true,
			"streamClose": true, "serviceOfferWithdraw": true, "serviceCommitAbandon": true,
		},
		"layerxexchange": {"cancelOrder": true, "cancelSpotOrder": true, "closePosition": true, "withdrawMargin": true},
		"layerxledger":   {"revokeGrant": true},
		"layerxauth":     {"revokeGrant": true},
		"layerxagent":    {"cancel": true},
	}
	require.Equal(t, expectedExits, pauseExitPaths)
	require.Equal(t, uint32(735), ErrPausedScope.ABCICode())

	for module, exits := range pauseExitPaths {
		t.Run(module, func(t *testing.T) {
			for method := range exits {
				t.Run(method, func(t *testing.T) {
					require.NoError(t, CheckPauseScope(module, false, method))
					require.NoError(t, CheckPauseScope(module, true, method))
				})
			}

			blocked := []string{"unknownMethod", "budgetSpend", "placeOrder", "register", "grantSession", "schedule"}
			for _, otherExits := range expectedExits {
				for method := range otherExits {
					if !exits[method] {
						blocked = append(blocked, method)
					}
				}
			}
			for method := range exits {
				blocked = append(blocked, strings.ToUpper(method), method+"Suffix")
			}
			for _, method := range blocked {
				t.Run(method, func(t *testing.T) {
					require.NoError(t, CheckPauseScope(module, false, method))
					err := CheckPauseScope(module, true, method)
					require.ErrorIs(t, err, ErrPausedScope)
					codespace, code, _ := sdkerrors.ABCIInfo(err, false)
					require.Equal(t, ModuleName, codespace)
					require.Equal(t, uint32(735), code)
				})
			}

			t.Run("empty_method", func(t *testing.T) {
				for _, paused := range []bool{false, true} {
					require.ErrorIs(t, CheckPauseScope(module, paused, ""), sdkerrors.ErrInvalidRequest)
				}
			})
		})
	}

	for _, module := range []string{"unknown-module", "tokenfactory"} {
		t.Run(module, func(t *testing.T) {
			for _, paused := range []bool{false, true} {
				for _, method := range []string{"unknownMethod", "cancel", "revokeGrant", "escrowRelease", "setDenomPaused", ""} {
					err := CheckPauseScope(module, paused, method)
					require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest, "paused=%t method=%q", paused, method)
				}
			}
		})
	}
}
