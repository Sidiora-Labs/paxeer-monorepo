package quoter

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

type readWaveCall struct {
	method   string
	block    string
	args     []any
	from, to common.Address
}

type readWaveNode struct {
	t                         *testing.T
	mu                        sync.Mutex
	account                   common.Address
	ids                       []*big.Int
	orders                    map[string]orderTuple
	position                  *big.Int
	calls                     []readWaveCall
	requests, active, maximum int
	gate                      <-chan struct{}
	gateDependent             bool
	started                   chan string
	alter                     func(*abi.Method, []any) []any
	fail                      string
}

func newReadWaveRPC(t *testing.T) (*RPC, *readWaveNode) {
	t.Helper()
	node := &readWaveNode{t: t, ids: []*big.Int{}, orders: map[string]orderTuple{}, position: new(big.Int), started: make(chan string, 1024)}
	server := httptest.NewServer(http.HandlerFunc(node.serveHTTP))
	t.Cleanup(server.Close)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	node.account = crypto.PubkeyToAddress(key.PublicKey)
	path := filepath.Join(t.TempDir(), "quoter.hex")
	require.NoError(t, os.WriteFile(path, []byte(common.Bytes2Hex(crypto.FromECDSA(key))), 0600))
	q := config.DefaultQuoterConfig()
	q.KeyFile = path
	cfg := &config.Config{Service: config.Quoter, ChainID: 125, RPCURLs: []string{server.URL}, Port: 8081, Quoting: &q}
	client, err := NewRPC(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(client.Close)
	node.reset()
	return client, node
}

func waveOrder(account common.Address, id, market int64) orderTuple {
	return orderTuple{OrderId: big.NewInt(id), User: account, MarketId: big.NewInt(market), IsLong: true, LimitPrice: new(big.Int).Mul(big.NewInt(59000), unit), TriggerPrice: new(big.Int), AcceptablePrice: new(big.Int), SizeUsd: new(big.Int).Mul(big.NewInt(25), unit), SizeBase: big.NewInt(400_000_000_000_000), FilledBase: new(big.Int), Leverage: clone(unit)}
}

func (n *readWaveNode) reset() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = nil
	n.requests, n.maximum = 0, 0
}

