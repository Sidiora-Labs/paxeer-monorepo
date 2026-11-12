package chain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/profile"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/readpolicy"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPinnedStateReaderUsesReportedIdentity(t *testing.T) {
	header := testHeader(100)
	var calls atomic.Int64
	server := pinnedServer(t, header, false, &calls)
	client, err := NewVerifiedClient(context.Background(), server.URL, "", VerifiedConfig{profile.Paxeer("e1", "d1"), 100, consensusHash(header)})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	reader := pinnedStateReader{e: client.primary, hash: consensusHash(header), height: big.NewInt(100), header: header}
	chainID, err := reader.ChainID(context.Background())
	if err != nil || chainID.Cmp(big.NewInt(125)) != 0 {
		t.Fatalf("chain identity = %v, %v", chainID, err)
	}
	copy, err := reader.HeaderByReportedHash(context.Background(), reader.hash)
	if err != nil || copy.Number.Uint64() != 100 || copy == header {
		t.Fatalf("pinned header = %+v, %v", copy, err)
	}
	if _, err := reader.HeaderByHash(context.Background(), common.HexToHash("0x11")); !errors.Is(err, errUnpinnedHash) {
		t.Fatalf("other header identity = %v", err)
	}
	address := common.HexToAddress("0x1111111111111111111111111111111111111111")
	if _, err := reader.CodeAtHash(context.Background(), address, common.HexToHash("0x11")); !errors.Is(err, errUnpinnedHash) {
		t.Fatalf("other code identity = %v", err)
	}
	if _, err := reader.StorageAtHash(context.Background(), address, common.Hash{}, common.HexToHash("0x11")); !errors.Is(err, errUnpinnedHash) {
		t.Fatalf("other storage identity = %v", err)
	}
	if _, err := reader.CallContractAtHash(context.Background(), ethereum.CallMsg{To: &address, Data: []byte{1, 2, 3, 4}}, common.HexToHash("0x11")); !errors.Is(err, errUnpinnedHash) {
		t.Fatalf("other call identity = %v", err)
	}
	result, err := reader.CallContractAtHash(context.Background(), ethereum.CallMsg{To: &address, Data: []byte{1, 2, 3, 4}}, reader.hash)
	if err != nil || len(result) != 1 || result[0] != 1 || calls.Load() != 1 {
		t.Fatalf("pinned call = %x, %v, calls = %d", result, err, calls.Load())
	}
}

