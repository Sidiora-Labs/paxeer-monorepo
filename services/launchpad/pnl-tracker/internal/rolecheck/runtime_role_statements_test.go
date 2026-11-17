package rolecheck

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func callKey(file, function string, ordinal int) string {
	return fmt.Sprintf("%s:%s:%d", file, function, ordinal)
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
	schemas := map[string]bool{"pg_catalog": true, "public": true}
	control := regexp.MustCompile(`(?m)^CREATE SCHEMA ([a-z][a-z0-9_]*) AUTHORIZATION migration_control_owner;`).FindStringSubmatch(production.MigrationControlDDL())
	if len(control) != 2 {
		t.Fatal("the coordinator DDL must declare its control schema")
	}
	schemas[control[1]] = true
	for _, migration := range registry.Registrations {
		schemas[migration.Schema] = true
	}
	if !schemas["pnl"] || !schemas["recovery"] || !schemas["api_stream"] {
		t.Fatal("pnl runtime schemas must have registered migrations")
	}
	return schemas
}

func validateInventory(t *testing.T, index *sourceIndex, statements []Statement, waiting []WaitingStatement) map[string]string {
	t.Helper()
	schemas := registeredSchemas(t, index.root)
	covered := map[string]bool{}
	ids := map[string]bool{}
	queries := map[string]string{}
	sourcePaths := map[string]bool{}
	for _, statement := range statements {
		if ids[statement.ID] {
			t.Errorf("duplicate statement %s", statement.ID)
		}
		ids[statement.ID] = true
		if statement.ID == "" || statement.File == "" || statement.Function == "" || statement.Role != "pnl_runtime" || statement.Kind == "" {
			t.Errorf("incomplete runtime statement: %+v", statement)
		}
		switch statement.Kind {
		case "INSERT", "UPDATE", "COPY":
			if len(statement.Tables) == 0 {
				t.Errorf("%s: runtime mutation requires its target table", statement.ID)
			}
		}
		covered[callKey(statement.File, statement.Function, statement.Call)] = true
		sourcePaths[statement.File] = true
		for _, object := range append(append(append([]string{}, statement.Tables...), statement.Sequences...), statement.Functions...) {
			schema, _, ok := strings.Cut(object, ".")
			if !ok || !schemas[schema] {
				t.Errorf("%s: object %s has no registered schema", statement.ID, object)
			}
		}
		bound := statement
		bound.Bindings = map[string]string{}
		for expression, value := range statement.Bindings {
			for schema := range schemas {
				value = strings.ReplaceAll(value, "{{schema:"+schema+"}}", schema)
				value = strings.ReplaceAll(value, "{{identifier:"+schema+"}}", pgx.Identifier{schema}.Sanitize())
			}
			if strings.Contains(value, "{{") {
				t.Errorf("%s: SQL schema binding %s has no registered migration", statement.ID, value)
			}
			bound.Bindings[expression] = value
		}
		sql, err := index.sql(bound)
		if err != nil {
			t.Errorf("%s: %v", statement.ID, err)
			continue
		}
		if actual := fmt.Sprintf("%x", sha256.Sum256([]byte(sql))); actual != statement.SHA256 {
			t.Errorf("%s: source SQL and identifier inventory differ: %s", statement.ID, sql)
		}
		if actual := strings.ToUpper(strings.Fields(sql)[0]); actual != statement.Kind && actual != "WITH" {
			t.Errorf("%s: statement kind %s differs from %s", statement.ID, statement.Kind, actual)
		}
		queries[statement.ID] = sql
	}
	if len(statements) == 0 {
		t.Error("the pnl runtime statement inventory is empty")
	}
	omissions, err := readDocument[omittedStatement]("omitted.json")
	if err != nil {
		t.Fatal(err)
	}
	omitted := map[string]bool{}
	for _, entry := range omissions {
		if entry.Reason == "" {
			t.Errorf("%s %s requires its runtime boundary", entry.File, entry.Function)
		}
		key := entry.File + ":" + entry.Function
		if omitted[key] {
			t.Errorf("duplicate omitted operation %s", key)
		}
		omitted[key] = true
		sourcePaths[entry.File] = true
	}
	// New service SQL calls must be represented alongside their runtime boundary.
	err = filepath.WalkDir(filepath.Join(index.root, "services", "launchpad", "pnl-tracker"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(index.root, path)
		if err != nil {
			return err
		}
		sourcePaths[relative] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for path := range sourcePaths {
		f, err := index.file(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range f.ast.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := functionName(fn)
			for ordinal := range statementCalls(fn) {
				key := callKey(path, name, ordinal+1)
				if !covered[key] && !omitted[path+":"+name] {
					t.Errorf("SQL call %s needs its runtime inventory entry", key)
				}
				if covered[key] && omitted[path+":"+name] {
					t.Errorf("SQL call %s has conflicting runtime boundaries", key)
				}
			}
		}
	}
	waitingIDs := map[string]bool{}
	for _, entry := range waiting {
		if !ids[entry.Statement] {
			t.Errorf("waiting statement %s is absent from the inventory", entry.Statement)
		}
		if waitingIDs[entry.Statement] {
			t.Errorf("duplicate waiting statement %s", entry.Statement)
		}
		waitingIDs[entry.Statement] = true
		if entry.Object == "" || entry.Right == "" || entry.Message == "" {
			t.Errorf("waiting statement %s needs its object, right and database message", entry.Statement)
		}
	}
	if t.Failed() {
		t.Fatal("runtime statement inventory must describe its source calls")
	}
	return queries
}

var sqlParameter = regexp.MustCompile(`\$[1-9][0-9]*`)

func planRuntimeStatement(ctx context.Context, conn *pgx.Conn, statement Statement, sql string) (planned bool, err error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rollbackErr := tx.Rollback(rollbackCtx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, rollbackErr)
		}
	}()
	switch statement.Kind {
	case "SET", "SAVEPOINT", "ROLLBACK":
		if statement.Kind == "ROLLBACK" {
			// The live wallet fold establishes this savepoint before its refusal branch.
			if _, err = tx.Exec(ctx, "SAVEPOINT live_wallet_fold"); err != nil {
				return false, err
			}
		}
		_, err = tx.Exec(ctx, sql)
		return false, err
	case "COPY":
		_, err = tx.Conn().PgConn().CopyFrom(ctx, copyBinaryEmpty(), sql)
		return false, err
	default:
		options := "FORMAT TEXT"
		if sqlParameter.MatchString(sql) {
			options += ", GENERIC_PLAN"
		}
		results, err := tx.Conn().PgConn().Exec(ctx, "EXPLAIN ("+options+") "+sql).ReadAll()
		if err != nil {
			return false, err
		}
		lines := 0
		for _, result := range results {
			for _, row := range result.Rows {
				if len(row) != 1 || strings.TrimSpace(string(row[0])) == "" {
					return false, fmt.Errorf("empty runtime plan line")
				}
				lines++
			}
		}
		if lines == 0 {
			return false, fmt.Errorf("runtime statement returned no plan")
		}
		return true, nil
	}
}

