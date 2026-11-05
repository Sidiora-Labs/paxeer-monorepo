package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/devidentity"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/proxy"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
)

const repair991Origin = "https://kindle.test"

func repair991IdentityRouter(t *testing.T) *chi.Mux {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "postgres://fixture@127.0.0.1:1/identity?sslmode=disable&pool_max_conns=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	now := time.Now()
	tokens, err := auth.NewIdentityTokens("gateway", "kindle", "ingress", []auth.SigningKey{{
		ID: "ingress", SecretReference: "unused_for_invalid_requests", ActiveAt: now.Add(-time.Hour), RetireAt: now.Add(time.Hour),
	}}, auth.EnvironmentIdentitySecrets{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := auth.NewIdentity(auth.IdentityConfig{
		Pool: pool, Domain: "kindle.test", URI: repair991Origin, AllowedOrigins: []string{repair991Origin},
		Verifier: auth.Chain125ProofVerifier{}, LimiterKey: []byte(strings.Repeat("l", 32)), Tokens: tokens,
	})
	if err != nil {
		t.Fatal(err)
	}
	router := sharedhttp.NewRouter(sharedhttp.ServerOptions{CORSOrigins: repair991Origin})
	if err := identity.RegisterV2Routes(router, auth.IdentityHTTPConfig{ServiceToken: strings.Repeat("s", 32)}); err != nil {
		t.Fatal(err)
	}
	return router
}

func TestRepairM_991_IdentityIngressDeadline(t *testing.T) {
	server := httptest.NewServer(publicGatewayHandler(repair991IdentityRouter(t), config.Config{}))
	defer server.Close()
	for _, tc := range []struct {
		name, path, origin, initial, code string
		length, status                    int
	}{
		{"challenge", "/v2/auth/challenges", repair991Origin, "{", "FIELD_INVALID", 1000, http.StatusUnprocessableEntity},
		{"session", "/v2/auth/sessions", repair991Origin, "{", "FIELD_INVALID", 1000, http.StatusUnprocessableEntity},
		{"refresh", "/v2/auth/sessions:refresh", repair991Origin, "{", "FIELD_INVALID", 1000, http.StatusUnprocessableEntity},
		{"early_origin_rejection", "/v2/auth/sessions", "https://other.test", "{", "SCOPE_DENIED", 1000, http.StatusForbidden},
		{"oversized_rejection", "/v2/auth/challenges", repair991Origin, "{" + strings.Repeat(" ", 16384), "FIELD_INVALID", 20000, http.StatusUnprocessableEntity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			started := time.Now()
			if err := conn.SetDeadline(started.Add(identityIngressReadTimeout + 2*time.Second)); err != nil {
				t.Fatal(err)
			}
			_, err = fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: kindle.test\r\nOrigin: %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", tc.path, tc.origin, tc.length, tc.initial)
			if err != nil {
				t.Fatal(err)
			}
			// Keep making progress: the limit must bound the entire body, not
			// restart every time the peer sends another byte.
			stop := make(chan struct{})
			written := make(chan int, 1)
			go func() {
				ticker := time.NewTicker(100 * time.Millisecond)
				defer ticker.Stop()
				count := 0
				defer func() { written <- count }()
				for {
					select {
					case <-stop:
						return
					case <-ticker.C:
						if _, err := io.WriteString(conn, " "); err != nil {
							return
						}
						count++
					}
				}
			}()
			response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodPost})
			elapsed := time.Since(started)
			close(stop)
			if count := <-written; count < 2 {
				t.Fatalf("only %d body bytes sent: handler refused before exercising a slow body", count)
			}
			if err != nil {
				t.Fatalf("slow body did not receive a bounded refusal after %s: %v", elapsed, err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tc.status || !strings.Contains(string(body), `"`+tc.code+`"`) {
				t.Fatalf("slow body: status=%d body=%s", response.StatusCode, body)
			}
			if elapsed < identityIngressReadTimeout-500*time.Millisecond || elapsed > identityIngressReadTimeout+time.Second {
				t.Fatalf("body deadline elapsed %s, expected %s", elapsed, identityIngressReadTimeout)
			}
		})
	}
}

func TestRepairM_991_IdentityDeadlineClearedOnKeepAlive(t *testing.T) {
	server := httptest.NewServer(publicGatewayHandler(repair991IdentityRouter(t), config.Config{}))
	defer server.Close()
	conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for attempt := 0; attempt < 2; attempt++ {
		if attempt == 1 {
			time.Sleep(identityIngressReadTimeout + 200*time.Millisecond)
		}
		if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Fprintf(conn, "POST /v2/auth/challenges HTTP/1.1\r\nHost: kindle.test\r\nOrigin: %s\r\nContent-Type: application/json\r\nContent-Length: 2\r\n\r\n{}", repair991Origin); err != nil {
			t.Fatal(err)
		}
		response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodPost})
		if err != nil {
			t.Fatalf("keep-alive request %d: %v", attempt, err)
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("request %d: status=%d read=%v", attempt, response.StatusCode, readErr)
		}
	}
}

