package app

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1143_V6120KnownPlanList(t *testing.T) {
	t.Run("ExactPlansAndHistoricalHandler", TestV6120IsAKnownPlanWithAHandler)
	t.Run("FinalPlanKeeperState", func(t *testing.T) {
		wrapper := NewTestWrapper(t, time.Unix(1800000000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), false)
		a, ctx := wrapper.App, wrapper.Ctx
		for _, name := range knownUpgradePlans() {
			require.True(t, a.UpgradeKeeper.HasHandler(name), name)
		}
		require.True(t, a.upgradePlanPending(ctx, AnchorStoreDeleteUpgrade))
		require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, AnchorStoreDeleteUpgrade))
		a.UpgradeKeeper.SetDone(ctx, AnchorStoreDeleteUpgrade)
		require.Equal(t, ctx.BlockHeight(), a.UpgradeKeeper.GetDoneHeight(ctx, AnchorStoreDeleteUpgrade))
		require.False(t, a.upgradePlanPending(ctx, AnchorStoreDeleteUpgrade))
		require.True(t, a.upgradePlanPending(ctx, V6120Upgrade))
		require.False(t, a.upgradePlanPending(ctx, "v6.11"))
	})
}

func TestRepairM_1143_XWebHistoricalUpgrade(t *testing.T) {
	t.Run("CurrentAndHistoricalTags", TestXWebUpgradeIsRegisteredUnderTheLatestTag)
	t.Run("StoreMigration", TestXWebUpgradeStoreLoaderAddsTheXWebStore)
	t.Run("PausedGenesisAndPrecompileConsumer", TestXWebUpgradeSetsPausedDefaultsAndServesThePrecompileOnlyAfterIt)
	t.Run("PreserveExistingState", TestXWebUpgradeKeepsExistingXWebState)
	t.Run("RequireGenesisParameters", TestXWebUpgradeMarkerRequiresGenesisParameters)
}
