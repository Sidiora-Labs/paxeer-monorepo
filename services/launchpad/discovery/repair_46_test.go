package discovery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

const repair46Wallet = "0x1111111111111111111111111111111111111111"
const repair46Token = "0x2222222222222222222222222222222222222222"
const repair46USDL = "0x3333333333333333333333333333333333333333"

type repair46HTTP struct {
	t               *testing.T
	store           Store
	dsn             string
	restartDatabase func()
	server          *httptest.Server
	password        string
}

func repair46Server(t *testing.T) *repair46HTTP {
	t.Helper()
	s, dsn, restart := proofDiscoveryStore(t)
	h := &repair46HTTP{t: t, store: s, dsn: dsn, restartDatabase: restart, password: uuid.NewString()}
	h.start()
	t.Cleanup(func() { h.server.Close(); h.store.DB.Close() })
	return h
}

func (h *repair46HTTP) start() {
	h.t.Helper()
	server := HTTP{Store: h.store, Authenticate: func(r *http.Request) (string, error) {
		actor, password, ok := r.BasicAuth()
		if !ok || actor == "" || subtle.ConstantTimeCompare([]byte(password), []byte(h.password)) != 1 {
			return "", ErrForbidden
		}
		return actor, nil
	}, Wallet: func(r *http.Request) (string, error) {
		_, password, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(password), []byte(h.password)) != 1 {
			return "", ErrForbidden
		}
		return repair46Wallet, nil
	}}
	h.server = httptest.NewTLSServer(server.Handler())
}

func (h *repair46HTTP) reopen() {
	h.t.Helper()
	h.server.Client().CloseIdleConnections()
	h.server.Close()
	old := h.store.DB
	if err := old.Close(); err != nil {
		h.t.Fatal(err)
	}
	if err := old.Ping(); err == nil {
		h.t.Fatal("old database connection is still open")
	}
	h.restartDatabase()
	db, err := sql.Open("pgx", h.dsn)
	if err != nil {
		h.t.Fatal(err)
	}
	if err = db.Ping(); err != nil {
		h.t.Fatal(err)
	}
	h.store = Store{DB: db}
	h.start()
}

func repair46JSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (h *repair46HTTP) request(actor, method, path string, input any, status int, schema string, dst any) []byte {
	h.t.Helper()
	var body []byte
	if input != nil {
		body = repair46JSON(h.t, input)
	}
	req, err := http.NewRequest(method, h.server.URL+path, bytes.NewReader(body))
	if err != nil {
		h.t.Fatal(err)
	}
	if actor != "" {
		req.SetBasicAuth(actor, h.password)
	}
	req.Header.Set("X-Request-Id", discoveryErrorRequestID)
	req.Header.Set("Content-Type", "application/json")
	res, err := h.server.Client().Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		h.t.Fatal(err)
	}
	if res.StatusCode != status || res.Header.Get("Cache-Control") != "private, no-store" {
		h.t.Fatalf("%s %s: status %d want %d, headers %v, body %s", method, path, res.StatusCode, status, res.Header, raw)
	}
	if schema != "" {
		if err = v2.ValidateJSON(schema, raw); err != nil {
			h.t.Fatalf("consumer schema %s: %v: %s", schema, err, raw)
		}
	}
	if dst != nil {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(dst); err != nil {
			h.t.Fatalf("consumer decode: %v: %s", err, raw)
		}
	}
	return raw
}

func (h *repair46HTTP) createSquad(owner, policy, description string) v2.Squad {
	h.t.Helper()
	input := v2.SquadCreateBody{Name: "Research", Description: description, Visibility: "private", JoinPolicy: policy}
	if err := v2.ValidateJSON("SquadCreateBody", repair46JSON(h.t, input)); err != nil {
		h.t.Fatal(err)
	}
	var result v2.SquadCreateResponse
	h.request(owner, "POST", "/v2/squads", input, 201, "SquadCreateResponse", &result)
	return result.Data.Squad
}

