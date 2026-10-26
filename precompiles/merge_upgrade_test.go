package precompiles_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/stretchr/testify/require"
)

func TestMergeUpgradeSingleSource(t *testing.T) {
	require.Equal(t, pcommon.MergeUpgrade, precompiles.MergeUpgrade)
}
