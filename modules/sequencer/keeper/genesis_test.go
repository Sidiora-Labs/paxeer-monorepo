package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/stretchr/testify/require"
)

func genesisStateForTest() types.GenesisState {
	gs := types.GenesisState{Leaders: [][32]byte{{0: 3, 31: 4}, {0: 1, 31: 2}}}
	for i := byte(1); i <= 2; i++ {
		private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{i}, ed25519.SeedSize))
		key := types.ReceiptKey{
			KeyId:       [types.KeyIDLength]byte{i},
			FirstHeight: 1,
			LastHeight:  math.MaxUint64,
		}
		copy(key.PublicKey[:], private.Public().(ed25519.PublicKey))
		gs.ReceiptKeys = append(gs.ReceiptKeys, key)
	}
	return gs
}

func TestInitGenesisDefaultsActivationToNextBlock(t *testing.T) {
	for _, tc := range []struct {
		name       string
		height     int64
		activation uint64
		want       int64
	}{
		{name: "initial chain", height: 0, want: 1},
		{name: "merge plan", height: 1000, want: 1001},
		{name: "explicit past height", height: 1000, activation: 900, want: 900},
		{name: "explicit current height", height: 1000, activation: 1000, want: 1000},
		{name: "explicit future height", height: 1000, activation: 1750, want: 1750},
		{name: "largest activation", height: 1000, activation: math.MaxInt64, want: math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx, key := newActivationKeeper(t)
			ctx = ctx.WithBlockHeight(tc.height)
			gs := genesisStateForTest()
			gs.ActivationHeight = tc.activation
			require.NoError(t, k.InitGenesis(ctx, gs))
			activation, err := k.ActivationHeight(ctx)
			require.NoError(t, err)
			require.Equal(t, tc.want, activation)
			require.Equal(t, binary.BigEndian.AppendUint64(nil, uint64(tc.want)), ctx.KVStore(key).Get(types.ActivationHeightKey))
			before, err := k.Sequenced(ctx, tc.want-1)
			require.NoError(t, err)
			require.False(t, before)
			at, err := k.Sequenced(ctx, tc.want)
			require.NoError(t, err)
			require.True(t, at)
		})
	}

	for _, tc := range []struct {
		name       string
		height     int64
		activation uint64
	}{
		{name: "carried height overflows", height: 1000, activation: uint64(math.MaxInt64) + 1},
		{name: "largest unsigned height", height: 1000, activation: math.MaxUint64},
		{name: "next block overflows", height: math.MaxInt64},
		{name: "nonpositive next block", height: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx, key := newActivationKeeper(t)
			ctx = ctx.WithBlockHeight(tc.height)
			gs := genesisStateForTest()
			gs.ActivationHeight = tc.activation
			require.ErrorIs(t, k.InitGenesis(ctx, gs), types.ErrInvalidGenesis)
			for _, entry := range [][]byte{types.ActivationHeightKey, types.LeaderListKey, types.ReceiptKeysKey, types.LastSeqKey, types.LastSeqHashKey} {
				require.Nil(t, ctx.KVStore(key).Get(entry))
			}
		})
	}
}