func (h *repair46HTTP) membership(actor, id, account, action, expected string, status int) v2.SquadMembershipResponse {
	h.t.Helper()
	var out v2.SquadMembershipResponse
	input := v2.SquadMembershipBody{Action: action, ExpectedRevision: v2.UInt256(expected)}
	schema := "SquadMembershipResponse"
	var dst any = &out
	if status != 200 {
		schema, dst = "Error", nil
	}
	h.request(actor, "PUT", "/v2/squads/"+id+"/members/"+account, input, status, schema, dst)
	return out
}

func TestRepair_46_legacy_squad_contract(t *testing.T) {
	h := repair46Server(t)
	owner, outsider, id := uuid.NewString(), uuid.NewString(), uuid.NewString()
	input := v2.ConnectedSquadCreateBody{ID: id, Name: "Connected", Policy: "invite"}
	if err := v2.ValidateJSON("ConnectedSquadCreateBody", repair46JSON(t, input)); err != nil {
		t.Fatal(err)
	}
	var created v2.ConnectedDiscoverySquad
	h.request(owner, "POST", "/v2/discovery/squads", input, 200, "ConnectedDiscoverySquad", &created)
	q, err := h.store.Squad(context.Background(), id)
	if err != nil || q.Visibility != "private" || q.OwnerAccountID != owner || q.Revision != 1 {
		t.Fatalf("connected creation: %+v %v", q, err)
	}
	q.Description, q.Revision = "Preserve private description", 2
	if err = h.store.UpdateSquad(context.Background(), owner, q, 1); err != nil {
		t.Fatal(err)
	}
	patch := v2.ConnectedSquadPatchBody{Expected: 2, Squad: v2.ConnectedDiscoverySquad{ID: id, Name: "Updated", OwnerAccountID: owner, Policy: "request", Revision: 3}}
	if err = v2.ValidateJSON("ConnectedSquadPatchBody", repair46JSON(t, patch)); err != nil {
		t.Fatal(err)
	}
	h.request(outsider, "PATCH", "/v2/discovery/squads/"+id, patch, 403, "Error", nil)
	var updated v2.ConnectedDiscoverySquad
	h.request(owner, "PATCH", "/v2/discovery/squads/"+id, patch, 200, "ConnectedDiscoverySquad", &updated)
	if updated != patch.Squad || created.ID != id || created.OwnerAccountID != owner {
		t.Fatalf("SDK readback: %+v %+v", created, updated)
	}
	h.reopen()
	recovered, err := h.store.Squad(context.Background(), id)
	if err != nil || recovered.Visibility != "private" || recovered.Description != q.Description || recovered.Revision != 3 || recovered.Name != "Updated" || recovered.Policy != "request" {
		t.Fatalf("connected recovery: %+v %v", recovered, err)
	}
	var read v2.SquadReadResponse
	h.request(owner, "GET", "/v2/squads/"+id, nil, 200, "SquadReadResponse", &read)
	if read.Data.Squad.Description != q.Description || read.Data.Squad.Visibility != "private" {
		t.Fatal(read)
	}
	h.request(outsider, "GET", "/v2/squads/"+id, nil, 404, "Error", nil)
}

