package rolecheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
)

const runtimeRole = "users_runtime"

func TestRuntimeRoleStatements(t *testing.T) {
	var statements []Statement
	var waiting []WaitingEntry
	decodeInventory(t, statementsJSON, &statements)
	decodeInventory(t, waitingJSON, &waiting)
	if len(statements) == 0 {
		t.Fatal("runtime statement inventory is empty")
	}
	root := coordinatortest.RepositoryRoot(t)
	live := sourceStatements(t, root)
	inventory := make(map[string]Statement, len(statements))
	counts := map[string]int{}
	for _, statement := range statements {
		if _, exists := inventory[statement.ID]; exists {
			t.Fatalf("duplicate statement %s", statement.ID)
		}
		if statement.Role != runtimeRole || statement.Kind == "" || statement.File == "" || statement.Function == "" || statement.Call < 1 {
			t.Fatalf("incomplete statement %s", statement.ID)
		}
		actual, ok := live[statement.ID]
		if !ok || actual.SQL != statement.SQL || !reflect.DeepEqual(actual.Bindings, statement.Bindings) {
			t.Fatalf("source statement differs: %s", statement.ID)
		}
		inventory[statement.ID] = statement
		counts[filepath.Dir(statement.File)]++
	}
	for id := range live {
		if _, ok := inventory[id]; !ok {
			t.Errorf("runtime statement absent from inventory: %s", id)
		}
	}
	if t.Failed() {
		return
	}
	held := map[string]WaitingEntry{}
	for _, entry := range waiting {
		if _, exists := inventory[entry.Statement]; !exists {
			t.Fatalf("waiting statement absent from inventory: %s", entry.Statement)
		}
		if _, exists := held[entry.Statement]; exists {
			t.Fatalf("duplicate waiting statement: %s", entry.Statement)
		}
		if len(entry.Requirements) == 0 || entry.Message == "" {
			t.Fatalf("incomplete waiting entry: %s", entry.Statement)
		}
		for _, requirement := range entry.Requirements {
			if requirement.Object == "" || requirement.Right == "" {
				t.Fatalf("incomplete requirement: %s", entry.Statement)
			}
		}
		held[entry.Statement] = entry
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
		t.Fatalf("runtime login differs: %s", role)
	}
	registered := registeredSchemas(t, root)
	planned, executed := 0, 0
	for _, statement := range statements {
		t.Run(strings.TrimPrefix(statement.ID, "services/launchpad/user/internal/"), func(t *testing.T) {
			sql := qualifyStatement(t, statement, registered)
			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := tx.Rollback(context.Background()); err != nil {
					t.Error(err)
				}
			}()
			if _, err = tx.Exec(ctx, "SET LOCAL search_path TO users, pg_catalog"); err != nil {
				t.Fatal(err)
			}
			utility := statement.Kind == "LOCK" || statement.Kind == "VARIABLESET"
			command := sql
			if !utility {
				prefix := "EXPLAIN "
				if parameter.MatchString(sql) {
					prefix = "EXPLAIN (GENERIC_PLAN) "
				}
				command = prefix + sql
			}
			results, execErr := tx.Conn().PgConn().Exec(ctx, command).ReadAll()
			err = execErr
			if err == nil {
				rows := 0
				for _, result := range results {
					if result.Err != nil {
						err = result.Err
						break
					}
					rows += len(result.Rows)
				}
				if err == nil && !utility && rows == 0 {
					err = fmt.Errorf("empty statement plan")
				}
			}
			entry, listed := held[statement.ID]
			if listed {
				t.Logf("waiting: %s; requirements=%v; database=%v", statement.ID, entry.Requirements, err)
			} else if err != nil {
				t.Fatalf("%s as %s: %v", sql, statement.Role, err)
			}
			if err == nil {
				if utility {
					executed++
				} else {
					planned++
				}
				t.Logf("%s as %s: qualified", statement.ID, statement.Role)
			}
		})
	}
	packages := make([]string, 0, len(counts))
	for name := range counts {
		packages = append(packages, name)
	}
	sort.Strings(packages)
	for _, name := range packages {
		t.Logf("inventory %s: %d", name, counts[name])
	}
	t.Logf("statements=%d planned=%d executed=%d waiting=%d", len(statements), planned, executed, len(waiting))
}

func decodeInventory(t *testing.T, raw []byte, target any) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		t.Fatal(err)
	}
}

var parameter = regexp.MustCompile(`\$[1-9][0-9]*`)
var relationReference = regexp.MustCompile(`(?i)\b(FROM|JOIN|UPDATE|INTO|TABLE)\s+("?[a-z_][a-z_0-9]*"?)(\s*\.)?`)

func registeredSchemas(t *testing.T, root string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "migrations/registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Registrations []struct {
			Schema string `json:"schema"`
		} `json:"registrations"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	schemas := map[string]bool{"pg_catalog": true, "migration_control": true, "public": true}
	for _, registration := range document.Registrations {
		schemas[registration.Schema] = true
	}
	if !schemas["users"] {
		t.Fatal("users schema is not registered")
	}
	return schemas
}

func qualifyStatement(t *testing.T, statement Statement, schemas map[string]bool) string {
	t.Helper()
	objects := map[string]string{}
	for _, object := range append(append(append([]string{}, statement.Tables...), statement.Sequences...), statement.Functions...) {
		parts := strings.Split(object, ".")
		if len(parts) != 2 || !schemas[parts[0]] {
			t.Fatalf("unregistered object schema: %s", object)
		}
	}
	for _, object := range statement.Tables {
		parts := strings.Split(object, ".")
		objects[parts[1]] = parts[0]
	}
	return relationReference.ReplaceAllStringFunc(statement.SQL, func(reference string) string {
		parts := relationReference.FindStringSubmatch(reference)
		if parts[3] != "" {
			return reference
		}
		name := strings.Trim(parts[2], `"`)
		schema, ok := objects[name]
		if !ok {
			return reference
		}
		return parts[1] + " " + pgx.Identifier{schema, name}.Sanitize()
	})
}

