package evmrpc_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func TestEthclientChainIDAndNetworkID(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	client, err := ethclient.DialContext(ctx, fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	t.Cleanup(client.Close)

	expected := EVMKeeper.ChainID(Ctx)
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	require.Equal(t, expected, chainID)

	networkID, err := client.NetworkID(ctx)
	require.NoError(t, err)
	require.Equal(t, expected, networkID)
}

func TestEthclientSyncProgress(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	client, err := ethclient.DialContext(ctx, fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	t.Cleanup(client.Close)

	var syncing bool
	require.NoError(t, client.Client().CallContext(ctx, &syncing, "eth_syncing"))
	require.False(t, syncing)

	progress, err := client.SyncProgress(ctx)
	require.NoError(t, err)
	require.Nil(t, progress)
}
