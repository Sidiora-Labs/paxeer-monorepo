package evmrpc_test

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http/httptest"
	"testing"
	"time"

	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/eth/gasprice"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func repair423Client(t *testing.T) *rpc.Client {
	t.Helper()
	// Empty history needs no chain state: the production handler must return
	// before consulting the consensus client, keeper or context provider.
	api := evmrpc.NewInfoAPI(nil, nil, nil, nil, "", 1024, evmrpc.ConnectionTypeHTTP, nil, nil)
	server := rpc.NewServer()
	t.Cleanup(server.Stop)
	require.NoError(t, server.RegisterName("eth", api))
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	client, err := rpc.DialHTTP(httpServer.URL)
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client
}

func TestRepair_423_ZeroCountHTTPQuantity(t *testing.T) {
	client := repair423Client(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	for _, count := range []any{"0x0", 0} {
		for _, tag := range []string{"latest", "earliest", "pending", "safe", "finalized", "0x1"} {
			for _, percentiles := range [][]float64{nil, {}, {0, 50, 100}} {
				var result json.RawMessage
				require.NoError(t, client.CallContext(ctx, &result, "eth_feeHistory", count, tag, percentiles))
				require.JSONEq(t, `{"oldestBlock":"0x0","gasUsedRatio":null}`, string(result),
					"count=%v tag=%s percentiles=%v", count, tag, percentiles)
				var decoded struct {
					OldestBlock string `json:"oldestBlock"`
				}
				require.NoError(t, json.Unmarshal(result, &decoded))
				oldest, err := hexutil.DecodeBig(decoded.OldestBlock)
				require.NoError(t, err, "oldestBlock must decode as a canonical quantity")
				require.Zero(t, oldest.Sign())
			}
		}
	}
}

func TestRepair_423_ZeroCountEthclientParity(t *testing.T) {
	client := ethclient.NewClient(repair423Client(t))
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	// Geth's production oracle also returns before using its backend for an
	// empty range, giving an offline reference without a substitute producer.
	oracle := new(gasprice.Oracle)
	for _, newest := range []*big.Int{nil, big.NewInt(0), big.NewInt(1)} {
		lastBlock := rpc.LatestBlockNumber
		if newest != nil {
			lastBlock = rpc.BlockNumber(newest.Int64())
		}
		for _, percentiles := range [][]float64{nil, {}, {0, 50, 100}} {
			oldest, rewards, baseFees, ratios, blobFees, blobRatios, err := oracle.FeeHistory(ctx, 0, lastBlock, percentiles)
			require.NoError(t, err)
			require.NotNil(t, oldest)
			require.Zero(t, oldest.Sign())
			require.Empty(t, rewards)
			require.Empty(t, baseFees)
			require.Nil(t, ratios)
			require.Empty(t, blobFees)
			require.Empty(t, blobRatios)

			history, err := client.FeeHistory(ctx, 0, newest, percentiles)
			require.NoError(t, err)
			require.NotNil(t, history)
			require.NotNil(t, history.OldestBlock, "typed client must receive a zero quantity, not nil")
			require.Zero(t, history.OldestBlock.Cmp(oldest), "decoded oldestBlock must equal geth's zero quantity")
			require.Len(t, history.Reward, len(rewards))
			require.Len(t, history.BaseFee, len(baseFees))
			require.Equal(t, ratios, history.GasUsedRatio)
		}
	}
}
