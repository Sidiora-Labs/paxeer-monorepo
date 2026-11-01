package app_test

import (
	"context"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func commitLeaderSourceBlock(t *testing.T, a *app.App, write func(sdk.Context)) {
	t.Helper()
	height := a.LastBlockHeight() + 1
	_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: "pax-test", Height: height},
	})
	require.NoError(t, err)
	write(a.GetContextForDeliverTx(nil))
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, height, a.LastBlockHeight())
}

func TestAppLeaderScheduleReadsKeeperList(t *testing.T) {
	w := app.NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a := w.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	const activation int64 = 2
	want := [][32]byte{{0: 3, 31: 4}, {0: 1, 31: 2}, {31: 5}}
	commitLeaderSourceBlock(t, a, func(ctx sdk.Context) {
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, activation))
		require.NoError(t, a.SequencerKeeper.SetLeaders(ctx, want))
	})
	for _, height := range []int64{activation, activation + 1, activation + 100} {
		gotActivation, leaders, err := a.LeaderSchedule(height)
		require.NoError(t, err)
		require.Equal(t, activation, gotActivation)
		require.Equal(t, want, leaders)
		ctx, err := a.CreateQueryContext(0, false)
		require.NoError(t, err)
		keeperActivation, keeperLeaders, err := a.SequencerKeeper.LeaderSchedule(ctx)
		require.NoError(t, err)
		require.Equal(t, keeperActivation, gotActivation)
		require.Equal(t, keeperLeaders, leaders)
	}

	require.NoError(t, a.SequencerKeeper.SetLeaders(a.GetCheckCtx(), [][32]byte{{9}}))
	_, leaders, err := a.LeaderSchedule(activation)
	require.NoError(t, err)
	require.Equal(t, want, leaders)

	want = [][32]byte{{31: 6}, {31: 7}}
	commitLeaderSourceBlock(t, a, func(ctx sdk.Context) {
		require.NoError(t, a.SequencerKeeper.SetLeaders(ctx, want))
	})
	gotActivation, leaders, err := a.LeaderSchedule(activation + 1)
	require.NoError(t, err)
	require.Equal(t, activation, gotActivation)
	require.Equal(t, want, leaders)

	for _, key := range [][]byte{seqtypes.LeaderListKey, seqtypes.ActivationHeightKey} {
		commitLeaderSourceBlock(t, a, func(ctx sdk.Context) {
			ctx.KVStore(a.GetKey(seqtypes.StoreKey)).Set(key, []byte{1})
		})
		ctx, err := a.CreateQueryContext(0, false)
		require.NoError(t, err)
		_, _, keeperErr := a.SequencerKeeper.LeaderSchedule(ctx)
		require.ErrorIs(t, keeperErr, seqkeeper.ErrCorruptStore)
		gotActivation, leaders, err := a.LeaderSchedule(a.LastBlockHeight() + 1)
		require.ErrorIs(t, err, seqkeeper.ErrCorruptStore)
		require.EqualError(t, err, keeperErr.Error())
		require.Zero(t, gotActivation)
		require.Nil(t, leaders)
	}
}

func TestAppLeaderScheduleEmptyBeforeActivation(t *testing.T) {
	w := app.NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a := w.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	commitLeaderSourceBlock(t, a, func(ctx sdk.Context) {
		require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, app.MergeUpgrade))
		ctx.KVStore(a.GetKey(seqtypes.StoreKey)).Delete(seqtypes.ActivationHeightKey)
		require.NoError(t, a.SequencerKeeper.SetLeaders(ctx, [][32]byte{{1}, {2}}))
	})
	for _, height := range []int64{1, 2, 1000} {
		activation, leaders, err := a.LeaderSchedule(height)
		require.NoError(t, err)
		require.Zero(t, activation)
		require.Empty(t, leaders)
	}

	require.NoError(t, a.SequencerKeeper.SetActivationHeight(a.GetCheckCtx(), 2))
	_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: "pax-test", Height: 2},
	})
	require.NoError(t, err)
	require.NoError(t, a.SequencerKeeper.SetActivationHeight(a.GetContextForDeliverTx(nil), 2))
	activation, leaders, err := a.LeaderSchedule(2)
	require.NoError(t, err)
	require.Zero(t, activation)
	require.Empty(t, leaders)
}
