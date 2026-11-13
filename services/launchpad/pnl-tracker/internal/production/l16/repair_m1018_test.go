package l16

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func repairM1018Inputs(t *testing.T) (string, production.MigrationRegistry, production.RoleManifest) {
	t.Helper()
	root := coordinatortest.RepositoryRoot(t)
	dir := filepath.Join(root, "migrations")
	raw, err := os.ReadFile(filepath.Join(dir, "registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := production.LoadMigrationRegistry(raw, production.Checksum(raw))
	if err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(dir, "grants.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Roles []production.RoleManifest `json:"roles"`
	}
	if err = json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	var matches []production.RoleManifest
	for _, role := range document.Roles {
		if role.RuntimeRole == "pnl_runtime" && role.Service == "core/pnl-tracker" {
			matches = append(matches, role)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("want one production PnL manifest, got %d", len(matches))
	}
	return root, registry, matches[0]
}

// The fixture itself is the component under repair. Bind its role request and
// consumer DSN to Bootstrap, so a partial local grant helper cannot return.
func TestRepairM_1018_CoordinatorRuntimeWiring(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "baseline_ahead_postgres_test.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var loader *ast.FuncDecl
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok {
			if function.Name.Name == "reviewedPnlRuntime" {
				t.Fatal("partial runtime grant helper remains")
			}
			if function.Name.Name == "loadRealMarketStage" {
				loader = function
			}
		}
	}
	if loader == nil {
		t.Fatal("real market loader is missing")
	}
	bootstraps, runtimeDSNs := 0, 0
	ast.Inspect(loader.Body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
				if owner, ok := selector.X.(*ast.Ident); ok && owner.Name == "coordinatortest" && selector.Sel.Name == "Bootstrap" {
					bootstraps++
					var roles []string
					for _, argument := range call.Args[2:] {
						literal, ok := argument.(*ast.BasicLit)
						if !ok || literal.Kind != token.STRING {
							t.Fatal("runtime role request must name reviewed roles")
						}
						role, err := strconv.Unquote(literal.Value)
						if err != nil {
							t.Fatal(err)
						}
						roles = append(roles, role)
					}
					if !reflect.DeepEqual(roles, []string{"indexer_runtime", "pnl_runtime"}) {
						t.Fatalf("coordinator must plan and provision both real roles: %v", roles)
					}
				}
			}
		}
		if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "DSN" {
			if index, ok := selector.X.(*ast.IndexExpr); ok {
				if runtimes, ok := index.X.(*ast.SelectorExpr); ok && runtimes.Sel.Name == "Runtimes" {
					if owner, ok := runtimes.X.(*ast.Ident); ok && owner.Name == "db" {
						if role, ok := index.Index.(*ast.BasicLit); ok && role.Value == `"pnl_runtime"` {
							runtimeDSNs++
						}
					}
				}
			}
		}
		return true
	})
	if bootstraps != 1 || runtimeDSNs != 1 {
		t.Fatalf("fixture must consume the coordinator login: bootstraps=%d pnl DSNs=%d", bootstraps, runtimeDSNs)
	}
}

func TestRepairM_1018_ProductionGrantRendering(t *testing.T) {
	_, _, role := repairM1018Inputs(t)
	raw, err := json.Marshal(role)
	if err != nil {
		t.Fatal(err)
	}
	var decoded production.RoleManifest
	if err = json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(role, decoded) || len(decoded.Grants) == 0 {
		t.Fatal("production role codec lost grants")
	}
	statements, err := decoded.GrantSQL()
	if err != nil {
		t.Fatalf("production runtime grants must all render: %v", err)
	}
	// Each individual production grant must survive the aggregate renderer,
	// including foreign schema USAGE and function/sequence privileges.
	expected := map[string]bool{}
	for _, grant := range decoded.Grants {
		individual := decoded
		individual.Grants, individual.OwnerReadGrants = []production.Grant{grant}, nil
		rendered, err := individual.GrantSQL()
		if err != nil {
			t.Fatalf("production grant %s.%s: %v", grant.Schema, grant.Object, err)
		}
		for _, statement := range rendered {
			expected[statement] = true
		}
	}
	if len(decoded.OwnerReadGrants) != 0 {
		t.Fatal("PnL runtime fixture must not discard newly introduced owner interfaces")
	}
	if len(statements) != len(expected) {
		t.Fatalf("grant renderer omitted or duplicated statements: got %d want %d", len(statements), len(expected))
	}
	for _, statement := range statements {
		if !expected[statement] {
			t.Fatalf("unexpected production grant: %s", statement)
		}
		delete(expected, statement)
	}
	if len(expected) != 0 {
		t.Fatalf("omitted production grants: %v", expected)
	}
	for _, privilege := range []string{"ALL", "CREATE", "TRUNCATE"} {
		t.Run(privilege, func(t *testing.T) {
			invalid := decoded
			invalid.Grants = append([]production.Grant(nil), decoded.Grants...)
			invalid.Grants[0].Privileges = []string{privilege}
			if sql, err := invalid.GrantSQL(); err == nil || len(sql) != 0 {
				t.Fatalf("invalid production grant returned usable SQL: %v, %v", sql, err)
			}
		})
	}
}

