package auth

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// Protocol peers are committed fixtures. Auth, pgx, Redis transport, signatures,
// pagination, collection and measurement consumers remain the production code.
func repairM990Redis(t *testing.T) *goredis.Client {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	values := map[string]string{}
	deadlines := map[string]time.Time{}
	var connections sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			go func() {
				defer connections.Done()
				defer conn.Close()
				reader := bufio.NewReader(conn)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if !strings.HasPrefix(line, "*") {
						return
					}
					count, err := strconv.Atoi(strings.TrimSpace(line[1:]))
					if err != nil || count < 1 || count > 16 {
						return
					}
					args := make([]string, count)
					for i := range args {
						line, err = reader.ReadString('\n')
						if err != nil || !strings.HasPrefix(line, "$") {
							return
						}
						n, e := strconv.Atoi(strings.TrimSpace(line[1:]))
						if e != nil || n < 0 || n > 65536 {
							return
						}
						raw := make([]byte, n+2)
						if _, err = io.ReadFull(reader, raw); err != nil {
							return
						}
						args[i] = string(raw[:n])
					}
					mu.Lock()
					now := time.Now()
					for key, deadline := range deadlines {
						if !now.Before(deadline) {
							delete(values, key)
							delete(deadlines, key)
						}
					}
					reply := ""
					switch strings.ToLower(args[0]) {
					case "hello":
						reply = "-ERR unknown command 'hello'\r\n"
					case "client":
						reply = "+OK\r\n"
					case "set":
						if len(args) != 5 {
							reply = "-ERR bounded SET expiry required\r\n"
							break
						}
						ttl, e := strconv.Atoi(args[4])
						if e != nil {
							reply = "-ERR invalid expiry\r\n"
							break
						}
						unit := time.Millisecond
						if strings.EqualFold(args[3], "ex") {
							unit = time.Second
						}
						values[args[1]] = args[2]
						deadlines[args[1]] = now.Add(time.Duration(ttl) * unit)
						reply = "+OK\r\n"
					case "get":
						value, ok := values[args[1]]
						if !ok {
							reply = "$-1\r\n"
						} else {
							reply = fmt.Sprintf("$%d\r\n%s\r\n", len(value), value)
						}
					case "eval":
						// Redis GET/DEL compare-and-consume transcript, with the exact script checked.
						if len(args) != 5 || args[1] != `if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) else return 0 end` || args[2] != "1" {
							reply = "-ERR unexpected script\r\n"
							break
						}
						if value, ok := values[args[3]]; ok && value == args[4] {
							delete(values, args[3])
							reply = ":1\r\n"
						} else {
							reply = ":0\r\n"
						}
					default:
						reply = "-ERR unexpected fixture command\r\n"
					}
					mu.Unlock()
					if _, err = io.WriteString(conn, reply); err != nil {
						return
					}
				}
			}()
		}
	}()
	client := goredis.NewClient(&goredis.Options{Addr: listener.Addr().String(), Protocol: 2, DisableIdentity: true, MaxRetries: -1, ReadTimeout: time.Second, WriteTimeout: time.Second})
	t.Cleanup(func() { client.Close(); listener.Close(); <-done; connections.Wait() })
	return client
}

func TestRepairM_990_LegacyMessageBinding(t *testing.T) {
	a := New(Deps{Redis: repairM990Redis(t), JWTSecret: "repair-m990-signing-secret", JWTTTL: time.Hour, NonceTTL: time.Minute, AppDomain: "kindle.test"})
	router := chi.NewRouter()
	a.RegisterRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()
	call := func(path string, data any) (int, []byte) {
		t.Helper()
		raw, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		response, err := server.Client().Post(server.URL+path, "application/json", strings.NewReader(string(raw)))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, body
	}
	for _, attack := range []string{"wrong_domain", "changed_purpose", "invalid_signature", "changed_wallet", "changed_issue_time"} {
		t.Run(attack, func(t *testing.T) {
			owner, other := newWallet(t), newWallet(t)
			status, body := call("/auth/nonce", nonceRequest{owner.addr})
			if status != 200 {
				t.Fatalf("nonce: %d %s", status, body)
			}
			var issued nonceResponse
			if json.Unmarshal(body, &issued) != nil {
				t.Fatal("nonce wire invalid")
			}
			message := issued.Message
			signer := owner
			switch attack {
			case "wrong_domain":
				message = strings.Replace(message, "kindle.test", "other.test", 1)
			case "changed_purpose":
				message = strings.Replace(message, "Sign in to Sidiora.", "Authorize a withdrawal.", 1)
			case "invalid_signature":
				signer = other
			case "changed_wallet":
				message = strings.Replace(message, owner.addr, other.addr, 1)
			case "changed_issue_time":
				message += "\nIssued At: 2099-01-01T00:00:00Z"
			}
			if status, _ = call("/auth/login", loginRequest{owner.addr, message, signer.sign(t, message)}); status != 401 {
				t.Fatalf("%s accepted: %d", attack, status)
			}
			request := loginRequest{owner.addr, issued.Message, owner.sign(t, issued.Message)}
			status, body = call("/auth/login", request)
			if status != 200 {
				t.Fatalf("invalid attempt burned exact proof: %d %s", status, body)
			}
			var result loginResponse
			if json.Unmarshal(body, &result) != nil {
				t.Fatal("login wire invalid")
			}
			req, err := http.NewRequest("GET", server.URL+"/auth/me", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+result.Token)
			response, err := server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != 200 {
				t.Fatal("minted session not consumed by real middleware")
			}
			if status, _ = call("/auth/login", request); status != 401 {
				t.Fatal("proof replay accepted")
			}
		})
	}
	owner := newWallet(t)
	status, raw := call("/auth/nonce", nonceRequest{owner.addr})
	if status != 200 {
		t.Fatal(status)
	}
	var issued nonceResponse
	if json.Unmarshal(raw, &issued) != nil {
		t.Fatal("nonce")
	}
	request := loginRequest{owner.addr, issued.Message, owner.sign(t, issued.Message)}
	statuses := make(chan int, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			raw, _ := json.Marshal(request)
			response, err := server.Client().Post(server.URL+"/auth/login", "application/json", strings.NewReader(string(raw)))
			if err != nil {
				statuses <- 0
				return
			}
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			statuses <- response.StatusCode
		}()
	}
	wg.Wait()
	close(statuses)
	successes := 0
	for status := range statuses {
		if status == 200 {
			successes++
		} else if status != 401 {
			t.Fatalf("concurrent proof status: %d", status)
		}
	}
	if successes != 1 {
		t.Fatalf("atomic consumption winners: %d", successes)
	}
}

