package storage

import (
	"context"
	"github.com/jackc/pgx/v5"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRuntimeRoleMediaAuthority(t *testing.T) {
	const coordinatorDSNEnv = "COORDINATOR_TEST_POSTGRES_DSN"
	admin := os.Getenv(coordinatorDSNEnv)
	if admin == "" {
		t.Skipf("%s is not set", coordinatorDSNEnv)
	}
	database := coordinatortest.Bootstrap(t, admin, "identity_runtime")
	runtime := database.Runtimes["identity_runtime"]
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	authority, err := NewConnectedAuthority(pool, "gateway", "metadata", "chat")
	if err != nil {
		t.Fatal(err)
	}

	adminPool, err := pgxpool.New(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(adminPool.Close)
	firstHash := strings.Repeat("a", 64)
	if _, err = adminPool.Exec(ctx, `INSERT INTO metadata.pools(pool,creator,token) VALUES('media-pool','media-creator','media-token')`); err != nil {
		t.Fatal(err)
	}
	if _, err = adminPool.Exec(ctx, `INSERT INTO metadata.assets(id,pool,state,hash,size,object_key,scanner_engine,scanner_revision) VALUES('media-asset','media-pool','APPROVED',$1,1,'approved/'||$1,'scanner','1')`, firstHash); err != nil {
		t.Fatal(err)
	}
	if err = authority.RegisterLineage(ctx, LineageObject{Hash: firstHash, Key: "approved/" + firstHash + "/256/" + firstHash + ".png", Kind: variantKind, RetainUntil: time.Now().Add(time.Hour)}); err != nil {
		t.Fatalf("initial lineage visibility: %v", err)
	}
	var initialRevision int64
	var initialState string
	var initialVisibility bool
	if err = pool.QueryRow(ctx, `SELECT revision,state,initial_visibility FROM gateway.restrictions WHERE hash=$1`, firstHash).Scan(&initialRevision, &initialState, &initialVisibility); err != nil || initialRevision != 1 || initialState != approvedState || !initialVisibility {
		t.Fatalf("initial visibility = %d, %q, %v: %v", initialRevision, initialState, initialVisibility, err)
	}
	if _, err = authority.Restrict(ctx, firstHash, 1, blockedState); err != nil {
		t.Fatalf("first moderation revision: %v", err)
	}
	if _, err = authority.Restrict(ctx, firstHash, 2, blockedState); err != nil {
		t.Fatalf("later restriction revision: %v", err)
	}
	var revision int64
	var state string
	var initial bool
	if err = pool.QueryRow(ctx, `SELECT revision,state,initial_visibility FROM gateway.restrictions WHERE hash=$1`, firstHash).Scan(&revision, &state, &initial); err != nil {
		t.Fatal(err)
	}
	if revision != 2 || state != blockedState || initial {
		t.Fatalf("restriction revision = %d, %q, initial=%v", revision, state, initial)
	}

	tombstoneHash := strings.Repeat("b", 64)
	if _, err = pool.Exec(ctx, `INSERT INTO gateway.tombstones(hash,revision) VALUES($1,3)`, tombstoneHash); err != nil {
		t.Fatal(err)
	}
	if err = ReplayTombstones(ctx, postgresTombstones{authority: authority}); err != nil {
		t.Fatalf("tombstone replay: %v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT revision,state,initial_visibility FROM gateway.restrictions WHERE hash=$1`, tombstoneHash).Scan(&revision, &state, &initial); err != nil {
		t.Fatal(err)
	}
	if revision != 3 || state != deletedState || initial {
		t.Fatalf("replayed restriction = %d, %q, initial=%v", revision, state, initial)
	}
	var operations int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM gateway.operations WHERE hash=$1`, tombstoneHash).Scan(&operations); err != nil || operations != 2 {
		t.Fatalf("replayed operations = %d, %v", operations, err)
	}
	if err = ReplayTombstones(ctx, postgresTombstones{authority: authority}); err != nil {
		t.Fatalf("second tombstone replay: %v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM gateway.operations WHERE hash=$1`, tombstoneHash).Scan(&operations); err != nil || operations != 2 {
		t.Fatalf("second replay operations = %d, %v", operations, err)
	}

	inventory := postgresInventory{authority: authority}
	pass, err := inventory.BeginInventory(ctx, "approved-bucket", "approved/")
	if err != nil {
		t.Fatalf("begin inventory: %v", err)
	}
	token, err := pass.Cursor(ctx)
	if err != nil || token != "" {
		t.Fatalf("empty inventory cursor = %q, %v", token, err)
	}
	if err = pass.Advance(ctx, "cursor-1"); err != nil {
		t.Fatal(err)
	}
	if err = pass.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	pass, err = inventory.BeginInventory(ctx, "approved-bucket", "approved/")
	if err != nil {
		t.Fatalf("read inventory cursor: %v", err)
	}
	token, err = pass.Cursor(ctx)
	if err != nil || token != "cursor-1" {
		t.Fatalf("stored inventory cursor = %q, %v", token, err)
	}
	if err = pass.Advance(ctx, "cursor-2"); err != nil {
		t.Fatal(err)
	}
	if err = pass.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	pass, err = inventory.BeginInventory(ctx, "approved-bucket", "approved/")
	if err != nil {
		t.Fatal(err)
	}
	token, err = pass.Cursor(ctx)
	if err != nil || token != "cursor-2" {
		t.Fatalf("updated inventory cursor = %q, %v", token, err)
	}
	if err = pass.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	var dataType, defaultValue, nullable string
	if err = pool.QueryRow(ctx, `SELECT data_type,COALESCE(column_default,''),is_nullable FROM information_schema.columns WHERE table_schema='gateway' AND table_name='restrictions' AND column_name='initial_visibility'`).Scan(&dataType, &defaultValue, &nullable); err != nil {
		t.Fatal(err)
	}
	if dataType != "boolean" || !strings.Contains(defaultValue, "false") || nullable != "NO" {
		t.Fatalf("initial_visibility catalog = %q, %q, %q", dataType, defaultValue, nullable)
	}
	var checkDefinition string
	if err = pool.QueryRow(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='gateway.restrictions'::regclass AND contype='c' AND pg_get_constraintdef(oid) LIKE '%initial_visibility%'`).Scan(&checkDefinition); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(checkDefinition, "revision") || !strings.Contains(checkDefinition, "approved") {
		t.Fatalf("initial_visibility check = %q", checkDefinition)
	}
	var primaryColumns int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE i.indrelid='gateway.inventory_cursors'::regclass AND i.indisprimary AND c.relname LIKE 'inventory_cursors_pkey%'`).Scan(&primaryColumns); err != nil || primaryColumns != 1 {
		t.Fatalf("inventory cursor primary key = %d, %v", primaryColumns, err)
	}
	assertRuntimeRoleMediaAuthorityCatalog(t, ctx, adminPool)

}

func assertRuntimeRoleMediaAuthorityCatalog(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	const helperSchema = "gateway_helper"
	if _, err = tx.Exec(ctx, `CREATE SCHEMA gateway_helper`); err != nil {
		t.Fatal(err)
	}
	authority, err := NewAuthority(pool, helperSchema, "metadata", "chat")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, authority.SchemaDDL()+authority.InventoryCursorSchemaDDL()); err != nil {
		t.Fatal(err)
	}
	columns := `SELECT jsonb_agg(jsonb_build_array(a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,pg_get_expr(d.adbin,d.adrelid)) ORDER BY a.attname)::text FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE a.attrelid=$1::regclass AND a.attnum>0 AND NOT a.attisdropped`
	constraints := `SELECT jsonb_agg(jsonb_build_array(contype,pg_get_constraintdef(oid)) ORDER BY contype,pg_get_constraintdef(oid))::text FROM pg_constraint WHERE conrelid=$1::regclass AND contype IN ('c','p','u')`
	for _, table := range []string{"restrictions", "inventory_cursors"} {
		for _, query := range []string{columns, constraints} {
			var registered, helper string
			if err = tx.QueryRow(ctx, query, "gateway."+table).Scan(&registered); err != nil {
				t.Fatal(err)
			}
			if err = tx.QueryRow(ctx, query, helperSchema+"."+table).Scan(&helper); err != nil {
				t.Fatal(err)
			}
			if registered != helper {
				t.Fatalf("%s catalog differs: registered=%s helper=%s", table, registered, helper)
			}
		}
	}
	const baselineSchema = "gateway_baseline"
	if _, err = tx.Exec(ctx, `CREATE SCHEMA gateway_baseline`); err != nil {
		t.Fatal(err)
	}
	root := coordinatortest.RepositoryRoot(t)
	apply := func(name string) {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		sql := strings.ReplaceAll(string(raw), `"gateway"`, pgx.Identifier{baselineSchema}.Sanitize())
		sql = strings.ReplaceAll(sql, "gateway.", pgx.Identifier{baselineSchema}.Sanitize()+".")
		if _, err = tx.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	apply("410000.sql")
	if _, err = tx.Exec(ctx, `INSERT INTO gateway_baseline.restrictions(hash,revision,state) VALUES(repeat('a',64),1,'approved'),(repeat('b',64),2,'blocked'),(repeat('c',64),3,'deleted')`); err != nil {
		t.Fatal(err)
	}
	apply("410001.sql")
	var unchanged bool
	if err = tx.QueryRow(ctx, `SELECT count(*)=3 AND bool_and(NOT initial_visibility AND ((hash=repeat('a',64) AND revision=1 AND state='approved') OR (hash=repeat('b',64) AND revision=2 AND state='blocked') OR (hash=repeat('c',64) AND revision=3 AND state='deleted'))) FROM gateway_baseline.restrictions`).Scan(&unchanged); err != nil || !unchanged {
		t.Fatalf("existing restriction semantics preserved=%v: %v", unchanged, err)
	}
}