func TestRepairM_991_LongLivedWebSocket(t *testing.T) {
	upgrader := websocket.Upgrader{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			kind, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(kind, data); err != nil {
				return
			}
		}
	}))
	defer upstream.Close()
	tunnel, err := proxy.NewWS(proxy.WSDeps{TargetBaseURL: "ws" + strings.TrimPrefix(upstream.URL, "http")})
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	router.Handle("/social/ws", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tunnel.ServeHTTP(w, r.WithContext(auth.WithActor(r.Context(), "0x1111111111111111111111111111111111111111")))
	}))
	server := httptest.NewServer(publicGatewayHandler(router, config.Config{}))
	defer server.Close()
	conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/social/ws", nil)
	if err != nil {
		t.Fatalf("upgrade: %v (%v)", err, response)
	}
	defer conn.Close()
	for attempt := 0; attempt < 2; attempt++ {
		if attempt == 1 {
			time.Sleep(identityIngressReadTimeout + 200*time.Millisecond)
		}
		if err := conn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte("still connected")); err != nil {
			t.Fatal(err)
		}
		if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		_, data, err := conn.ReadMessage()
		if err != nil || string(data) != "still connected" {
			t.Fatalf("WebSocket after wait %d: data=%q err=%v", attempt, data, err)
		}
		// A client-side read deadline must not cause the idle wait to fail.
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			t.Fatal(err)
		}
	}
}

// repair991Peer serves a single committed protocol conversation over TCP.
// It implements no database or cache operations; pgx and go-redis perform
// their actual handshakes and pings against the wire transcript peers below.
func repair991Peer(t *testing.T, conversation func(net.Conn) error) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var conn net.Conn
	stopped := false
	done := make(chan error, 1)
	go func() {
		accepted, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		mu.Lock()
		conn = accepted
		closed := stopped
		mu.Unlock()
		defer accepted.Close()
		if closed {
			done <- net.ErrClosed
			return
		}
		_ = accepted.SetDeadline(time.Now().Add(15 * time.Second))
		done <- conversation(accepted)
	}()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			mu.Lock()
			stopped = true
			listener.Close()
			if conn != nil {
				conn.Close()
			}
			mu.Unlock()
			select {
			case err := <-done:
				if err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
					t.Errorf("protocol transcript: %v", err)
				}
			case <-time.After(time.Second):
				t.Error("protocol peer did not stop")
			}
		})
	}
	t.Cleanup(stop)
	return listener.Addr().String(), stop
}

func repair991Postgres(t *testing.T) (string, func(), <-chan struct{}) {
	t.Helper()
	pinged := make(chan struct{}, 8)
	address, stop := repair991Peer(t, func(conn net.Conn) error {
		backend := pgproto3.NewBackend(conn, conn)
		startup, err := backend.ReceiveStartupMessage()
		if err != nil {
			return err
		}
		message, ok := startup.(*pgproto3.StartupMessage)
		if !ok || message.Parameters["database"] != "identity" || message.Parameters["search_path"] != devidentity.Schema {
			return fmt.Errorf("unexpected PostgreSQL startup: %v", startup)
		}
		backend.Send(&pgproto3.AuthenticationOk{})
		backend.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.4"})
		backend.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
		backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
		if err := backend.Flush(); err != nil {
			return err
		}
		for {
			message, err := backend.Receive()
			if err != nil {
				return err
			}
			if _, ok := message.(*pgproto3.Terminate); ok {
				return nil
			}
			query, ok := message.(*pgproto3.Query)
			if !ok || query.String != "-- ping" {
				return fmt.Errorf("unrecorded PostgreSQL request: %v", message)
			}
			backend.Send(&pgproto3.EmptyQueryResponse{})
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
			if err := backend.Flush(); err != nil {
				return err
			}
			pinged <- struct{}{}
		}
	})
	return "postgres://fixture@" + address + "/identity?sslmode=disable&pool_max_conns=1", stop, pinged
}

