package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/client"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
)

func repairM1108Server(t *testing.T) (string, *http.Client) {
	t.Helper()
	config := rpcserver.DefaultConfig()
	listener, err := rpcserver.Listen("tcp://127.0.0.1:0", config.MaxOpenConnections)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	stopped := make(chan error, 1)
	go func() { stopped <- serveExample(ctx, listener, config) }()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	t.Cleanup(func() {
		client.CloseIdleConnections()
		cancel()
		select {
		case err := <-stopped:
			if !errors.Is(err, http.ErrServerClosed) {
				t.Errorf("example shutdown: %v", err)
			}
		case <-time.After(5 * time.Second):
			_ = listener.Close()
			t.Error("example failed to shut down after cancellation")
		}
	})
	return "http://" + listener.Addr().String(), client
}

func repairM1108HTTP(t *testing.T, client *http.Client, method, endpoint string, body []byte, status int, contentType string) []byte {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, endpoint, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != status || resp.Header.Get("Content-Type") != contentType {
		t.Fatalf("HTTP status/type = %d %q, want %d %q; body=%s", resp.StatusCode, resp.Header.Get("Content-Type"), status, contentType, data)
	}
	return data
}

func repairM1108JSONEqual(t *testing.T, actual []byte, expected string) {
	t.Helper()
	var got, want any
	if err := json.Unmarshal(actual, &got); err != nil {
		t.Fatalf("response is not JSON: %v; body=%s", err, actual)
	}
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("response=%s, want %s", actual, expected)
	}
}

func repairM1108Request(t *testing.T, id int, params any) []byte {
	t.Helper()
	req := rpctypes.NewRequest(id)
	if err := req.SetMethodAndParams("hello_world", params); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRepairM_1108_TypedTransports(t *testing.T) {
	endpoint, client := repairM1108Server(t)
	caller, err := rpcclient.NewWithHTTPClient(endpoint, client)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		request HelloWorldRequest
		want    string
	}{
		{"named", HelloWorldRequest{Name: "my_world", Num: 5}, "hi my_world 5"},
		{"escaped", HelloWorldRequest{Name: "world & + / 雪", Num: -7}, "hi world & + / 雪 -7"},
		{"zero", HelloWorldRequest{}, "hi  0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result Result
			if err := caller.Call(t.Context(), "hello_world", &tc.request, &result); err != nil {
				t.Fatal(err)
			}
			if result.Result != tc.want {
				t.Fatalf("JSON-RPC result=%q, want %q", result.Result, tc.want)
			}
			name, err := json.Marshal(tc.request.Name)
			if err != nil {
				t.Fatal(err)
			}
			num, err := json.Marshal(tc.request.Num)
			if err != nil {
				t.Fatal(err)
			}
			query := url.Values{"name": {string(name)}, "num": {string(num)}}
			body := repairM1108HTTP(t, client, http.MethodGet, endpoint+"/hello_world?"+query.Encode(), nil, http.StatusOK, "application/json")
			want, err := json.Marshal(Result{Result: tc.want})
			if err != nil {
				t.Fatal(err)
			}
			repairM1108JSONEqual(t, body, string(want))
		})
	}
}

func TestRepairM_1108_TransportContracts(t *testing.T) {
	endpoint, client := repairM1108Server(t)
	for _, tc := range []struct {
		name  string
		query url.Values
		want  string
	}{
		{"quoted", url.Values{"name": {`"my_world"`}, "num": {"5"}}, `{"Result":"hi my_world 5"}`},
		{"hex_name", url.Values{"name": {"0x41424344"}, "num": {"123"}}, `{"Result":"hi ABCD 123"}`},
		{"missing", url.Values{}, `{"Result":"hi  0"}`},
		{"unquoted", url.Values{"name": {"abcd"}, "num": {"123"}}, `{"Result":"hi abcd 123"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := repairM1108HTTP(t, client, http.MethodGet, endpoint+"/hello_world?"+tc.query.Encode(), nil, http.StatusOK, "application/json")
			repairM1108JSONEqual(t, body, tc.want)
		})
	}
	fixture, err := os.ReadFile("data.json")
	if err != nil {
		t.Fatal(err)
	}
	body := repairM1108HTTP(t, client, http.MethodPost, endpoint+"/", fixture, http.StatusOK, "application/json")
	repairM1108JSONEqual(t, body, `{"jsonrpc":"2.0","id":"","result":{"Result":"hi my_world 5"}}`)

	body = repairM1108HTTP(t, client, http.MethodPost, endpoint+"/", repairM1108Request(t, 8, []any{"positional", 12}), http.StatusOK, "application/json")
	repairM1108JSONEqual(t, body, `{"jsonrpc":"2.0","id":8,"result":{"Result":"hi positional 12"}}`)

	body = repairM1108HTTP(t, client, http.MethodPost, endpoint+"/", repairM1108Request(t, 9, &HelloWorldRequest{}), http.StatusOK, "application/json")
	repairM1108JSONEqual(t, body, `{"jsonrpc":"2.0","id":9,"result":{"Result":"hi  0"}}`)
}

func TestRepairM_1108_TypedRefusals(t *testing.T) {
	endpoint, client := repairM1108Server(t)
	query := url.Values{"name": {`"abcd"`}, "num": {"0xabcd"}}
	uriBody := repairM1108HTTP(t, client, http.MethodGet, endpoint+"/hello_world?"+query.Encode(), nil, http.StatusOK, "application/json")
	var uriError rpctypes.RPCError
	if err := json.Unmarshal(uriBody, &uriError); err != nil {
		t.Fatal(err)
	}
	if uriError.Code != int(rpctypes.CodeInvalidParams) || uriError.Message != rpctypes.CodeInvalidParams.String() || uriError.Data == "" {
		t.Fatalf("URI error=%+v", uriError)
	}
	encoded, err := json.Marshal(uriError)
	if err != nil {
		t.Fatal(err)
	}
	repairM1108JSONEqual(t, uriBody, string(encoded))

	params := struct {
		Name string `json:"name"`
		Num  string `json:"num"`
	}{Name: "abcd", Num: "0xabcd"}
	body := repairM1108HTTP(t, client, http.MethodPost, endpoint+"/", repairM1108Request(t, 7, params), http.StatusOK, "application/json")
	var response rpctypes.RPCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if response.ID() != "7" || len(response.Result) != 0 || response.Error == nil || *response.Error != uriError {
		t.Fatalf("JSON-RPC refusal=%s; URI error=%+v", body, uriError)
	}
	repairM1108JSONEqual(t, body, `{"jsonrpc":"2.0","id":7,"error":`+string(encoded)+`}`)

	caller, err := rpcclient.NewWithHTTPClient(endpoint, client)
	if err != nil {
		t.Fatal(err)
	}
	var result Result
	err = caller.Call(t.Context(), "hello_world", params, &result)
	var rpcError *rpctypes.RPCError
	if !errors.As(err, &rpcError) || *rpcError != uriError {
		t.Fatalf("consumer error=%v, want typed error %+v", err, uriError)
	}
	malformed := url.Values{"name": {"0xZZ"}, "num": {"123"}}
	badBody := repairM1108HTTP(t, client, http.MethodGet, endpoint+"/hello_world?"+malformed.Encode(), nil, http.StatusBadRequest, "text/plain")
	if len(bytes.TrimSpace(badBody)) == 0 || json.Valid(badBody) {
		t.Fatalf("malformed URI must have a nonempty plain-text refusal: %s", badBody)
	}
}
