package rehearsal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/archive"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/attestor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/envelope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/migrate"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/testsupport"
)

type repairM174Isolation struct {
	options        Options
	env            map[string]string
	deliveryCalls  atomic.Int64
	rehearsalCalls atomic.Int64
}

func repairM174ProtectedDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func repairM174IsolationFixture(t *testing.T) *repairM174Isolation {
	t.Helper()
	f := &repairM174Isolation{}
	dir := repairM174ProtectedDir(t)
	ca, err := newAuthority()
	if err != nil {
		t.Fatal(err)
	}
	caFile := filepath.Join(dir, "ca.pem")
	if err := writePEM(caFile, "CERTIFICATE", ca.cert.Raw); err != nil {
		t.Fatal(err)
	}
	operatorCA, err := newAuthority()
	if err != nil {
		t.Fatal(err)
	}
	if err := writePEM(filepath.Join(dir, "operator-ca.pem"), "CERTIFICATE", operatorCA.cert.Raw); err != nil {
		t.Fatal(err)
	}
	_, cert, key, err := operatorCA.issue(dir, "operator", x509.ExtKeyUsageClientAuth)
	if err != nil {
		t.Fatal(err)
	}
	_, gatewayCert, gatewayKey, err := ca.issue(dir, "gateway", x509.ExtKeyUsageClientAuth)
	if err != nil {
		t.Fatal(err)
	}
	tokens := filepath.Join(dir, "owner-tokens.json")
	if err := os.WriteFile(tokens, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	config := attestor.Config{CertFile: cert, KeyFile: key, CAFile: caFile, GatewayCertFile: gatewayCert, GatewayKeyFile: gatewayKey, OwnerTokensFile: tokens}
	delivery := config
	roots := x509.NewCertPool()
	roots.AddCert(ca.cert)
	roots.AddCert(operatorCA.cert)
	for i := 0; i < attestor.NodeCount; i++ {
		id := fmt.Sprintf("node-%d", i+1)
		member, certPath, keyPath, err := ca.issue(dir, id, x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth)
		if err != nil {
			t.Fatal(err)
		}
		pair, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			t.Fatal(err)
		}
		for _, production := range []bool{true, false} {
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if production {
					f.deliveryCalls.Add(1)
					t.Error("rehearsal contacted a delivery member")
					http.Error(w, "delivery boundary crossed", http.StatusForbidden)
					return
				}
				f.rehearsalCalls.Add(1)
				var request struct {
					SessionID string `json:"session_id"`
					KeyID     string `json:"key_id"`
				}
				if r.Method != "POST" || r.URL.Path != attestor.PathDescribe || json.NewDecoder(r.Body).Decode(&request) != nil || request.SessionID == "" || request.KeyID != "repair-m174-absent" {
					t.Error("unexpected isolated-member request")
					http.Error(w, "invalid transcript request", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				if err := json.NewEncoder(w).Encode(attestor.ErrorBody{Error: attestor.ErrorDetail{Category: "not_found", Code: "key_not_found", Message: "key not found"}}); err != nil {
					t.Error(err)
				}
			}))
			srv.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
			srv.StartTLS()
			t.Cleanup(srv.Close)
			node := attestor.NodeConfig{ID: id, URL: srv.URL, Pin: attestor.SPKIHash(member)}
			if production {
				delivery.Nodes = append(delivery.Nodes, node)
			} else {
				config.Nodes = append(config.Nodes, node)
			}
		}
	}
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			JSONRPC string `json:"jsonrpc"`
			ID      int    `json:"id"`
			Method  string `json:"method"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || request.JSONRPC != "2.0" || request.ID != 1 || request.Method != "eth_chainId" {
			t.Error("unexpected RPC request")
			http.Error(w, "invalid RPC evidence", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": "0x7d"}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(rpc.Close)
	rehearsalJournal, deliveryJournal := repairM174ProtectedDir(t), repairM174ProtectedDir(t)
	journalKey := bytes.Repeat([]byte{0x74}, 32)
	keyFile := filepath.Join(dir, "journal.key")
	if err := os.WriteFile(keyFile, journalKey, 0600); err != nil {
		t.Fatal(err)
	}
	ceremony := migrate.Options{CeremonyID: "repair-m174", JournalDir: rehearsalJournal, JournalKey: journalKey, RehearsalReceipt: filepath.Join(dir, "receipt.sealed"), Rehearsal: true, RPCURL: rpc.URL}
	f.options = Options{SourceURL: "postgres://reader@127.0.0.1:1/source", MigrationsDir: dir, AdminURL: "postgres://admin@127.0.0.1:1/postgres", ArchivePath: filepath.Join(dir, "funded.archive"), MasterKey: bytes.Repeat([]byte{0x17}, 32), Passphrase: []byte("repair m174 archive passphrase sufficiently long"), AttestorConfig: &config, DeliveryAttestorConfig: &delivery, CeremonyOptions: &ceremony, DeliveryJournalDir: deliveryJournal}
	f.env = map[string]string{
		EnvSourceURL: f.options.SourceURL, EnvGatewayMigrationsDir: dir, EnvAdminURL: f.options.AdminURL,
		archive.EnvPath: f.options.ArchivePath, archive.EnvPassphrase: string(f.options.Passphrase), envelope.EnvMasterKey: base64.StdEncoding.EncodeToString(f.options.MasterKey),
		"CEREMONY_ID": ceremony.CeremonyID, "CEREMONY_JOURNAL_DIR": deliveryJournal, "CEREMONY_JOURNAL_KEY_FILE": keyFile, "CEREMONY_REHEARSAL_RECEIPT": ceremony.RehearsalReceipt, "CEREMONY_RPC_URL": rpc.URL,
		"CEREMONY_REHEARSAL_JOURNAL_DIR": rehearsalJournal, "CEREMONY_REHEARSAL_RPC_URL": rpc.URL,
	}
	for _, group := range []struct {
		prefix string
		cfg    attestor.Config
	}{{"CEREMONY_", delivery}, {"CEREMONY_REHEARSAL_", config}} {
		var nodes, pins []string
		for _, node := range group.cfg.Nodes {
			nodes = append(nodes, node.ID+"="+node.URL)
			pins = append(pins, node.ID+"="+hex.EncodeToString(node.Pin[:]))
		}
		for name, value := range map[string]string{"NODES": strings.Join(nodes, ","), "NODE_PINS": strings.Join(pins, ","), "TLS_CERT_FILE": group.cfg.CertFile, "TLS_KEY_FILE": group.cfg.KeyFile, "TLS_CA_FILE": group.cfg.CAFile, "GATEWAY_TLS_CERT_FILE": group.cfg.GatewayCertFile, "GATEWAY_TLS_KEY_FILE": group.cfg.GatewayKeyFile, "OWNER_TOKENS_FILE": group.cfg.OwnerTokensFile} {
			f.env[group.prefix+name] = value
		}
	}
	return f
}

// The external-package loader test shares this fully populated environment.
func RepairM174Environment(t *testing.T) map[string]string {
	t.Helper()
	return repairM174IsolationFixture(t).env
}

func TestRepairM_174_RehearsalStaleOptions(t *testing.T) {
	f := repairM174IsolationFixture(t)
	sentinel := []byte("delivery journal must remain untouched")
	path := filepath.Join(f.options.DeliveryJournalDir, "production.sealed")
	if err := os.WriteFile(path, sentinel, 0600); err != nil {
		t.Fatal(err)
	}
	before := sha256.Sum256(sentinel)
	options, err := LoadOptions(func(k string) string { return f.env[k] })
	if err != nil {
		t.Fatal(err)
	}
	defer options.Wipe()
	if !options.CeremonyOptions.Rehearsal || options.CeremonyOptions.RPCURL != f.options.CeremonyOptions.RPCURL || options.CeremonyOptions.JournalDir != f.options.CeremonyOptions.JournalDir || options.DeliveryJournalDir != f.options.DeliveryJournalDir {
		t.Fatal("isolated options changed in transport")
	}
	client, err := attestor.New(*options.AttestorConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	production, err := attestor.New(*options.DeliveryAttestorConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer production.Close()
	if !reflect.DeepEqual(client.Membership(), production.Membership()) {
		t.Fatal("rehearsal member identities differ from delivery")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.RequireAbsent(ctx, "repair-m174-absent"); err != nil {
		t.Fatalf("real pinned mTLS client: %v", err)
	}
	if f.rehearsalCalls.Load() != 5 || f.deliveryCalls.Load() != 0 {
		t.Fatalf("isolated=%d delivery=%d", f.rehearsalCalls.Load(), f.deliveryCalls.Load())
	}
	after, err := os.ReadFile(path)
	if err != nil || sha256.Sum256(after) != before {
		t.Fatal("delivery journal changed")
	}
	payload := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}`)
	req, err := http.NewRequestWithContext(ctx, "POST", options.CeremonyOptions.RPCURL, payload)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var evidence struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&evidence); err != nil || response.StatusCode != 200 || evidence.Result != "0x7d" {
		t.Fatalf("RPC evidence: %+v %v", evidence, err)
	}
	for name := range f.env {
		if name == "CEREMONY_RPC_URL" {
			continue
		}
		t.Run("requires-"+name, func(t *testing.T) {
			unexpected, err := LoadOptions(func(key string) string {
				if key == name {
					return ""
				}
				return f.env[key]
			})
			if err == nil {
				unexpected.Wipe()
				t.Fatalf("options loaded without %s", name)
			}
		})
	}
}

