package keeper_test

import (
	"fmt"
	"math/bits"
	"testing"

	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/types"
	"github.com/stretchr/testify/require"
)

func seedRepairM599Window(t *testing.T, s *repair284Store, window, index int64) (cryptotypes.PubKey, []bool) {
	t.Helper()
	params := s.k.GetParams(s.ctx)
	params.SignedBlocksWindow = window
	params.MinSignedPerWindow = sdk.ZeroDec()
	s.k.SetParams(s.ctx, params)
	pk := s.addValidator(t)
	expected := make([]bool, window)
	for i := range expected {
		expected[i] = i%2 == 1 || int64(i) == window-1
	}
	// Fill every slot, then advance to the requested next-write offset using
	// the same votes. Both the packed bitmap and counter come from production.
	for n := int64(0); n < window+index; n++ {
		s.vote(pk, !expected[n%window])
	}
	return pk, expected
}

func expandedRepairM599Window(original []bool, index, window int64) []bool {
	expected := make([]bool, window)
	for i, missed := range original {
		destination := int64(i)
		if destination >= index {
			destination += window - int64(len(original))
		}
		expected[destination] = missed
	}
	return expected
}

func assertRepairM599Bitmap(t *testing.T, s *repair284Store, missed types.ValidatorMissedBlockArray, info types.ValidatorSigningInfo, expected []bool, index int64) {
	t.Helper()
	require.Equal(t, info.Address, missed.Address)
	require.Equal(t, int64(len(expected)), missed.WindowSize)
	require.Len(t, missed.MissedBlocks, (len(expected)+63)/64)
	var count int64
	for i, want := range expected {
		// Inspect the persisted representation independently of the keeper's
		// pack/unpack helpers so paired encoder/decoder bugs cannot hide a bit.
		actual := missed.MissedBlocks[i/64]&(uint64(1)<<uint(i%64)) != 0
		require.Equal(t, want, actual, "bit %d", i)
		require.Equal(t, want, s.k.GetBooleanFromBitGroups(missed.MissedBlocks, int64(i)), "keeper bit %d", i)
		if want {
			count++
		}
	}
	var population int64
	for _, word := range missed.MissedBlocks {
		population += int64(bits.OnesCount64(word))
	}
	require.Equal(t, count, population, "all set bits, including unused padding")
	require.Equal(t, population, info.MissedBlocksCounter)
	require.Equal(t, index, info.IndexOffset)
	require.Equal(t, expected, s.k.ParseBitGroupsToBoolArray(missed.MissedBlocks, missed.WindowSize))
}

func assertRepairM599StoredWindow(t *testing.T, s *repair284Store, pk cryptotypes.PubKey, expected []bool, index int64) {
	t.Helper()
	addr := sdk.ConsAddress(pk.Address())
	missed, found := s.k.GetValidatorMissedBlocks(s.ctx, addr)
	require.True(t, found)
	info, found := s.k.GetValidatorSigningInfo(s.ctx, addr)
	require.True(t, found)
	require.Equal(t, addr.String(), missed.Address)
	assertRepairM599Bitmap(t, s, missed, info, expected, index)
}

func TestRepairM_599_ResizeFixture(t *testing.T) {
	TestResizeMissedBlockArray(t)
}

func TestRepairM_599_BeginBlockResizeReload(t *testing.T) {
	for _, oldWindow := range []int64{1, 10, 64, 65} {
		for _, signed := range []bool{true, false} {
			t.Run(fmt.Sprintf("window_%d_signed_%t", oldWindow, signed), func(t *testing.T) {
				dir := t.TempDir()
				s := openRepair284Store(t, dir)
				defer func() { require.NoError(t, s.cms.Close()) }()
				index := oldWindow - 1
				pk, original := seedRepairM599Window(t, s, oldWindow, index)
				assertRepairM599StoredWindow(t, s, pk, original, index)
				before, found := s.staking.GetValidatorByConsAddr(s.ctx, sdk.ConsAddress(pk.Address()))
				require.True(t, found)

				reopen := func() {
					t.Helper()
					height := s.ctx.BlockHeight()
					commit := s.cms.Commit(true)
					require.Greater(t, commit.Version, int64(0))
					require.NoError(t, s.cms.Close())
					s = openRepair284Store(t, dir)
					require.Equal(t, commit, s.cms.LastCommitID())
					s.ctx = s.ctx.WithBlockHeight(height)
				}
				reopen()
				assertRepairM599StoredWindow(t, s, pk, original, index)

				window := oldWindow + 65
				params := s.k.GetParams(s.ctx)
				params.SignedBlocksWindow = window
				s.k.SetParams(s.ctx, params)
				expected := expandedRepairM599Window(original, index, window)
				require.True(t, expected[window-1], "the old final missed vote must survive")
				s.vote(pk, signed)
				expected[index] = !signed
				index = (index + 1) % window
				assertRepairM599StoredWindow(t, s, pk, expected, index)

				reopen()
				require.Equal(t, params, s.k.GetParams(s.ctx))
				assertRepairM599StoredWindow(t, s, pk, expected, index)
				// Continue through a complete ring after reopening, overwriting
				// both retained misses and inserted slots across word boundaries.
				for n := int64(0); n < window; n++ {
					missed := n%3 == 0
					s.vote(pk, !missed)
					expected[index] = missed
					index = (index + 1) % window
					assertRepairM599StoredWindow(t, s, pk, expected, index)
				}
				after, found := s.staking.GetValidatorByConsAddr(s.ctx, sdk.ConsAddress(pk.Address()))
				require.True(t, found)
				require.False(t, after.Jailed)
				require.Equal(t, before.Tokens, after.Tokens)
				reopen()
				assertRepairM599StoredWindow(t, s, pk, expected, index)
			})
		}
	}
}
