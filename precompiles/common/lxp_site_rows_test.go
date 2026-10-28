package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLXPSiteRowsRegistered(t *testing.T) {
	cases := []struct {
		code    string
		message string
	}{
		{code: "LXP_PRICEFEED_DELEGATECALL", message: "cannot delegatecall pricefeed"},
		{code: "LXP_PRICEFEED_STATICCALL", message: "cannot call pricefeed <method> from staticcall"},
		{code: "LXP_PRICEFEED_KEEPER_MISSING", message: "pricefeed: the pricefeed keeper is not wired into this application"},
		{code: "LXP_PRICEFEED_BATCH_TOO_LARGE", message: "getPrices takes at most 64 market ids"},
		{code: "LXP_AGENT_NOT_ACTIVE", message: "<plan> not active at height <height>; no merge upgrade activation reader bound"},
		{code: "LXP_BANK_STATICCALL", message: "cannot call <method> from staticcall"},
		{code: "LXP_BANK_DELEGATECALL", message: "cannot delegatecall <method>"},
		{code: "LXP_BANK_ZERO_VALUE", message: "set `value` field to non-zero to send"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			reason := lxpDecodeRevertForTest(t, LXPRevert(tc.code, ""))
			require.NotContains(t, reason, "LXP_UNKNOWN")
			require.Equal(t, tc.code+": "+tc.message, reason)
		})
	}
}
