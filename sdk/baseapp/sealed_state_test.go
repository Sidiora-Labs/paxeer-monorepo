package baseapp

import (
	"context"
	"fmt"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
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

func TestCommitSealedRetention(t *testing.T) {
	for _, deliverAge := range []int64{0, 1, 10} {
		t.Run(fmt.Sprintf("deliver-evidence-%d", deliverAge), func(t *testing.T) {
			app := NewBaseApp(t.Name(), dbm.NewMemDB(), nil, nil, &testutil.TestAppOpts{},
				SetPruning(sdk.PruningOptions{KeepRecent: 20, KeepEvery: 4, Interval: 1}))
			key := sdk.NewKVStoreKey("sealed")
			paramsKey := sdk.NewKVStoreKey("params")
			transientKey := sdk.NewTransientStoreKey("transient_params")
			app.MountStores(key, paramsKey, transientKey)
			amino := codec.NewLegacyAmino()
			params := paramtypes.NewSubspace(codec.NewProtoCodec(codectypes.NewInterfaceRegistry()), amino,
				paramsKey, transientKey, Paramspace).WithKeyTable(paramtypes.NewKeyTable(
				paramtypes.NewParamSetPair(ParamStoreKeyEvidenceParams, &tmproto.EvidenceParams{}, ValidateEvidenceParams)))
			app.SetParamStore(params)
			var wantHeader tmproto.Header
			var wantValue []byte
			app.SetPreCommitHandler(func(ctx sdk.Context) error {
				require.Equal(t, wantHeader, ctx.BlockHeader())
				require.Equal(t, wantValue, ctx.KVStore(key).Get([]byte("block")))
				require.Equal(t, int64(5), app.GetConsensusParams(ctx).Evidence.MaxAgeNumBlocks)
				return nil
			})
			require.NoError(t, app.LoadLatestVersion())
			defer func() { require.NoError(t, app.Close()) }()
			for height := int64(1); height <= 12; height++ {
				if height == 12 {
					SetMinRetainBlocks(2)(app)
				}
				if deliverAge != 0 {
					app.setDeliverState(tmproto.Header{Height: height + 100, ChainID: "divergent"})
					params.Set(app.deliverState.ctx, ParamStoreKeyEvidenceParams, tmproto.EvidenceParams{
						MaxAgeNumBlocks: deliverAge, MaxAgeDuration: time.Hour})
					app.deliverState.ctx.KVStore(key).Set([]byte("block"), []byte("deliver"))
				} else {
					require.Nil(t, app.deliverState)
				}
				wantHeader = tmproto.Header{ChainID: app.ChainID, Height: height, Time: time.Unix(1234+height, 0).UTC()}
				wantValue = []byte(fmt.Sprintf("sealed-%d", height))
				ms := app.cms.CacheMultiStore()
				ctx := sdk.NewContext(ms, wantHeader, false)
				params.Set(ctx, ParamStoreKeyEvidenceParams, tmproto.EvidenceParams{
					MaxAgeNumBlocks: 5, MaxAgeDuration: time.Hour})
				ctx.KVStore(key).Set([]byte("block"), wantValue)
				require.NoError(t, app.SetSealedStateToCommit(ctx, ms))
				var res *abci.ResponseCommit
				var err error
				require.NotPanics(t, func() { res, err = app.Commit(context.Background()) })
				require.NoError(t, err)
				require.NotNil(t, res)
				wantRetention := int64(0)
				if height == 12 {
					wantRetention = 7
				}
				require.Equal(t, wantRetention, res.RetainHeight)
				require.Equal(t, wantHeader, app.checkState.ctx.BlockHeader())
				require.Equal(t, height, app.LastBlockHeight())
				require.Equal(t, wantValue, app.cms.GetKVStore(key).Get([]byte("block")))
				require.Equal(t, int64(5), app.GetConsensusParams(app.checkState.ctx).Evidence.MaxAgeNumBlocks)
				require.Nil(t, app.stateToCommit)
				require.Nil(t, app.deliverState)
			}
		})
	}
}
