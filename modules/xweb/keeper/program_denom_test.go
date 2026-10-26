package keeper

import (
	"bytes"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdktestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestProgramFeeDenomKeyLayout(t *testing.T) {
	var asset types.Hash32
	for i := range asset {
		asset[i] = byte(i + 1)
	}
	key := ProgramFeeDenomKey(asset)
	require.Len(t, key, 44)
	require.Equal(t, []byte("web/feedenom"), key[:12])
	require.Equal(t, asset[:], key[12:])

	for _, prefix := range [][]byte{
		types.ParamsKey,
		types.AttestorSetKey,
		types.PausedKey,
		types.NonceKey,
		types.RequestPrefix,
		types.ResultPrefix,
		[]byte(programPendingPrefix),
		[]byte(programAnswerPrefix),
		programPendingCountKey,
		programCommittedCountKey,
	} {
		require.False(t, bytes.HasPrefix([]byte(programFeeDenomPrefix), prefix), "overlapping prefix %x", prefix)
		require.False(t, bytes.HasPrefix(prefix, []byte(programFeeDenomPrefix)), "overlapping prefix %x", prefix)
	}

	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	ctx := sdktestutil.DefaultContext(storeKey, sdk.NewTransientStoreKey("transient_xweb"))
	k := Keeper{storeKey: storeKey}
	denom, found := k.ProgramFeeDenom(ctx, asset)
	require.False(t, found)
	require.Empty(t, denom)

	k.setProgramFeeDenom(ctx, asset, types.ProgramFeeDenomPAX)
	require.Equal(t, []byte(types.ProgramFeeDenomPAX), ctx.KVStore(storeKey).Get(key))
	denom, found = k.ProgramFeeDenom(ctx, asset)
	require.True(t, found)
	require.Equal(t, types.ProgramFeeDenomPAX, denom)

	otherAsset := asset
	otherAsset[31]++
	denom, found = k.ProgramFeeDenom(ctx, otherAsset)
	require.False(t, found)
	require.Empty(t, denom)

	k.setProgramFeeDenom(ctx, otherAsset, types.ProgramFeeDenomSID)
	denom, found = k.ProgramFeeDenom(ctx, otherAsset)
	require.True(t, found)
	require.Equal(t, types.ProgramFeeDenomSID, denom)
	denom, found = k.ProgramFeeDenom(ctx, asset)
	require.True(t, found)
	require.Equal(t, types.ProgramFeeDenomPAX, denom)
}
