package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/client"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/stretchr/testify/require"
)

type repair88Args struct {
	Value string `json:"value"`
}

func repair88Server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	RegisterRPCFuncs(mux, map[string]*RPCFunc{
		"uppercase": NewRPCFunc(func(_ context.Context, args *repair88Args) (string, error) {
			return strings.ToUpper(args.Value), nil
		}),
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestRepair_88_SingleRequestBatch(t *testing.T) {
	srv := repair88Server(t)
	client, err := rpcclient.NewWithHTTPClient(srv.URL, srv.Client())
	require.NoError(t, err)
	batch := client.NewRequestBatch()
	var result string
	require.NoError(t, batch.Call(t.Context(), "uppercase", &repair88Args{Value: "one request"}, &result))
	require.Equal(t, 1, batch.Count())
	results, err := batch.Send(t.Context())
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Same(t, &result, results[0])
	require.Equal(t, "ONE REQUEST", result)
	require.Zero(t, batch.Count())

	var single string
	require.NoError(t, client.Call(t.Context(), "uppercase", &repair88Args{Value: "single object"}, &single))
	require.Equal(t, "SINGLE OBJECT", single)

	var first, second string
	require.NoError(t, batch.Call(t.Context(), "uppercase", &repair88Args{Value: "first"}, &first))
	require.NoError(t, batch.Call(t.Context(), "uppercase", &repair88Args{Value: "second"}, &second))
	results, err = batch.Send(t.Context())
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, "FIRST", first)
	require.Equal(t, "SECOND", second)
}

func TestRepair_88_BatchNotifications(t *testing.T) {
	srv := repair88Server(t)
	for _, tc := range []struct {
		name      string
		method    string
		respond   bool
		wantError rpctypes.ErrorCode
	}{
		{name: "one result", method: "uppercase", respond: true},
		{name: "one error", method: "unknown", respond: true, wantError: rpctypes.CodeMethodNotFound},
		{name: "notifications only", method: "uppercase"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var notification rpctypes.RPCRequest
			require.NoError(t, notification.SetMethodAndParams("uppercase", &repair88Args{Value: "notification"}))
			requests := []rpctypes.RPCRequest{notification}
			request := rpctypes.NewRequest(88)
			if tc.respond {
				require.NoError(t, request.SetMethodAndParams(tc.method, &repair88Args{Value: "batch result"}))
				requests = append(requests, request, notification)
			}
			payload, err := json.Marshal(requests)
			require.NoError(t, err)
			payload = append([]byte(" \n\t"), payload...)
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, bytes.NewReader(payload))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			response, err := srv.Client().Do(req)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, http.StatusOK, response.StatusCode)
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			if !tc.respond {
				require.Empty(t, body)
				return
			}
			require.Equal(t, "application/json", response.Header.Get("Content-Type"))
			var responses []rpctypes.RPCResponse
			require.NoError(t, json.Unmarshal(body, &responses))
			require.Len(t, responses, 1)
			require.Equal(t, request.ID(), responses[0].ID())
			if tc.wantError != 0 {
				require.NotNil(t, responses[0].Error)
				require.Equal(t, int(tc.wantError), responses[0].Error.Code)
			} else {
				require.Nil(t, responses[0].Error)
				var result string
				require.NoError(t, json.Unmarshal(responses[0].Result, &result))
				require.Equal(t, "BATCH RESULT", result)
			}
		})
	}
}
