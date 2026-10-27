package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLXPAgentRowsRegistered(t *testing.T) {
	for _, row := range []struct {
		code    string
		message string
	}{
		{code: "LXP_AGENT_NOT_ACTIVE", message: "merge-plan not active"},
		{code: "LXP_AGENT_SCHEDULER_CALLDATA_TOO_LARGE", message: "scheduler calldata too large"},
	} {
		t.Run(row.code, func(t *testing.T) {
			reason := lxpDecodeRevertForTest(t, LXPRevert(row.code, ""))
			require.False(t, strings.HasPrefix(reason, "LXP_UNKNOWN"), reason)
			require.True(t, strings.HasPrefix(reason, row.code+": "), reason)
			require.Equal(t, row.code+": "+row.message, reason)
		})
	}
}

func TestLXPPayRowsRegistered(t *testing.T) {
	for _, row := range []struct {
		code    string
		message string
	}{
		{code: "LXP_PAY_UNBOUND_CALLER", message: "unbound caller"},
		{code: "LXP_PAY_KERNEL_GATE_MISMATCH", message: "kernel gate mismatch"},
	} {
		t.Run(row.code, func(t *testing.T) {
			reason := lxpDecodeRevertForTest(t, LXPRevert(row.code, ""))
			require.False(t, strings.HasPrefix(reason, "LXP_UNKNOWN"), reason)
			require.True(t, strings.HasPrefix(reason, row.code+": "), reason)
			require.Equal(t, row.code+": "+row.message, reason)
		})
	}
}

func TestLXPArbiterRowsCarryKernelValues(t *testing.T) {
	for _, row := range []struct {
		code  string
		value int32
	}{
		{code: "LXP_ARBITER_REQUIRED", value: -737},
		{code: "LXP_UNAUTHORIZED_ARBITER", value: -738},
		{code: "LXP_ESCROW_LINK_MISMATCH", value: -739},
	} {
		t.Run(row.code, func(t *testing.T) {
			entry, ok := lxpRevertEntryByCode(row.code)
			require.True(t, ok, "registry row must exist")
			require.True(t, entry.kernel, "registry row must carry a kernel value")
			require.Equal(t, row.value, entry.value)
			require.Equal(t, row.code, LXPCodeName(row.value))
		})
	}
}
