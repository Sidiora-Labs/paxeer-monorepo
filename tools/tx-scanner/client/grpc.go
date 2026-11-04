package client

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/grpc/tmservice"

	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

const grpcDialTimeout = 5 * time.Second

var GrpcConn *grpc.ClientConn

var (
	grpcMu        sync.Mutex
	grpcLifecycle *grpcConnectionLifecycle
)

type grpcConnectionLifecycle struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

func InitializeGRPCClient(targetEndpoint string, port int) error {
	return InitializeGRPCClientContext(context.Background(), targetEndpoint, port)
}

// InitializeGRPCClientContext binds the connection to the command context.
// The command must handle the returned error and defer CloseGRPCClient.
func InitializeGRPCClientContext(ctx context.Context, targetEndpoint string, port int) error {
	grpcMu.Lock()
	defer grpcMu.Unlock()

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("initialize gRPC client: %w", err)
	}
	if grpcLifecycle != nil {
		if grpcLifecycle.ctx.Err() == nil && GrpcConn.GetState() != connectivity.Shutdown {
			return nil
		}
		if err := closeGRPCClientLocked(); err != nil {
			return err
		}
	}

	address := net.JoinHostPort(targetEndpoint, strconv.Itoa(port))
	dialCtx, cancelDial := context.WithTimeout(ctx, grpcDialTimeout)
	defer cancelDial()
	conn, err := grpc.DialContext(dialCtx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(20*1024*1024),
			grpc.MaxCallSendMsgSize(20*1024*1024)),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("connect to gRPC server %s: %w", address, err)
	}
	if err := ctx.Err(); err != nil {
		conn.Close()
		return fmt.Errorf("initialize gRPC client: %w", err)
	}

	lifecycleCtx, cancel := context.WithCancel(ctx)
	lifecycle := &grpcConnectionLifecycle{ctx: lifecycleCtx, cancel: cancel, done: make(chan struct{})}
	GrpcConn = conn
	grpcLifecycle = lifecycle
	go lifecycle.run(conn)
	return nil
}

func (l *grpcConnectionLifecycle) run(conn *grpc.ClientConn) {
	defer close(l.done)
	defer l.cancel()
	defer func() {
		if conn.GetState() != connectivity.Shutdown {
			l.err = conn.Close()
		}
	}()
	for l.ctx.Err() == nil {
		state := conn.GetState()
		if state == connectivity.Shutdown || !conn.WaitForStateChange(l.ctx, state) {
			return
		}
	}
}

// CloseGRPCClient cancels the connection lifecycle and waits for cleanup.
func CloseGRPCClient() error {
	grpcMu.Lock()
	defer grpcMu.Unlock()
	return closeGRPCClientLocked()
}

func closeGRPCClientLocked() error {
	if grpcLifecycle == nil {
		return nil
	}
	grpcLifecycle.cancel()
	<-grpcLifecycle.done
	err := grpcLifecycle.err
	grpcLifecycle = nil
	GrpcConn = nil
	return err
}

func GetTmServiceClient() tmservice.ServiceClient {
	grpcMu.Lock()
	defer grpcMu.Unlock()
	return tmservice.NewServiceClient(GrpcConn)
}

func GetTxClient() txtypes.ServiceClient {
	grpcMu.Lock()
	defer grpcMu.Unlock()
	return txtypes.NewServiceClient(GrpcConn)
}