func TestRepairM_174_TLSIdentityPins(t *testing.T) {
	f := repairM174IsolationFixture(t)
	config := *f.options.AttestorConfig
	config.Nodes = append(config.Nodes[:0:0], config.Nodes...)
	config.Nodes[0].Pin[0] ^= 1
	client, err := attestor.New(config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.RequireAbsent(ctx, "repair-m174-absent"); !errors.Is(err, attestor.ErrDisagree) {
		t.Fatalf("wrong server identity accepted: %v", err)
	}
	if f.rehearsalCalls.Load() != 0 || f.deliveryCalls.Load() != 0 {
		t.Fatal("request body crossed an unpinned TLS connection")
	}
}

func TestRepairM_174_IsolationRefusals(t *testing.T) {
	f := repairM174IsolationFixture(t)
	cases := []struct {
		name string
		edit func(*Options)
	}{
		{"missing-config", func(o *Options) { o.AttestorConfig = nil }},
		{"missing-delivery-config", func(o *Options) { o.DeliveryAttestorConfig = nil }},
		{"missing-ceremony", func(o *Options) { o.CeremonyOptions = nil }},
		{"delivery-mode", func(o *Options) { o.CeremonyOptions.Rehearsal = false }},
		{"missing-rpc", func(o *Options) { o.CeremonyOptions.RPCURL = "" }},
		{"same-journal", func(o *Options) { o.CeremonyOptions.JournalDir = o.DeliveryJournalDir }},
		{"relative-journal", func(o *Options) { o.CeremonyOptions.JournalDir = "relative" }},
		{"wrong-pin", func(o *Options) { o.AttestorConfig.Nodes[0].Pin[0] ^= 1 }},
		{"wrong-member", func(o *Options) { o.AttestorConfig.Nodes[0].ID = "other" }},
		{"missing-member", func(o *Options) { o.AttestorConfig.Nodes = o.AttestorConfig.Nodes[:4] }},
		{"delivery-endpoint", func(o *Options) { o.AttestorConfig.Nodes[0].URL = o.DeliveryAttestorConfig.Nodes[0].URL }},
		{"duplicate-endpoint", func(o *Options) { o.AttestorConfig.Nodes[0].URL = o.AttestorConfig.Nodes[1].URL }},
		{"duplicate-member", func(o *Options) { o.AttestorConfig.Nodes[0].ID = o.AttestorConfig.Nodes[1].ID }},
		{"insecure-endpoint", func(o *Options) { o.AttestorConfig.Nodes[0].URL = "http://127.0.0.1:1" }},
		{"operator-as-gateway", func(o *Options) { o.AttestorConfig.GatewayCertFile = o.AttestorConfig.CertFile }},
		{"missing-owner-evidence", func(o *Options) { o.AttestorConfig.OwnerTokensFile = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := f.options
			config, delivery, ceremony := *o.AttestorConfig, *o.DeliveryAttestorConfig, *o.CeremonyOptions
			config.Nodes = append([]attestor.NodeConfig(nil), config.Nodes...)
			delivery.Nodes = append([]attestor.NodeConfig(nil), delivery.Nodes...)
			o.AttestorConfig, o.DeliveryAttestorConfig, o.CeremonyOptions = &config, &delivery, &ceremony
			tc.edit(&o)
			if _, err := Rehearse(context.Background(), o); !errors.Is(err, ErrConfig) {
				t.Fatalf("unsafe options reached database boundary: %v", err)
			}
		})
	}
	t.Run("readable-journal", func(t *testing.T) {
		if err := os.Chmod(f.options.CeremonyOptions.JournalDir, 0750); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(f.options.CeremonyOptions.JournalDir, 0700)
		if _, err := Rehearse(context.Background(), f.options); !errors.Is(err, ErrConfig) {
			t.Fatalf("readable journal accepted: %v", err)
		}
	})
	t.Run("aliased-journal", func(t *testing.T) {
		o := f.options
		ceremony := *o.CeremonyOptions
		o.CeremonyOptions = &ceremony
		link := filepath.Join(t.TempDir(), "delivery-link")
		if err := os.Symlink(o.DeliveryJournalDir, link); err != nil {
			t.Fatal(err)
		}
		ceremony.JournalDir = link
		if _, err := Rehearse(context.Background(), o); !errors.Is(err, ErrConfig) {
			t.Fatalf("aliased journal accepted: %v", err)
		}
	})
	if f.deliveryCalls.Load() != 0 || f.rehearsalCalls.Load() != 0 {
		t.Fatal("refused rehearsal contacted members")
	}
}

type repairM174Exchange struct {
	sql        string
	extended   bool
	messages   []pgproto3.BackendMessage
	status     byte
	disconnect bool
}

func repairM174Command(sql, tag string, status byte) repairM174Exchange {
	return repairM174Exchange{sql: sql, messages: []pgproto3.BackendMessage{&pgproto3.CommandComplete{CommandTag: []byte(tag)}}, status: status}
}
func repairM174Count(file string, present int) repairM174Exchange {
	// pgx pads simple-protocol substitutions to keep values separate from SQL tokens.
	return repairM174Exchange{sql: fmt.Sprintf("select count(*) from _migrations where filename =  '%s' ", file), messages: []pgproto3.BackendMessage{
		&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("count"), DataTypeOID: 20, DataTypeSize: 8, TypeModifier: -1}}},
		&pgproto3.DataRow{Values: [][]byte{[]byte(fmt.Sprint(present))}}, &pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")}}, status: 'T'}
}

