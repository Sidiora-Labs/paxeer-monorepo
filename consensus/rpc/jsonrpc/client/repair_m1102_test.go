package client

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func repairM1102HTTP(t *testing.T, mode string) (*Client, <-chan int) {
	t.Helper()
	mux := http.NewServeMux()
	funcs := make(map[string]*rpcserver.RPCFunc)
	for _, method := range []string{"status", "block", "block_by_hash", "block_results", "genesis", "genesis_chunked"} {
		funcs[method] = rpcserver.NewRPCFunc(func(context.Context) (string, error) { return strings.Repeat("x", 512), nil })
	}
	rpcserver.RegisterRPCFuncs(mux, funcs)
	sizes := make(chan int, 16)
	release := make(chan struct{})
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorded := httptest.NewRecorder()
		mux.ServeHTTP(recorded, r)
		body := recorded.Body.Bytes()
		sizes <- len(body)
		switch mode {
		case "known":
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			_, _ = w.Write(body)
		case "chunked":
			w.(http.Flusher).Flush()
			_, _ = w.Write(body)
		case "gzip":
			w.Header().Set("Content-Encoding", "gzip")
			stream := gzip.NewWriter(w)
			_, _ = stream.Write(body)
			_ = stream.Close()
		case "stalled":
			_, _ = w.Write(body[:129])
			w.(http.Flusher).Flush()
			select {
			case <-release:
			case <-r.Context().Done():
			}
		default:
			t.Errorf("unknown fixture delivery mode %q", mode)
		}
	}))
	t.Cleanup(endpoint.Close)
	t.Cleanup(func() { close(release) })
	c, err := NewWithHTTPClient(endpoint.URL, endpoint.Client())
	require.NoError(t, err)
	return c, sizes
}

func TestRepairM_1102_ResponseBounds(t *testing.T) {
	for _, mode := range []string{"known", "chunked", "gzip", "stalled"} {
		for _, batch := range []bool{false, true} {
			t.Run(mode+"/batch="+strconv.FormatBool(batch), func(t *testing.T) {
				c, sizes := repairM1102HTTP(t, mode)
				require.NoError(t, c.SetResponseLimits(ResponseLimits{Default: 128, Block: 128, Genesis: 128, Batch: 128}))
				ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
				defer cancel()
				result := "unchanged"
				start := time.Now()
				var err error
				if batch {
					b := c.NewRequestBatch()
					require.NoError(t, b.Call(ctx, "genesis", nil, &result))
					_, err = b.Send(ctx)
					require.Zero(t, b.Count())
				} else {
					err = c.Call(ctx, "block", nil, &result)
				}
				require.ErrorIs(t, err, ErrResponseTooLarge)
				require.Equal(t, "unchanged", result)
				require.Greater(t, <-sizes, 128)
				if mode == "stalled" {
					require.Less(t, time.Since(start), time.Second, "must reject before the producer finishes its stream")
				}
			})
		}
	}
}

func TestRepairM_1102_ResponseLimitConfiguration(t *testing.T) {
	c, sizes := repairM1102HTTP(t, "known")
	defaults := DefaultResponseLimits()
	require.Positive(t, defaults.Default)
	require.Greater(t, defaults.Block, defaults.Default)
	require.Greater(t, defaults.Genesis, defaults.Block)
	require.Positive(t, defaults.Batch)
	var result string
	require.NoError(t, c.Call(t.Context(), "status", nil, &result))
	require.Equal(t, strings.Repeat("x", 512), result)
	size := int64(<-sizes)
	// Every following one-digit request ID has the same envelope length.
	limits := ResponseLimits{Default: size - 1, Block: size, Genesis: size + 1, Batch: 2*size + 16}
	require.NoError(t, c.SetResponseLimits(limits))
	for _, method := range []string{"block", "block_by_hash", "block_results", "genesis", "genesis_chunked"} {
		result = ""
		require.NoError(t, c.Call(t.Context(), method, nil, &result), method)
		require.Equal(t, strings.Repeat("x", 512), result)
		require.EqualValues(t, size, <-sizes)
	}
	result = "unchanged"
	require.ErrorIs(t, c.Call(t.Context(), "status", nil, &result), ErrResponseTooLarge)
	require.Equal(t, "unchanged", result)
	<-sizes
	b := c.NewRequestBatch()
	var first, second string
	require.NoError(t, b.Call(t.Context(), "block", nil, &first))
	require.NoError(t, b.Call(t.Context(), "genesis", nil, &second))
	_, err := b.Send(t.Context())
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("x", 512), first)
	require.Equal(t, first, second)
	require.LessOrEqual(t, int64(<-sizes), limits.Batch)
	for _, bad := range []ResponseLimits{
		{0, 1, 1, 1}, {1, -1, 1, 1}, {1, 1, 0, 1}, {1, 1, 1, 0}, {1, 1, 1, 1<<63 - 1},
	} {
		require.Error(t, c.SetResponseLimits(bad))
		require.Equal(t, limits, c.responseLimits)
	}
}