type repairM990Field struct {
	name string
	oid  uint32
}
type repairM990Reply struct {
	fields []repairM990Field
	rows   [][][]byte
	tag    string
	err    error
}

func repairM990TextRows(names []string, rows ...[]string) repairM990Reply {
	reply := repairM990Reply{tag: "SELECT " + strconv.Itoa(len(rows))}
	for _, name := range names {
		oid := uint32(25)
		if name == "count" || name == "requests" || name == "auth_epoch" {
			oid = 20
		}
		reply.fields = append(reply.fields, repairM990Field{name, oid})
	}
	for _, row := range rows {
		values := [][]byte{}
		for _, value := range row {
			values = append(values, []byte(value))
		}
		reply.rows = append(reply.rows, values)
	}
	return reply
}

// A loopback PostgreSQL wire transcript peer avoids a database executable in this
// bounded gate. It does not claim to qualify PostgreSQL SQL execution or restart.
func repairM990Postgres(t *testing.T, respond func(string) repairM990Reply) *pgxpool.Pool {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var connMu sync.Mutex
	conns := []net.Conn{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connMu.Lock()
			conns = append(conns, conn)
			connMu.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer conn.Close()
				backend := pgproto3.NewBackend(conn, conn)
				if _, err := backend.ReceiveStartupMessage(); err != nil {
					return
				}
				backend.Send(&pgproto3.AuthenticationOk{})
				for _, parameter := range [][2]string{{"server_version", "16.0"}, {"client_encoding", "UTF8"}, {"standard_conforming_strings", "on"}, {"TimeZone", "UTC"}, {"integer_datetimes", "on"}} {
					backend.Send(&pgproto3.ParameterStatus{Name: parameter[0], Value: parameter[1]})
				}
				backend.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
				backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
				if backend.Flush() != nil {
					return
				}
				txStatus := byte('I')
				for {
					message, err := backend.Receive()
					if err != nil {
						return
					}
					switch query := message.(type) {
					case *pgproto3.Terminate:
						return
					case *pgproto3.Query:
						reply := respond(query.String)
						if reply.err != nil {
							backend.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: reply.err.Error()})
						} else {
							if len(reply.fields) > 0 {
								fields := []pgproto3.FieldDescription{}
								for _, field := range reply.fields {
									fields = append(fields, pgproto3.FieldDescription{Name: []byte(field.name), DataTypeOID: field.oid, DataTypeSize: -1})
								}
								backend.Send(&pgproto3.RowDescription{Fields: fields})
								for _, row := range reply.rows {
									backend.Send(&pgproto3.DataRow{Values: row})
								}
							}
							if reply.tag == "" {
								reply.tag = "DELETE 0"
							}
							backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(reply.tag)})
						}
						if strings.HasPrefix(strings.ToLower(query.String), "begin") {
							txStatus = 'T'
						}
						if strings.EqualFold(query.String, "commit") || strings.EqualFold(query.String, "rollback") {
							txStatus = 'I'
						}
						backend.Send(&pgproto3.ReadyForQuery{TxStatus: txStatus})
						if backend.Flush() != nil {
							return
						}
					default:
						return
					}
				}
			}()
		}
	}()
	cfg, err := pgxpool.ParseConfig("postgres://fixture@" + listener.Addr().String() + "/fixture?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.ConnConfig.ConnectTimeout = time.Second
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		listener.Close()
		<-done
		connMu.Lock()
		for _, conn := range conns {
			conn.Close()
		}
		connMu.Unlock()
		wg.Wait()
	})
	return pool
}