func TestRuntimeRoleStatements(t *testing.T) {
	statements, err := Statements()
	if err != nil {
		t.Fatal(err)
	}
	waiting, err := Waiting()
	if err != nil {
		t.Fatal(err)
	}
	if len(waiting) != 0 {
		t.Fatal("every connected runtime statement requires an asserted plan")
	}
	index := &sourceIndex{root: coordinatortest.RepositoryRoot(t), positions: token.NewFileSet(), files: map[string]*sourceFile{}, schema: "pnl"}
	queries := validateInventory(t, index, statements, waiting)
	admin := os.Getenv("COORDINATOR_TEST_POSTGRES_DSN")
	if admin == "" {
		t.Fatal("COORDINATOR_TEST_POSTGRES_DSN is required for runtime statement planning")
	}
	database := coordinatortest.Bootstrap(t, admin, "pnl_runtime")
	runtime := database.Runtimes["pnl_runtime"]
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if _, err := conn.Exec(ctx, "SET search_path TO pnl,pg_catalog; SET statement_timeout TO '30s'"); err != nil {
		t.Fatal(err)
	}
	var login string
	if err := conn.QueryRow(ctx, "SELECT current_user").Scan(&login); err != nil {
		t.Fatal(err)
	}
	if login != runtime.Role {
		t.Fatalf("runtime statements require the pnl login: %s", login)
	}
	pending := map[string]WaitingStatement{}
	for _, entry := range waiting {
		pending[entry.Statement] = entry
	}
	planned, executed, held := 0, 0, 0
	packages := map[string]int{}
	for _, statement := range statements {
		packages[filepath.Dir(statement.File)]++
		t.Run(statement.ID, func(t *testing.T) {
			t.Logf("file=%s function=%s role=%s kind=%s SQL=%q", statement.File, statement.Function, statement.Role, statement.Kind, queries[statement.ID])
			bounded, cancel := context.WithTimeout(ctx, 35*time.Second)
			defer cancel()
			plan, err := planRuntimeStatement(bounded, conn, statement, queries[statement.ID])
			if entry, ok := pending[statement.ID]; ok {
				held++
				t.Logf("waiting: object=%s right=%s database=%s; current result=%v", entry.Object, entry.Right, entry.Message, err)
				return
			}
			if err != nil {
				var postgres *pgconn.PgError
				if errors.As(err, &postgres) {
					t.Errorf("runtime statement: SQLSTATE=%s message=%s table=%s column=%s detail=%s", postgres.Code, postgres.Message, postgres.TableName, postgres.ColumnName, postgres.Detail)
				} else {
					t.Error(err)
				}
				return
			}
			if plan {
				planned++
				t.Log("planned as pnl_runtime")
			} else {
				executed++
				t.Log("executed as pnl_runtime and rolled back")
			}
		})
	}
	names := make([]string, 0, len(packages))
	for name := range packages {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Logf("package %s: %d statements", name, packages[name])
	}
	t.Logf("runtime statement totals: inventory=%d planned=%d rolled_back=%d waiting=%d", len(statements), planned, executed, held)
}
