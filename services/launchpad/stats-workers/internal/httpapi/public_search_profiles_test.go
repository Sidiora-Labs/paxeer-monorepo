package httpapi_test

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/search"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go/jetstream"
)

func (p *publicSearchPath) accountViews() {
	p.t.Helper()
	tx, err := p.pool.Begin(p.ctx)
	if err != nil {
		p.t.Fatal(err)
	}
	defer tx.Rollback(p.ctx)
	if _, err = tx.Exec(p.ctx, "CREATE SCHEMA users"); err != nil {
		p.t.Fatal(err)
	}
	for _, name := range []string{"330000.sql", "350000.sql", "350001.sql", "350002.sql"} {
		raw, err := os.ReadFile("../../../../../migrations/" + name)
		if err != nil {
			p.t.Fatal(err)
		}
		if _, err = tx.Exec(p.ctx, string(raw)); err != nil {
			p.t.Fatal(name, err)
		}
	}
	if err = tx.Commit(p.ctx); err != nil {
		p.t.Fatal(err)
	}
}

type publicSearchAccount struct {
	id     uuid.UUID
	wallet string
}

func (p *publicSearchPath) account(number int, label, visibility string) publicSearchAccount {
	p.t.Helper()
	a := publicSearchAccount{id: uuid.New(), wallet: publicSearchAddress(number)}
	tx, err := p.pool.Begin(p.ctx)
	if err != nil {
		p.t.Fatal(err)
	}
	defer tx.Rollback(p.ctx)
	if _, err = tx.Exec(p.ctx, `INSERT INTO users.identity_accounts(account_id,created_at) VALUES($1,clock_timestamp())`, a.id); err != nil {
		p.t.Fatal(err)
	}
	if _, err = tx.Exec(p.ctx, `INSERT INTO users.identity_wallet_links(link_id,account_id,chain_id,wallet,visibility,state,linked_at) VALUES($1,$2,125,$3,$4,'verified',clock_timestamp())`, uuid.New(), a.id, a.wallet, visibility); err != nil {
		p.t.Fatal(err)
	}
	profile := map[string]string{"wallet": a.wallet, "bio": "Account biography"}
	if label != "" {
		profile["displayName"] = label
	}
	if _, err = tx.Exec(p.ctx, `INSERT INTO users.account_state(account_id,wallet,revision,profile,preferences) VALUES($1,$2,1,$3,'{}')`, a.id, a.wallet, publicSearchJSON(p.t, profile)); err != nil {
		p.t.Fatal(err)
	}
	if err = tx.Commit(p.ctx); err != nil {
		p.t.Fatal(err)
	}
	return a
}

func (p *publicSearchPath) accountEvent(a publicSearchAccount, kind string, revision int64, deleted bool) *natsdriver.Message {
	p.t.Helper()
	raw := publicSearchJSON(p.t, map[string]string{"accountId": a.id.String()})
	event := delivery.Envelope{EventID: uuid.NewString(), EventType: kind, SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: strconv.FormatInt(revision, 10), TransactionIndex: "0", LogIndex: "0"}, EmittedAt: time.Now().UTC(), TraceID: "profile", Payload: raw, PayloadHash: delivery.Hash(raw)}
	return p.deliverEnvelope(kind, event, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE users.account_state SET revision=$2,deleted=$3,updated_at=clock_timestamp() WHERE account_id=$1`, a.id, revision, deleted); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO users.account_outbox(event_id,account_id,revision,kind,payload) VALUES($1,$2,$3,$4,$5)`, event.EventID, a.id, revision, kind, raw)
		return err
	})
}

