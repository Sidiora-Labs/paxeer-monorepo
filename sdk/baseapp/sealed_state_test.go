package baseapp

import (
	"context"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestCommitWritesSealedState(t *testing.T) {
	db := dbm.NewMemDB()
	app := NewBaseApp(t.Name(), db, nil, nil, &testutil.TestAppOpts{})
	storeKey := sdk.NewKVStoreKey("sealed")
	app.MountStores(storeKey)
	header := tmproto.Header{ChainID: app.ChainID, Height: 1, Time: time.Unix(1234, 0).UTC()}
	key, value := []byte("block"), []byte("sealed value")
	preCommitKey, preCommitValue := []byte("precommit"), []byte("sealed precommit value")
	var sealedStore sdk.CacheMultiStore
	preCommitCalled := false
	app.SetPreCommitHandler(func(ctx sdk.Context) error {
		preCommitCalled = true
		require.Equal(t, header, ctx.BlockHeader())
		require.Equal(t, sealedStore, ctx.MultiStore())
		require.Same(t, sealedStore.GetKVStore(storeKey), ctx.MultiStore().GetKVStore(storeKey))
		require.Equal(t, value, ctx.KVStore(storeKey).Get(key))
		ctx.KVStore(storeKey).Set(preCommitKey, preCommitValue)
		return nil
	})
	require.NoError(t, app.LoadLatestVersion())
	app.setDeliverState(tmproto.Header{ChainID: app.ChainID, Height: 99})
	app.deliverState.ms.GetKVStore(storeKey).Set(key, []byte("deliver value"))
	app.SetDeliverStateToCommit()
	app.setProcessProposalState(tmproto.Header{ChainID: app.ChainID, Height: 100})
	app.processProposalState.ms.GetKVStore(storeKey).Set(key, []byte("proposal value"))

	sealedStore = app.cms.CacheMultiStore()
	sealedStore.GetKVStore(storeKey).Set(key, value)
	ctx := sdk.NewContext(app.cms, header, false)
	require.NoError(t, app.SetSealedStateToCommit(ctx, sealedStore))
	require.Equal(t, sealedStore, app.stateToCommit.MultiStore())
	require.Same(t, sealedStore.GetKVStore(storeKey), app.stateToCommit.MultiStore().GetKVStore(storeKey))
	require.Equal(t, sealedStore, app.stateToCommit.Context().MultiStore())
	require.Same(t, sealedStore.GetKVStore(storeKey), app.stateToCommit.Context().MultiStore().GetKVStore(storeKey))
	require.Same(t, app.cms, ctx.MultiStore())
	require.NotSame(t, app.deliverState, app.stateToCommit)
	require.NotSame(t, app.processProposalState, app.stateToCommit)
	require.Nil(t, app.cms.GetKVStore(storeKey).Get(key))

	res, err := app.Commit(context.Background())
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, preCommitCalled)
	require.Equal(t, header, app.checkState.Context().BlockHeader())
	require.Equal(t, int64(1), app.LastBlockHeight())
	require.Equal(t, value, app.cms.GetKVStore(storeKey).Get(key))
	require.Equal(t, preCommitValue, app.cms.GetKVStore(storeKey).Get(preCommitKey))
	require.Nil(t, app.stateToCommit)

	reloaded := NewBaseApp(t.Name(), db, nil, nil, &testutil.TestAppOpts{})
	reloaded.MountStores(storeKey)
	require.NoError(t, reloaded.LoadLatestVersion())
	require.Equal(t, app.LastCommitID(), reloaded.LastCommitID())
	require.Equal(t, value, reloaded.cms.GetKVStore(storeKey).Get(key))
	require.Equal(t, preCommitValue, reloaded.cms.GetKVStore(storeKey).Get(preCommitKey))
}

func TestSetSealedStateToCommitRefusesNilMultiStore(t *testing.T) {
	app := NewBaseApp(t.Name(), dbm.NewMemDB(), nil, nil, &testutil.TestAppOpts{})
	storeKey := sdk.NewKVStoreKey("sealed")
	app.MountStores(storeKey)
	require.NoError(t, app.LoadLatestVersion())
	ctx := sdk.NewContext(app.cms, tmproto.Header{ChainID: app.ChainID, Height: 1}, false)

	require.Error(t, app.SetSealedStateToCommit(ctx, nil))
	require.Nil(t, app.stateToCommit)
	require.PanicsWithValue(t, "no state to commit", func() {
		_, _ = app.Commit(context.Background())
	})

	app.setDeliverState(ctx.BlockHeader())
	app.SetDeliverStateToCommit()
	stateToCommit := app.stateToCommit
	key, value := []byte("block"), []byte("original value")
	stateToCommit.ms.GetKVStore(storeKey).Set(key, value)
	require.Error(t, app.SetSealedStateToCommit(ctx.WithBlockHeight(99), nil))
	require.Same(t, stateToCommit, app.stateToCommit)
	require.Equal(t, ctx.BlockHeader(), app.stateToCommit.Context().BlockHeader())
	_, err := app.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, value, app.cms.GetKVStore(storeKey).Get(key))
	require.Equal(t, ctx.BlockHeader(), app.checkState.Context().BlockHeader())
}
