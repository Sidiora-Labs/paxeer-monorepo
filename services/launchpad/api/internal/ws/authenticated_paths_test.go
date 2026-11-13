package ws_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/ws"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/migrations"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

func wsStreamFixture(t *testing.T) (*broker.PostgresStream, *pgxpool.Pool, string) {
	t.Helper()
	t.Setenv(internaltest.ExternalPostgresEnv, os.Getenv("API_TEST_DATABASE_URL"))
	pool := internaltest.NewPostgres(t)
	migration, err := migrations.Migrations().ReadFile("00001_api_stream.sql")
	if err != nil {
		t.Fatal(err)
	}
	_, up, ok := strings.Cut(string(migration), "-- +goose Up")
	if !ok {
		t.Fatal("stream migration missing Up")
	}
	up, _, ok = strings.Cut(up, "-- +goose Down")
	if !ok {
		t.Fatal("stream migration missing Down")
	}
	if _, err = pool.Exec(context.Background(), up); err != nil {
		t.Fatal(err)
	}
	stream, err := broker.NewPostgresStream(pool, "api_stream", bytes.Repeat([]byte{9}, 32), sharedauth.HTTPStreamAuthority{})
	if err != nil {
		t.Fatal(err)
	}
	address := "0x" + strings.Repeat("1", 40)
	topic := "market.price." + address
	asset := map[string]any{"kind": "erc20", "chainId": "125", "address": address, "currencyCode": nil, "decimals": 18}
	quote := map[string]any{"kind": "erc20", "chainId": "125", "address": "0x" + strings.Repeat("2", 40), "currencyCode": nil, "decimals": 6}
	payload, err := json.Marshal(map[string]any{"numerator": "1", "denominator": "1", "baseAsset": asset, "quoteAsset": quote, "source": map[string]any{"kind": "post_swap_spot", "amountBasis": "effective_reserves", "sourceEventId": nil, "position": nil, "providerId": nil, "observedAt": "2026-10-12T00:00:00.000Z", "sourceReference": nil}})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	f := broker.Frame{EventID: uuid.NewString(), Topic: topic, Sequence: "1", Revision: "1", OccurredAt: "2026-10-12T00:00:00.000Z", Watermark: json.RawMessage("null"), Payload: payload}
	if err = stream.AppendProjectionTx(context.Background(), tx, f); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return stream, pool, topic
}

func TestAuthenticatedWebSocketRejectsInvalidHandshakeAndSubscribe(t *testing.T) {
	stream, _, topic := wsStreamFixture(t)
	const origin = "https://web.example"
	srv, _ := newServer(t, ws.Options{PrivateStream: stream, AllowedOrigins: []string{origin}})
	endpoint := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v2/stream"
	for _, check := range []struct {
		name, path, requestOrigin string
		protocols                 []string
		status                    int
	}{
		{"missing protocol", endpoint, origin, nil, 401},
		{"unknown protocol", endpoint, origin, []string{"sidiora.v1"}, 401},
		{"wrong origin", endpoint, "https://other.example", []string{"sidiora.v2"}, 403},
		{"query string", endpoint + "?cursor=x", origin, []string{"sidiora.v2"}, 403},
	} {
		t.Run(check.name, func(t *testing.T) {
			connection, response, err := (&websocket.Dialer{Subprotocols: check.protocols}).Dial(check.path, http.Header{"Origin": []string{check.requestOrigin}})
			if connection != nil {
				_ = connection.Close()
			}
			if response != nil {
				defer response.Body.Close()
			}
			if err == nil || response == nil || response.StatusCode != check.status {
				t.Fatalf("handshake: status=%v error=%v", response, err)
			}
		})
	}
	for _, check := range []struct {
		name string
		body []byte
	}{
		{"malformed JSON", []byte(`{`)},
		{"invalid topic", []byte(`{"op":"subscribe","subscriptionId":"price","topics":["market.price.*"],"afterCursor":null}`)},
		{"wrong operation", []byte(`{"op":"ping"}`)},
	} {
		t.Run(check.name, func(t *testing.T) {
			connection, response, err := (&websocket.Dialer{Subprotocols: []string{"sidiora.v2"}}).Dial(endpoint, http.Header{"Origin": []string{origin}})
			if err != nil {
				t.Fatalf("connect: %v %v", err, response)
			}
			defer connection.Close()
			if err := connection.WriteMessage(websocket.TextMessage, check.body); err != nil {
				t.Fatal(err)
			}
			_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, _, err = connection.ReadMessage()
			var closeError *websocket.CloseError
			if !errors.As(err, &closeError) || closeError.Code != websocket.ClosePolicyViolation {
				t.Fatalf("invalid subscribe closed with %v, want policy violation", err)
			}
		})
	}
	connection, response, err := (&websocket.Dialer{Subprotocols: []string{"sidiora.v2"}}).Dial(endpoint, http.Header{"Origin": []string{origin}})
	if err != nil {
		t.Fatalf("connect: %v %v", err, response)
	}
	defer connection.Close()
	request, err := json.Marshal(map[string]any{"op": "subscribe", "subscriptionId": "price", "topics": []string{topic}, "afterCursor": nil})
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.WriteMessage(websocket.BinaryMessage, request); err != nil {
		t.Fatal(err)
	}
	_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err = connection.ReadMessage()
	var closeError *websocket.CloseError
	if !errors.As(err, &closeError) || closeError.Code != websocket.ClosePolicyViolation {
		t.Fatalf("binary subscribe closed with %v, want policy violation", err)
	}
}