func (p *publicSearchPath) replayAccountProfile(original *natsdriver.Message) {
	p.t.Helper()
	js, err := jetstream.New(p.nc)
	if err != nil {
		p.t.Fatal(err)
	}
	config := natsdriver.ConsumerConfig{Stream: original.Stream, Durable: "profile_replay_" + uuid.NewString(), Subject: original.Subject, Batch: 1, MaxAckPending: 1, MaxWaiting: 1, MaxDeliver: 3, FetchWait: time.Second, AckWait: time.Second}
	native, err := config.BrokerConsumerConfig()
	if err != nil {
		p.t.Fatal(err)
	}
	if _, err = js.CreateOrUpdateConsumer(p.ctx, original.Stream, native); err != nil {
		p.t.Fatal(err)
	}
	consumer, err := natsdriver.NewConsumer(p.ctx, p.nc, config)
	if err != nil {
		p.t.Fatal(err)
	}
	defer consumer.Close()
	messages, err := consumer.Fetch(p.ctx)
	if err != nil || len(messages) != 1 {
		p.t.Fatalf("profile replay count=%d: %v", len(messages), err)
	}
	message := messages[0]
	if message.EventID != original.EventID || !bytes.Equal(message.Wire, original.Wire) {
		p.t.Fatal("profile replay identity", message.EventID, original.EventID)
	}
	if err := p.projection.ConsumeAccountProfile(p.ctx, message.Subject, message.Wire, message); err != nil {
		p.t.Fatal(err)
	}
}

func assertPublicProfile(t *testing.T, page publicSearchEnvelope, a publicSearchAccount, label, match string) {
	t.Helper()
	if len(page.Data.Items) != 1 || page.Data.Status != "ok" || page.Data.MatchKind != match || page.Data.Generation == "" || page.Revision == "" {
		t.Fatalf("profile page: %+v", page)
	}
	hit := page.Data.Items[0]
	if hit.Kind != "profile" || hit.ID != "125:profile:"+a.wallet || hit.Wallet == nil || *hit.Wallet != a.wallet || hit.Label != label || hit.PoolAddress != nil || hit.TokenAddress != nil || hit.Score != "1" {
		t.Fatalf("profile hit: %+v", hit)
	}
}

func TestPublicSearchServesProfilesFromAccountEvents(t *testing.T) {
	p := newPublicSearchPath(t)
	p.accountViews()
	first := p.account(501, "Alice Orchard", "public_optin")
	second := p.account(502, "Alice Birch", "public_optin")
	private := p.account(503, "Alice Private", "private")
	unnamed := p.account(504, "", "public_optin")
	message := p.accountEvent(first, "profile.updated", 1, false)
	p.accountEvent(second, "profile.updated", 1, false)
	p.accountEvent(private, "profile.updated", 1, false)
	p.accountEvent(unnamed, "profile.updated", 1, false)
	for _, tc := range []struct{ query, match string }{{"orch", "text"}, {first.wallet, "exact_address"}} {
		page := p.get(publicSearchQuery(tc.query, "profile", 25), 200)
		assertPublicProfile(t, page, first, "Alice Orchard", tc.match)
	}
	before := p.get(publicSearchQuery("alice", "profile", 25), 200)
	if len(before.Data.Items) != 2 {
		t.Fatal("public named profiles", before)
	}
	var invalidations int
	if err := p.pool.QueryRow(p.ctx, `SELECT count(*) FROM repair_search_delivery.delivery_outbox`).Scan(&invalidations); err != nil || invalidations != 4 {
		t.Fatal("profile invalidations", invalidations, err)
	}
	p.replayAccountProfile(message)
	after := p.get(publicSearchQuery("alice", "profile", 25), 200)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("duplicate changed search page", before, after)
	}
	if err := p.pool.QueryRow(p.ctx, `SELECT count(*) FROM repair_search_delivery.delivery_outbox`).Scan(&invalidations); err != nil || invalidations != 4 {
		t.Fatal("duplicate changed invalidations", invalidations, err)
	}
	for _, tc := range []struct{ query, status string }{{private.wallet, "not-indexed"}, {unnamed.wallet, "not-indexed"}, {"nobody", "ok"}, {publicSearchAddress(505), "not-indexed"}} {
		page := p.get(publicSearchQuery(tc.query, "profile", 25), 200)
		if page.Data.Items == nil || len(page.Data.Items) != 0 || page.Next != nil || page.Data.Status != tc.status {
			t.Fatal("empty profile page", tc.query, page)
		}
	}
	page := p.get(publicSearchQuery("alice", "profile", 1), 200)
	assertPublicProfile(t, page, first, "Alice Orchard", "text")
	if page.Next == nil {
		t.Fatal("missing profile cursor", page)
	}
	next := publicSearchQuery("alice", "profile", 1)
	next.Set("cursor", *page.Next)
	for _, replacement := range []struct{ key, value string }{{"q", "orch"}, {"type", "pool"}, {"type", "token"}, {"limit", "2"}} {
		rebound := publicSearchQuery("alice", "profile", 1)
		rebound.Set("cursor", *page.Next)
		rebound.Set(replacement.key, replacement.value)
		p.get(rebound, 400)
	}
	p.restart()
	last := p.get(next, 200)
	assertPublicProfile(t, last, second, "Alice Birch", "text")
	if last.Next != nil || last.Revision != page.Revision {
		t.Fatal("profile cursor snapshot", page, last)
	}
	p.accountEvent(first, "account.deletion.requested", 2, true)
	p.get(next, 400)
	for _, query := range []string{"orch", first.wallet} {
		if deleted := p.get(publicSearchQuery(query, "profile", 25), 200); len(deleted.Data.Items) != 0 {
			t.Fatal("deleted profile found", deleted)
		}
	}
	p.replayAccountProfile(message)
	p.restart()
	if deleted := p.get(publicSearchQuery(first.wallet, "profile", 25), 200); len(deleted.Data.Items) != 0 {
		t.Fatal("deleted profile restored", deleted)
	}
}

