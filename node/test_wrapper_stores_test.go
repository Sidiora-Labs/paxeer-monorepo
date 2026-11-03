package app_test

import (
	"testing"
	"time"

	layerxchainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"
)

func TestTestWrapperMountsRpcStores(t *testing.T) {
	w := app.NewTestWrapper(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), false)
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })

	mounted := w.Ctx.MultiStore().StoreKeys()
	for _, name := range []string{seqtypes.StoreKey, layerxchainstypes.StoreKey} {
		key := w.App.GetKey(name)
		require.NotNil(t, key, name)
		require.NotNil(t, w.App.CommitMultiStore().GetCommitKVStore(key), name)
		count := 0
		for _, mountedKey := range mounted {
			if mountedKey.Name() == name {
				require.Same(t, key, mountedKey, name)
				count++
			}
		}
		require.Equal(t, 1, count, name)
	}

	require.NotPanics(t, func() {
		keys, err := w.App.SequencerKeeper.ReceiptKeys(w.Ctx)
		require.NoError(t, err)
		require.Empty(t, keys)
		lastSeq, err := w.App.SequencerKeeper.LastSeq(w.Ctx)
		require.NoError(t, err)
		require.Zero(t, lastSeq)
		lastHash, err := w.App.SequencerKeeper.LastSeqHash(w.Ctx)
		require.NoError(t, err)
		require.Equal(t, [seqtypes.HashLength]byte{}, lastHash)
	})

	require.NotNil(t, w.App.LayerXChainsKeeper)
	require.NotPanics(t, func() {
		require.Equal(t, layerxchainstypes.DefaultParams(), w.App.LayerXChainsKeeper.GetParams(w.Ctx))
		domain, found := w.App.LayerXChainsKeeper.GetDomain(w.Ctx, 1)
		require.False(t, found)
		require.Equal(t, layerxchainstypes.Domain{}, domain)
		require.Empty(t, w.App.LayerXChainsKeeper.DomainTxs(w.Ctx, 1, uint64(w.Ctx.BlockHeight())))
	})
}
