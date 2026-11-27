package l16

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const acceptedHeadWatermarkSQL = `SELECT LEAST(raw_complete_height,typed_complete_height) FROM indexer.indexer_ingest_watermarks_v1 WHERE chain_id=125 AND stream_id=$1`
const acceptedHeadHeightSQL = `SELECT c.height,c.block_hash FROM indexer.indexer_block_completeness_v1 c JOIN indexer.indexer_committed_heads_v1 h ON h.chain_id=c.chain_id AND h.stream_id=c.stream_id AND h.height=$3::numeric AND h.block_hash=c.block_hash AND h.integrity='healthy' WHERE c.chain_id=125 AND c.stream_id=$1 AND c.height=$2::bigint AND c.fetch_complete AND c.required_decode_complete AND c.raw_count=(SELECT count(*) FROM indexer.indexer_raw_logs_v1 r WHERE r.chain_id=c.chain_id AND r.height=c.height)`
const acceptedHeadJoinedSQL = `SELECT c.height,c.block_hash FROM indexer.indexer_ingest_watermarks_v1 w JOIN indexer.indexer_block_completeness_v1 c ON c.chain_id=w.chain_id AND c.stream_id=w.stream_id AND c.height=LEAST(w.raw_complete_height,w.typed_complete_height) JOIN indexer.indexer_committed_heads_v1 h ON h.chain_id=c.chain_id AND h.stream_id=c.stream_id AND h.height=c.height AND h.block_hash=c.block_hash AND h.integrity='healthy' WHERE w.chain_id=125 AND w.stream_id=$1 AND c.fetch_complete AND c.required_decode_complete AND c.raw_count=(SELECT count(*) FROM indexer.indexer_raw_logs_v1 r WHERE r.chain_id=c.chain_id AND r.height=c.height)`

type acceptedHeadBoundFixture struct {
	ctx     context.Context
	admin   *pgxpool.Pool
	runtime *pgxpool.Pool
	worker  *LiveTransferWorker
	stream  string
	height  int64
	hash    string
}

