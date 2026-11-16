package rolecheck

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestRuntimeRoleStatements(t *testing.T) {
	if len(WaitingStatements) != 0 {
		t.Fatal("every connected runtime statement requires an asserted plan")
	}
	root := coordinatortest.RepositoryRoot(t)
	source := sourceReader{root: root}
	source.assertCoverage(t)
	registered := registeredSchemas(t, root)
	seen := make(map[string]bool, len(RuntimeRoleStatements))
	for _, statement := range RuntimeRoleStatements {
		if statement.ID == "" || statement.File == "" || statement.Function == "" || statement.SQL == "" || statement.Kind == "" || seen[statement.ID] {
			t.Fatalf("invalid or duplicate statement: %+v", statement)
		}
		seen[statement.ID] = true
		if (statement.Kind == "INSERT" || statement.Kind == "UPDATE" || statement.Kind == "DELETE") && len(statement.Tables) == 0 {
			t.Fatalf("%s: statement relations are absent", statement.ID)
		}
		source.assertStatement(t, statement)
		for _, objects := range [][]string{statement.Tables, statement.Sequences, statement.Functions} {
			for _, object := range objects {
				schema, _, ok := strings.Cut(object, ".")
				if !ok || (schema != "pg_catalog" && !registered[schema]) {
					t.Fatalf("%s names unregistered object %s", statement.ID, object)
				}
			}
		}
	}
	for id, waiting := range WaitingStatements {
		if !seen[id] || waiting.Object == "" || waiting.Privilege == "" || waiting.Message == "" {
			t.Fatalf("invalid waiting statement %s: %+v", id, waiting)
		}
	}
	admin := os.Getenv("COORDINATOR_TEST_POSTGRES_DSN")
	if admin == "" {
		t.Fatal("COORDINATOR_TEST_POSTGRES_DSN is required")
	}
	database := coordinatortest.Bootstrap(t, admin, "identity_runtime", "control_runtime")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	connections := make(map[string]*pgx.Conn, 2)
	for _, role := range []string{"identity_runtime", "control_runtime"} {
		runtime, ok := database.Runtimes[role]
		if !ok {
			t.Fatalf("runtime role %s was not bootstrapped", role)
		}
		conn, err := pgx.Connect(ctx, runtime.DSN)
		if err != nil {
			t.Fatalf("connect %s: %v", role, err)
		}
		connections[role] = conn
		t.Cleanup(func() {
			if err := conn.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		var login string
		if err := conn.QueryRow(ctx, "SELECT current_user").Scan(&login); err != nil {
			t.Fatal(err)
		}
		if login != runtime.Role {
			t.Fatalf("connection for %s logs in as %s", role, login)
		}
	}
	totals, planned, executed := map[string]int{}, map[string]int{}, map[string]int{}
	for _, statement := range RuntimeRoleStatements {
		totals[statement.Package]++
		t.Run(statement.ID, func(t *testing.T) {
			conn := connections[statement.Role]
			if conn == nil {
				t.Fatalf("unknown runtime role %q", statement.Role)
			}
			err := planStatement(ctx, conn, statement)
			if waiting, listed := WaitingStatements[statement.ID]; listed {
				t.Logf("waiting role=%s object=%s privilege=%s message=%s; database=%v", statement.Role, waiting.Object, waiting.Privilege, waiting.Message, err)
				return
			}
			if err != nil {
				t.Fatalf("%s:%s call=%d role=%s: %v", statement.File, statement.Function, statement.Call, statement.Role, err)
			}
			if isUtility(statement.Kind) {
				executed[statement.Package]++
			} else {
				planned[statement.Package]++
			}
		})
	}
	packages := make([]string, 0, len(totals))
	for name := range totals {
		packages = append(packages, name)
	}
	sort.Strings(packages)
	for _, name := range packages {
		t.Logf("package=%s statements=%d planned=%d executed_in_rollback=%d waiting=%d", name, totals[name], planned[name], executed[name], totals[name]-planned[name]-executed[name])
	}
	t.Logf("gateway runtime statements: total=%d planned=%d executed_in_rollback=%d waiting=%d", len(RuntimeRoleStatements), sumCounts(planned), sumCounts(executed), len(WaitingStatements))
}

var parameter = regexp.MustCompile(`\$[1-9][0-9]*`)

func isUtility(kind string) bool { return kind == "SET" || kind == "SAVEPOINT" || kind == "ROLLBACK" }

func planStatement(ctx context.Context, conn *pgx.Conn, statement Statement) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	sql := statement.SQL
	if statement.Kind == "ROLLBACK" {
		if _, err = tx.Conn().PgConn().Exec(ctx, "SAVEPOINT mutation").ReadAll(); err != nil {
			return err
		}
	}
	if !isUtility(statement.Kind) {
		prefix := "EXPLAIN "
		if parameter.MatchString(sql) {
			prefix = "EXPLAIN (GENERIC_PLAN) "
		}
		sql = prefix + sql
	}
	results, err := tx.Conn().PgConn().Exec(ctx, sql).ReadAll()
	if err != nil {
		return err
	}
	if !isUtility(statement.Kind) {
		count := 0
		for _, result := range results {
			count += len(result.Rows)
		}
		if count == 0 {
			return fmt.Errorf("statement %s returned no plan", statement.ID)
		}
	}
	return tx.Rollback(ctx)
}

func registeredSchemas(t *testing.T, root string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "migrations", "registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var registry struct {
		Registrations []struct {
			Schema string `json:"schema"`
		} `json:"registrations"`
	}
	if err := json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	schemas := map[string]bool{"migration_control": true}
	for _, registration := range registry.Registrations {
		schemas[registration.Schema] = true
	}
	return schemas
}

func sumCounts(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}
