package keeper

import (
	"bytes"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repair692Keeper(t *testing.T, now time.Time) (Keeper, sdk.Context, *codec.ProtoCodec, *sdk.KVStoreKey) {
	t.Helper()
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(StoreKey)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	registry := codectypes.NewInterfaceRegistry()
	authz.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	return NewKeeper(key, cdc, router), sdk.NewContext(cms, tmproto.Header{Height: 1, Time: now}, false), cdc, key
}

func repair692RoundTrip(t *testing.T, k Keeper, ctx sdk.Context, cdc *codec.ProtoCodec) (Keeper, sdk.Context, *sdk.KVStoreKey) {
	t.Helper()
	exported := k.ExportGenesis(ctx)
	wire, err := cdc.Marshal(exported)
	require.NoError(t, err)
	var decoded authz.GenesisState
	require.NoError(t, cdc.Unmarshal(wire, &decoded))
	require.NoError(t, authz.ValidateGenesis(decoded))
	restored, restoredCtx, _, restoredKey := repair692Keeper(t, ctx.BlockTime())
	restored.InitGenesis(restoredCtx, &decoded)
	require.Equal(t, exported, restored.ExportGenesis(restoredCtx))
	return restored, restoredCtx, restoredKey
}

func TestRepairM_692_ExpiryCleanupAndExportImport(t *testing.T) {
	for _, tc := range []struct {
		name  string
		after time.Duration
	}{
		{name: "at_expiration", after: 0},
		{name: "after_expiration", after: time.Nanosecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
			k, ctx, cdc, storeKey := repair692Keeper(t, now)
			granter := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))
			grantee := sdk.AccAddress(bytes.Repeat([]byte{2}, 20))
			grant := &banktypes.SendAuthorization{SpendLimit: sdk.NewCoins(sdk.NewInt64Coin("steak", 100))}
			msgType := grant.MsgTypeURL()
			expires := now.Add(time.Hour)
			key := grantStoreKey(grantee, granter, msgType)
			require.NoError(t, k.SaveGrant(ctx, grantee, granter, grant, expires))

			otherMsg := authz.NewGenericAuthorization(sdk.MsgTypeURL(&banktypes.MsgMultiSend{}))
			controlExpires := expires.Add(time.Hour)
			require.NoError(t, k.SaveGrant(ctx, granter, grantee, grant, controlExpires))
			require.NoError(t, k.SaveGrant(ctx, grantee, granter, otherMsg, controlExpires))
			require.NotEmpty(t, ctx.KVStore(storeKey).Get(key))
			stored, expiration := k.GetCleanAuthorization(ctx.WithBlockTime(expires.Add(-time.Nanosecond)), grantee, granter, msgType)
			require.Equal(t, grant, stored)
			require.Equal(t, expires, expiration)
			require.Len(t, k.ExportGenesis(ctx).Authorization, 3)

			expiredCtx := ctx.WithBlockTime(expires.Add(tc.after))
			stored, expiration = k.GetCleanAuthorization(expiredCtx, grantee, granter, msgType)
			require.Nil(t, stored)
			require.Equal(t, time.Time{}, expiration)
			require.False(t, expiredCtx.KVStore(storeKey).Has(key), "clean lookup must delete the exact persisted grant")
			count := 0
			k.IterateGrants(expiredCtx, func(g, r sdk.AccAddress, entry authz.Grant) bool {
				count++
				require.False(t, g.Equals(granter) && r.Equals(grantee) && entry.GetAuthorization().MsgTypeURL() == msgType)
				return false
			})
			require.Equal(t, 2, count)
			require.Len(t, k.ExportGenesis(expiredCtx).Authorization, 2)

			restored, restoredCtx, restoredKey := repair692RoundTrip(t, k, expiredCtx, cdc)
			require.False(t, restoredCtx.KVStore(restoredKey).Has(key))
			stored, expiration = restored.GetCleanAuthorization(restoredCtx, grantee, granter, msgType)
			require.Nil(t, stored)
			require.Equal(t, time.Time{}, expiration)
			stored, expiration = restored.GetCleanAuthorization(restoredCtx, granter, grantee, msgType)
			require.Equal(t, grant, stored)
			require.Equal(t, controlExpires, expiration)
			stored, expiration = restored.GetCleanAuthorization(restoredCtx, grantee, granter, otherMsg.MsgTypeURL())
			require.Equal(t, otherMsg, stored)
			require.Equal(t, controlExpires, expiration)
		})
	}
}

func TestRepairM_692_ExpiredImportIsCleaned(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	k, ctx, cdc, storeKey := repair692Keeper(t, now)
	granter := sdk.AccAddress(bytes.Repeat([]byte{3}, 20))
	grantee := sdk.AccAddress(bytes.Repeat([]byte{4}, 20))
	grant := &banktypes.SendAuthorization{SpendLimit: sdk.NewCoins(sdk.NewInt64Coin("steak", 100))}
	expires := now.Add(time.Hour)
	msgType := grant.MsgTypeURL()
	key := grantStoreKey(grantee, granter, msgType)
	require.NoError(t, k.SaveGrant(ctx, grantee, granter, grant, expires))
	stored, expiration := k.GetCleanAuthorization(ctx, grantee, granter, msgType)
	require.Equal(t, grant, stored)
	require.Equal(t, expires, expiration)
	require.True(t, ctx.KVStore(storeKey).Has(key))

	expiredCtx := ctx.WithBlockTime(expires.Add(time.Nanosecond))
	require.Len(t, k.ExportGenesis(expiredCtx).Authorization, 1)
	restored, restoredCtx, restoredKey := repair692RoundTrip(t, k, expiredCtx, cdc)
	require.True(t, restoredCtx.KVStore(restoredKey).Has(key), "import must actually restore the expired record before cleanup")
	stored, expiration = restored.GetCleanAuthorization(restoredCtx, grantee, granter, msgType)
	require.Nil(t, stored)
	require.Equal(t, time.Time{}, expiration)
	require.False(t, restoredCtx.KVStore(restoredKey).Has(key))
	require.Empty(t, restored.ExportGenesis(restoredCtx).Authorization)

	reopened, reopenedCtx, reopenedKey := repair692RoundTrip(t, restored, restoredCtx, cdc)
	require.False(t, reopenedCtx.KVStore(reopenedKey).Has(key))
	stored, expiration = reopened.GetCleanAuthorization(reopenedCtx, grantee, granter, msgType)
	require.Nil(t, stored)
	require.Equal(t, time.Time{}, expiration)
	require.Empty(t, reopened.ExportGenesis(reopenedCtx).Authorization)
}
