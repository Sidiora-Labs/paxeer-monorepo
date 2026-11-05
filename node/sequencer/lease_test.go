package sequencer_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer/wal"
	"github.com/stretchr/testify/require"
)

func TestLeaseHolderAdvancesEpochOnStart(t *testing.T) {
	for _, tc := range []struct {
		name  string
		epoch uint64
	}{
		{name: "replayed lease", epoch: 4},
		{name: "empty WAL", epoch: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			listener := listen(t)
			p := newPrimary(t, listener.Addr().String(), replicationTimeout)
			sb := newStandby(t, p)
			sb.serve(t, listener)
			if tc.epoch != 0 {
				_, err := p.repl.Release(ctx, &wal.Lease{
					Epoch: tc.epoch, KeyID: [32]byte{1},
					ExpiryMs: uint64(time.Now().Add(-time.Minute).UnixMilli()),
				})
				require.NoError(t, err)
			}
			replay, err := p.wal.Replay(appReplayer{exec: p.h.exec})
			require.NoError(t, err)
			if tc.epoch != 0 {
				require.NotNil(t, replay.Lease)
				require.Equal(t, tc.epoch, replay.Lease.Epoch)
			} else {
				require.Nil(t, replay.Lease)
			}
			const duration = 30 * time.Second
			keyID := [32]byte{2, 3, 4}
			holder, err := sequencer.NewLeaseHolder(sequencer.LeaseHolderConfig{
				Replicator: p.repl, Replay: replay, KeyID: keyID, LeaseDuration: duration,
			})
			require.NoError(t, err)
			require.Zero(t, holder.Epoch())
			require.True(t, holder.Expiry().IsZero())
			before, err := os.ReadFile(filepath.Join(sb.dir, "head.seg"))
			require.NoError(t, err)

			earliest := time.Now().Add(duration).UnixMilli()
			require.NoError(t, holder.Start(ctx))
			latest := time.Now().Add(duration).UnixMilli()
			require.Equal(t, tc.epoch+1, holder.Epoch())
			require.GreaterOrEqual(t, holder.Expiry().UnixMilli(), earliest)
			require.LessOrEqual(t, holder.Expiry().UnixMilli(), latest)
			want := &wal.Lease{
				Epoch: tc.epoch + 1, KeyID: keyID, ExpiryMs: uint64(holder.Expiry().UnixMilli()),
			}
			primaryReplay, err := p.wal.Replay(appReplayer{exec: p.h.exec})
			require.NoError(t, err)
			require.Equal(t, want, primaryReplay.Lease)
			standbyReplay, err := sb.wal.Replay(appReplayer{exec: sb.h.exec})
			require.NoError(t, err)
			require.Equal(t, want, standbyReplay.Lease)
			frame, err := wal.NewFrame(want)
			require.NoError(t, err)
			after, err := os.ReadFile(filepath.Join(sb.dir, "head.seg"))
			require.NoError(t, err)
			require.Equal(t, append(before, frame...), after)
			require.Equal(t, want.Epoch, p.repl.Epoch())
			require.Error(t, holder.Start(ctx))
			require.Equal(t, want.Epoch, holder.Epoch())
		})
	}
}

func TestLeaseHolderStartFencedByHigherEpoch(t *testing.T) {
	ctx := context.Background()
	listener := listen(t)
	p := newPrimary(t, listener.Addr().String(), replicationTimeout)
	sb := newStandby(t, p)
	sb.serve(t, listener)
	_, err := p.repl.Release(ctx, &wal.Lease{Epoch: 4})
	require.NoError(t, err)
	replay, err := p.wal.Replay(appReplayer{exec: p.h.exec})
	require.NoError(t, err)
	require.NotNil(t, replay.Lease)
	require.Equal(t, uint64(4), replay.Lease.Epoch)
	held := &wal.Lease{Epoch: 6, KeyID: [32]byte{6}, ExpiryMs: uint64(time.Now().Add(time.Minute).UnixMilli())}
	require.NoError(t, sb.s.TakeLease(held))
	before, err := os.ReadFile(filepath.Join(sb.dir, "head.seg"))
	require.NoError(t, err)
	holder, err := sequencer.NewLeaseHolder(sequencer.LeaseHolderConfig{
		Replicator: p.repl, Replay: replay, KeyID: [32]byte{5}, LeaseDuration: 30 * time.Second,
	})
	require.NoError(t, err)

	require.ErrorIs(t, holder.Start(ctx), sequencer.ErrFenced)
	require.Zero(t, holder.Epoch())
	require.True(t, holder.Expiry().IsZero())
	primaryReplay, err := p.wal.Replay(appReplayer{exec: p.h.exec})
	require.NoError(t, err)
	require.NotNil(t, primaryReplay.Lease)
	require.Equal(t, uint64(5), primaryReplay.Lease.Epoch)
	standbyReplay, err := sb.wal.Replay(appReplayer{exec: sb.h.exec})
	require.NoError(t, err)
	require.Equal(t, held, standbyReplay.Lease)
	after, err := os.ReadFile(filepath.Join(sb.dir, "head.seg"))
	require.NoError(t, err)
	require.Equal(t, before, after)
	_, err = p.h.exec.Snapshot()
	require.ErrorIs(t, err, sequencer.ErrLayerNotOpen)
	require.NoError(t, sb.s.Err())
}
