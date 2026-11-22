package sequencer_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRPCE2EPortOutsideLocalRange(t *testing.T) {
	low, high := rpcE2ELocalPortRange(t)
	for i := 0; i < 200; i++ {
		port := rpcE2EPort(t)
		require.Greater(t, port, 1023)
		require.LessOrEqual(t, port, 65535)
		require.True(t, port < low || port > high,
			"port %d lies in system local port range %d-%d", port, low, high)
	}
}

func TestRPCE2EPortUnique(t *testing.T) {
	seen := make(map[int]bool)
	for i := 0; i < 200; i++ {
		port := rpcE2EPort(t)
		require.False(t, seen[port], "port %d was handed out twice", port)
		seen[port] = true
	}
	require.Len(t, seen, 200)
}

func TestRPCE2EPortSkipsListener(t *testing.T) {
	low, high := rpcE2ELocalPortRange(t)
	listener := func() net.Listener {
		rpcE2EPorts.Lock()
		defer rpcE2EPorts.Unlock()
		for tried := 0; tried < rpcE2EPortCount; tried++ {
			port := rpcE2EPorts.next
			if (port < low || port > high) && !rpcE2EPorts.used[port] {
				l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
				if err == nil {
					return l
				}
			}
			rpcE2EPorts.next = rpcE2EMinPort + (port-rpcE2EMinPort+1)%rpcE2EPortCount
		}
		t.Fatal("no loopback port available for held-listener case")
		return nil
	}()
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	heldPort := listener.Addr().(*net.TCPAddr).Port
	require.NotEqual(t, heldPort, rpcE2EPort(t), "a held port must not be handed out")
}
