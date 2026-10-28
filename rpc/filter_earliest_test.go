package evmrpc

import (
	"context"
	"math/big"
	"testing"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/pebbledb/mvcc"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestGetLogsFailsWhenEarliestHeightUnavailable(t *testing.T) {
	for _, entryPoint := range []string{"eth_getLogs", "GetLogsByFilters", "fetchBlocksByCrit"} {
		t.Run(entryPoint, func(t *testing.T) {
			stateStore, err := mvcc.OpenDB(t.TempDir(), config.DefaultStateStoreConfig())
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, stateStore.Close()) })
			require.Zero(t, stateStore.GetEarliestVersion())
			require.NoError(t, stateStore.SetLatestVersion(10))

			heightReads := 0
			var transitionErr error
			ctxProvider := func(int64) sdk.Context {
				heightReads++
				if heightReads == 2 {
					transitionErr = stateStore.SetLatestVersion(0)
				}
				return sdk.Context{}
			}
			watermarks := NewWatermarkManager(nil, ctxProvider, stateStore, nil)
			filterConfig := NewFilterConfig(0, DefaultMaxLogLimit, DefaultMaxBlockRange)
			fetcher := &LogFetcher{filterConfig: filterConfig, watermarks: watermarks}
			crit := filters.FilterCriteria{FromBlock: big.NewInt(0), ToBlock: big.NewInt(10)}
			ctx := context.Background()

			switch entryPoint {
			case "eth_getLogs":
				api := &FilterAPI{
					namespace:      "eth",
					connectionType: ConnectionTypeHTTP,
					filterConfig:   filterConfig,
					logFetcher:     fetcher,
				}
				server := rpc.NewServer()
				t.Cleanup(server.Stop)
				require.NoError(t, server.RegisterName("eth", api))
				client := rpc.DialInProc(server)
				t.Cleanup(client.Close)
				var logs []*FilterLog
				err = client.CallContext(ctx, &logs, "eth_getLogs", map[string]string{
					"fromBlock": "0x0",
					"toBlock":   "0xa",
				})
				require.EqualError(t, err, errNoHeightSource.Error())
				require.Nil(t, logs)
			case "GetLogsByFilters":
				logs, end, err := fetcher.GetLogsByFilters(ctx, crit, 0)
				require.ErrorIs(t, err, errNoHeightSource)
				require.Nil(t, logs)
				require.Zero(t, end)
			case "fetchBlocksByCrit":
				blocks, end, applyLimit, err := fetcher.fetchBlocksByCrit(ctx, crit, 0, nil)
				require.ErrorIs(t, err, errNoHeightSource)
				require.Nil(t, blocks)
				require.Zero(t, end)
				require.False(t, applyLimit)
			}

			require.NoError(t, transitionErr)
			require.Equal(t, 2, heightReads, "the latest lookup must succeed and the earliest lookup must fail immediately")
			require.Zero(t, stateStore.GetEarliestVersion())
		})
	}
}
