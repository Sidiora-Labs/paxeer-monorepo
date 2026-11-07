package keeper

import (
	"testing"

	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestAuthPauseRefusesGrants(t *testing.T) {
	h := newHarness(t)
	methods := []string{"grantSession", "grantSessionSuccessor", "grantCapability"}
	for _, method := range methods {
		require.NoError(t, h.keeper.checkPause(h.ctx, method), method)
	}

	params := h.keeper.GetParams(h.ctx)
	params.Paused = true
	require.NoError(t, h.keeper.SetParams(h.ctx, params))
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			err := h.keeper.checkPause(h.ctx, method)
			require.ErrorIs(t, err, govtypes.ErrPausedScope)
			require.Contains(t, err.Error(), "LXP_PAUSED_SCOPE")
			require.Contains(t, err.Error(), "layerxauth."+method)
			codespace, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, govtypes.ModuleName, codespace)
			require.Equal(t, uint32(735), code)
		})
	}
	require.Equal(t, params, h.keeper.GetParams(h.ctx))

	params.Paused = false
	require.NoError(t, h.keeper.SetParams(h.ctx, params))
	for _, method := range methods {
		require.NoError(t, h.keeper.checkPause(h.ctx, method), method)
	}
}

func TestAuthPauseKeepsRevoke(t *testing.T) {
	h := newHarness(t)
	params := h.keeper.GetParams(h.ctx)
	params.Paused = true
	require.NoError(t, h.keeper.SetParams(h.ctx, params))

	require.NoError(t, h.keeper.checkPause(h.ctx, "revokeGrant"))
	require.Equal(t, params, h.keeper.GetParams(h.ctx))
	require.ErrorIs(t, h.keeper.checkPause(h.ctx, "grantSession"), govtypes.ErrPausedScope)

	params.Paused = false
	require.NoError(t, h.keeper.SetParams(h.ctx, params))
	require.NoError(t, h.keeper.checkPause(h.ctx, "revokeGrant"))
}