func repair991Redis(t *testing.T) *goredis.Client {
	t.Helper()
	address, _ := repair991Peer(t, func(conn net.Conn) error {
		reader := bufio.NewReader(conn)
		readCommand := func() ([]string, error) {
			line, err := reader.ReadString('\n')
			if err != nil {
				return nil, err
			}
			if !strings.HasPrefix(line, "*") {
				return nil, fmt.Errorf("expected RESP array, got %q", line)
			}
			count, err := strconv.Atoi(strings.TrimSpace(line[1:]))
			if err != nil || count < 1 || count > 8 {
				return nil, fmt.Errorf("unexpected RESP count: %q", line)
			}
			args := make([]string, count)
			for i := range args {
				line, err = reader.ReadString('\n')
				if err != nil || !strings.HasPrefix(line, "$") {
					return nil, fmt.Errorf("expected RESP bulk length: %q (%v)", line, err)
				}
				n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
				if err != nil || n < 0 || n > 1024 {
					return nil, fmt.Errorf("unexpected RESP bulk length: %q", line)
				}
				value := make([]byte, n+2)
				if _, err := io.ReadFull(reader, value); err != nil {
					return nil, err
				}
				args[i] = string(value[:n])
			}
			return args, nil
		}
		args, err := readCommand()
		if err != nil || strings.Join(args, " ") != "hello 2" {
			return fmt.Errorf("unrecorded Redis handshake: %v (%v)", args, err)
		}
		if _, err := io.WriteString(conn, "*2\r\n$6\r\nserver\r\n$5\r\nredis\r\n"); err != nil {
			return err
		}
		for {
			args, err := readCommand()
			if err != nil {
				return err
			}
			if strings.Join(args, " ") != "ping" {
				return fmt.Errorf("unrecorded Redis request: %v", args)
			}
			if _, err := io.WriteString(conn, "+PONG\r\n"); err != nil {
				return err
			}
		}
	})
	client := goredis.NewClient(&goredis.Options{Addr: address, Protocol: 2, DisableIdentity: true, MaxRetries: -1})
	t.Cleanup(func() { client.Close() })
	return client
}

func TestRepairM_991_HealthFixture(t *testing.T) {
	dsn, stopDatabase, pinged := repair991Postgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := devidentity.NewPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	rdb := repair991Redis(t)
	cfg := config.Config{NodeEnv: "development"}
	check := identityDatabaseCheck(cfg, pool)
	router := sharedhttp.NewRouter(sharedhttp.ServerOptions{Health: sharedhttp.HealthDeps{
		DB: check, Redis: func(ctx context.Context) error { return rdb.Ping(ctx).Err() },
	}})
	server := httptest.NewServer(publicGatewayHandler(router, cfg))
	defer server.Close()
	assertReady := func(want int, database string) {
		t.Helper()
		response, err := server.Client().Get(server.URL + "/health/ready")
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var body struct {
			Status string            `json:"status"`
			Checks map[string]string `json:"checks"`
		}
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want || body.Checks["database"] != database || body.Checks["redis"] != "ok" {
			t.Fatalf("readiness = %d %+v, want status=%d database=%s redis=ok", response.StatusCode, body, want, database)
		}
	}
	assertReady(http.StatusOK, "ok")
	for i := 0; i < 2; i++ {
		select {
		case <-pinged:
		case <-time.After(time.Second):
			t.Fatal("real pgx did not send the bootstrap/readiness ping")
		}
	}
	stopDatabase()
	assertReady(http.StatusServiceUnavailable, "failed")
	pool.Close()
	assertReady(http.StatusServiceUnavailable, "failed")
	if err := identityDatabaseCheck(cfg, nil)(ctx); err == nil || err.Error() != "identity_database_missing" {
		t.Fatalf("missing identity database was not refused: %v", err)
	}
	missing := httptest.NewServer(publicGatewayHandler(sharedhttp.NewRouter(sharedhttp.ServerOptions{Health: sharedhttp.HealthDeps{
		DB: identityDatabaseCheck(cfg, nil), Redis: func(ctx context.Context) error { return rdb.Ping(ctx).Err() },
	}}), cfg))
	defer missing.Close()
	response, err := missing.Client().Get(missing.URL + "/health/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body struct {
		Checks map[string]string `json:"checks"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusServiceUnavailable || body.Checks["database"] != "failed" || body.Checks["redis"] != "ok" {
		t.Fatalf("missing database readiness = %d %+v", response.StatusCode, body)
	}
}