type repairM1102Event struct {
	Query      string `json:"query"`
	Generation int    `json:"generation"`
}

type repairM1102Peer struct {
	server      *httptest.Server
	connections chan *websocket.Conn
	generation  int
}

func repairM1102Server(t *testing.T, address string, generation int) *repairM1102Peer {
	t.Helper()
	listener, err := net.Listen("tcp", address)
	require.NoError(t, err)
	peer := &repairM1102Peer{connections: make(chan *websocket.Conn, 16), generation: generation}
	upgrade := websocket.Upgrader{}
	peer.server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrade.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		peer.connections <- conn
		for {
			var req rpctypes.RPCRequest
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			if req.Method != "subscribe" {
				t.Errorf("unexpected method: %s", req.Method)
				return
			}
			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				t.Errorf("decode subscription: %v", err)
				return
			}
			if err := conn.WriteJSON(req.MakeResponse(repairM1102Event{Query: params.Query, Generation: peer.generation})); err != nil {
				return
			}
		}
	}))
	peer.server.Listener = listener
	peer.server.Start()
	t.Cleanup(peer.server.Close)
	return peer
}

func repairM1102Await[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value, ok := <-ch:
		require.True(t, ok, "channel closed before observation")
		return value
	case <-time.After(8 * time.Second):
		t.Fatal("timed out waiting for completed lifecycle observation")
		var zero T
		return zero
	}
}

func repairM1102Start(t *testing.T, peer *repairM1102Peer, reconnect func(*WSClient, context.Context)) (*WSClient, context.Context) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	c, err := NewWS(peer.server.URL, "/websocket")
	require.NoError(t, err)
	if reconnect != nil {
		c.OnReconnect(func() { reconnect(c, ctx) })
	}
	require.NoError(t, c.Start(ctx))
	t.Cleanup(func() {
		cancel()
		stopped := make(chan error, 1)
		go func() { stopped <- c.Stop() }()
		require.NoError(t, repairM1102Await(t, stopped))
	})
	return c, ctx
}

func TestRepairM_1102_ServerRestartRestoresSubscription(t *testing.T) {
	for _, code := range []int{websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseProtocolError, websocket.CloseAbnormalClosure} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			peer := repairM1102Server(t, "127.0.0.1:0", 1)
			callbacks := make(chan error, 4)
			const query = "tm.event='NewBlock'"
			c, ctx := repairM1102Start(t, peer, func(c *WSClient, ctx context.Context) { callbacks <- c.Subscribe(ctx, query) })
			require.Zero(t, c.pingPeriod)
			conn := repairM1102Await(t, peer.connections)
			require.NoError(t, c.Subscribe(ctx, query))
			check := func(generation int) {
				resp := repairM1102Await(t, c.ResponsesCh)
				require.Nil(t, resp.Error)
				var event repairM1102Event
				require.NoError(t, tmjson.Unmarshal(resp.Result, &event))
				require.Equal(t, repairM1102Event{Query: query, Generation: generation}, event)
			}
			check(1)
			address := peer.server.Listener.Addr().String()
			peer.server.Close()
			if code != websocket.CloseAbnormalClosure {
				require.NoError(t, conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, "restart"), time.Now().Add(time.Second)))
			}
			require.NoError(t, conn.Close())
			reopened := repairM1102Server(t, address, 2)
			// No caller request or ping wakes the idle writer: recovery must do so.
			require.NoError(t, repairM1102Await(t, callbacks))
			check(2)
			replacement := repairM1102Await(t, reopened.connections)
			t.Cleanup(func() { _ = replacement.Close() })
		})
	}
}

type repairM1102Dial struct {
	started, finished time.Time
	err               error
}

