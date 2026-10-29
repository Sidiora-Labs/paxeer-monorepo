// Copyright 2026 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package rpc

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHTTPAppliesDenyListMixedBatches(t *testing.T) {
	testDenyListMixedBatches(t, false)
}

func TestWSAppliesDenyListMixedBatches(t *testing.T) {
	testDenyListMixedBatches(t, true)
}

func testDenyListMixedBatches(t *testing.T, useWebSocket bool) {
	t.Helper()
	server := NewServer()
	t.Cleanup(server.Stop)
	// Use real metadata methods, including registered methods that would succeed
	// without the deny list, rather than an EVM service double.
	for _, namespace := range []string{"allowed", "blocked"} {
		if err := server.RegisterName(namespace, &RPCService{server}); err != nil {
			t.Fatal(err)
		}
	}
	for _, method := range []string{"rpc_modules", "blocked_modules", "eth_subscribe", "eth_unsubscribe"} {
		server.RegisterDenyList(method)
	}
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	wsServer := httptest.NewServer(server.WebsocketHandler([]string{"*"}))
	t.Cleanup(wsServer.Close)
	httpClient := &http.Client{Timeout: 5 * time.Second}
	t.Cleanup(httpClient.CloseIdleConnections)
	var conn *websocket.Conn
	if useWebSocket {
		var err error
		dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
		conn, _, err = dialer.Dial("ws"+strings.TrimPrefix(wsServer.URL, "http"), nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
	}
	exchange := func(t *testing.T, body string) []byte {
		t.Helper()
		if useWebSocket {
			if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(body)); err != nil {
				t.Fatal(err)
			}
			if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			_, reply, err := conn.ReadMessage()
			if err != nil {
				t.Fatal(err)
			}
			return reply
		}
		resp, err := httpClient.Post(httpServer.URL, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("HTTP status: got %d, want %d", resp.StatusCode, http.StatusOK)
		}
		reply, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return reply
	}
	for _, tc := range []struct{ name, body, want string }{
		{
			name: "denied_then_allowed",
			body: `[{"jsonrpc":"2.0","id":1,"method":"rpc_modules","params":[]},{"jsonrpc":"2.0","id":2,"method":"allowed_modules","params":[]}]`,
			want: `[{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"the method rpc_modules does not exist/is not available"}},{"jsonrpc":"2.0","id":2,"result":{"rpc":"1.0","allowed":"1.0","blocked":"1.0"}}]`,
		},
		{
			name: "allowed_denied_allowed_duplicate_ids",
			body: `[{"jsonrpc":"2.0","id":"same","method":"allowed_modules"},{"jsonrpc":"2.0","id":"same","method":"rpc_modules"},{"jsonrpc":"2.0","id":"same","method":"allowed_modules"}]`,
			want: `[{"jsonrpc":"2.0","id":"same","result":{"rpc":"1.0","allowed":"1.0","blocked":"1.0"}},{"jsonrpc":"2.0","id":"same","error":{"code":-32601,"message":"the method rpc_modules does not exist/is not available"}},{"jsonrpc":"2.0","id":"same","result":{"rpc":"1.0","allowed":"1.0","blocked":"1.0"}}]`,
		},
		{
			name: "all_denied_distinct_methods_and_id_types",
			body: `[{"jsonrpc":"2.0","id":null,"method":"rpc_modules"},{"jsonrpc":"2.0","id":9007199254740993,"method":"blocked_modules"},{"jsonrpc":"2.0","id":"text","method":"rpc_modules"}]`,
			want: `[{"jsonrpc":"2.0","id":null,"error":{"code":-32601,"message":"the method rpc_modules does not exist/is not available"}},{"jsonrpc":"2.0","id":9007199254740993,"error":{"code":-32601,"message":"the method blocked_modules does not exist/is not available"}},{"jsonrpc":"2.0","id":"text","error":{"code":-32601,"message":"the method rpc_modules does not exist/is not available"}}]`,
		},
		{
			name: "denied_subscription_builtins",
			body: `[{"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":["newHeads"]},{"jsonrpc":"2.0","id":2,"method":"eth_unsubscribe","params":["0x1"]},{"jsonrpc":"2.0","id":3,"method":"allowed_modules"}]`,
			want: `[{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"the method eth_subscribe does not exist/is not available"}},{"jsonrpc":"2.0","id":2,"error":{"code":-32601,"message":"the method eth_unsubscribe does not exist/is not available"}},{"jsonrpc":"2.0","id":3,"result":{"rpc":"1.0","allowed":"1.0","blocked":"1.0"}}]`,
		},
		{
			name: "single_denied_request",
			body: `{"jsonrpc":"2.0","id":"single","method":"rpc_modules","params":[]}`,
			want: `{"jsonrpc":"2.0","id":"single","error":{"code":-32601,"message":"the method rpc_modules does not exist/is not available"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertDenyListBatchReply(t, exchange(t, tc.body), tc.want)
			assertDenyListBatchReply(t, exchange(t, `{"jsonrpc":"2.0","id":"next","method":"allowed_modules"}`),
				`{"jsonrpc":"2.0","id":"next","result":{"rpc":"1.0","allowed":"1.0","blocked":"1.0"}}`)
		})
	}
}

func assertDenyListBatchReply(t *testing.T, got []byte, want string) {
	t.Helper()
	decode := func(body string) any {
		t.Helper()
		decoder := json.NewDecoder(strings.NewReader(body))
		// Preserve large numeric IDs exactly and compare arrays in request order.
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			t.Fatalf("invalid JSON response %q: %v", body, err)
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			t.Fatalf("expected one JSON response, got trailing data in %q: %v", body, err)
		}
		return value
	}
	if !reflect.DeepEqual(decode(string(got)), decode(want)) {
		t.Fatalf("wrong response\ngot:  %s\nwant: %s", got, want)
	}
}
