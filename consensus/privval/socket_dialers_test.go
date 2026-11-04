package privval

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmnet "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/net"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
)

var (
	testSocketListenerKey = ed25519.TestSecretKey([]byte("privval listener fixture"))
	testSocketDialerKey   = ed25519.TestSecretKey([]byte("privval dialer fixture"))
)

// getFreeLocalhostAddrPort returns a free localhost:port address
func getFreeLocalhostAddrPort(t *testing.T) string {
	t.Helper()
	port, err := tmnet.GetFreePort()
	require.NoError(t, err)

	return fmt.Sprintf("127.0.0.1:%d", port)
}

func getDialerTestCases(t *testing.T) []dialerTestCase {
	tcpAddr := getFreeLocalhostAddrPort(t)
	unixFilePath, err := testUnixAddr(t)
	require.NoError(t, err)
	unixAddr := fmt.Sprintf("unix://%s", unixFilePath)

	return []dialerTestCase{
		{
			addr:   tcpAddr,
			dialer: DialTCPFn(tcpAddr, testTimeoutReadWrite, testSocketDialerKey, testSocketListenerKey.Public()),
		},
		{
			addr:   unixAddr,
			dialer: DialUnixFn(unixFilePath),
		},
	}
}

func TestIsConnTimeoutForFundamentalTimeouts(t *testing.T) {
	err := stalledHandshakeError(t)
	assert.True(t, IsConnTimeout(err))
}

func TestIsConnTimeoutForConnectionRefusal(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	conn, err := DialTCPFn(addr, time.Second, testSocketDialerKey, testSocketListenerKey.Public())()
	require.Error(t, err)
	require.Nil(t, conn)
	assert.False(t, IsConnTimeout(err))
}

// stalledHandshakeError accepts the TCP connection but never participates in
// SecretConnection, so the dialer's real handshake deadline must expire.
func stalledHandshakeError(t *testing.T) error {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	accepted := make(chan struct{})
	var dialErr error
	err = scope.Run(t.Context(), func(ctx context.Context, s scope.Scope) error {
		s.SpawnBg(func() error {
			conn, err := ln.Accept()
			if err != nil {
				return err
			}
			defer conn.Close()
			close(accepted)
			<-ctx.Done()
			return nil
		})
		conn, err := DialTCPFn(ln.Addr().String(), 50*time.Millisecond, testSocketDialerKey, testSocketListenerKey.Public())()
		dialErr = err
		if conn != nil {
			conn.Close()
		}
		select {
		case <-accepted:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	require.NoError(t, err)
	require.Error(t, dialErr)
	return dialErr
}

func TestIsConnTimeoutForWrappedConnTimeouts(t *testing.T) {
	err := stalledHandshakeError(t)
	err = fmt.Errorf("%v: %w", err, ErrConnectionTimeout)
	assert.True(t, IsConnTimeout(err))
}
