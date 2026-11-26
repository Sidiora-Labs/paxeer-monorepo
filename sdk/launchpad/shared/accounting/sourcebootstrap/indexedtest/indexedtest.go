// Package indexedtest checks source bootstrap statements against registered
// PostgreSQL schemas and generated source histories.
package indexedtest

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const Stream = "indexed-commits"

// Fixture uses the coordinator's registered schema and a runtime login.
type Fixture struct {
	Admin, Runtime *pgxpool.Pool
}

func Open(t testing.TB, environment, role string) *Fixture {
	t.Helper()
	admin := os.Getenv(environment)
	if admin == "" {
		t.Fatalf("%s is required for indexed commit proofs", environment)
	}
	db := coordinatortest.Bootstrap(t, admin, role)
	open := func(dsn string) *pgxpool.Pool {
		pool, err := pgxpool.New(context.Background(), dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		return pool
	}
	f := &Fixture{Admin: open(db.AdminDSN), Runtime: open(db.Runtimes[role].DSN)}
	var current string
	if err := f.Runtime.QueryRow(context.Background(), "SELECT current_user").Scan(&current); err != nil || current != db.Runtimes[role].Role {
		t.Fatalf("runtime login: %s %v", current, err)
	}
	return f
}

func (f *Fixture) Exec(t testing.TB, sql string, args ...any) {
	t.Helper()
	if _, err := f.Admin.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (f *Fixture) Seed(t testing.TB, commits, width int64) {
	t.Helper()
	f.Exec(t, `TRUNCATE indexer.indexer_ingest_commits, indexer.indexer_block_completeness, indexer.indexer_raw_logs CASCADE`)
	f.Exec(t, `INSERT INTO indexer.indexer_ingest_commits(chain_id,stream_id,batch_id,previous_height,last_complete_height,block_hash,record_count,outbox_count)
 SELECT 125,$1,'commit-'||h,(h-1)*$3,h*$3,'block-'||(h*$3),((h*$3)/7-((h-1)*$3)/7)::integer,0 FROM generate_series(1,$2::bigint) h`, Stream, commits, width)
}

func (f *Fixture) Blocks(t testing.TB, from, to int64) {
	t.Helper()
	f.Exec(t, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete)
 SELECT 125,$1,h,'block-'||h,CASE WHEN h%7=0 THEN 1 ELSE 0 END,true,true FROM generate_series($2::bigint+1,$3::bigint) h`, Stream, from, to)
	f.Exec(t, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data)
 SELECT 125,'tx-'||h,0,h,0,'block-'||h,'emitter','["event","pool"]'::jsonb,''::bytea FROM generate_series($1::bigint+1,$2::bigint) h WHERE h%7=0`, from, to)
}

// Statement reads the SQL expression executed by the named source function.
func Statement(t testing.TB, file, function string) string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(coordinatortest.RepositoryRoot(t), file), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var evaluate func(ast.Expr) string
	evaluate = func(expr ast.Expr) string {
		switch v := expr.(type) {
		case *ast.BasicLit:
			value, err := strconv.Unquote(v.Value)
			if err != nil {
				t.Fatal(err)
			}
			return value
		case *ast.Ident:
			if v.Obj != nil {
				if declaration, ok := v.Obj.Decl.(*ast.ValueSpec); ok {
					for i, name := range declaration.Names {
						if name.Name == v.Name {
							return evaluate(declaration.Values[i])
						}
					}
				}
			}
		}
		t.Fatalf("SQL expression is not a string constant: %T", expr)
		return ""
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != function {
			continue
		}
		var statement string
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			method, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || method.Sel.Name != "QueryRow" {
				return true
			}
			candidate := evaluate(call.Args[1])
			if strings.Contains(candidate, "indexer_ingest_commits_v1") {
				if statement != "" {
					t.Fatalf("multiple commit statements in %s", function)
				}
				statement = candidate
			}
			return true
		})
		if statement == "" {
			t.Fatalf("commit statement missing in %s", function)
		}
		return statement
	}
	t.Fatalf("function missing: %s", function)
	return ""
}

func References(t testing.TB) map[string]string {
	t.Helper()
	raw, err := os.ReadFile("testdata/bootstrap_statements.json")
	if err != nil {
		t.Fatal(err)
	}
	var statements map[string]string
	if err := json.Unmarshal(raw, &statements); err != nil {
		t.Fatal(err)
	}
	return statements
}

func results(t testing.TB, pool *pgxpool.Pool, sql string, args ...any) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), sql, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, string(raw))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(result)
	return result
}

func Equal(t testing.TB, pool *pgxpool.Pool, current, reference string, args ...any) {
	t.Helper()
	want, got := results(t, pool, reference, args...), results(t, pool, current, args...)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statement result: got %v, want %v; args %v", got, want, args)
	}
}

