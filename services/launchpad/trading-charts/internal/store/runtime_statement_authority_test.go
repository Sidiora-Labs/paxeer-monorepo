package store

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5/pgxpool"
)


func assertRuntimeStatementAuthority(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	root := coordinatortest.RepositoryRoot(t)
	charts := "services/launchpad/trading-charts/internal/"
	shared := "sdk/launchpad/shared/delivery/"
	// The legacy candle store and development sources run only in development.
	// Production uses ExactStore, the accounting consumer and ordered delivery.
	paths := []string{
		charts + "store/transactional.go", charts + "store/series.go",
		charts + "store/rebuild.go", charts + "store/rebuild_stream.go", charts + "store/retention.go",
		charts + "engine/exact_transaction.go", charts + "engine/series.go",
		charts + "engine/rollup.go", charts + "engine/candle_stream.go", charts + "engine/archive_parity.go",
		charts + "consumer/swap_consumer.go", charts + "consumer/source_bootstrap.go",
		charts + "consumer/source_authority.go", charts + "consumer/completeness.go", charts + "app/app.go",
		charts + "httpapi/symbols.go",
		shared + "store.go", shared + "partition.go", shared + "stream.go",
	}
	for _, path := range paths {
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, filepath.Join(root, path), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if path == shared+"store.go" && fn.Name.Name != "ConsumeOrdered" && fn.Name.Name != "QuarantineAfterRollback" {
				continue
			}
			if path == shared+"partition.go" && (fn.Name.Name == "AppendTx" || fn.Name.Name == "PartitionSchemaDDL") {
				continue
			}
			if path == shared+"stream.go" && fn.Name.Name != "AppendTx" {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) < 2 {
					return true
				}
				method, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (method.Sel.Name != "Exec" && method.Sel.Name != "Query" && method.Sel.Name != "QueryRow") {
					return true
				}
				predicates := []string{"consumer_id=$1 AND projection_generation=$2 AND partition_kind=$3 AND partition_key=$4"}
				if fn.Name.Name == "QuarantineAfterRollback" {
					predicates = append(predicates, "consumer_id=$1 AND projection_generation=$2")
				}
				for i, predicate := range predicates {
					schema := "candles"
					if path == charts+"consumer/source_authority.go" || path == charts+"httpapi/symbols.go" {
						schema = "indexer"
					}
					statement := runtimeStatementSQL(t, call.Args[1], schema, predicate)
					name := fmt.Sprintf("%s/%s/%d/%d", filepath.Base(path), fn.Name.Name, set.Position(call.Pos()).Line, i)
					t.Run(name, func(t *testing.T) {
						ctx := context.Background()
						tx, err := pool.Begin(ctx)
						if err != nil {
							t.Fatal(err)
						}
						defer func() {
							if err := tx.Rollback(ctx); err != nil {
								t.Error(err)
							}
						}()
						t.Logf("%s: %s", path, statement)
						if strings.HasPrefix(statement, "LOCK TABLE ") {
							if _, err := tx.Exec(ctx, statement); err != nil {
								t.Fatal(err)
							}
							return
						}
						conn := tx.Conn()
						plan, err := conn.Prepare(ctx, "", "EXPLAIN "+statement)
						if err != nil {
							t.Fatal(err)
						}
						rows, err := tx.Query(ctx, "EXPLAIN "+statement, make([]any, len(plan.ParamOIDs))...)
						if err != nil {
							t.Fatal(err)
						}
						rows.Close()
						if err := rows.Err(); err != nil {
							t.Fatal(err)
						}
					})
				}
				return true
			})
		}
	}
}

