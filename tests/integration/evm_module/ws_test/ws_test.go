// Package ws_test exercises WebSocket JSON-RPC subscriptions against a
// live Pax EVM RPC. The test is consensus-mode agnostic: it dials the
// EVM WS port and asserts that eth_subscribe("newHeads") delivers a
// head notification. It runs under both standard CometBFT clusters and
// Autobahn clusters — the producer hook differs between them (legacy
// event bus vs in-process notifier), but the externally observable
// behaviour must be identical.
//
// Env:
//   - PAX_EVM_WS_RUN_INTEGRATION=1 to run (set by integration scripts/CI).
//     Otherwise the test skips so `go test ./...` stays cheap.
//   - PAX_EVM_WS_URL overrides the default ws://127.0.0.1:8546.
//   - PAX_EVM_HTTP_URL overrides the default http://127.0.0.1:8545.
package ws_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
)

func wsURL() string {
	if u := os.Getenv("PAX_EVM_WS_URL"); u != "" {
		return u
	}
	return "ws://127.0.0.1:8546"
}

func TestEthSubscribeNewHeads(t *testing.T) {
	if os.Getenv("PAX_EVM_WS_RUN_INTEGRATION") != "1" {
		t.Skip("EVM WS integration tests skipped (set PAX_EVM_WS_RUN_INTEGRATION=1 to run)")
	}

	url := wsURL()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", url, err)
	}
	defer conn.Close()

	raw, err := readNewHead(conn)
	if err != nil {
		t.Fatal(err)
	}
	httpURL := os.Getenv("PAX_EVM_HTTP_URL")
	if httpURL == "" {
		httpURL = "http://127.0.0.1:8545"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := rpc.DialContext(ctx, httpURL)
	if err != nil {
		t.Fatalf("dial %s: %v", httpURL, err)
	}
	defer client.Close()
	header, err := checkCommittedHead(ctx, client, raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("received committed head: number=%s", header.Number)
}

func readNewHead(conn *websocket.Conn) (json.RawMessage, error) {
	if err := conn.WriteJSON(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "eth_subscribe",
		"params":  []string{"newHeads"},
	}); err != nil {
		return nil, fmt.Errorf("write subscribe: %w", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}
	var ack struct {
		Result string                 `json:"result"`
		Error  map[string]interface{} `json:"error"`
	}
	if err := conn.ReadJSON(&ack); err != nil {
		return nil, fmt.Errorf("read subscribe ack: %w", err)
	}
	if ack.Error != nil {
		return nil, fmt.Errorf("subscribe error: %v", ack.Error)
	}
	if ack.Result == "" {
		return nil, fmt.Errorf("subscribe returned empty subscription id")
	}
	if err := conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}
	var note struct {
		Method string `json:"method"`
		Params struct {
			Subscription string          `json:"subscription"`
			Result       json.RawMessage `json:"result"`
		} `json:"params"`
	}
	if err := conn.ReadJSON(&note); err != nil {
		return nil, fmt.Errorf("read head notification: %w", err)
	}
	if note.Method != "eth_subscription" {
		return nil, fmt.Errorf("expected eth_subscription, got %q", note.Method)
	}
	if note.Params.Subscription != ack.Result {
		return nil, fmt.Errorf("subscription id mismatch: got %q want %q", note.Params.Subscription, ack.Result)
	}
	return note.Params.Result, nil
}

var committedHeadFields = []string{
	"hash", "number", "parentHash", "sha3Uncles", "miner", "stateRoot",
	"transactionsRoot", "receiptsRoot", "logsBloom", "difficulty", "gasLimit",
	"gasUsed", "timestamp", "extraData", "mixHash", "nonce", "baseFeePerGas",
	"withdrawalsRoot", "blobGasUsed", "excessBlobGas", "parentBeaconBlockRoot",
}

func decodeHead(raw json.RawMessage) (*ethtypes.Header, map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, nil, fmt.Errorf("decode header fields: %w", err)
	}
	for _, field := range committedHeadFields {
		if len(fields[field]) == 0 || bytes.Equal(fields[field], []byte("null")) {
			return nil, nil, fmt.Errorf("header missing %s", field)
		}
	}
	var header ethtypes.Header
	if err := json.Unmarshal(raw, &header); err != nil {
		return nil, nil, fmt.Errorf("decode full header: %w", err)
	}
	// Pax's wire hash is the committed consensus identity, not Header.Hash().
	var hash common.Hash
	if err := json.Unmarshal(fields["hash"], &hash); err != nil {
		return nil, nil, fmt.Errorf("decode header hash: %w", err)
	}
	if hash == (common.Hash{}) || header.Number.Sign() <= 0 || header.Time == 0 {
		return nil, nil, fmt.Errorf("header has zero hash, number or timestamp")
	}
	if header.GasUsed > header.GasLimit {
		return nil, nil, fmt.Errorf("header gasUsed exceeds gasLimit")
	}
	encoded, err := json.Marshal(&header)
	if err != nil {
		return nil, nil, fmt.Errorf("encode full header: %w", err)
	}
	var canonical map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &canonical); err != nil {
		return nil, nil, err
	}
	canonical["hash"], err = json.Marshal(hash)
	if err != nil {
		return nil, nil, err
	}
	for field, expected := range canonical {
		if actual, ok := fields[field]; ok {
			var compact bytes.Buffer
			if err := json.Compact(&compact, actual); err != nil {
				return nil, nil, err
			}
			if !bytes.Equal(compact.Bytes(), expected) {
				return nil, nil, fmt.Errorf("header %s is not canonically encoded", field)
			}
		}
	}
	return &header, canonical, nil
}

func checkCommittedHead(ctx context.Context, client *rpc.Client, raw json.RawMessage) (*ethtypes.Header, error) {
	header, headFields, err := decodeHead(raw)
	if err != nil {
		return nil, fmt.Errorf("newHeads: %w", err)
	}
	var block json.RawMessage
	if err := client.CallContext(ctx, &block, "eth_getBlockByNumber", hexutil.EncodeBig(header.Number), false); err != nil {
		return nil, fmt.Errorf("read committed block: %w", err)
	}
	_, blockFields, err := decodeHead(block)
	if err != nil {
		return nil, fmt.Errorf("committed block: %w", err)
	}
	for _, field := range append(append([]string{}, committedHeadFields...), "requestsHash") {
		if !bytes.Equal(headFields[field], blockFields[field]) {
			return nil, fmt.Errorf("committed header %s mismatch: newHeads=%s block=%s", field, headFields[field], blockFields[field])
		}
	}
	return header, nil
}
