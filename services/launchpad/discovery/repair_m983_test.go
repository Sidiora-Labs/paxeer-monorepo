package discovery

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"modernc.org/sqlite"
)

// The fixture peer is an actual, pure Go SQL database. Only PostgreSQL syntax is
// translated; SQL reads, constraints, JSON updates, transactions and persistence
// are executed by SQLite. One connection serializes writes in place of advisory
// locks. This is not PostgreSQL lock/privilege qualification (see repair_m983.kvx).
type repairM983Driver struct{ driver.Driver }
type repairM983Conn struct{ driver.Conn }

var repairM983Parameters = regexp.MustCompile(`\$(\d+)`)
var repairM983JSONText = regexp.MustCompile(`([a-zA-Z_.]+)->>'([a-zA-Z_]+)'`)

func repairM983SQL(query string) string {
	query = strings.NewReplacer(
		"narrative_discovery.", "",
		" FOR UPDATE SKIP LOCKED", "",
		" FOR UPDATE", "",
		"::jsonb", "",
		"::timestamptz", "",
		"clock_timestamp()-interval '1 minute'", "strftime('%Y-%m-%dT%H:%M:%fZ','now','-1 minute')",
		"clock_timestamp()", "strftime('%Y-%m-%dT%H:%M:%fZ','now')",
		"pg_advisory_xact_lock(hashtextextended($1,210021))", "length($1)",
		"jsonb_set(", "repair_m983_json_set(",
	).Replace(query)
	query = repairM983JSONText.ReplaceAllString(query, "repair_m983_json_text($1,'$2')")
	return repairM983Parameters.ReplaceAllString(query, "?$1")
}

func (d *repairM983Driver) Open(name string) (driver.Conn, error) {
	c, err := d.Driver.Open(name)
	if err != nil {
		return nil, err
	}
	return &repairM983Conn{c}, nil
}
func (c *repairM983Conn) Prepare(query string) (driver.Stmt, error) {
	return c.Conn.Prepare(repairM983SQL(query))
}
func (c *repairM983Conn) CheckNamedValue(v *driver.NamedValue) error {
	switch x := v.Value.(type) {
	case []byte:
		v.Value = string(x)
	case uint64:
		v.Value = strconv.FormatUint(x, 10)
	case time.Time:
		v.Value = x.UTC().Format(time.RFC3339Nano)
	default:
		var err error
		v.Value, err = driver.DefaultParameterConverter.ConvertValue(v.Value)
		return err
	}
	return nil
}
func (c *repairM983Conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, repairM983SQL(query), args)
}
func (c *repairM983Conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, repairM983SQL(query), args)
}

func init() {
	registered, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	sql.Register("repair-m983-sql", &repairM983Driver{registered.Driver()})
	if err := registered.Close(); err != nil {
		panic(err)
	}
	sqlite.MustRegisterDeterministicScalarFunction("repair_m983_json_set", 3, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		var doc map[string]json.RawMessage
		if err := json.Unmarshal([]byte(args[0].(string)), &doc); err != nil {
			return nil, err
		}
		path := strings.Trim(args[1].(string), "{}")
		if path == "" || strings.Contains(path, ",") {
			return nil, fmt.Errorf("unsupported JSON path %q", path)
		}
		value := json.RawMessage(args[2].(string))
		if !json.Valid(value) {
			return nil, fmt.Errorf("invalid JSON value %s", value)
		}
		doc[path] = value
		raw, err := json.Marshal(doc)
		return string(raw), err
	})
	sqlite.MustRegisterDeterministicScalarFunction("repair_m983_json_text", 2, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		var doc map[string]any
		d := json.NewDecoder(strings.NewReader(args[0].(string)))
		d.UseNumber()
		if err := d.Decode(&doc); err != nil {
			return nil, err
		}
		value := doc[args[1].(string)]
		if value == nil {
			return nil, nil
		}
		return fmt.Sprint(value), nil
	})
}

type repairM983Fixture struct {
	t        *testing.T
	store    Store
	path     string
	server   *httptest.Server
	password string
}