func repairM990Identity(t *testing.T, pool *pgxpool.Pool) *Identity {
	t.Helper()
	identity, err := NewIdentity(IdentityConfig{Pool: pool, Domain: "kindle.test", URI: "https://kindle.test/auth", AllowedOrigins: []string{"https://kindle.test"}, Verifier: Chain125ProofVerifier{}, LimiterKey: []byte(strings.Repeat("k", 32))})
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

func TestRepairM_990_WalletLinkHistory(t *testing.T) {
	owner := Principal{accountID: uuid.NewString(), sessionID: uuid.NewString(), wallet: newWallet(t).addr}
	at := time.Now().UTC().Truncate(time.Millisecond)
	ids := []string{}
	for i := 1; i <= 151; i++ {
		ids = append(ids, fmt.Sprintf("00000000-0000-4000-8000-%012d", i))
	}
	page := 0
	currentRead := false
	pool := repairM990Postgres(t, func(sql string) repairM990Reply {
		if strings.HasPrefix(sql, "SELECT a.account_id::text,s.session_id::text,s.wallet,s.auth_epoch,s.reauthenticated_at") {
			return repairM990Reply{fields: []repairM990Field{{"account_id", 25}, {"session_id", 25}, {"wallet", 25}, {"auth_epoch", 20}, {"reauthenticated_at", 1184}}, rows: [][][]byte{{[]byte(owner.accountID), []byte(owner.sessionID), []byte(owner.wallet), []byte("0"), []byte(at.Format("2006-01-02 15:04:05.999999999-07:00"))}}, tag: "SELECT 1"}
		}
		if strings.HasPrefix(sql, "SELECT EXISTS(SELECT 1 FROM identity_sessions") {
			return repairM990Reply{fields: []repairM990Field{{"exists", 16}}, rows: [][][]byte{{[]byte("t")}}, tag: "SELECT 1"}
		}
		if !strings.HasPrefix(sql, "SELECT w.link_id::text") {
			return repairM990Reply{err: fmt.Errorf("unexpected history transcript: %s", sql)}
		}
		for _, fragment := range []string{"w.account_id=", owner.accountID, "w.state<>'revoked'", "w.state<>'pending'", "c.consumed_at IS NULL", "c.expires_at>", "ORDER BY w.link_id LIMIT"} {
			if !strings.Contains(sql, fragment) {
				t.Errorf("history query lost %s", fragment)
			}
		}
		start, end := 0, 101
		if currentRead {
			start, end = 150, 151
			if !regexp.MustCompile(`\(\s*'f'\s*::boolean`).MatchString(sql) {
				t.Error("current read included revoked history")
			}
		} else if page == 1 {
			start, end = 100, 151
			if !strings.Contains(sql, ids[99]) {
				t.Error("cursor did not cross transport")
			}
		} else if page != 0 {
			t.Error("unexpected extra page")
		}
		if !currentRead {
			page++
		}
		reply := repairM990Reply{fields: []repairM990Field{{"link_id", 25}, {"account_id", 25}, {"wallet", 25}, {"visibility", 25}, {"state", 25}, {"linked_at", 1184}}, tag: fmt.Sprintf("SELECT %d", end-start)}
		for i := start; i < end; i++ {
			state := "revoked"
			if i == 150 {
				state = "verified"
			}
			reply.rows = append(reply.rows, [][]byte{[]byte(ids[i]), []byte(owner.accountID), []byte(owner.wallet), []byte("private"), []byte(state), []byte(at.Format("2006-01-02 15:04:05.999999999-07:00"))})
		}
		return reply
	})
	identity := repairM990Identity(t, pool)
	router := chi.NewRouter()
	identity.registerDevelopmentOwnerRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()
	req := httptest.NewRequest("GET", "/v2/me/wallet-links?limit=100", nil)
	limit, cursor, err := walletPageQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	all := []WalletLink{}
	for {
		query := "?limit=" + strconv.Itoa(limit)
		if cursor != "" {
			query += "&cursor=" + cursor
		}
		request, err := http.NewRequest("GET", server.URL+"/v2/me/wallet-links"+query, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 64))
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		wire, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 || response.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("wallet history HTTP: %d %s %v", response.StatusCode, wire, err)
		}
		var decoded Envelope[struct {
			Items []WalletLink `json:"items"`
		}]
		if json.Unmarshal(wire, &decoded) != nil {
			t.Fatal("wallet page consumer decode failed")
		}
		for _, item := range decoded.Data.Items {
			if item.LinkedAt == nil || !item.LinkedAt.Equal(at) || item.AccountID != owner.accountID || item.ChainID != "125" {
				t.Fatal("verified/revoked history changed")
			}
		}
		all = append(all, decoded.Data.Items...)
		if decoded.NextCursor == nil {
			break
		}
		cursor = *decoded.NextCursor
		if _, _, err = identity.WalletLinksPage(context.Background(), owner, 99, cursor); err == nil {
			t.Fatal("cursor page bound changed")
		}
		other := owner
		other.sessionID = uuid.NewString()
		if _, _, err = identity.WalletLinksPage(context.Background(), other, 100, cursor); err == nil {
			t.Fatal("cursor session binding lost")
		}
		if _, _, err = identity.WalletLinksPage(context.Background(), owner, 100, cursor+"00"); err == nil {
			t.Fatal("cursor tamper accepted")
		}
	}
	if len(all) != 151 || page != 2 || all[150].State != "verified" {
		t.Fatalf("history inaccessible: %d records, %d pages", len(all), page)
	}
	for i, item := range all {
		if item.LinkID != ids[i] {
			t.Fatal("page overlap or omission")
		}
	}
	currentRead = true
	current, next, err := identity.CurrentWalletLinksPage(context.Background(), owner, 100, "")
	if err != nil || next != nil || len(current) != 1 || current[0].State != "verified" {
		t.Fatalf("current links unavailable after history growth: %+v %v", current, err)
	}
	if _, _, err = identity.CurrentWalletLinksPage(context.Background(), owner, 100, cursor); err == nil {
		t.Fatal("history cursor reused for current view")
	}
	for _, query := range []string{"limit=101", "limit=01", "limit=1&limit=2", "cursor=a&cursor=b", "unexpected=1"} {
		if _, _, err := walletPageQuery(httptest.NewRequest("GET", "/?"+query, nil)); err == nil {
			t.Fatalf("invalid pagination accepted: %s", query)
		}
	}
}