func newAcceptedHeadBoundFixture(t *testing.T) acceptedHeadBoundFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	db := coordinatortest.Bootstrap(t, realMarketEnv(t, "PNL_TEST_POSTGRES_DSN"), "pnl_runtime")
	admin, err := pgxpool.New(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	runtime, err := pgxpool.New(ctx, db.Runtimes["pnl_runtime"].DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	f := acceptedHeadBoundFixture{ctx: ctx, admin: admin, runtime: runtime, stream: "accepted-head-bound", height: 42, hash: "0x" + strings.Repeat("a", 64)}
	f.worker = &LiveTransferWorker{service: &Service{authority: &BaselineAuthority{pool: runtime, sourceStream: f.stream}}}
	for _, sql := range []string{
		`INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,$1,$2,$2)`,
		`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,0,true,true)`,
		`INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) VALUES(125,$1,$2,$3,'healthy','canonical-node')`,
	} {
		args := []any{f.stream, f.height}
		if strings.Contains(sql, "$3") {
			args = append(args, f.hash)
		}
		if _, err := admin.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f acceptedHeadBoundFixture) transaction(t *testing.T) pgx.Tx {
	t.Helper()
	tx, err := f.runtime.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { inventoryRollback(tx) })
	return tx
}

func TestAcceptedHeadBoundReadsTheHead(t *testing.T) {
	f := newAcceptedHeadBoundFixture(t)
	for _, heights := range [][2]int64{{43, 42}, {42, 43}} {
		if _, err := f.admin.Exec(f.ctx, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=$2,typed_complete_height=$3 WHERE stream_id=$1`, f.stream, heights[0], heights[1]); err != nil {
			t.Fatal(err)
		}
		height, hash, err := f.worker.acceptedHeadTx(f.ctx, f.transaction(t))
		if err != nil || height != f.height || hash != f.hash {
			t.Fatalf("accepted head = (%d, %q, %v), want (%d, %q, nil)", height, hash, err, f.height, f.hash)
		}
	}
}

func TestAcceptedHeadBoundWithoutAHead(t *testing.T) {
	f := newAcceptedHeadBoundFixture(t)
	for _, tc := range []struct{ name, sql string }{
		{"missing", `DELETE FROM indexer.indexer_committed_heads WHERE stream_id=$1`},
		{"other height", `INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) VALUES(125,$1,43,'0x'||repeat('a',64),'healthy','canonical-node')`},
		{"other stream", `INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) VALUES(125,$1||'-other',42,'0x'||repeat('a',64),'healthy','canonical-node')`},
		{"quarantined", `INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) VALUES(125,$1,42,'0x'||repeat('a',64),'quarantined','canonical-node')`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.admin.Exec(f.ctx, tc.sql, f.stream); err != nil {
				t.Fatal(err)
			}
			height, hash, err := f.worker.acceptedHeadTx(f.ctx, f.transaction(t))
			if !errors.Is(err, pgx.ErrNoRows) || height != 0 || hash != "" {
				t.Fatalf("accepted head = (%d, %q, %v), want (0, empty, ErrNoRows)", height, hash, err)
			}
		})
	}
}

func TestAcceptedHeadBoundWithoutAWatermark(t *testing.T) {
	f := newAcceptedHeadBoundFixture(t)
	if _, err := f.admin.Exec(f.ctx, `DELETE FROM indexer.indexer_ingest_watermarks WHERE stream_id=$1`, f.stream); err != nil {
		t.Fatal(err)
	}
	height, hash, err := f.worker.acceptedHeadTx(f.ctx, f.transaction(t))
	if !errors.Is(err, pgx.ErrNoRows) || height != 0 || hash != "" {
		t.Fatalf("accepted head = (%d, %q, %v), want (0, empty, ErrNoRows)", height, hash, err)
	}
}

func acceptedHeadBoundStatements(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "live_transfers.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var statements []string
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "acceptedHeadTx" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "QueryRow" {
				return true
			}
			literal, ok := call.Args[1].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				t.Fatal("accepted head statements require SQL literals")
			}
			sql, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			statements = append(statements, sql)
			return true
		})
	}
	if len(statements) != 2 || statements[0] != acceptedHeadWatermarkSQL || statements[1] != acceptedHeadHeightSQL {
		t.Fatalf("accepted head statements = %q", statements)
	}
	return statements
}

type acceptedHeadPlanNode struct {
	NodeType  string                 `json:"Node Type"`
	Schema    string                 `json:"Schema"`
	Relation  string                 `json:"Relation Name"`
	IndexCond string                 `json:"Index Cond"`
	Plans     []acceptedHeadPlanNode `json:"Plans"`
}

func acceptedHeadBoundPlan(t *testing.T, f acceptedHeadBoundFixture, tx pgx.Tx, sql string, args ...any) []acceptedHeadPlanNode {
	t.Helper()
	var raw []byte
	if err := tx.QueryRow(f.ctx, "EXPLAIN (FORMAT JSON, VERBOSE) "+sql, args...).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var plan []struct {
		Plan acceptedHeadPlanNode `json:"Plan"`
	}
	if err := json.Unmarshal(raw, &plan); err != nil || len(plan) != 1 {
		t.Fatalf("query plan = %s, error = %v", raw, err)
	}
	var reads []acceptedHeadPlanNode
	var visit func(acceptedHeadPlanNode)
	visit = func(node acceptedHeadPlanNode) {
		if node.Schema == "indexer" && node.Relation == "indexer_committed_heads" {
			reads = append(reads, node)
		}
		for _, child := range node.Plans {
			visit(child)
		}
	}
	visit(plan[0].Plan)
	if len(reads) == 0 {
		t.Fatalf("query plan has no committed head reads: %s", raw)
	}
	return reads
}

func TestAcceptedHeadBoundPlan(t *testing.T) {
	f := newAcceptedHeadBoundFixture(t)
	tx := f.transaction(t)
	if _, err := tx.Exec(f.ctx, "SET LOCAL enable_seqscan=off"); err != nil {
		t.Fatal(err)
	}
	sql := acceptedHeadBoundStatements(t)[1]
	for _, node := range acceptedHeadBoundPlan(t, f, tx, sql, f.stream, f.height, f.height) {
		if !strings.Contains(node.IndexCond, "height") {
			t.Errorf("committed head %s requires a height index condition: %+v", node.NodeType, node)
		}
	}
	unbounded := 0
	for _, node := range acceptedHeadBoundPlan(t, f, tx, acceptedHeadJoinedSQL, f.stream) {
		if !strings.Contains(node.IndexCond, "height") {
			unbounded++
			t.Logf("joined head read: %s, index condition %q", node.NodeType, node.IndexCond)
		}
	}
	if unbounded == 0 {
		t.Fatal("joined head plan requires a committed head read without a height index condition")
	}
}
