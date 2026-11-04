package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/client"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1109_BatchArrays(t *testing.T) {
	TestRPCNotificationInBatch(t)
}

func TestRepairM_1109_RequestBatchNotifications(t *testing.T) {
	for _, tc := range rpcNotificationBatchCases() {
		t.Run(fmt.Sprintf("responses_%d", tc.expectCount), func(t *testing.T) {
			var template []rpctypes.RPCRequest
			require.NoError(t, json.Unmarshal([]byte(tc.payload), &template))

			backend := httptest.NewServer(testMux())
			t.Cleanup(backend.Close)
			target, err := url.Parse(backend.URL)
			require.NoError(t, err)
			proxy := httputil.NewSingleHostReverseProxy(target)
			responseBodies := make(chan []byte, 1)
			proxy.ModifyResponse = func(response *http.Response) error {
				body, err := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if err != nil {
					return err
				}
				responseBodies <- body
				response.Body = io.NopCloser(bytes.NewReader(body))
				return nil
			}

			requestBatches := make(chan []rpctypes.RPCRequest, 1)
			// RequestBatch has no notification API. This loopback gateway adds
			// the fixture notifications while preserving every client-produced
			// call and forwards the production server's response unchanged.
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var calls []rpctypes.RPCRequest
				err := json.NewDecoder(r.Body).Decode(&calls)
				_ = r.Body.Close()
				if err != nil || len(calls) != tc.expectCount {
					http.Error(w, "invalid client batch", http.StatusBadRequest)
					return
				}
				requests := make([]rpctypes.RPCRequest, 0, len(template))
				next := 0
				for _, request := range template {
					if !request.IsNotification() {
						request = calls[next]
						next++
					}
					requests = append(requests, request)
				}
				body, err := json.Marshal(requests)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				requestBatches <- requests
				r.Body = io.NopCloser(bytes.NewReader(body))
				r.ContentLength = int64(len(body))
				proxy.ServeHTTP(w, r)
			}))
			t.Cleanup(gateway.Close)

			client, err := rpcclient.NewWithHTTPClient(gateway.URL, gateway.Client())
			require.NoError(t, err)
			batch := client.NewRequestBatch()
			results := make([]string, tc.expectCount)
			next := 0
			for _, request := range template {
				if !request.IsNotification() {
					require.NoError(t, batch.Call(t.Context(), request.Method, request.Params, &results[next]))
					next++
				}
			}
			require.Equal(t, tc.expectCount, batch.Count())
			decoded, err := batch.Send(t.Context())
			require.NoError(t, err)
			require.Len(t, decoded, tc.expectCount)
			require.Zero(t, batch.Count())

			requests := <-requestBatches
			require.Len(t, requests, len(template))
			var responses []rpctypes.RPCResponse
			require.NoError(t, json.Unmarshal(<-responseBodies, &responses), "every batch response must be an array")
			require.Len(t, responses, tc.expectCount)
			next = 0
			ids := make(map[string]bool)
			for i, request := range requests {
				require.Equal(t, template[i].IsNotification(), request.IsNotification())
				if request.IsNotification() {
					continue
				}
				require.Equal(t, template[i].Method, request.Method)
				require.JSONEq(t, string(template[i].Params), string(request.Params))
				require.NotEmpty(t, request.ID())
				require.False(t, ids[request.ID()], "client must generate distinct IDs")
				ids[request.ID()] = true
				require.Equal(t, request.ID(), responses[next].ID())
				require.Nil(t, responses[next].Error)
				require.JSONEq(t, `"foo"`, string(responses[next].Result))
				require.Same(t, &results[next], decoded[next])
				require.Equal(t, "foo", results[next])
				next++
			}
		})
	}
}