func TestRepairM_990_IdentityCollection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	statements := []string{}
	committed := make(chan struct{})
	var committedOnce sync.Once
	pool := repairM990Postgres(t, func(sql string) repairM990Reply {
		mu.Lock()
		statements = append(statements, sql)
		mu.Unlock()
		if sql == "commit" {
			committedOnce.Do(func() { close(committed) })
			return repairM990Reply{tag: "COMMIT"}
		}
		if sql == "begin" {
			return repairM990Reply{tag: "BEGIN"}
		}
		if strings.HasPrefix(sql, "DELETE FROM ") {
			if !regexp.MustCompile(`LIMIT\s+'7'\s+FOR`).MatchString(sql) || !strings.Contains(sql, "SKIP LOCKED") {
				t.Errorf("unbounded collector statement: %s", sql)
			}
			if strings.Contains(sql, "identity_wallet_links") && (!strings.Contains(sql, "w.state='pending'") || !strings.Contains(sql, "NOT EXISTS(SELECT 1 FROM identity_challenges")) {
				t.Error("pending collection lost challenge/history protection")
			}
			if strings.Contains(sql, "identity_outbox") || strings.Contains(sql, "identity_revisions") || strings.Contains(sql, "delivery_outbox") {
				t.Error("collector touched durable evidence")
			}
			return repairM990Reply{tag: "DELETE 1"}
		}
		return repairM990Reply{err: fmt.Errorf("unexpected collection transcript: %s", sql)}
	})
	identity := repairM990Identity(t, pool)
	for _, limit := range []int{0, 1001} {
		if identity.CollectExpired(context.Background(), limit) == nil {
			t.Fatal("collector limit accepted")
		}
	}
	result := make(chan error, 1)
	go func() { result <- identity.RunCollector(ctx, time.Second, 500*time.Millisecond, 7) }()
	select {
	case <-committed:
	case <-time.After(2 * time.Second):
		t.Fatal("collector never committed")
	}
	deadline := time.Now().Add(time.Second)
	for pool.Stat().AcquiredConns() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if pool.Stat().AcquiredConns() != 0 {
		t.Fatal("collector never released its committed transaction")
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("collector cancellation: %v", err)
	}
	mu.Lock()
	captured := append([]string(nil), statements...)
	mu.Unlock()
	if len(captured) != len(identityCollectionStatements)+2 {
		t.Fatalf("unexpected collector transaction: %v", captured)
	}
	for _, table := range []string{"identity_challenge_requests", "identity_owner_requests"} {
		found := false
		for _, sql := range captured {
			if strings.HasPrefix(sql, "DELETE FROM "+table+" ") && strings.Contains(sql, "retain_until<=clock_timestamp()") {
				found = true
			}
		}
		if !found {
			t.Fatalf("expired replay retention omitted: %s", table)
		}
	}
	if identity.RunCollector(context.Background(), time.Millisecond, time.Millisecond, 7) == nil {
		t.Fatal("unbounded collector cadence accepted")
	}
	failedStatements := []string{}
	failed := repairM990Postgres(t, func(sql string) repairM990Reply {
		failedStatements = append(failedStatements, sql)
		if strings.HasPrefix(sql, "DELETE FROM identity_challenges ") {
			return repairM990Reply{err: errors.New("recorded storage interruption")}
		}
		return repairM990Reply{tag: "ROLLBACK"}
	})
	if err := repairM990Identity(t, failed).CollectExpired(context.Background(), 1); err == nil {
		t.Fatal("collection failure silently accepted")
	}
	if len(failedStatements) != 4 || failedStatements[len(failedStatements)-1] != "rollback" {
		t.Fatalf("collection was not atomic: %v", failedStatements)
	}
}

// pgx's pinned simple-protocol encoder transports []byte JSON as a quoted
// bytea literal. Decode those exact producer bytes before retaining the response.
func repairM990JSONLiteral(sql string) ([]byte, error) {
	match := regexp.MustCompile(`'\\x([0-9a-f]+)'\s*::jsonb`).FindStringSubmatch(sql)
	if len(match) != 2 {
		return nil, errors.New("producer JSON bytea literal absent")
	}
	raw, err := hex.DecodeString(match[1])
	if err != nil || !json.Valid(raw) {
		return nil, errors.New("producer JSON bytes invalid")
	}
	return raw, nil
}

func TestRepairM_990_RequestReplayRetention(t *testing.T) {
	request := ChallengeRequest{Wallet: newWallet(t).addr, Origin: "https://kindle.test"}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(body)
	var stored []byte
	expired := false
	retired := 0
	respond := func(sql string) repairM990Reply {
		switch {
		case sql == "begin":
			return repairM990Reply{tag: "BEGIN"}
		case sql == "commit":
			return repairM990Reply{tag: "COMMIT"}
		case sql == "rollback":
			return repairM990Reply{tag: "ROLLBACK"}
		case strings.HasPrefix(sql, "SELECT pg_advisory_xact_lock"):
			return repairM990Reply{tag: "SELECT 1"}
		case strings.HasPrefix(sql, "DELETE FROM identity_challenge_requests "):
			if !strings.Contains(sql, "retain_until<=clock_timestamp()") {
				t.Error("request replay expiry lost")
			}
			if expired && stored != nil {
				stored = nil
				retired++
				expired = false
			}
			return repairM990Reply{tag: "DELETE 0"}
		case strings.HasPrefix(sql, "SELECT body_hash,response FROM identity_challenge_requests "):
			result := repairM990Reply{fields: []repairM990Field{{"body_hash", 17}, {"response", 3802}}, tag: "SELECT 0"}
			if stored != nil {
				result.rows = [][][]byte{{[]byte("\\x" + hex.EncodeToString(hash[:])), stored}}
				result.tag = "SELECT 1"
			}
			return result
		case strings.HasPrefix(sql, "INSERT INTO identity_limits"):
			return repairM990TextRows([]string{"requests"}, []string{"1"})
		case strings.HasPrefix(sql, "SELECT count(*) FROM identity_challenges"):
			return repairM990TextRows([]string{"count"}, []string{"0"})
		case strings.HasPrefix(sql, "INSERT INTO identity_challenges"):
			return repairM990Reply{tag: "INSERT 0 1"}
		case strings.HasPrefix(sql, "INSERT INTO identity_challenge_requests"):
			var err error
			stored, err = repairM990JSONLiteral(sql)
			if err != nil {
				t.Error(err)
			}
			if !strings.Contains(sql, hex.EncodeToString(hash[:])) || !strings.Contains(sql, "interval '24 hours'") {
				t.Error("producer replay digest/retention changed")
			}
			return repairM990Reply{tag: "INSERT 0 1"}
		default:
			return repairM990Reply{err: fmt.Errorf("unexpected replay transcript: %s", sql)}
		}
	}
	pool := repairM990Postgres(t, respond)
	identity := repairM990Identity(t, pool)
	key := uuid.NewString()
	first, err := identity.IssueLoginIdempotent(context.Background(), request, "127.0.0.1", key, body)
	if err != nil {
		t.Fatal(err)
	}
	retried, err := identity.IssueLoginIdempotent(context.Background(), request, "127.0.0.1", key, body)
	if err != nil || retried.ChallengeID != first.ChallengeID || retried.Message != first.Message {
		t.Fatalf("retained replay changed: %v", err)
	}
	if _, err = identity.IssueLoginIdempotent(context.Background(), request, "127.0.0.1", key, append(body, ' ')); err == nil {
		t.Fatal("changed request replay accepted")
	}
	// Reopen the real client/component against the retained transcript state.
	// A real PostgreSQL stop/start remains an explicitly recorded integration request.
	pool.Close()
	reopened := repairM990Identity(t, repairM990Postgres(t, respond))
	recovered, err := reopened.IssueLoginIdempotent(context.Background(), request, "127.0.0.1", key, body)
	if err != nil || recovered.ChallengeID != first.ChallengeID {
		t.Fatalf("component reopen lost retained replay: %v", err)
	}
	expired = true
	fresh, err := reopened.IssueLoginIdempotent(context.Background(), request, "127.0.0.1", key, body)
	if err != nil || fresh.ChallengeID == first.ChallengeID || retired != 1 {
		t.Fatalf("expired replay was reused: %v", err)
	}
}

