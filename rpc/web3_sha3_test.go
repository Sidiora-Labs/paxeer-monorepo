package evmrpc_test

import (
	"context"
	"testing"
	"time"

	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestWeb3Sha3HashesInput(t *testing.T) {
	server := evmrpc.NewHTTPServer(rpc.DefaultHTTPTimeouts)
	t.Cleanup(server.Stop)
	require.NoError(t, server.EnableRPC([]rpc.API{
		{Namespace: "web3", Service: &evmrpc.Web3API{}},
	}, evmrpc.HTTPConfig{Modules: []string{"web3"}}))
	require.NoError(t, server.SetListenAddr("127.0.0.1", 0))
	require.NoError(t, server.Start())

	client, err := rpc.DialHTTP("http://" + server.ListenAddr())
	require.NoError(t, err)
	t.Cleanup(client.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result string
	require.NoError(t, client.CallContext(ctx, &result, "web3_sha3", "0x68656c6c6f20776f726c64"))
	require.Equal(t, "0x47173285a8d7341e5e972fc677286384f802f8ef42a5ec5f03bbfa254cb01fad", result)
}
