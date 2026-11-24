// Package rolecheck lists the SQL used by the livestream runtime pool.
package rolecheck

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const RuntimeRole = "livestream_runtime"
const serviceInternal = "services/launchpad/livestream/internal"
const sharedRoot = "sdk/launchpad/shared"

// Statement identifies a call site and the objects named by its SQL. Sequences
// include the defaults of inserted serial columns; columns include CTE columns.
type Statement struct {
	ID        string   `json:"id"`
	Package   string   `json:"package"`
	File      string   `json:"file"`
	Function  string   `json:"function"`
	Ordinal   int      `json:"ordinal"`
	Role      string   `json:"role"`
	Kind      string   `json:"kind"`
	Tables    []string `json:"tables"`
	Sequences []string `json:"sequences"`
	Functions []string `json:"functions"`
	Columns   []string `json:"columns"`
	SHA256    string   `json:"sha256"`
	SQL       string   `json:"-"`
}

//go:embed statements.json
var inventoryJSON []byte

// These are the shared database functions called by Config.CheckRuntimeSchema
// and by the connected runtime's operation ledger.
var sharedFunctions = map[string]map[string]bool{
	sharedRoot + "/auth/ledger.go":              {"Ledger.ExecuteValidatedTx": true, "AppendAudit": true},
	sharedRoot + "/db/production/migrations.go": {"CatalogFingerprint": true, "CheckReadiness": true, "checkPrivilege": true, "checkUnexpectedPrivileges": true},
}

// Statements reads SQL from the service's call expressions and constants. The
// inventory binds each object's description to those exact statement bytes.
func Statements(root, schema string) ([]Statement, error) {
	var inventory []Statement
	if err := json.Unmarshal(inventoryJSON, &inventory); err != nil {
		return nil, err
	}
	files := []string{}
	err := filepath.WalkDir(filepath.Join(root, serviceInternal), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "rolecheck" || d.Name() == "sqlcdb" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for path := range sharedFunctions {
		files = append(files, path)
	}
	sort.Strings(files)
	fset := token.NewFileSet()
	parsed := map[string]*ast.File{}
	constants := map[string]map[string]ast.Expr{}
	for _, path := range files {
		file, err := parser.ParseFile(fset, filepath.Join(root, path), nil, 0)
		if err != nil {
			return nil, err
		}
		parsed[path] = file
		pkg := filepath.ToSlash(filepath.Dir(path))
		if constants[pkg] == nil {
			constants[pkg] = map[string]ast.Expr{}
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value := spec.(*ast.ValueSpec)
				for i, name := range value.Names {
					if i < len(value.Values) {
						constants[pkg][name.Name] = value.Values[i]
					}
				}
			}
		}
	}
	actual := map[string]string{}
	for _, path := range files {
		pkg := filepath.ToSlash(filepath.Dir(path))
		for _, decl := range parsed[path].Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			name := functionName(function)
			if allowed, shared := sharedFunctions[path]; shared && !allowed[name] {
				continue
			}
			ordinal := 0
			var scanErr error
			ast.Inspect(function.Body, func(n ast.Node) bool {
				if scanErr != nil {
					return false
				}
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) < 2 {
					return true
				}
				method, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (method.Sel.Name != "Exec" && method.Sel.Name != "Query" && method.Sel.Name != "QueryRow") {
					return true
				}
				ordinal++
				id := strings.TrimSuffix(path, ".go") + ":" + name + ":" + strconv.Itoa(ordinal)
				sql, err := sqlText(call.Args[1], constants[pkg], schema)
				if err != nil {
					scanErr = fmt.Errorf("%s: %w", id, err)
					return false
				}
				actual[id] = sql
				return true
			})
			if scanErr != nil {
				return nil, scanErr
			}
		}
	}
	seen := map[string]bool{}
	for i := range inventory {
		statement := &inventory[i]
		id := strings.TrimSuffix(statement.File, ".go") + ":" + statement.Function + ":" + strconv.Itoa(statement.Ordinal)
		if statement.ID != id || seen[id] || statement.Role != RuntimeRole || statement.Package != filepath.ToSlash(filepath.Dir(statement.File)) {
			return nil, fmt.Errorf("invalid statement identity %s", statement.ID)
		}
		seen[id] = true
		sql, ok := actual[id]
		if !ok {
			return nil, fmt.Errorf("statement call site absent: %s", id)
		}
		digest := sha256.Sum256([]byte(sql))
		if hex.EncodeToString(digest[:]) != statement.SHA256 {
			return nil, fmt.Errorf("statement objects must describe current SQL: %s", id)
		}
		switch statement.Kind {
		case "SELECT", "INSERT", "UPDATE", "DELETE", "SET":
		default:
			return nil, fmt.Errorf("statement kind absent: %s", id)
		}
		statement.SQL = sql
		delete(actual, id)
	}
	if len(actual) > 0 {
		missing := make([]string, 0, len(actual))
		for id := range actual {
			missing = append(missing, id)
		}
		sort.Strings(missing)
		return nil, fmt.Errorf("statements need object descriptions: %s", strings.Join(missing, ", "))
	}
	if len(inventory) == 0 {
		return nil, fmt.Errorf("runtime statement inventory is empty")
	}
	return inventory, nil
}

