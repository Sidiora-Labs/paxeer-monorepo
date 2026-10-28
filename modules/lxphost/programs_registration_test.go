//go:build cgo

package lxphost

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostProgramsModuleAcceptsAccountSandboxAndRetirementActivities(t *testing.T) {
	for _, activity := range []struct {
		name   string
		typeID uint32
	}{
		{"account", 0x00090006},
		{"wind-down", 0x00090007},
		{"sandbox", 0x00090009},
		{"storage-retirement", 0x0009000B},
	} {
		t.Run(activity.name, func(t *testing.T) {
			chain := newTestChain(t)
			host := chain.host(t)
			authority := executeActivityAuthority(t, [32]byte{0x73})
			ctx := chain.context(1, bindingBlockTime, 41)

			receipt, err := ExecuteActivity(ctx, host, authority, activity.typeID, nil, 0)
			require.NoError(t, err)
			require.NotEqual(t, Code(-106), receipt.ResultCode)
			require.Equal(t, Code(-1), receipt.ResultCode)
			require.Equal(t, "LXP_ERR_TRUNCATED", receipt.ResultCode.Error())
		})
	}
}

func TestHostProgramsModuleRefusesUnknownOrdinal(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	authority := executeActivityAuthority(t, [32]byte{0x74})
	ctx := chain.context(1, bindingBlockTime, 41)

	_, err := ExecuteActivity(ctx, host, authority, 0x0009000C, nil, 0)
	require.Equal(t, Code(-106), err)
	require.EqualError(t, err, "LXP_ERR_UNKNOWN_ACTIVITY")
}
