package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta/lib/server"
)

// Compose the production offline client with an actual closeable transport.
// The concrete online client's missing Close interface is a separate wiring request.
type repairM1311Client struct {
	*rosetta.Client
	*grpc.ClientConn
}

func repairM1311Settings(t *testing.T, addr, network string) (server.Settings, *grpc.ClientConn) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	peer := grpc.NewServer()
	status := health.NewServer()
	status.SetServingStatus(network, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(peer, status)
	served := make(chan error, 1)
	go func() { served <- peer.Serve(listener) }()
	t.Cleanup(func() {
		peer.Stop()
		require.NoError(t, <-served)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	response, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{Service: network})
	require.NoError(t, err)
	require.Equal(t, healthpb.HealthCheckResponse_SERVING, response.Status)
	cdc, registry := rosetta.MakeCodec()
	client, err := rosetta.NewClient(&rosetta.Config{Codec: cdc, InterfaceRegistry: registry})
	require.NoError(t, err)
	return server.Settings{
		Network: &types.NetworkIdentifier{Blockchain: "paxeer", Network: network},
		Client:  &repairM1311Client{Client: client, ClientConn: conn},
		Listen:  addr,
		Offline: true,
	}, conn
}

func repairM1311Start(t *testing.T, srv server.Server, addr string) <-chan error {
	t.Helper()
	stopped := make(chan error, 1)
	go func() { stopped <- srv.Start() }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, srv.Shutdown(ctx))
	})
	require.Eventually(t, func() bool {
		select {
		case err := <-stopped:
			t.Fatalf("Rosetta stopped before becoming ready: %v", err)
		default:
		}
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			return false
		}
		require.NoError(t, conn.Close())
		return true
	}, 5*time.Second, 10*time.Millisecond)
	return stopped
}

func repairM1311WaitStopped(t *testing.T, stopped <-chan error) {
	t.Helper()
	select {
	case err := <-stopped:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Rosetta Start did not return after shutdown")
	}
}

func TestRepairM_1311_RestartRecovery(t *testing.T) {
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := reservation.Addr().String()
	require.NoError(t, reservation.Close())
	for cycle := 1; cycle <= 2; cycle++ {
		network := fmt.Sprintf("reopened-node-%d", cycle)
		settings, conn := repairM1311Settings(t, addr, network)
		srv, err := server.NewServer(settings)
		require.NoError(t, err)
		stopped := repairM1311Start(t, srv, addr)
		transport := &http.Transport{}
		client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
		response, err := client.Post("http://"+addr+"/network/list", "application/json", strings.NewReader(`{}`))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		var networks types.NetworkListResponse
		require.NoError(t, json.NewDecoder(response.Body).Decode(&networks))
		require.NoError(t, response.Body.Close())
		require.Equal(t, []*types.NetworkIdentifier{settings.Network}, networks.NetworkIdentifiers,
			"the reopened handler must expose the new node's network state")
		copyOfServer := srv
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		require.NoError(t, copyOfServer.Shutdown(ctx))
		cancel()
		repairM1311WaitStopped(t, stopped)
		require.Equal(t, connectivity.Shutdown, conn.GetState())
		_, err = healthpb.NewHealthClient(conn).Check(context.Background(), &healthpb.HealthCheckRequest{Service: network})
		require.Error(t, err, "the old node connection must no longer serve requests")
		require.NoError(t, srv.Shutdown(context.Background()), "shutdown through another copy is idempotent")
		_, err = client.Post("http://"+addr+"/network/list", "application/json", strings.NewReader(`{}`))
		require.Error(t, err, "shutdown must close the listener and idle HTTP connections")
		transport.CloseIdleConnections()
	}
}

func TestRepairM_1311_ShutdownBeforeStart(t *testing.T) {
	settings, conn := repairM1311Settings(t, "127.0.0.1:0", "stopped-before-start")
	srv, err := server.NewServer(settings)
	require.NoError(t, err)
	require.NoError(t, srv.Shutdown(context.Background()))
	require.Equal(t, connectivity.Shutdown, conn.GetState())
	stopped := make(chan error, 1)
	go func() { stopped <- srv.Start() }()
	repairM1311WaitStopped(t, stopped)
}

func TestRepairM_1311_StartupFailuresReleaseClient(t *testing.T) {
	t.Run("constructor", func(t *testing.T) {
		settings, conn := repairM1311Settings(t, "127.0.0.1:0", "invalid-network")
		settings.Network = &types.NetworkIdentifier{}
		_, err := server.NewServer(settings)
		require.ErrorContains(t, err, "cannot build asserter")
		require.Equal(t, connectivity.Shutdown, conn.GetState())
	})
	t.Run("listener", func(t *testing.T) {
		occupied, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer occupied.Close()
		settings, conn := repairM1311Settings(t, occupied.Addr().String(), "occupied-address")
		srv, err := server.NewServer(settings)
		require.NoError(t, err)
		require.Error(t, srv.Start())
		require.Equal(t, connectivity.Shutdown, conn.GetState())
		require.NoError(t, srv.Shutdown(context.Background()))
	})
}
