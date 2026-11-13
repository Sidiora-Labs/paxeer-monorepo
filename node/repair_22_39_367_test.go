package app

import (
	"context"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/admin"
	admintypes "github.com/Sidiora-Labs/Paxeer-X-Network/admin/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	dbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestRepair2239367CloseWithUnfinishedAdminRPC(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	server, err := admin.StartServer(address)
	require.NoError(t, err)
	t.Cleanup(server.Stop)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	connection, err := grpc.DialContext(ctx, address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	// Send the real unary RPC's headers but withhold its request body. The
	// server cannot finish receiving it until the client or server cancels it.
	stream, err := connection.NewStream(ctx, &grpc.StreamDesc{}, "/paxprotocol.paxchain.admin.v0.AdminService/ListLoggers")
	require.NoError(t, err)
	stack := func() string {
		buffer := make([]byte, 1<<20)
		return string(buffer[:runtime.Stack(buffer, true)])
	}
	require.Eventually(t, func() bool {
		return strings.Contains(stack(), "google.golang.org/grpc.(*Server).processUnaryRPC")
	}, 2*time.Second, time.Millisecond, "admin server must be waiting for the request body")

	cfg := dbconfig.DefaultReceiptStoreConfig()
	cfg.DBDirectory = t.TempDir()
	key := storetypes.NewKVStoreKey("evm")
	store, err := receipt.NewReceiptStore(cfg, key)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	app := &App{adminServer: server, receiptStore: store}
	done := make(chan error, 1)
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	go func() { done <- app.HandleClose() }()
	var blocked string
	require.Eventually(t, func() bool {
		blocked = stack()
		return strings.Contains(blocked, "google.golang.org/grpc.(*Server).GracefulStop")
	}, 2*time.Second, time.Millisecond, "close must attempt to drain the unfinished admin RPC")
	t.Logf("admin drain goroutine dump:\n%s", blocked)
	select {
	case err := <-done:
		t.Fatalf("close returned before the held RPC drained: %v", err)
	default:
	}
	// Receipt shutdown precedes the admin drain, including release of its
	// ownership lock. Reopening also proves the database worker has joined.
	reopened, err := receipt.NewReceiptStore(cfg, key)
	require.NoError(t, err)
	require.NoError(t, reopened.Close())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-deadline.C:
		t.Fatal("application close did not return within ten seconds with an unfinished admin RPC")
	}
	err = stream.RecvMsg(&admintypes.ListLoggersResponse{})
	require.Equal(t, codes.Unavailable, status.Code(err), "server shutdown must cancel the held RPC without client cancellation")
}
