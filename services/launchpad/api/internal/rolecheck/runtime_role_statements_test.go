package rolecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var runtimeRoles = []string{"api_runtime", "api_rest_runtime", "api_sockets_runtime"}

type statementResult struct {
	Statement string          `json:"statement"`
	Role      string          `json:"role"`
	Planned   bool            `json:"planned"`
	Executed  bool            `json:"executed"`
	Waiting   bool            `json:"waiting"`
	SQLState  string          `json:"sqlstate,omitempty"`
	Message   string          `json:"message,omitempty"`
	Plan      json.RawMessage `json:"plan,omitempty"`
}

type statementReport struct {
	Packages map[string]int    `json:"packages"`
	Results  []statementResult `json:"results"`
}

func TestRuntimeRoleStatements(t *testing.T) {
	root, err := filepath.Abs(coordinatortest.RepositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	statements, err := Statements()
	if err != nil {
		t.Fatal(err)
	}
	waiting, err := WaitingList()
	if err != nil {
		t.Fatal(err)
	}
	if len(waiting) != 0 {
		t.Fatal("every connected runtime statement requires an asserted plan")
	}
	var coverage Coverage
	if err = readDocument("coverage.json", &coverage); err != nil {
		t.Fatal(err)
	}
	if len(statements) == 0 || len(coverage.Sources) == 0 {
		t.Fatal("connected SQL inventory is empty")
	}
	report := statementReport{Packages: map[string]int{}, Results: []statementResult{}}
	if path := os.Getenv("RUNTIME_ROLE_STATEMENTS_REPORT"); path != "" {
		defer func() {
			raw, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				t.Error(err)
				return
			}
			if err = os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
				t.Error(err)
			}
		}()
	}

	var registry struct {
		Registrations []struct {
			Schema string `json:"schema"`
		} `json:"registrations"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "migrations/registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	schemas := map[string]bool{}
	for _, registration := range registry.Registrations {
		schemas[registration.Schema] = true
	}
	bindings := map[string]string{
		"s.config.StatsSchema": "stats", "s.config.ChartsSchema": "candles", "s.config.StreamSchema": "api_stream",
		"s.config.SourcePartitionSchema": "indexer", "a.config.SourceSchema": "indexer", "b.Schema": "api_limits",
		"partitionSchema": "indexer", "s.schema": `"api_stream"`, "g.schema": `"recovery"`,
		"commits": `"stats"."accounting_market_commits_v1"`,
	}
	for _, schema := range []string{"stats", "candles", "api_stream", "indexer", "api_limits", "recovery"} {
		if !schemas[schema] {
			t.Fatalf("statement schema %s has no registered migration", schema)
		}
	}

	sources := map[string]*sourceFile{}
	for _, scope := range coverage.Sources {
		source, err := openSource(root, scope.File)
		if err != nil {
			t.Fatal(err)
		}
		if sources[scope.File] != nil {
			t.Fatalf("repeated source scope %s", scope.File)
		}
		sources[scope.File] = source
	}
	listed := map[string]Statement{}
	expressions := map[string]bool{}
	queries := map[string]string{}
	for _, statement := range statements {
		if _, exists := listed[statement.ID]; exists {
			t.Fatalf("duplicate statement %s", statement.ID)
		}
		listed[statement.ID] = statement
		if statement.ID == "" || statement.Package != filepath.Dir(statement.File) || len(statement.Roles) == 0 || len(statement.Kinds) == 0 || statement.SQL == "" {
			t.Fatalf("incomplete statement %s", statement.ID)
		}
		for _, role := range statement.Roles {
			if !slices.Contains(runtimeRoles, role) {
				t.Fatalf("unknown runtime role %s", role)
			}
		}
		source := sources[statement.File]
		if source == nil {
			t.Fatalf("source scope absent: %s", statement.File)
		}
		fn := source.function(statement.Function)
		if fn == nil {
			t.Fatalf("function absent: %s", statement.ID)
		}
		expression := source.expression(fn, statement.Expression)
		if expression == nil {
			t.Fatalf("SQL expression differs from source: %s", statement.ID)
		}
		local := make(map[string]string, len(bindings)+len(statement.Bindings))
		for key, value := range bindings {
			local[key] = value
		}
		for key, value := range statement.Bindings {
			if key != "deliverySchema" || !schemas[value] {
				t.Fatalf("unregistered schema binding %s=%s", key, value)
			}
			local[key] = value
		}
		sql, err := source.evaluate(expression, source.definitions(fn), local, 0)
		if err != nil {
			t.Fatalf("%s: %v", statement.ID, err)
		}
		if sql != statement.SQL {
			t.Fatalf("SQL inventory differs from connected expression: %s", statement.ID)
		}
		queries[statement.ID] = sql
		expressions[statement.File+"\x00"+statement.Function+"\x00"+statement.Expression] = true
		report.Packages[statement.Package]++
	}
	for _, scope := range coverage.Sources {
		source := sources[scope.File]
		for _, declaration := range source.file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := functionName(fn)
			if len(scope.Functions) > 0 && !slices.Contains(scope.Functions, fn.Name.Name) {
				continue
			}
			if slices.Contains(scope.ExcludedFunctions, fn.Name.Name) {
				continue
			}
			for expression := range source.sqlExpressions(fn) {
				if !expressions[scope.File+"\x00"+name+"\x00"+expression] {
					t.Errorf("unlisted SQL in %s:%s: %s", scope.File, name, expression)
				}
			}
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	waitingByKey := map[string]WaitingStatement{}
	for _, entry := range waiting {
		statement, ok := listed[entry.Statement]
		if !ok || !slices.Contains(statement.Roles, entry.Role) {
			t.Fatalf("waiting statement has no matching inventory authority: %+v", entry)
		}
		key := entry.Statement + "/" + entry.Role
		if _, exists := waitingByKey[key]; exists {
			t.Fatalf("duplicate waiting statement %s", key)
		}
		if entry.Object == "" || entry.Right == "" || entry.SQLState == "" || entry.Message == "" {
			t.Fatalf("incomplete waiting statement %s", key)
		}
		waitingByKey[key] = entry
	}
	admin := os.Getenv("COORDINATOR_TEST_POSTGRES_DSN")
	if admin == "" {
		t.Fatal("COORDINATOR_TEST_POSTGRES_DSN is required for runtime statement plans")
	}
	database := coordinatortest.Bootstrap(t, admin, runtimeRoles...)
	connections := map[string]*pgx.Conn{}
	for _, role := range runtimeRoles {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		conn, err := pgx.Connect(ctx, database.Runtimes[role].DSN)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := conn.Close(ctx); err != nil {
				t.Error(err)
			}
		})
		var current string
		if err = conn.QueryRow(context.Background(), "SELECT current_user").Scan(&current); err != nil || current != database.Runtimes[role].Role {
			t.Fatalf("runtime login %s: %s, %v", role, current, err)
		}
		connections[role] = conn
	}
	for _, statement := range statements {
		for _, role := range statement.Roles {
			t.Run(statement.ID+"/"+role, func(t *testing.T) {
				result := statementResult{Statement: statement.ID, Role: role}
				_, result.Waiting = waitingByKey[statement.ID+"/"+role]
				defer func() { report.Results = append(report.Results, result) }()
				result.Plan, result.Executed, err = planStatement(connections[role], queries[statement.ID], statement.Kinds)
				result.Planned = err == nil && !result.Executed
				if err != nil {
					result.Message = err.Error()
					var pgerr *pgconn.PgError
					if errors.As(err, &pgerr) {
						result.SQLState = pgerr.Code
						result.Message = pgerr.Message
					}
					t.Logf("%s: %s: %s", role, result.SQLState, result.Message)
				}
				if result.Waiting {
					return
				}
				if err != nil {
					t.Errorf("runtime statement: %v", err)
					return
				}
				if !result.Executed {
					var plan []struct {
						Plan map[string]any `json:"Plan"`
					}
					if json.Unmarshal(result.Plan, &plan) != nil || len(plan) != 1 || len(plan[0].Plan) == 0 {
						t.Errorf("statement returned no plan: %s", result.Plan)
					}
				}
			})
		}
	}
	packages := make([]string, 0, len(report.Packages))
	for pkg := range report.Packages {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	for _, pkg := range packages {
		t.Logf("%s: %d statements", pkg, report.Packages[pkg])
	}
	planned, executed, refused := 0, 0, 0
	for _, result := range report.Results {
		if result.Planned {
			planned++
		} else if result.Executed && result.Message == "" {
			executed++
		} else {
			refused++
		}
	}
	t.Logf("statements=%d runtime_checks=%d planned=%d executed_in_rollback=%d refused=%d waiting=%d", len(statements), len(report.Results), planned, executed, refused, len(waiting))
}

var parameter = regexp.MustCompile(`\$[1-9][0-9]*`)

func planStatement(conn *pgx.Conn, sql string, kinds []string) (json.RawMessage, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	executed := slices.Contains(kinds, "SET")
	explain := "EXPLAIN (FORMAT JSON) "
	if parameter.MatchString(sql) {
		explain = "EXPLAIN (GENERIC_PLAN, FORMAT JSON) "
	}
	if executed {
		explain = ""
	}
	results, err := tx.Conn().PgConn().Exec(ctx, explain+sql).ReadAll()
	rollbackErr := tx.Rollback(ctx)
	if err != nil {
		return nil, executed, err
	}
	if rollbackErr != nil {
		return nil, executed, rollbackErr
	}
	if executed {
		return nil, true, nil
	}
	if len(results) != 1 || len(results[0].Rows) != 1 || len(results[0].Rows[0]) != 1 {
		return nil, false, fmt.Errorf("statement returned no single JSON plan")
	}
	return json.RawMessage(results[0].Rows[0][0]), false, nil
}