func repairM1102ReconnectWithJitter(t *testing.T, clients int, maxAttempts uint) {
	t.Helper()
	peer := repairM1102Server(t, "127.0.0.1:0", 1)
	type observedClient struct {
		client   *WSClient
		conn     *websocket.Conn
		mu       sync.Mutex
		attempts []repairM1102Dial
		closedAt time.Time
	}
	observed := make([]*observedClient, clients)
	for i := range observed {
		o := &observedClient{}
		c, err := NewWS(peer.server.URL, "/websocket")
		require.NoError(t, err)
		c.maxReconnectAttempts = maxAttempts
		dial := c.Dialer
		c.Dialer = func(network, address string) (net.Conn, error) {
			start := time.Now()
			conn, err := dial(network, address)
			o.mu.Lock()
			o.attempts = append(o.attempts, repairM1102Dial{start, time.Now(), err})
			o.mu.Unlock()
			return conn, err
		}
		ctx, cancel := context.WithCancel(t.Context())
		require.NoError(t, c.Start(ctx))
		o.client = c
		o.conn = repairM1102Await(t, peer.connections)
		t.Cleanup(func() { cancel(); require.NoError(t, c.Stop()) })
		observed[i] = o
	}
	peer.server.Close() // Redials now fail against a real stopped listener.
	completed := make(chan error, clients)
	for _, o := range observed {
		o.closedAt = time.Now()
		require.NoError(t, o.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "stopping"), time.Now().Add(time.Second)))
		require.NoError(t, o.conn.Close())
		go func() { completed <- o.client.Stop() }()
	}
	// Wait for terminal policy, not merely for the scheduling of reconnects.
	timeout := time.NewTimer(time.Duration((1<<(maxAttempts+1))-1+int(maxAttempts)+1)*time.Second + 5*time.Second)
	defer timeout.Stop()
	for range observed {
		select {
		case err := <-completed:
			require.NoError(t, err)
		case <-timeout.C:
			t.Fatal("reconnect attempts did not complete")
		}
	}
	seen := make(map[time.Duration]bool)
	count := 0
	for _, o := range observed {
		o.mu.Lock()
		attempts := append([]repairM1102Dial(nil), o.attempts...)
		o.mu.Unlock()
		require.Len(t, attempts, int(maxAttempts)+2, "initial dial plus every completed retry")
		require.NoError(t, attempts[0].err)
		previous := o.closedAt
		for attempt, dial := range attempts[1:] {
			require.Error(t, dial.err, "must observe an actual failed connection attempt")
			delay := dial.started.Sub(previous)
			base := time.Duration(1<<attempt) * time.Second
			require.GreaterOrEqual(t, delay, base)
			require.Less(t, delay, base+time.Second+500*time.Millisecond, "bounded jitter plus scheduling allowance")
			jitter := delay - base
			require.False(t, seen[jitter], "repeated measured reconnect delay")
			seen[jitter] = true
			count++
			t.Logf("completed reconnect attempt=%d delay=%s", attempt+1, delay)
			previous = dial.finished
		}
		_, open := <-o.client.ResponsesCh
		require.False(t, open, "exhausted recovery must visibly terminate the response stream")
		require.False(t, o.client.IsReconnecting())
	}
	require.Equal(t, clients*(int(maxAttempts)+1), count)
	require.Greater(t, len(seen), 1, "qualification must observe multiple actual delays")
}

func TestRepairM_1102_ReconnectWithJitter(t *testing.T) {
	repairM1102ReconnectWithJitter(t, 4, 1)
}

func TestRepairM_1102_CancelDuringReconnect(t *testing.T) {
	peer := repairM1102Server(t, "127.0.0.1:0", 1)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	c, err := NewWS(peer.server.URL, "/websocket")
	require.NoError(t, err)
	c.maxReconnectAttempts = 0
	require.NoError(t, c.Start(ctx))
	conn := repairM1102Await(t, peer.connections)
	peer.server.Close()
	require.NoError(t, conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "stop"), time.Now().Add(time.Second)))
	require.NoError(t, conn.Close())
	require.Eventually(t, c.IsReconnecting, time.Second, time.Millisecond)
	cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- c.Stop() }()
	require.NoError(t, repairM1102Await(t, stopped))
	require.NoError(t, c.Stop(), "waiting for shutdown is safe more than once")
	_, open := <-c.ResponsesCh
	require.False(t, open)
	require.False(t, c.IsReconnecting())
	require.True(t, errors.Is(ctx.Err(), context.Canceled))
}