// This peer replays a bounded PostgreSQL protocol exchange. It has no SQL
// interpreter or simulated durable database; assertions qualify the production
// migration client's transaction boundaries, including a lost connection.
func repairM174Connect(t *testing.T, script []repairM174Exchange) *pgx.Conn {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	completed := make(chan error, 1)
	go func() {
		completed <- func() error {
			socket, err := listener.Accept()
			if err != nil {
				return err
			}
			defer socket.Close()
			if err := socket.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
				return err
			}
			peer := pgproto3.NewBackend(socket, socket)
			startup, err := peer.ReceiveStartupMessage()
			if err != nil {
				return err
			}
			if _, ok := startup.(*pgproto3.StartupMessage); !ok {
				return fmt.Errorf("startup: %T", startup)
			}
			peer.Send(&pgproto3.AuthenticationOk{})
			peer.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"})
			peer.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
			peer.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
			if err := peer.Flush(); err != nil {
				return err
			}
			for _, exchange := range script {
				if exchange.extended {
					request, err := peer.Receive()
					if err != nil {
						return err
					}
					parse, ok := request.(*pgproto3.Parse)
					if !ok || parse.Query != exchange.sql {
						return fmt.Errorf("want extended %q, got %#v", exchange.sql, request)
					}
					peer.Send(&pgproto3.ParseComplete{})
					request, err = peer.Receive()
					if err != nil {
						return err
					}
					if _, ok := request.(*pgproto3.Bind); !ok {
						return fmt.Errorf("want Bind, got %T", request)
					}
					peer.Send(&pgproto3.BindComplete{})
					request, err = peer.Receive()
					if err != nil {
						return err
					}
					if _, ok := request.(*pgproto3.Describe); !ok {
						return fmt.Errorf("want Describe, got %T", request)
					}
					peer.Send(&pgproto3.NoData{})
					request, err = peer.Receive()
					if err != nil {
						return err
					}
					if _, ok := request.(*pgproto3.Execute); !ok {
						return fmt.Errorf("want Execute, got %T", request)
					}
					request, err = peer.Receive()
					if err != nil {
						return err
					}
					if _, ok := request.(*pgproto3.Sync); !ok {
						return fmt.Errorf("want Sync, got %T", request)
					}
				} else {
					request, err := peer.Receive()
					if err != nil {
						return err
					}
					query, ok := request.(*pgproto3.Query)
					if !ok || query.String != exchange.sql {
						return fmt.Errorf("want query %q, got %#v", exchange.sql, request)
					}
				}
				if exchange.disconnect {
					return nil
				}
				for _, message := range exchange.messages {
					peer.Send(message)
				}
				peer.Send(&pgproto3.ReadyForQuery{TxStatus: exchange.status})
				if err := peer.Flush(); err != nil {
					return err
				}
			}
			message, err := peer.Receive()
			if err != nil {
				return err
			}
			if _, ok := message.(*pgproto3.Terminate); !ok {
				return fmt.Errorf("want Terminate, got %T", message)
			}
			return nil
		}()
	}()
	cfg, err := pgx.ParseConfig("postgres://fixture@" + listener.Addr().String() + "/fixture?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		conn.Close(closeCtx)
		select {
		case err := <-completed:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(2 * time.Second):
			t.Error("protocol peer did not finish")
		}
	})
	return conn
}