type repairM990WorkloadPeer struct {
	mu               sync.Mutex
	challenges       map[string]Challenge
	sessions         map[string]LoginData
	acknowledgements []IdentityAcknowledgement
	account          string
	delay            time.Duration
}

func repairM990Workload(t *testing.T, delay time.Duration) (*repairM990WorkloadPeer, *httptest.Server) {
	t.Helper()
	peer := &repairM990WorkloadPeer{challenges: map[string]Challenge{}, sessions: map[string]LoginData{}, account: uuid.NewString(), delay: delay}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", r.Header.Get("X-Request-Id"))
		w.Header().Set("Content-Type", "application/json")
		peer.mu.Lock()
		defer peer.mu.Unlock()
		respond := func(data any) {
			if err := json.NewEncoder(w).Encode(Envelope[any]{RequestID: r.Header.Get("X-Request-Id"), Data: data, Quality: "complete", GeneratedAt: time.Now().UTC()}); err != nil {
				t.Error(err)
			}
		}
		credentials := func(data LoginData) {
			first, err := randomSecret()
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			second, err := randomSecret()
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			csrf, err := randomSecret()
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			refresh := first + "." + second
			peer.sessions[refresh] = data
			http.SetCookie(w, SessionCookie("__Host-kindle_refresh", refresh, data.Session.RefreshExpiresAt))
			w.Header().Set("X-CSRF-Token", csrf)
			respond(data)
		}
		switch r.URL.Path {
		case "/v2/auth/challenges":
			var request ChallengeRequest
			if DecodeIdentityRequest(r.Body, &request) != nil || !validUUID(r.Header.Get("Idempotency-Key")) {
				w.WriteHeader(422)
				return
			}
			nonce, err := randomSecret()
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			now := time.Now().UTC().Truncate(time.Millisecond)
			challenge := Challenge{ChallengeID: uuid.NewString(), Wallet: request.Wallet, Nonce: nonce, Domain: "kindle.test", URI: "https://kindle.test/auth", ChainID: "125", Purpose: "login", IssuedAt: now, ExpiresAt: now.Add(300 * time.Second)}
			challenge.Message, err = BuildSIWE(challenge, "")
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			peer.challenges[challenge.ChallengeID] = challenge
			w.WriteHeader(201)
			respond(struct {
				Challenge Challenge `json:"challenge"`
			}{challenge})
		case "/v2/auth/sessions":
			var request LoginRequest
			if DecodeIdentityRequest(r.Body, &request) != nil {
				w.WriteHeader(422)
				return
			}
			challenge, ok := peer.challenges[request.ChallengeID]
			if !ok || !sharedauth.VerifyWalletSignature(request.Proof.Wallet, challenge.Message, request.Proof.Signature) {
				w.WriteHeader(401)
				return
			}
			if peer.delay > 0 {
				time.Sleep(peer.delay)
			}
			data := LoginData{Session: Session{SessionID: uuid.NewString(), Wallet: request.Proof.Wallet, AccessExpiresAt: time.Now().UTC().Add(time.Hour), RefreshExpiresAt: time.Now().UTC().Truncate(time.Millisecond).Add(24 * time.Hour), Scopes: []string{"account:owner"}}, AccessToken: uuid.NewString()}
			peer.acknowledgements = append(peer.acknowledgements, IdentityAcknowledgement{data.Session.SessionID, challenge.ChallengeID})
			credentials(data)
		case "/v2/auth/sessions:refresh":
			cookie, err := r.Cookie("__Host-kindle_refresh")
			if err != nil || len(r.Header.Get("X-CSRF-Token")) != 64 {
				w.WriteHeader(401)
				return
			}
			data, ok := peer.sessions[cookie.Value]
			if !ok {
				w.WriteHeader(401)
				return
			}
			delete(peer.sessions, cookie.Value)
			data.AccessToken = uuid.NewString()
			credentials(data)
		case "/v2/auth/sessions:logout-all":
			if bearer(r.Header.Get("Authorization")) == "" {
				w.WriteHeader(401)
				return
			}
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	return peer, server
}
func (p *repairM990WorkloadPeer) checkpointReply(sql string) repairM990Reply {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case strings.HasPrefix(sql, "begin"):
		return repairM990Reply{tag: "BEGIN"}
	case sql == "commit":
		return repairM990Reply{tag: "COMMIT"}
	case sql == "rollback":
		return repairM990Reply{tag: "ROLLBACK"}
	case strings.HasPrefix(sql, "SET LOCAL"):
		return repairM990Reply{tag: "SET"}
	case sql == "SELECT current_database()":
		return repairM990TextRows([]string{"database"}, []string{"recorded_identity"})
	case strings.HasPrefix(sql, "SELECT session_id::text,account_id::text FROM "):
		rows := [][]string{}
		for _, ack := range p.acknowledgements {
			rows = append(rows, []string{ack.SessionID, p.account})
		}
		return repairM990TextRows([]string{"session_id", "account_id"}, rows...)
	case strings.HasPrefix(sql, "SELECT count(*) FROM "):
		return repairM990TextRows([]string{"count"}, []string{strconv.Itoa(len(p.acknowledgements))})
	case strings.Contains(sql, "row_to_json(t)::text"):
		rows := [][]string{}
		for _, ack := range p.acknowledgements {
			rows = append(rows, []string{ack.SessionID, `{"recorded":"committed-ledger"}`})
		}
		return repairM990TextRows([]string{"id", "record"}, rows...)
	case strings.HasPrefix(sql, "SELECT event_id,envelope FROM "):
		reply := repairM990Reply{fields: []repairM990Field{{"event_id", 25}, {"envelope", 17}}, tag: "SELECT 1"}
		for _, ack := range p.acknowledgements {
			reply.rows = append(reply.rows, [][]byte{[]byte(ack.SessionID), []byte("\\x" + hex.EncodeToString([]byte(`{"recorded":"delivery"}`)))})
		}
		return reply
	default:
		return repairM990Reply{err: fmt.Errorf("unexpected checkpoint transcript: %s", sql)}
	}
}
func repairM990MeasurementConfig(t *testing.T, delay time.Duration) IdentityMeasurementConfig {
	t.Helper()
	peer, server := repairM990Workload(t, delay)
	pool := repairM990Postgres(t, peer.checkpointReply)
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	_, candidate, err := identityMeasurementSources(root)
	if err != nil {
		t.Fatal(err)
	}
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return IdentityMeasurementConfig{Binding: IdentityMeasurementBinding{CandidateSHA256: candidate, InterfaceSHA256: measurementHash([]byte("recorded-v2-contract")), TopologySHA256: measurementHash([]byte("recorded-loopback-peers")), Region: "loopback", RunID: uuid.NewString()}, Clients: []IdentityMeasurementClient{{BaseURL: server.URL, Origin: "https://kindle.test", Client: client}}, Phases: []IdentityMeasurementPhase{{Name: "bounded", SignInRefreshRPS: 2, DynamicBudgetRPS: 4, WriteBudgetRPS: 4, Duration: time.Second}}, Workers: 1, QueueCapacity: 2, MaxAcknowledgements: 2, RefreshesPerLogin: 1, SourceRoot: root, RequestTimeout: 2 * time.Second, EvidenceDirectory: t.TempDir(), Pool: pool, Schema: "users"}
}
func TestRepairM_990_MeasurementLoadWindow(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		delay   time.Duration
		healthy bool
	}{{"healthy", 0, true}, {"drain", 1100 * time.Millisecond, false}} {
		t.Run(scenario.name, func(t *testing.T) {
			cfg := repairM990MeasurementConfig(t, scenario.delay)
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			summary, err := RunIdentityMeasurement(ctx, cfg)
			if err == nil || summary.FinalAcceptance {
				t.Fatal("bounded harness claimed shared acceptance")
			}
			if summary.Requests != 4 || summary.SuccessfulRequests != 4 || summary.SuccessfulSignInRefresh != 2 || summary.GeneratorRejected != 0 || summary.UnexpectedFailures != 0 || summary.Checkpoint == nil {
				t.Fatalf("real HTTP workload failed: %+v", summary)
			}
			window := summary.PhaseWindows["bounded"]
			if window == nil || window.OfferedJourneys != 1 || window.EnqueuedJourneys != 1 || summary.PhaseElapsedNS["bounded"] != int64(time.Second) || window.WorkersCompletedAt.Before(window.DrainStartedAt) || window.SamplingStoppedAt.Before(window.WorkersCompletedAt) || window.SchedulingCompletedAt.Before(window.StartedAt) || window.DrainNS < 0 {
				t.Fatalf("phase lifecycle evidence invalid: %+v", window)
			}
			if summary.Assertions["achieved_load_bounded"] != scenario.healthy || !summary.Assertions["offered_load_bounded"] || summary.Assertions["D11_029.2_latency"] != scenario.healthy {
				t.Fatalf("workload or latency assertion changed: %v", summary.Assertions)
			}
			if scenario.healthy {
				if window.CompletedSignInRefreshInWindow != 2 || window.CompletedSignInRefreshInDrain != 0 || summary.PhaseActualSignInRefreshRPS["bounded"] != 2 {
					t.Fatal("shutdown overhead reduced healthy throughput")
				}
			} else if window.CompletedSignInRefreshInDrain != 2 || window.DrainNS < int64(50*time.Millisecond) || summary.CorrectedP99NS <= int64(400*time.Millisecond) {
				t.Fatal("drained completions hid overloaded latency/throughput")
			}
			raw, err := os.ReadFile(filepath.Join(cfg.EvidenceDirectory, "requests.jsonl"))
			if err != nil || measurementHash(raw) != summary.RawSHA256 {
				t.Fatal("raw request evidence lost")
			}
			scanner := bufio.NewScanner(strings.NewReader(string(raw)))
			samples := 0
			for scanner.Scan() {
				var sample IdentityRequestMeasurement
				if json.Unmarshal(scanner.Bytes(), &sample) != nil || sample.CorrelationID != sample.RequestID || sample.CorrectedLatencyNS < sample.LatencyNS || sample.CompletedAt.Before(sample.StartedAt) {
					t.Fatal("request lifecycle evidence invalid")
				}
				samples++
			}
			if scanner.Err() != nil || samples != 4 {
				t.Fatal("offered request evidence incomplete")
			}
		})
	}
	cfg := repairM990MeasurementConfig(t, 0)
	cfg.Phases[0].Duration = time.Second + time.Nanosecond
	if validateIdentityMeasurement(cfg) == nil {
		t.Fatal("fractional offered journey silently truncated")
	}
}

