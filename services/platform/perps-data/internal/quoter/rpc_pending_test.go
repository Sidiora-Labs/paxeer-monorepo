package quoter

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/config"
)

type pendingTagNode struct {
	t                 *testing.T
	mu                sync.Mutex
	mode              string
	mined             uint64
	pending           uint64
	pendingRequests   int
	sent              []*types.Transaction
	receiptRequests   int
	receiptsAvailable bool
	minedReadRefused  bool
	sendRefused       bool
	quoting           config.QuoterConfig
}

func newPendingTagRPC(t *testing.T, mode, quotingMode string) (*RPC, *pendingTagNode) {
	t.Helper()
	node := &pendingTagNode{t: t, mode: mode, mined: 7, pending: 7, receiptsAvailable: true}
	server := httptest.NewServer(http.HandlerFunc(node.serveHTTP))
	t.Cleanup(server.Close)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	keyFile := filepath.Join(t.TempDir(), "quoter.hex")
	require.NoError(t, os.WriteFile(keyFile, []byte(common.Bytes2Hex(crypto.FromECDSA(key))), 0600))
	q := config.DefaultQuoterConfig()
	q.KeyFile, q.Mode = keyFile, quotingMode
	q.Markets = []uint64{0}
	node.quoting = q
	cfg := &config.Config{Service: config.Quoter, ChainID: 125, RPCURLs: []string{server.URL}, Port: 8081, Quoting: &q}
	r, err := NewRPC(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(r.Close)
	return r, node
}

func (n *pendingTagNode) serveHTTP(w http.ResponseWriter, request *http.Request) {
	n.mu.Lock()
	defer n.mu.Unlock()
	var call struct {
		ID     json.RawMessage   `json:"id"`
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(request.Body).Decode(&call); err != nil {
		n.t.Errorf("decode JSON-RPC request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	reply := func(result any) {
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": call.ID, "result": result}); err != nil {
			n.t.Errorf("encode JSON-RPC result: %v", err)
		}
	}
	refuse := func(code int, message string) {
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": call.ID, "error": map[string]any{"code": code, "message": message}}); err != nil {
			n.t.Errorf("encode JSON-RPC error: %v", err)
		}
	}
	switch call.Method {
	case "eth_chainId":
		reply("0x7d")
	case "eth_getTransactionCount":
		var tag string
		if len(call.Params) != 2 || json.Unmarshal(call.Params[1], &tag) != nil {
			n.t.Error("transaction count requires an account and block tag")
			refuse(-32602, "invalid params")
			return
		}
		if tag == "pending" {
			n.pendingRequests++
			switch n.mode {
			case "refused":
				refuse(-32000, "pending served by sequencer")
			case "other-refusal":
				refuse(-32602, "unsupported block tag")
			case "disconnected":
				connection, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					n.t.Errorf("take JSON-RPC connection: %v", err)
					return
				}
				_ = connection.Close()
			case "malformed":
				_, _ = w.Write([]byte(`{"jsonrpc":`))
			case "invalid-count":
				reply("not-a-count")
			case "http-error":
				http.Error(w, "pending served by sequencer", http.StatusBadGateway)
			case "timeout":
				<-request.Context().Done()
			default:
				reply(hexutil.EncodeUint64(n.pending))
			}
		} else if tag == "latest" {
			if n.minedReadRefused {
				refuse(-32000, "account count unavailable")
			} else {
				reply(hexutil.EncodeUint64(n.mined))
			}
		} else {
			n.t.Errorf("unexpected transaction count tag %q", tag)
			refuse(-32602, "invalid block tag")
		}
	case "eth_getBlockByNumber":
		reply(&types.Header{Number: big.NewInt(12), Difficulty: new(big.Int), GasLimit: 30_000_000, Time: uint64(time.Now().Unix()), Extra: []byte{}})
	case "eth_call":
		var message struct {
			To    common.Address `json:"to"`
			Input hexutil.Bytes  `json:"input"`
			Data  hexutil.Bytes  `json:"data"`
		}
		if len(call.Params) != 2 || json.Unmarshal(call.Params[0], &message) != nil {
			n.t.Error("contract read requires a call and block tag")
			refuse(-32602, "invalid params")
			return
		}
		data := message.Input
		if len(data) == 0 {
			data = message.Data
		}
		contract := venueABI
		if message.To == feedAddress {
			contract = feedABI
		} else if message.To != venueAddress {
			n.t.Errorf("unexpected read destination %s", message.To)
		}
		method, err := contract.MethodById(data)
		if err != nil {
			n.t.Errorf("decode contract method: %v", err)
			refuse(-32602, "unknown method")
			return
		}
		values := pendingTagReadValues(n.t, method)
		output, err := method.Outputs.Pack(values...)
		if err != nil {
			n.t.Errorf("encode %s: %v", method.Name, err)
			refuse(-32603, "invalid result")
			return
		}
		reply(hexutil.Bytes(output))
	case "eth_gasPrice":
		reply("0x3b9aca00")
	case "eth_estimateGas":
		reply("0x186a0")
	case "eth_sendRawTransaction":
		var raw hexutil.Bytes
		if len(call.Params) != 1 || json.Unmarshal(call.Params[0], &raw) != nil {
			n.t.Error("submission requires a signed transaction")
			refuse(-32602, "invalid params")
			return
		}
		tx := new(types.Transaction)
		if err := tx.UnmarshalBinary(raw); err != nil {
			n.t.Errorf("decode signed transaction: %v", err)
			refuse(-32602, "invalid transaction")
			return
		}
		n.sent = append(n.sent, tx)
		if n.sendRefused {
			refuse(-32000, "nonce too low")
			return
		}
		n.mined = tx.Nonce() + 1
		n.pending = n.mined
		reply(tx.Hash())
	case "eth_getTransactionReceipt":
		n.receiptRequests++
		var hash common.Hash
		if len(call.Params) != 1 || json.Unmarshal(call.Params[0], &hash) != nil {
			n.t.Error("receipt read requires a transaction hash")
			refuse(-32602, "invalid params")
			return
		}
		for _, tx := range n.sent {
			if tx.Hash() == hash && n.receiptsAvailable {
				reply(&types.Receipt{Status: types.ReceiptStatusSuccessful, CumulativeGasUsed: 100_000, GasUsed: 100_000, TxHash: hash, BlockHash: common.HexToHash("0xc"), BlockNumber: big.NewInt(12), Logs: []*types.Log{}})
				return
			}
		}
		reply(nil)
	default:
		n.t.Errorf("unexpected JSON-RPC method %s", call.Method)
		refuse(-32601, "method not found")
	}
}

