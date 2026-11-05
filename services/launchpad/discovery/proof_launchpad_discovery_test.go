// Proves: Store.SaveRule/DisableRule/CreateSquad/RequestJoin/SetMemberRole/Squad -> HTTP.Handler and AdaptV2AlertCreate/AdaptV2SquadCreate/AdaptV2SquadRead/AdaptV2SquadMembership -> TestProofLaunchpadDiscovery and v2.ValidateJSON.
// The first consumer is this test: it decodes the real server's frozen v2 bodies.
// PostgreSQL, database/sql, net/http and mutual TLS all run without doubles.
// The proof owns an isolated PostgreSQL process and restarts it on the same data
// directory; it needs PostgreSQL binaries, but no configured database service.
// Repairs: errors.go writeError accepted malformed request IDs; alerts_v2.go
// AdaptV2AlertCreate accepted an absent generation time; squads_v2.go squadV2
// discarded description and visibility, with compensation in http.go Handler.
// Handler also failed to resolve Wallet on deletion. alerts.go DisableRule and
// squads.go Store.Squad trusted a document ID different from its database key;
// Squad also returned semantically malformed producer documents as successful reads.
package discovery

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

// proofDiscoveryDecode is the consumer boundary: schema validation precedes
// decoding because encoding/json alone accepts missing fields and unknown fields.
func proofDiscoveryDecode(schema string, body []byte, dst any) error {
	if err := v2.ValidateJSON(schema, body); err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}