func TestAuthenticatedWebSocketSnapshotAndSubscription(t *testing.T) {
	stream, _, topic := wsStreamFixture(t)
	const origin = "https://web.example"
	srv, _ := newServer(t, ws.Options{PrivateStream: stream, AllowedOrigins: []string{origin}})
	request, err := http.NewRequest(http.MethodGet, srv.URL+"/v2/snapshots?topics="+topic, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", origin)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var snapshot struct {
		Data struct {
			Snapshot struct {
				ResumeCursor string `json:"resumeCursor"`
			} `json:"snapshot"`
		} `json:"data"`
	}
	if err = json.NewDecoder(response.Body).Decode(&snapshot); err != nil || response.StatusCode != 200 || snapshot.Data.Snapshot.ResumeCursor == "" {
		t.Fatalf("snapshot status %d cursor %q error %v", response.StatusCode, snapshot.Data.Snapshot.ResumeCursor, err)
	}
	dial := func(t *testing.T) *websocket.Conn {
		t.Helper()
		url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v2/stream"
		conn, response, err := (&websocket.Dialer{Subprotocols: []string{"sidiora.v2"}}).Dial(url, http.Header{"Origin": []string{origin}})
		if err != nil {
			t.Fatalf("dial: %v, response %v", err, response)
		}
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	for _, tc := range []struct{ name, cursor, action, want string }{
		{"snapshot required", "", "", "resnapshot_required"},
		{"resume and ping", snapshot.Data.Snapshot.ResumeCursor, `{"op":"ping"}`, "heartbeat"},
		{"scope change", snapshot.Data.Snapshot.ResumeCursor, `{"op":"unsubscribe"}`, "resnapshot_required"},
		{"invalid action", snapshot.Data.Snapshot.ResumeCursor, `{"op":"invalid"}`, "error"},
		{"malformed action", snapshot.Data.Snapshot.ResumeCursor, `{`, "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := dial(t)
			var cursor any
			if tc.cursor != "" {
				cursor = tc.cursor
			}
			raw, err := json.Marshal(map[string]any{"op": "subscribe", "subscriptionId": "price", "topics": []string{topic}, "afterCursor": cursor})
			if err != nil {
				t.Fatal(err)
			}
			if err = conn.WriteMessage(websocket.TextMessage, raw); err != nil {
				t.Fatal(err)
			}
			if got := readFrame(t, conn); got["op"] != "subscribed" {
				t.Fatalf("subscribe: %v", got)
			}
			if tc.action != "" {
				if got := readFrame(t, conn); got["op"] != "checkpoint" || got["code"] != "REPLAY_COMPLETE" {
					t.Fatalf("checkpoint: %v", got)
				}
				if err = conn.WriteMessage(websocket.TextMessage, []byte(tc.action)); err != nil {
					t.Fatal(err)
				}
			}
			if got := readFrame(t, conn); got["op"] != tc.want {
				t.Fatalf("control: %v, want %s", got, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name, path, origin string
		want               int
	}{
		{"forbidden origin", "/v2/snapshots?topics=" + topic, "https://other.example", 403},
		{"missing topics", "/v2/snapshots", origin, 422},
		{"oversized query", "/v2/snapshots?topics=" + strings.Repeat("x", 8193), origin, 422},
		{"invalid query encoding", "/v2/snapshots?topics=%zz", origin, 422},
		{"invalid topic shape", "/v2/snapshots?topics=" + strings.Repeat("x", 257), origin, 422},
		{"private authentication", "/v2/snapshots?topics=private.notifications.0x" + strings.Repeat("1", 40), origin, 401},
		{"unknown topic", "/v2/snapshots?topics=market.price.0x" + strings.Repeat("3", 40), origin, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, srv.URL+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Origin", tc.origin)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.want {
				t.Fatalf("status %d, want %d", res.StatusCode, tc.want)
			}
		})
	}
}

func TestAuthenticatedWebSocketClassifiesUnregisteredPools(t *testing.T) {
	stream, pool, topic := wsStreamFixture(t)
	registry, err := rest.NewDevelopmentStream(context.Background(), rest.DevelopmentStreamOptions{
		Pool: pool, Store: store.New(pool, nil), SigningKey: []byte(strings.Repeat("k", 32)), Authority: sharedauth.HTTPStreamAuthority{},
	})
	if err != nil {
		t.Fatal(err)
	}
	const origin = "https://web.example"
	srv, _ := newServer(t, ws.Options{PrivateStream: stream, TopicRegistry: registry, AllowedOrigins: []string{origin}})
	endpoint := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v2/stream"
	subscribe, err := json.Marshal(map[string]any{"op": "subscribe", "subscriptionId": "price", "topics": []string{topic}, "afterCursor": nil})
	if err != nil {
		t.Fatal(err)
	}
	connect := func(t *testing.T) *websocket.Conn {
		t.Helper()
		connection, response, err := (&websocket.Dialer{Subprotocols: []string{"sidiora.v2"}}).Dial(endpoint, http.Header{"Origin": []string{origin}})
		if err != nil {
			t.Fatalf("connect: %v %v", err, response)
		}
		t.Cleanup(func() { _ = connection.Close() })
		if err := connection.WriteMessage(websocket.TextMessage, subscribe); err != nil {
			t.Fatal(err)
		}
		return connection
	}
	connection := connect(t)
	if frame := readFrame(t, connection); frame["op"] != "error" || frame["code"] != "NOT_FOUND" {
		t.Fatalf("unknown pool control: %v", frame)
	}
	_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err = connection.ReadMessage()
	var closeError *websocket.CloseError
	if !errors.As(err, &closeError) || closeError.Code != websocket.ClosePolicyViolation {
		t.Fatalf("unknown pool close: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `DROP TABLE indexer.pools CASCADE`); err != nil {
		t.Fatal(err)
	}
	connection = connect(t)
	_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err = connection.ReadMessage()
	if !errors.As(err, &closeError) || closeError.Code != websocket.CloseInternalServerErr {
		t.Fatalf("unavailable registry close: %v", err)
	}
}

func TestAuthenticatedWebSocketDeliversCommittedLiveFrame(t *testing.T) {
	stream, pool, topic := wsStreamFixture(t)
	const origin = "https://web.example"
	srv, _ := newServer(t, ws.Options{PrivateStream: stream, AllowedOrigins: []string{origin}})
	request, err := http.NewRequest(http.MethodGet, srv.URL+"/v2/snapshots?topics="+topic, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", origin)
	response, err := srv.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var snapshot struct {
		Data struct {
			Snapshot struct {
				ResumeCursor string `json:"resumeCursor"`
			} `json:"snapshot"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil || response.StatusCode != 200 || snapshot.Data.Snapshot.ResumeCursor == "" {
		t.Fatalf("snapshot: status=%d cursor=%q err=%v", response.StatusCode, snapshot.Data.Snapshot.ResumeCursor, err)
	}
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v2/stream"
	conn, response, err := (&websocket.Dialer{Subprotocols: []string{"sidiora.v2"}}).Dial(url, http.Header{"Origin": []string{origin}})
	if err != nil {
		t.Fatalf("dial: %v, response %v", err, response)
	}
	defer conn.Close()
	subscribe, err := json.Marshal(map[string]any{"op": "subscribe", "subscriptionId": "price", "topics": []string{topic}, "afterCursor": snapshot.Data.Snapshot.ResumeCursor})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, subscribe); err != nil {
		t.Fatal(err)
	}
	if frame := readFrame(t, conn); frame["op"] != "subscribed" {
		t.Fatalf("subscription: %v", frame)
	}
	if frame := readFrame(t, conn); frame["op"] != "checkpoint" || frame["code"] != "REPLAY_COMPLETE" {
		t.Fatalf("replay completion: %v", frame)
	}
	ctx := context.Background()
	var prior []byte
	if err := pool.QueryRow(ctx, `SELECT frame FROM api_stream.stream_projection WHERE topic=$1`, topic).Scan(&prior); err != nil {
		t.Fatal(err)
	}
	var next broker.Frame
	if err := json.Unmarshal(prior, &next); err != nil {
		t.Fatal(err)
	}
	next.EventID = uuid.NewString()
	next.Sequence, next.Revision = "2", "2"
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := stream.AppendProjectionTx(ctx, tx, next); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		frame := readFrame(t, conn)
		if frame["sequence"] == "2" && frame["topic"] == topic {
			if frame["eventId"] != next.EventID {
				t.Fatalf("live event identity: %v", frame)
			}
			return
		}
		if frame["op"] == "error" || frame["op"] == "resnapshot_required" {
			t.Fatalf("live projection interrupted: %v", frame)
		}
	}
	t.Fatal("committed live frame was not delivered")
}

func TestAuthenticatedWebSocketDrainAndTicketRejection(t *testing.T) {
	stream, _, topic := wsStreamFixture(t)
	const origin = "https://web.example"
	hub := ws.NewHub(ws.Options{PrivateStream: stream, AllowedOrigins: []string{origin}})
	router := chi.NewRouter()
	hub.Register(router)
	server := httptest.NewServer(router)
	defer server.Close()
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/v2/stream"
	for _, tc := range []struct {
		name      string
		protocols []string
		code      string
	}{
		{"invalid ticket syntax", []string{"sidiora.v2", "ticket.invalid"}, ""},
		{"unknown ticket", []string{"sidiora.v2", "ticket." + strings.Repeat("a", 64)}, "AUTH_REQUIRED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, response, err := (&websocket.Dialer{Subprotocols: tc.protocols}).Dial(endpoint, http.Header{"Origin": []string{origin}})
			if tc.code == "" {
				if conn != nil {
					_ = conn.Close()
				}
				if err == nil || response == nil || response.StatusCode != http.StatusUnauthorized {
					t.Fatalf("ticket syntax response: %v, %v", response, err)
				}
				response.Body.Close()
				return
			}
			if err != nil {
				t.Fatalf("ticket handshake: %v", err)
			}
			defer conn.Close()
			raw, err := json.Marshal(map[string]any{"op": "subscribe", "subscriptionId": "price", "topics": []string{topic}, "afterCursor": nil})
			if err != nil {
				t.Fatal(err)
			}
			if err = conn.WriteMessage(websocket.TextMessage, raw); err != nil {
				t.Fatal(err)
			}
			if frame := readFrame(t, conn); frame["op"] != "error" || frame["code"] != tc.code {
				t.Fatalf("ticket control: %v", frame)
			}
		})
	}
	request, err := http.NewRequest(http.MethodGet, server.URL+"/v2/snapshots?topics="+topic, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", origin)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		Data struct {
			Snapshot struct {
				ResumeCursor string `json:"resumeCursor"`
			} `json:"snapshot"`
		} `json:"data"`
	}
	decodeErr := json.NewDecoder(response.Body).Decode(&snapshot)
	response.Body.Close()
	if decodeErr != nil || response.StatusCode != http.StatusOK || snapshot.Data.Snapshot.ResumeCursor == "" {
		t.Fatalf("snapshot: %d %+v %v", response.StatusCode, snapshot, decodeErr)
	}
	conn, _, err := (&websocket.Dialer{Subprotocols: []string{"sidiora.v2"}}).Dial(endpoint, http.Header{"Origin": []string{origin}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	raw, err := json.Marshal(map[string]any{"op": "subscribe", "subscriptionId": "price", "topics": []string{topic}, "afterCursor": snapshot.Data.Snapshot.ResumeCursor})
	if err != nil {
		t.Fatal(err)
	}
	if err = conn.WriteMessage(websocket.TextMessage, raw); err != nil {
		t.Fatal(err)
	}
	if frame := readFrame(t, conn); frame["op"] != "subscribed" {
		t.Fatalf("subscription: %v", frame)
	}
	if frame := readFrame(t, conn); frame["op"] != "checkpoint" || frame["code"] != "REPLAY_COMPLETE" {
		t.Fatalf("replay checkpoint: %v", frame)
	}
	hub.Drain()
	if frame := readFrame(t, conn); frame["op"] != "heartbeat" || frame["code"] != "SERVER_DRAINING" {
		t.Fatalf("drain notice: %v", frame)
	}
	if hub.Ready() {
		t.Fatal("draining hub remained ready")
	}
}

func TestAuthenticatedWebSocketBindsPrivateTicketToWalletTopic(t *testing.T) {
	stream, _, _ := wsStreamFixture(t)
	const origin = "https://web.example"
	const wallet = "0x1111111111111111111111111111111111111111"
	topic := "private.notifications." + wallet
	ticket := strings.Repeat("a", 64)
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer "+strings.Repeat("s", 32) {
			t.Errorf("identity request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/internal/v2/identity/ticket:consume":
			var request struct {
				Ticket  string `json:"ticket"`
				Binding struct {
					Origin string `json:"origin"`
				} `json:"binding"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Ticket != ticket || request.Binding.Origin != origin {
				t.Errorf("ticket binding: %+v %v", request, err)
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_ = json.NewEncoder(w).Encode(sharedauth.StreamPrincipal{AccountID: "00000000-0000-4000-8000-000000000001", SessionID: "00000000-0000-4000-8000-000000000002", Wallet: wallet, AuthEpoch: 7})
		case "/internal/v2/identity/session:check":
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("identity route: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer identity.Close()
	stream.Authority = sharedauth.HTTPStreamAuthority{Endpoint: identity.URL, ServiceToken: strings.Repeat("s", 32)}
	server, _ := newServer(t, ws.Options{PrivateStream: stream, AllowedOrigins: []string{origin}})
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/v2/stream"
	for _, check := range []struct {
		name, topic string
		accepted    bool
	}{
		{"own wallet", topic, true},
		{"other wallet", "private.notifications.0x2222222222222222222222222222222222222222", false},
	} {
		t.Run(check.name, func(t *testing.T) {
			conn, response, err := (&websocket.Dialer{Subprotocols: []string{"sidiora.v2", "ticket." + ticket}}).Dial(endpoint, http.Header{"Origin": []string{origin}})
			if err != nil {
				t.Fatalf("connect: %v %v", err, response)
			}
			defer conn.Close()
			raw, err := json.Marshal(map[string]any{"op": "subscribe", "subscriptionId": "private", "topics": []string{check.topic}, "afterCursor": nil})
			if err != nil {
				t.Fatal(err)
			}
			if err = conn.WriteMessage(websocket.TextMessage, raw); err != nil {
				t.Fatal(err)
			}
			first := readFrame(t, conn)
			if !check.accepted {
				if first["op"] != "error" || first["code"] != "AUTH_REQUIRED" {
					t.Fatalf("foreign wallet topic: %v", first)
				}
				return
			}
			if first["op"] != "subscribed" {
				t.Fatalf("private subscription: %v", first)
			}
			decisions, ok := first["topics"].([]any)
			if !ok || len(decisions) != 1 || decisions[0].(map[string]any)["accepted"] != check.accepted {
				t.Fatalf("wallet topic decision: %v", first)
			}
			if terminal := readFrame(t, conn); terminal["op"] != "resnapshot_required" || terminal["code"] != "SNAPSHOT_REQUIRED" {
				t.Fatalf("private snapshot handoff: %v", terminal)
			}
		})
	}
}