func TestMeasurementOutcomeAndProxyRequests(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{{nil, valueSuccess}, {context.Canceled, "CANCELED"}, {context.DeadlineExceeded, "DEADLINE_EXCEEDED"}, {errors.New("upstream down"), "UNAVAILABLE"}, {&readpolicy.ReadError{Code: "SEMANTIC_REVERT"}, "SEMANTIC_REVERT"}} {
		if got := measurementOutcome(tc.err); got != tc.want {
			t.Fatalf("outcome for %v = %s, want %s", tc.err, got, tc.want)
		}
	}
	for _, tc := range []struct {
		a, b []byte
		want bool
	}{{[]byte{1, 2}, []byte{1, 2}, true}, {[]byte{1}, []byte{1, 2}, false}, {[]byte{1, 2}, []byte{1, 3}, false}} {
		if got := bytesEqual(tc.a, tc.b); got != tc.want {
			t.Fatalf("byte identity for %x and %x = %t", tc.a, tc.b, got)
		}
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "committed")
	}))
	defer upstream.Close()
	endpoint, err := NewChainMeasurementEndpoint(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer endpoint.Close()
	response, err := http.Get(endpoint.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || response.StatusCode != http.StatusOK || string(body) != "committed" || endpoint.requests.read() != 1 {
		t.Fatalf("proxied response = %d %q, %v; requests = %d", response.StatusCode, body, err, endpoint.requests.read())
	}
}

func TestDurableMeasurementComparesCommittedRawRows(t *testing.T) {
	ctx := context.Background()
	pool := newMeasurementDatabase(t)
	for _, statement := range []string{
		`CREATE SCHEMA indexer`,
		`CREATE TABLE indexer.indexer_block_completeness_v1(chain_id bigint NOT NULL,stream_id text NOT NULL,height bigint NOT NULL,block_hash text NOT NULL,raw_count bigint NOT NULL,fetch_complete boolean NOT NULL,required_decode_complete boolean NOT NULL,completed_at timestamptz NOT NULL,PRIMARY KEY(chain_id,stream_id,height))`,
		`CREATE TABLE indexer.indexer_ingest_watermarks_v1(chain_id bigint NOT NULL,stream_id text NOT NULL,raw_complete_height bigint NOT NULL,typed_complete_height bigint NOT NULL,PRIMARY KEY(chain_id,stream_id))`,
		`CREATE TABLE indexer.indexer_raw_logs_v1(chain_id bigint NOT NULL,height bigint NOT NULL,transaction_hash text NOT NULL,log_index bigint NOT NULL,transaction_index bigint NOT NULL,block_hash text NOT NULL,emitter text NOT NULL,topics jsonb NOT NULL,data bytea NOT NULL)`,
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	pin := BlockPin{Number: 100, Hash: common.HexToHash("0x1234")}
	log := types.Log{Address: common.HexToAddress("0x1111111111111111111111111111111111111111"), Topics: []common.Hash{common.HexToHash("0x5678")}, Data: []byte{1, 2, 3}, BlockNumber: pin.Number, BlockHash: pin.Hash, TxHash: common.HexToHash("0xabcd"), TxIndex: 2, Index: 3}
	topics, err := json.Marshal([]string{log.Topics[0].Hex()})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		statement string
		args      []any
	}{
		{`INSERT INTO indexer.indexer_ingest_watermarks_v1 VALUES(125,'committed',100,100)`, nil},
		{`INSERT INTO indexer.indexer_block_completeness_v1 VALUES(125,'committed',100,$1,1,true,true,clock_timestamp())`, []any{pin.Hash.Hex()}},
		{`INSERT INTO indexer.indexer_raw_logs_v1 VALUES(125,100,$2,3,2,$1,$3,$4,$5)`, []any{pin.Hash.Hex(), log.TxHash.Hex(), log.Address.Hex(), topics, log.Data}},
	} {
		if _, err := pool.Exec(ctx, row.statement, row.args...); err != nil {
			t.Fatal(err)
		}
	}
	visible, recorded, err := observeDurableMeasurement(ctx, pool, "committed", pin, []types.Log{log})
	if err != nil || visible.IsZero() || recorded.IsZero() || visible.Before(recorded) {
		t.Fatalf("committed raw event observation = %s, %s, %v", visible, recorded, err)
	}
	for _, tc := range []struct {
		name    string
		change  string
		restore string
	}{
		{"data", `UPDATE indexer.indexer_raw_logs_v1 SET data='\\x00'::bytea`, `UPDATE indexer.indexer_raw_logs_v1 SET data='\\x010203'::bytea`},
		{"topic", `UPDATE indexer.indexer_raw_logs_v1 SET topics='["0x00"]'::jsonb`, `UPDATE indexer.indexer_raw_logs_v1 SET topics=$1`},
		{"block", `UPDATE indexer.indexer_block_completeness_v1 SET block_hash='other'`, `UPDATE indexer.indexer_block_completeness_v1 SET block_hash=$1`},
		{"count", `UPDATE indexer.indexer_block_completeness_v1 SET raw_count=2`, `UPDATE indexer.indexer_block_completeness_v1 SET raw_count=1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, tc.change); err != nil {
				t.Fatal(err)
			}
			if _, _, err := observeDurableMeasurement(ctx, pool, "committed", pin, []types.Log{log}); err == nil {
				t.Fatal("changed durable identity was accepted")
			}
			var restore any = pin.Hash.Hex()
			if tc.name == "topic" {
				restore = topics
			}
			args := []any{}
			if tc.name == "topic" || tc.name == "block" {
				args = append(args, restore)
			}
			if _, err := pool.Exec(ctx, tc.restore, args...); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, mode := range []string{"topics type", "extra raw", "index range", "missing raw table", "missing completeness", "waiting source"} {
		t.Run(mode, func(t *testing.T) {
			expected := []types.Log{log}
			callCtx := ctx
			var restore string
			switch mode {
			case "topics type":
				if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_raw_logs_v1 SET topics='{}'`); err != nil {
					t.Fatal(err)
				}
				restore = `UPDATE indexer.indexer_raw_logs_v1 SET topics='["` + log.Topics[0].Hex() + `"]'`
			case "extra raw":
				expected = nil
				if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_block_completeness_v1 SET raw_count=0`); err != nil {
					t.Fatal(err)
				}
				restore = `UPDATE indexer.indexer_block_completeness_v1 SET raw_count=1`
			case "index range":
				expected[0].Index = ^uint(0)
			case "missing raw table":
				if _, err := pool.Exec(ctx, `ALTER TABLE indexer.indexer_raw_logs_v1 RENAME TO original_raw`); err != nil {
					t.Fatal(err)
				}
				restore = `ALTER TABLE indexer.original_raw RENAME TO indexer_raw_logs_v1`
			case "missing completeness":
				if _, err := pool.Exec(ctx, `ALTER TABLE indexer.indexer_block_completeness_v1 RENAME TO original_completeness`); err != nil {
					t.Fatal(err)
				}
				restore = `ALTER TABLE indexer.original_completeness RENAME TO indexer_block_completeness_v1`
			case "waiting source":
				if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_block_completeness_v1 SET fetch_complete=false`); err != nil {
					t.Fatal(err)
				}
				restore = `UPDATE indexer.indexer_block_completeness_v1 SET fetch_complete=true`
				bounded, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
				defer cancel()
				callCtx = bounded
			}
			if _, _, err := observeDurableMeasurement(callCtx, pool, "committed", pin, expected); err == nil {
				t.Fatal("incomplete durable evidence admitted")
			}
			if restore != "" {
				if _, err := pool.Exec(ctx, restore); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	if _, err := pool.Exec(ctx, `DELETE FROM indexer.indexer_raw_logs_v1`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := observeDurableMeasurement(ctx, pool, "committed", pin, []types.Log{log}); err == nil {
		t.Fatal("missing durable event was accepted")
	}
	if _, _, err := observeDurableMeasurement(ctx, pool, "", pin, nil); err == nil {
		t.Fatal("durable observation without a stream admitted")
	}
	chainHeader := testHeader(100)
	var calls atomic.Int64
	upstream := pinnedServer(t, chainHeader, false, &calls)
	primary, err := NewChainMeasurementEndpoint(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer primary.Close()
	fallback, err := NewChainMeasurementEndpoint(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer fallback.Close()
	client, err := NewVerifiedClient(ctx, primary.URL, fallback.URL, VerifiedConfig{profile.Paxeer("e1", "d1"), 100, consensusHash(chainHeader)})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	buildPath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	build, err := os.ReadFile(buildPath)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(build)
	sha := hex.EncodeToString(digest[:])
	adapter := &ReadAdapter{client: client, binding: profile.ManifestBinding{Profile: profile.Paxeer("e1", "d1"), ManifestHash: sha, InterfaceHash: sha}, configurationRevision: "measurement-fixture", maxAge: time.Minute}
	cfg := ChainMeasurementConfig{Binding: readpolicy.MeasurementBinding{CandidateSHA256: sha, InterfaceSHA256: sha, ManifestSHA256: sha, TopologySHA256: sha, EnvironmentSHA256: sha, BuildSHA256: sha, WorkloadSeed: "pinned-measurement", ProfileVersion: "e1"}, OutputDirectory: t.TempDir() + "/capture", StreamID: "committed", Duration: 10 * time.Second, Pool: common.HexToAddress("0x2222222222222222222222222222222222222222"), LogAddresses: []common.Address{log.Address}, MaxRows: 2000, MaxBytes: 1 << 20}
	run, err := RunChainMeasurement(ctx, adapter, pool, primary, fallback, cfg)
	if err == nil || run == nil || run.FailureCode != "HOT_KEY_GROUP_MISMATCH" || run.HotKeyRequests != 1000 {
		t.Fatalf("unattested workload = %+v, %v", run, err)
	}
	result, err := FinishChainMeasurement(run, ChainMeasurementEvidence{})
	if err == nil || result.LaneHarnessPassed || result.FailureCode != run.FailureCode {
		t.Fatalf("unattested workload artifact = %+v, %v", result, err)
	}
}

func newMeasurementDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DELIVERY_TEST_DSN")
	if dsn == "" {
		t.Fatal("private PostgreSQL fixture required")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Hostname() != "127.0.0.1" || parsed.Port() != "25432" || parsed.Path != "/kindle_fixture" {
		t.Fatal("isolated PostgreSQL fixture required")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	database := fmt.Sprintf("chain_measure_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{database}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanup, "DROP DATABASE "+quoted+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	})
	parsed.Path = "/" + database
	pool, err := pgxpool.New(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