func (n *readWaveNode) serveHTTP(w http.ResponseWriter, request *http.Request) {
	var call struct {
		ID     json.RawMessage   `json:"id"`
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(request.Body).Decode(&call); err != nil {
		n.t.Errorf("decode request: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	reply := func(value any) {
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": call.ID, "result": value}); err != nil {
			n.t.Errorf("encode reply: %v", err)
		}
	}
	n.mu.Lock()
	n.requests++
	n.mu.Unlock()
	switch call.Method {
	case "eth_chainId":
		reply("0x7d")
	case "eth_getTransactionCount":
		reply("0x7")
	case "eth_getBlockByNumber":
		reply(&types.Header{Number: big.NewInt(12), Difficulty: new(big.Int), GasLimit: 30_000_000, Time: 1_700_000_010, Extra: []byte{}})
	case "eth_call":
		var message struct {
			From  common.Address `json:"from"`
			To    common.Address `json:"to"`
			Input hexutil.Bytes  `json:"input"`
			Data  hexutil.Bytes  `json:"data"`
		}
		var block string
		if len(call.Params) != 2 || json.Unmarshal(call.Params[0], &message) != nil || json.Unmarshal(call.Params[1], &block) != nil {
			n.t.Error("read requires a call and block")
			return
		}
		data := message.Input
		if len(data) == 0 {
			data = message.Data
		}
		contract := venueABI
		if message.To == feedAddress {
			contract = feedABI
		}
		method, err := contract.MethodById(data)
		if err != nil {
			n.t.Errorf("decode method: %v", err)
			return
		}
		args, err := method.Inputs.Unpack(data[4:])
		if err != nil {
			n.t.Errorf("decode arguments: %v", err)
			return
		}
		for i, arg := range args {
			if number, ok := arg.(*big.Int); ok {
				args[i] = clone(number)
			}
		}
		n.mu.Lock()
		n.active++
		if n.active > n.maximum {
			n.maximum = n.active
		}
		n.calls = append(n.calls, readWaveCall{method.RawName, block, args, message.From, message.To})
		gate, fail := n.gate, n.fail == method.RawName
		busy := n.active > 4
		if busy {
			gate = nil
		}
		if n.gateDependent && method.RawName != "getOrderDetails" && method.RawName != "getPositionDetails" {
			gate = nil
		}
		var values []any
		switch method.RawName {
		case "getUserOrderIds":
			values = []any{n.ids}
		case "getOrderDetails":
			values = []any{n.orders[args[0].(*big.Int).String()]}
		case "getUserMarketPosition":
			values = []any{n.position}
		case "getPositionDetails":
			values = []any{positionTuple{PositionId: clone(n.position), User: n.account, MarketId: new(big.Int), IsLong: false, SizeBase: big.NewInt(123), SizeUsdAtMark: new(big.Int), EntryPrice: new(big.Int), Leverage: clone(unit), UnrealizedPnlUsd: new(big.Int), FundingOwedUsd: new(big.Int), LiquidationPrice: new(big.Int), TakeProfitPrice: new(big.Int), StopLossPrice: new(big.Int), OpenedAt: new(big.Int)}}
		default:
			values = pendingTagReadValues(n.t, method)
		}
		if n.alter != nil {
			values = n.alter(method, values)
		}
		output, packErr := method.Outputs.Pack(values...)
		n.mu.Unlock()
		var finish sync.Once
		stop := func() { n.mu.Lock(); n.active--; n.mu.Unlock() }
		defer finish.Do(stop)
		n.started <- method.RawName
		if gate != nil {
			select {
			case <-gate:
			case <-request.Context().Done():
				return
			}
		}
		finish.Do(stop)
		if packErr != nil {
			n.t.Errorf("encode %s: %v", method.Name, packErr)
			return
		}
		if busy {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": call.ID, "error": map[string]any{"code": -32000, "message": "eth_call rejected due to rate limit: server busy"}})
			return
		}
		if fail {
			// Return a venue refusal so the client's original reason can be checked.
			data := venueABI.Errors["PerpsPaused"].ID.Bytes()[:4]
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": call.ID, "error": map[string]any{"code": -32000, "message": "execution reverted", "data": hexutil.Encode(data)}})
			return
		}
		reply(hexutil.Bytes(output))
	default:
		n.t.Errorf("unexpected method %s", call.Method)
	}
}

func waveDetailIDs(n *readWaveNode) []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := []string{}
	for _, call := range n.calls {
		if call.method == "getOrderDetails" {
			out = append(out, call.args[0].(*big.Int).String())
		}
	}
	return out
}

func TestReadWaveIndependentReadsOverlapWithinEight(t *testing.T) {
	require.Greater(t, maxReadWave, 1)
	require.LessOrEqual(t, maxReadWave, 8)
	r, n := newReadWaveRPC(t)
	release := make(chan struct{})
	n.gate = release
	var once sync.Once
	defer once.Do(func() { close(release) })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := r.Snapshot(ctx, 0); done <- err }()
	for i := 0; i < maxReadWave; i++ {
		select {
		case <-n.started:
		case <-ctx.Done():
			t.Fatal("independent reads must fill the wave")
		}
	}
	n.mu.Lock()
	active := n.active
	n.mu.Unlock()
	require.Equal(t, maxReadWave, active)
	once.Do(func() { close(release) })
	require.NoError(t, <-done)
	n.mu.Lock()
	defer n.mu.Unlock()
	require.Equal(t, maxReadWave, n.maximum)
	require.Len(t, n.calls, 9)
}

