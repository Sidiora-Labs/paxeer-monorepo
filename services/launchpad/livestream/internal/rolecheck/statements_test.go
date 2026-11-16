package rolecheck

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
)

var parameter = regexp.MustCompile(`\$[1-9][0-9]*`)

func TestRuntimeRoleStatements(t *testing.T) {
	root := coordinatortest.RepositoryRoot(t)
	schemas, err := registeredSchemas(root)
	if err != nil {
		t.Fatal(err)
	}
	statements, err := Statements(root, schemas["media/livestream"])
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Statement{}
	for _, statement := range statements {
		byID[statement.ID] = statement
	}
	waiting := map[string]WaitingStatement{}
	for _, entry := range Waiting {
		statement, ok := byID[entry.Statement]
		if !ok {
			t.Fatalf("waiting statement is not in inventory: %s", entry.Statement)
		}
		if _, duplicate := waiting[entry.Statement]; duplicate {
			t.Fatalf("duplicate waiting statement: %s", entry.Statement)
		}
		if entry.Right == "" || entry.Message == "" || !namesObject(statement, entry.Object) {
			t.Fatalf("waiting statement needs an inventoried object, right and message: %+v", entry)
		}
		waiting[entry.Statement] = entry
	}
	admin := os.Getenv("COORDINATOR_TEST_POSTGRES_DSN")
	if admin == "" {
		t.Fatal("COORDINATOR_TEST_POSTGRES_DSN is required")
	}
	database := coordinatortest.Bootstrap(t, admin, RuntimeRole)
	runtime := database.Runtimes[RuntimeRole]
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cfg, err := pgx.ParseConfig(runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	// The ledger's unqualified names belong to the registered service schema.
	cfg.RuntimeParams["client_encoding"] = "UTF8"
	cfg.RuntimeParams["search_path"] = pgx.Identifier{schemas["media/livestream"]}.Sanitize() + ", pg_catalog"
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := conn.Close(cleanup); err != nil {
			t.Error(err)
		}
	}()
	var role string
	if err = conn.QueryRow(ctx, "SELECT current_user").Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != runtime.Role {
		t.Fatalf("runtime login: got %s, want %s", role, runtime.Role)
	}
	type counts struct{ total, planned, executed, waiting int }
	totals := map[string]*counts{}
	for _, statement := range statements {
		if totals[statement.Package] == nil {
			totals[statement.Package] = &counts{}
		}
		count := totals[statement.Package]
		count.total++
		t.Run(statement.ID, func(t *testing.T) {
			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
				defer stop()
				if err := tx.Rollback(cleanup); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
					t.Error(err)
				}
			}()
			plan, err := inspectStatement(ctx, tx, statement)
			explain := statement.Kind != "SET"
			if entry, listed := waiting[statement.ID]; listed {
				count.waiting++
				t.Logf("waiting: object=%s right=%s response=%v", entry.Object, entry.Right, err)
				return
			}
			if err != nil {
				t.Fatalf("%s %s as %s: %v", statement.Kind, statement.Function, statement.Role, err)
			}
			if explain {
				if len(plan) == 0 {
					t.Fatal("EXPLAIN returned no plan")
				}
				count.planned++
				t.Logf("planned: %s", strings.Join(plan, "; "))
			} else {
				count.executed++
				t.Log("executed; transaction rolled back")
			}
		})
	}
	packages := make([]string, 0, len(totals))
	for pkg := range totals {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	for _, pkg := range packages {
		count := totals[pkg]
		t.Logf("%s: statements=%d planned=%d executed=%d waiting=%d", pkg, count.total, count.planned, count.executed, count.waiting)
	}
}

func namesObject(statement Statement, object string) bool {
	for _, name := range append(append(append([]string{}, statement.Tables...), statement.Sequences...), statement.Functions...) {
		if object == name {
			return true
		}
		if strings.HasPrefix(object, name+".") {
			column := strings.TrimPrefix(object, name+".")
			for _, named := range statement.Columns {
				if named == column {
					return true
				}
			}
		}
	}
	return false
}

func inspectStatement(ctx context.Context, tx pgx.Tx, statement Statement) ([]string, error) {
	if statement.Kind == "SET" {
		_, err := tx.Exec(ctx, statement.SQL)
		return nil, err
	}
	explain := "EXPLAIN "
	if parameter.MatchString(statement.SQL) {
		explain = "EXPLAIN (GENERIC_PLAN) "
	}
	results, err := tx.Conn().PgConn().Exec(ctx, explain+statement.SQL).ReadAll()
	if err != nil {
		return nil, err
	}
	var plan []string
	count := 0
	for _, result := range results {
		count += len(result.Rows)
		for _, row := range result.Rows {
			if len(row) > 0 {
				plan = append(plan, string(row[0]))
			}
		}
	}
	if count == 0 {
		return nil, fmt.Errorf("statement %s returned no plan", statement.ID)
	}
	return plan, nil
}
