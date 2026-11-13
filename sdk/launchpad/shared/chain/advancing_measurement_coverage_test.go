package chain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/readpolicy"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestChainMeasurementTracksCommittedIngestAcrossEndpointLoss(t *testing.T) {
	ctx := context.Background()
	adapter, backend := snapshotRuntimeFixture(t, "surface")
	headers := map[uint64]*types.Header{100: backend.header}
	for height := uint64(101); height <= 102; height++ {
		header := types.CopyHeader(backend.header)
		header.Number = new(big.Int).SetUint64(height)
		header.ParentHash = consensusHash(headers[height-1])
		headers[height] = header
	}
	pool := newMeasurementDatabase(t)
	var started atomic.Int64
	started.Store(time.Now().UnixNano())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		var req struct {
			Method string
			Params []json.RawMessage
		}
		if json.Unmarshal(raw, &req) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		selected := headers[100]
		if req.Method == "eth_getBlockByNumber" || req.Method == "eth_getBlockByHash" {
			var selector string
			if len(req.Params) > 0 {
				_ = json.Unmarshal(req.Params[0], &selector)
			}
			if selector == "latest" {
				switch elapsed := time.Since(time.Unix(0, started.Load())); {
				case elapsed > 7*time.Second:
					selected = headers[102]
				case elapsed > 2*time.Second:
					selected = headers[101]
				}
			} else if req.Method == "eth_getBlockByNumber" {
				if number, ok := new(big.Int).SetString(selector[2:], 16); ok {
					if header := headers[number.Uint64()]; header != nil {
						selected = header
					}
				}
			} else {
				for _, header := range headers {
					if consensusHash(header).Hex() == selector {
						selected = header
						break
					}
				}
			}
		}
		if req.Method == "eth_getBlockByNumber" && selected.Number.Uint64() > 100 {
			if _, err := pool.Exec(r.Context(), "UPDATE indexer.indexer_ingest_watermarks_v1 SET raw_complete_height=102,typed_complete_height=102"); err != nil {
				t.Error(err)
				http.Error(w, "durable source unavailable", 500)
				return
			}
		}
		copyBackend := *backend
		copyBackend.header = selected
		r.Body = io.NopCloser(bytes.NewReader(raw))
		copyBackend.ServeHTTP(w, r)
	}))
	defer server.Close()
	primary, err := NewChainMeasurementEndpoint(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer primary.Close()
	fallback, err := NewChainMeasurementEndpoint(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer fallback.Close()
	client, err := NewVerifiedClient(ctx, primary.URL, fallback.URL, VerifiedConfig{adapter.binding.Profile, 100, consensusHash(backend.header)})
	if err != nil {
		t.Fatal(err)
	}
	adapter.client.Close()
	adapter.client = client
	if _, err := pool.Exec(ctx, `CREATE SCHEMA indexer;
CREATE TABLE indexer.indexer_ingest_watermarks_v1(chain_id bigint,stream_id text,raw_complete_height bigint,typed_complete_height bigint);
CREATE TABLE indexer.indexer_block_completeness_v1(chain_id bigint,stream_id text,height bigint,block_hash text,raw_count bigint,fetch_complete boolean,required_decode_complete boolean,completed_at timestamptz);
CREATE TABLE indexer.indexer_raw_logs_v1(chain_id bigint,height bigint,transaction_hash text,transaction_index bigint,log_index bigint,block_hash text,emitter text,topics jsonb,data bytea);
INSERT INTO indexer.indexer_ingest_watermarks_v1 VALUES(125,'advancing',100,100)`); err != nil {
		t.Fatal(err)
	}
	for height := uint64(101); height <= 102; height++ {
		if _, err := pool.Exec(ctx, "INSERT INTO indexer.indexer_block_completeness_v1 VALUES(125,'advancing',$1,$2,0,true,true,clock_timestamp())", height, consensusHash(headers[height]).Hex()); err != nil {
			t.Fatal(err)
		}
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(binary)
	digest := hex.EncodeToString(hash[:])
	binding := readpolicy.MeasurementBinding{CandidateSHA256: digest, BuildSHA256: digest, InterfaceSHA256: adapter.binding.InterfaceHash, ManifestSHA256: adapter.binding.ManifestHash, TopologySHA256: digest, EnvironmentSHA256: digest, WorkloadSeed: "advancing-source", ProfileVersion: "e1"}
	config := ChainMeasurementConfig{Binding: binding, OutputDirectory: filepath.Join(t.TempDir(), "capture"), StreamID: "advancing", Duration: 10 * time.Second, Pool: backend.pool, LogAddresses: []common.Address{backend.pool}, MaxRows: 3000, MaxBytes: 4 << 20}
	for _, mode := range []string{"binding", "manifest", "interface", "profile", "build", "log address", "row budget", "existing directory", "duration"} {
		candidate := config
		switch mode {
		case "binding":
			candidate.Binding.CandidateSHA256 = "invalid"
		case "manifest":
			candidate.Binding.ManifestSHA256 = strings.Repeat("0", 64)
		case "interface":
			candidate.Binding.InterfaceSHA256 = strings.Repeat("0", 64)
		case "profile":
			candidate.Binding.ProfileVersion = "other"
		case "build":
			candidate.Binding.BuildSHA256 = strings.Repeat("0", 64)
		case "log address":
			candidate.LogAddresses = []common.Address{{}}
		case "row budget":
			candidate.MaxRows = 0
		case "existing directory":
			candidate.OutputDirectory = t.TempDir()
		case "duration":
			candidate.Duration = time.Second
		}
		if run, err := RunChainMeasurement(ctx, adapter, pool, primary, fallback, candidate); err == nil || run != nil {
			t.Fatalf("invalid %s measurement admitted: %+v %v", mode, run, err)
		}
	}
	started.Store(time.Now().UnixNano())
	run, err := RunChainMeasurement(ctx, adapter, pool, primary, fallback, config)
	if err != nil || run == nil || run.FailureCode != "" || run.HotKeyRequests != 1000 || run.HotKeyUpstreamGroups != 1 || run.IngestedBlocks != 2 || run.AfterLossIngestedBlocks != 1 || run.EndpointLossRequests == 0 || run.UpstreamRPCRequests == 0 || len(run.IngestLatencies) != 2 {
		t.Fatalf("committed ingest measurement: %+v %v", run, err)
	}
	result, err := FinishChainMeasurement(run, ChainMeasurementEvidence{})
	if err == nil || result.LaneHarnessPassed || result.RawRows <= 1000 || result.FailureCode != "MISSING_ENVIRONMENT_OR_TOPOLOGY_BINDING" {
		t.Fatalf("source evidence admission: %+v %v", result, err)
	}
}