func repairM983Open(t *testing.T) *repairM983Fixture {
	t.Helper()
	f := &repairM983Fixture{t: t, path: filepath.Join(t.TempDir(), "discovery.sqlite"), password: uuid.NewString()}
	f.open()
	for _, statement := range strings.Split(Schema, ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" || strings.HasPrefix(statement, "CREATE SCHEMA") || strings.HasPrefix(statement, "REVOKE") {
			continue
		}
		if _, err := f.store.DB.Exec(statement); err != nil {
			t.Fatalf("fixture schema %s: %v", statement, err)
		}
	}
	f.start()
	t.Cleanup(func() { f.server.Close(); f.store.DB.Close() })
	return f
}

func (f *repairM983Fixture) open() {
	f.t.Helper()
	db, err := sql.Open("repair-m983-sql", f.path)
	if err != nil {
		f.t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		f.t.Fatal(err)
	}
	f.store = Store{DB: db}
}

func (f *repairM983Fixture) start() {
	h := HTTP{Store: f.store, Authenticate: func(r *http.Request) (string, error) {
		account, password, ok := r.BasicAuth()
		if !ok || account == "" || password != f.password {
			return "", ErrForbidden
		}
		return account, nil
	}, Wallet: func(*http.Request) (string, error) { return repair46Wallet, nil }}
	f.server = httptest.NewServer(h.Handler())
}

func (f *repairM983Fixture) reopen() {
	f.t.Helper()
	f.server.CloseClientConnections()
	f.server.Close()
	old := f.store.DB
	if err := old.Close(); err != nil {
		f.t.Fatal(err)
	}
	if err := old.Ping(); err == nil {
		f.t.Fatal("old store remains open")
	}
	f.open()
	f.start()
}

func (f *repairM983Fixture) request(actor, method, path string, input any, status int, schema string, dst any) []byte {
	f.t.Helper()
	var body []byte
	if input != nil {
		body = repair46JSON(f.t, input)
	}
	req, err := http.NewRequest(method, f.server.URL+path, bytes.NewReader(body))
	if err != nil {
		f.t.Fatal(err)
	}
	req.SetBasicAuth(actor, f.password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", discoveryErrorRequestID)
	res, err := f.server.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	if res.StatusCode != status || res.Header.Get("Cache-Control") != "private, no-store" {
		f.t.Fatalf("%s %s: status=%d want=%d body=%s", method, path, res.StatusCode, status, raw)
	}
	if schema != "" {
		if err := v2.ValidateJSON(schema, raw); err != nil {
			f.t.Fatalf("consumer %s: %v: %s", schema, err, raw)
		}
	}
	if dst != nil {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.DisallowUnknownFields()
		if err := d.Decode(dst); err != nil {
			f.t.Fatal(err)
		}
	}
	return raw
}

func (f *repairM983Fixture) create(owner, policy string) Squad {
	f.t.Helper()
	var result v2.SquadCreateResponse
	f.request(owner, "POST", "/v2/squads", v2.SquadCreateBody{Name: "Research", Description: "Persisted contract", Visibility: "private", JoinPolicy: policy}, 201, "SquadCreateResponse", &result)
	q, err := f.store.Squad(context.Background(), result.Data.Squad.SquadId)
	if err != nil {
		f.t.Fatal(err)
	}
	return q
}

func (f *repairM983Fixture) membership(actor, id, account, action, expected string, status int) v2.SquadMembershipResponse {
	f.t.Helper()
	var result v2.SquadMembershipResponse
	schema, dst := "SquadMembershipResponse", any(&result)
	if status != 200 {
		schema, dst = "Error", nil
	}
	f.request(actor, "PUT", "/v2/squads/"+id+"/members/"+account, v2.SquadMembershipBody{Action: action, ExpectedRevision: v2.UInt256(expected)}, status, schema, dst)
	return result
}

func (f *repairM983Fixture) member(id, account string) Membership {
	f.t.Helper()
	var raw []byte
	if err := f.store.DB.QueryRow(`SELECT document FROM narrative_discovery.members WHERE squad_id=$1 AND account_id=$2`, id, account).Scan(&raw); err != nil {
		f.t.Fatal(err)
	}
	var m Membership
	if err := json.Unmarshal(raw, &m); err != nil {
		f.t.Fatal(err)
	}
	return m
}

func TestRepairM_983_alert_registry_signature(t *testing.T) {
	f := repairM983Open(t)
	owner := uuid.NewString()
	at := time.Now().UTC().Truncate(time.Millisecond)
	chain, currency, provider, asset := "125", "USD", "market-provider", repair46Wallet
	decimals := v2.SafeInteger(18)
	input := v2.AlertCreateBody{PoolAddress: asset, Direction: "crosses_above", CooldownSeconds: 300, Threshold: v2.InputPrice{
		Numerator: "900719925474099312345678901234567890", Denominator: "1000000000000000001",
		BaseAsset:  v2.InputAssetRef{Kind: "erc20", ChainId: &chain, Address: &asset, Decimals: &decimals},
		QuoteAsset: v2.InputAssetRef{Kind: "fiat_display", CurrencyCode: &currency},
		Source:     v2.InputPriceSource{Kind: "external_display", AmountBasis: "provider_display", ProviderId: &provider, ObservedAt: at.Format("2006-01-02T15:04:05.000Z")},
	}}
	var created v2.AlertCreateResponse
	f.request(owner, "POST", "/v2/me/alerts", input, 201, "AlertCreateResponse", &created)
	f.reopen()
	var raw []byte
	if err := f.store.DB.QueryRow(`SELECT document FROM narrative_discovery.rules WHERE id=$1 AND account_id=$2`, created.Data.Alert.AlertId, owner).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var rule AlertRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		t.Fatal(err)
	}
	if rule.Price == nil || !reflect.DeepEqual(*rule.Price, input.Threshold) || rule.Threshold != string(input.Threshold.Numerator)+"/"+string(input.Threshold.Denominator) {
		t.Fatalf("persisted threshold changed: %+v", rule)
	}
	// This typed call is a compiling proof of both the registry signature and
	// the actual handler's consumer shape, read from persisted producer state.
	var adapt func(AlertRule, string, v2.InputPrice, string, time.Time) (v2.AlertCreateResponse, error) = AdaptV2AlertCreate
	produced, err := adapt(rule, repair46Wallet, *rule.Price, discoveryErrorRequestID, at)
	if err != nil {
		t.Fatal(err)
	}
	var consumed v2.AlertCreateResponse
	if err := proofDiscoveryDecode("AlertCreateResponse", repair46JSON(t, produced), &consumed); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(consumed.Data, created.Data) || consumed.Revision == nil || *consumed.Revision != "1" {
		t.Fatalf("HTTP/adapter mismatch: %+v %+v", consumed, created)
	}
	mismatch := *rule.Price
	mismatch.Source.ProviderId = func() *string { v := "different-provider"; return &v }()
	if _, err := adapt(rule, repair46Wallet, mismatch, discoveryErrorRequestID, at); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unpersisted provenance accepted: %v", err)
	}
	mismatch = *rule.Price
	mismatch.Numerator = "1"
	if _, err := adapt(rule, repair46Wallet, mismatch, discoveryErrorRequestID, at); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unpersisted ratio accepted: %v", err)
	}
}