func TestReadWaveEveryReadUsesTheHeaderBlock(t *testing.T) {
	r, n := newReadWaveRPC(t)
	n.ids = []*big.Int{big.NewInt(1), big.NewInt(2)}
	n.orders["1"], n.orders["2"] = waveOrder(r.account, 1, 0), waveOrder(r.account, 2, 0)
	n.position = big.NewInt(99)
	snap, err := r.Snapshot(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(-123), snap.PositionBase)
	require.Len(t, snap.Orders, 2)
	n.mu.Lock()
	defer n.mu.Unlock()
	require.Len(t, n.calls, 12)
	seen := map[string]int{}
	for _, call := range n.calls {
		require.Equal(t, "0xc", call.block)
		require.Equal(t, r.account, call.from)
		seen[call.method]++
		switch call.method {
		case "getPrice":
			require.Equal(t, feedAddress, call.to)
			require.Equal(t, []any{big.NewInt(0)}, call.args)
		case "getMarketConfig", "getMarkPriceDetails", "quoteMarket":
			require.Equal(t, []any{big.NewInt(0)}, call.args)
		case "getUserMarketPosition":
			require.Equal(t, []any{r.account, big.NewInt(0)}, call.args)
		case "getUserOrderIds", "getAccount", "getFeeTier":
			require.Equal(t, []any{r.account}, call.args)
		case "getAccountSummary":
			require.Equal(t, []any{r.account, collateralAddress}, call.args)
		case "getPositionDetails":
			require.Equal(t, []any{big.NewInt(99)}, call.args)
		}
		if call.method != "getPrice" {
			require.Equal(t, venueAddress, call.to)
		}
	}
	require.Equal(t, map[string]int{"getMarketConfig": 1, "getMarkPriceDetails": 1, "quoteMarket": 1, "getPrice": 1, "getUserOrderIds": 1, "getUserMarketPosition": 1, "getAccount": 1, "getAccountSummary": 1, "getFeeTier": 1, "getOrderDetails": 2, "getPositionDetails": 1}, seen)
}

func TestReadWaveOrdersSeedOnlyMarketMembership(t *testing.T) {
	r, n := newReadWaveRPC(t)
	n.ids = []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(3), big.NewInt(4)}
	for i := int64(1); i <= 4; i++ {
		n.orders[big.NewInt(i).String()] = waveOrder(r.account, i, (i-1)/2)
	}
	_, err := r.Snapshot(context.Background(), 0)
	require.NoError(t, err)
	n.mu.Lock()
	require.Equal(t, 16, n.requests)
	n.mu.Unlock()
	n.reset()
	all, err := r.Orders(context.Background())
	require.NoError(t, err)
	require.Len(t, all, 4)
	require.ElementsMatch(t, []string{"1", "2", "3", "4"}, waveDetailIDs(n))
	n.mu.Lock()
	o := n.orders["1"]
	o.FilledBase = big.NewInt(100)
	n.orders["1"] = o
	n.mu.Unlock()
	n.reset()
	snap, err := r.Snapshot(context.Background(), 0)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"1", "2"}, waveDetailIDs(n))
	require.Len(t, snap.Orders, 2)
	require.Equal(t, new(big.Int).Sub(o.SizeBase, o.FilledBase), snap.Orders[0].Base)
	n.mu.Lock()
	require.Equal(t, 14, n.requests)
	n.mu.Unlock()
	t.Log("snapshot with two market orders and two other-market orders: 16 requests before, 14 after membership discovery")
	n.reset()
	_, err = r.Orders(context.Background())
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"1", "2", "3", "4"}, waveDetailIDs(n))
}