func TestRepairM_990_MeasurementFinalEvidence(t *testing.T) {
	cfg := repairM990MeasurementConfig(t, 0)
	owner := repairM990Identity(t, cfg.Pool)
	directory := t.TempDir()
	if err := owner.WriteRuntimeMeasurementEvidence(directory, cfg.SourceRoot, cfg.Binding); err != nil {
		t.Fatal(err)
	}
	provider := IdentityRuntimeArchive{Directory: directory, Binding: cfg.Binding}
	actual, err := provider.CaptureRuntimeMeasurement()
	if err != nil || actual.ProcessID != os.Getpid() || actual.AdmissionCapacity != 128 || actual.PoolMax != 4 || actual.ProcessCPUNS < 0 {
		t.Fatalf("owning process runtime evidence unavailable: %+v %v", actual, err)
	}
	wrong := provider
	wrong.Binding.TopologySHA256 = measurementHash([]byte("other topology"))
	if _, err = wrong.CaptureRuntimeMeasurement(); err == nil {
		t.Fatal("runtime evidence reused across topology")
	}
	wrongBinding := cfg.Binding
	wrongBinding.CandidateSHA256 = measurementHash([]byte("other candidate"))
	if err = owner.WriteRuntimeMeasurementEvidence(directory, cfg.SourceRoot, wrongBinding); err == nil {
		t.Fatal("runtime source candidate mismatch accepted")
	}
	// The serial handoff emits the runner's own durable checkpoint. A peer's
	// absent fault/replay/socket evidence must remain a failure, even if it says passed.
	finalPath := filepath.Join(t.TempDir(), "final.json")
	checkpointPath := filepath.Join(cfg.EvidenceDirectory, "checkpoint.json")
	var handed IdentityDurabilityCheckpoint
	cfg.Runtime = owner
	cfg.FinalEvidence = func(ctx context.Context, checkpoint IdentityDurabilityCheckpoint) (IdentityFinalEvidence, error) {
		handed = checkpoint
		evidence := IdentityFinalEvidence{Binding: cfg.Binding, Recovery: IdentityRecoveryEvidence{DurabilityPassed: true, AZLossAcceptance: true}, Revocation: repairM990Revocation(t, cfg.Binding), Shared: IdentitySharedAcceptance{Binding: cfg.Binding, Gate: "9.1", CompletedAt: time.Now().UTC(), Assertions: map[string]bool{"declared_pass": true}}}
		if err := writeIdentityImmutableJSON(finalPath, evidence); err != nil {
			return IdentityFinalEvidence{}, err
		}
		return AwaitIdentityFinalEvidence(finalPath, checkpointPath)(ctx, checkpoint)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	summary, err := RunIdentityMeasurement(ctx, cfg)
	if err == nil || summary.FinalAcceptance || summary.Checkpoint == nil || handed.SHA256 != summary.Checkpoint.SHA256 {
		t.Fatal("serial path failed to bind its actual workload checkpoint")
	}
	var transported IdentityDurabilityCheckpoint
	if readIdentityEvidence(checkpointPath, &transported) != nil || transported.SHA256 != handed.SHA256 || identityDurabilityDigest(transported) != transported.SHA256 {
		t.Fatal("serial checkpoint producer/consumer mismatch")
	}
	for _, name := range []string{"AZ_loss_recovery", "event_replay_integrity", "final_shared_acceptance", "configured_D11_AUTH_PROFILE"} {
		if summary.Assertions[name] {
			t.Fatalf("missing actual evidence passed %s", name)
		}
	}
	if !summary.Assertions["private_socket_revocation_p99"] {
		t.Fatal("actual HTTP/socket revocation evidence was not aggregated")
	}
	if !summary.Assertions["achieved_load_bounded"] || !summary.Assertions["offered_load_bounded"] || !summary.Assertions["D11_029.2_latency"] || !summary.Assertions["source_unchanged"] || !summary.Assertions["candidate_matches_source"] {
		t.Fatalf("local acceptance was lost: %v", summary.Assertions)
	}
	after, err := CaptureIdentityDurability(ctx, cfg.Pool, cfg.Schema, cfg.Binding, handed.Acknowledged)
	if err != nil || CompareIdentityRecovery(handed, after) != nil {
		t.Fatalf("actual durable capture transport changed: %v", err)
	}
	// A receipt for an observed loopback event cannot stand in for approved AZ loss.
	start := time.Now().UTC()
	end := start.Add(time.Nanosecond)
	receiptPath := filepath.Join(t.TempDir(), "loopback-receipt.json")
	if err = writeIdentityImmutableJSON(receiptPath, IdentityFaultReceipt{Binding: cfg.Binding, Kind: "loopback_restart", StartedAt: start, EndedAt: end, Recovered: true}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	after.CapturedAt = end.Add(time.Nanosecond)
	if WriteIdentityRecoveryEvidence(filepath.Join(t.TempDir(), "recovery.json"), handed, after, receiptPath, measurementHash(raw), start, end) == nil {
		t.Fatal("loopback fixture claimed actual AZ-loss acceptance")
	}
	changed := IdentityFinalEvidence{Binding: wrongBinding}
	applyIdentityFinalEvidence(&summary, changed)
	if len(summary.Failures) == 0 || summary.Failures[len(summary.Failures)-1] != "final_evidence_binding_mismatch" {
		t.Fatal("final evidence candidate not checked")
	}
	deadline, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	_, err = AwaitIdentityFinalEvidence(filepath.Join(t.TempDir(), "missing.json"), filepath.Join(t.TempDir(), "checkpoint.json"))(deadline, handed)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("missing final evidence wait is not bounded: %v", err)
	}
}

type repairM990KeyStore map[string][]byte

func (keys repairM990KeyStore) ResolveIdentityKey(ctx context.Context, reference string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key, ok := keys[reference]
	if !ok {
		return nil, errors.New("key not found in fixture store")
	}
	return append([]byte(nil), key...), nil
}
func repairM990Revocation(t *testing.T, binding IdentityMeasurementBinding) IdentityRevocationMeasurement {
	t.Helper()
	now := time.Now().UTC()
	keys := repairM990KeyStore{"measurement-key": []byte(strings.Repeat("k", 48))}
	tokens, err := NewIdentityTokens("https://kindle.test", "gateway", "A", []SigningKey{{ID: "A", SecretReference: "measurement-key", ActiveAt: now.Add(-time.Hour), RetireAt: now.Add(time.Hour)}}, keys, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	owner := Principal{accountID: uuid.NewString(), sessionID: uuid.NewString(), wallet: newWallet(t).addr, accessID: strings.Repeat("a", 64)}
	second := owner
	second.sessionID = uuid.NewString()
	second.accessID = strings.Repeat("b", 64)
	var mu sync.Mutex
	revoked, pending := false, false
	pool := repairM990Postgres(t, func(sql string) repairM990Reply {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case sql == "begin":
			return repairM990Reply{tag: "BEGIN"}
		case sql == "commit":
			if pending {
				revoked = true
			}
			return repairM990Reply{tag: "COMMIT"}
		case sql == "rollback":
			pending = false
			return repairM990Reply{tag: "ROLLBACK"}
		case strings.HasPrefix(sql, "SELECT a.account_id::text,s.session_id::text,s.wallet,s.auth_epoch,s.reauthenticated_at"):
			reply := repairM990Reply{fields: []repairM990Field{{"account_id", 25}, {"session_id", 25}, {"wallet", 25}, {"auth_epoch", 20}, {"reauthenticated_at", 1184}}, tag: "SELECT 0"}
			if !revoked {
				principal := owner
				if strings.Contains(sql, second.sessionID) {
					principal = second
				}
				reply.rows = [][][]byte{{[]byte(principal.accountID), []byte(principal.sessionID), []byte(principal.wallet), []byte("0"), []byte(now.Format("2006-01-02 15:04:05.999999999-07:00"))}}
				reply.tag = "SELECT 1"
			}
			return reply
		case strings.HasPrefix(sql, "SELECT EXISTS(SELECT 1 FROM identity_sessions"):
			value := "t"
			if revoked {
				value = "f"
			}
			return repairM990Reply{fields: []repairM990Field{{"exists", 16}}, rows: [][][]byte{{[]byte(value)}}, tag: "SELECT 1"}
		case strings.HasPrefix(sql, "SELECT auth_epoch FROM identity_accounts"):
			return repairM990TextRows([]string{"auth_epoch"}, []string{"0"})
		case strings.HasPrefix(sql, "UPDATE identity_accounts SET auth_epoch"):
			pending = true
			return repairM990Reply{tag: "UPDATE 1"}
		case strings.HasPrefix(sql, "UPDATE identity_sessions SET revoked_at"):
			return repairM990Reply{tag: "UPDATE 2"}
		case strings.HasPrefix(sql, "INSERT INTO identity_outbox"):
			return repairM990Reply{tag: "INSERT 0 1"}
		default:
			return repairM990Reply{err: fmt.Errorf("unexpected revocation transcript: %s", sql)}
		}
	})
	identity := repairM990Identity(t, pool)
	identity.tokens = tokens
	router := chi.NewRouter()
	identity.registerOwnerV2Routes(router)
	if err := identity.RegisterV2Routes(router, IdentityHTTPConfig{ServiceToken: strings.Repeat("t", 32)}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == "https://kindle.test" }}
	router.Get("/private", func(w http.ResponseWriter, r *http.Request) {
		principal, err := identity.AuthenticateRequest(r.Context(), r)
		if err != nil {
			identity.writeIdentityError(w, err)
			return
		}
		connection, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		identity.EnforceWebSocket(ctx, principal, connection)
	})
	server := httptest.NewServer(router)
	defer func() { cancel(); server.Close() }()
	client := IdentityMeasurementClient{BaseURL: server.URL, Origin: "https://kindle.test", Client: server.Client()}
	probes := []IdentityRevocationProbe{}
	for _, principal := range []Principal{owner, second} {
		token, err := tokens.Mint(ctx, principal)
		if err != nil {
			t.Fatal(err)
		}
		connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/private", http.Header{"Authorization": []string{"Bearer " + token}, "Origin": []string{"https://kindle.test"}})
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Close()
		probes = append(probes, IdentityRevocationProbe{Client: client, AccessToken: token, Socket: connection})
	}
	measurement, err := identity.MeasureRevocationPropagation(ctx, probes[0], probes, binding)
	if err != nil || !measurement.Passed || measurement.P99UpperBoundNS > int64(5*time.Second) || len(measurement.Observations) != 2 {
		t.Fatalf("actual HTTP/private-socket measurement: %+v %v", measurement, err)
	}
	for _, observation := range measurement.Observations {
		if observation.HTTPDeniedAt.IsZero() || observation.SocketClosedAt.IsZero() || observation.Failure != "" {
			t.Fatal("actual revocation observation missing")
		}
	}
	return measurement
}