func pendingTagReadValues(t *testing.T, method *abi.Method) []any {
	mark := new(big.Int).Mul(big.NewInt(60_000), unit)
	switch method.RawName {
	case "getActiveMarketIds":
		return []any{[]*big.Int{big.NewInt(0)}}
	case "getUserOrderIds":
		return []any{[]*big.Int{}}
	case "getMarketConfig":
		return []any{marketTuple{MarketId: new(big.Int), Symbol: "BTC", Name: "Bitcoin", OracleSymbol: "BTC", SzDecimals: 8, PxDecimals: 2, TickSize: clone(unit), LotSize: big.NewInt(10_000_000_000), MaxLeverage: clone(unit), ImpactNotionalUsd: new(big.Int), OiCapUsd: new(big.Int), FundingCapPpmPerHour: new(big.Int), Enabled: true}}
	case "getMarkPriceDetails":
		return []any{mark, mark, new(big.Int), true}
	case "quoteMarket":
		return []any{quoteTuple{IndexPrice: mark, MarkPrice: mark, OracleTWAP: mark, FundingRatePerSecond: new(big.Int), FundingRate24h: new(big.Int), LongOI: new(big.Int), ShortOI: new(big.Int), MaxLeverage: clone(unit), MaintenanceMarginBps: big.NewInt(500), Enabled: true}}
	case "getPrice":
		return []any{priceTuple{IndexPrice: mark, MarkPrice: mark, TimestampMs: uint64(time.Now().UnixMilli()), Height: 12, Sources: 1, Fresh: true}}
	case "getUserMarketPosition":
		return []any{new(big.Int)}
	case "getAccount":
		return []any{accountTuple{Collateral: big.NewInt(1_000_000_000), EquityUsd: new(big.Int).Mul(big.NewInt(1000), unit), InitialMarginUsd: new(big.Int), MaintenanceMarginUsd: new(big.Int), Withdrawable: big.NewInt(1_000_000_000), Volume14dUsd: new(big.Int), PositionCount: new(big.Int), OpenOrderCount: new(big.Int)}}
	case "getAccountSummary":
		return []any{summaryTuple{Idle: big.NewInt(1_000_000_000), TotalLocked: new(big.Int), PositionCount: new(big.Int)}}
	case "getFeeTier":
		return []any{uint8(0), big.NewInt(500), new(big.Int), new(big.Int)}
	default:
		t.Errorf("unexpected contract method %s", method.Sig)
		return nil
	}
}