func repairM174Migration(t *testing.T, sql string) (Migrations, string) {
	t.Helper()
	dir := t.TempDir()
	file := "001_non_idempotent.sql"
	if err := os.WriteFile(filepath.Join(dir, file), []byte(sql), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := LoadMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	return m, file
}

func TestRepairM_174_MigrationLedgerAtomicity(t *testing.T) {
	const statement = "insert into repair_events(value) values ('once')"
	m, file := repairM174Migration(t, statement+";")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	begin := repairM174Command("begin", "BEGIN", 'T')
	executed := repairM174Command(statement, "INSERT 0 1", 'T')
	executed.extended = true
	record := repairM174Command("insert into _migrations(filename) values ( '"+file+"' )", "INSERT 0 1", 'T')
	// Lose the real pgx connection after execution and before any ledger response.
	interrupted := record
	interrupted.disconnect = true
	first := repairM174Connect(t, []repairM174Exchange{begin, repairM174Count(file, 0), executed, interrupted})
	if err := m.apply(ctx, first, file); !errors.Is(err, ErrTarget) || !strings.Contains(err.Error(), "record ") {
		t.Fatalf("interruption: %v", err)
	}
	if !first.IsClosed() {
		t.Fatal("interrupted connection remained usable")
	}
	// Reopen the production client against the rollback/retry transcript.
	second := repairM174Connect(t, []repairM174Exchange{begin, repairM174Count(file, 0), executed, record, repairM174Command("commit", "COMMIT", 'I')})
	if err := m.apply(ctx, second, file); err != nil {
		t.Fatalf("reopened migration: %v", err)
	}
	if second.PgConn().TxStatus() != 'I' {
		t.Fatal("successful migration left an open transaction")
	}
	second.Close(ctx)
	// A further reopened client sees the committed ledger and never replays SQL.
	third := repairM174Connect(t, []repairM174Exchange{begin, repairM174Count(file, 1), repairM174Command("rollback", "ROLLBACK", 'I')})
	if err := m.apply(ctx, third, file); err != nil {
		t.Fatalf("committed migration repeated: %v", err)
	}
	third.Close(ctx)
}

func TestRepairM_174_MigrationFailuresRollback(t *testing.T) {
	const statement = "insert into repair_events(value) values ('once')"
	for _, stage := range []string{"execute", "record", "commit"} {
		t.Run(stage, func(t *testing.T) {
			m, file := repairM174Migration(t, statement)
			begin := repairM174Command("begin", "BEGIN", 'T')
			executed := repairM174Command(statement, "INSERT 0 1", 'T')
			executed.extended = true
			record := repairM174Command("insert into _migrations(filename) values ( '"+file+"' )", "INSERT 0 1", 'T')
			failure := func(exchange repairM174Exchange, status byte) repairM174Exchange {
				exchange.messages = []pgproto3.BackendMessage{&pgproto3.ErrorResponse{Severity: "ERROR", Code: "40001", Message: "interrupted transaction"}}
				exchange.status = status
				return exchange
			}
			script := []repairM174Exchange{begin, repairM174Count(file, 0)}
			switch stage {
			case "execute":
				script = append(script, failure(executed, 'E'), repairM174Command("rollback", "ROLLBACK", 'I'))
			case "record":
				script = append(script, executed, failure(record, 'E'), repairM174Command("rollback", "ROLLBACK", 'I'))
			case "commit":
				script = append(script, executed, record, failure(repairM174Command("commit", "COMMIT", 'I'), 'I'))
			}
			conn := repairM174Connect(t, script)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := m.apply(ctx, conn, file); !errors.Is(err, ErrTarget) {
				t.Fatalf("%s failure accepted: %v", stage, err)
			}
			if conn.PgConn().TxStatus() != 'I' {
				t.Fatal("failed migration retained uncommitted state")
			}
			conn.Close(ctx)
		})
	}
}

func TestRepairM_174_TransactionControlSQL(t *testing.T) {
	cases := []string{"BEGIN; select 1; COMMIT", "select 1; /* comment */ COMMIT", "START TRANSACTION", "END WORK", "ROLLBACK", "ABORT", "SAVEPOINT s", "RELEASE SAVEPOINT s", "PREPARE TRANSACTION 'x'", "EXECUTE unsafe", "select 1; -- line\n /* outer /* nested */ */ CoMmIt AND CHAIN", `select E'escaped\'quote;'; COMMIT`, "select 'backslash\\'; COMMIT"}
	for _, sql := range cases {
		t.Run(sql, func(t *testing.T) {
			m, file := repairM174Migration(t, sql)
			// A nil connection proves refusal precedes all SQL, including BEGIN.
			if err := m.apply(context.Background(), nil, file); !errors.Is(err, ErrMigrations) {
				t.Fatalf("transaction control accepted: %v", err)
			}
		})
	}
	valid := []string{
		"-- BEGIN;\ncreate table events (value text); insert into events values ('COMMIT; BEGIN; it''s data')",
		`DO $body$ BEGIN PERFORM 'COMMIT;'; END $body$; COMMENT ON TABLE events IS 'ROLLBACK;'`,
		`/* outer /* COMMIT; */ comment */ SELECT E'escaped\'quote;'; SELECT "COMMIT;" FROM events`,
	}
	for _, sql := range valid {
		statements, err := migrationStatements(sql)
		if err != nil || len(statements) != 2 {
			t.Fatalf("valid SQL split incorrectly: %q: %v %v", sql, statements, err)
		}
	}
	for _, sql := range []string{"select 'unterminated", "DO $body$ BEGIN END", "/* unterminated"} {
		if _, err := migrationStatements(sql); err == nil {
			t.Fatalf("malformed SQL accepted: %q", sql)
		}
	}
}

func TestRepairM_174_CopyDigests(t *testing.T) {
	const data = "1\twallet-one\n2\twallet-two\n"
	table := Table{Name: "wallets", Columns: []Column{{Name: "id"}, {Name: "address"}}, PrimaryKey: []string{"id"}}
	copyOut := func(query string, body string) repairM174Exchange {
		return repairM174Exchange{sql: query, messages: []pgproto3.BackendMessage{
			&pgproto3.CopyOutResponse{ColumnFormatCodes: []uint16{0, 0}}, &pgproto3.CopyData{Data: []byte(body)}, &pgproto3.CopyDone{}, &pgproto3.CommandComplete{CommandTag: []byte("COPY 2")}}, status: 'I'}
	}
	source := repairM174Connect(t, []repairM174Exchange{copyOut(table.CopyOut(table.Ident(), "s"), data)})
	// COPY FROM has its own streaming protocol, rather than Query/CommandComplete.
	target, finish := repairM174CopyTarget(t, table.CopyIn(table.Ident()), []byte(data))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	streamed, loaded, digest, err := Stream(ctx, &Source{conn: source}, target.PgConn(), table, table.CopyIn(table.Ident()))
	if err != nil || streamed != 2 || loaded != 2 || digest != sha256.Sum256([]byte(data)) {
		t.Fatalf("producer -> COPY transport -> consumer: %d %d %x %v", streamed, loaded, digest, err)
	}
	finish()
	target.Close(ctx)
	source.Close(ctx)
	copied := repairM174Connect(t, []repairM174Exchange{copyOut(table.CopyOut(table.Ident(), "t"), data), copyOut(table.CopyOut(table.Ident(), "t"), strings.Replace(data, "wallet-two", "wallet-bad", 1))})
	targetDigest, n, err := Digest(ctx, copied.PgConn(), table.CopyOut(table.Ident(), "t"))
	if err != nil {
		t.Fatal(err)
	}
	report := CopyReport{Tables: []TableCopy{{Table: table.Name, SourceRows: streamed, TargetRows: n, SourceDigest: digest, TargetDigest: targetDigest}}}
	if err := report.Verify(); err != nil {
		t.Fatal(err)
	}
	altered, n, err := Digest(ctx, copied.PgConn(), table.CopyOut(table.Ident(), "t"))
	if err != nil || n != 2 {
		t.Fatalf("reread altered stream: %d %v", n, err)
	}
	report.Tables[0].TargetDigest = altered
	if err := report.Verify(); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("altered target accepted: %v", err)
	}
	report.Tables[0].TargetRows = 1
	if err := report.Verify(); !errors.Is(err, ErrCountMismatch) {
		t.Fatalf("short target accepted: %v", err)
	}
	copied.Close(ctx)
}

