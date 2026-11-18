package app_test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func commitLeaderSourceBlock(t *testing.T, a *app.App, write func(sdk.Context)) {
	t.Helper()
	commitProofNodeAbciSchedule(t, a, write)
	app.WaitForTestQueryHeight(t, a, a.LastBlockHeight())
}

func TestAppLeaderScheduleReadsKeeperList(t *testing.T) {
	w := app.NewTestWrapperWithSc(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a := w.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	require.NoError(t, a.SequencerKeeper.SetReceiptKeys(w.Ctx, seqkeeper.ReceiptKeys{{
		KeyId: sequencerKeyID, PublicKey: sequencerPublicKey(), FirstHeight: 1, LastHeight: ^uint64(0),
	}}))
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
		t.Run(string(key), func(t *testing.T) {
			w := app.NewTestWrapperWithSc(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), false)
			a := w.App
			t.Cleanup(func() { require.NoError(t, a.Close()) })
			require.NoError(t, a.SequencerKeeper.SetReceiptKeys(w.Ctx, seqkeeper.ReceiptKeys{{
				KeyId: sequencerKeyID, PublicKey: sequencerPublicKey(), FirstHeight: 1, LastHeight: ^uint64(0),
			}}))
			commitLeaderSourceBlock(t, a, func(ctx sdk.Context) {
				require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, activation))
				require.NoError(t, a.SequencerKeeper.SetLeaders(ctx, want))
			})
			commitLeaderSourceBlock(t, a, func(ctx sdk.Context) { ctx.KVStore(a.GetKey(seqtypes.StoreKey)).Set(key, []byte{1}) })
			ctx, err := a.CreateQueryContext(0, false)
			require.NoError(t, err)
			_, _, keeperErr := a.SequencerKeeper.LeaderSchedule(ctx)
			require.ErrorIs(t, keeperErr, seqkeeper.ErrCorruptStore)
			gotActivation, leaders, err := a.LeaderSchedule(a.LastBlockHeight() + 1)
			require.ErrorIs(t, err, seqkeeper.ErrCorruptStore)
			require.EqualError(t, err, keeperErr.Error())
			require.Zero(t, gotActivation)
			require.Nil(t, leaders)
		})
	}
}

func TestAppLeaderScheduleEmptyBeforeActivation(t *testing.T) {
	w := app.NewTestWrapperWithSc(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), false)
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

func TestAppLeaderScheduleImmediatelyAfterCommit(t *testing.T) {
	w := app.NewTestWrapperWithSc(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a := w.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })

	for i := int64(1); i <= 3; i++ {
		activation := 100 + i
		want := [][32]byte{{0: byte(i), 31: 1}, {0: byte(i), 31: 2}}
		commitProofNodeAbciSchedule(t, a, func(ctx sdk.Context) {
			ctx.KVStore(a.GetKey(seqtypes.StoreKey)).Delete(seqtypes.ActivationHeightKey)
			require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, activation))
			require.NoError(t, a.SequencerKeeper.SetLeaders(ctx, want))
		})
		gotActivation, leaders, err := a.LeaderSchedule(i + 1)
		require.NoError(t, err)
		require.Equal(t, activation, gotActivation)
		require.Equal(t, want, leaders)

		leaders[0][0] ^= 0xff
		_, unchanged, err := a.LeaderSchedule(i + 1)
		require.NoError(t, err)
		require.Equal(t, want, unchanged)
	}
}

func TestAppLeaderScheduleConcurrentCommits(t *testing.T) {
	w := app.NewTestWrapperWithSc(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a := w.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	const commits = 8
	schedules := make(map[int64][][32]byte, commits)
	for i := int64(1); i <= commits; i++ {
		schedules[100+i] = [][32]byte{{0: byte(i), 31: 1}, {0: byte(i), 31: 2}}
	}
	commitProofNodeAbciSchedule(t, a, func(ctx sdk.Context) {
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, 101))
		require.NoError(t, a.SequencerKeeper.SetLeaders(ctx, schedules[101]))
	})

	type observation struct {
		height  int64
		leaders [][32]byte
		err     error
	}
	type report struct {
		reads int
		err   error
	}
	request := make(chan struct{})
	samples := make(chan observation)
	stop := make(chan struct{})
	done := make(chan report, 1)
	go func() {
		var result report
		read := func() observation {
			height, leaders, err := a.LeaderSchedule(1000)
			result.reads++
			if result.err == nil {
				if err != nil {
					result.err = err
				} else if want, ok := schedules[height]; !ok || !slices.Equal(want, leaders) {
					result.err = fmt.Errorf("schedule at activation %d has leaders %X outside the committed schedules", height, leaders)
				}
			}
			return observation{height, leaders, err}
		}
		for {
			select {
			case <-stop:
				done <- result
				return
			case <-request:
				samples <- read()
			default:
				read()
				runtime.Gosched()
			}
		}
	}()
	t.Cleanup(func() {
		close(stop)
		result := <-done
		require.NoError(t, result.err)
		require.GreaterOrEqual(t, result.reads, 3*(commits-1))
	})
	readAt := func(height int64) {
		request <- struct{}{}
		got := <-samples
		require.NoError(t, got.err)
		require.Equal(t, height, got.height)
		require.Equal(t, schedules[height], got.leaders)
	}
	for i := int64(2); i <= commits; i++ {
		activation := 100 + i
		commitProofNodeAbciSchedule(t, a, func(ctx sdk.Context) {
			require.NoError(t, a.SequencerKeeper.SetLeaders(ctx, schedules[activation]))
			a.SetDeliverStateToCommit()
			a.WriteState()
			a.GetWorkingHash()
			readAt(activation - 1)
			ctx.KVStore(a.GetKey(seqtypes.StoreKey)).Delete(seqtypes.ActivationHeightKey)
			require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, activation))
			a.WriteState()
			a.GetWorkingHash()
			readAt(activation - 1)
		})
		readAt(activation)
	}
}

func TestAppLeaderScheduleWithoutCommittedBlock(t *testing.T) {
	w := app.NewTestWrapperWithSc(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a := w.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	require.Zero(t, a.LastBlockHeight())
	require.NoError(t, a.SequencerKeeper.SetActivationHeight(w.Ctx, 2))
	require.NoError(t, a.SequencerKeeper.SetLeaders(w.Ctx, [][32]byte{{1}, {2}}))
	height, leaders, err := a.LeaderSchedule(2)
	require.NoError(t, err)
	require.Zero(t, height)
	require.Nil(t, leaders)
}

func TestAppLeaderScheduleRestoresCommittedSchedule(t *testing.T) {
	w := app.NewTestWrapperWithSc(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a := w.App
	t.Cleanup(func() {
		if a != nil {
			require.NoError(t, a.Close())
		}
	})
	want := [][32]byte{{1}, {2}}
	commitProofNodeAbciSchedule(t, a, func(ctx sdk.Context) {
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, 100))
		require.NoError(t, a.SequencerKeeper.SetLeaders(ctx, want))
	})
	info, err := a.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	home := filepath.Dir(filepath.Dir(info))
	require.NoError(t, a.Close())
	a = nil
	a = app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, home, 1, false,
		tmconfig.TestConfig(), app.MakeEncodingConfig(), nil, app.TestAppOpts{UseSc: true}, app.EmptyWasmOpts, nil)
	require.EqualValues(t, 1, a.LastBlockHeight())
	height, leaders, err := a.LeaderSchedule(2)
	require.NoError(t, err)
	require.EqualValues(t, 100, height)
	require.Equal(t, want, leaders)
}