func TestRepairM_983_invite_revision_race(t *testing.T) {
	for _, route := range []string{"store", "frozen HTTP", "connected HTTP"} {
		t.Run(route, func(t *testing.T) {
			f := repairM983Open(t)
			ctx := context.Background()
			owner, recipient := uuid.NewString(), uuid.NewString()
			q := f.create(owner, "invite")
			expires := time.Now().UTC().Add(time.Hour)
			first := strings.Repeat("a", 64)
			if err := f.store.Invite(ctx, owner, q.ID, recipient, first, 0, expires); err != nil {
				t.Fatal(err)
			}
			start := make(chan struct{})
			type outcome struct {
				token  string
				status int
				err    error
			}
			results := make(chan outcome, 2)
			var wg sync.WaitGroup
			for _, token := range []string{strings.Repeat("b", 64), strings.Repeat("c", 64)} {
				wg.Add(1)
				go func(token string) {
					defer wg.Done()
					<-start
					if route == "store" {
						err := f.store.Invite(ctx, owner, q.ID, recipient, token, 1, expires)
						status := 200
						if errors.Is(err, ErrConflict) {
							status = 409
						} else if err != nil {
							status = 500
						}
						results <- outcome{token, status, err}
						return
					}
					path := "/v2/squads/" + q.ID + "/members/" + recipient
					method := "PUT"
					var input any = v2.SquadMembershipBody{Action: "invite", ExpectedRevision: "1"}
					if route == "connected HTTP" {
						path, method = "/v2/discovery/squads/"+q.ID+"/invitations", "POST"
						input = struct {
							Account, Token string
							Expires        time.Time
							Expected       uint64
						}{recipient, token, expires, 1}
					}
					raw, err := json.Marshal(input)
					if err != nil {
						results <- outcome{token, 0, err}
						return
					}
					req, err := http.NewRequest(method, f.server.URL+path, bytes.NewReader(raw))
					if err != nil {
						results <- outcome{token, 0, err}
						return
					}
					req.SetBasicAuth(owner, f.password)
					req.Header.Set("Content-Type", "application/json")
					res, err := f.server.Client().Do(req)
					if err != nil {
						results <- outcome{token, 0, err}
						return
					}
					defer res.Body.Close()
					body, err := io.ReadAll(res.Body)
					if err == nil && res.StatusCode == 409 {
						err = v2.ValidateJSON("Error", body)
					}
					if err == nil && route == "frozen HTTP" && res.StatusCode == 200 {
						err = v2.ValidateJSON("SquadMembershipResponse", body)
					}
					results <- outcome{token, res.StatusCode, err}
				}(token)
			}
			close(start)
			wg.Wait()
			close(results)
			success, conflict, winningToken := 0, 0, ""
			for result := range results {
				if result.err != nil && !(route == "store" && errors.Is(result.err, ErrConflict)) {
					t.Fatal(result.err)
				}
				switch result.status {
				case 200:
					success++
					winningToken = result.token
				case 409:
					conflict++
				default:
					t.Fatalf("unexpected invitation status: %+v", result)
				}
			}
			if success != 1 || conflict != 1 {
				t.Fatalf("one revision admitted %d successes and %d conflicts", success, conflict)
			}
			f.reopen()
			m := f.member(q.ID, recipient)
			delivery, err := f.store.InvitationDelivery(ctx, recipient, q.ID, time.Now().UTC())
			if err != nil || m.Revision != 2 || m.Status != "invited" || delivery.Revision != 2 || delivery.Token == first {
				t.Fatalf("winning invitation not recovered: %+v %+v %v", m, delivery, err)
			}
			if route != "frozen HTTP" && delivery.Token != winningToken {
				t.Fatal("losing request replaced the winning token")
			}
			if err := f.store.Invite(ctx, owner, q.ID, recipient, first, 1, expires); !errors.Is(err, ErrConflict) {
				t.Fatalf("stale direct caller admitted: %v", err)
			}
			after, err := f.store.InvitationDelivery(ctx, recipient, q.ID, time.Now().UTC())
			if err != nil || after != delivery {
				t.Fatalf("conflict changed private authorization: %+v %v", after, err)
			}
			if _, err := f.store.RedeemInvitation(ctx, recipient, q.ID, delivery.Token, 2, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRepairM_983_join_registry_signature(t *testing.T) {
	f := repairM983Open(t)
	ctx := context.Background()
	owner, applicant, httpApplicant := uuid.NewString(), uuid.NewString(), uuid.NewString()
	q := f.create(owner, "request")
	q.Revision = 9
	if err := f.store.UpdateSquad(ctx, owner, q, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("nonsequential squad revision accepted: %v", err)
	}
	q.Revision = 2
	if err := f.store.UpdateSquad(ctx, owner, q, 1); err != nil {
		t.Fatal(err)
	}
	var request func(context.Context, string, string, uint64) (Membership, error) = f.store.RequestJoin
	requested, err := request(ctx, applicant, q.ID, 0)
	if err != nil || requested.Revision != 1 || requested.Status != "requested" {
		t.Fatalf("exported request: %+v %v", requested, err)
	}
	f.reopen()
	if got := f.member(q.ID, applicant); got != requested {
		t.Fatalf("request did not survive reopen: %+v", got)
	}
	if _, err := f.store.RequestJoin(ctx, applicant, q.ID, 0); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale request: %v", err)
	}
	if _, err := f.store.ApproveRequest(ctx, owner, q.ID, applicant, q.Revision, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("approval accepted squad revision: %v", err)
	}
	if _, err := f.store.ApproveRequest(ctx, applicant, q.ID, applicant, 1, time.Now()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("self approval: %v", err)
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	var approve func(context.Context, string, string, string, uint64, time.Time) (Membership, error) = f.store.ApproveRequest
	approved, err := approve(ctx, owner, q.ID, applicant, 1, at)
	if err != nil || approved.Revision != 2 || approved.Status != "active" || !approved.JoinedAt.Equal(at) {
		t.Fatalf("exported approval: %+v %v", approved, err)
	}
	f.reopen()
	if got := f.member(q.ID, applicant); got != approved {
		t.Fatalf("approval did not survive reopen: %+v", got)
	}
	requestedHTTP := f.membership(httpApplicant, q.ID, httpApplicant, "request", "0", 200)
	if requestedHTTP.Data.Membership.Revision != "1" || requestedHTTP.Data.Membership.State != "requested" || requestedHTTP.Data.SquadRevision != "2" {
		t.Fatal(requestedHTTP)
	}
	f.reopen()
	f.membership(owner, q.ID, httpApplicant, "approve", "0", 409)
	approvedHTTP := f.membership(owner, q.ID, httpApplicant, "approve", "1", 200)
	if approvedHTTP.Data.Membership.Revision != "2" || approvedHTTP.Data.Membership.State != "active" || approvedHTTP.Data.SquadRevision != "2" {
		t.Fatal(approvedHTTP)
	}
	f.membership(owner, q.ID, httpApplicant, "approve", "1", 409)
	if got, err := f.store.Squad(ctx, q.ID); err != nil || got != q {
		t.Fatalf("membership mutation changed squad identity: %+v %v", got, err)
	}
}

func TestRepairM_983_squad_boundary_tests(t *testing.T) {
	f := repairM983Open(t)
	assertSquadTextBoundaries(t, f.store)
	owner := uuid.NewString()
	for _, symbol := range []string{"a", "é", "🛰"} {
		for _, length := range []int{500, 501} {
			input := v2.SquadCreateBody{Name: "HTTP boundary", Description: strings.Repeat(symbol, length), Visibility: "private", JoinPolicy: "request"}
			if length == 500 {
				var created v2.SquadCreateResponse
				f.request(owner, "POST", "/v2/squads", input, 201, "SquadCreateResponse", &created)
				f.reopen()
				var read v2.SquadReadResponse
				f.request(owner, "GET", "/v2/squads/"+created.Data.Squad.SquadId, nil, 200, "SquadReadResponse", &read)
				if read.Data.Squad.Description != input.Description {
					t.Fatal("HTTP boundary text did not survive reopen")
				}
			} else {
				f.request(owner, "POST", "/v2/squads", input, 400, "Error", nil)
			}
		}
	}
}

func TestRepairM_983_squad_revision_test_fixture(t *testing.T) {
	f := repairM983Open(t)
	ctx := context.Background()
	owner, applicant := uuid.NewString(), uuid.NewString()
	q := f.create(owner, "request")
	for q.Revision < 5 {
		expected := q.Revision
		q.Revision++
		if err := f.store.UpdateSquad(ctx, owner, q, expected); err != nil {
			t.Fatal(err)
		}
	}
	mutated, err := f.store.ChangeMembership(ctx, applicant, q.ID, applicant, "request", 0)
	if err != nil || mutated.Squad.Revision != 5 || mutated.Membership.Revision != 1 {
		t.Fatalf("real mutation result: %+v %v", mutated, err)
	}
	f.reopen()
	read, err := f.store.Squad(ctx, q.ID)
	if err != nil || read != mutated.Squad || f.member(q.ID, applicant) != mutated.Membership {
		t.Fatalf("recovered identities changed: %+v %v", read, err)
	}
	assertSquadMembershipRevisions(t, MembershipMutation{Squad: read, Membership: f.member(q.ID, applicant)}, time.Now().UTC())
	approved := f.membership(owner, q.ID, applicant, "approve", "1", 200)
	if approved.Data.SquadRevision != "5" || approved.Data.Membership.Revision != "2" || approved.Revision == nil || *approved.Revision != "2" {
		t.Fatal(approved)
	}
	f.reopen()
	// Persist uint64 values above the exact JS range through the same SQL store
	// and reopen, so precision is checked at persistence and HTTP boundaries.
	q.Revision = math.MaxUint64 - 1
	m := f.member(q.ID, applicant)
	m.Revision = math.MaxUint64
	if _, err := f.store.DB.Exec(`UPDATE narrative_discovery.squads SET document=$1 WHERE id=$2`, encode(q), q.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB.Exec(`UPDATE narrative_discovery.members SET document=$1 WHERE squad_id=$2 AND account_id=$3`, encode(m), q.ID, applicant); err != nil {
		t.Fatal(err)
	}
	f.reopen()
	q, err = f.store.Squad(ctx, q.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertSquadMembershipRevisions(t, MembershipMutation{Squad: q, Membership: f.member(q.ID, applicant)}, time.Now().UTC())
	var readHTTP v2.SquadReadResponse
	f.request(owner, "GET", "/v2/squads/"+q.ID, nil, 200, "SquadReadResponse", &readHTTP)
	if readHTTP.Data.Squad.Revision != "18446744073709551614" {
		t.Fatalf("HTTP rounded squad revision: %+v", readHTTP)
	}
}

// Publisher is the committed loopback transport peer. Runtime.PublishBatch and
// Store.DeleteAccount remain the production components under test.
type repairM983Publisher struct {
	client *http.Client
	url    string
}

func (p repairM983Publisher) Publish(ctx context.Context, kind, id string, body json.RawMessage) error {
	req, err := http.NewRequestWithContext(ctx, "POST", p.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-Event-Kind", kind)
	req.Header.Set("X-Event-Id", id)
	res, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 204 {
		return fmt.Errorf("delivery status %d", res.StatusCode)
	}
	return nil
}

func TestRepairM_983_deleted_actor_outbox(t *testing.T) {
	f := repairM983Open(t)
	ctx := context.Background()
	actor, otherActor := uuid.NewString(), uuid.NewString()
	created := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	var ids []string
	// Produce events through the exact persistence path shared by Publish/Edit.
	for _, author := range []string{actor, actor, otherActor} {
		c := Callout{ID: uuid.NewString(), AuthorAccountID: author, PoolAddress: repair46Wallet, Body: "Private old body", Direction: "bullish", Disclosure: "none", ModerationStatus: "approved", PositionDisclosure: "private position", CreatedAt: created, Revision: 1}
		if err := f.store.transaction(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, `INSERT INTO narrative_discovery.callouts(id,document)VALUES($1,$2)`, c.ID, encode(c)); err != nil {
				return err
			}
			return persistCallout(ctx, tx, c)
		}); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, c.ID)
	}
	// Deletion must also scrub a target that was already removed before account deletion.
	if _, err := (CalloutService{Store: f.store}).Edit(ctx, actor, ids[1], "", 1, true); err != nil {
		t.Fatal(err)
	}
	if err := f.store.DeleteAccount(ctx, actor); err != nil {
		t.Fatal(err)
	}
	// Inspect durable bytes before any worker can repair/sanitize the output.
	for _, table := range []string{"callouts", "callout_revisions", "sources", "outbox"} {
		rows, err := f.store.DB.Query(`SELECT document FROM narrative_discovery.` + table)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(raw, []byte(actor)) {
				t.Fatalf("deleted identifier retained in %s: %s", table, raw)
			}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}
	f.reopen()
	for _, id := range ids[:2] {
		c, err := (CalloutService{Store: f.store}).Read(ctx, id)
		if err != nil || !c.Deleted || c.AuthorAccountID != "" || c.Body != "" || c.PositionDisclosure != "" {
			t.Fatalf("deleted callout recovery: %+v %v", c, err)
		}
		if _, err := (OwnedSources{Callouts: CalloutService{Store: f.store}}).ResolvePublic(ctx, "callout", id); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("deleted public read: %v", err)
		}
	}
	var mu sync.Mutex
	received := map[string]CalloutTargetEvent{}
	failOnce := true
	consumer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if failOnce {
			failOnce = false
			w.WriteHeader(503)
			return
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		var event CalloutTargetEvent
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if bytes.Contains(raw, []byte(actor)) || r.Header.Get("X-Event-Kind") != "callout-target-ref-v2" {
			t.Errorf("public identity leak: %s", raw)
			w.WriteHeader(400)
			return
		}
		if event.TargetID == ids[0] || event.TargetID == ids[1] {
			wantRevision := uint64(2)
			if event.TargetID == ids[1] {
				wantRevision = 3
			}
			if !event.Removed || event.Actor != "" || event.Revision != wantRevision {
				t.Errorf("deletion fence bypassed: %+v", event)
				w.WriteHeader(400)
				return
			}
		} else if event.TargetID != ids[2] || event.Actor != otherActor || event.Removed {
			t.Errorf("unrelated actor changed: %+v", event)
			w.WriteHeader(400)
			return
		}
		received[r.Header.Get("X-Event-Id")] = event
		w.WriteHeader(204)
	}))
	defer consumer.Close()
	publisher := repairM983Publisher{consumer.Client(), consumer.URL}
	if n, err := (Runtime{Store: f.store, Publisher: publisher}).PublishBatch(ctx, 100); err == nil || n != 0 {
		t.Fatalf("failed transport acknowledged: %d %v", n, err)
	}
	f.reopen()
	if n, err := (Runtime{Store: f.store, Publisher: publisher}).PublishBatch(ctx, 100); err != nil || n != 6 {
		t.Fatalf("recovered outbox delivery: %d %v", n, err)
	}
	mu.Lock()
	if len(received) != 6 {
		t.Fatalf("events lost: %+v", received)
	}
	mu.Unlock()
	f.reopen()
	if n, err := (Runtime{Store: f.store, Publisher: publisher}).PublishBatch(ctx, 100); err != nil || n != 0 {
		t.Fatalf("published events replayed: %d %v", n, err)
	}
}

func TestRepairM_983_missing_interface_contract_pages(t *testing.T) {
	// Each affected canonical registry target has an in-scope maintained page
	// with the same anchor. Registry publication belongs to the contract owner.
	mapping := map[string]string{
		"contracts/services.md#launchpad-discovery": "contracts/services.md#launchpad-discovery",
		"contracts/services.md#launchpad-api":       "contracts/services.md#launchpad-api",
		"contracts/clients.md#launchpad-web-app":    "contracts/clients.md#launchpad-web-app",
		"contracts/clients.md#launchpad-sdk-sdk":    "contracts/clients.md#launchpad-sdk-sdk",
	}
	check := func(target string) {
		t.Helper()
		path, anchor, _ := strings.Cut(target, "#")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("contract link %s: %v", target, err)
		}
		if anchor != "" && !bytes.Contains(raw, []byte(`id="`+anchor+`"`)) {
			t.Fatalf("contract anchor missing: %s", target)
		}
	}
	registry, err := os.ReadFile("../../../spec/layerx-merge/interfaces.kvx")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, section := range strings.Split(string(registry), "\n[interface.") {
		for canonical, local := range mapping {
			if strings.Contains(section, `contract = "`+canonical+`"`) {
				check(local)
				seen[canonical]++
			}
		}
	}
	for target := range mapping {
		if seen[target] == 0 {
			t.Fatalf("registry target disappeared without reconciliation: %s", target)
		}
	}
	links := regexp.MustCompile(`\]\(([^)]+)\)`)
	for _, page := range []string{"contracts/services.md", "contracts/clients.md"} {
		raw, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range links.FindAllSubmatch(raw, -1) {
			target := string(match[1])
			if strings.Contains(target, "://") {
				t.Fatalf("contract must use checked repository links: %s", target)
			}
			path, anchor, found := strings.Cut(target, "#")
			if path == "" {
				path = filepath.Base(page)
			}
			path = filepath.Clean(filepath.Join(filepath.Dir(page), path))
			if found {
				path += "#" + anchor
			}
			check(path)
		}
	}
	page, err := os.ReadFile("contracts/services.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{"AdaptV2AlertCreate", "v2.AlertCreateResponse", "rule.Price", "RequestJoin", "ApproveRequest", "expected", "500", "CalloutTargetEvent", "Removed", "Actor"} {
		if !bytes.Contains(page, []byte(contract)) {
			t.Fatalf("maintained contract omits %s", contract)
		}
	}
	record, err := os.ReadFile("repair_m983.kvx")
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range []string{"alert-registry-signature", "invite-revision-race", "join-registry-signature", "squad-boundary-tests", "squad-revision-test-fixture", "deleted-actor-outbox", "missing-interface-contract-pages"} {
		if !bytes.Contains(record, []byte("[finding."+finding+"]")) || !bytes.Contains(record, []byte("TestRepairM_983_"+strings.ReplaceAll(finding, "-", "_"))) {
			t.Fatalf("missing finding proof mapping: %s", finding)
		}
	}
}
