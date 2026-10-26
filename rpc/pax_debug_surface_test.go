package evmrpc_test

import (
	"sync"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/legacyabci"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestPaxDebugServesOnlyExcludeTraceFail(t *testing.T) {
	api := evmrpc.NewPaxDebugAPI(
		nil, EVMKeeper, legacyabci.BeginBlockKeepers{},
		func(int64) sdk.Context { return Ctx },
		func(int64) client.TxConfig { return TxConfig },
		nil, nil, nil, evmrpc.ConnectionTypeHTTP, evmrpcconfig.DefaultConfig,
		evmrpc.NewBlockCache(8), &sync.Mutex{}, nil,
	)
	methods := evmrpc.ServedMethods([]rpc.API{{Namespace: "pax", Service: api}})

	require.Equal(t, []string{
		"pax_traceBlockByHashExcludeTraceFail",
		"pax_traceBlockByNumberExcludeTraceFail",
	}, methods)
}