func TestReadWaveUnknownOrdersAreRememberedAndRemovedOrdersForgotten(t *testing.T) {
	r, n := newReadWaveRPC(t)
	n.ids = []*big.Int{big.NewInt(1), big.NewInt(2)}
	n.orders["1"], n.orders["2"] = waveOrder(r.account, 1, 0), waveOrder(r.account, 2, 1)
	snap, err := r.Snapshot(context.Background(), 0)
	require.NoError(t, err)
	require.Len(t, snap.Orders, 1)
	require.ElementsMatch(t, []string{"1", "2"}, waveDetailIDs(n))
	n.reset()
	_, err = r.Snapshot(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, []string{"1"}, waveDetailIDs(n))
	n.mu.Lock()
	n.ids = []*big.Int{big.NewInt(1)}
	n.mu.Unlock()
	_, err = r.Snapshot(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, map[string]uint64{"1": 0}, r.orderMarkets)
	n.mu.Lock()
	n.ids = []*big.Int{}
	n.mu.Unlock()
	_, err = r.Orders(context.Background())
	require.NoError(t, err)
	require.Empty(t, r.orderMarkets)
	n.mu.Lock()
	n.ids = []*big.Int{big.NewInt(2)}
	n.mu.Unlock()
	n.reset()
	_, err = r.Snapshot(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, []string{"2"}, waveDetailIDs(n))
	require.Equal(t, map[string]uint64{"2": 1}, r.orderMarkets)
}

func TestReadWaveFailuresReturnOriginalReasonAndRecover(t *testing.T) {
	for _, method := range []string{"getMarketConfig", "getMarkPriceDetails", "quoteMarket", "getPrice", "getUserOrderIds", "getUserMarketPosition", "getAccount", "getAccountSummary", "getFeeTier", "getOrderDetails", "getPositionDetails"} {
		t.Run(method, func(t *testing.T) {
			r, n := newReadWaveRPC(t)
			n.ids = []*big.Int{big.NewInt(1)}
			n.orders["1"] = waveOrder(r.account, 1, 0)
			n.position = big.NewInt(99)
			n.fail = method
			snap, err := r.Snapshot(context.Background(), 0)
			require.EqualError(t, err, "PerpsPaused")
			require.Equal(t, Snapshot{}, snap)
			n.mu.Lock()
			n.fail = ""
			n.mu.Unlock()
			snap, err = r.Snapshot(context.Background(), 0)
			require.NoError(t, err)
			require.Len(t, snap.Orders, 1)
		})
	}
}

func TestReadWaveChangedMarketIsInvalid(t *testing.T) {
	r, n := newReadWaveRPC(t)
	n.ids = []*big.Int{big.NewInt(1)}
	n.orders["1"] = waveOrder(r.account, 1, 0)
	_, err := r.Orders(context.Background())
	require.NoError(t, err)
	n.mu.Lock()
	n.orders["1"] = waveOrder(r.account, 1, 1)
	n.mu.Unlock()
	_, err = r.Snapshot(context.Background(), 0)
	require.EqualError(t, err, "invalid chain response")
	_, err = r.Orders(context.Background())
	require.EqualError(t, err, "invalid chain response")
}

func TestReadWaveResponseChecksArePreserved(t *testing.T) {
	for _, check := range []string{"timestamp", "fee width", "fee bound", "tick", "lot", "order owner", "order market", "overfill"} {
		t.Run(check, func(t *testing.T) {
			r, n := newReadWaveRPC(t)
			n.ids = []*big.Int{big.NewInt(1)}
			n.orders["1"] = waveOrder(r.account, 1, 0)
			n.alter = func(method *abi.Method, values []any) []any {
				switch {
				case check == "timestamp" && method.RawName == "getPrice":
					p := values[0].(priceTuple)
					p.TimestampMs = 1 << 63
					values[0] = p
				case check == "fee width" && method.RawName == "getFeeTier":
					values[1] = new(big.Int).Lsh(big.NewInt(1), 65)
				case check == "fee bound" && method.RawName == "getFeeTier":
					values[1] = big.NewInt(1_000_001)
				case (check == "tick" || check == "lot") && method.RawName == "getMarketConfig":
					m := values[0].(marketTuple)
					if check == "tick" {
						m.TickSize = new(big.Int)
					} else {
						m.LotSize = new(big.Int)
					}
					values[0] = m
				case method.RawName == "getOrderDetails":
					o := values[0].(orderTuple)
					switch check {
					case "order owner":
						o.User = common.Address{}
					case "order market":
						o.MarketId = new(big.Int).Lsh(big.NewInt(1), 65)
					case "overfill":
						o.FilledBase = new(big.Int).Add(o.SizeBase, big.NewInt(1))
					}
					values[0] = o
				}
				return values
			}
			_, err := r.Snapshot(context.Background(), 0)
			require.EqualError(t, err, "invalid chain response")
		})
	}
}

