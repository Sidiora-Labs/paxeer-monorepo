package rolecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
)

type migrationRegistry struct {
	Registrations []struct {
		Schema string `json:"schema"`
	} `json:"registrations"`
}

func registeredSchemas(t testing.TB) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(coordinatortest.RepositoryRoot(t), "migrations", "registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var registry migrationRegistry
	if err := json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	result := map[string]bool{"migration_control": true, "pg_catalog": true}
	for _, registration := range registry.Registrations {
		result[registration.Schema] = true
	}
	return result
}

var parameter = regexp.MustCompile(`\$[1-9][0-9]*`)

func planRuntimeStatement(ctx context.Context, conn *pgx.Conn, statement Statement, sql string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if statement.Kind == "SET" {
		_, err = tx.Exec(ctx, sql)
	} else {
		explain := "EXPLAIN "
		if parameter.MatchString(sql) {
			explain = "EXPLAIN (GENERIC_PLAN) "
		}
		results, planErr := tx.Conn().PgConn().Exec(ctx, explain+sql).ReadAll()
		err = planErr
		if err == nil {
			count := 0
			for _, result := range results {
				count += len(result.Rows)
			}
			if count == 0 {
				return fmt.Errorf("statement %s returned no plan", statement.ID)
			}
		}
	}
	rollbackErr := tx.Rollback(ctx)
	if err != nil {
		return err
	}
	return rollbackErr
}

func TestRuntimeRoleStatements(t *testing.T) {
	source := sourceReader{root: coordinatortest.RepositoryRoot(t), files: map[string]*ast.File{}}
	source.assertCoverage(t)
	schemas := registeredSchemas(t)
	inventory := map[string]Statement{}
	resolved := map[string]string{}
	counts := map[string]int{}
	for _, statement := range RuntimeStatements {
		if statement.ID == "" || statement.Function == "" || statement.File == "" || statement.Package == "" || statement.Role != runtimeRole {
			t.Fatalf("incomplete statement inventory entry: %+v", statement)
		}
		if _, exists := inventory[statement.ID]; exists {
			t.Fatalf("duplicate statement name %q", statement.ID)
		}
		for _, object := range statement.Objects {
			parts := strings.Split(object.Name, ".")
			if len(parts) != 2 || !schemas[parts[0]] || len(object.Rights) == 0 {
				t.Fatalf("%s: incomplete object or unregistered schema: %+v", statement.ID, object)
			}
		}
		inventory[statement.ID] = statement
		resolved[statement.ID] = source.resolve(t, statement)
		counts[statement.Package]++
	}
	waiting := map[string]WaitingStatement{}
	for _, entry := range WaitingStatements {
		if _, exists := inventory[entry.Statement]; !exists {
			t.Fatalf("waiting list names unknown statement %q", entry.Statement)
		}
		if _, exists := waiting[entry.Statement]; exists {
			t.Fatalf("duplicate waiting statement %q", entry.Statement)
		}
		if entry.Object == "" || entry.Right == "" || entry.Message == "" {
			t.Fatalf("incomplete waiting entry: %+v", entry)
		}
		waiting[entry.Statement] = entry
	}
	admin := os.Getenv("COORDINATOR_TEST_POSTGRES_DSN")
	if admin == "" {
		t.Fatal("COORDINATOR_TEST_POSTGRES_DSN is required")
	}
	database := coordinatortest.Bootstrap(t, admin, runtimeRole)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	config, err := pgx.ParseConfig(database.Runtimes[runtimeRole].DSN)
	if err != nil {
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = "metadata,pg_catalog"
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	var role string
	if err := conn.QueryRow(ctx, "SELECT current_user").Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != database.Runtimes[runtimeRole].Role {
		t.Fatal("statement connection differs from runtime role")
	}
	planned, executed := 0, 0
	for _, statement := range RuntimeStatements {
		t.Run(statement.ID, func(t *testing.T) {
			sql := resolved[statement.ID]
			t.Logf("%s:%s call=%d role=%s kind=%s SQL=%s", statement.File, statement.Function, statement.Call, statement.Role, statement.Kind, sql)
			err := planRuntimeStatement(ctx, conn, statement, sql)
			if entry, listed := waiting[statement.ID]; listed {
				t.Logf("waiting object=%s right=%s message=%s; database=%v", entry.Object, entry.Right, entry.Message, err)
				return
			}
			if err != nil {
				t.Fatalf("%s:%s call=%d: %v", statement.File, statement.Function, statement.Call, err)
			}
			if statement.Kind == "SET" {
				executed++
			} else {
				planned++
			}
		})
	}
	packages := make([]string, 0, len(counts))
	for pkg := range counts {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	for _, pkg := range packages {
		t.Logf("package=%s statements=%d", pkg, counts[pkg])
	}
	t.Logf("statements=%d planned=%d executed_in_rollback=%d waiting=%d", len(RuntimeStatements), planned, executed, len(WaitingStatements))
	t.Logf("development-only=%v", DevelopmentOnlyStatements)
}
