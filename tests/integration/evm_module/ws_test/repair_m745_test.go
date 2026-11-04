package ws_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
)

// The peer fixtures use the production header codec and JSON-RPC server. The
// component under repair is the live test's subscription and parity assertion.
type repairM745Peer struct {
	head, block json.RawMessage
	requests    chan rpc.BlockNumber
}

func (p *repairM745Peer) NewHeads(ctx context.Context) (*rpc.Subscription, error) {
	notifier, ok := rpc.NotifierFromContext(ctx)
	if !ok {
		return nil, rpc.ErrNotificationsUnsupported
	}
	sub := notifier.CreateSubscription()
	if err := notifier.Notify(sub.ID, p.head); err != nil {
		return nil, err
	}
	return sub, nil
}

func (p *repairM745Peer) GetBlockByNumber(number rpc.BlockNumber, fullTx bool) (json.RawMessage, error) {
	p.requests <- number
	if number != 42 || fullTx {
		return nil, fmt.Errorf("want height 0x2a and fullTx=false, got %v, %v", number, fullTx)
	}
	return p.block, nil
}

func repairM745Header() *ethtypes.Header {
	withdrawals := ethtypes.EmptyWithdrawalsHash
	beacon := common.Hash{}
	blobGas, excess := uint64(0), uint64(0)
	header := &ethtypes.Header{
		ParentHash: common.HexToHash("0x11"), UncleHash: ethtypes.EmptyUncleHash,
		Coinbase: common.HexToAddress("0x22"), Root: common.HexToHash("0x33"),
		TxHash: common.HexToHash("0x44"), ReceiptHash: common.HexToHash("0x55"),
		Difficulty: big.NewInt(0), Number: big.NewInt(42), GasLimit: 30_000_000,
		GasUsed: 21_000, Time: 1_700_000_000, Extra: []byte{}, BaseFee: big.NewInt(1_000_000_000),
		WithdrawalsHash: &withdrawals, BlobGasUsed: &blobGas, ExcessBlobGas: &excess,
		ParentBeaconRoot: &beacon,
	}
	header.Bloom = ethtypes.CreateBloom(&ethtypes.Receipt{Logs: []*ethtypes.Log{{
		Address: header.Coinbase, Topics: []common.Hash{common.HexToHash("0x66")},
	}}})
	return header
}

func repairM745Encode(t *testing.T, header *ethtypes.Header) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	// The consensus hash is deliberately distinct from the RLP header hash.
	return repairM745Field(t, raw, "hash", common.HexToHash("0xabcdef"))
}

