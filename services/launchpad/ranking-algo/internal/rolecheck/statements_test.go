package rolecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
)

func TestRuntimeRoleStatements(t *testing.T) {
	root := coordinatortest.RepositoryRoot(t)
	source := sourceReader{root: root, files: map[string]*ast.File{}}
	source.assertCoverage(t)
	inventory := map[string]Statement{}
	resolved := map[string]resolvedStatement{}
	packages := map[string]int{}
	registered := registeredSchemas(t, root)
	for _, s := range Statements {
		if s.ID == "" || s.File == "" || s.Function == "" || s.Role != runtimeRole {
			t.Fatalf("invalid runtime statement: %+v", s)
		}
		if _, exists := inventory[s.ID]; exists {
			t.Fatalf("duplicate runtime statement %s", s.ID)
		}
		for _, object := range s.Objects {
			parts := strings.Split(object.Name, ".")
			if len(parts) != 2 || len(object.Rights) == 0 {
				t.Fatalf("%s: incomplete object %s", s.ID, object.Name)
			}
			if parts[0] != "pg_catalog" && !registered[parts[0]] {
				t.Fatalf("%s: unregistered schema %s", s.ID, parts[0])
			}
		}
		inventory[s.ID] = s
		resolved[s.ID] = source.resolve(t, s)
		packages[filepath.Dir(s.File)]++
	}
	waiting := map[string]WaitingStatement{}
	for _, entry := range Waiting {
		if _, exists := inventory[entry.Statement]; !exists {
			t.Fatalf("waiting statement %s is absent from inventory", entry.Statement)
		}
		if _, exists := waiting[entry.Statement]; exists {
			t.Fatalf("duplicate waiting statement %s", entry.Statement)
		}
		if entry.Object == "" || entry.Right == "" || entry.Message == "" {
			t.Fatalf("waiting statement %s requires object, right and database message", entry.Statement)
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
	conn, err := pgx.Connect(ctx, database.Runtimes[runtimeRole].DSN)
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
	for _, s := range Statements {
		t.Run(s.ID, func(t *testing.T) {
			query := resolved[s.ID]
			t.Logf("%s:%s call=%d role=%s kind=%s SQL=%s", s.File, s.Function, s.Call, s.Role, s.Kind, query.SQL)
			err := planStatement(ctx, conn, s, query)
			if entry, listed := waiting[s.ID]; listed {
				t.Logf("waiting object=%s right=%s message=%s; database=%v", entry.Object, entry.Right, entry.Message, err)
				return
			}
			if err != nil {
				t.Fatalf("%s:%s: %v", s.File, s.Function, err)
			}
			if s.Kind == "SET" || s.Kind == "COPY" {
				executed++
			} else {
				planned++
			}
		})
	}
	for pkg, count := range packages {
		t.Logf("package=%s statements=%d", pkg, count)
	}
	t.Logf("statements=%d planned=%d executed_in_rollback=%d waiting=%d", len(Statements), planned, executed, len(Waiting))
}

var parameter = regexp.MustCompile(`\$[1-9][0-9]*`)

func planStatement(ctx context.Context, conn *pgx.Conn, s Statement, query resolvedStatement) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if s.Kind == "COPY" {
		_, err = tx.CopyFrom(ctx, query.Table, query.Columns, pgx.CopyFromRows(nil))
	} else if s.Kind == "SET" {
		_, err = tx.Exec(ctx, query.SQL)
	} else {
		explain := "EXPLAIN "
		if parameter.MatchString(query.SQL) {
			explain = "EXPLAIN (GENERIC_PLAN) "
		}
		results, planErr := tx.Conn().PgConn().Exec(ctx, explain+query.SQL).ReadAll()
		err = planErr
		if err == nil {
			count := 0
			for _, result := range results {
				count += len(result.Rows)
			}
			if count == 0 {
				return fmt.Errorf("statement %s returned no plan", s.ID)
			}
		}
	}
	rollbackErr := tx.Rollback(ctx)
	if err != nil {
		return err
	}
	return rollbackErr
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
	if err = json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{"migration_control": true}
	for _, m := range registry.Registrations {
		names[m.Schema] = true
	}
	return names
}
