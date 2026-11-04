package ethbloom

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/rpc"
)

// The fixture executes LOG0 through LOG4, plus a LOG2 with reversed topics.
// No log payload is manufactured by the transport fixture below.
func repairM1295Producer(t *testing.T) []*ethtypes.Log {
	t.Helper()
	topics := [][]byte{nil, {1}, {1, 2}, {1, 2, 3}, {1, 2, 3, 4}, {2, 1}}
	var code []byte
	for i, event := range topics {
		code = append(code, byte(vm.PUSH1), byte(i+1), byte(vm.PUSH1), 0, byte(vm.MSTORE8))
		for j := len(event) - 1; j >= 0; j-- {
			code = append(code, byte(vm.PUSH1), event[j])
		}
		code = append(code, byte(vm.PUSH1), 1, byte(vm.PUSH1), 0, byte(vm.LOG0)+byte(len(event)))
	}
	code = append(code, byte(vm.STOP))
	db, err := state.New(ethtypes.EmptyRootHash, state.NewDatabaseForTesting())
	if err != nil {
		t.Fatal(err)
	}
	txHash := crypto.Keccak256Hash(code)
	db.SetTxContext(txHash, 0)
	if _, _, err := runtime.Execute(code, nil, &runtime.Config{State: db, GasLimit: 1_000_000}); err != nil {
		t.Fatal(err)
	}
	logs := db.GetLogs(txHash, 42, crypto.Keccak256Hash([]byte("repair-m1295-block")))
	if len(logs) != len(topics) {
		t.Fatalf("EVM produced %d logs, want %d", len(logs), len(topics))
	}
	for i, log := range logs {
		if len(log.Topics) != len(topics[i]) || !reflect.DeepEqual(log.Data, []byte{byte(i + 1)}) {
			t.Fatalf("unexpected LOG%d event: %+v", len(topics[i]), log)
		}
		for j, topic := range topics[i] {
			if log.Topics[j] != common.BytesToHash([]byte{topic}) {
				t.Fatalf("event %d topic %d: got %s", i, j, log.Topics[j])
			}
		}
	}
	return logs
}

type repairM1295Case struct {
	name    string
	topics  []any
	address any
	want    []int
}

func repairM1295Cases(logs []*ethtypes.Log) []repairM1295Case {
	a, b, c, d := common.HexToHash("0x1").Hex(), common.HexToHash("0x2").Hex(), common.HexToHash("0x3").Hex(), common.HexToHash("0x4").Hex()
	miss := common.HexToHash("0xff").Hex()
	return []repairM1295Case{
		{name: "unconstrained_LOG0_to_LOG4", want: []int{0, 1, 2, 3, 4, 5}},
		{name: "wildcard_requires_first_topic", topics: []any{nil}, want: []int{1, 2, 3, 4, 5}},
		{name: "empty_OR_requires_first_topic", topics: []any{[]any{}}, want: []int{1, 2, 3, 4, 5}},
		{name: "two_wildcards", topics: []any{nil, nil}, want: []int{2, 3, 4, 5}},
		{name: "trailing_wildcard_rejects_LOG1", topics: []any{a, nil}, want: []int{2, 3, 4}},
		{name: "trailing_empty_OR_rejects_LOG1", topics: []any{a, []any{}}, want: []int{2, 3, 4}},
		{name: "OR_then_trailing_wildcard", topics: []any{[]any{miss, a}, nil}, want: []int{2, 3, 4}},
		{name: "OR_null_is_wildcard_with_cardinality", topics: []any{[]any{miss, nil}, nil}, want: []int{2, 3, 4, 5}},
		{name: "three_wildcards", topics: []any{nil, nil, nil}, want: []int{3, 4}},
		{name: "trailing_wildcard_rejects_LOG2", topics: []any{a, b, nil}, want: []int{3, 4}},
		{name: "trailing_wildcard_rejects_LOG3", topics: []any{a, b, c, nil}, want: []int{4}},
		{name: "four_wildcards", topics: []any{nil, nil, nil, nil}, want: []int{4}},
		{name: "four_exact_topics", topics: []any{a, b, c, d}, want: []int{4}},
		{name: "OR_within_position", topics: []any{[]any{miss, a}}, want: []int{1, 2, 3, 4}},
		{name: "AND_between_OR_positions", topics: []any{[]any{a, b}, []any{miss, a}}, want: []int{5}},
		{name: "topic_order_is_significant", topics: []any{b, a}, want: []int{5}},
		{name: "wrong_second_topic", topics: []any{a, c}},
		{name: "missing_OR_alternatives", topics: []any{[]any{miss, d}}},
		{name: "address_OR_and_topics", topics: []any{a, nil}, address: []common.Address{common.HexToAddress("0xff"), logs[0].Address}, want: []int{2, 3, 4}},
		{name: "wrong_address", topics: []any{nil}, address: common.HexToAddress("0xff")},
	}
}

func (c repairM1295Case) wire() map[string]any {
	crit := map[string]any{"fromBlock": "0x2a", "toBlock": "0x2a", "topics": c.topics}
	if c.address != nil {
		crit["address"] = c.address
	}
	return crit
}

