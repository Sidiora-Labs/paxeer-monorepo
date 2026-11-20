package analytics_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5/pgxpool"
)

type runtimeStatement struct {
	RuntimeRole  string   `json:"runtime_role"`
	ID           string   `json:"id"`
	File         string   `json:"file"`
	Function     string   `json:"function"`
	Call         int      `json:"call"`
	Expression   string   `json:"expression"`
	SQL          string   `json:"sql"`
	SourceSHA256 string   `json:"source_sha256"`
	Kind         string   `json:"kind"`
	Tables       []string `json:"tables"`
	Sequences    []string `json:"sequences"`
	Functions    []string `json:"functions"`
	Boundary     string   `json:"boundary"`
}

func runtimeFunctionName(fn *ast.FuncDecl) string {
	if fn.Recv == nil {
		return fn.Name.Name
	}
	typ := fn.Recv.List[0].Type
	if ptr, ok := typ.(*ast.StarExpr); ok {
		typ = ptr.X
	}
	return typ.(*ast.Ident).Name + "." + fn.Name.Name
}

func validateRuntimeInventory(t *testing.T, statements, boundaries []runtimeStatement) {
	t.Helper()
	root := coordinatortest.RepositoryRoot(t)
	covered := map[string]bool{}
	files := map[string]bool{}
	for _, s := range append(append([]runtimeStatement{}, statements...), boundaries...) {
		key := fmt.Sprintf("%s:%s:%d", s.File, s.Function, s.Call)
		covered[key] = true
		files[s.File] = true
		if s.Boundary != "" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, s.File))
		if err != nil {
			t.Fatal(err)
		}
		if actual := fmt.Sprintf("%x", sha256.Sum256(raw)); actual != s.SourceSHA256 {
			t.Errorf("%s source differs from the runtime statement inventory", s.File)
		}
		if s.ID == "" || s.SQL == "" || s.Kind == "" {
			t.Errorf("incomplete runtime statement: %+v", s)
		}
	}
	err := filepath.WalkDir(filepath.Join(root, "services/launchpad/stats-workers"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files[relative] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for file := range files {
		set := token.NewFileSet()
		parsed, err := parser.ParseFile(set, filepath.Join(root, file), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ordinal := 0
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) < 2 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "Exec", "Query", "QueryRow", "CopyFrom":
				default:
					return true
				}
				ordinal++
				key := fmt.Sprintf("%s:%s:%d", file, runtimeFunctionName(fn), ordinal)
				if !covered[key] {
					t.Errorf("runtime SQL call requires an inventory entry: %s", key)
				}
				return true
			})
		}
	}
}

func assertRuntimeStatements(t *testing.T, ctx context.Context, pool *pgxpool.Pool, rolePools map[string]*pgxpool.Pool) {
	t.Helper()
	raw, err := os.ReadFile("runtime_statements.json")
	if err != nil {
		t.Fatal(err)
	}
	var inventory struct {
		Statements []runtimeStatement `json:"statements"`
		Boundaries []runtimeStatement `json:"boundaries"`
	}
	if err := json.Unmarshal(raw, &inventory); err != nil {
		t.Fatal(err)
	}
	if len(inventory.Statements) == 0 {
		t.Fatal("runtime statement inventory is empty")
	}
	validateRuntimeInventory(t, inventory.Statements, inventory.Boundaries)
	for _, s := range inventory.Statements {
		t.Run(s.ID, func(t *testing.T) {
			statementPool := pool
			if s.RuntimeRole != "" {
				var ok bool
				statementPool, ok = rolePools[s.RuntimeRole]
				if !ok || statementPool == nil {
					t.Fatalf("runtime statement role unavailable: %s", s.RuntimeRole)
				}
			}
			tx, err := statementPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, "SET LOCAL search_path = pg_catalog, stats"); err != nil {
				t.Fatal(err)
			}
			if s.Kind == "SET" || s.Kind == "SAVEPOINT" || s.Kind == "ROLLBACK" {
				if s.Kind == "ROLLBACK" {
					if _, err := tx.Exec(ctx, "SAVEPOINT holdings_apply"); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := tx.Exec(ctx, s.SQL); err != nil {
					t.Fatalf("%s %s: %v", s.File, s.Function, err)
				}
				return
			}
			options := "FORMAT TEXT"
			if strings.Contains(s.SQL, "$") {
				options += ", GENERIC_PLAN"
			}
			results, err := tx.Conn().PgConn().Exec(ctx, "EXPLAIN ("+options+") "+s.SQL).ReadAll()
			if err != nil {
				t.Fatalf("%s %s: %s: %v", s.File, s.Function, s.SQL, err)
			}
			lines := 0
			for _, result := range results {
				lines += len(result.Rows)
			}
			if lines == 0 {
				t.Fatal("runtime statement returned no plan")
			}
			for _, seq := range s.Sequences {
				var allowed bool
				if err := tx.QueryRow(ctx, "SELECT has_sequence_privilege(current_user,$1,'USAGE')", seq).Scan(&allowed); err != nil || !allowed {
					t.Fatalf("%s sequence usage: %v", seq, err)
				}
			}
			for _, function := range s.Functions {
				parts := strings.Split(function, ".")
				if len(parts) != 2 {
					continue
				}
				var allowed bool
				if err := tx.QueryRow(ctx, `SELECT COALESCE(bool_and(has_function_privilege(current_user,p.oid,'EXECUTE')),false) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname=$1 AND p.proname=$2`, parts[0], parts[1]).Scan(&allowed); err != nil || !allowed {
					t.Fatalf("%s function execution: %v", function, err)
				}
			}
		})
	}
}
