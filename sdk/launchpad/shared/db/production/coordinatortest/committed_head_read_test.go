package coordinatortest

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/jackc/pgx/v5"
)

const committedHeadRowBound = 32

type committedHeadPlan struct {
	NodeType  string              `json:"Node Type"`
	Relation  string              `json:"Relation Name"`
	Index     string              `json:"Index Name"`
	Rows      float64             `json:"Actual Rows"`
	Loops     float64             `json:"Actual Loops"`
	Filtered  float64             `json:"Rows Removed by Filter"`
	Rechecked float64             `json:"Rows Removed by Index Recheck"`
	Plans     []committedHeadPlan `json:"Plans"`
}

func committedHeadSourceSQL(t *testing.T, relative, function string) string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(RepositoryRoot(t), relative), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var evaluate func(ast.Expr) string
	evaluate = func(expression ast.Expr) string {
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
				return evaluate(value.X) + evaluate(value.Y)
			}
		case *ast.SelectorExpr:
			if config, ok := value.X.(*ast.SelectorExpr); ok && value.Sel.Name == "SourceSchema" && config.Sel.Name == "config" {
				if receiver, ok := config.X.(*ast.Ident); ok && receiver.Name == "a" {
					return "indexer"
				}
			}
		case *ast.CallExpr:
			if method, ok := value.Fun.(*ast.SelectorExpr); ok && method.Sel.Name == "Sanitize" && len(value.Args) == 0 {
				if literal, ok := method.X.(*ast.CompositeLit); ok {
					if typ, ok := literal.Type.(*ast.SelectorExpr); ok && typ.Sel.Name == "Identifier" {
						parts := make([]string, len(literal.Elts))
						for i, element := range literal.Elts {
							parts[i] = evaluate(element)
						}
						return pgx.Identifier(parts).Sanitize()
					}
				}
			}
		}
		t.Fatalf("unresolved committed head SQL expression in %s: %T", relative, expression)
		return ""
	}
	var queries []string
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Name.Name != function || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			if method, ok := call.Fun.(*ast.SelectorExpr); ok && method.Sel.Name == "QueryRow" {
				queries = append(queries, evaluate(call.Args[1]))
			}
			return true
		})
	}
	if len(queries) != 1 {
		t.Fatalf("%s:%s has %d committed head statements, want one", relative, function, len(queries))
	}
	return queries[0]
}

func committedHeadLegacySQL(t *testing.T) string {
	t.Helper()
	raw := readBound(t, RepositoryRoot(t), "migrations/60001.sql", "")
	_, definition, found := strings.Cut(string(raw), "CREATE VIEW indexer.indexer_committed_heads_v2 AS\n")
	if !found {
		t.Fatal("committed head view definition absent")
	}
	definition, _, found = strings.Cut(definition, ";")
	if !found {
		t.Fatal("committed head view definition is incomplete")
	}
	return definition
}

func assertCommittedHeadPlan(t *testing.T, plan committedHeadPlan) (float64, []string) {
	t.Helper()
	var reads float64
	var nodes []string
	var walk func(committedHeadPlan)
	walk = func(node committedHeadPlan) {
		if strings.Contains(node.NodeType, "Sort") || strings.Contains(node.NodeType, "Aggregate") {
			t.Fatalf("committed head read sorts or aggregates: %+v", node)
		}
		if node.Relation == "indexer_committed_heads" {
			reads += (node.Rows + node.Filtered + node.Rechecked) * node.Loops
		}
		if node.Loops > 0 {
			nodes = append(nodes, fmt.Sprintf("%s[%s] rows=%g loops=%g", node.NodeType, node.Index, node.Rows, node.Loops))
		}
		for _, child := range node.Plans {
			walk(child)
		}
	}
	walk(plan)
	if plan.Rows != 1 || plan.Loops != 1 || reads < 1 || reads >= committedHeadRowBound {
		t.Fatalf("committed head plan reads %g rows (bound <%d), returns %g rows in %g loops: %+v", reads, committedHeadRowBound, plan.Rows, plan.Loops, plan)
	}
	return reads, nodes
}

