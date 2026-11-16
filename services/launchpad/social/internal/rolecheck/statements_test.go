package rolecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRuntimeRoleStatements(t *testing.T) {
	if len(Waiting) != 0 {
		t.Fatal("every connected runtime statement requires an asserted plan")
	}
	root := coordinatortest.RepositoryRoot(t)
	statements, err := Statements(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(statements) == 0 {
		t.Fatal("runtime SQL inventory is empty")
	}
	seen := map[string]Statement{}
	for _, s := range statements {
		if _, duplicate := seen[s.ID]; duplicate {
			t.Fatalf("duplicate statement %s", s.ID)
		}
		if s.Role != RuntimeRole || s.File == "" || s.Function == "" || s.Kind == "" || strings.TrimSpace(s.SQL) == "" {
			t.Fatalf("incomplete statement %s", s.ID)
		}
		seen[s.ID] = s
	}
	assertSourceCoverage(t, root, statements)
	assertRegisteredSchemas(t, root, statements)
	waiting := map[string]WaitingStatement{}
	for _, entry := range Waiting {
		if _, ok := seen[entry.Statement]; !ok {
			t.Errorf("waiting statement absent from inventory: %s", entry.Statement)
		}
		if _, duplicate := waiting[entry.Statement]; duplicate {
			t.Errorf("duplicate waiting statement: %s", entry.Statement)
		}
		if entry.Object == "" || entry.Right == "" || entry.Message == "" {
			t.Errorf("incomplete waiting entry: %s", entry.Statement)
		}
		waiting[entry.Statement] = entry
	}
	if t.Failed() {
		t.FailNow()
	}
	admin := os.Getenv("COORDINATOR_TEST_POSTGRES_DSN")
	if admin == "" {
		t.Fatal("COORDINATOR_TEST_POSTGRES_DSN is required")
	}
	database := coordinatortest.Bootstrap(t, admin, RuntimeRole)
	runtime := database.Runtimes[RuntimeRole]
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
	defer cancel()
	config, err := pgx.ParseConfig(runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = "chat,public"
	config.RuntimeParams["client_encoding"] = "UTF8"
	config.RuntimeParams["statement_timeout"] = "10000"
	config.RuntimeParams["lock_timeout"] = "2000"
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	var user string
	var superuser bool
	if err = conn.QueryRow(ctx, "SELECT current_user,rolsuper FROM pg_roles WHERE rolname=current_user").Scan(&user, &superuser); err != nil {
		t.Fatal(err)
	}
	if user != runtime.Role || superuser {
		t.Fatalf("runtime connection authority: user=%s superuser=%t", user, superuser)
	}
	planned, executed := 0, 0
	for _, statement := range statements {
		t.Run(statement.ID, func(t *testing.T) {
			isPlan, err := planStatement(ctx, conn, statement)
			if entry, listed := waiting[statement.ID]; listed {
				t.Logf("waiting: object=%s right=%s database=%v", entry.Object, entry.Right, err)
				return
			}
			if err != nil {
				var pgerr *pgconn.PgError
				if errors.As(err, &pgerr) {
					t.Fatalf("%s %s: SQLSTATE %s: %s", statement.Role, statement.Kind, pgerr.Code, pgerr.Message)
				}
				t.Fatal(err)
			}
			if isPlan {
				planned++
			} else {
				executed++
			}
		})
	}
	counts := Packages(statements)
	for _, name := range packageNames(counts) {
		t.Logf("package %s: %d statements", name, counts[name])
	}
	t.Logf("runtime SQL: %d statements, %d planned, %d executed in rolled-back transactions, %d waiting", len(statements), planned, executed, len(waiting))
}

func planStatement(ctx context.Context, conn *pgx.Conn, statement Statement) (bool, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return false, err
	}
	if statement.Kind == "SET" {
		_, err = tx.Exec(ctx, statement.SQL)
	} else {
		prefix := "EXPLAIN "
		if strings.Contains(statement.SQL, "$") {
			prefix = "EXPLAIN (GENERIC_PLAN) "
		}
		results, execErr := tx.Conn().PgConn().Exec(ctx, prefix+statement.SQL).ReadAll()
		err = execErr
		if err == nil {
			lines := 0
			for _, result := range results {
				lines += len(result.Rows)
			}
			if lines == 0 {
				err = fmt.Errorf("statement returned no plan")
			}
		}
	}
	rollbackErr := tx.Rollback(ctx)
	if err != nil {
		return statement.Kind != "SET", err
	}
	return statement.Kind != "SET", rollbackErr
}

func assertSourceCoverage(t *testing.T, root string, statements []Statement) {
	t.Helper()
	represented := map[string]bool{}
	for _, statement := range statements {
		represented[sourceKey(statement.File, statement.Function, statement.Ordinal)] = true
	}
	excluded := map[string]string{
		sourceKey("services/launchpad/social/internal/db/sqlcdb/query.sql.go", "Queries.GetPoolCreator", 1): "Connected app.New supplies PoolCreator from indexer_canonical_creations_v1.",
		sourceKey("services/launchpad/social/internal/devsocial/runtime.go", "runtimeKey", 1):               "Development persists a generated cursor key; connected runtimes supply it.",
		sourceKey("services/launchpad/social/internal/devsocial/runtime.go", "runtimeKey", 2):               "Development reads its persisted cursor key; connected runtimes supply it.",
		sourceKey("services/launchpad/social/internal/devsocial/policy.go", "Policy.SendChat", 1):           "Canonical direct-message policy delegates to the identity guards.",
		sourceKey("services/launchpad/social/internal/interactions/policy.go", "PGPolicy.trusted", 2):       "Connected runtimes use the registered session guard.",
	}
	err := filepath.WalkDir(filepath.Join(root, "services/launchpad/social"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "internaltest" || entry.Name() == "rolecheck" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			for ordinal := range queryCalls(function) {
				key := sourceKey(relative, functionName(function), ordinal+1)
				if !represented[key] && excluded[key] == "" {
					t.Errorf("runtime SQL call absent from inventory: %s", key)
				}
				delete(excluded, key)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for key := range excluded {
		t.Errorf("excluded SQL call absent from source: %s", key)
	}
	shared := map[string]map[string]bool{}
	for _, statement := range statements {
		if !strings.HasPrefix(statement.File, "sdk/") {
			continue
		}
		if shared[statement.File] == nil {
			shared[statement.File] = map[string]bool{}
		}
		shared[statement.File][statement.Function] = true
	}
	for path, functions := range shared {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, path), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || !functions[functionName(function)] {
				continue
			}
			for ordinal := range queryCalls(function) {
				key := sourceKey(path, functionName(function), ordinal+1)
				if !represented[key] {
					t.Errorf("shared runtime SQL call absent from inventory: %s", key)
				}
			}
		}
	}
}

func sourceKey(file, function string, ordinal int) string {
	return fmt.Sprintf("%s:%s:%d", file, function, ordinal)
}

func assertRegisteredSchemas(t *testing.T, root string, statements []Statement) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "migrations/registry.json"))
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
	registered := map[string]bool{"pg_catalog": true, "migration_control": true}
	for _, migration := range registry.Registrations {
		registered[migration.Schema] = true
	}
	for _, statement := range statements {
		objects := append(append(append([]string{}, statement.Tables...), statement.Sequences...), statement.Functions...)
		for _, object := range objects {
			schema, _, qualified := strings.Cut(object, ".")
			if !qualified || !registered[schema] {
				t.Errorf("%s names unregistered schema: %s", statement.ID, object)
			}
		}
	}
}
