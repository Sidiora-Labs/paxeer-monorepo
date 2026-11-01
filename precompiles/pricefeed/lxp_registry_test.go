package pricefeed_test

import (
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pricefeed"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestLXPRegistryNamesEveryPricefeedCode(t *testing.T) {
	registered := types.LXPCodes()
	require.Len(t, registered, 15)
	names := make([]string, 0, len(registered)+4)
	for _, code := range registered {
		names = append(names, code.Name)
	}
	names = append(names,
		pricefeed.LXP_PRICEFEED_DELEGATECALL,
		pricefeed.LXP_PRICEFEED_STATICCALL,
		pricefeed.LXP_PRICEFEED_KEEPER_MISSING,
		pricefeed.LXP_PRICEFEED_BATCH_TOO_LARGE,
	)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			reason, err := abi.UnpackRevert(pcommon.LXPRevert(name, ""))
			require.NoError(t, err)
			got, _, found := strings.Cut(reason, ": ")
			require.True(t, found, reason)
			require.NotEqual(t, "LXP_UNKNOWN", got, reason)
			require.Equal(t, name, got, reason)
		})
	}
}