func pendingTagOrder(round uint64) Order {
	return Order{Market: 0, Buy: true, Price: new(big.Int).Mul(big.NewInt(59_940), unit), SizeUSD: new(big.Int).Mul(big.NewInt(25), unit), Base: big.NewInt(400_000_000_000_000), Cloid: ClientID(0, true, 1, round)}
}

func TestPendingTagRefusedDryReadsComplete(t *testing.T) {
	for _, mode := range []string{"refused", "other-refusal"} {
		t.Run(mode, func(t *testing.T) {
			r, node := newPendingTagRPC(t, mode, "dry")
			open, err := r.Orders(context.Background())
			require.NoError(t, err)
			require.Empty(t, open)
			snap, err := r.Snapshot(context.Background(), 0)
			require.NoError(t, err)
			require.True(t, snap.Market.Enabled)
			require.True(t, snap.Price.Fresh)
			require.Equal(t, new(big.Int).Mul(big.NewInt(1000), unit), snap.FreeUSD)
			var logs bytes.Buffer
			engine, err := New(r, node.quoting, slog.New(slog.NewJSONHandler(&logs, nil)))
			require.NoError(t, err)
			plan, err := engine.Round(context.Background(), time.Now().UTC())
			require.NoError(t, err)
			require.Len(t, plan[0], 4)
			require.Contains(t, logs.String(), "quoter place")
			node.mu.Lock()
			defer node.mu.Unlock()
			require.Equal(t, 1, node.pendingRequests)
			require.Empty(t, node.sent)
		})
	}
}

func TestPendingTagRefusedSendTakesTheMinedCount(t *testing.T) {
	t.Run("receipt", func(t *testing.T) {
		r, node := newPendingTagRPC(t, "refused", "live")
		order := pendingTagOrder(1)
		require.NoError(t, r.Place(context.Background(), order))
		require.Nil(t, r.pending)
		require.Len(t, r.completed, 1)
		require.NoError(t, r.Place(context.Background(), order))
		node.mu.Lock()
		defer node.mu.Unlock()
		require.Len(t, node.sent, 1)
		tx := node.sent[0]
		require.Equal(t, uint64(7), tx.Nonce())
		require.Equal(t, r.chain, tx.ChainId())
		sender, err := types.Sender(types.LatestSignerForChainID(r.chain), tx)
		require.NoError(t, err)
		require.Equal(t, r.account, sender)
		require.Equal(t, venueAddress, *tx.To())
		data, err := venueABI.Pack("placeOrder", params(order))
		require.NoError(t, err)
		require.Equal(t, data, tx.Data())
		require.Equal(t, 1, node.receiptRequests)
		require.Equal(t, 1, node.pendingRequests)
	})
	t.Run("retained transaction", func(t *testing.T) {
		r, node := newPendingTagRPC(t, "refused", "live")
		node.receiptsAvailable = false
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		require.EqualError(t, r.Place(ctx, pendingTagOrder(1)), "transaction pending")
		require.NotNil(t, r.pending)
		hash := r.pending.tx.Hash()
		ctx, cancelNext := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancelNext()
		require.EqualError(t, r.Place(ctx, pendingTagOrder(2)), "transaction pending")
		require.Equal(t, hash, r.pending.tx.Hash())
		node.mu.Lock()
		require.Len(t, node.sent, 1)
		node.receiptsAvailable = true
		node.mu.Unlock()
		require.NoError(t, r.Place(context.Background(), pendingTagOrder(2)))
		require.Nil(t, r.pending)
		node.mu.Lock()
		defer node.mu.Unlock()
		require.Len(t, node.sent, 2)
		require.Equal(t, uint64(8), node.sent[1].Nonce())
	})
	t.Run("nonce conflict", func(t *testing.T) {
		r, node := newPendingTagRPC(t, "refused", "live")
		node.sendRefused = true
		err := r.Place(context.Background(), pendingTagOrder(1))
		require.EqualError(t, err, "nonce conflict")
		require.True(t, isRefusal(err))
		require.Nil(t, r.pending)
	})
}

