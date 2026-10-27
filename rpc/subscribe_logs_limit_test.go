package evmrpc

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestLogsSubscriptionLimit(t *testing.T) {
	newAPI := func(limit uint64) *SubscriptionAPI {
		return &SubscriptionAPI{
			subscriptonConfig: &SubscriptionConfig{logLimit: limit},
			logFetcher:        &LogFetcher{}, receiptLogs: NewReceiptLogFeed(1),
			connectionType: ConnectionTypeWS,
		}
	}
	live := func(api *SubscriptionAPI) int {
		api.logSubscriptionsMtx.Lock()
		defer api.logSubscriptionsMtx.Unlock()
		return len(api.logSubscriptions)
	}
	subscribe := func(client *rpc.Client, crit map[string]any) (*rpc.ClientSubscription, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return client.EthSubscribe(ctx, make(chan json.RawMessage, 1), "logs", crit)
	}

	t.Run("limit and unsubscribe", func(t *testing.T) {
		api := newAPI(1)
		client := dialLogsSubscription(t, api)
		otherClient := dialLogsSubscription(t, api)
		first, err := subscribe(client, nil)
		require.NoError(t, err)
		defer first.Unsubscribe()
		require.Equal(t, 1, live(api))
		second, err := subscribe(otherClient, nil)
		if second != nil {
			defer second.Unsubscribe()
		}
		require.EqualError(t, err, "no new subscription can be created")
		require.Nil(t, second)
		first.Unsubscribe()
		third, err := subscribe(otherClient, nil)
		require.NoError(t, err)
		defer third.Unsubscribe()
		require.Equal(t, 1, live(api))
		third.Unsubscribe()
		require.Eventually(t, func() bool { return live(api) == 0 }, time.Second, time.Millisecond)
	})

	t.Run("concurrent admission", func(t *testing.T) {
		api := newAPI(1)
		client := dialLogsSubscription(t, api)
		type result struct {
			sub *rpc.ClientSubscription
			err error
		}
		const callers = 8
		results := make(chan result, callers)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < callers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				sub, err := subscribe(client, nil)
				results <- result{sub, err}
			}()
		}
		close(start)
		wg.Wait()
		close(results)
		accepted := 0
		for res := range results {
			if res.sub != nil {
				defer res.sub.Unsubscribe()
			}
			if res.err == nil {
				accepted++
			} else {
				require.EqualError(t, res.err, "no new subscription can be created")
			}
		}
		require.Equal(t, 1, accepted)
		require.Equal(t, 1, live(api))
	})

	t.Run("zero is unbounded", func(t *testing.T) {
		api := newAPI(0)
		client := dialLogsSubscription(t, api)
		for i := 0; i < 3; i++ {
			sub, err := subscribe(client, nil)
			require.NoError(t, err)
			defer sub.Unsubscribe()
		}
		require.Equal(t, 3, live(api))
	})

	t.Run("disconnect releases slot", func(t *testing.T) {
		api := newAPI(1)
		client := dialLogsSubscription(t, api)
		sub, err := subscribe(client, nil)
		require.NoError(t, err)
		defer sub.Unsubscribe()
		client.Close()
		require.Eventually(t, func() bool { return live(api) == 0 }, time.Second, time.Millisecond)
		next, err := subscribe(dialLogsSubscription(t, api), nil)
		require.NoError(t, err)
		defer next.Unsubscribe()
	})

	for _, path := range []string{"receipt", "polling", "blockHash"} {
		t.Run(path+" exit releases slot", func(t *testing.T) {
			api := newAPI(1)
			crit := map[string]any{"toBlock": "0x1"}
			if path != "receipt" {
				api.receiptLogs = nil
				provider := func(int64) sdk.Context { return sdk.Context{}.WithBlockHeight(1) }
				api.logFetcher.watermarks = NewWatermarkManager(nil, provider, nil, nil)
				crit = map[string]any{"fromBlock": "0x1", "toBlock": "0x2"}
				if path == "blockHash" {
					api.logFetcher.watermarks = NewWatermarkManager(nil, nil, nil, nil)
					crit = map[string]any{"blockHash": common.HexToHash("0x1234")}
				}
			}
			client := dialLogsSubscription(t, api)
			first, err := subscribe(client, crit)
			require.NoError(t, err)
			defer first.Unsubscribe()
			require.Eventually(t, func() bool { return live(api) == 0 }, time.Second, time.Millisecond)
			second, err := subscribe(client, crit)
			require.NoError(t, err)
			defer second.Unsubscribe()
			require.Eventually(t, func() bool { return live(api) == 0 }, time.Second, time.Millisecond)
		})
	}
}
