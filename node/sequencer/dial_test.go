package sequencer_test

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/stretchr/testify/require"
)

func TestDialStandbyConnectsWithinTimeout(t *testing.T) {
	listener, auth := listenAuthenticated(t)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := listener.Accept()
		if err == nil {
			accepted <- c
		}
	}()
	cfg := sequencer.ReplicatorConfig{ReplicationTimeout: 2 * time.Second}
	cfg.Dial = sequencer.DialStandby(listener.Addr().String(), cfg.ReplicationTimeout, auth)
	received := make(chan []byte, 1)
	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		c := <-accepted
		defer c.Close()
		_ = c.SetDeadline(time.Now().Add(cfg.ReplicationTimeout))
		secured, ok := c.(*tls.Conn)
		if !ok {
			return
		}
		if secured.HandshakeContext(t.Context()) != nil {
			return
		}
		buf := make([]byte, len("standby replication link"))
		if _, err := io.ReadFull(c, buf); err == nil {
			received <- buf
		}
	}()
	started := time.Now()
	conn, err := cfg.Dial(t.Context())
	require.NoError(t, err)
	defer conn.Close()
	require.Less(t, time.Since(started), cfg.ReplicationTimeout)
	require.IsType(t, &tls.Conn{}, conn)
	require.Equal(t, listener.Addr().String(), conn.RemoteAddr().String())
	require.NoError(t, conn.SetDeadline(time.Now().Add(cfg.ReplicationTimeout)))
	payload := []byte("standby replication link")
	n, err := conn.Write(payload)
	require.NoError(t, err)
	require.Equal(t, len(payload), n)
	select {
	case got := <-received:
		require.Equal(t, payload, got)
	case <-time.After(cfg.ReplicationTimeout):
		t.Fatal("authenticated peer did not receive payload")
	}
	<-peerDone
	t.Run("configured timeout", func(t *testing.T) {
		c, err := sequencer.DialStandby(listener.Addr().String(), time.Nanosecond, auth)(t.Context())
		if c != nil {
			_ = c.Close()
		}
		require.Nil(t, c)
		require.Error(t, err)
		var networkError net.Error
		require.ErrorAs(t, err, &networkError)
		require.True(t, networkError.Timeout())
	})
	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		c, err := cfg.Dial(ctx)
		require.Nil(t, c)
		require.ErrorIs(t, err, context.Canceled)
	})
	for _, timeout := range []time.Duration{0, -time.Second} {
		t.Run("invalid timeout "+timeout.String(), func(t *testing.T) {
			c, err := sequencer.DialStandby(listener.Addr().String(), timeout, auth)(t.Context())
			require.Nil(t, c)
			require.ErrorIs(t, err, sequencer.ErrConfigReplicationTimeout)
		})
	}
	t.Run("missing trust", func(t *testing.T) {
		c, err := sequencer.DialStandby(listener.Addr().String(), time.Second)(t.Context())
		require.Nil(t, c)
		require.ErrorIs(t, err, sequencer.ErrConfigValue)
	})
}

func TestDialStandbyRefusesMalformedAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "localhost", "", "[::1]", "::1:1234", "[::1:1234"} {
		t.Run(address, func(t *testing.T) {
			conn, err := sequencer.DialStandby(address, time.Second)(t.Context())
			if conn != nil {
				_ = conn.Close()
			}
			require.True(t, errors.Is(err, sequencer.ErrConfigStandbyAddress), "got %v", err)
			require.Nil(t, conn)
		})
	}
}
