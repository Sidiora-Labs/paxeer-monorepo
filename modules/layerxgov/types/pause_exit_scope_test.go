package types_test

import (
	"testing"

	layerxauthtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	layerxledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/stretchr/testify/require"
)

func TestLedgerPauseScopeAllowsRevokeGrant(t *testing.T) {
	require.NoError(t, types.CheckPauseScope(layerxledgertypes.ModuleName, true, "revokeGrant"))
	require.ErrorIs(t, types.CheckPauseScope(layerxledgertypes.ModuleName, true, "register"), types.ErrPausedScope)
	require.NoError(t, types.CheckPauseScope(layerxledgertypes.ModuleName, false, "register"))
}

func TestAuthPauseScopeAllowsRevokeGrant(t *testing.T) {
	require.NoError(t, types.CheckPauseScope(layerxauthtypes.ModuleName, true, "revokeGrant"))
	require.ErrorIs(t, types.CheckPauseScope(layerxauthtypes.ModuleName, true, "grantSession"), types.ErrPausedScope)
	require.NoError(t, types.CheckPauseScope(layerxauthtypes.ModuleName, false, "grantSession"))
}

func TestAgentPauseScopeAllowsCancel(t *testing.T) {
	require.NoError(t, types.CheckPauseScope("layerxagent", true, "cancel"))
	require.ErrorIs(t, types.CheckPauseScope("layerxagent", true, "schedule"), types.ErrPausedScope)
	require.NoError(t, types.CheckPauseScope("layerxagent", false, "schedule"))
}
