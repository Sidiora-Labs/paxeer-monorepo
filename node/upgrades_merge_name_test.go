package app

import (
	"testing"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/stretchr/testify/require"
)

func TestMergeUpgradeNameAgreesAcrossPackages(t *testing.T) {
	require.Equal(t, pcommon.MergeUpgrade, MergeUpgrade)

	plans := knownUpgradePlans()
	require.NotEmpty(t, plans)
	require.Equal(t, MergeUpgrade, plans[len(plans)-1])
}
