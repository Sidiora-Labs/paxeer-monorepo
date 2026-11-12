package cache

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
)

func TestAdapterRejectsChangedDatabaseProjectionAndRecovers(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	client, err := sharedredis.NewBoundedClient(internaltest.NewRedisURL(t), 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, err := db.Exec(ctx, `CREATE SCHEMA cache_projection;
		CREATE TABLE cache_projection.cache_query_budgets_v2(namespace text NOT NULL,origin text NOT NULL,window_bucket bigint NOT NULL,window_ms bigint NOT NULL,budget_limit bigint NOT NULL CHECK(budget_limit>0),used bigint NOT NULL CHECK(used>0 AND used<=budget_limit),PRIMARY KEY(namespace,origin,window_bucket,window_ms));
		CREATE TABLE cache_projection.source_records(id text PRIMARY KEY,generation text NOT NULL,revision text NOT NULL,visibility text NOT NULL,watermark bytea NOT NULL,body bytea NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	policy := RoutePolicy{ID: "market_projection", Version: "v2", Origin: "postgres", Generation: "g1", Class: PublicSnapshot, FreshTTL: time.Minute, RequestLimit: 100, RequestWindow: time.Hour, Parameters: map[string]ParameterRule{"record": {Required: true, MaxBytes: 32}}}
	adapter, err := NewAdapter(AdapterConfig{
		Redis: client, Namespace: "api_projection_integrity", Policies: []RoutePolicy{policy},
		Local:        Config{Capacity: 32, MaxBytes: 1 << 20, MaxEntryBytes: 1 << 16, MaxFetches: 4, MaxPerOrigin: 4, FetchTimeout: time.Second},
		Budget:       PostgresQueryBudget{Pool: db, Schema: "cache_projection", Namespace: "api_projection_integrity", Limit: 100, QueryCost: 1, Window: time.Hour},
		Emergency:    EmergencyAllocation{AllocationID: "api_projection_integrity", ReplicaSlot: 0, ReplicaCount: 1, AggregateLimit: 100, Window: time.Hour},
		RedisTimeout: 200 * time.Millisecond, LeaseTTL: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	valid := Projection{Generation: "g1", Revision: "1", Visibility: "public", SourceWatermark: json.RawMessage(`null`), Body: json.RawMessage(`{"price":"100"}`)}
	fetch := func(ctx context.Context, request ProjectionRequest) (Projection, error) {
		var projection Projection
		err := db.QueryRow(ctx, `SELECT generation,revision,visibility,watermark,body FROM cache_projection.source_records WHERE id=$1`, request.Parameters.Get("record")).Scan(&projection.Generation, &projection.Revision, &projection.Visibility, &projection.SourceWatermark, &projection.Body)
		return projection, err
	}
	scope := VerifiedScope{NetworkID: "127.0.0.1"}
	for _, check := range []struct {
		name   string
		change func(*Projection)
	}{
		{"generation", func(p *Projection) { p.Generation = "g2" }},
		{"visibility", func(p *Projection) { p.Visibility = "private" }},
		{"malformed body", func(p *Projection) { p.Body = json.RawMessage(`{`) }},
		{"null body", func(p *Projection) { p.Body = json.RawMessage(`null`) }},
		{"malformed watermark", func(p *Projection) { p.SourceWatermark = json.RawMessage(`{`) }},
	} {
		t.Run(check.name, func(t *testing.T) {
			p := valid
			check.change(&p)
			if _, err := db.Exec(ctx, `INSERT INTO cache_projection.source_records(id,generation,revision,visibility,watermark,body) VALUES($1,$2,$3,$4,$5,$6)`, check.name, p.Generation, p.Revision, p.Visibility, []byte(p.SourceWatermark), []byte(p.Body)); err != nil {
				t.Fatal(err)
			}
			query := url.Values{"record": {check.name}}
			if result, _, err := adapter.Read(ctx, policy.ID, query, scope, fetch); !errors.Is(err, ErrProjectionChanged) || result.Revision != "" {
				t.Fatalf("changed %s admitted: %+v %v", check.name, result, err)
			}
			if _, err := db.Exec(ctx, `UPDATE cache_projection.source_records SET generation=$2,revision=$3,visibility=$4,watermark=$5,body=$6 WHERE id=$1`, check.name, valid.Generation, valid.Revision, valid.Visibility, []byte(valid.SourceWatermark), []byte(valid.Body)); err != nil {
				t.Fatal(err)
			}
			recovered, stale, err := adapter.Read(ctx, policy.ID, query, scope, fetch)
			if err != nil || stale || recovered.Revision != "1" || string(recovered.Body) != string(valid.Body) {
				t.Fatalf("corrected %s source: %+v stale=%t err=%v", check.name, recovered, stale, err)
			}
		})
	}
}
