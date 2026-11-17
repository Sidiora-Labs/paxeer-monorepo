package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
)

type canonicalReadinessTransport struct {
	head        atomic.Int64
	failure     atomic.Int32
	latestReads atomic.Int64
}

func canonicalReadinessRecordedTransport(t *testing.T, r *canonicalSourceReadiness) *canonicalReadinessTransport {
	t.Helper()
	var endpoint string
	for _, value := range r.doc.Committed.Endpoints {
		endpoint = value
		break
	}
	target, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	forward := httputil.NewSingleHostReverseProxy(target)
	transport := new(canonicalReadinessTransport)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var call struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(body, &call); err != nil {
			t.Error(err)
			return
		}
		if call.Method == "eth_getBlockByNumber" && len(call.Params) == 2 && string(call.Params[1]) == "true" {
			latest := string(call.Params[0]) == `"latest"`
			if latest {
				transport.latestReads.Add(1)
			}
			if transport.failure.Load() == 1 && latest || transport.failure.Load() == 2 && !latest {
				http.Error(w, "source read unavailable", http.StatusBadGateway)
				return
			}
			if height := transport.head.Load(); latest && height != 0 {
				call.Params[0] = json.RawMessage(fmt.Sprintf(`"0x%x"`, height))
				body, err = json.Marshal(call)
				if err != nil {
					t.Error(err)
					return
				}
			} else if height != 0 && !latest {
				var tag string
				if err := json.Unmarshal(call.Params[0], &tag); err != nil {
					t.Error(err)
					return
				}
				requested, err := strconv.ParseInt(tag, 0, 64)
				if err != nil {
					t.Error(err)
					return
				}
				if requested > height {
					http.Error(w, "block above observed head", http.StatusBadGateway)
					return
				}
			}
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		forward.ServeHTTP(w, req)
	}))
	t.Cleanup(proxy.Close)
	options := r.doc.Committed
	options.Endpoints = make(map[string]string, len(options.Endpoints))
	for id := range r.doc.Committed.Endpoints {
		options.Endpoints[id] = proxy.URL
	}
	provider, err := source.NewCommittedSource(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = provider.Close() })
	r.provider = provider
	return transport
}