func TestRuntimeRoleSymbolSourceColumns(t *testing.T) {
	root := coordinatortest.RepositoryRoot(t)
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "services/launchpad/trading-charts/internal/httpapi/symbols.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	queryPattern := regexp.MustCompile(`(?i)^SELECT\s+([a-z_][a-z0-9_]*(?:\s*,\s*[a-z_][a-z0-9_]*)*)\s+FROM\s+indexer\.indexer_canonical_creations_v1\s+WHERE\s+([a-z_][a-z0-9_]*)\s*=\s*\$1$`)
	var columns []string
	statements := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "Resolve" || fn.Recv == nil || fn.Body == nil || len(fn.Recv.List) != 1 {
			continue
		}
		receiver := fn.Recv.List[0].Type
		if pointer, ok := receiver.(*ast.StarExpr); ok {
			receiver = pointer.X
		}
		name, ok := receiver.(*ast.Ident)
		if !ok || name.Name != "CanonicalPoolMetadataSource" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			method, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || method.Sel.Name != "QueryRow" || len(call.Args) < 2 {
				return true
			}
			statement := runtimeStatementSQL(t, call.Args[1], "indexer", "")
			parts := queryPattern.FindStringSubmatch(statement)
			if parts == nil {
				t.Fatalf("unsupported canonical symbol statement: %s", statement)
			}
			for _, column := range strings.Split(parts[1], ",") {
				columns = append(columns, strings.TrimSpace(column))
			}
			columns = append(columns, parts[2])
			statements++
			return true
		})
	}
	if statements != 1 || len(columns) == 0 {
		t.Fatalf("canonical symbol statements: got %d with columns %v", statements, columns)
	}
	raw, err := os.ReadFile(filepath.Join(root, "migrations/grants.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Roles []production.RoleManifest `json:"roles"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	var grants []production.Grant
	for _, role := range document.Roles {
		if role.RuntimeRole != "candles_runtime" {
			continue
		}
		for _, grant := range role.Grants {
			if grant.Schema == "indexer" && grant.Object == "indexer_canonical_creations_v1" {
				grants = append(grants, grant)
			}
		}
	}
	if len(grants) != 1 {
		t.Fatalf("canonical creations grants for candles_runtime: got %d", len(grants))
	}
	grant := grants[0]
	if !grant.View || grant.ObjectKind != "view" || grant.GrantOption || len(grant.Privileges) != 1 || grant.Privileges[0] != "SELECT" || len(grant.Columns) == 0 {
		t.Fatalf("canonical creations requires a column-scoped SELECT grant: %+v", grant)
	}
	for _, column := range columns {
		found := false
		for _, allowed := range grant.Columns {
			found = found || column == allowed
		}
		if !found {
			t.Errorf("candles_runtime canonical creations grant omits symbol source column %q", column)
		}
	}
}

func runtimeStatementSQL(t *testing.T, expr ast.Expr, schema, predicate string) string {
	t.Helper()
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			s, err := strconv.Unquote(v.Value)
			if err != nil {
				t.Fatal(err)
			}
			return s
		}
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			return runtimeStatementSQL(t, v.X, schema, predicate) + runtimeStatementSQL(t, v.Y, schema, predicate)
		}
	case *ast.Ident:
		switch v.Name {
		case "q", "schema":
			return schema
		case "predicate", "consumerPredicate":
			return predicate
		}
	case *ast.SelectorExpr:
		if v.Sel.Name == "Schema" {
			return "candles"
		}
	case *ast.CallExpr:
		if method, ok := v.Fun.(*ast.SelectorExpr); ok {
			if method.Sel.Name == "table" && len(v.Args) == 1 {
				return "candles." + runtimeStatementSQL(t, v.Args[0], schema, predicate)
			}
			if method.Sel.Name == "Sanitize" {
				if id, ok := method.X.(*ast.CompositeLit); ok && len(id.Elts) == 2 {
					first, schemaOK := id.Elts[0].(*ast.SelectorExpr)
					second, functionOK := id.Elts[1].(*ast.Ident)
					if schemaOK && functionOK && first.Sel.Name == "Schema" && second.Name == "function" {
						return "api_stream.append_candle"
					}
				}
			}
		}
	}
	t.Fatalf("unresolved runtime SQL expression: %T at %d", expr, expr.Pos())
	return ""
}