func repairM174CopyTarget(t *testing.T, query string, want []byte) (*pgx.Conn, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	done := make(chan error, 1)
	go func() {
		done <- func() error {
			socket, err := listener.Accept()
			if err != nil {
				return err
			}
			defer socket.Close()
			if err := socket.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
				return err
			}
			peer := pgproto3.NewBackend(socket, socket)
			if _, err := peer.ReceiveStartupMessage(); err != nil {
				return err
			}
			peer.Send(&pgproto3.AuthenticationOk{})
			peer.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
			if err := peer.Flush(); err != nil {
				return err
			}
			message, err := peer.Receive()
			if err != nil {
				return err
			}
			request, ok := message.(*pgproto3.Query)
			if !ok || request.String != query {
				return fmt.Errorf("want COPY %q, got %#v", query, message)
			}
			peer.Send(&pgproto3.CopyInResponse{ColumnFormatCodes: []uint16{0, 0}})
			if err := peer.Flush(); err != nil {
				return err
			}
			var received bytes.Buffer
			for {
				message, err = peer.Receive()
				if err != nil {
					return err
				}
				switch value := message.(type) {
				case *pgproto3.CopyData:
					received.Write(value.Data)
				case *pgproto3.CopyDone:
					if !bytes.Equal(received.Bytes(), want) {
						return fmt.Errorf("copied bytes differ: %q", received.Bytes())
					}
					peer.Send(&pgproto3.CommandComplete{CommandTag: []byte("COPY 2")})
					peer.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
					if err := peer.Flush(); err != nil {
						return err
					}
					message, err = peer.Receive()
					if err != nil {
						return err
					}
					if _, ok := message.(*pgproto3.Terminate); !ok {
						return fmt.Errorf("want Terminate, got %T", message)
					}
					return nil
				default:
					return fmt.Errorf("unexpected COPY message %T", message)
				}
			}
		}()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, "postgres://fixture@"+listener.Addr().String()+"/fixture?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	finish := func() {
		t.Helper()
		once.Do(func() {
			conn.Close(context.Background())
			select {
			case err := <-done:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(2 * time.Second):
				t.Error("COPY peer did not finish")
			}
		})
	}
	t.Cleanup(finish)
	return conn, finish
}

