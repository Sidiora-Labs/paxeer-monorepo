package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/search"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type repair69Envelope struct {
	Data struct {
		Items      []search.Hit `json:"items"`
		Generation string       `json:"generation"`
		MatchKind  string       `json:"matchKind"`
		Status     string       `json:"status"`
	} `json:"data"`
	Revision string  `json:"revision"`
	Next     *string `json:"nextCursor"`
}

type repair69Path struct {
	t          *testing.T
	ctx        context.Context
	dsn        string
	pool       *pgxpool.Pool
	nc         *nats.Conn
	source     *delivery.Store
	projection *search.Projection
	consumers  map[string]*natsdriver.Consumer
	relay      delivery.Relay
	router     http.Handler
	sequence   int
}

func repair69JSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func repair69Broker(t *testing.T) *nats.Conn {
	t.Helper()
	binary, err := exec.LookPath("nats-server")
	if err != nil {
		t.Fatal("nats-server is required for the real file-backed delivery path:", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	log, err := os.Create(filepath.Join(directory, "nats.log"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "-js", "-a", "127.0.0.1", "-p", strconv.Itoa(port), "-sd", filepath.Join(directory, "store"))
	command.Stdout, command.Stderr = log, log
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait(); _ = log.Close() })
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", port), nats.NoReconnect(), nats.Timeout(100*time.Millisecond))
		if err == nil {
			t.Cleanup(connection.Close)
			return connection
		}
		time.Sleep(20 * time.Millisecond)
	}
	raw, _ := os.ReadFile(log.Name())
	t.Fatalf("real broker did not start: %s", raw)
	return nil
}