func TestReadWaveDependentReadsAndOrdersShareTheBound(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(map[bool]string{false: "snapshot", true: "orders"}[all], func(t *testing.T) {
			r, n := newReadWaveRPC(t)
			for i := int64(1); i <= 12; i++ {
				n.ids = append(n.ids, big.NewInt(i))
				n.orders[big.NewInt(i).String()] = waveOrder(r.account, i, 0)
			}
			n.position = big.NewInt(99)
			release := make(chan struct{})
			n.gate, n.gateDependent = release, true
			var once sync.Once
			defer once.Do(func() { close(release) })
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
			for count := 0; count < maxReadWave; {
				select {
				case method := <-n.started:
					if method == "getOrderDetails" || method == "getPositionDetails" {
						count++
					}
				case <-ctx.Done():
					t.Fatal("dependent reads must fill the wave")
				}
			}
			n.mu.Lock()
			active := n.active
			n.mu.Unlock()
			require.Equal(t, maxReadWave, active)
			once.Do(func() { close(release) })
			require.NoError(t, <-done)
			n.mu.Lock()
			defer n.mu.Unlock()
			require.Equal(t, maxReadWave, n.maximum)
			require.Len(t, r.orderMarkets, 12)
			if !all {
				require.Len(t, n.calls, 22)
			}
		})
	}
}

func TestReadWaveSnapshotAndOrdersSerializeMembership(t *testing.T) {
	r, n := newReadWaveRPC(t)
	n.ids = []*big.Int{big.NewInt(1), big.NewInt(2)}
	n.orders["1"], n.orders["2"] = waveOrder(r.account, 1, 0), waveOrder(r.account, 2, 1)
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Go(func() { _, err := r.Snapshot(context.Background(), 0); require.NoError(t, err) })
		group.Go(func() { all, err := r.Orders(context.Background()); require.NoError(t, err); require.Len(t, all, 2) })
	}
	group.Wait()
	require.Equal(t, map[string]uint64{"1": 0, "2": 1}, r.orderMarkets)
	n.mu.Lock()
	defer n.mu.Unlock()
	require.LessOrEqual(t, n.maximum, maxReadWave)
}

func TestReadWaveIndependentClientsShareNodeCapacity(t *testing.T) {
	r, n := newReadWaveRPC(t)
	other := &RPC{client: r.client, account: r.account, now: time.Now}
	release := make(chan struct{})
	n.gate = release
	var once sync.Once
	defer once.Do(func() { close(release) })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 2)
	for _, client := range []*RPC{r, other} {
		go func() { _, err := client.Snapshot(ctx, 0); done <- err }()
	}
	for i := 0; i < 4; i++ {
		select {
		case <-n.started:
		case <-ctx.Done():
			t.Fatal("both clients must read side by side within the node capacity")
		}
	}
	n.mu.Lock()
	active := n.active
	n.mu.Unlock()
	require.Equal(t, 4, active)
	once.Do(func() { close(release) })
	for i := 0; i < 2; i++ {
		require.NoError(t, <-done)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	require.Equal(t, 4, n.maximum)
	require.Len(t, n.calls, 18)
}
