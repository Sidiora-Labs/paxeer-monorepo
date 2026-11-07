package evmrpc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/cw20"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

const transportLimitRequest = `{"jsonrpc":"2.0","id":1,"method":"web3_sha3","params":["0x"]}`
const transportLimitHash = "0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"

type transportLimitResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  string          `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func transportLimitConfig() evmrpcconfig.Config {
	config := evmrpcconfig.DefaultConfig
	config.IPRateLimitRPS = 1
	config.IPRateLimitBurst = 1
	config.BatchRequestLimit = 2
	return config
}

func newTransportLimitServer(t *testing.T, config evmrpcconfig.Config, ws bool) *HTTPServer {
	t.Helper()
	server := NewHTTPServer(rpc.DefaultHTTPTimeouts)
	require.NoError(t, server.SetListenAddr("127.0.0.1", 0))
	apis := []rpc.API{{Namespace: "web3", Service: &Web3API{}}}
	if ws {
		endpoint := newRPCEndpointConfig(config)
		endpoint.readLimit = DefaultWebsocketMaxMessageSize
		require.NoError(t, server.EnableWS(apis, WsConfig{
			Origins: []string{"http://localhost"}, DenyList: config.DenyList,
			RPCEndpointConfig: endpoint,
		}))
	} else {
		require.NoError(t, server.EnableRPC(apis, HTTPConfig{
			Vhosts: []string{"*"}, DenyList: config.DenyList,
			RPCEndpointConfig: newRPCEndpointConfig(config),
		}))
	}
	t.Cleanup(server.Stop)
	require.NoError(t, server.Start())
	return server
}

func transportLimitPOST(t *testing.T, server *HTTPServer, body, contentType string) *http.Response {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Post("http://"+server.ListenAddr(), contentType, strings.NewReader(body))
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func assertTransportRateLimit(t *testing.T, response *http.Response) {
	t.Helper()
	require.Equal(t, http.StatusTooManyRequests, response.StatusCode)
	require.Equal(t, "application/json", response.Header.Get("Content-Type"))
	var message transportLimitResponse
	require.NoError(t, json.NewDecoder(response.Body).Decode(&message))
	require.Equal(t, "2.0", message.JSONRPC)
	require.JSONEq(t, "null", string(message.ID))
	require.NotNil(t, message.Error)
	require.Equal(t, -32005, message.Error.Code)
	require.Equal(t, "rate limit exceeded", message.Error.Message)
	require.Empty(t, message.Result)
}

func TestHTTPRateLimitAnswers429(t *testing.T) {
	t.Run("HTTP", func(t *testing.T) {
		server := newTransportLimitServer(t, transportLimitConfig(), false)
		response := transportLimitPOST(t, server, transportLimitRequest, "application/json")
		require.Equal(t, http.StatusOK, response.StatusCode)
		var message transportLimitResponse
		require.NoError(t, json.NewDecoder(response.Body).Decode(&message))
		require.Nil(t, message.Error)
		require.Equal(t, transportLimitHash, message.Result)
		assertTransportRateLimit(t, transportLimitPOST(t, server, "invalid JSON", "text/plain"))
		time.Sleep(1050 * time.Millisecond)
		response = transportLimitPOST(t, server, transportLimitRequest, "application/json")
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.NoError(t, json.NewDecoder(response.Body).Decode(&message))
		require.Nil(t, message.Error)
		require.Equal(t, transportLimitHash, message.Result)
	})
	t.Run("WebSocket upgrade", func(t *testing.T) {
		server := newTransportLimitServer(t, transportLimitConfig(), true)
		dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
		conn, _, err := dialer.Dial("ws://"+server.ListenAddr(), http.Header{"Origin": {"http://localhost"}})
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(transportLimitRequest)))
		var message transportLimitResponse
		require.NoError(t, conn.ReadJSON(&message))
		require.Nil(t, message.Error)
		require.Equal(t, transportLimitHash, message.Result)
		refused, response, err := dialer.Dial("ws://"+server.ListenAddr(), http.Header{"Origin": {"http://forbidden.example"}})
		require.Error(t, err)
		require.Nil(t, refused)
		require.NotNil(t, response)
		defer response.Body.Close()
		assertTransportRateLimit(t, response)
	})
	t.Run("disabled", func(t *testing.T) {
		config := transportLimitConfig()
		config.IPRateLimitRPS = 0
		server := newTransportLimitServer(t, config, false)
		for i := 0; i < 3; i++ {
			response := transportLimitPOST(t, server, transportLimitRequest, "application/json")
			require.Equal(t, http.StatusOK, response.StatusCode)
			var message transportLimitResponse
			require.NoError(t, json.NewDecoder(response.Body).Decode(&message))
			require.Nil(t, message.Error)
			require.Equal(t, transportLimitHash, message.Result)
		}
	})
}

func TestBatchRequestLimitEnforced(t *testing.T) {
	for _, ws := range []bool{false, true} {
		for _, denyList := range [][]string{nil, {"web3_clientVersion"}} {
			name := "HTTP"
			if ws {
				name = "WebSocket"
			}
			if denyList != nil {
				name += "/deny list"
			}
			t.Run(name, func(t *testing.T) {
				config := transportLimitConfig()
				config.DenyList = denyList
				server := newTransportLimitServer(t, config, ws)
				var exchange func(string) []transportLimitResponse
				if ws {
					dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
					conn, _, err := dialer.Dial("ws://"+server.ListenAddr(), http.Header{"Origin": {"http://localhost"}})
					require.NoError(t, err)
					t.Cleanup(func() { _ = conn.Close() })
					exchange = func(body string) []transportLimitResponse {
						require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
						require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(body)))
						var responses []transportLimitResponse
						require.NoError(t, conn.ReadJSON(&responses))
						return responses
					}
				} else {
					exchange = func(body string) []transportLimitResponse {
						response := transportLimitPOST(t, server, body, "application/json")
						require.Equal(t, http.StatusOK, response.StatusCode)
						defer response.Body.Close()
						data, err := io.ReadAll(response.Body)
						require.NoError(t, err)
						var responses []transportLimitResponse
						require.NoError(t, json.Unmarshal(data, &responses))
						return responses
					}
				}
				second := strings.Replace(transportLimitRequest, `"id":1`, `"id":2`, 1)
				third := strings.Replace(transportLimitRequest, `"id":1`, `"id":3`, 1)
				responses := exchange("[" + transportLimitRequest + "," + second + "," + third + "]")
				require.Len(t, responses, 1)
				require.NotNil(t, responses[0].Error)
				require.Equal(t, -32600, responses[0].Error.Code)
				require.Equal(t, "batch too large", responses[0].Error.Message)
				require.Empty(t, responses[0].Result)
				if !ws {
					time.Sleep(1050 * time.Millisecond)
				}
				responses = exchange("[" + transportLimitRequest + "," + second + "]")
				require.Len(t, responses, 2)
				for i, response := range responses {
					require.Equal(t, "2.0", response.JSONRPC)
					require.Equal(t, json.RawMessage{byte('1' + i)}, response.ID)
					require.Nil(t, response.Error)
					require.Equal(t, transportLimitHash, response.Result)
				}
			})
		}
	}
}

// RunRepairM1293LegacyBatchLimit is shared with the bounded repair gate.
func RunRepairM1293LegacyBatchLimit(t *testing.T) {
	for _, limit := range []int{1, 2, 4} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			api := NewTestAPI()
			server := NewHTTPServer(rpc.DefaultHTTPTimeouts)
			t.Cleanup(server.Stop)
			require.NoError(t, server.SetListenAddr("127.0.0.1", 0))
			config := transportLimitConfig()
			config.IPRateLimitRPS, config.BatchRequestLimit = 0, limit
			require.NoError(t, server.EnableRPC([]rpc.API{
				{Namespace: "test", Service: api}, {Namespace: "web3", Service: &Web3API{}},
			}, HTTPConfig{Vhosts: []string{"*"}, PaxLegacyAllowlist: BuildPaxLegacyEnabledSet(nil), RPCEndpointConfig: newRPCEndpointConfig(config)}))
			require.NoError(t, server.Start())
			state := sdk.Context{}.WithChainID("rpc-repair-test")
			initial := cw20.CurrentVersion(state)
			call := `{"jsonrpc":"2.0","id":7,"method":"test_incrementPointerVersion","params":["cw20",5]}`
			for _, extra := range []string{
				`{"jsonrpc":"2.0","id":8,"method":"pax_getBlockByNumber","params":["latest",false]}`,
				`{"jsonrpc":"2.0","method":"pax_getBlockByNumber","params":["latest",false]}`,
				`17`,
				`{"jsonrpc":"2.0","id":{},"method":"web3_sha3","params":["0x"]}`,
			} {
				entries := []string{call, extra}
				for len(entries) <= limit {
					entries = append(entries, `null`)
				}
				response := transportLimitPOST(t, server, "["+strings.Join(entries, ",")+"]", "application/json")
				var messages []transportLimitResponse
				require.NoError(t, json.NewDecoder(response.Body).Decode(&messages))
				require.Len(t, messages, 1)
				require.NotNil(t, messages[0].Error)
				require.Equal(t, -32600, messages[0].Error.Code)
				require.Equal(t, "batch too large", messages[0].Error.Message)
				require.Equal(t, initial, cw20.CurrentVersion(state), "rejected batch executed a forwarded method")
			}
			response := transportLimitPOST(t, server, "["+call+"]", "application/json")
			var messages []json.RawMessage
			require.NoError(t, json.NewDecoder(response.Body).Decode(&messages))
			require.Len(t, messages, 1)
			require.Equal(t, initial+5, cw20.CurrentVersion(state), "in-limit call must execute")
		})
	}
}

func TestLegacyBatchRequestLimitEnforced(t *testing.T) { RunRepairM1293LegacyBatchLimit(t) }

func RepairM1293EnableHTTPAPIsWS(t *testing.T, server *HTTPServer) {
	t.Helper()
	require.NoError(t, server.EnableWS(server.apis, WsConfig{
		Origins: []string{"http://localhost"}, RPCEndpointConfig: server.HTTPConfig.RPCEndpointConfig,
	}))
	installTxPoolWSScopes(server)
}

func RepairM1293Baker(t *testing.T, cache *keeper.TraceDB) (*TraceBaker, <-chan struct{}) {
	t.Helper()
	serverTraceBakers.Lock()
	defer serverTraceBakers.Unlock()
	baker := serverTraceBakers.owners[cache]
	require.NotNil(t, baker)
	t.Cleanup(baker.Stop)
	joined := make(chan struct{})
	go func() {
		<-baker.done
		baker.wg.Wait()
		close(joined)
	}()
	return baker, joined
}

func RepairM1293ServerStopped(t *testing.T, server *HTTPServer) func() {
	t.Helper()
	server.mu.Lock()
	done := server.serveDone
	server.mu.Unlock()
	require.NotNil(t, done)
	return func() {
		t.Helper()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("server listener did not finish shutdown")
		}
	}
}

func RepairM1293SubscriptionStopped(t *testing.T, server *HTTPServer) func() {
	t.Helper()
	var subscriptions []*SubscriptionAPI
	for _, api := range server.wsAPIs {
		if subscription, ok := api.Service.(*SubscriptionAPI); ok {
			subscriptions = append(subscriptions, subscription)
		}
	}
	require.Len(t, subscriptions, 1)
	return func() {
		for _, api := range subscriptions {
			require.ErrorIs(t, api.shutdownCtx.Err(), context.Canceled)
			joined := make(chan struct{})
			go func() { api.wg.Wait(); close(joined) }()
			select {
			case <-joined:
			case <-time.After(time.Second):
				t.Fatal("subscription consumers did not join")
			}
			api.newHeadListenersMtx.RLock()
			require.Empty(t, api.newHeadListeners)
			api.newHeadListenersMtx.RUnlock()
			api.logSubscriptionsMtx.Lock()
			require.Empty(t, api.logSubscriptions)
			api.logSubscriptionsMtx.Unlock()
			if api.subscriptionManager != nil {
				api.subscriptionManager.subMu.Lock()
				require.Empty(t, api.subscriptionManager.SubscriptionInfo)
				api.subscriptionManager.subMu.Unlock()
			}
		}
	}
}

func RepairM1293Metrics(t *testing.T) func() map[string]int64 {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previousMeter, previousMetrics := rpcTelemetryMeter, metrics
	rpcTelemetryMeter = provider.Meter("evmrpc")
	metrics = newRPCMetrics(rpcTelemetryMeter)
	t.Cleanup(func() {
		rpcTelemetryMeter, metrics = previousMeter, previousMetrics
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	return func() map[string]int64 {
		var data metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(context.Background(), &data))
		values := map[string]int64{}
		for _, scope := range data.ScopeMetrics {
			for _, instrument := range scope.Metrics {
				if sum, ok := instrument.Data.(metricdata.Sum[int64]); ok {
					for _, point := range sum.DataPoints {
						values[instrument.Name] += point.Value
					}
				}
			}
		}
		return values
	}
}