func TestCanonicalSourceReadinessRefreshPreservesIngestionState(t *testing.T) {
	r, _ := canonicalSourceReadinessCase(t)
	ctx := context.Background()
	rows := func() string {
		t.Helper()
		var result string
		if err := r.pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(h) ORDER BY height),'[]'::jsonb)::text FROM indexer.indexer_committed_heads h WHERE chain_id=125 AND stream_id=$1`, r.stream).Scan(&result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if before := rows(); before != "[]" {
		t.Fatalf("initial committed heads: %s", before)
	}
	if err := r.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if after := rows(); after != "[]" {
		t.Fatalf("refresh retained committed heads: %s", after)
	}
	durable := new(atomic.Uint64)
	head := &canonicalRuntimeHead{source: r.provider, store: r.store, stream: r.stream, height: durable}
	if _, err := head.Head(ctx); err != nil {
		t.Fatal(err)
	}
	before, height := rows(), durable.Load()
	if before == "[]" || height != uint64(r.doc.InitialCheckpointHeight) {
		t.Fatalf("ingestion state: heads=%s durable=%d", before, height)
	}
	observed := time.Now().UTC()
	for range 4 {
		if err := r.refresh(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if after := rows(); after != before || durable.Load() != height {
		t.Fatalf("refresh changed ingestion state: heads=%s durable=%d", after, durable.Load())
	}
	if err := r.provider.ReportDurableHeight(observed, height); err != nil {
		t.Fatalf("refresh reported a newer durable observation: %v", err)
	}
	if err := r.provider.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.refresh(ctx); err == nil {
		t.Fatal("closed source refreshed successfully")
	}
	if after := rows(); after != before || durable.Load() != height {
		t.Fatalf("failed refresh changed ingestion state: heads=%s durable=%d", after, durable.Load())
	}
}

func TestCanonicalSourceReadinessFollowsMovingWatermarkAtChainTip(t *testing.T) {
	r, _ := canonicalSourceReadinessCase(t)
	transport := canonicalReadinessRecordedTransport(t, r)
	transport.head.Store(r.doc.InitialCheckpointHeight)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	durable := new(atomic.Uint64)
	head := &canonicalRuntimeHead{source: r.provider, store: r.store, stream: r.stream, height: durable}
	if _, err := head.Head(ctx); err != nil {
		t.Fatal(err)
	}
	fence, err := r.store.AcquireLease(ctx, 125, r.stream, "ingestion", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	const advances = 10
	done := make(chan error, 1)
	stopped := make(chan struct{})
	defer func() { cancel(); <-stopped }()
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for offset := int64(1); offset <= advances; offset++ {
			select {
			case <-ctx.Done():
				done <- ctx.Err()
				return
			case <-ticker.C:
			}
			height := r.doc.InitialCheckpointHeight + offset
			transport.head.Store(height)
			if _, err := head.Head(ctx); err != nil {
				done <- err
				return
			}
			block, err := r.provider.BlockAt(ctx, uint64(height))
			if err == nil {
				err = r.store.CommitCanonical(ctx, store.CanonicalBatch{ChainID: 125, StreamID: r.stream, Owner: "ingestion", BatchID: fmt.Sprintf("tip-%d", height), Fence: fence, PreviousHeight: height - 1, Blocks: []store.BlockCompleteness{{Height: height, Hash: strings.ToLower(block.Hash.Hex()), FetchComplete: true, RequiredDecodeComplete: true}}}, nil)
			}
			if err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	for {
		if err := r.refresh(ctx); err != nil {
			t.Fatalf("refresh during ingestion: %v", err)
		}
		if err := r.Ready(ctx); err != nil {
			t.Fatalf("readiness during ingestion: %v", err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
			watermark, err := r.store.GetCanonicalWatermark(ctx, 125, r.stream)
			if err != nil || watermark != r.doc.InitialCheckpointHeight+advances || transport.latestReads.Load() <= advances+1 {
				t.Fatalf("moving stream watermark=%d head reads=%d: %v", watermark, transport.latestReads.Load(), err)
			}
			if err := r.refresh(ctx); err != nil {
				t.Fatalf("refresh at newest chain height: %v", err)
			}
			var unhealthy int
			if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_committed_heads WHERE stream_id=$1 AND integrity<>'healthy'`, r.stream).Scan(&unhealthy); err != nil || unhealthy != 0 {
				t.Fatalf("committed heads unhealthy=%d: %v", unhealthy, err)
			}
			return
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestCanonicalSourceReadinessHoldsObservedCheckpointWhileWatermarkAdvances(t *testing.T) {
	r, _ := canonicalSourceReadinessCase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	next, err := r.provider.BlockAt(ctx, uint64(r.doc.InitialCheckpointHeight+1))
	if err != nil {
		t.Fatal(err)
	}
	fence, err := r.store.AcquireLease(ctx, 125, r.stream, "ingestion", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	transport := canonicalReadinessRecordedTransport(t, r)
	transport.head.Store(r.doc.InitialCheckpointHeight)
	lock := canonicalSourceReadinessLock(t, r)
	result := make(chan error, 1)
	go func() { result <- r.refresh(ctx) }()
	for {
		var waiting int
		if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT raw_complete_height,typed_complete_height%'`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting == 1 {
			break
		}
		select {
		case err := <-result:
			t.Fatalf("refresh did not wait for watermark: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
	transactional := store.NewFromDBTX(lock)
	if err := transactional.CommitCanonical(ctx, store.CanonicalBatch{ChainID: 125, StreamID: r.stream, Owner: "ingestion", BatchID: "advance-checkpoint", Fence: fence, PreviousHeight: r.doc.InitialCheckpointHeight, Blocks: []store.BlockCompleteness{{Height: int64(next.Height), Hash: strings.ToLower(next.Hash.Hex()), FetchComplete: true, RequiredDecodeComplete: true}}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := lock.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatalf("refresh with advanced watermark: %v", err)
	}
	if err := r.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	for _, incomplete := range []bool{false, true} {
		doc := r.doc
		want := errCanonicalCheckpointMismatch
		if _, err := r.pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,0,true,$4)`, r.stream, doc.InitialCheckpointHeight, "0x"+strings.Repeat("b", 64), !incomplete); err != nil {
			t.Fatal(err)
		}
		if incomplete {
			r.doc.InitialCheckpointHeight--
			want = errCanonicalCheckpointIncomplete
		}
		if err := r.refresh(ctx); !errors.Is(err, want) {
			t.Fatalf("observed checkpoint: %v", err)
		}
		if err := r.Ready(ctx); !errors.Is(err, want) {
			t.Fatalf("observed checkpoint readiness: %v", err)
		}
		r.doc = doc
		if _, err := r.pool.Exec(ctx, `DELETE FROM indexer.indexer_block_completeness WHERE stream_id=$1 AND height=$2`, r.stream, doc.InitialCheckpointHeight); err != nil {
			t.Fatal(err)
		}
		if err := r.refresh(ctx); err != nil {
			t.Fatalf("observed checkpoint recovery: %v", err)
		}
	}
	transport.head.Store(int64(next.Height))
	if err := r.refresh(ctx); err != nil {
		t.Fatalf("refresh after source catches watermark: %v", err)
	}
}

func TestCanonicalSourceReadinessHoldsAndLogsCheckpointFailures(t *testing.T) {
	r, clock := canonicalSourceReadinessCase(t)
	transport := canonicalReadinessRecordedTransport(t, r)
	ctx := context.Background()
	var output bytes.Buffer
	r.logger = slog.New(slog.NewJSONHandler(&output, nil))
	if err := r.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	passedAt := r.lastSuccess
	if _, err := r.pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,0,true,true)`, r.stream, r.doc.InitialCheckpointHeight, "0x"+strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	want := errCanonicalCheckpointMismatch
	doc := r.doc
	for _, incomplete := range []bool{false, true} {
		if incomplete {
			r.doc.InitialCheckpointHeight--
			if _, err := r.pool.Exec(ctx, `UPDATE indexer.indexer_block_completeness SET required_decode_complete=false WHERE stream_id=$1`, r.stream); err != nil {
				t.Fatal(err)
			}
			want = errCanonicalCheckpointIncomplete
		}
		for range 3 {
			clock.Add(int64(time.Second))
			if err := r.refresh(ctx); !errors.Is(err, want) {
				t.Fatalf("checkpoint refresh: %v", err)
			}
			if err := r.Ready(ctx); !errors.Is(err, want) {
				t.Fatalf("checkpoint readiness: %v", err)
			}
		}
	}
	r.doc = doc
	if _, err := r.pool.Exec(ctx, `DELETE FROM indexer.indexer_block_completeness WHERE stream_id=$1`, r.stream); err != nil {
		t.Fatal(err)
	}
	transport.failure.Store(1)
	for range 3 {
		if err := r.refresh(ctx); err == nil {
			t.Fatal("unavailable source refreshed successfully")
		}
		if err := r.Ready(ctx); !errors.Is(err, want) {
			t.Fatalf("read error cleared incomplete checkpoint: %v", err)
		}
	}
	if !r.lastSuccess.Equal(passedAt) {
		t.Fatal("failed refresh renewed success time")
	}
	transport.failure.Store(0)
	if err := r.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.Ready(ctx); err != nil || !r.lastSuccess.After(passedAt) {
		t.Fatalf("recovered readiness: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("transition lines=%d: %s", len(lines), output.String())
	}
	for i, wantClass := range []string{"checkpoint_mismatch", "checkpoint_incomplete", "semantic_rpc", "none"} {
		var entry map[string]any
		if err := json.Unmarshal([]byte(lines[i]), &entry); err != nil {
			t.Fatal(err)
		}
		step := "checkpoint"
		if i >= 2 {
			step = "source_height"
		}
		if entry["step"] != step || entry["class"] != wantClass {
			t.Fatalf("transition: %s", lines[i])
		}
		t.Log(lines[i])
	}
}

func TestCanonicalSourceReadinessLogsReadTransitions(t *testing.T) {
	for _, step := range []string{"source_height", "watermark", "checkpoint"} {
		t.Run(step, func(t *testing.T) {
			r, _ := canonicalSourceReadinessCase(t)
			transport := canonicalReadinessRecordedTransport(t, r)
			var output bytes.Buffer
			r.logger = slog.New(slog.NewJSONHandler(&output, nil))
			ctx := context.Background()
			setFailure := func(on bool) {
				t.Helper()
				if step == "watermark" {
					raw := r.doc.InitialCheckpointHeight
					if on {
						raw++
					}
					if _, err := r.pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=$2 WHERE stream_id=$1`, r.stream, raw); err != nil {
						t.Fatal(err)
					}
				} else {
					var fault int32
					if on && step == "source_height" {
						fault = 1
					}
					if on && step == "checkpoint" {
						fault = 2
					}
					transport.failure.Store(fault)
				}
			}
			class := "semantic_rpc"
			if step == "watermark" {
				class = "invalid_watermark"
			}
			for _, initiallyReady := range []bool{false, true} {
				if initiallyReady {
					if err := r.refresh(ctx); err != nil {
						t.Fatal(err)
					}
					if output.Len() != 0 {
						t.Fatalf("repeated success logged: %s", output.String())
					}
				}
				setFailure(true)
				for range 3 {
					if err := r.refresh(ctx); err == nil {
						t.Fatal("unavailable read refreshed successfully")
					}
				}
				if !initiallyReady {
					if err := r.Ready(ctx); !errors.Is(err, errCanonicalSourceUnrefreshed) {
						t.Fatalf("readiness without success: %v", err)
					}
				}
				setFailure(false)
				for range 3 {
					if err := r.refresh(ctx); err != nil {
						t.Fatal(err)
					}
				}
				lines := strings.Split(strings.TrimSpace(output.String()), "\n")
				if len(lines) != 2 {
					t.Fatalf("transition lines=%d: %s", len(lines), output.String())
				}
				for i, line := range lines {
					var entry map[string]any
					if err := json.Unmarshal([]byte(line), &entry); err != nil {
						t.Fatal(err)
					}
					wantClass, state := class, "unavailable"
					if i == 1 {
						wantClass, state = "none", "ready"
					}
					if len(entry) != 6 || entry["msg"] != "canonical source refresh" || entry["step"] != step || entry["class"] != wantClass || entry["state"] != state {
						t.Fatalf("transition: %s", line)
					}
					t.Log(line)
				}
				if strings.Contains(output.String(), "http") || strings.Contains(output.String(), "source read unavailable") {
					t.Fatalf("source details in log: %s", output.String())
				}
				output.Reset()
			}
		})
	}
}