func TestRepairM_1018_CompleteDependencyMigrations(t *testing.T) {
	root, registry, role := repairM1018Inputs(t)
	catalog, err := registry.ValidateOrder()
	if err != nil {
		t.Fatal(err)
	}
	schemas := map[string]bool{role.Schema: true, "indexer": true}
	for _, grant := range role.Grants {
		if grant.Schema != "migration_control" {
			schemas[grant.Schema] = true
		}
	}
	needed := map[int64]bool{}
	var include func(int64)
	include = func(id int64) {
		if needed[id] {
			return
		}
		migration, ok := catalog[id]
		if !ok {
			t.Fatalf("unregistered dependency %d", id)
		}
		needed[id] = true
		for _, predecessor := range migration.PredecessorIDs {
			include(predecessor)
		}
	}
	for _, migration := range registry.Registrations {
		if schemas[migration.Schema] {
			include(migration.MigrationID)
		}
	}
	// Read the exact checksum-bound inputs consumed by the coordinator, in its
	// approved order. This is an offline contract check, not a SQL executor.
	applied := map[int64]string{}
	sources := map[string]string{"migration_control": production.MigrationControlDDL()}
	for _, id := range registry.ApplicationOrder {
		if !needed[id] {
			continue
		}
		migration := catalog[id]
		raw, err := os.ReadFile(filepath.Join(root, migration.Path))
		if err != nil {
			t.Fatal(err)
		}
		// Bootstrap's readBound admits these SQL inputs by their registered
		// checksum; release-admission metadata is not part of that interface.
		if digest := production.Checksum(raw); digest != migration.SHA256 {
			t.Fatalf("coordinator dependency %d checksum: got %s want %s", id, digest, migration.SHA256)
		}
		for _, predecessor := range migration.PredecessorIDs {
			if applied[predecessor] != catalog[predecessor].SHA256 {
				t.Fatalf("coordinator dependency %d precedes registered predecessor %d", id, predecessor)
			}
		}
		applied[id] = migration.SHA256
		sources[migration.Schema] += "\n" + string(raw)
	}
	for _, id := range []int64{60000, 160001, 160004, 180000, 180001, 150002, 440000} {
		if applied[id] == "" {
			t.Errorf("complete runtime plan omitted dependency %d", id)
		}
	}
	for _, grant := range role.Grants {
		kind := grant.ObjectKind
		if kind == "" {
			kind = "table"
		}
		// Accept qualified or search_path-relative declarations, preserving
		// the schema boundary. Sequence grants must name an actual serial
		// column or an explicit CREATE SEQUENCE, never an invented sequence.
		name := `(?:"?` + regexp.QuoteMeta(grant.Schema) + `"?\s*\.\s*)?"?` + regexp.QuoteMeta(grant.Object) + `"?\s*\(`
		pattern := `(?i)\bCREATE\s+(?:OR\s+REPLACE\s+)?` + kind + `\s+(?:IF\s+NOT\s+EXISTS\s+)?` + name
		if kind == "view" {
			pattern = strings.TrimSuffix(pattern, `\s*\(`) + `(?:\s|\()`
		}
		if kind == "sequence" {
			pattern = strings.TrimSuffix(pattern, `\s*\(`) + `(?:\s|;)`
			parts := strings.Split(strings.TrimSuffix(grant.Object, "_seq"), "_")
			column := parts[len(parts)-1]
			table := strings.Join(parts[:len(parts)-1], "_")
			serial := `(?is)\bCREATE\s+TABLE\s+"?` + regexp.QuoteMeta(table) + `"?\s*\([^;]*\b` + regexp.QuoteMeta(column) + `\s+(?:bigserial|serial|smallserial)\b`
			pattern = `(?:` + pattern + `)|(?:` + serial + `)`
		}
		if !regexp.MustCompile(pattern).MatchString(sources[grant.Schema]) {
			t.Errorf("production grant has no declaration in its complete registered schema plan: %s %s.%s", kind, grant.Schema, grant.Object)
		}
	}
}
