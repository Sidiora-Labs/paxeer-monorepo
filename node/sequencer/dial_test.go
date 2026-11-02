package sequencer_test

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/stretchr/testify/require"
)

func TestDialStandbyConnectsWithinTimeout(t *testing.T) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	cfg := sequencer.ReplicatorConfig{ReplicationTimeout: 2 * time.Second}
	cfg.Dial = sequencer.DialStandby(listener.Addr().String(), cfg.ReplicationTimeout)
	started := time.Now()
	conn, err := cfg.Dial(context.Background())
	elapsed := time.Since(started)
	require.NoError(t, err)
	require.NotNil(t, conn)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	require.Less(t, elapsed, cfg.ReplicationTimeout)
	require.IsType(t, &net.TCPConn{}, conn)
	require.Equal(t, listener.Addr().String(), conn.RemoteAddr().String())

	deadline := time.Now().Add(cfg.ReplicationTimeout)
	require.NoError(t, listener.SetDeadline(deadline))
	standby, err := listener.AcceptTCP()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, standby.Close()) })
	require.Equal(t, conn.LocalAddr().String(), standby.RemoteAddr().String())
	require.NoError(t, conn.SetDeadline(deadline))
	require.NoError(t, standby.SetDeadline(deadline))
	payload := []byte("standby replication link")
	n, err := conn.Write(payload)
	require.NoError(t, err)
	require.Equal(t, len(payload), n)
	received := make([]byte, len(payload))
	_, err = io.ReadFull(standby, received)
	require.NoError(t, err)
	require.Equal(t, payload, received)

	t.Run("configured timeout", func(t *testing.T) {
		conn, err := sequencer.DialStandby(listener.Addr().String(), time.Nanosecond)(context.Background())
		if conn != nil {
			t.Cleanup(func() { require.NoError(t, conn.Close()) })
		}
		require.Error(t, err)
		require.Nil(t, conn)
		var netErr net.Error
		require.ErrorAs(t, err, &netErr)
		require.True(t, netErr.Timeout())
	})

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		conn, err := cfg.Dial(ctx)
		if conn != nil {
			t.Cleanup(func() { require.NoError(t, conn.Close()) })
		}
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, conn)
	})

	for _, timeout := range []time.Duration{0, -time.Second} {
		t.Run("invalid timeout "+timeout.String(), func(t *testing.T) {
			conn, err := sequencer.DialStandby(listener.Addr().String(), timeout)(context.Background())
			if conn != nil {
				t.Cleanup(func() { require.NoError(t, conn.Close()) })
			}
			require.ErrorIs(t, err, sequencer.ErrConfigReplicationTimeout)
			require.Nil(t, conn)
		})
	}
}

func TestDialStandbyRefusesMalformedAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "localhost", "", "[::1]", "::1:1234", "[::1:1234"} {
		t.Run(address, func(t *testing.T) {
			conn, err := sequencer.DialStandby(address, time.Second)(context.Background())
			if conn != nil {
				t.Cleanup(func() { require.NoError(t, conn.Close()) })
			}
			require.True(t, errors.Is(err, sequencer.ErrConfigStandbyAddress), "got %v", err)
			require.Nil(t, conn)
		})
	}
}
