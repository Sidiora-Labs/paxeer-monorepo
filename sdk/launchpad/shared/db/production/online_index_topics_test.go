package production

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
)

func topicIndexPosition(position int) *int { return &position }

func topicIndexPlan() OnlineIndexPlan {
	return OnlineIndexPlan{Schema: "stats", Table: "topic_logs", Name: "topic_logs_wallet", Keys: []OnlineIndexKey{
		{Column: "chain_id"},
		{Column: "topics", JSONTextPosition: topicIndexPosition(0)},
		{Column: "topics", JSONTextPosition: topicIndexPosition(2), Descending: true},
		{Column: "height"},
	}}
}

func TestOnlineIndexTopicSQL(t *testing.T) {
	plan := topicIndexPlan()
	sql, err := plan.CreateSQL()
	const want = `CREATE INDEX CONCURRENTLY "topic_logs_wallet" ON "stats"."topic_logs" ("chain_id", ("topics" ->> 0), ("topics" ->> 2) DESC, "height");`
	if err != nil || sql != want {
		t.Fatalf("topic index SQL = %q: %v", sql, err)
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var decoded OnlineIndexPlan
	if err = json.Unmarshal(raw, &decoded); err != nil || !reflect.DeepEqual(plan, decoded) {
		t.Fatalf("topic plan round trip = %+v: %v", decoded, err)
	}
	for _, position := range []int{0, 1, 2, 3} {
		plan.Keys = []OnlineIndexKey{{Column: "topics", JSONTextPosition: topicIndexPosition(position)}}
		if _, err := plan.CreateSQL(); err != nil {
			t.Fatalf("position %d: %v", position, err)
		}
	}
}

func TestOnlineIndexTopicRejectsInvalidKeys(t *testing.T) {
	for name, keys := range map[string][]OnlineIndexKey{
		"negative position":    {{Column: "topics", JSONTextPosition: topicIndexPosition(-1)}},
		"position above three": {{Column: "topics", JSONTextPosition: topicIndexPosition(4)}},
		"duplicate position":   {{Column: "topics", JSONTextPosition: topicIndexPosition(0)}, {Column: "topics", JSONTextPosition: topicIndexPosition(0), Descending: true}},
		"numeric expression":   {{Column: "topics", JSONTextPosition: topicIndexPosition(0), Numeric: true}},
	} {
		t.Run(name, func(t *testing.T) {
			plan := topicIndexPlan()
			plan.Keys = keys
			if _, err := plan.CreateSQL(); err == nil {
				t.Fatal("invalid topic key admitted")
			}
		})
	}
}

func TestOnlineIndexTopicCatalog(t *testing.T) {
	pool, dsn := phase2MigrationFixture(t)
	ctx := context.Background()
	exec(t, pool, `SET ROLE stats_owner; CREATE TABLE stats.topic_logs(chain_id integer,topics jsonb,height bigint); INSERT INTO stats.topic_logs VALUES(125,'["transfer","pool","wallet","token"]',1); RESET ROLE`)
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	plan := topicIndexPlan()
	sql, err := plan.CreateSQL()
	if err != nil {
		t.Fatal(err)
	}
	m := phase2RegisteredMigration(70001, sql, nil)
	m.TransactionMode, m.OnlineIndex = "nontransactional", &plan
	registry := map[int64]MigrationManifest{m.MigrationID: m}
	if err = ApplyOnlineIndexMigration(ctx, conn, m, plan, registry, m.InterfaceDigest, m.ApprovalManifestDigest, 1, nil); err != nil {
		t.Fatal(err)
	}
	state, err := InspectOnlineIndex(ctx, conn, plan)
	if err != nil || !state.Valid || !state.Ready || !state.matches(plan) {
		t.Fatalf("topic index catalog = %+v: %v", state, err)
	}
	other := topicIndexPlan()
	other.Keys[2].JSONTextPosition = topicIndexPosition(3)
	if state.matches(other) {
		t.Fatal("different topic position matched catalog")
	}
	otherState, err := InspectOnlineIndex(ctx, conn, other)
	if err != nil || !otherState.Exists || otherState.DefinitionMatches || otherState.matches(other) {
		t.Fatalf("different topic definition = %+v: %v", otherState, err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM stats.topic_logs`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("raw rows = %d: %v", count, err)
	}
}
