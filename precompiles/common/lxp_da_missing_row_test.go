package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLXPDAMissingIsNotKernel(t *testing.T) {
	entry, ok := lxpRevertEntryByCode("LXP_DA_MISSING")
	require.True(t, ok, "LXP_DA_MISSING must remain in the registry")
	require.Equal(t, "LXP_DA_MISSING", entry.code)
	require.Equal(t, "da missing", entry.message)
	require.False(t, entry.kernel)
	require.Zero(t, entry.value)

	for _, row := range lxpRevertTable {
		require.NotEqual(t, int32(-804), row.value, "registry row %s retains removed kernel value", row.code)
	}
	require.Equal(t, "LXP_UNKNOWN", LXPCodeName(-804))

	require.Equal(t, "LXP_DA_MISSING: da missing; unavailable",
		lxpDecodeRevertForTest(t, LXPRevert("LXP_DA_MISSING", "unavailable")))
}