func functionName(function *ast.FuncDecl) string {
	name := function.Name.Name
	if function.Recv != nil {
		receiver := function.Recv.List[0].Type
		if ptr, ok := receiver.(*ast.StarExpr); ok {
			receiver = ptr.X
		}
		if ident, ok := receiver.(*ast.Ident); ok {
			name = ident.Name + "." + name
		}
	}
	return name
}

func sqlText(expr ast.Expr, constants map[string]ast.Expr, schema string) (string, error) {
	switch value := expr.(type) {
	case *ast.BasicLit:
		if value.Kind == token.STRING {
			return strconv.Unquote(value.Value)
		}
	case *ast.Ident:
		if constant, ok := constants[value.Name]; ok {
			return sqlText(constant, constants, schema)
		}
		if value.Name == "schema" {
			return schema, nil
		}
	case *ast.BinaryExpr:
		if value.Op == token.ADD {
			left, err := sqlText(value.X, constants, schema)
			if err != nil {
				return "", err
			}
			right, err := sqlText(value.Y, constants, schema)
			return left + right, err
		}
	case *ast.SelectorExpr:
		if value.Sel.Name == "schema" {
			return schema, nil
		}
	case *ast.CallExpr:
		if method, ok := value.Fun.(*ast.SelectorExpr); ok && method.Sel.Name == "Sprintf" {
			if pkg, ok := method.X.(*ast.Ident); ok && pkg.Name == "fmt" && len(value.Args) > 0 {
				format, err := sqlText(value.Args[0], constants, schema)
				if err != nil {
					return "", err
				}
				args := make([]any, 0, len(value.Args)-1)
				for _, arg := range value.Args[1:] {
					text, err := sqlText(arg, constants, schema)
					if err != nil {
						return "", err
					}
					args = append(args, text)
				}
				rendered := fmt.Sprintf(format, args...)
				if strings.Contains(rendered, "%!") {
					return "", fmt.Errorf("invalid SQL format")
				}
				return rendered, nil
			}
		}
	}
	return "", fmt.Errorf("SQL expression %T requires an explicit binding", expr)
}

func registeredSchemas(root string) (map[string]string, error) {
	//nolint:gosec // The repository root supplies the fixed migration registry path.
	raw, err := os.ReadFile(filepath.Join(root, "migrations/registry.json"))
	if err != nil {
		return nil, err
	}
	var registry struct {
		Registrations []struct {
			Service string `json:"service"`
			Schema  string `json:"schema"`
		} `json:"registrations"`
	}
	if err = json.Unmarshal(raw, &registry); err != nil {
		return nil, err
	}
	schemas := map[string]string{}
	for _, migration := range registry.Registrations {
		if old, ok := schemas[migration.Service]; ok && old != migration.Schema {
			return nil, fmt.Errorf("service %s has multiple schemas", migration.Service)
		}
		schemas[migration.Service] = migration.Schema
	}
	for service, want := range map[string]string{"media/livestream": "livestream", "media/social": "chat", "core/indexer": "indexer", "core/stats-workers": "stats"} {
		if schemas[service] != want {
			return nil, fmt.Errorf("runtime schema binding differs for %s", service)
		}
	}
	return schemas, nil
}
