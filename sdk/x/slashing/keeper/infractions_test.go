package keeper_test

import (
	"fmt"
	"testing"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestResizeMissedBlockArray(t *testing.T) {
	for _, tc := range []struct {
		window int64
		index  int64
	}{
		{1, 0}, {10, 0}, {10, 5}, {10, 9},
		{65, 0}, {65, 32}, {65, 64},
	} {
		t.Run(fmt.Sprintf("window_%d_index_%d", tc.window, tc.index), func(t *testing.T) {
			s := openRepair284Store(t, t.TempDir())
			defer func() { require.NoError(t, s.cms.Close()) }()
			pk, original := seedRepairM599Window(t, s, tc.window, tc.index)
			addr := sdk.ConsAddress(pk.Address())
			missed, found := s.k.GetValidatorMissedBlocks(s.ctx, addr)
			require.True(t, found)
			info, found := s.k.GetValidatorSigningInfo(s.ctx, addr)
			require.True(t, found)
			assertRepairM599Bitmap(t, s, missed, info, original, tc.index)

			window := tc.window * 2
			expected := expandedRepairM599Window(original, tc.index, window)
			resized, signInfo, index := s.k.ResizeMissedBlockArray(missed, info, window, tc.index)
			assertRepairM599Bitmap(t, s, resized, signInfo, expected, tc.index)
			require.Equal(t, info, signInfo)
			require.Equal(t, tc.index, index)

			shrinkWindow := (tc.window + 1) / 2
			resized, signInfo, index = s.k.ResizeMissedBlockArray(resized, signInfo, shrinkWindow, index)
			assertRepairM599Bitmap(t, s, resized, signInfo, make([]bool, shrinkWindow), 0)
			expectedInfo := info
			expectedInfo.MissedBlocksCounter = 0
			expectedInfo.IndexOffset = 0
			require.Equal(t, expectedInfo, signInfo)
			require.Zero(t, index)

			// A resize only returns pending state; persistence belongs to BeginBlocker.
			stored, found := s.k.GetValidatorMissedBlocks(s.ctx, addr)
			require.True(t, found)
			storedInfo, found := s.k.GetValidatorSigningInfo(s.ctx, addr)
			require.True(t, found)
			require.Equal(t, missed, stored)
			require.Equal(t, info, storedInfo)
		})
	}
}