func TestCommittedHeadReadBounded(t *testing.T) {
	admin := os.Getenv(PostgresDSNEnv)
	if admin == "" {
		t.Fatalf("%s is required", PostgresDSNEnv)
	}
	database := Bootstrap(t, admin, "users_runtime", "metadata_runtime", "api_rest_runtime")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	execute := func(sql string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	execute(`INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id)
 SELECT 125,'source',height,'0x'||lpad(to_hex(height),64,'0'),CASE WHEN height IN (12,222222) THEN 'quarantined' ELSE 'healthy' END,'canonical'
 FROM generate_series(1,300000) height;
 INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id)
 SELECT 125,'fork',height,'0x'||repeat('f',64),'healthy','canonical' FROM unnest(ARRAY[7,100000,200000]) height;
 INSERT INTO indexer.indexer_quarantine(chain_id,stream_id,transaction_hash,log_index,block_hash,height,raw_digest,raw_payload,error_code,abi_revision,requirement,required,attempt_count,next_attempt_at,resolution)
 VALUES(125,'absent','absent',0,'block',1,repeat('a',64),'{}','decode','v1','decode',true,1,clock_timestamp(),'unresolved'),
 (125,'source','optional',0,'block',2,repeat('b',64),'{}','decode','v1','decode',false,1,clock_timestamp(),'unresolved'),
 (125,'source','repaired',0,'block',3,repeat('c',64),'{}','decode','v1','decode',true,1,clock_timestamp(),'repaired');
 ANALYZE indexer.indexer_committed_heads;
 ANALYZE indexer.indexer_quarantine;`)
	var count int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM indexer.indexer_committed_heads WHERE chain_id=125").Scan(&count); err != nil || count < 300000 {
		t.Fatalf("committed head population=%d: %v", count, err)
	}
	legacy := committedHeadLegacySQL(t)
	readers := []struct {
		name, role, schema, file, function string
		timestamp                          bool
	}{
		{"user authorization", "users_runtime", "users", "sdk/launchpad/shared/auth/contract.go", "CommittedBlock", false},
		{"metadata authorization", "metadata_runtime", "metadata", "sdk/launchpad/shared/auth/contract.go", "CommittedBlock", false},
		{"API authority", "api_rest_runtime", "public", "services/launchpad/api/internal/app/authority.go", "head", false},
		{"API holder rewards", "api_rest_runtime", "public", "services/launchpad/api/internal/store/holder_rewards_readmodel.go", "committedWatermark", true},
	}
	var report *os.File
	if path := os.Getenv("COMMITTED_HEAD_READ_PLAN_FILE"); path != "" {
		report, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			t.Fatal(err)
		}
		defer report.Close()
	}
	for _, scenario := range []struct {
		name, sql, integrity string
	}{
		{"healthy", "", "healthy"},
		{"agreeing streams", `INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) SELECT 125,'replica_'||n,300000,'0x'||lpad(to_hex(300000),64,'0'),'healthy','canonical' FROM generate_series(1,300000) n`, "healthy"},
		{"conflicting hashes", `INSERT INTO indexer.indexer_committed_heads VALUES(125,'latest_fork',300000,'0x'||repeat('f',64),'healthy','canonical',clock_timestamp())`, "quarantined"},
		{"quarantined stream", `DELETE FROM indexer.indexer_committed_heads WHERE stream_id='latest_fork'; UPDATE indexer.indexer_committed_heads SET integrity='quarantined' WHERE stream_id='source' AND height=300000`, "quarantined"},
		{"required quarantine below head", `UPDATE indexer.indexer_committed_heads SET integrity='healthy' WHERE stream_id='source' AND height=300000;
 INSERT INTO indexer.indexer_quarantine(chain_id,stream_id,transaction_hash,log_index,block_hash,height,raw_digest,raw_payload,error_code,abi_revision,requirement,required,attempt_count,next_attempt_at)
 VALUES(125,'source','required',0,'block',299999,repeat('d',64),'{}','decode','v1','decode',true,1,clock_timestamp())`, "quarantined"},
		{"valid waiver", `INSERT INTO indexer.indexer_replay_authorizations(authorization_id,operator_identity,authorization_evidence,action,target,generation,projection_revision,from_height,to_height,max_concurrent,expires_at)
 VALUES('waiver','operator',repeat('e',64),'waive','indexer','current','v1',0,300000,1,clock_timestamp()+interval '1 hour');
 INSERT INTO indexer.indexer_quarantine_waivers(raw_digest,requirement,operator_identity,authorization_id,reason,from_height,to_height,expires_at)
 VALUES(repeat('d',64),'decode','operator','waiver','reviewed',0,300000,clock_timestamp()+interval '1 hour')`, "healthy"},
		{"expired waiver", `UPDATE indexer.indexer_quarantine_waivers SET expires_at=clock_timestamp()-interval '1 hour'`, "quarantined"},
		{"expired authorization", `UPDATE indexer.indexer_quarantine_waivers SET expires_at=clock_timestamp()+interval '1 hour'; UPDATE indexer.indexer_replay_authorizations SET expires_at=clock_timestamp()-interval '1 hour'`, "quarantined"},
		{"revoked authorization", `UPDATE indexer.indexer_replay_authorizations SET expires_at=clock_timestamp()+interval '1 hour',revoked_at=clock_timestamp()`, "quarantined"},
		{"waiver outside range", `UPDATE indexer.indexer_replay_authorizations SET revoked_at=NULL; UPDATE indexer.indexer_quarantine_waivers SET from_height=300000`, "quarantined"},
		{"repaired quarantine", `UPDATE indexer.indexer_quarantine SET resolution='repaired' WHERE transaction_hash='required'`, "healthy"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if scenario.sql != "" {
				if _, err := conn.Exec(ctx, scenario.sql); err != nil {
					t.Fatal(err)
				}
			}
			execute(`ANALYZE indexer.indexer_committed_heads; ANALYZE indexer.indexer_quarantine; ANALYZE indexer.indexer_quarantine_waivers; ANALYZE indexer.indexer_replay_authorizations`)
			var wantHeight, wantHash, wantIntegrity string
			if err := conn.QueryRow(ctx, `WITH legacy AS (`+legacy+`) SELECT height::text,block_hash,integrity FROM legacy WHERE chain_id=125 AND height=300000`).Scan(&wantHeight, &wantHash, &wantIntegrity); err != nil {
				t.Fatal(err)
			}
			if wantHeight != "300000" || wantIntegrity != scenario.integrity {
				t.Fatalf("committed head reference=%s %s, want 300000 %s", wantHeight, wantIntegrity, scenario.integrity)
			}
			for _, reader := range readers {
				t.Run(reader.name, func(t *testing.T) {
					config, err := pgx.ParseConfig(database.Runtimes[reader.role].DSN)
					if err != nil {
						t.Fatal(err)
					}
					config.RuntimeParams["search_path"] = pgx.Identifier{reader.schema}.Sanitize() + ",pg_catalog"
					runtime, err := pgx.ConnectConfig(ctx, config)
					if err != nil {
						t.Fatal(err)
					}
					defer runtime.Close(context.Background())
					var role string
					if err := runtime.QueryRow(ctx, "SELECT current_user").Scan(&role); err != nil || role != database.Runtimes[reader.role].Role {
						t.Fatalf("committed head runtime role=%s: %v", role, err)
					}
					statement := committedHeadSourceSQL(t, reader.file, reader.function)
					var raw []byte
					if err := runtime.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+statement).Scan(&raw); err != nil {
						t.Fatal(err)
					}
					var plans []struct{ Plan committedHeadPlan }
					if err := json.Unmarshal(raw, &plans); err != nil || len(plans) != 1 {
						t.Fatalf("committed head plan=%s: %v", raw, err)
					}
					reads, nodes := assertCommittedHeadPlan(t, plans[0].Plan)
					t.Logf("reader=%s role=%s committed rows=%g plan=%s", reader.name, reader.role, reads, strings.Join(nodes, " -> "))
					if report != nil {
						if err := json.NewEncoder(report).Encode(struct {
							Scenario, Reader, Role string
							Rows                   float64
							Plan                   json.RawMessage
						}{scenario.name, reader.name, reader.role, reads, raw}); err != nil {
							t.Fatal(err)
						}
					}
					var height, hash, integrity string
					var observed time.Time
					destinations := []any{&height, &hash, &integrity}
					if reader.timestamp {
						destinations = append(destinations, &observed)
					}
					if err := runtime.QueryRow(ctx, statement).Scan(destinations...); err != nil {
						t.Fatal(err)
					}
					if height != wantHeight || hash != wantHash || integrity != wantIntegrity || (reader.timestamp && observed.IsZero()) {
						t.Fatalf("committed head=%s %s %s %s, want %s %s %s", height, hash, integrity, observed, wantHeight, wantHash, wantIntegrity)
					}
				})
			}
		})
		if t.Failed() {
			return
		}
	}
	for _, height := range []int64{7, 9, 10, 12, 99999, 100000, 200000, 222222, 300000} {
		var differences int
		if err := conn.QueryRow(ctx, `WITH legacy AS (`+legacy+`), expected AS (SELECT * FROM legacy WHERE chain_id=125 AND height=$1), actual AS (SELECT * FROM indexer.indexer_committed_heads_v2 WHERE chain_id=125 AND height=$1) SELECT count(*) FROM ((SELECT * FROM expected EXCEPT ALL SELECT * FROM actual) UNION ALL (SELECT * FROM actual EXCEPT ALL SELECT * FROM expected)) differences`, height).Scan(&differences); err != nil || differences != 0 {
			t.Fatalf("committed height %d differs from grouped view: %d: %v", height, differences, err)
		}
		if err := conn.QueryRow(ctx, `WITH legacy AS (`+legacy+`), expected AS (SELECT h.chain_id,h.stream_id,h.height,h.block_hash,CASE WHEN h.integrity='healthy' THEN v.integrity ELSE 'quarantined' END AS integrity,h.source_id,h.observed_at FROM indexer.indexer_committed_heads h JOIN legacy v USING(chain_id,height) WHERE h.chain_id=125 AND h.height=$1), actual AS (SELECT * FROM indexer.indexer_committed_heads_v1 WHERE chain_id=125 AND height=$1) SELECT count(*) FROM ((SELECT * FROM expected EXCEPT ALL SELECT * FROM actual) UNION ALL (SELECT * FROM actual EXCEPT ALL SELECT * FROM expected)) differences`, height).Scan(&differences); err != nil || differences != 0 {
			t.Fatalf("committed v1 height %d differs from grouped view: %d: %v", height, differences, err)
		}
	}
}

