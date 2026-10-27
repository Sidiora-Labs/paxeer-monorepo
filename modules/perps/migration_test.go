package perps_test

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdktestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/stretchr/testify/require"
)

func ConsensusVersion(t *testing.T, am perps.AppModule) {
	t.Helper()
	require.Equal(t, uint64(1), am.ConsensusVersion())
}

func RunMigrations(t *testing.T, manager *module.Manager, ctx sdk.Context, cfg module.Configurator) {
	t.Helper()
	from := module.VersionMap{types.ModuleName: 1}
	updated, err := manager.RunMigrations(ctx, cfg, from)
	require.NoError(t, err)
	require.Equal(t, module.VersionMap{types.ModuleName: 1}, updated)
	require.Equal(t, module.VersionMap{types.ModuleName: 1}, from)
}

func perpsMigrationSnapshot(t *testing.T, store sdk.KVStore, prefix []byte) [][2][]byte {
	t.Helper()
	iterator := sdk.KVStorePrefixIterator(store, prefix)
	defer func() { require.NoError(t, iterator.Close()) }()
	var entries [][2][]byte
	for ; iterator.Valid(); iterator.Next() {
		entries = append(entries, [2][]byte{
			append([]byte(nil), iterator.Key()...),
			append([]byte(nil), iterator.Value()...),
		})
	}
	return entries
}

func TestRunMigrationsAtPerpsV1IsNoop(t *testing.T) {
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx := sdktestutil.DefaultContext(key, sdk.NewTransientStoreKey("transient_perps_migration")).
		WithBlockHeight(1000).
		WithBlockTime(time.Unix(1_800_000_000, 0))
	k := keeper.NewKeeper(key, nil, nil)
	require.NoError(t, keeper.SeedV6120(ctx, *k))
	store := ctx.KVStore(key)
	require.Len(t, perpsMigrationSnapshot(t, store, types.ParamsKey), 1)
	require.Len(t, perpsMigrationSnapshot(t, store, types.MarketPrefix), 18)
	require.Len(t, perpsMigrationSnapshot(t, store, types.AccountPrefix), 2)

	prefixes := []struct {
		name   string
		prefix []byte
	}{
		{"params", types.ParamsKey},
		{"markets", types.MarketPrefix},
		{"market states", types.MarketStatePrefix},
		{"accounts", types.AccountPrefix},
		{"orders", types.OrderPrefix},
		{"books", types.BookPrefix},
		{"cloids", types.CloidPrefix},
		{"account orders", types.AccountOrderPrefix},
		{"positions", types.PositionPrefix},
		{"account market positions", types.AccountMarketPositionPrefix},
		{"triggers", types.TriggerPrefix},
		{"expiries", types.ExpiryPrefix},
		{"volumes", types.VolumePrefix},
		{"venue volumes", types.VenueVolumePrefix},
		{"liquidation cooldowns", types.LiquidationCooldownPrefix},
		{"sequences", types.SequencePrefix},
		{"attached take profit and stop loss", types.AttachedTpSlPrefix},
		{"margin ratio index", types.MarginRatioIndexPrefix},
		{"adl rank index", types.AdlRankIndexPrefix},
		{"sweep cursor", types.SweepCursorKey},
		{"venue books", types.VenueBooksKey},
	}
	before := make([][][2][]byte, len(prefixes))
	for i, prefix := range prefixes {
		before[i] = perpsMigrationSnapshot(t, store, prefix.prefix)
	}
	wholeStoreBefore := perpsMigrationSnapshot(t, store, nil)

	am := perps.NewAppModule(k)
	ConsensusVersion(t, am)
	manager := module.NewManager(am)
	registry := cdctypes.NewInterfaceRegistry()
	am.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	msgRouter := baseapp.NewMsgServiceRouter()
	msgRouter.SetInterfaceRegistry(registry)
	cfg := module.NewConfigurator(cdc, msgRouter, baseapp.NewGRPCQueryRouter())
	manager.RegisterServices(cfg)
	require.NotNil(t, msgRouter.HandlerByTypeURL("/paxprotocol.paxchain.perps.MsgUpdateParams"))
	require.NotNil(t, msgRouter.HandlerByTypeURL("/paxprotocol.paxchain.perps.MsgSetMarket"))
	require.NotNil(t, msgRouter.HandlerByTypeURL("/paxprotocol.paxchain.perps.MsgSetPaused"))

	RunMigrations(t, manager, ctx, cfg)
	ConsensusVersion(t, am)
	for i, prefix := range prefixes {
		require.Equal(t, before[i], perpsMigrationSnapshot(t, store, prefix.prefix), "%s prefix %x", prefix.name, prefix.prefix)
	}
	require.Equal(t, wholeStoreBefore, perpsMigrationSnapshot(t, store, nil))
}
