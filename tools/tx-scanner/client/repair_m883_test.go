package client

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/grpc/tmservice"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func repairM883Reset(t *testing.T) {
	t.Helper()
	if err := CloseGRPCClient(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := CloseGRPCClient(); err != nil {
			t.Error(err)
		}
	})
}

func repairM883Server(t *testing.T) (string, int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("scanner", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, healthServer)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		select {
		case err := <-served:
			if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("loopback server did not stop")
		}
	})
	addr := listener.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

func repairM883CheckServing(t *testing.T, conn *grpc.ClientConn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	response, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{Service: "scanner"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("health response: %v", response)
	}
}

func repairM883WaitStopped(t *testing.T, lifecycle *grpcConnectionLifecycle, conn *grpc.ClientConn) {
	t.Helper()
	select {
	case <-lifecycle.done:
	case <-time.After(time.Second):
		t.Fatal("connection lifecycle did not stop")
	}
	if conn.GetState() != connectivity.Shutdown {
		t.Fatalf("connection state after cleanup: %v", conn.GetState())
	}
	if lifecycle.ctx.Err() != context.Canceled {
		t.Fatalf("lifecycle context after cleanup: %v", lifecycle.ctx.Err())
	}
}

func TestRepairM_883_BoundedStartup(t *testing.T) {
	repairM883Reset(t)
	// A real TCP listener that never speaks HTTP/2 stalls the handshake.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	addr := listener.Addr().(*net.TCPAddr)
	started := time.Now()
	err = InitializeGRPCClient(addr.IP.String(), addr.Port)
	elapsed := time.Since(started)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("startup error: %v", err)
	}
	if elapsed < grpcDialTimeout || elapsed > grpcDialTimeout+time.Second {
		t.Fatalf("startup took %v, expected the %v deadline", elapsed, grpcDialTimeout)
	}
	if GrpcConn != nil || grpcLifecycle != nil {
		t.Fatal("failed dial published a connection or started a lifecycle")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started = time.Now()
	err = InitializeGRPCClientContext(ctx, addr.IP.String(), addr.Port)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > time.Second {
		t.Fatalf("caller deadline not honored: %v", err)
	}
	if GrpcConn != nil || grpcLifecycle != nil {
		t.Fatal("caller deadline left a connection or lifecycle")
	}
}

func TestRepairM_883_CancelStartupAndRetry(t *testing.T) {
	repairM883Reset(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	addr := listener.Addr().(*net.TCPAddr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	timer := time.AfterFunc(100*time.Millisecond, cancel)
	defer timer.Stop()
	started := time.Now()
	err = InitializeGRPCClientContext(ctx, addr.IP.String(), addr.Port)
	if !errors.Is(err, context.Canceled) || time.Since(started) > time.Second {
		t.Fatalf("startup cancellation not honored: %v", err)
	}
	if GrpcConn != nil || grpcLifecycle != nil {
		t.Fatal("canceled dial published a connection or lifecycle")
	}
	host, port := repairM883Server(t)
	if err := InitializeGRPCClient(host, port); err != nil {
		t.Fatal(err)
	}
	repairM883CheckServing(t, GrpcConn)
}

func TestRepairM_883_CommandCleanup(t *testing.T) {
	repairM883Reset(t)
	host, port := repairM883Server(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := InitializeGRPCClientContext(ctx, host, port); err != nil {
		t.Fatal(err)
	}
	conn, lifecycle := GrpcConn, grpcLifecycle
	repairM883CheckServing(t, conn)
	if err := InitializeGRPCClientContext(ctx, host, port); err != nil {
		t.Fatal(err)
	}
	if GrpcConn != conn || grpcLifecycle != lifecycle {
		t.Fatal("repeated initialization replaced the connection or created another lifecycle")
	}

	// The generated scanner clients must use this live transport as well.
	rpcCtx, cancelRPC := context.WithTimeout(ctx, time.Second)
	defer cancelRPC()
	_, err := GetTmServiceClient().GetLatestBlock(rpcCtx, &tmservice.GetLatestBlockRequest{})
	if status.Code(err) != codes.Unimplemented || !strings.Contains(err.Error(), "cosmos.base.tendermint.v1beta1.Service") {
		t.Fatalf("Tendermint client did not reach the loopback server: %v", err)
	}
	txClient := GetTxClient()
	_, err = txClient.GetTx(rpcCtx, &txtypes.GetTxRequest{Hash: "m883"})
	if status.Code(err) != codes.Unimplemented || !strings.Contains(err.Error(), "cosmos.tx.v1beta1.Service") {
		t.Fatalf("transaction client did not reach the loopback server: %v", err)
	}

	if err := CloseGRPCClient(); err != nil {
		t.Fatal(err)
	}
	repairM883WaitStopped(t, lifecycle, conn)
	if GrpcConn != nil || grpcLifecycle != nil {
		t.Fatal("command cleanup retained the active connection")
	}
	if ctx.Err() != nil {
		t.Fatal("connection cleanup canceled the command's parent context")
	}
	_, err = txClient.GetTx(rpcCtx, &txtypes.GetTxRequest{Hash: "m883"})
	if status.Code(err) != codes.Canceled {
		t.Fatalf("retained client after close: %v", err)
	}
	if err := CloseGRPCClient(); err != nil {
		t.Fatalf("repeated cleanup: %v", err)
	}
	if err := InitializeGRPCClient(host, port); err != nil {
		t.Fatal(err)
	}
	if GrpcConn == conn || grpcLifecycle == lifecycle {
		t.Fatal("initialization reused a closed connection or lifecycle")
	}
	repairM883CheckServing(t, GrpcConn)
}

func TestRepairM_883_CommandCancellation(t *testing.T) {
	repairM883Reset(t)
	host, port := repairM883Server(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := InitializeGRPCClientContext(ctx, host, port); err != nil {
		t.Fatal(err)
	}
	conn, lifecycle := GrpcConn, grpcLifecycle
	repairM883CheckServing(t, conn)
	cancel()
	repairM883WaitStopped(t, lifecycle, conn)
	if err := InitializeGRPCClientContext(ctx, host, port); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled command reused a connection: %v", err)
	}
	if err := InitializeGRPCClient(host, port); err != nil {
		t.Fatal(err)
	}
	if GrpcConn == conn || grpcLifecycle == lifecycle {
		t.Fatal("new command reused the canceled lifecycle")
	}
	repairM883CheckServing(t, GrpcConn)
}

func TestRepairM_883_ConnectionShutdown(t *testing.T) {
	repairM883Reset(t)
	host, port := repairM883Server(t)
	if err := InitializeGRPCClient(host, port); err != nil {
		t.Fatal(err)
	}
	conn, lifecycle := GrpcConn, grpcLifecycle
	repairM883CheckServing(t, conn)
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	repairM883WaitStopped(t, lifecycle, conn)
	if err := InitializeGRPCClient(host, port); err != nil {
		t.Fatal(err)
	}
	if GrpcConn == conn || grpcLifecycle == lifecycle {
		t.Fatal("initialization reused the shutdown connection")
	}
	repairM883CheckServing(t, GrpcConn)
}