func TestPublicProfileSearchRejectsInapplicableFields(t *testing.T) {
	router := chi.NewRouter()
	httpapi.RegisterCanonicalSearch(router, search.Store{})
	for _, filter := range []string{
		`{"poolStatus":"active"}`, `{"creator":"` + publicSearchAddress(501) + `"}`,
		`{"creationAge":{"minInclusiveSeconds":1}}`, `{"verifiedQuoteAsset":true}`,
		`{"volumeWindow":{"window":"24h","range":{"minInclusive":"1"}}}`,
		`{"liquidityRange":{"asset":"USDL","decimals":6,"depth":"total","range":{"minInclusive":"1"}}}`,
		`{"riskDisclosureFlags":["unverified"]}`, `{"version":2}`,
	} {
		values := publicSearchQuery("alice", "profile", 25)
		values.Set("filter", filter)
		assertPublicProfileFieldError(t, router, values)
	}
	for _, sort := range []string{"liquidity", "createdAt", "address"} {
		values := publicSearchQuery("alice", "profile", 25)
		values.Set("sort", sort)
		assertPublicProfileFieldError(t, router, values)
	}
}

func assertPublicProfileFieldError(t *testing.T, router http.Handler, values url.Values) {
	t.Helper()
	response := serve(t, router, http.MethodGet, "/v2/search?"+values.Encode(), nil, nil)
	body, err := v2.DecodeError(response.Body.Bytes())
	if response.Code != 422 || err != nil || body.Code != "FIELD_INVALID" {
		t.Fatal(values, response.Code, response.Body.String(), err)
	}
}

func TestPublicProfileSearchRejectsPoolCursor(t *testing.T) {
	p := newPublicSearchPath(t)
	p.create(1, publicSearchAddress(100))
	p.create(2, publicSearchAddress(101))
	page := p.get(publicSearchQuery("coin", "pool", 1), 200)
	if page.Next == nil {
		t.Fatal("missing pool cursor", page)
	}
	values := publicSearchQuery("coin", "profile", 1)
	values.Set("cursor", *page.Next)
	p.get(values, 400)
}
