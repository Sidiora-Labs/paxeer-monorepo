package capability_test

import (
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability/types"
	"github.com/stretchr/testify/require"
)

func beginBlockerDuration(reader *oteltest.Reader) uint64 {
	return reader.HistogramCount("paxcosmos_x_capability", "capability_begin_blocker_duration")
}

func TestBeginBlockerRecordsCapabilityBeginBlockerDuration(t *testing.T) {
	reader := oteltest.Install()
	suite := new(CapabilityTestSuite)
	suite.SetT(t)
	suite.SetupTest()

	k := keeper.NewKeeper(suite.cdc, suite.app.GetKey(types.StoreKey), suite.app.GetMemKey(types.MemStoreKey))
	scoped := k.ScopeToModule(banktypes.ModuleName)
	cap, err := scoped.NewCapability(suite.ctx, "transfer")
	require.NoError(t, err)
	require.NotNil(t, cap)
	k.Seal()

	ctx := suite.app.BaseApp.NewUncachedContext(false, tmproto.Header{Height: 1})
	require.False(t, k.IsInitialized(ctx))
	ctx, _ = ctx.CacheContext()
	require.False(t, k.IsInitialized(ctx))

	before := beginBlockerDuration(reader)
	capability.BeginBlocker(ctx, *k)
	after := beginBlockerDuration(reader)
	require.Equal(t, before+1, after, "initializing BeginBlocker must record one capability duration")
	require.True(t, k.IsInitialized(ctx))

	before = after
	capability.BeginBlocker(ctx, *k)
	after = beginBlockerDuration(reader)
	require.Equal(t, before+1, after, "initialized BeginBlocker must record one capability duration")
	require.True(t, k.IsInitialized(ctx))
}
