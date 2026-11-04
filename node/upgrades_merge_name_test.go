package app

import (
	"testing"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/stretchr/testify/require"
)

func TestMergeUpgradeNameAgreesAcrossPackages(t *testing.T) {
	require.Equal(t, pcommon.MergeUpgrade, MergeUpgrade)

	plans := knownUpgradePlans()
	require.Equal(t, "v7.0", MergeUpgrade)
	require.Equal(t, []string{"v6.9", "v6.10", "v6.12.0", "v7.0", "v7.0-anchor-store-delete"}, plans)
	require.Equal(t, []string{MergeUpgrade, AnchorStoreDeleteUpgrade}, plans[len(plans)-2:])
}