func TestExportGenesisKeepsActivationHeight(t *testing.T) {
	for _, activation := range []uint64{0, 1750} {
		for _, sequenced := range []bool{false, true} {
			gs := genesisStateForTest()
			gs.ActivationHeight = activation
			if sequenced {
				gs.LastSeq = 37
				gs.LastSeqHash = [types.HashLength]byte{0: 0x42, 31: 0x24}
			}
			k, ctx, _ := newActivationKeeper(t)
			require.NoError(t, k.InitGenesis(ctx, gs))
			if activation == 0 {
				gs.ActivationHeight = uint64(ctx.BlockHeight() + 1)
			}
			exported, err := k.ExportGenesis(ctx.WithBlockHeight(10000))
			require.NoError(t, err)
			require.Equal(t, gs, exported)
			require.NoError(t, exported.Validate())

			restored, freshCtx, _ := newActivationKeeper(t)
			freshCtx = freshCtx.WithBlockHeight(5000)
			require.NoError(t, restored.InitGenesis(freshCtx, exported))
			height, err := restored.ActivationHeight(freshCtx)
			require.NoError(t, err)
			require.Equal(t, int64(gs.ActivationHeight), height)
			leaders, err := restored.Leaders(freshCtx)
			require.NoError(t, err)
			require.Equal(t, gs.Leaders, leaders)
			keys, err := restored.ReceiptKeys(freshCtx)
			require.NoError(t, err)
			require.Equal(t, gs.ReceiptKeys, keys)
			lastSeq, err := restored.LastSeq(freshCtx)
			require.NoError(t, err)
			require.Equal(t, gs.LastSeq, lastSeq)
			lastSeqHash, err := restored.LastSeqHash(freshCtx)
			require.NoError(t, err)
			require.Equal(t, gs.LastSeqHash, lastSeqHash)
			reexported, err := restored.ExportGenesis(freshCtx)
			require.NoError(t, err)
			require.Equal(t, exported, reexported)
		}
	}

	for name, encoded := range map[string][]byte{
		"short activation":    {1},
		"overflow activation": binary.BigEndian.AppendUint64(nil, uint64(math.MaxInt64)+1),
	} {
		t.Run(name, func(t *testing.T) {
			k, ctx, key := newActivationKeeper(t)
			require.NoError(t, k.InitGenesis(ctx, genesisStateForTest()))
			ctx.KVStore(key).Set(types.ActivationHeightKey, encoded)
			exported, err := k.ExportGenesis(ctx)
			require.ErrorIs(t, err, keeper.ErrCorruptStore)
			require.Equal(t, types.GenesisState{}, exported)
		})
	}
}

func TestLeadersRefusesCorruptList(t *testing.T) {
	for _, tc := range []struct {
		name    string
		leaders [][32]byte
	}{
		{name: "duplicate", leaders: [][32]byte{{1}, {1}}},
		{name: "nonadjacent duplicate", leaders: [][32]byte{{1}, {2}, {1}}},
		{name: "zero key", leaders: [][32]byte{{}}},
		{name: "zero among valid keys", leaders: [][32]byte{{1}, {}, {2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx, key := newActivationKeeper(t)
			var packed []byte
			for _, leader := range tc.leaders {
				packed = append(packed, leader[:]...)
			}
			ctx.KVStore(key).Set(types.LeaderListKey, packed)
			leaders, err := k.Leaders(ctx)
			require.ErrorIs(t, err, keeper.ErrCorruptStore)
			require.Nil(t, leaders)
			validationErr := types.ValidateLeaders(tc.leaders)
			require.Error(t, validationErr)
			require.Contains(t, err.Error(), validationErr.Error())
			require.Equal(t, packed, ctx.KVStore(key).Get(types.LeaderListKey))
		})
	}

	for _, length := range []int{1, 31, 33} {
		k, ctx, key := newActivationKeeper(t)
		ctx.KVStore(key).Set(types.LeaderListKey, bytes.Repeat([]byte{1}, length))
		leaders, err := k.Leaders(ctx)
		require.ErrorIs(t, err, keeper.ErrCorruptStore)
		require.Nil(t, leaders)
	}

	t.Run("unset and valid lists", func(t *testing.T) {
		k, ctx, _ := newActivationKeeper(t)
		leaders, err := k.Leaders(ctx)
		require.NoError(t, err)
		require.Empty(t, leaders)
		want := [][32]byte{{0: 3, 31: 4}, {0: 1, 31: 2}}
		require.NoError(t, k.SetLeaders(ctx, want))
		leaders, err = k.Leaders(ctx)
		require.NoError(t, err)
		require.Equal(t, want, leaders)
	})
}
