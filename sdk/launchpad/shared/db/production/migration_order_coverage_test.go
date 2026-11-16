package production

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestMigrationRegistryHonorsPredecessorOrder(t *testing.T) {
	first := migration([]byte("CREATE TABLE stats.first_entry(id bigint)"))
	first.MigrationID = 70001
	first.Path = "first.sql"
	first.PredecessorIDs = nil
	second := migration([]byte("CREATE TABLE stats.second_entry(id bigint)"))
	second.MigrationID = 70002
	second.Path = "second.sql"
	second.PredecessorIDs = []int64{first.MigrationID}
	base := MigrationRegistry{SchemaVersion: 2, ApprovalDigest: Checksum([]byte("approved registry")), ApplicationOrder: []int64{first.MigrationID, second.MigrationID}, Registrations: []MigrationManifest{first, second}}
	catalog, err := base.ValidateOrder()
	if err != nil || len(catalog) != 2 || catalog[first.MigrationID].Path != first.Path || catalog[second.MigrationID].Path != second.Path {
		t.Fatalf("ordered migration catalog = %+v, %v", catalog, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*MigrationRegistry)
	}{
		{"schema", func(r *MigrationRegistry) { r.SchemaVersion = 1 }},
		{"approval", func(r *MigrationRegistry) { r.ApprovalDigest = "other" }},
		{"missing order", func(r *MigrationRegistry) { r.ApplicationOrder = nil }},
		{"duplicate registration", func(r *MigrationRegistry) { r.Registrations[1] = first }},
		{"unknown order", func(r *MigrationRegistry) { r.ApplicationOrder[1] = 70003 }},
		{"repeated order", func(r *MigrationRegistry) { r.ApplicationOrder[1] = first.MigrationID }},
		{"predecessor after child", func(r *MigrationRegistry) {
			r.ApplicationOrder[0], r.ApplicationOrder[1] = r.ApplicationOrder[1], r.ApplicationOrder[0]
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := base
			candidate.ApplicationOrder = append([]int64(nil), base.ApplicationOrder...)
			candidate.Registrations = append([]MigrationManifest(nil), base.Registrations...)
			tc.change(&candidate)
			if catalog, err := candidate.ValidateOrder(); err == nil || catalog != nil {
				t.Fatalf("invalid migration order admitted: %+v, %v", catalog, err)
			}
		})
	}
	if err := ApplyRegisteredMigration(context.Background(), nil, first, []byte("SELECT 1"), catalog, first.InterfaceDigest, first.ApprovalManifestDigest, 1); err == nil {
		t.Fatal("migration without direct coordinator connection admitted")
	}
}

func TestMigrationRegistryHonorsPredecessorOrderOnlineIndexKeys(t *testing.T) {
	plan := OnlineIndexPlan{Schema: "stats", Table: "pool_holders", Name: "idx_ph_balance_numeric", Keys: []OnlineIndexKey{{Column: "pool_address"}, {Column: "balance", Numeric: true, Descending: true}, {Column: "holder_address"}}}
	sql, err := plan.CreateSQL()
	if err != nil || sql != `CREATE INDEX CONCURRENTLY "idx_ph_balance_numeric" ON "stats"."pool_holders" ("pool_address", ("balance"::numeric) DESC, "holder_address");` {
		t.Fatalf("numeric holder index SQL = %q: %v", sql, err)
	}
	m := migration([]byte(sql))
	m.Path = "migrations/70001.sql"
	m.PredecessorIDs = nil
	m.TransactionMode = "nontransactional"
	m.OnlineIndex = &plan
	registry := map[int64]MigrationManifest{m.MigrationID: m}
	if err := m.Validate([]byte(sql), registry, map[int64]string{}, m.InterfaceDigest, m.ApprovalManifestDigest, 1); err != nil {
		t.Fatal(err)
	}
	changed := m
	changedPlan := plan
	changedPlan.Keys = append([]OnlineIndexKey(nil), plan.Keys...)
	changedPlan.Keys[1].Descending = false
	changed.OnlineIndex = &changedPlan
	if err := changed.Validate([]byte(sql), registry, map[int64]string{}, m.InterfaceDigest, m.ApprovalManifestDigest, 1); err == nil {
		t.Fatal("unapproved index order admitted")
	}
	registry[m.MigrationID] = changed
	if err := changed.Validate([]byte(sql), registry, map[int64]string{}, m.InterfaceDigest, m.ApprovalManifestDigest, 1); err == nil {
		t.Fatal("registered plan differing from SQL admitted")
	}
	for _, invalid := range []OnlineIndexPlan{
		{Schema: "stats", Table: "pool_holders", Name: "idx", Keys: []OnlineIndexKey{{Column: "balance::numeric"}}},
		{Schema: "stats", Table: "pool_holders", Name: "idx", Keys: []OnlineIndexKey{{Column: "balance"}, {Column: "balance"}}},
		{Schema: "stats", Table: "pool_holders", Name: "idx", Columns: []string{"balance"}, Keys: []OnlineIndexKey{{Column: "balance", Numeric: true}}},
	} {
		if _, err := invalid.CreateSQL(); err == nil {
			t.Fatal("ambiguous or unbounded index key admitted")
		}
	}
}

func TestMigrationRegistryHonorsPredecessorOrderOnlineIndexOnPopulatedTable(t *testing.T) {
	pool, dsn := phase2MigrationFixture(t)
	ctx := context.Background()
	exec(t, pool, `SET ROLE stats_owner; CREATE TABLE stats.pool_holders(pool_address varchar(42),balance text,holder_address varchar(42)); INSERT INTO stats.pool_holders VALUES('pool','9','b'),('pool','100','a'); RESET ROLE`)
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	plan := OnlineIndexPlan{Schema: "stats", Table: "pool_holders", Name: "idx_ph_balance_numeric", Keys: []OnlineIndexKey{{Column: "pool_address"}, {Column: "balance", Numeric: true, Descending: true}, {Column: "holder_address"}}}
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
	if err != nil || !state.matches(plan) || !state.Valid || !state.Ready {
		t.Fatalf("populated numeric index: %+v: %v", state, err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM stats.pool_holders`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("holder rows after concurrent index: %d: %v", count, err)
	}
}