func TestCommittedHeadReadReadinessDependencies(t *testing.T) {
	admin := os.Getenv(PostgresDSNEnv)
	if admin == "" {
		t.Fatalf("%s is required", PostgresDSNEnv)
	}
	for _, role := range []string{"users_runtime", "metadata_runtime", "api_rest_runtime"} {
		t.Run(role, func(t *testing.T) {
			database := Bootstrap(t, admin, role)
			for _, id := range []int64{60004, 60005, 60006} {
				if database.Applied[id] == "" {
					t.Fatalf("%s committed head migration %d absent", role, id)
				}
			}
			runtime := database.Runtimes[role]
			expected, err := production.LoadExpectedSchema(runtime.ExpectedSchema, runtime.ExpectedSchemaSHA256)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []int64{60004, 60005, 60006} {
				if expected.Migrations[id] != database.Applied[id] {
					t.Fatalf("%s readiness differs for committed head migration %d", role, id)
				}
			}
		})
	}
}

func TestMigrationRegistryHonorsPredecessorOrderCommittedHeads(t *testing.T) {
	root := RepositoryRoot(t)
	raw := readBound(t, root, "migrations/registry.json", "")
	registry, err := production.LoadMigrationRegistry(raw, production.Checksum(raw))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := registry.ValidateOrder()
	if err != nil {
		t.Fatal(err)
	}
	positions := map[int64]int{}
	for i, id := range registry.ApplicationOrder {
		positions[id] = i
	}
	for _, id := range []int64{60004, 60005} {
		migration := catalog[id]
		if migration.OnlineIndex == nil || migration.TransactionMode != "nontransactional" || migration.OnlineIndex.Schema != "indexer" || migration.OnlineIndex.Table != "indexer_committed_heads" {
			t.Fatalf("committed head migration %d requires a concurrent index on the committed heads", id)
		}
		statement, err := migration.OnlineIndex.CreateSQL()
		if err != nil || statement != string(readBound(t, root, migration.Path, migration.SHA256)) {
			t.Fatalf("committed head migration %d index source differs: %v", id, err)
		}
		if len(migration.PredecessorIDs) != 1 || migration.PredecessorIDs[0] != id-1 || positions[id] <= positions[id-1] {
			t.Fatalf("committed head index migration %d predecessor differs", id)
		}
	}
	view := catalog[60006]
	if view.TransactionMode != "transactional" || len(view.PredecessorIDs) != 2 || view.PredecessorIDs[0] != 60001 || view.PredecessorIDs[1] != 60005 || positions[60006] <= positions[60005] {
		t.Fatal("committed head view requires the committed head table and both concurrent indexes")
	}
	if !strings.Contains(string(readBound(t, root, view.Path, view.SHA256)), "CREATE OR REPLACE VIEW indexer.indexer_committed_heads_v2 AS") {
		t.Fatal("committed head migration must retain the granted view identity")
	}
}