func repairM745Field(t *testing.T, raw json.RawMessage, field string, value interface{}) json.RawMessage {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if value == nil {
		delete(fields, field)
	} else {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		fields[field] = encoded
	}
	out, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func repairM745Exchange(t *testing.T, head, block json.RawMessage) (*ethtypes.Header, error, []rpc.BlockNumber) {
	t.Helper()
	peer := &repairM745Peer{head: head, block: block, requests: make(chan rpc.BlockNumber, 2)}
	server := rpc.NewServer()
	t.Cleanup(server.Stop)
	if err := server.RegisterName("eth", peer); err != nil {
		t.Fatal(err)
	}
	wsHandler := server.WebsocketHandler([]string{"*"})
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if websocket.IsWebSocketUpgrade(r) {
			wsHandler.ServeHTTP(w, r)
		} else {
			server.ServeHTTP(w, r)
		}
	}))
	t.Cleanup(endpoint.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(endpoint.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	raw, err := readNewHead(conn)
	if err != nil {
		t.Fatalf("real WebSocket subscription: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := rpc.DialContext(ctx, endpoint.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	header, checkErr := checkCommittedHead(ctx, client, raw)
	var requests []rpc.BlockNumber
	for len(peer.requests) != 0 {
		requests = append(requests, <-peer.requests)
	}
	return header, checkErr, requests
}

func TestRepairM_745_CommittedHeader(t *testing.T) {
	header := repairM745Header()
	raw := repairM745Encode(t, header)
	// HTTP blocks also carry body fields; only their full header is compared.
	block := repairM745Field(t, raw, "transactions", []common.Hash{common.HexToHash("0x77")})
	decoded, err, requests := repairM745Exchange(t, raw, block)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Number.Cmp(header.Number) != 0 || decoded.Bloom != header.Bloom || decoded.GasUsed != header.GasUsed {
		t.Fatalf("decoded header lost committed fields: %+v", decoded)
	}
	if len(requests) != 1 || requests[0] != 42 {
		t.Fatalf("must fetch exactly the notified height over HTTP: %v", requests)
	}
	if decoded.Hash() == common.HexToHash("0xabcdef") {
		t.Fatal("fixture must distinguish the consensus identity from the computed header hash")
	}
}

func TestRepairM_745_RejectsCommittedMismatch(t *testing.T) {
	mutations := []struct {
		field  string
		change func(*ethtypes.Header)
	}{
		{"number", func(h *ethtypes.Header) { h.Number = big.NewInt(43) }},
		{"parentHash", func(h *ethtypes.Header) { h.ParentHash[0] ^= 1 }},
		{"sha3Uncles", func(h *ethtypes.Header) { h.UncleHash[0] ^= 1 }},
		{"miner", func(h *ethtypes.Header) { h.Coinbase[0] ^= 1 }},
		{"stateRoot", func(h *ethtypes.Header) { h.Root[0] ^= 1 }},
		{"transactionsRoot", func(h *ethtypes.Header) { h.TxHash[0] ^= 1 }},
		{"receiptsRoot", func(h *ethtypes.Header) { h.ReceiptHash[0] ^= 1 }},
		{"logsBloom", func(h *ethtypes.Header) { h.Bloom[0] ^= 1 }},
		{"difficulty", func(h *ethtypes.Header) { h.Difficulty = big.NewInt(1) }},
		{"gasLimit", func(h *ethtypes.Header) { h.GasLimit++ }},
		{"gasUsed", func(h *ethtypes.Header) { h.GasUsed++ }},
		{"timestamp", func(h *ethtypes.Header) { h.Time++ }},
		{"extraData", func(h *ethtypes.Header) { h.Extra = []byte{1} }},
		{"mixHash", func(h *ethtypes.Header) { h.MixDigest[0] ^= 1 }},
		{"nonce", func(h *ethtypes.Header) { h.Nonce[0] ^= 1 }},
		{"baseFeePerGas", func(h *ethtypes.Header) { h.BaseFee = big.NewInt(2) }},
		{"withdrawalsRoot", func(h *ethtypes.Header) { h.WithdrawalsHash[0] ^= 1 }},
		{"blobGasUsed", func(h *ethtypes.Header) { *h.BlobGasUsed = 1 }},
		{"excessBlobGas", func(h *ethtypes.Header) { *h.ExcessBlobGas = 1 }},
		{"parentBeaconBlockRoot", func(h *ethtypes.Header) { h.ParentBeaconRoot[0] ^= 1 }},
		{"requestsHash", func(h *ethtypes.Header) { v := common.HexToHash("0x99"); h.RequestsHash = &v }},
	}
	head := repairM745Encode(t, repairM745Header())
	for _, mutation := range mutations {
		t.Run(mutation.field, func(t *testing.T) {
			changed := repairM745Header()
			mutation.change(changed)
			_, err, requests := repairM745Exchange(t, head, repairM745Encode(t, changed))
			if err == nil || !strings.Contains(err.Error(), mutation.field+" mismatch") {
				t.Fatalf("must reject %s mismatch even when hash agrees, got %v", mutation.field, err)
			}
			if len(requests) != 1 || requests[0] != 42 {
				t.Fatalf("lookup must use notification height: %v", requests)
			}
		})
	}
	t.Run("hash", func(t *testing.T) {
		block := repairM745Field(t, head, "hash", common.HexToHash("0xfedcba"))
		_, err, _ := repairM745Exchange(t, head, block)
		if err == nil || !strings.Contains(err.Error(), "hash mismatch") {
			t.Fatalf("must compare the committed wire hash, got %v", err)
		}
	})
}

func TestRepairM_745_RejectsMissingFields(t *testing.T) {
	raw := repairM745Encode(t, repairM745Header())
	for _, side := range []string{"newHeads", "committed block"} {
		for _, field := range committedHeadFields {
			for _, null := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/null=%v", side, field, null), func(t *testing.T) {
					var value interface{}
					if null {
						value = json.RawMessage("null")
					}
					bad := repairM745Field(t, raw, field, value)
					head, block := raw, raw
					if side == "newHeads" {
						head = bad
					} else {
						block = bad
					}
					_, err, _ := repairM745Exchange(t, head, block)
					if err == nil || !strings.Contains(err.Error(), side+": header missing "+field) {
						t.Fatalf("must reject missing/null %s on %s, got %v", field, side, err)
					}
				})
			}
		}
	}
}

func TestRepairM_745_RejectsMalformedEncoding(t *testing.T) {
	raw := repairM745Encode(t, repairM745Header())
	cases := []struct {
		field string
		value interface{}
	}{
		{"hash", "0x01"}, {"hash", common.Hash{}}, {"number", "0x02a"},
		{"number", "0x0"}, {"timestamp", "0x0"}, {"timestamp", 1_700_000_000},
		{"stateRoot", "0x11"}, {"transactionsRoot", "0xzz"}, {"receiptsRoot", "11"},
		{"logsBloom", "0x00"}, {"miner", "0x22"}, {"nonce", "0x0"},
		{"gasLimit", "0x01"}, {"gasUsed", "0x10000000000000000"},
		{"gasUsed", "0x1c9c381"}, {"baseFeePerGas", "1000000000"},
		{"extraData", "0x1"}, {"number", "0x2A"},
	}
	for _, side := range []string{"newHeads", "committed block"} {
		for i, tc := range cases {
			t.Run(fmt.Sprintf("%s/%s/%d", side, tc.field, i), func(t *testing.T) {
				bad := repairM745Field(t, raw, tc.field, tc.value)
				head, block := raw, raw
				if side == "newHeads" {
					head = bad
				} else {
					block = bad
				}
				_, err, _ := repairM745Exchange(t, head, block)
				if err == nil || !strings.Contains(err.Error(), side+":") {
					t.Fatalf("must reject malformed %s on %s, got %v", tc.field, side, err)
				}
			})
		}
	}
	t.Run("unavailable block", func(t *testing.T) {
		_, err, requests := repairM745Exchange(t, raw, json.RawMessage("null"))
		if err == nil || !strings.Contains(err.Error(), "committed block:") || len(requests) != 1 {
			t.Fatalf("null committed block must fail after HTTP lookup: requests=%v err=%v", requests, err)
		}
	})
}