func repairM1295AssertLogs(t *testing.T, produced, got []*ethtypes.Log, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d logs, want producer indices %v: %+v", len(got), want, got)
	}
	for i, index := range want {
		// JSON encodes an absent topic list as an empty array.
		expected := *produced[index]
		actual := *got[i]
		if len(expected.Topics) == 0 && len(actual.Topics) == 0 {
			expected.Topics, actual.Topics = nil, nil
		}
		if !reflect.DeepEqual(expected, actual) {
			t.Fatalf("log %d: got %+v, want producer event %+v", i, actual, expected)
		}
	}
}

func TestRepairM_1295_WildcardTopicCardinality(t *testing.T) {
	logs := repairM1295Producer(t)
	for _, tc := range repairM1295Cases(logs) {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.wire())
			if err != nil {
				t.Fatal(err)
			}
			var crit filters.FilterCriteria
			if err := json.Unmarshal(encoded, &crit); err != nil {
				t.Fatal(err)
			}
			var got []*ethtypes.Log
			for _, log := range logs {
				if MatchesCriteria(log, crit) {
					got = append(got, log)
				}
			}
			repairM1295AssertLogs(t, logs, got, tc.want)
		})
	}
	// Direct callers also cannot match a selector longer than LOG4.
	for _, log := range logs {
		if MatchesCriteria(log, filters.FilterCriteria{Topics: make([][]common.Hash, 5)}) {
			t.Fatalf("five wildcard positions matched LOG%d", len(log.Topics))
		}
	}
}

// This loopback peer supplies EVM-produced events to the production matcher
// using geth's actual criteria decoder, JSON-RPC dispatcher and notifier.
// It does not stand in for the full node's keeper-backed RPC handlers.
type repairM1295LogTransport struct {
	logs []*ethtypes.Log
	done chan error
}

func (p *repairM1295LogTransport) GetLogs(_ context.Context, crit filters.FilterCriteria) []*ethtypes.Log {
	logs := make([]*ethtypes.Log, 0)
	for _, log := range p.logs {
		if MatchesCriteria(log, crit) {
			logs = append(logs, log)
		}
	}
	return logs
}

func (p *repairM1295LogTransport) Logs(ctx context.Context, crit filters.FilterCriteria) (*rpc.Subscription, error) {
	notifier, ok := rpc.NotifierFromContext(ctx)
	if !ok {
		return nil, rpc.ErrNotificationsUnsupported
	}
	sub := notifier.CreateSubscription()
	go func() {
		for _, log := range p.logs {
			if MatchesCriteria(log, crit) {
				if err := notifier.Notify(sub.ID, log); err != nil {
					p.done <- err
					return
				}
			}
		}
		p.done <- nil
	}()
	return sub, nil
}

func TestRepairM_1295_HTTPGetLogs(t *testing.T) {
	logs := repairM1295Producer(t)
	server := rpc.NewServer()
	defer server.Stop()
	if err := server.RegisterName("eth", &repairM1295LogTransport{logs: logs}); err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()
	client, err := rpc.DialHTTP(httpServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for _, tc := range repairM1295Cases(logs) {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var got []*ethtypes.Log
			if err := client.CallContext(ctx, &got, "eth_getLogs", tc.wire()); err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("eth_getLogs must return an array, including for no matches")
			}
			repairM1295AssertLogs(t, logs, got, tc.want)
		})
	}
}

func TestRepairM_1295_WebSocketLogs(t *testing.T) {
	logs := repairM1295Producer(t)
	for _, tc := range repairM1295Cases(logs) {
		t.Run(tc.name, func(t *testing.T) {
			peer := &repairM1295LogTransport{logs: logs, done: make(chan error, 1)}
			server := rpc.NewServer()
			defer server.Stop()
			if err := server.RegisterName("eth", peer); err != nil {
				t.Fatal(err)
			}
			ws := httptest.NewServer(server.WebsocketHandler([]string{"*"}))
			defer ws.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client, err := rpc.DialWebsocket(ctx, "ws"+strings.TrimPrefix(ws.URL, "http"), "")
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			ch := make(chan *ethtypes.Log, len(logs))
			sub, err := client.EthSubscribe(ctx, ch, "logs", tc.wire())
			if err != nil {
				t.Fatal(err)
			}
			defer sub.Unsubscribe()
			var got []*ethtypes.Log
			for range tc.want {
				select {
				case log := <-ch:
					got = append(got, log)
				case err := <-sub.Err():
					t.Fatalf("subscription ended: %v", err)
				case <-ctx.Done():
					t.Fatal("timed out awaiting matching EVM log")
				}
			}
			repairM1295AssertLogs(t, logs, got, tc.want)
			select {
			case err := <-peer.done:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("producer transport did not finish")
			}
			select {
			case unexpected := <-ch:
				t.Fatalf("unexpected extra notification: %+v", unexpected)
			case err := <-sub.Err():
				t.Fatalf("subscription ended: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
		})
	}
}
