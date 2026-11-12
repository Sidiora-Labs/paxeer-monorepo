package internaltest

import (
	"net"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/cleanup"
)

// FreePorts chooses distinct loopback ports while holding every listener open.
func FreePorts(t *testing.T, count int) []int {
	t.Helper()
	listeners := make([]net.Listener, count)
	for i := range listeners {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("reserve loopback port: %v", err)
		}
		defer cleanup.Close(listener)
		listeners[i] = listener
	}
	ports := make([]int, count)
	for i, listener := range listeners {
		ports[i] = listener.Addr().(*net.TCPAddr).Port
	}
	return ports
}