func sourceStatements(t *testing.T, root string) map[string]Statement {
	t.Helper()
	sources := map[string]map[string]bool{
		"sdk/launchpad/shared/auth/contract.go":            {"CommittedBlock": true},
		"sdk/launchpad/shared/auth/ledger.go":              {"ExecuteValidatedTx": true, "AppendAudit": true},
		"sdk/launchpad/shared/auth/http_authorization.go":  {"Resolve": true, "ResolveTx": true},
		"sdk/launchpad/shared/delivery/store.go":           {"Enqueue": true, "Claim": true, "MarkPublished": true, "Retry": true, "CheckAttemptBudget": true},
		"sdk/launchpad/shared/db/production/migrations.go": {"CheckReadiness": true, "CatalogFingerprint": true, "checkPrivilege": true, "checkUnexpectedPrivileges": true},
	}
	base := "services/launchpad/user/"
	err := filepath.WalkDir(filepath.Join(root, base), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch relative {
			case base + "internal/rolecheck", base + "internal/internaltest", base + "internal/migrate", base + "internal/user", base + "internal/db/sqlcdb", base + "internal/app/testdata", base + "migrations":
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") && !strings.HasPrefix(name, "development_") {
			sources[relative] = nil
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]Statement{}
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, path), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		selected := sources[path]
		found := map[string]bool{}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			if selected != nil && !selected[name] {
				continue
			}
			found[name] = true
			symbol := name
			if fn.Recv != nil {
				typ := fn.Recv.List[0].Type
				if pointer, ok := typ.(*ast.StarExpr); ok {
					typ = pointer.X
				}
				switch generic := typ.(type) {
				case *ast.IndexExpr:
					typ = generic.X
				case *ast.IndexListExpr:
					typ = generic.X
				}
				id, ok := typ.(*ast.Ident)
				if !ok {
					t.Fatalf("unresolved receiver in %s", path)
				}
				symbol = id.Name + "." + name
			}
			ordinal := 0
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) < 2 {
					return true
				}
				method, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if method.Sel.Name != "Exec" && method.Sel.Name != "Query" && method.Sel.Name != "QueryRow" {
					return true
				}
				ordinal++
				variants := []map[string]string{{}}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					loop, ok := node.(*ast.RangeStmt)
					if !ok || call.Pos() < loop.Body.Pos() || call.End() > loop.Body.End() {
						return true
					}
					variable, ok := loop.Value.(*ast.Ident)
					if !ok {
						return true
					}
					used := false
					ast.Inspect(call.Args[1], func(n ast.Node) bool {
						if id, ok := n.(*ast.Ident); ok && id.Name == variable.Name {
							used = true
						}
						return true
					})
					if !used {
						return true
					}
					values, ok := loop.X.(*ast.CompositeLit)
					if !ok {
						t.Fatalf("unresolved SQL range in %s/%s", path, symbol)
					}
					expanded := []map[string]string{}
					for _, binding := range variants {
						for _, value := range values.Elts {
							next := map[string]string{}
							for k, v := range binding {
								next[k] = v
							}
							next[variable.Name] = sourceSQL(t, value, binding)
							expanded = append(expanded, next)
						}
					}
					variants = expanded
					return true
				})
				for i, binding := range variants {
					id := fmt.Sprintf("%s/%s/%d", path, symbol, ordinal)
					if len(variants) > 1 {
						id += fmt.Sprintf("/%d", i+1)
					}
					if _, exists := out[id]; exists {
						t.Fatalf("duplicate source statement %s", id)
					}
					out[id] = Statement{ID: id, SQL: sourceSQL(t, call.Args[1], binding), Bindings: binding}
				}
				return true
			})
		}
		for name := range selected {
			if !found[name] {
				t.Fatalf("source function absent: %s/%s", path, name)
			}
		}
	}
	return out
}

func sourceSQL(t *testing.T, expression ast.Expr, bindings map[string]string) string {
	t.Helper()
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind == token.STRING {
			text, err := strconv.Unquote(value.Value)
			if err != nil {
				t.Fatal(err)
			}
			return text
		}
	case *ast.BinaryExpr:
		if value.Op == token.ADD {
			return sourceSQL(t, value.X, bindings) + sourceSQL(t, value.Y, bindings)
		}
	case *ast.Ident:
		if text, ok := bindings[value.Name]; ok {
			return text
		}
	case *ast.CallExpr:
		if method, ok := value.Fun.(*ast.SelectorExpr); ok && method.Sel.Name == "table" && len(value.Args) == 1 {
			return pgx.Identifier{"users"}.Sanitize() + "." + sourceSQL(t, value.Args[0], bindings)
		}
	}
	t.Fatalf("unresolved runtime SQL expression: %T", expression)
	return ""
}