// This service-bound companion is deliberately outside the repair gate's name
// filter. The gate itself must run without PostgreSQL or subprocesses.
func TestMigrationLedgerAtomicityPostgresAfterInterruption(t *testing.T) {
	pg := testsupport.StartPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, pg.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	for _, sql := range []string{ledgerDDL, `create table repair_events (value text primary key)`,
		`create function interrupt_migration_record() returns trigger language plpgsql as $$ begin perform pg_terminate_backend(pg_backend_pid()); return NEW; end $$`,
		`create trigger interrupt_migration_record before insert on _migrations for each row execute function interrupt_migration_record()`} {
		if _, err := conn.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	m, file := repairM174Migration(t, `insert into repair_events(value) values ('once');`)
	if err := m.apply(ctx, conn, file); !errors.Is(err, ErrTarget) || !strings.Contains(err.Error(), "record ") {
		t.Fatalf("real PostgreSQL interruption: %v", err)
	}
	conn.Close(ctx)
	reopened, err := pgx.Connect(ctx, pg.URL)
	if err != nil {
		t.Fatal(err)
	}
	var events, ledger int
	if err := reopened.QueryRow(ctx, `select (select count(*) from repair_events), (select count(*) from _migrations)`).Scan(&events, &ledger); err != nil {
		t.Fatal(err)
	}
	if events != 0 || ledger != 0 {
		t.Fatalf("interrupted transaction survived reopen: events=%d ledger=%d", events, ledger)
	}
	if _, err := reopened.Exec(ctx, `drop trigger interrupt_migration_record on _migrations`); err != nil {
		t.Fatal(err)
	}
	if err := m.apply(ctx, reopened, file); err != nil {
		t.Fatal(err)
	}
	reopened.Close(ctx)
	recovered, err := pgx.Connect(ctx, pg.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close(context.Background())
	if err := m.apply(ctx, recovered, file); err != nil {
		t.Fatalf("non-idempotent migration replayed after reopen: %v", err)
	}
	if err := recovered.QueryRow(ctx, `select (select count(*) from repair_events), (select count(*) from _migrations)`).Scan(&events, &ledger); err != nil {
		t.Fatal(err)
	}
	if events != 1 || ledger != 1 {
		t.Fatalf("recovered effects and ledger disagree: events=%d ledger=%d", events, ledger)
	}
}

func TestRepairM_174_GatewayMigrationSyntax(t *testing.T) {
	m, err := LoadMigrations(testsupport.GatewayMigrationsDir(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range m.Files {
		t.Run(file, func(t *testing.T) {
			sql, err := os.ReadFile(filepath.Join(m.Dir, file))
			if err != nil {
				t.Fatal(err)
			}
			statements, err := migrationStatements(string(sql))
			if err != nil || len(statements) == 0 {
				t.Fatalf("gateway migration is unsupported: %v (%d statements)", err, len(statements))
			}
		})
	}
}

func TestRepairM_174_RejectsEnclosingTransaction(t *testing.T) {
	m, file := repairM174Migration(t, "select 1")
	conn := repairM174Connect(t, []repairM174Exchange{repairM174Command("begin", "BEGIN", 'T'), repairM174Command("rollback", "ROLLBACK", 'I')})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := conn.Exec(ctx, "begin"); err != nil {
		t.Fatal(err)
	}
	if err := m.apply(ctx, conn, file); !errors.Is(err, ErrTarget) || !strings.Contains(err.Error(), "idle connection") {
		t.Fatalf("enclosing transaction accepted: %v", err)
	}
	if _, err := conn.Exec(ctx, "rollback"); err != nil {
		t.Fatal(err)
	}
	conn.Close(ctx)
}