func TestRepair_46_frozen_squad_validation(t *testing.T) {
	h := repair46Server(t)
	owner, applicant := uuid.NewString(), uuid.NewString()
	var created []v2.Squad
	for _, description := range []string{strings.Repeat("a", 281), strings.Repeat("é", 141), strings.Repeat("é", 500), strings.Repeat("a", 500)} {
		q := h.createSquad(owner, "open", description)
		if q.Description != description || q.JoinPolicy != "open" {
			t.Fatal(q)
		}
		joined := h.membership(applicant, q.SquadId, applicant, "request", "0", 200)
		if joined.Data.Membership.State != "active" || joined.Data.Membership.Revision != "1" || joined.Data.SquadRevision != "1" {
			t.Fatal(joined)
		}
		created = append(created, q)
	}
	for _, description := range []string{strings.Repeat("a", 501), strings.Repeat("é", 501)} {
		h.request(owner, "POST", "/v2/squads", v2.SquadCreateBody{Name: "Bounds", Description: description, Visibility: "public", JoinPolicy: "open"}, 400, "Error", nil)
	}
	h.reopen()
	for _, q := range created {
		var read v2.SquadReadResponse
		h.request(applicant, "GET", "/v2/squads/"+q.SquadId, nil, 200, "SquadReadResponse", &read)
		if read.Data.Squad.Description != q.Description || read.Data.Squad.JoinPolicy != "open" || read.Data.Squad.MemberCount != "2" {
			t.Fatal(read)
		}
	}
	id := created[0].SquadId
	h.membership(owner, id, applicant, "remove", "1", 200)
	joined := h.membership(applicant, id, applicant, "request", "2", 200)
	if joined.Data.Membership.State != "active" || joined.Data.Membership.Revision != "3" {
		t.Fatal(joined)
	}
	h.membership(owner, id, applicant, "ban", "3", 200)
	h.membership(applicant, id, applicant, "request", "4", 403)
	inviteOnly := h.createSquad(owner, "invite", "")
	h.membership(applicant, inviteOnly.SquadId, applicant, "request", "0", 403)
	requested := h.createSquad(owner, "request", "")
	member := h.membership(applicant, requested.SquadId, applicant, "request", "0", 200)
	if member.Data.Membership.State != "requested" {
		t.Fatal(member)
	}
}

