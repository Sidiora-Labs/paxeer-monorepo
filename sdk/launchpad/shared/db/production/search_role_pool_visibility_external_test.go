package production_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSearchRolePoolVisibilityDecisions(t *testing.T) {
	assertPoolVisibilityDecisions(t, "stats_search_runtime")
}

func TestStatsRolePoolVisibilityDecisions(t *testing.T) {
	assertPoolVisibilityDecisions(t, "stats_runtime")
}

func assertPoolVisibilityDecisions(t *testing.T, runtimeRole string) {
	t.Helper()
	database := coordinatortest.Bootstrap(t, os.Getenv("COORDINATOR_TEST_POSTGRES_DSN"), runtimeRole)
	if _, ok := database.Applied[380003]; !ok {
		t.Fatal("pool visibility migration was not applied")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	runtime := database.Runtimes[runtimeRole]
	reader, err := pgx.Connect(ctx, runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close(context.Background())
	execute := func(sql string, args ...any) {
		t.Helper()
		if _, err := admin.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	pool := "0x00000000000000000000000000000000000000Ab"
	token := "0x00000000000000000000000000000000000000Cd"
	execute(`INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload)
 VALUES($1,$2,'creator','factory','optical',1,1,'block','transaction',0,0,1,'{}','{}')`, token, pool)
	assertVisibility := func(public bool, revision int64) {
		t.Helper()
		rows, err := reader.Query(ctx, `SELECT pool,token,public,revision FROM chat.pool_search_visibility_v2`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			count++
			var gotPool, gotToken string
			var gotPublic bool
			var gotRevision int64
			if err := rows.Scan(&gotPool, &gotToken, &gotPublic, &gotRevision); err != nil {
				t.Fatal(err)
			}
			if gotPool != pool || gotToken != token || gotPublic != public || gotRevision != revision {
				t.Fatalf("pool visibility = (%s,%s,%v,%d), want (%s,%s,%v,%d)", gotPool, gotToken, gotPublic, gotRevision, pool, token, public, revision)
			}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("canonical visibility rows = %d, want one", count)
		}
	}
	assertVisibility(true, 0)
	execute(`INSERT INTO chat.safety_cases(id,target_type,target_id,reporter,category,description)
 VALUES('00000000-0000-0000-0000-000000000001','token',$1,'reporter','spam','visibility')`, token)
	execute(`INSERT INTO chat.safety_visibility(target_type,target_id,revision,status,case_id)
 VALUES('token',$1,2,'removed','00000000-0000-0000-0000-000000000001')`, token)
	assertVisibility(false, 2)
	execute(`INSERT INTO chat.safety_visibility(target_type,target_id,revision,status,case_id)
 VALUES('pool',$1,3,'restored','00000000-0000-0000-0000-000000000001')`, pool)
	assertVisibility(false, 5)
	execute(`UPDATE chat.safety_visibility SET status='restored',revision=4 WHERE target_type='token' AND target_id=$1`, token)
	assertVisibility(true, 7)
	execute(`UPDATE chat.safety_visibility SET status='removed',revision=4 WHERE target_type='pool' AND target_id=$1`, pool)
	assertVisibility(false, 8)
	var columns []string
	if err := admin.QueryRow(ctx, `SELECT array_agg(column_name::text ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema='chat' AND table_name='pool_search_visibility_v2'`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if len(columns) != 4 || columns[0] != "pool" || columns[1] != "token" || columns[2] != "public" || columns[3] != "revision" {
		t.Fatalf("view columns: %v", columns)
	}
	var publicGrant bool
	if err := admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL aclexplode(c.relacl) a WHERE n.nspname='chat' AND c.relname='pool_search_visibility_v2' AND a.grantee=0)`).Scan(&publicGrant); err != nil || publicGrant {
		t.Fatalf("PUBLIC view privileges = %v: %v", publicGrant, err)
	}
	denied := []string{
		`SELECT * FROM chat.social_canonical_pools()`,
		`SELECT * FROM chat.safety_visibility`,
		`SELECT * FROM chat.safety_cases`,
		`SELECT * FROM chat.safety_outbox`,
	}
	if runtimeRole == "stats_search_runtime" {
		denied = append(denied, `SELECT pool,token FROM indexer.indexer_canonical_creations_v1`)
	}
	for _, sql := range denied {
		_, err := reader.Exec(ctx, sql)
		var refused *pgconn.PgError
		if !errors.As(err, &refused) || refused.Code != "42501" {
			t.Fatalf("%s must be refused %s: %v", runtimeRole, sql, err)
		}
	}
	var canWrite bool
	if err := reader.QueryRow(ctx, `SELECT has_table_privilege(current_user,'chat.pool_search_visibility_v2','INSERT,UPDATE,DELETE')`).Scan(&canWrite); err != nil || canWrite {
		t.Fatalf("%s view write privileges = %v: %v", runtimeRole, canWrite, err)
	}
	expected, err := production.LoadExpectedSchema(runtime.ExpectedSchema, runtime.ExpectedSchemaSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := expected.Schemas["chat"]; !ok {
		t.Fatalf("%s readiness does not include chat", runtimeRole)
	}
}
