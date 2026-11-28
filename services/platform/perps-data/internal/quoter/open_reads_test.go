package quoter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestOpenReadsConfiguredSnapshotBound(t *testing.T) {
	for _, bound := range []int{1, 2, 8} {
		t.Run(fmt.Sprint(bound), func(t *testing.T) {
			r, n := newReadWaveRPC(t, bound)
			require.Equal(t, bound, r.maxOpenReads)
			n.capacity = 8
			for id := int64(1); id <= 4; id++ {
				n.ids = append(n.ids, big.NewInt(id))
				n.orders[fmt.Sprint(id)] = waveOrder(r.account, id, 0)
			}
			n.position = big.NewInt(99)
			release := make(chan struct{})
			n.gate = release
			var once sync.Once
			defer once.Do(func() { close(release) })
			r.wait = func(context.Context, time.Duration) error {
				t.Error("a snapshot within node capacity must not wait")
				return nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			type result struct {
				snapshot Snapshot
				err      error
			}
			done := make(chan result, 1)
			go func() { snapshot, err := r.Snapshot(ctx, 0); done <- result{snapshot, err} }()
			for i := 0; i < bound; i++ {
				select {
				case <-n.started:
				case <-ctx.Done():
					t.Fatal("snapshot did not fill its configured read bound")
				}
			}
			n.mu.Lock()
			active := n.active
			n.mu.Unlock()
			require.Equal(t, bound, active)
			once.Do(func() { close(release) })
			got := <-done
			require.NoError(t, got.err)
			require.Len(t, got.snapshot.Orders, 4)
			require.Equal(t, big.NewInt(-123), got.snapshot.PositionBase)
			require.Equal(t, new(big.Int).Mul(big.NewInt(1000), unit), got.snapshot.FreeUSD)
			n.mu.Lock()
			defer n.mu.Unlock()
			require.Equal(t, 17, n.requests)
			require.Len(t, n.calls, 14)
			require.Equal(t, bound, n.maximum)
			for _, call := range n.calls {
				require.Equal(t, "0xc", call.block)
			}
			require.Zero(t, r.BusyResponses())
			t.Logf("snapshot bound=%d requests=%d contract_reads=%d maximum_open=%d", bound, n.requests, len(n.calls), n.maximum)
		})
	}
}

func TestOpenReadsClientWithoutSettingUsesTwo(t *testing.T) {
	r, n := newReadWaveRPC(t)
	r.maxOpenReads = 0
	r.wait = func(context.Context, time.Duration) error { t.Error("default reads fit node capacity"); return nil }
	require.Equal(t, 2, r.openReadLimit())
	_, err := r.Snapshot(context.Background(), 0)
	require.NoError(t, err)
	n.mu.Lock()
	defer n.mu.Unlock()
	require.LessOrEqual(t, n.maximum, 2)
	require.Len(t, n.calls, 9)
}

func TestOpenReadsBusyAsksAgainEightTimes(t *testing.T) {
	r, n := newReadWaveRPC(t)
	n.busyRemaining = 8
	var waits []time.Duration
	r.wait = func(ctx context.Context, delay time.Duration) error {
		require.NoError(t, ctx.Err())
		waits = append(waits, delay)
		return nil
	}
	values, err := r.venue(context.Background(), big.NewInt(12), "getMarketConfig", big.NewInt(0))
	require.NoError(t, err)
	require.True(t, tuple[marketTuple](values).Enabled)
	require.Equal(t, uint64(8), r.BusyResponses())
	require.Equal(t, []time.Duration{50 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond, 1600 * time.Millisecond, 3200 * time.Millisecond, 6400 * time.Millisecond}, waits)
	n.mu.Lock()
	defer n.mu.Unlock()
	require.Len(t, n.calls, 9)
	for _, call := range n.calls {
		require.Equal(t, n.calls[0], call)
	}
}

func TestOpenReadsBusyExhaustionIsSanitized(t *testing.T) {
	r, n := newReadWaveRPC(t)
	n.busyRemaining = -1
	waits := 0
	r.wait = func(context.Context, time.Duration) error { waits++; return nil }
	values, err := r.venue(context.Background(), big.NewInt(12), "getMarketConfig", big.NewInt(0))
	require.Nil(t, values)
	require.EqualError(t, err, "chain request failed")
	require.False(t, isRefusal(err))
	require.Equal(t, "chain request failed", reason(err))
	require.Equal(t, 8, waits)
	require.Equal(t, uint64(9), r.BusyResponses())
	n.mu.Lock()
	defer n.mu.Unlock()
	require.Len(t, n.calls, 9)
	require.Equal(t, 9, n.busyAnswers)
}

func TestOpenReadsCancellationStopsBusyWait(t *testing.T) {
	r, n := newReadWaveRPC(t)
	n.busyRemaining = -1
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waits := 0
	r.wait = func(ctx context.Context, delay time.Duration) error {
		waits++
		cancel()
		return waitReadRetry(ctx, delay)
	}
	_, err := r.venue(ctx, big.NewInt(12), "getMarketConfig", big.NewInt(0))
	require.EqualError(t, err, "chain request failed")
	require.Equal(t, 1, waits)
	require.Equal(t, uint64(1), r.BusyResponses())
	n.mu.Lock()
	defer n.mu.Unlock()
	require.Len(t, n.calls, 1)
}

func TestOpenReadsOtherFailuresAreNotRetried(t *testing.T) {
	for _, message := range []string{"execution reverted", "server busy", "rate limit", "transport unavailable", ""} {
		t.Run(message, func(t *testing.T) {
			r, n := newReadWaveRPC(t)
			n.fail, n.errorText = "getMarketConfig", message
			r.wait = func(context.Context, time.Duration) error {
				t.Error("only the node rate limit response may retry")
				return nil
			}
			_, err := r.venue(context.Background(), big.NewInt(12), "getMarketConfig", big.NewInt(0))
			if message == "" {
				require.EqualError(t, err, "PerpsPaused")
			} else {
				require.EqualError(t, err, "chain request failed")
			}
			require.Zero(t, r.BusyResponses())
			n.mu.Lock()
			defer n.mu.Unlock()
			require.Len(t, n.calls, 1)
		})
	}
}

func TestOpenReadsBusyWaveRecoversAtTheSameBlock(t *testing.T) {
	r, n := newReadWaveRPC(t, 8)
	n.capacity = 4
	release := make(chan struct{})
	n.gate = release
	var once sync.Once
	defer once.Do(func() { close(release) })
	r.wait = func(context.Context, time.Duration) error { once.Do(func() { close(release) }); return nil }
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	snapshot, err := r.Snapshot(ctx, 0)
	require.NoError(t, err)
	require.True(t, snapshot.Market.Enabled)
	require.Positive(t, r.BusyResponses())
	n.mu.Lock()
	defer n.mu.Unlock()
	require.Equal(t, uint64(n.busyAnswers), r.BusyResponses())
	require.Len(t, n.calls, 9+n.busyAnswers)
	require.LessOrEqual(t, n.maximum, 8)
	for _, call := range n.calls {
		require.Equal(t, "0xc", call.block)
	}
}

func TestOpenReadsRoundCountsBusyAndKeepsRestingOrders(t *testing.T) {
	r, n := newReadWaveRPC(t, 8)
	n.capacity = 8
	q := config.DefaultQuoterConfig()
	q.KeyFile, q.Mode, q.Markets, q.MaxOpenReads = "account-key", "live", []uint64{0}, 8
	now := time.Now().UTC()
	market := Market{ID: 0, Tick: clone(unit), Lot: big.NewInt(10_000_000_000), MinOrderUSD: clone(minimumOrderUSD), Enabled: true}
	ladder := Ladder(market, usd(t, "60000"), q, uint64(now.Add(-time.Second).UnixMilli()))
	require.Len(t, ladder, 4)
	for i, order := range ladder {
		id := int64(i + 1)
		detail := waveOrder(r.account, id, 0)
		detail.IsLong, detail.LimitPrice, detail.SizeUsd, detail.SizeBase, detail.Cloid = order.Buy, order.Price, order.SizeUSD, order.Base, order.Cloid
		n.ids = append(n.ids, big.NewInt(id))
		n.orders[fmt.Sprint(id)] = detail
	}
	n.alter = func(method *abi.Method, values []any) []any {
		if method.RawName == "getPrice" {
			price := values[0].(priceTuple)
			price.TimestampMs = uint64(now.UnixMilli())
			values[0] = price
		}
		return values
	}
	r.wait = func(context.Context, time.Duration) error { return nil }
	n.busyRemaining, n.busyMethod = 1, "getMarketConfig"
	_, err := r.venue(context.Background(), big.NewInt(12), "getMarketConfig", big.NewInt(0))
	require.NoError(t, err)
	n.mu.Lock()
	n.busyRemaining = 1
	n.mu.Unlock()
	var logs bytes.Buffer
	engine, err := New(r, q, slog.New(slog.NewJSONHandler(&logs, nil)))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for i := 0; i < 2; i++ {
		plans, err := engine.Round(ctx, now.Add(time.Duration(i)*time.Millisecond))
		require.NoError(t, err, "%s", logs.String())
		require.Len(t, plans[0], 4)
		require.Zero(t, engine.Health().TransactionsLastRound)
		require.Len(t, engine.Health().Markets[0].Orders, 4)
		require.Empty(t, engine.Health().Markets[0].LastError)
	}
	require.Equal(t, uint64(2), r.BusyResponses())
	require.Contains(t, logs.String(), `"msg":"quoter keep"`)
	require.NotContains(t, logs.String(), "quoter cancel")
	require.NotContains(t, logs.String(), "quoter place")
	var rounds []uint64
	for _, line := range bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte{'\n'}) {
		var entry struct {
			Message string `json:"msg"`
			Busy    uint64 `json:"busy"`
		}
		require.NoError(t, json.Unmarshal(line, &entry))
		if entry.Message == "quoter round" {
			rounds = append(rounds, entry.Busy)
		}
	}
	require.Equal(t, []uint64{1, 0}, rounds)
}