func TestRepair_46_invitation_token_lost(t *testing.T) {
	h := repair46Server(t)
	owner, recipient, outsider := uuid.NewString(), uuid.NewString(), uuid.NewString()
	q := h.createSquad(owner, "invite", "")
	invited := h.membership(owner, q.SquadId, recipient, "invite", "0", 200)
	if invited.Data.Membership.State != "invited" {
		t.Fatal(invited)
	}
	var page InvitationPage
	h.request(recipient, "GET", "/v2/me/squad-invitations", nil, 200, "", &page)
	if len(page.Items) != 1 {
		t.Fatal(page)
	}
	delivery := page.Items[0]
	if delivery.AccountID != recipient || delivery.SquadID != q.SquadId || len(delivery.Token) < 32 || delivery.Revision != 1 || time.Until(delivery.ExpiresAt) < 23*time.Hour {
		t.Fatal(delivery)
	}
	var raw []byte
	if err := h.store.DB.QueryRow(`SELECT document FROM narrative_discovery.invitations WHERE squad_id=$1 AND account_id=$2`, q.SquadId, recipient).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var authorization Invitation
	if json.Unmarshal(raw, &authorization) != nil || authorization.TokenHash != sha256.Sum256([]byte(delivery.Token)) || bytes.Contains(raw, []byte(delivery.Token)) {
		t.Fatalf("private authorization storage: %s", raw)
	}
	h.request(outsider, "GET", "/v2/me/squad-invitations/"+q.SquadId, nil, 404, "Error", nil)
	h.request("", "GET", "/v2/me/squad-invitations/"+q.SquadId, nil, 401, "Error", nil)
	h.reopen()
	var recovered InvitationDelivery
	h.request(recipient, "GET", "/v2/me/squad-invitations/"+q.SquadId, nil, 200, "", &recovered)
	if !reflect.DeepEqual(delivery, recovered) {
		t.Fatalf("lost delivered authorization after restart: %+v %+v", delivery, recovered)
	}
	path := "/v2/me/squad-invitations/" + q.SquadId + "/accept"
	h.request(outsider, "POST", path, v2.ConnectedSquadAcceptBody{Expected: 1, Token: delivery.Token}, 404, "Error", nil)
	h.request(recipient, "POST", path, v2.ConnectedSquadAcceptBody{Expected: 1, Token: uuid.NewString()}, 403, "Error", nil)
	h.request(recipient, "POST", path, v2.ConnectedSquadAcceptBody{Expected: 2, Token: delivery.Token}, 409, "Error", nil)
	var accepted v2.SquadMembershipResponse
	h.request(recipient, "POST", path, v2.ConnectedSquadAcceptBody{Expected: 1, Token: delivery.Token}, 200, "SquadMembershipResponse", &accepted)
	if accepted.Data.Membership.State != "active" || accepted.Data.Membership.Revision != "2" || accepted.Data.SquadRevision != "1" {
		t.Fatal(accepted)
	}
	h.request(recipient, "POST", path, v2.ConnectedSquadAcceptBody{Expected: 2, Token: delivery.Token}, 403, "Error", nil)
	h.request(recipient, "GET", "/v2/me/squad-invitations/"+q.SquadId, nil, 404, "Error", nil)
	h.request(recipient, "GET", "/v2/me/squad-invitations", nil, 200, "", &page)
	if len(page.Items) != 0 {
		t.Fatal(page)
	}
	// Exercise expiry through the real connected producer and recipient HTTP
	// consumer, with a short explicitly requested expiry rather than a clock fake.
	expiring := uuid.NewString()
	token := uuid.NewString() + uuid.NewString()
	expires := time.Now().UTC().Add(250 * time.Millisecond)
	h.request(owner, "POST", "/v2/discovery/squads/"+q.SquadId+"/invitations", v2.ConnectedSquadInviteBody{Account: expiring, Token: token, Expires: expires.Format(time.RFC3339Nano)}, 200, "", nil)
	time.Sleep(time.Until(expires) + time.Millisecond)
	h.request(expiring, "GET", "/v2/me/squad-invitations/"+q.SquadId, nil, 404, "Error", nil)
	h.request(expiring, "POST", path, v2.ConnectedSquadAcceptBody{Expected: 1, Token: token}, 403, "Error", nil)
	var count int
	if err := h.store.DB.QueryRow(`SELECT count(*) FROM narrative_discovery.members WHERE squad_id=$1 AND account_id=$2 AND document->>'Status'='active'`, q.SquadId, expiring).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestRepair_46_membership_squad_revision(t *testing.T) {
	h := repair46Server(t)
	owner, applicant := uuid.NewString(), uuid.NewString()
	q := h.createSquad(owner, "request", "")
	stored, err := h.store.Squad(context.Background(), q.SquadId)
	if err != nil {
		t.Fatal(err)
	}
	stored.Revision, stored.Description = 2, "Squad revision is independent"
	if err = h.store.UpdateSquad(context.Background(), owner, stored, 1); err != nil {
		t.Fatal(err)
	}
	for i, action := range []string{"request", "approve", "promote", "demote"} {
		actor := owner
		if action == "request" {
			actor = applicant
		}
		result := h.membership(actor, q.SquadId, applicant, action, strconv.Itoa(i), 200)
		if result.Data.Membership.Revision != v2.PositiveUInt256(strconv.Itoa(i+1)) || result.Data.SquadRevision != "2" || result.Revision == nil || *result.Revision != result.Data.Membership.Revision {
			t.Fatalf("%s conflates revisions: %+v", action, result)
		}
	}
	h.reopen()
	var read v2.SquadReadResponse
	h.request(owner, "GET", "/v2/squads/"+q.SquadId, nil, 200, "SquadReadResponse", &read)
	if read.Data.Squad.Revision != "2" || read.Revision == nil || *read.Revision != "2" {
		t.Fatal(read)
	}
	result := h.membership(owner, q.SquadId, applicant, "ban", "4", 200)
	if result.Data.Membership.Revision != "5" || result.Data.SquadRevision != "2" {
		t.Fatal(result)
	}
	mutated, err := h.store.ChangeMembership(context.Background(), owner, q.SquadId, applicant, "remove", 5)
	if err != nil || mutated.Squad.Revision != 2 || mutated.Membership.Revision != 6 {
		t.Fatal(mutated, err)
	}
	raw, err := AdaptV2SquadMembership(discoveryErrorRequestID, mutated.Membership, mutated.Squad, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var decoded v2.SquadMembershipResponse
	if err = proofDiscoveryDecode("SquadMembershipResponse", raw, &decoded); err != nil || decoded.Data.SquadRevision != "2" || decoded.Data.Membership.Revision != "6" {
		t.Fatal(decoded, err)
	}
}

// The source fixture installs the real owner migrations. append_market_price
// validates the committed fill, assets, units, source and watermark and emits
// the wire frame subsequently consumed by discovery; the test never writes a
// frame into stream_projection or substitutes a price authority implementation.
func repair46PriceSource(t *testing.T, h *repair46HTTP) {
	t.Helper()
	exec := func(statement string) {
		t.Helper()
		if _, err := h.store.DB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE SCHEMA stats`)
	migrations := filepath.Join("..", "..", "..", "launchpad", "spec", "production-system", "integration", "migrations")
	raw, err := os.ReadFile(filepath.Join(migrations, "130000.sql"))
	if err != nil {
		t.Fatal(err)
	}
	exec(string(raw))
	raw, err = os.ReadFile(filepath.Join("..", "api", "migrations", "00001_api_stream.sql"))
	if err != nil {
		t.Fatal(err)
	}
	up, _, ok := strings.Cut(string(raw), "-- +goose Down")
	if !ok {
		t.Fatal("canonical stream migration has no up section")
	}
	exec(up)
	raw, err = os.ReadFile(filepath.Join(migrations, "150004.sql"))
	if err != nil {
		t.Fatal(err)
	}
	exec(string(raw))
}

func repair46PublishPrice(t *testing.T, h *repair46HTTP, value int64, revision uint64, observed time.Time) Metric {
	t.Helper()
	observed = observed.UTC().Truncate(time.Second)
	hash := func(label string) string {
		digest := sha256.Sum256([]byte(label))
		return "0x" + hex.EncodeToString(digest[:])
	}
	rev := strconv.FormatUint(revision, 10)
	position := accounting.SourcePosition{ChainID: "125", Height: rev, TransactionIndex: "0", LogIndex: "0", TransactionHash: hash("tx:" + rev), BlockHash: hash("block:" + rev)}
	fill := accounting.FillV1{
		Version: accounting.FillVersion, Position: position, ActualEmitter: pool, PoolAddress: pool,
		Token: accounting.Asset{Address: repair46Token, Decimals: 18}, Settlement: accounting.Asset{Address: repair46USDL, Decimals: 6},
		Side: "buy", TokenAmountRaw: "1000000000000000000", USDLAmountRaw: strconv.FormatInt(value*1000000, 10), PaidInputRaw: strconv.FormatInt(value*1000000, 10), FeeRaw: "0", InputDerivationQuality: "receipt_incomplete", FeeDisposition: "distributed_usdl",
		Attribution:           accounting.Attribution{TransactionSender: repair46Wallet, Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Execution:             accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Provenance:            accounting.Provenance{DeploymentID: "repair46", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "emitter-1", PoolCodeEpoch: "pool-1", ReceiptSHA256: strings.Repeat("a", 64), HeaderSHA256: strings.Repeat("b", 64)},
		BlockTimestampSeconds: strconv.FormatInt(observed.Unix(), 10),
	}
	fill.SourceFillID = accounting.SourceFillIdentity(position, fill.Provenance.DeploymentID)
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.store.DB.Exec(`INSERT INTO stats.accounting_market_commits(publication_id,source_fill_id,source_fact_digest,projection_version,pool,bucket_start,position,fill,sequence,revision)VALUES($1,$2,$3,'repair46',$4,$5,$6,$7,$8,$8)`, "publication:"+rev, fill.SourceFillID, string(digest), pool, observed.Unix()/60*60, repair46JSON(t, position), repair46JSON(t, fill), revision); err != nil {
		t.Fatal(err)
	}
	chain, token, usdl := "125", repair46Token, repair46USDL
	tokenDecimals, usdlDecimals := v2.SafeInteger(18), v2.SafeInteger(6)
	sourceEvent := "125:" + position.TransactionHash + ":0"
	price := v2.InputPrice{
		Numerator: v2.PriceComponent(strconv.FormatInt(value, 10)), Denominator: "1",
		BaseAsset:  v2.InputAssetRef{Kind: "erc20", ChainId: &chain, Address: &token, Decimals: &tokenDecimals},
		QuoteAsset: v2.InputAssetRef{Kind: "erc20", ChainId: &chain, Address: &usdl, Decimals: &usdlDecimals},
		Source:     v2.InputPriceSource{Kind: "gross_execution", AmountBasis: "paid_input", SourceEventId: &sourceEvent, ObservedAt: observed.Format("2006-01-02T15:04:05.000Z"), Position: &v2.InputChainPosition{ChainId: "125", Height: v2.UInt256(rev), BlockHash: position.BlockHash, TransactionHash: position.TransactionHash, TransactionIndex: "0", LogIndex: "0"}},
	}
	update := map[string]any{"eventId": "price:" + sourceEvent, "topic": "market.price." + pool, "revision": rev, "occurredAt": price.Source.ObservedAt, "payload": price, "watermark": v2.Watermark{ChainId: "125", Height: v2.UInt256(rev), BlockHash: position.BlockHash, ProjectionRevision: v2.PositiveUInt256(rev), ObservedAt: price.Source.ObservedAt, Integrity: "healthy"}}
	proof := map[string]any{"kind": "price_commit", "pool": pool, "projectionVersion": "repair46", "sequence": rev, "sourceFillId": fill.SourceFillID, "sourceFactDigest": string(digest)}
	if _, err = h.store.DB.Exec(`SELECT api_stream.append_market_price($1::jsonb,$2::jsonb)`, repair46JSON(t, update), repair46JSON(t, proof)); err != nil {
		t.Fatal(err)
	}
	metric, err := h.store.CanonicalPrice(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if metric.Price == nil || !reflect.DeepEqual(*metric.Price, price) || metric.Revision != revision {
		t.Fatalf("canonical producer/consumer mismatch: %+v", metric)
	}
	return metric
}

func (h *repair46HTTP) createAlert(owner string, price v2.InputPrice) AlertRule {
	h.t.Helper()
	price.Numerator, price.Denominator = "10", "1"
	input := v2.AlertCreateBody{PoolAddress: pool, Threshold: price, Direction: "crosses_above", CooldownSeconds: 60}
	if err := v2.ValidateJSON("AlertCreateBody", repair46JSON(h.t, input)); err != nil {
		h.t.Fatal(err)
	}
	var response v2.AlertCreateResponse
	h.request(owner, "POST", "/v2/me/alerts", input, 201, "AlertCreateResponse", &response)
	var raw []byte
	if err := h.store.DB.QueryRow(`SELECT document FROM narrative_discovery.rules WHERE id=$1`, response.Data.Alert.AlertId).Scan(&raw); err != nil {
		h.t.Fatal(err)
	}
	var rule AlertRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		h.t.Fatal(err)
	}
	if rule.Price == nil || !reflect.DeepEqual(*rule.Price, input.Threshold) || rule.OwnerAccountID != owner {
		h.t.Fatalf("persisted threshold context: %+v", rule)
	}
	if !bytes.Equal(repair46JSON(h.t, response.Data.Alert.Threshold), repair46JSON(h.t, input.Threshold)) {
		h.t.Fatal("HTTP response differs from stored threshold")
	}
	return rule
}

func repair46Checkpoint(t *testing.T, s Store, id string) AlertCheckpoint {
	t.Helper()
	var raw []byte
	if err := s.DB.QueryRow(`SELECT checkpoint FROM narrative_discovery.rules WHERE id=$1`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var checkpoint AlertCheckpoint
	if err := json.Unmarshal(raw, &checkpoint); err != nil {
		t.Fatal(err)
	}
	return checkpoint
}

func TestRepair_46_alert_price_context_lost(t *testing.T) {
	h := repair46Server(t)
	repair46PriceSource(t, h)
	ctx := context.Background()
	baseline := repair46PublishPrice(t, h, 9, 1, time.Now())
	owner := uuid.NewString()
	rule := h.createAlert(owner, *baseline.Price)
	// Persist two further equal-ratio thresholds with different denominations
	// and authorities. They must remain distinguishable after reopening storage.
	inverted := *rule.Price
	inverted.BaseAsset, inverted.QuoteAsset = inverted.QuoteAsset, inverted.BaseAsset
	other := h.createAlert(owner, inverted)
	external := *rule.Price
	provider, currency := "display-provider", "USD"
	external.QuoteAsset = v2.InputAssetRef{Kind: "fiat_display", CurrencyCode: &currency}
	external.Source = v2.InputPriceSource{Kind: "external_display", AmountBasis: "provider_display", ProviderId: &provider, ObservedAt: external.Source.ObservedAt}
	display := h.createAlert(owner, external)
	h.reopen()
	for _, want := range []AlertRule{rule, other, display} {
		var raw []byte
		if err := h.store.DB.QueryRow(`SELECT document FROM narrative_discovery.rules WHERE id=$1`, want.ID).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var recovered AlertRule
		if err := json.Unmarshal(raw, &recovered); err != nil {
			t.Fatal(err)
		}
		response, err := AdaptV2AlertCreate(recovered, repair46Wallet, *recovered.Price, discoveryErrorRequestID, time.Now())
		if err != nil || !reflect.DeepEqual(recovered, want) || !bytes.Equal(repair46JSON(t, response.Data.Alert.Threshold), repair46JSON(t, want.Price)) {
			t.Fatalf("persisted response: %+v %v", response, err)
		}
	}
	now := time.Now().UTC()
	crossed, why, err := h.store.EvaluateRule(ctx, rule.ID, baseline, now)
	if err != nil || crossed || why != "baseline" {
		t.Fatal(crossed, why, err)
	}
	checkpoint := repair46Checkpoint(t, h.store, rule.ID)
	next := repair46PublishPrice(t, h, 11, 2, time.Now())
	mismatches := map[string]func(*Metric){
		"pool":           func(m *Metric) { m.PoolAddress = repair46Token },
		"base asset":     func(m *Metric) { v := repair46Wallet; m.Price.BaseAsset.Address = &v },
		"quote asset":    func(m *Metric) { v := repair46Wallet; m.Price.QuoteAsset.Address = &v },
		"base decimals":  func(m *Metric) { v := v2.SafeInteger(6); m.Price.BaseAsset.Decimals = &v },
		"quote decimals": func(m *Metric) { v := v2.SafeInteger(18); m.Price.QuoteAsset.Decimals = &v },
		"chain":          func(m *Metric) { v := "1"; m.Price.BaseAsset.ChainId = &v },
		"source": func(m *Metric) {
			m.Price.Source = v2.InputPriceSource{Kind: "external_display", AmountBasis: "provider_display", ProviderId: &provider, ObservedAt: m.Price.Source.ObservedAt}
		},
		"missing identity": func(m *Metric) { m.Price = nil },
	}
	for name, mutate := range mismatches {
		t.Run(name, func(t *testing.T) {
			metric := next
			price := *next.Price
			metric.Price = &price
			mutate(&metric)
			crossed, why, err := h.store.EvaluateRule(ctx, rule.ID, metric, time.Now())
			if err != nil || crossed || why != "price_context_mismatch" || repair46Checkpoint(t, h.store, rule.ID) != checkpoint {
				t.Fatal(crossed, why, err)
			}
		})
	}
	stale := next
	stale.ObservedAt = time.Now().Add(-11 * time.Second).UTC().Truncate(time.Millisecond)
	price := *next.Price
	stale.Price = &price
	stale.Price.Source.ObservedAt = stale.ObservedAt.Format("2006-01-02T15:04:05.000Z")
	if crossed, why, err = h.store.EvaluateRule(ctx, rule.ID, stale, time.Now()); err != nil || crossed || why != "stale_or_unqualified" {
		t.Fatal(crossed, why, err)
	}
	for _, different := range []AlertRule{other, display} {
		if crossed, why, err = h.store.EvaluateRule(ctx, different.ID, next, time.Now()); err != nil || crossed || why != "price_context_mismatch" {
			t.Fatal(different.ID, crossed, why, err)
		}
	}
	h.reopen()
	next, err = h.store.CanonicalPrice(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if crossed, why, err = h.store.EvaluateRule(ctx, rule.ID, next, time.Now()); err != nil || !crossed || why != "crossed" {
		t.Fatal(crossed, why, err)
	}
	if crossed, why, err = h.store.EvaluateRule(ctx, rule.ID, next, time.Now()); err != nil || crossed || why != "replay" {
		t.Fatal(crossed, why, err)
	}
	var count int
	if err = h.store.DB.QueryRow(`SELECT count(*) FROM narrative_discovery.triggers WHERE rule_id=$1`, rule.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func TestRepair_46_alert_evaluator_unwired(t *testing.T) {
	h := repair46Server(t)
	repair46PriceSource(t, h)
	ctx := context.Background()
	baseline := repair46PublishPrice(t, h, 9, 1, time.Now())
	owner := uuid.NewString()
	rule := h.createAlert(owner, *baseline.Price)
	runtime := Runtime{Store: h.store}
	if n, err := runtime.EvaluateBatch(ctx, 1, time.Now()); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	before := repair46Checkpoint(t, h.store, rule.ID)
	if before.MetricRevision != 1 || !before.BaselineSet || !before.Armed {
		t.Fatal(before)
	}
	h.reopen()
	next := repair46PublishPrice(t, h, 11, 2, time.Now())
	runtime = Runtime{Store: h.store}
	run, cancel := context.WithTimeout(ctx, 5*time.Second)
	err := runtime.Run(run)
	cancel()
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("publish failure must propagate after durable evaluation: %v", err)
	}
	checkpoint := repair46Checkpoint(t, h.store, rule.ID)
	if checkpoint.MetricRevision != 2 || checkpoint.Armed || checkpoint.LastTrigger.IsZero() {
		t.Fatal(checkpoint)
	}
	h.reopen()
	runtime = Runtime{Store: h.store}
	if n, err := runtime.EvaluateBatch(ctx, 1, time.Now()); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if repair46Checkpoint(t, h.store, rule.ID) != checkpoint {
		t.Fatal("redelivery changed durable checkpoint")
	}
	rows, err := h.store.DB.Query(`SELECT id,kind,document,published FROM narrative_discovery.outbox WHERE kind='threshold-crossing-v2'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, kind string
		var raw []byte
		var published bool
		if err = rows.Scan(&id, &kind, &raw, &published); err != nil {
			t.Fatal(err)
		}
		var trigger AlertTrigger
		if err = json.Unmarshal(raw, &trigger); err != nil {
			t.Fatal(err)
		}
		if kind != "threshold-crossing-v2" || published || trigger.ID != id || trigger.ID != fmt.Sprintf("%s:1:2", rule.ID) || trigger.Rule.ID != rule.ID || trigger.Metric.Revision != next.Revision || trigger.Rule.Price == nil || !samePriceIdentity(*trigger.Rule.Price, *trigger.Metric.Price) {
			t.Fatalf("outbox consumer: %+v %s %t", trigger, kind, published)
		}
		count++
	}
	if err = rows.Err(); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	rows.Close()
	// The durable scheduler also visits later IDs and skips disabled rules.
	second := rule
	second.ID = uuid.NewString()
	if err = h.store.SaveRule(ctx, owner, second, 0); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = runtime.EvaluateBatch(ctx, 1, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if !repair46Checkpoint(t, h.store, second.ID).BaselineSet {
		t.Fatal("bounded cursor starved a later rule")
	}
	if err = h.store.DisableRule(ctx, owner, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.EvaluateBatch(ctx, 101, time.Now()); !errors.Is(err, ErrInvalid) {
		t.Fatal("unbounded evaluator accepted", err)
	}
}
