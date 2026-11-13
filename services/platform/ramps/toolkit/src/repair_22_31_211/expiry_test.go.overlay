package keeper_test

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/stretchr/testify/require"
)

func TestRepairRampExpiry2231211(t *testing.T) {
	data, err := hex.DecodeString(os.Getenv("RAMP_REPAIR_CALLDATA"))
	require.NoError(t, err)
	require.Len(t, data, 196)
	expiry := binary.BigEndian.Uint64(data[156:164])
	require.Equal(t, uint64(1800000100000), expiry)
	for _, at := range []uint64{expiry - 1, expiry} {
		f, actor, args := payExactFixture(t)
		// Authorization and funds come from the real native keeper fixture;
		// expiry is consumed unchanged from the ramp compiler's ABI word.
		args.ExpiresAt = expiry
		ctx := f.ctx.WithBlockTime(time.UnixMilli(int64(at)))
		before, err := f.keeper.BalanceOf(ctx, args.From, args.Asset)
		require.NoError(t, err)
		result, err := f.keeper.PayExact(ctx, actor, args)
		after, readErr := f.keeper.BalanceOf(ctx, args.From, args.Asset)
		require.NoError(t, readErr)
		if at < expiry {
			require.NoError(t, err)
			require.Zero(t, result.Receipt.ResultCode)
			require.True(t, after.Equal(before.Sub(args.Amount)))
		} else {
			require.ErrorIs(t, err, types.ErrExpired)
			require.True(t, after.Equal(before))
		}
	}
}