// Histories compares ranges before, at, across and after every source fault.
func Histories(t *testing.T, f *Fixture, compare func(*testing.T, int64, int64)) {
	t.Helper()
	faults := []struct{ name, sql string }{
		{"contiguous", ""},
		{"gap", `DELETE FROM indexer.indexer_ingest_commits WHERE batch_id='commit-8'`},
		{"overlap", `UPDATE indexer.indexer_ingest_commits SET previous_height=26 WHERE batch_id='commit-8'`},
		{"empty", `UPDATE indexer.indexer_ingest_commits SET last_complete_height=previous_height WHERE batch_id='commit-8'`},
		{"reversed", `UPDATE indexer.indexer_ingest_commits SET last_complete_height=previous_height-5 WHERE batch_id='commit-8'`},
		{"below_replay", `UPDATE indexer.indexer_ingest_commits SET previous_height=-2 WHERE batch_id='commit-8'`},
		{"above_first", `UPDATE indexer.indexer_ingest_commits SET last_complete_height=100 WHERE batch_id='commit-8'`},
		{"block_hash", `UPDATE indexer.indexer_ingest_commits SET block_hash='other-block' WHERE batch_id='commit-8'`},
		{"missing_block", `DELETE FROM indexer.indexer_block_completeness WHERE height=30`},
		{"record_count", `UPDATE indexer.indexer_ingest_commits SET record_count=2 WHERE batch_id='commit-8'`},
		{"duplicate_range", `INSERT INTO indexer.indexer_ingest_commits SELECT chain_id,stream_id,'duplicate',previous_height,last_complete_height,block_hash,record_count,outbox_count,commit_time FROM indexer.indexer_ingest_commits WHERE batch_id='commit-8'`},
	}
	for _, fault := range faults {
		t.Run(fault.name, func(t *testing.T) {
			f.Seed(t, 16, 4)
			f.Blocks(t, 0, 64)
			f.Exec(t, `UPDATE indexer.indexer_ingest_commits a SET record_count=(SELECT COALESCE(sum(raw_count),0) FROM indexer.indexer_block_completeness b WHERE b.height>a.previous_height AND b.height<=a.last_complete_height)`)
			if fault.sql != "" {
				f.Exec(t, fault.sql)
			}
			for _, bounds := range [][2]int64{{16, 20}, {24, 28}, {28, 32}, {32, 36}, {36, 40}, {20, 44}, {29, 31}, {30, 34}, {0, 64}} {
				t.Run(fmt.Sprintf("%d-%d", bounds[0], bounds[1]), func(t *testing.T) { compare(t, bounds[0], bounds[1]) })
			}
		})
	}
}

// Plan is the row accounting reported by EXPLAIN ANALYZE.
type Plan struct {
	NodeType  string  `json:"Node Type"`
	Relation  string  `json:"Relation Name"`
	Index     string  `json:"Index Name"`
	Rows      float64 `json:"Actual Rows"`
	Loops     float64 `json:"Actual Loops"`
	Removed   float64 `json:"Rows Removed by Filter"`
	Rechecked float64 `json:"Rows Removed by Index Recheck"`
	Plans     []Plan  `json:"Plans"`
}

type Explanation struct {
	Plan         Plan    `json:"Plan"`
	Milliseconds float64 `json:"Execution Time"`
}

func Explain(t testing.TB, pool *pgxpool.Pool, sql string, args ...any) Explanation {
	t.Helper()
	var raw []byte
	if err := pool.QueryRow(context.Background(), "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+sql, args...).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var plans []Explanation
	if err := json.Unmarshal(raw, &plans); err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 {
		t.Fatalf("EXPLAIN returned %d plans", len(plans))
	}
	t.Logf("execution %.3f ms", plans[0].Milliseconds)
	return plans[0]
}

// AssertIndexed includes filtered entries and bitmap index entries in the bound.
func AssertIndexed(t testing.TB, plan Plan, maximum float64) {
	t.Helper()
	var visited float64
	var indexes int
	var walk func(Plan)
	walk = func(node Plan) {
		if node.Relation == "indexer_ingest_commits" {
			if strings.Contains(node.NodeType, "Seq Scan") {
				t.Fatal("sequential commit scan")
			}
			switch node.NodeType {
			case "Index Scan", "Index Only Scan":
				indexes++
				visited += (node.Rows + node.Removed + node.Rechecked) * node.Loops
			case "Bitmap Heap Scan":
				visited += (node.Removed + node.Rechecked) * node.Loops
			default:
				t.Fatalf("unexpected commit access: %s", node.NodeType)
			}
		}
		if node.NodeType == "Bitmap Index Scan" && strings.HasPrefix(node.Index, "indexer_ingest_commits") {
			indexes++
			visited += (node.Rows + node.Removed) * node.Loops
		}
		for _, child := range node.Plans {
			walk(child)
		}
	}
	walk(plan)
	if indexes == 0 || visited > maximum {
		t.Fatalf("commit index reads %.0f, bound %.0f, indexed nodes %d; plan %+v", visited, maximum, indexes, plan)
	}
}

func Rows(plan Plan) float64 {
	count := (plan.Rows + plan.Removed + plan.Rechecked) * plan.Loops
	for _, child := range plan.Plans {
		count += Rows(child)
	}
	return count
}

func (f *Fixture) Analyze(t testing.TB) {
	t.Helper()
	for _, table := range []string{"indexer_ingest_commits", "indexer_block_completeness", "indexer_raw_logs"} {
		f.Exec(t, "VACUUM (ANALYZE) indexer."+pgx.Identifier{table}.Sanitize())
	}
}

func (f *Fixture) Million(t testing.TB) {
	t.Helper()
	f.Seed(t, 1000000, 1)
	for _, from := range []int64{0, 498000, 995904} {
		f.Blocks(t, from, from+4096)
	}
	f.Analyze(t)
}

func (f *Fixture) WithoutHeightIndex(t *testing.T, measure func(*testing.T)) {
	t.Helper()
	f.Exec(t, `DROP INDEX indexer.indexer_ingest_commits_height`)
	t.Run("before", measure)
	sql, err := coordinatortest.ReadMigrationSource(coordinatortest.RepositoryRoot(t), "migrations/60009.sql", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range strings.Split(string(sql), "\n") {
		if strings.TrimSpace(statement) == "" {
			t.Fatal("empty online index statement")
		}
		f.Exec(t, statement)
	}
}
