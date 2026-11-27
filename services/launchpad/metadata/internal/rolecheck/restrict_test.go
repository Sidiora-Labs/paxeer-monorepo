package rolecheck

import (
	"fmt"
	"go/ast"
	"reflect"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestRuntimeRoleStatementsRestrictSQL(t *testing.T) {
	const path = "services/launchpad/metadata/internal/metadata/production.go"
	source := sourceReader{root: coordinatortest.RepositoryRoot(t), files: map[string]*ast.File{}}
	wantSQL := []string{
		`SELECT COALESCE(d.moderation_revision >= $2::numeric,false) FROM "metadata"."pools" p LEFT JOIN "metadata"."documents" d ON d.pool=p.pool WHERE p.pool=$1`,
		`SELECT creator FROM "metadata"."pools" WHERE pool=$1 FOR UPDATE`,
		`INSERT INTO "metadata"."documents"(pool,moderation_revision,blocked) VALUES($1,$2::numeric,$3) ON CONFLICT(pool) DO UPDATE SET moderation_revision=EXCLUDED.moderation_revision,blocked=EXCLUDED.blocked WHERE "metadata"."documents".moderation_revision<EXCLUDED.moderation_revision`,
	}
	wantObjects := [][]Object{
		{object("relation", "metadata.pools", "pool", "SELECT"), object("relation", "metadata.documents", "moderation_revision pool", "SELECT")},
		{object("relation", "metadata.pools", "creator pool", "SELECT UPDATE")},
		{object("relation", "metadata.documents", "blocked moderation_revision pool", "INSERT SELECT UPDATE")},
	}
	calls := sqlCalls(source.function(t, path, "Restrict"))
	if len(calls) != len(wantSQL) {
		t.Fatalf("Restrict SQL calls = %d, want %d", len(calls), len(wantSQL))
	}
	count := 0
	for _, statement := range RuntimeStatements {
		if statement.File != path || statement.Function != "Restrict" {
			continue
		}
		if statement.Call < 0 || statement.Call >= len(wantSQL) {
			t.Fatalf("unexpected Restrict call %d", statement.Call)
		}
		kind := "SELECT"
		if statement.Call == 2 {
			kind = "INSERT"
		}
		want := Statement{ID: fmt.Sprintf("production.Restrict.%d", statement.Call), Package: "internal/metadata", File: path, Function: "Restrict", Call: statement.Call, Kind: kind, Role: runtimeRole, SQL: wantSQL[statement.Call], Objects: wantObjects[statement.Call]}
		if !reflect.DeepEqual(statement, want) {
			t.Fatalf("Restrict statement = %+v, want %+v", statement, want)
		}
		if sql := source.resolve(t, statement); sql != wantSQL[statement.Call] {
			t.Fatalf("Restrict call %d SQL = %s, want %s", statement.Call, sql, wantSQL[statement.Call])
		}
		count++
	}
	if count != len(wantSQL) {
		t.Fatalf("Restrict inventory entries = %d, want %d", count, len(wantSQL))
	}
}
