package app_test

import (
	"os"
	"strings"
	"testing"
	"time"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"
)

func TestUpgradeTagsIncludeMergeUpgrade(t *testing.T) {
	t.Setenv("UPGRADE_VERSION_LIST", "")

	content, err := os.ReadFile("tags")
	require.NoError(t, err)
	tags := strings.Split(strings.TrimSpace(string(content)), "\n")
	require.GreaterOrEqual(t, len(tags), 2)
	require.Equal(t, []string{"v6.8", "v7.0"}, tags[len(tags)-2:])
	require.Equal(t, "v7.0", pcommon.MergeUpgrade)
	require.Equal(t, pcommon.MergeUpgrade, app.MergeUpgrade)
	require.Equal(t, pcommon.MergeUpgrade, app.LatestUpgrade)

	wrapper := app.NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	wrapper.App.RegisterUpgradeHandlers()
	require.True(t, wrapper.App.UpgradeKeeper.HasHandler(pcommon.MergeUpgrade))
}
