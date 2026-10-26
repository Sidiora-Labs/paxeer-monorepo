package keeper_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/stretchr/testify/require"
)

func TestSetLeadersRefusesInvalidList(t *testing.T) {
	for _, tc := range []struct {
		name    string
		leaders [][32]byte
	}{
		{name: "nil"},
		{name: "empty", leaders: [][32]byte{}},
		{name: "zero key", leaders: [][32]byte{{1}, {}}},
		{name: "duplicate", leaders: [][32]byte{{1}, {2}, {1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx, key := newActivationKeeper(t)
			validationErr := types.ValidateLeaders(tc.leaders)
			require.ErrorIs(t, validationErr, types.ErrInvalidGenesis)
			require.EqualError(t, k.SetLeaders(ctx, tc.leaders), validationErr.Error())
			require.Nil(t, ctx.KVStore(key).Get(types.LeaderListKey))

			original := [][32]byte{{3}, {4}}
			require.NoError(t, k.SetLeaders(ctx, original))
			stored := bytes.Clone(ctx.KVStore(key).Get(types.LeaderListKey))
			require.EqualError(t, k.SetLeaders(ctx, tc.leaders), validationErr.Error())
			require.Equal(t, stored, ctx.KVStore(key).Get(types.LeaderListKey))
			leaders, err := k.Leaders(ctx)
			require.NoError(t, err)
			require.Equal(t, original, leaders)
		})
	}

	t.Run("valid replacement and packed layout", func(t *testing.T) {
		k, ctx, key := newActivationKeeper(t)
		for _, want := range [][][32]byte{
			{{0: 3, 31: 4}, {0: 1, 31: 2}, {31: 5}},
			{{31: 6}},
		} {
			require.NoError(t, types.ValidateLeaders(want))
			require.NoError(t, k.SetLeaders(ctx, want))
			packed := ctx.KVStore(key).Get(types.LeaderListKey)
			require.Len(t, packed, len(want)*32)
			for i, leader := range want {
				require.Equal(t, leader[:], packed[i*32:(i+1)*32])
			}
			got, err := k.Leaders(ctx)
			require.NoError(t, err)
			require.Equal(t, want, got)
		}
	})
}

func TestLeaderScheduleBeforeActivation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		packed []byte
	}{
		{name: "absent"},
		{name: "pinned", packed: bytes.Repeat([]byte{1}, 32)},
		{name: "corrupt", packed: []byte{1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx, key := newActivationKeeper(t)
			if tc.packed != nil {
				ctx.KVStore(key).Set(types.LeaderListKey, tc.packed)
			}
			for _, explicitZero := range []bool{false, true} {
				if explicitZero {
					ctx.KVStore(key).Set(types.ActivationHeightKey, make([]byte, 8))
				}
				height, leaders, err := k.LeaderSchedule(ctx)
				require.NoError(t, err)
				require.Zero(t, height)
				require.Nil(t, leaders)
				require.Equal(t, tc.packed, ctx.KVStore(key).Get(types.LeaderListKey))
			}
		})
	}
}

func TestLeaderScheduleAfterActivation(t *testing.T) {
	k, ctx, key := newActivationKeeper(t)
	const activation int64 = 1001
	require.NoError(t, k.SetActivationHeight(ctx, activation))
	for _, want := range [][][32]byte{
		{{0: 3, 31: 4}, {0: 1, 31: 2}, {31: 5}},
		{{31: 6}},
	} {
		require.NoError(t, k.SetLeaders(ctx, want))
		for _, blockHeight := range []int64{activation - 1, activation, activation + 1} {
			height, leaders, err := k.LeaderSchedule(ctx.WithBlockHeight(blockHeight))
			require.NoError(t, err)
			require.Equal(t, activation, height)
			stored, err := k.Leaders(ctx)
			require.NoError(t, err)
			require.Equal(t, want, leaders)
			require.Equal(t, stored, leaders)
		}
	}

	t.Run("corrupt leaders", func(t *testing.T) {
		ctx.KVStore(key).Set(types.LeaderListKey, []byte{1})
		_, leaders, err := k.LeaderSchedule(ctx)
		require.ErrorIs(t, err, keeper.ErrCorruptStore)
		require.Nil(t, leaders)
	})

	for name, encoded := range map[string][]byte{
		"short activation":    {1},
		"overflow activation": binary.BigEndian.AppendUint64(nil, uint64(math.MaxInt64)+1),
	} {
		t.Run(name, func(t *testing.T) {
			k, ctx, key := newActivationKeeper(t)
			require.NoError(t, k.SetLeaders(ctx, [][32]byte{{1}}))
			ctx.KVStore(key).Set(types.ActivationHeightKey, encoded)
			height, leaders, err := k.LeaderSchedule(ctx)
			require.ErrorIs(t, err, keeper.ErrCorruptStore)
			require.Zero(t, height)
			require.Nil(t, leaders)
		})
	}
}