func TestOpenReadsRoundWithoutCounterReportsZero(t *testing.T) {
	chain := newKeeperChain(t)
	engine, logs := testEngine(t, chain, func(q *config.QuoterConfig) { q.Mode = "dry" })
	plans := round(t, engine, chain)
	require.Len(t, plans[0], 4)
	require.Contains(t, logs.String(), `"busy":0`)
	require.Zero(t, chain.places)
	require.Zero(t, chain.cancels)
}

func TestOpenReadsDependentReadsAndOrdersUseConfiguredBound(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(map[bool]string{false: "snapshot", true: "orders"}[all], func(t *testing.T) {
			r, n := newReadWaveRPC(t, 8)
			n.capacity = 8
			for id := int64(1); id <= 12; id++ {
				n.ids = append(n.ids, big.NewInt(id))
				n.orders[fmt.Sprint(id)] = waveOrder(r.account, id, 0)
			}
			n.position = big.NewInt(99)
			release := make(chan struct{})
			n.gate, n.gateDependent = release, true
			var once sync.Once
			defer once.Do(func() { close(release) })
			r.wait = func(context.Context, time.Duration) error { t.Error("dependent reads fit node capacity"); return nil }
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				if all {
					_, err := r.Orders(ctx)
					done <- err
				} else {
					_, err := r.Snapshot(ctx, 0)
					done <- err
				}
			}()
			for count := 0; count < r.openReadLimit(); {
				select {
				case method := <-n.started:
					if method == "getOrderDetails" || method == "getPositionDetails" {
						count++
					}
				case <-ctx.Done():
					t.Fatal("dependent reads did not fill configured bound")
				}
			}
			n.mu.Lock()
			active := n.active
			n.mu.Unlock()
			require.Equal(t, 8, active)
			once.Do(func() { close(release) })
			require.NoError(t, <-done)
			n.mu.Lock()
			defer n.mu.Unlock()
			require.Equal(t, 8, n.maximum)
			require.Len(t, r.orderMarkets, 12)
			if all {
				require.Len(t, n.calls, 13)
			} else {
				require.Len(t, n.calls, 22)
			}
		})
	}
}

func TestOpenReadsBusyTextIsMatchedWithoutDisclosure(t *testing.T) {
	r, n := newReadWaveRPC(t)
	n.fail, n.errorText = "getMarketConfig", "eth_call REJECTED DUE TO RATE LIMIT: server busy; private-node-text"
	r.wait = func(context.Context, time.Duration) error {
		n.mu.Lock()
		n.fail = ""
		n.mu.Unlock()
		return nil
	}
	values, err := r.venue(context.Background(), big.NewInt(12), "getMarketConfig", big.NewInt(0))
	require.NoError(t, err)
	require.True(t, tuple[marketTuple](values).Enabled)
	require.Equal(t, uint64(1), r.BusyResponses())
	n.mu.Lock()
	defer n.mu.Unlock()
	require.Len(t, n.calls, 2)
}