func proofDiscoveryStore(t *testing.T) (Store, string, func()) {
	t.Helper()
	output, err := exec.Command("pg_config", "--bindir").CombinedOutput()
	if err != nil {
		t.Fatalf("services/launchpad/discovery proof requires real PostgreSQL binaries (pg_config): %v: %s", err, output)
	}
	bin := strings.TrimSpace(string(output))
	dir, err := os.MkdirTemp("/tmp", "discovery-proof-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	var credential *syscall.Credential
	if os.Geteuid() == 0 {
		// PostgreSQL refuses root. Drop privileges only for its child processes.
		account, err := user.Lookup("postgres")
		if err != nil {
			t.Fatalf("PostgreSQL process owner: %v", err)
		}
		uid, err := strconv.ParseUint(account.Uid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		gid, err := strconv.ParseUint(account.Gid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		if err := os.Chown(dir, int(uid), int(gid)); err != nil {
			t.Fatal(err)
		}
	}
	run := func(name string, args ...string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, filepath.Join(bin, name), args...)
		cmd.Dir = dir
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
		output, err := cmd.CombinedOutput()
		if err != nil {
			log, _ := os.ReadFile(filepath.Join(dir, "postgres.log"))
			return fmt.Errorf("PostgreSQL %s: %w: %s\n%s", name, err, output, log)
		}
		return nil
	}
	data := filepath.Join(dir, "data")
	if err := run("initdb", "-D", data, "-U", "postgres", "-A", "trust", "--no-locale", "-E", "UTF8"); err != nil {
		t.Fatal(err)
	}
	start := func() {
		t.Helper()
		if err := run("pg_ctl", "-D", data, "-l", filepath.Join(dir, "postgres.log"), "-w", "-t", "20", "-o", "-h '' -p 5432 -k "+dir, "start"); err != nil {
			t.Fatal(err)
		}
	}
	stop := func() error { return run("pg_ctl", "-D", data, "-w", "-t", "20", "-m", "fast", "stop") }
	start()
	t.Cleanup(func() {
		if err := stop(); err != nil {
			t.Error(err)
		}
	})
	dsn := "host=" + dir + " port=5432 user=postgres dbname=postgres sslmode=disable"
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, Schema); err != nil {
		t.Fatal(err)
	}
	return Store{DB: db}, dsn, func() {
		t.Helper()
		if err := stop(); err != nil {
			t.Fatal(err)
		}
		start()
	}
}

func TestProofLaunchpadDiscovery(t *testing.T) {
	s, dsn, restartDatabase := proofDiscoveryStore(t)
	ctx := context.Background()
	owner, applicant, outsider := uuid.NewString(), uuid.NewString(), uuid.NewString()
	requestID := uuid.NewString()
	const wallet = "0x1234567890abcdef1234567890abcdef12345678"
	at := time.Now().UTC().Truncate(time.Millisecond)
	marshal := func(value any) []byte {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	consume := func(schema string, body []byte, dst any) {
		t.Helper()
		if err := proofDiscoveryDecode(schema, body, dst); err != nil {
			t.Fatalf("consumer rejected %s: %v: %s", schema, err, body)
		}
	}

	// The supplied identity functions derive their results from certificates
	// verified by the real TLS stack, just as the HTTP surface requires.
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "discovery proof CA"}, NotBefore: at.Add(-time.Hour), NotAfter: at.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err = x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	certificate := func(account, wallet string, serial int64) tls.Certificate {
		t.Helper()
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{SerialNumber: account, CommonName: wallet}, NotBefore: at.Add(-time.Hour), NotAfter: at.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
		der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	}
	certificates := map[string]tls.Certificate{
		owner: certificate(owner, wallet, 2), applicant: certificate(applicant, wallet, 3),
		outsider: certificate(outsider, wallet, 4), "no-wallet": certificate(owner, "", 5),
	}
	identity := func(r *http.Request) (*x509.Certificate, error) {
		if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 {
			return nil, ErrForbidden
		}
		return r.TLS.VerifiedChains[0][0], nil
	}
	var server *httptest.Server
	var clients map[string]*http.Client
	start := func() {
		t.Helper()
		h := HTTP{Store: s, Authenticate: func(r *http.Request) (string, error) {
			cert, err := identity(r)
			if err != nil {
				return "", err
			}
			return cert.Subject.SerialNumber, nil
		}, Wallet: func(r *http.Request) (string, error) {
			cert, err := identity(r)
			if err != nil {
				return "", err
			}
			if cert.Subject.CommonName == "" {
				return "", ErrUnavailable
			}
			return cert.Subject.CommonName, nil
		}}
		server = httptest.NewUnstartedServer(h.Handler())
		server.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots, MinVersion: tls.VersionTLS12}
		server.StartTLS()
		t.Cleanup(server.Close)
		clients = make(map[string]*http.Client)
		for name, cert := range certificates {
			transport := server.Client().Transport.(*http.Transport).Clone()
			transport.TLSClientConfig = transport.TLSClientConfig.Clone()
			transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
			clients[name] = &http.Client{Transport: transport, Timeout: 10 * time.Second}
			t.Cleanup(transport.CloseIdleConnections)
		}
	}
	start()
	request := func(actor, method, path string, body []byte, status int, schema string, dst any) []byte {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Request-Id", requestID)
		req.Header.Set("Content-Type", "application/json")
		resp, err := clients[actor].Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != status || resp.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatalf("%s %s: status=%d headers=%v body=%s", method, path, resp.StatusCode, resp.Header, raw)
		}
		if schema == "" {
			if len(raw) != 0 {
				t.Fatalf("expected empty response: %s", raw)
			}
		} else {
			if resp.Header.Get("Content-Type") != "application/json" {
				t.Fatal(resp.Header)
			}
			consume(schema, raw, dst)
			var envelope struct {
				RequestID string `json:"requestId"`
			}
			if err := json.Unmarshal(raw, &envelope); err != nil {
				t.Fatal(err)
			}
			if _, err := uuid.Parse(envelope.RequestID); err != nil {
				t.Fatal(err)
			}
			if _, err := uuid.Parse(requestID); err == nil && envelope.RequestID != requestID {
				t.Fatalf("request correlation lost: %s", raw)
			}
		}
		return raw
	}
	refusal := func(actor, method, path string, body []byte, status int, code string) {
		t.Helper()
		var observed v2.Error
		request(actor, method, path, body, status, "Error", &observed)
		if observed.Code != code || observed.Message != http.StatusText(status) || observed.Retryable != (status == 503) || observed.RetryAfterMs != nil || observed.FieldErrors == nil || len(observed.FieldErrors) != 0 || observed.Details != nil {
			t.Fatalf("consumer error branch: %+v", observed)
		}
	}

	var created v2.SquadCreateResponse
	request(owner, "POST", "/v2/squads", marshal(v2.SquadCreateBody{Name: "Research", Description: "Producer description", Visibility: "public", JoinPolicy: "request"}), 201, "SquadCreateResponse", &created)
	id := created.Data.Squad.SquadId
	var alertIDs []string
	t.Cleanup(func() {
		defer s.DB.Close()
		for _, table := range []string{"transfers", "invitations", "members"} {
			if _, err := s.DB.Exec(`DELETE FROM narrative_discovery.`+table+` WHERE squad_id=$1`, id); err != nil {
				t.Error(err)
			}
		}
		if _, err := s.DB.Exec(`DELETE FROM narrative_discovery.squads WHERE id=$1`, id); err != nil {
			t.Error(err)
		}
		for _, id := range alertIDs {
			if _, err := s.DB.Exec(`DELETE FROM narrative_discovery.rules WHERE id=$1`, id); err != nil {
				t.Error(err)
			}
		}
	})
	if created.Data.Squad.Description != "Producer description" || created.Data.Squad.Visibility != "public" {
		t.Fatalf("create lost fields: %+v", created)
	}
	read := func(want v2.Squad) []byte {
		t.Helper()
		var last []byte
		for range 2 {
			requestID = uuid.NewString()
			var observed v2.SquadReadResponse
			last = request(outsider, "GET", "/v2/squads/"+id, nil, 200, "SquadReadResponse", &observed)
			if observed.Data.Squad != want || observed.Data.AllowedActions == nil || observed.Revision == nil || v2.UInt256(*observed.Revision) != want.Revision {
				t.Fatalf("read=%+v want=%+v", observed, want)
			}
		}
		return last
	}
	rawRead := read(created.Data.Squad)
	for _, malformed := range [][]byte{rawRead[:len(rawRead)/2], bytes.Replace(rawRead, []byte(`"visibility":"public"`), []byte(`"visibility":"unrecognized"`), 1)} {
		var observed v2.SquadReadResponse
		if err := proofDiscoveryDecode("SquadReadResponse", malformed, &observed); err == nil {
			t.Fatalf("consumer accepted corrupted producer body: %s", malformed)
		}
	}
	q, err := s.Squad(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		body, err := AdaptV2SquadCreate(requestID, q, at)
		if err != nil {
			t.Fatal(err)
		}
		var observed v2.SquadCreateResponse
		consume("SquadCreateResponse", body, &observed)
		if observed.Data.Squad != created.Data.Squad {
			t.Fatalf("exported adapter lost fields: %+v", observed)
		}
	}
	// A real update followed by a PostgreSQL and HTTP server restart defeats
	// a cached response or a Store value reusing the previous connection pool.
	q.Description, q.Revision = "Changed producer description", 2
	if err := s.UpdateSquad(ctx, owner, q, 1); err != nil {
		t.Fatal(err)
	}
	server.Close()
	if err := s.DB.Close(); err != nil {
		t.Fatal(err)
	}
	restartDatabase()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	s = Store{DB: db}
	start()
	want := created.Data.Squad
	want.Description, want.Revision = q.Description, "2"
	read(want)
	refusal(owner, "GET", "/v2/squads/"+uuid.NewString(), nil, 404, "NOT_FOUND")
	memberPath := "/v2/squads/" + id + "/members/" + applicant
	membership := func(actor, action, expected, state, role, revision string) {
		t.Helper()
		var observed v2.SquadMembershipResponse
		request(actor, "PUT", memberPath, []byte(fmt.Sprintf(`{"action":%q,"expectedRevision":%q}`, action, expected)), 200, "SquadMembershipResponse", &observed)
		want := v2.SquadMembership{SquadId: id, AccountId: applicant, State: state, Role: role, Revision: v2.PositiveUInt256(revision)}
		if observed.Data.Membership != want || observed.Revision == nil || string(*observed.Revision) != revision {
			t.Fatalf("membership=%+v want=%+v", observed, want)
		}
	}
	refusal(outsider, "PUT", memberPath, []byte(`{"action":"request","expectedRevision":"0"}`), 403, "SCOPE_DENIED")
	membership(applicant, "request", "0", "requested", "member", "1")
	if _, err := s.RequestJoin(ctx, applicant, id, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("repeat request: %v", err)
	}
	refusal(applicant, "PUT", memberPath, []byte(`{"action":"approve","expectedRevision":"1"}`), 403, "SCOPE_DENIED")
	membership(owner, "approve", "1", "active", "member", "2")
	membership(owner, "promote", "2", "active", "moderator", "3")
	refusal(owner, "PUT", memberPath, []byte(`{"action":"demote","expectedRevision":"2"}`), 409, "CONFLICT")
	if err := s.SetMemberRole(ctx, outsider, id, applicant, "member", 3); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-owner role change: %v", err)
	}
	if err := s.SetMemberRole(ctx, owner, id, applicant, "member", 3); err != nil {
		t.Fatal(err)
	}
	var memberDoc []byte
	if err := db.QueryRow(`SELECT document FROM narrative_discovery.members WHERE squad_id=$1 AND account_id=$2`, id, applicant).Scan(&memberDoc); err != nil {
		t.Fatal(err)
	}
	var m Membership
	if err := json.Unmarshal(memberDoc, &m); err != nil {
		t.Fatal(err)
	}
	if m.Role != "member" || m.Status != "active" || m.Revision != 4 {
		t.Fatalf("persisted role: %+v", m)
	}
	for range 2 {
		body, err := AdaptV2SquadMembership(requestID, m, q, at)
		if err != nil {
			t.Fatal(err)
		}
		var observed v2.SquadMembershipResponse
		consume("SquadMembershipResponse", body, &observed)
		if observed.Data.Membership.Role != m.Role || observed.Data.Membership.Revision != "4" {
			t.Fatal(observed)
		}
	}
	want.MemberCount = "2"
	read(want)

	chain, currency, provider := "125", "USD", "market-provider"
	decimals := v2.SafeInteger(18)
	input := v2.AlertCreateBody{PoolAddress: wallet, Direction: "crosses_above", CooldownSeconds: 300, Threshold: v2.InputPrice{
		Numerator: "900719925474099312345678901234567890", Denominator: "1000000000000000001",
		BaseAsset:  v2.InputAssetRef{Kind: "erc20", ChainId: &chain, Address: func() *string { v := wallet; return &v }(), Decimals: &decimals},
		QuoteAsset: v2.InputAssetRef{Kind: "fiat_display", CurrencyCode: &currency},
		Source:     v2.InputPriceSource{Kind: "external_display", AmountBasis: "provider_display", ProviderId: &provider, ObservedAt: at.Format("2006-01-02T15:04:05.000Z")},
	}}
	for range 2 {
		var response v2.AlertCreateResponse
		request(owner, "POST", "/v2/me/alerts", marshal(input), 201, "AlertCreateResponse", &response)
		alert := response.Data.Alert
		for _, previous := range alertIDs {
			if previous == alert.AlertId {
				t.Fatal("create reused cached alert")
			}
		}
		alertIDs = append(alertIDs, alert.AlertId)
		if alert.Wallet != wallet || alert.PoolAddress != wallet || alert.State != "enabled" || alert.Direction != input.Direction || alert.CooldownSeconds != input.CooldownSeconds || !bytes.Equal(marshal(alert.Threshold), marshal(input.Threshold)) {
			t.Fatalf("alert lost fields: %+v", response)
		}
		var document []byte
		if err := db.QueryRow(`SELECT document FROM narrative_discovery.rules WHERE id=$1`, alert.AlertId).Scan(&document); err != nil {
			t.Fatal(err)
		}
		var rule AlertRule
		if err := json.Unmarshal(document, &rule); err != nil {
			t.Fatal(err)
		}
		if rule.OwnerAccountID != owner || rule.Revision != 1 || rule.Disabled {
			t.Fatal(rule)
		}
		for range 2 {
			produced, err := AdaptV2AlertCreate(rule, wallet, input.Threshold, requestID, at)
			if err != nil {
				t.Fatal(err)
			}
			var observed v2.AlertCreateResponse
			consume("AlertCreateResponse", marshal(produced), &observed)
			if !reflect.DeepEqual(observed.Data.Alert, alert) {
				t.Fatalf("adapter=%+v HTTP=%+v", observed.Data.Alert, alert)
			}
		}
		if _, err := AdaptV2AlertCreate(rule, wallet, input.Threshold, requestID, time.Time{}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("absent producer time: %v", err)
		}
		mismatch := input.Threshold
		mismatch.Numerator = "1"
		if _, err := AdaptV2AlertCreate(rule, wallet, mismatch, requestID, at); !errors.Is(err, ErrInvalid) {
			t.Fatalf("mismatched threshold: %v", err)
		}
		path := "/v2/me/alerts/" + alert.AlertId
		refusal(outsider, "DELETE", path, nil, 404, "NOT_FOUND")
		refusal("no-wallet", "DELETE", path, nil, 503, "DEPENDENCY_UNAVAILABLE")
		var afterRefusal []byte
		if err := db.QueryRow(`SELECT document FROM narrative_discovery.rules WHERE id=$1`, rule.ID).Scan(&afterRefusal); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(document, afterRefusal) {
			t.Fatal("refused deletion mutated producer state")
		}
		// Corrupt the real persisted producer document, retaining its original
		// bytes for restoration: the mutation must not disable a different ID.
		corrupt := rule
		corrupt.ID = uuid.NewString()
		if _, err := db.Exec(`UPDATE narrative_discovery.rules SET document=$1 WHERE id=$2`, marshal(corrupt), rule.ID); err != nil {
			t.Fatal(err)
		}
		if err := s.DisableRule(ctx, owner, rule.ID); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("mismatched document ID: %v", err)
		}
		if _, err := db.Exec(`UPDATE narrative_discovery.rules SET document=$1 WHERE id=$2`, document, rule.ID); err != nil {
			t.Fatal(err)
		}
		request(owner, "DELETE", path, nil, 204, "", nil)
		refusal(owner, "DELETE", path, nil, 404, "NOT_FOUND")
		if err := db.QueryRow(`SELECT document FROM narrative_discovery.rules WHERE id=$1`, rule.ID).Scan(&document); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(document, &rule); err != nil {
			t.Fatal(err)
		}
		if !rule.Disabled || rule.Revision != 2 {
			t.Fatalf("disable not persisted: %+v", rule)
		}
	}
	// The exported read must refuse a malformed real producer document at
	// its decode boundary, before a consumer can treat it as a valid Squad.
	for _, corrupt := range []Squad{func() Squad { c := q; c.ID = uuid.NewString(); return c }(), func() Squad { c := q; c.Visibility = "unrecognized"; return c }()} {
		if _, err := db.Exec(`UPDATE narrative_discovery.squads SET document=$1 WHERE id=$2`, marshal(corrupt), id); err != nil {
			t.Fatal(err)
		}
		if got, err := s.Squad(ctx, id); !errors.Is(err, ErrInvalid) || got != (Squad{}) {
			t.Fatalf("malformed producer accepted: %+v %v", got, err)
		}
		refusal(owner, "GET", "/v2/squads/"+id, nil, 400, "INVALID_REQUEST")
	}
	if _, err := db.Exec(`UPDATE narrative_discovery.squads SET document=$1 WHERE id=$2`, marshal(q), id); err != nil {
		t.Fatal(err)
	}
	requestID = "malformed-request-id"
	refusal(owner, "POST", "/v2/squads", []byte(`{"name":`), 400, "INVALID_REQUEST")
}
