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

func TestLeaseRenewedBeforeExpiry(t *testing.T) {
	listener := listen(t)
	p := newPrimary(t, listener.Addr().String(), replicationTimeout)
	sb := newStandby(t, p)
	sb.serve(t, listener)
	const duration = 10 * time.Second
	keyID := [32]byte{2, 3, 4}
	holder, err := sequencer.NewLeaseHolder(sequencer.LeaseHolderConfig{
		Replicator: p.repl, KeyID: keyID, LeaseDuration: duration,
	})
	require.NoError(t, err)
	require.NoError(t, holder.Start(context.Background()))
	epoch := holder.Epoch()
	require.NotZero(t, epoch)

	for _, elapsed := range []time.Duration{6 * time.Second, duration / 2} {
		previousExpiry := holder.Expiry()
		now := previousExpiry.Add(-duration).Add(elapsed)
		require.True(t, now.Before(previousExpiry))
		before, err := os.ReadFile(filepath.Join(sb.dir, "head.seg"))
		require.NoError(t, err)

		require.NoError(t, holder.RenewAt(now))
		want := &wal.Lease{
			Epoch: epoch, KeyID: keyID, ExpiryMs: uint64(now.Add(duration).UnixMilli()),
		}
		require.Equal(t, epoch, holder.Epoch())
		require.Equal(t, epoch, p.repl.Epoch())
		require.Equal(t, int64(want.ExpiryMs), holder.Expiry().UnixMilli())
		require.True(t, holder.Expiry().After(previousExpiry))
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
	}

	previousExpiry := holder.Expiry()
	held := &wal.Lease{Epoch: epoch + 1, KeyID: [32]byte{5}, ExpiryMs: uint64(previousExpiry.Add(duration).UnixMilli())}
	require.NoError(t, sb.s.TakeLease(held))
	before, err := os.ReadFile(filepath.Join(sb.dir, "head.seg"))
	require.NoError(t, err)
	require.ErrorIs(t, holder.RenewAt(previousExpiry.Add(-duration/2)), sequencer.ErrFenced)
	require.Equal(t, epoch, holder.Epoch())
	require.Equal(t, previousExpiry, holder.Expiry())
	after, err := os.ReadFile(filepath.Join(sb.dir, "head.seg"))
	require.NoError(t, err)
	require.Equal(t, before, after)
	standbyReplay, err := sb.wal.Replay(appReplayer{exec: sb.h.exec})
	require.NoError(t, err)
	require.Equal(t, held, standbyReplay.Lease)
}

func TestLeaseNotRenewedEarly(t *testing.T) {
	listener := listen(t)
	p := newPrimary(t, listener.Addr().String(), replicationTimeout)
	sb := newStandby(t, p)
	sb.serve(t, listener)
	const duration = 10 * time.Second
	keyID := [32]byte{2, 3, 4}
	holder, err := sequencer.NewLeaseHolder(sequencer.LeaseHolderConfig{
		Replicator: p.repl, KeyID: keyID, LeaseDuration: duration,
	})
	require.NoError(t, err)
	require.Error(t, holder.RenewAt(time.Now()))
	require.Zero(t, holder.Epoch())
	require.True(t, holder.Expiry().IsZero())
	primaryReplay, err := p.wal.Replay(appReplayer{exec: p.h.exec})
	require.NoError(t, err)
	require.Nil(t, primaryReplay.Lease)
	require.NoError(t, holder.Start(context.Background()))
	epoch, expiry := holder.Epoch(), holder.Expiry()
	want := &wal.Lease{Epoch: epoch, KeyID: keyID, ExpiryMs: uint64(expiry.UnixMilli())}
	before, err := os.ReadFile(filepath.Join(sb.dir, "head.seg"))
	require.NoError(t, err)

	for _, elapsed := range []time.Duration{-time.Second, 0, 2 * time.Second, duration/2 - time.Nanosecond} {
		require.NoError(t, holder.RenewAt(expiry.Add(-duration).Add(elapsed)))
		require.Equal(t, epoch, holder.Epoch())
		require.Equal(t, expiry, holder.Expiry())
		primaryReplay, err := p.wal.Replay(appReplayer{exec: p.h.exec})
		require.NoError(t, err)
		require.Equal(t, want, primaryReplay.Lease)
		standbyReplay, err := sb.wal.Replay(appReplayer{exec: sb.h.exec})
		require.NoError(t, err)
		require.Equal(t, want, standbyReplay.Lease)
		after, err := os.ReadFile(filepath.Join(sb.dir, "head.seg"))
		require.NoError(t, err)
		require.Equal(t, before, after)
	}
}
