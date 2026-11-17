package rolecheck

import (
	"context"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5/pgxpool"
)

const coordinatorDSNEnv = "COORDINATOR_TEST_POSTGRES_DSN"

var parameterMarker = regexp.MustCompile(`\$[0-9]+`)

// TestRuntimeRoleStatements plans every connected-form indexer statement as
// indexer_runtime against the coordinator's least-privilege database.
func TestRuntimeRoleStatements(t *testing.T) {
	if len(Waiting) != 0 {
		t.Fatal("every connected runtime statement requires an asserted plan")
	}
	admin := os.Getenv(coordinatorDSNEnv)
	if admin == "" {
		t.Skipf("%s is not set", coordinatorDSNEnv)
	}
	database := coordinatortest.Bootstrap(t, admin, "indexer_runtime")
	runtime := database.Runtimes["indexer_runtime"]
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err = pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	waiting := make(map[string]WaitingStatement, len(Waiting))
	for _, item := range Waiting {
		if _, duplicate := waiting[item.StatementID]; duplicate {
			t.Fatalf("waiting list repeats statement %s", item.StatementID)
		}
		waiting[item.StatementID] = item
	}
	byID := make(map[string]Statement, len(RuntimeRoleStatements))
	counts := map[string]int{}
	planned := 0
	failures := make([]string, 0)
	for _, statement := range RuntimeRoleStatements {
		if statement.Role != "indexer_runtime" {
			t.Fatalf("%s has role %q", statement.ID, statement.Role)
		}
		if _, duplicate := byID[statement.ID]; duplicate {
			t.Fatalf("statement id %s is repeated", statement.ID)
		}
		byID[statement.ID] = statement
		counts[statement.Package]++
		if _, listed := waiting[statement.ID]; listed {
			continue
		}
		if err := planStatement(ctx, pool, statement); err != nil {
			failures = append(failures, statement.ID+" "+statement.File+":"+statement.Function+": "+err.Error())
			continue
		}
		planned++
	}
	for _, item := range Waiting {
		if _, ok := byID[item.StatementID]; !ok {
			t.Errorf("waiting entry %s does not name an inventory statement", item.StatementID)
		}
		if item.Object == "" || item.Privilege == "" || item.Message == "" {
			t.Errorf("waiting entry %s must name object, privilege and database message", item.StatementID)
		}
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		t.Fatalf("unlisted statements refused by indexer_runtime:\n%s", strings.Join(failures, "\n"))
	}
	packages := make([]string, 0, len(counts))
	for packageName := range counts {
		packages = append(packages, packageName)
	}
	sort.Strings(packages)
	parts := make([]string, 0, len(packages))
	for _, packageName := range packages {
		parts = append(parts, packageName+"="+itoa(counts[packageName]))
	}
	listed := make([]string, 0, len(Waiting))
	for _, item := range Waiting {
		listed = append(listed, item.StatementID+" ("+item.Object+"; "+item.Privilege+"; "+item.Message+")")
	}
	t.Logf("indexer_runtime statements: packages=%s planned=%d waiting=%s", strings.Join(parts, ", "), planned, strings.Join(listed, ", "))
}

func planStatement(ctx context.Context, pool *pgxpool.Pool, statement Statement) error {
	if parameterMarker.MatchString(statement.SQL) {
		_, err := pool.Exec(ctx, "EXPLAIN (GENERIC_PLAN, FORMAT TEXT) "+statement.SQL)
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, statement.SQL)
	return err
}

func itoa(value int) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = digits[value%10]
		value /= 10
	}
	return string(buf[i:])
}
