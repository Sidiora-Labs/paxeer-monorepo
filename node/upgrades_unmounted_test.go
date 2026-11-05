package app

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/stretchr/testify/require"
)

func TestUnmountedPlanStoresJudgesMergePlan(t *testing.T) {
	a := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false).App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	mounted := a.GetKey(keys.SequencerStoreKey)
	require.NotNil(t, mounted)
	require.NotNil(t, a.CommitMultiStore().GetCommitKVStore(mounted))
	require.Empty(t, a.unmountedPlanStores(MergeUpgrade))

	t.Run("unmounted store", func(t *testing.T) {
		a.keys[keys.SequencerStoreKey] = sdk.NewKVStoreKey(keys.SequencerStoreKey)
		t.Cleanup(func() { a.keys[keys.SequencerStoreKey] = mounted })
		require.Nil(t, a.CommitMultiStore().GetCommitKVStore(a.GetKey(keys.SequencerStoreKey)))
		require.Equal(t, []string{keys.SequencerStoreKey}, a.unmountedPlanStores(MergeUpgrade))
		require.Empty(t, a.unmountedPlanStores(ActivationUpgrade))
		require.Empty(t, a.unmountedPlanStores(V6120Upgrade))
		require.Nil(t, a.unmountedPlanStores(V610Upgrade))
		require.Nil(t, a.unmountedPlanStores("unknown"))
	})

	t.Run("missing store key", func(t *testing.T) {
		delete(a.keys, keys.SequencerStoreKey)
		t.Cleanup(func() { a.keys[keys.SequencerStoreKey] = mounted })
		require.Equal(t, []string{keys.SequencerStoreKey}, a.unmountedPlanStores(MergeUpgrade))
	})

	require.Empty(t, a.unmountedPlanStores(MergeUpgrade))
}