func TestPendingTagTransportFailureStaysAFailure(t *testing.T) {
	for _, mode := range []string{"disconnected", "malformed", "invalid-count", "http-error", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			r, node := newPendingTagRPC(t, mode, "live")
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			_, err := r.Orders(ctx)
			require.EqualError(t, err, "chain request failed")
			require.False(t, isRefusal(err))
			require.True(t, r.pendingTagRetryAt.IsZero())
			node.mu.Lock()
			require.Positive(t, node.pendingRequests)
			require.Empty(t, node.sent)
			node.mode = "refused"
			node.mu.Unlock()
			_, err = r.Orders(context.Background())
			require.NoError(t, err)
		})
	}
	t.Run("mined read", func(t *testing.T) {
		r, node := newPendingTagRPC(t, "refused", "live")
		node.minedReadRefused = true
		require.EqualError(t, r.Place(context.Background(), pendingTagOrder(1)), "chain request failed")
		node.mu.Lock()
		defer node.mu.Unlock()
		require.Empty(t, node.sent)
	})
}

func TestPendingTagRefusedIsNotAskedAgainWithinAMinute(t *testing.T) {
	r, node := newPendingTagRPC(t, "refused", "dry")
	now := time.Unix(1_800_000_000, 0)
	r.now = func() time.Time { return now }
	_, err := r.Orders(context.Background())
	require.NoError(t, err)
	for _, elapsed := range []time.Duration{0, 30 * time.Second, 60*time.Second - time.Nanosecond} {
		now = time.Unix(1_800_000_000, 0).Add(elapsed)
		_, err = r.Orders(context.Background())
		require.NoError(t, err)
		_, err = r.Snapshot(context.Background(), 0)
		require.NoError(t, err)
		node.mu.Lock()
		require.Equal(t, 1, node.pendingRequests)
		node.mu.Unlock()
	}
	now = time.Unix(1_800_000_000, 0).Add(time.Minute)
	_, err = r.Orders(context.Background())
	require.NoError(t, err)
	_, err = r.Snapshot(context.Background(), 0)
	require.NoError(t, err)
	node.mu.Lock()
	require.Equal(t, 2, node.pendingRequests)
	node.mode = "served"
	node.pending = node.mined + 1
	node.mu.Unlock()
	now = now.Add(time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = r.Orders(ctx)
	require.EqualError(t, err, "transaction pending")
	require.True(t, r.pendingTagRetryAt.IsZero())
	node.mu.Lock()
	defer node.mu.Unlock()
	require.Equal(t, 3, node.pendingRequests)
}

func TestPendingTagServedStillHoldsBackWhileItDiffers(t *testing.T) {
	r, node := newPendingTagRPC(t, "served", "live")
	node.pending = node.mined + 1
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := r.Place(ctx, pendingTagOrder(1))
	require.EqualError(t, err, "transaction pending")
	require.False(t, isRefusal(err))
	require.Nil(t, r.pending)
	require.True(t, r.pendingTagRetryAt.IsZero())
	node.mu.Lock()
	defer node.mu.Unlock()
	require.Positive(t, node.pendingRequests)
	require.Empty(t, node.sent)
}
