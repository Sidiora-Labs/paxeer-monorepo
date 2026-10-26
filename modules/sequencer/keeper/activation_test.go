package keeper_test

import (
	"encoding/binary"
	"math"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func newActivationKeeper(t *testing.T) (keeper.Keeper, sdk.Context, sdk.StoreKey) {
	t.Helper()
	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(types.StoreKey)
	ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, tmproto.Header{Height: 1000}, false)
	return keeper.NewKeeper(key), ctx, key
}

func TestActivationHeightUnsetIsZero(t *testing.T) {
	k, ctx, key := newActivationKeeper(t)
	require.Nil(t, ctx.KVStore(key).Get(types.ActivationHeightKey))
	height, err := k.ActivationHeight(ctx)
	require.NoError(t, err)
	require.Zero(t, height)
	for _, h := range []int64{math.MinInt64, -1, 0, 1, 1000, math.MaxInt64} {
		sequenced, err := k.Sequenced(ctx, h)
		require.NoError(t, err)
		require.False(t, sequenced, "unset activation at height %d", h)
	}
	require.Nil(t, ctx.KVStore(key).Get(types.ActivationHeightKey))
}

func TestSequencedFromActivationHeight(t *testing.T) {
	for _, activation := range []int64{1, 1001, math.MaxInt64} {
		k, ctx, key := newActivationKeeper(t)
		require.NoError(t, k.SetActivationHeight(ctx, activation))
		require.Equal(t, binary.BigEndian.AppendUint64(nil, uint64(activation)), ctx.KVStore(key).Get(types.ActivationHeightKey))
		stored, err := k.ActivationHeight(ctx)
		require.NoError(t, err)
		require.Equal(t, activation, stored)
		for _, h := range []int64{math.MinInt64, -1, 0, activation - 1, activation, math.MaxInt64} {
			sequenced, err := k.Sequenced(ctx.WithBlockHeight(activation-1), h)
			require.NoError(t, err)
			require.Equal(t, h >= activation, sequenced, "activation %d, height %d", activation, h)
		}
		if activation < math.MaxInt64 {
			sequenced, err := k.Sequenced(ctx, activation+1)
			require.NoError(t, err)
			require.True(t, sequenced)
		}
	}
}

func TestSetActivationHeightRefusesRewrite(t *testing.T) {
	k, ctx, key := newActivationKeeper(t)
	for _, height := range []int64{math.MinInt64, -1, 0} {
		require.ErrorIs(t, k.SetActivationHeight(ctx, height), types.ErrInvalidGenesis)
		require.Nil(t, ctx.KVStore(key).Get(types.ActivationHeightKey))
	}
	const activation int64 = 1001
	require.NoError(t, k.SetActivationHeight(ctx, activation))
	require.NoError(t, k.SetActivationHeight(ctx, activation))
	for _, height := range []int64{math.MinInt64, -1, 0, 1, activation - 1, activation + 1, math.MaxInt64} {
		require.ErrorIs(t, k.SetActivationHeight(ctx, height), types.ErrInvalidGenesis)
		stored, err := k.ActivationHeight(ctx)
		require.NoError(t, err)
		require.Equal(t, activation, stored)
	}

	for name, encoded := range map[string][]byte{
		"short encoding":  {0x01},
		"long encoding":   make([]byte, 9),
		"height overflow": binary.BigEndian.AppendUint64(nil, uint64(math.MaxInt64)+1),
	} {
		t.Run(name, func(t *testing.T) {
			k, ctx, key := newActivationKeeper(t)
			ctx.KVStore(key).Set(types.ActivationHeightKey, encoded)
			height, err := k.ActivationHeight(ctx)
			require.ErrorIs(t, err, keeper.ErrCorruptStore)
			require.Zero(t, height)
			sequenced, err := k.Sequenced(ctx, activation)
			require.ErrorIs(t, err, keeper.ErrCorruptStore)
			require.False(t, sequenced)
			require.ErrorIs(t, k.SetActivationHeight(ctx, activation), keeper.ErrCorruptStore)
			require.Equal(t, encoded, ctx.KVStore(key).Get(types.ActivationHeightKey))
		})
	}
}