func newRepair69Path(t *testing.T) *repair69Path {
	t.Helper()
	ctx := context.Background()
	dsn, pool := internaltest.NewPostgresWithDSN(t)
	p := &repair69Path{t: t, ctx: ctx, dsn: dsn, pool: pool, nc: repair69Broker(t), consumers: map[string]*natsdriver.Consumer{}}
	if _, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm; CREATE SCHEMA stats_search; CREATE SCHEMA indexer; CREATE SCHEMA repair_source; CREATE SCHEMA repair_source_delivery; CREATE SCHEMA repair_search_delivery"); err != nil {
		t.Fatal(err)
	}
	migrationRoot := "../../../../../launchpad/spec/production-system/integration/migrations/"
	schema, err := os.ReadFile(migrationRoot + "200001.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, string(schema)); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	creationDDL, err := os.ReadFile(migrationRoot + "190001.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, strings.Split(string(creationDDL), "CREATE TABLE indexer.indexer_token_transfers")[0]); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO stats_search.generations_v2(generation,state,source_watermark,source_observed_at) VALUES('repair_generation','active',1,clock_timestamp()); INSERT INTO stats_search.active_v2 VALUES(true,'repair_generation',nextval('stats_search.revision_v2'),1); CREATE TABLE repair_source.metadata_v2(event_id text PRIMARY KEY,payload jsonb NOT NULL); CREATE TABLE repair_source.visibility_v2(event_id text PRIMARY KEY,payload jsonb NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"repair_source_delivery", "repair_search_delivery"} {
		ddl, err := delivery.SchemaDDL(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	js, err := jetstream.New(p.nc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = js.CreateStream(ctx, jetstream.StreamConfig{Name: "REPAIR69", Subjects: []string{"repair69.>"}, Storage: jetstream.FileStorage, MaxBytes: 10485760}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"creation", "metadata", "visibility"} {
		config := natsdriver.ConsumerConfig{Stream: "REPAIR69", Durable: kind, Subject: "repair69." + kind, Batch: 1, MaxAckPending: 1, MaxWaiting: 1, MaxDeliver: 3, FetchWait: time.Second, AckWait: time.Second}
		native, err := config.BrokerConsumerConfig()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = js.CreateOrUpdateConsumer(ctx, "REPAIR69", native); err != nil {
			t.Fatal(err)
		}
		p.consumers[kind], err = natsdriver.NewConsumer(ctx, p.nc, config)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(p.consumers[kind].Close)
	}
	publisher, err := natsdriver.NewPublisher(p.nc, "REPAIR69")
	if err != nil {
		t.Fatal(err)
	}
	p.relay = delivery.Relay{Publisher: publisher, Owner: "repair69", Batch: 1, LeaseDuration: 10 * time.Second, PublishTimeout: time.Second, RetryDelay: time.Second, MaxAttempts: 3}
	p.open()
	return p
}

func (p *repair69Path) open() {
	p.t.Helper()
	policy := delivery.Policy{Subjects: map[string]bool{"repair69.creation": true, "repair69.metadata": true, "repair69.visibility": true, "repair69.invalidation": true}, EventTypes: map[string]bool{"creation": true, "metadata": true, "visibility": true, "invalidation": true}}
	var err error
	p.source, err = delivery.NewStore(p.pool, "repair_source_delivery", policy)
	if err != nil {
		p.t.Fatal(err)
	}
	consumer, err := delivery.NewStore(p.pool, "repair_search_delivery", policy)
	if err != nil {
		p.t.Fatal(err)
	}
	p.relay.Store = p.source
	p.projection = &search.Projection{Delivery: consumer, ConsumerID: "repair69", InboxGeneration: "repair69", Sources: map[string]search.SourceRelation{"metadata": {Schema: "repair_source", View: "metadata_v2"}, "visibility": {Schema: "repair_source", View: "visibility_v2"}}, InvalidationSubject: "repair69.invalidation", InvalidationEventType: "invalidation"}
	router := chi.NewRouter()
	httpapi.RegisterSearch(router, store.New(p.pool), nil)
	p.router = router
}

func (p *repair69Path) restart() {
	p.t.Helper()
	p.pool.Close()
	pool, err := pgxpool.New(p.ctx, p.dsn)
	if err != nil {
		p.t.Fatal(err)
	}
	p.pool = pool
	p.t.Cleanup(pool.Close)
	p.open()
}

func (p *repair69Path) deliver(kind string, payload any, effect func(context.Context, pgx.Tx) error) {
	p.t.Helper()
	p.sequence++
	raw := repair69JSON(p.t, payload)
	event := delivery.Envelope{EventID: fmt.Sprintf("repair69_%d", p.sequence), EventType: kind, SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: "1", TransactionIndex: "0", LogIndex: "0"}, EmittedAt: time.Now().UTC(), TraceID: "repair69", Payload: raw, PayloadHash: delivery.Hash(raw)}
	subject := "repair69." + kind
	if err := p.source.Produce(p.ctx, []delivery.Pending{{Subject: subject, Envelope: event}}, func(ctx context.Context, tx pgx.Tx) error {
		if effect != nil {
			return effect(ctx, tx)
		}
		_, err := tx.Exec(ctx, "INSERT INTO "+pgx.Identifier{"repair_source", kind + "_v2"}.Sanitize()+" VALUES($1,$2)", event.EventID, raw)
		return err
	}); err != nil {
		p.t.Fatal(err)
	}
	if err := p.relay.RunOnce(p.ctx); err != nil {
		p.t.Fatal(err)
	}
	messages, err := p.consumers[kind].Fetch(p.ctx)
	if err != nil || len(messages) != 1 {
		p.t.Fatalf("delivery count=%d: %v", len(messages), err)
	}
	message := messages[0]
	if kind == "creation" {
		err = p.projection.ConsumeCanonicalCreation(p.ctx, message.Subject, message.Wire, message)
	} else {
		err = p.projection.Consume(p.ctx, message.Subject, message.Wire, message)
	}
	if err != nil {
		p.t.Fatal(err)
	}
}

func repair69Address(number int) string { return fmt.Sprintf("0x%040x", number) }

func (p *repair69Path) create(number int, token string) string {
	p.t.Helper()
	pool := repair69Address(number)
	fact := search.CanonicalCreation{Pool: pool, Token: token, Creator: repair69Address(999), BlockTimestampSeconds: "1700000000"}
	fact.Position.ChainID, fact.Position.Height, fact.Position.TransactionIndex, fact.Position.LogIndex = "125", "1", "0", "0"
	fact.Position.TransactionHash = fmt.Sprintf("0x%064x", number)
	fact.Position.BlockHash = "0x" + strings.Repeat("1", 64)
	fact.Metadata = &struct {
		Name   string `json:"name"`
		Symbol string `json:"symbol"`
	}{Name: "Canonical Coin", Symbol: "COIN"}
	p.deliver("creation", fact, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations VALUES($1,$2,$3,$4,$5,1,1,$6,$7,0,0,1700000000,'{}',$8)`, token, pool, fact.Creator, repair69Address(901), repair69Address(902), fact.Position.BlockHash, fact.Position.TransactionHash, repair69JSON(p.t, fact))
		return err
	})
	return pool
}

func (p *repair69Path) change(change search.Change) {
	p.t.Helper()
	if err := change.Validate(); err != nil {
		p.t.Fatal(err)
	}
	p.deliver(change.Kind, change, nil)
}

func (p *repair69Path) get(values url.Values, status int) repair69Envelope {
	p.t.Helper()
	response := serve(p.t, p.router, http.MethodGet, "/v2/search?"+values.Encode(), nil, nil)
	if response.Code != status {
		p.t.Fatalf("query %v: status %d, want %d: %s", values, response.Code, status, response.Body.String())
	}
	schema := "SearchResponse"
	if status != 200 {
		schema = "Error"
	}
	if err := v2.ValidateJSON(schema, response.Body.Bytes()); err != nil {
		p.t.Fatalf("frozen %s: %v: %s", schema, err, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Schema-Version") != "2" {
		p.t.Fatal(response.Header())
	}
	var body repair69Envelope
	if status == 200 {
		decode(p.t, response, &body)
	} else if !strings.Contains(response.Body.String(), "CURSOR_INVALID") {
		p.t.Fatal(response.Body.String())
	}
	return body
}

func repair69Query(query, kind string, limit int) url.Values {
	return url.Values{"q": {query}, "type": {kind}, "limit": {strconv.Itoa(limit)}}
}

func TestRepair_69_PublicSearchSourceDeliveryAndRestart(t *testing.T) {
	p := newRepair69Path(t)
	token := repair69Address(100)
	first := p.create(1, token)
	p.create(2, first)
	// The production migration deliberately has no legacy search tables.
	var legacy *string
	if err := p.pool.QueryRow(p.ctx, "SELECT to_regclass('stats_search.generation')::text").Scan(&legacy); err != nil || legacy != nil {
		t.Fatal("legacy source unexpectedly installed", legacy, err)
	}
	before := p.get(repair69Query(first, "pool", 25), 200)
	if len(before.Data.Items) != 0 || before.Data.Status != "not-indexed" {
		t.Fatal("missing visibility must fail closed", before)
	}
	for _, address := range []string{first, repair69Address(2)} {
		p.change(search.Change{Version: 2, Kind: "visibility", Revision: 1, PoolAddress: address, Public: true})
	}
	for _, kind := range []string{"pool", "token"} {
		page := p.get(repair69Query("coin", kind, 25), 200)
		if len(page.Data.Items) != 2 || page.Data.Items[0].Kind != kind || page.Data.Items[0].Label != "Canonical Coin" || *page.Data.Items[0].PoolAddress != first || *page.Data.Items[0].TokenAddress != token {
			t.Fatal(page)
		}
	}
	exact := p.get(repair69Query(first, "pool", 1), 200)
	if len(exact.Data.Items) != 1 || *exact.Data.Items[0].PoolAddress != first || exact.Next == nil {
		t.Fatal("exact pool must precede token associations", exact)
	}
	next := repair69Query(first, "pool", 1)
	next.Set("cursor", *exact.Next)
	for _, replacement := range []struct{ key, value string }{{"q", "coin"}, {"type", "token"}, {"limit", "2"}, {"filter", `{"poolStatus":"active"}`}} {
		rebound := repair69Query(first, "pool", 1)
		rebound.Set("cursor", *exact.Next)
		rebound.Set(replacement.key, replacement.value)
		p.get(rebound, 400)
	}
	if page := p.get(repair69Query("%_", "pool", 25), 200); len(page.Data.Items) != 0 {
		t.Fatal("LIKE metacharacters became wildcards", page)
	}
	if page := p.get(repair69Query("ＣＯＩＮ", "pool", 25), 200); len(page.Data.Items) != 2 {
		t.Fatal("normalization no longer matches projected keys", page)
	}
	if page := p.get(repair69Query("0x000", "pool", 25), 200); len(page.Data.Items) != 2 || page.Data.MatchKind != "partial_address" {
		t.Fatal(page)
	}
	p.restart()
	second := p.get(next, 200)
	if len(second.Data.Items) != 1 || *second.Data.Items[0].PoolAddress != repair69Address(2) || second.Next != nil || second.Revision != exact.Revision {
		t.Fatal("restart lost cursor snapshot", second)
	}
	var document search.IndexedDocument
	var raw []byte
	if err := p.pool.QueryRow(p.ctx, "SELECT payload FROM stats_search.source_documents_v2 WHERE pool_address=$1", first).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	document.Name, document.NameKey, document.MetadataRevision, document.VisibilityRevision = "Updated Coin", "updated coin", 2, 2
	p.change(search.Change{Version: 2, Kind: "metadata", Revision: 2, PoolAddress: first, Document: &document})
	p.get(next, 400)
	// Authority must catch up to the metadata visibility revision before disclosure.
	if page := p.get(repair69Query("updated", "token", 25), 200); len(page.Data.Items) != 0 {
		t.Fatal(page)
	}
	p.change(search.Change{Version: 2, Kind: "visibility", Revision: 2, PoolAddress: first, Public: true})
	page := p.get(repair69Query("updated", "token", 25), 200)
	if len(page.Data.Items) != 1 || page.Data.Items[0].Label != "Updated Coin" || *page.Data.Items[0].TokenAddress != token {
		t.Fatal(page)
	}
	// The expanded and frozen routes now read the same production generation.
	expanded, err := (search.Store{Pool: p.pool}).SearchIntegrated(p.ctx, search.IntegratedRequest{Request: search.Request{Query: "updated", Kind: "pool", Limit: 25}, Sort: "address"})
	if err != nil || len(expanded.Items) != 1 || expanded.Generation != page.Data.Generation || strconv.FormatInt(expanded.Revision, 10) != page.Revision {
		t.Fatal(expanded, err, page)
	}
	observed := time.Now().UTC()
	searchStore := search.Store{Pool: p.pool}
	if err := searchStore.BeginGeneration(p.ctx, "repair_generation_two", 1, observed); err != nil {
		t.Fatal(err)
	}
	tx, err := p.pool.Begin(p.ctx)
	if err != nil {
		t.Fatal(err)
	}
	count, digest, err := search.GenerationDigest(p.ctx, tx, "repair_generation_two")
	_ = tx.Rollback(p.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := searchStore.ActivateGeneration(p.ctx, search.GenerationProof{Generation: "repair_generation_two", SourceWatermark: 1, SourceObservedAt: observed, Rows: count, Digest: digest}); err != nil {
		t.Fatal(err)
	}
	p.get(next, 400)
	if page := p.get(repair69Query("updated", "pool", 25), 200); len(page.Data.Items) != 1 || page.Data.Generation != "repair_generation_two" {
		t.Fatal("generation cutover lost canonical hit", page)
	}
	p.change(search.Change{Version: 2, Kind: "visibility", Revision: 3, PoolAddress: first, Public: false, Tombstone: true})
	p.restart()
	hidden := p.get(repair69Query(token, "token", 25), 200)
	if len(hidden.Data.Items) != 0 || hidden.Data.Status != "not-indexed" {
		t.Fatal("restart resurrected moderated token", hidden)
	}
	p.get(next, 400)
	// An older visibility delivery cannot reverse the current moderation decision.
	p.change(search.Change{Version: 2, Kind: "visibility", Revision: 2, PoolAddress: first, Public: true})
	if page := p.get(repair69Query("updated", "pool", 25), 200); len(page.Data.Items) != 0 {
		t.Fatal("stale authority resurrected hit", page)
	}
}

func TestRepair_69_FrozenQueriesAndCursorBounds(t *testing.T) {
	router := chi.NewRouter()
	httpapi.RegisterCanonicalSearch(router, search.Store{})
	for _, test := range []struct {
		query  url.Values
		status int
		code   string
	}{
		{url.Values{"q": {"coin", "other"}, "type": {"pool"}}, 400, "QUERY_INVALID"},
		{repair69Query("coin", "profile", 25), 412, "PRECONDITION_FAILED"},
		{repair69Query("coin", "wallet", 25), 422, "FIELD_INVALID"},
		{repair69Query("coin", "pool", 26), 422, "FIELD_INVALID"},
		{url.Values{"q": {"coin"}, "type": {"pool"}, "sort": {"address"}}, 400, "QUERY_INVALID"},
		{url.Values{"q": {"coin"}, "type": {"pool"}, "filter": {`{"poolStatus":"active","poolStatus":"paused"}`}}, 400, "QUERY_INVALID"},
		{url.Values{"q": {"coin"}, "type": {"pool"}, "cursor": {strings.Repeat("x", 2049)}}, 413, "PAYLOAD_TOO_LARGE"},
		{url.Values{"q": {"coin"}, "type": {"pool"}, "cursor": {`{"version":1}`}}, 400, "CURSOR_INVALID"},
	} {
		response := serve(t, router, http.MethodGet, "/v2/search?"+test.query.Encode(), nil, nil)
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.code) {
			t.Fatal(test.query, response.Code, response.Body.String())
		}
		if err := v2.ValidateJSON("Error", response.Body.Bytes()); err != nil {
			t.Fatal(err)
		}
	}
}
