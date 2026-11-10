package grpc

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestRepair2239367StopIdleGRPCServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	t.Cleanup(server.Stop)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	done := make(chan bool, 1)
	go func() { done <- StopGRPCServer(server, 5*time.Second) }()
	select {
	case drained := <-done:
		require.True(t, drained, "idle server must drain without forced cancellation")
	case <-time.After(time.Second):
		t.Fatal("idle server shutdown waited for its grace deadline")
	}
	select {
	case err := <-served:
		require.True(t, err == nil || err == grpc.ErrServerStopped, "Serve must finish after shutdown: %v", err)
	case <-time.After(time.Second):
		t.Fatal("gRPC serving goroutine was not joined")
	}
	_, err = listener.Accept()
	require.ErrorIs(t, err, net.ErrClosed)
}
