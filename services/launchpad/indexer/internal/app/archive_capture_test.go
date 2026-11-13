package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/processor"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestRuntimeArchivesCommittedEmptyBlocksAndResumesAfterPairedManifests(t *testing.T) {
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	for _, build := range []func(string) (string, error){delivery.PartitionSchemaDDL, delivery.ReplayDDL} {
		ddl, err := build("indexer")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	applyRuntimeCreationSchema(t, pool)
	parameters := DefaultDeliveryParameters()
	parameters.MaxDeliveryLagMS = 60000
	documents, err := RenderRuntimeDocuments(renderInputs(t), parameters)
	if err != nil {
		t.Fatal(err)
	}
	var receiptDoc receiptRuntimeDocument
	if err = json.Unmarshal(documents["receipt.json"].Bytes, &receiptDoc); err != nil {
		t.Fatal(err)
	}
	_, lifecycle, _ := lifecycleDocuments(t)
	endpoint := status1695RPC(t, ctx)
	for id := range lifecycle.Committed.Endpoints {
		lifecycle.Committed.Endpoints[id] = endpoint
	}
	provider, err := source.NewCommittedSource(ctx, lifecycle.Committed)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	client, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	authority, err := receipt.NewRPCQualification(ctx, client, receiptDoc.Production)
	if err != nil {
		t.Fatal(err)
	}
	qualified := &source.HistoricalQualifiedSource{History: &source.HistoricalSource{RPC: provider}, Authority: authority, DurableHeight: func() uint64 { return 33963717 }}
	batch, issued, err := qualified.FetchQualifiedBatch(ctx, 33963717, 33963717)
	if err != nil || len(batch.Blocks) != 1 || len(batch.Records) != 0 || len(issued) != 0 {
		t.Fatalf("recorded empty batch: %+v %v", batch, err)
	}
	st := store.New(pool)
	const stream = "runtime-archive"
	fence, err := st.AcquireLease(ctx, 125, stream, "archiver", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,$1,33963716,33963716)`, stream); err != nil {
		t.Fatal(err)
	}
	batch.StreamID = stream
	batch.Owner = "archiver"
	batch.Fence = fence
	batch.PreviousHeight = 33963716
	batch.BatchID = "empty-source"
	if err = st.CommitCanonical(ctx, batch, nil); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	var mu sync.Mutex
	puts := 0
	objects := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		relative := strings.TrimPrefix(r.URL.Path, "/")
		if strings.Contains(relative, "..") {
			w.WriteHeader(400)
			return
		}
		path := filepath.Join(directory, filepath.FromSlash(relative))
		switch r.Method {
		case http.MethodPut:
			body, e := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if e != nil {
				t.Error(e)
				w.WriteHeader(500)
				return
			}
			if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
				t.Error(e)
				w.WriteHeader(500)
				return
			}
			if e = os.WriteFile(path, body, 0600); e != nil {
				t.Error(e)
				w.WriteHeader(500)
				return
			}
			mu.Lock()
			puts++
			mu.Unlock()
			w.Header().Set("ETag", `"stored"`)
		case http.MethodHead, http.MethodGet:
			body, e := os.ReadFile(path)
			if e != nil {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.Header().Set("ETag", `"stored"`)
			if r.Method == http.MethodGet {
				if _, e = w.Write(body); e != nil {
					t.Error(e)
				}
			}
		default:
			w.WriteHeader(405)
		}
	}))
	defer objects.Close()
	storageClient, err := storage.New(ctx, storage.Config{Endpoint: objects.URL, Region: "auto", Bucket: "archive", AccessKeyID: "local-access", SecretAccessKey: "local-secret"})
	if err != nil {
		t.Fatal(err)
	}
	durable, err := delivery.NewStore(pool, "indexer", delivery.Policy{Subjects: map[string]bool{"archive": true}, EventTypes: map[string]bool{"archive": true}})
	if err != nil {
		t.Fatal(err)
	}
	pin := delivery.Hash([]byte("archive-code"))
	raw, err := delivery.NewArchive(durable, storageClient, "canonical", stream, "chain-125", pin, pin, pin)
	if err != nil {
		t.Fatal(err)
	}
	typed := &processor.Archive{Store: st, Objects: storageClient, Source: qualified}
	policy := lifecycleArchiveDocument{FromHeight: 33963717, BatchBlocks: 32}
	if err = captureArchiveBatch(ctx, pool, st, typed, raw, policy, stream); err != nil {
		t.Fatal(err)
	}
	manifest, err := st.FindArchive(ctx, stream, 33963717, 33963717)
	if err != nil || manifest.EventCount != 0 || manifest.TypedCount != 0 {
		t.Fatalf("typed empty archive: %+v %v", manifest, err)
	}
	segment, err := typed.Restore(ctx, stream, 33963717, 33963717)
	if err != nil || len(segment.Blocks) != 1 || len(segment.Records) != 0 {
		t.Fatalf("restored empty archive: %+v %v", segment, err)
	}
	var rawBody []byte
	if err = pool.QueryRow(ctx, `SELECT manifest FROM indexer.delivery_archive_manifests WHERE source_stream=$1`, stream).Scan(&rawBody); err != nil {
		t.Fatal(err)
	}
	var rawManifest delivery.ArchiveManifest
	if err = json.Unmarshal(rawBody, &rawManifest); err != nil || rawManifest.RawCount != 0 || len(rawManifest.Blocks) != 1 {
		t.Fatalf("raw empty archive: %s %v", rawBody, err)
	}
	mu.Lock()
	before := puts
	mu.Unlock()
	if err = captureArchiveBatch(ctx, pool, st, typed, raw, policy, stream); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	after := puts
	mu.Unlock()
	if before == 0 || after != before {
		t.Fatalf("completed range uploaded again: %d %d", before, after)
	}
	if _, err = pool.Exec(ctx, `ALTER TABLE indexer.delivery_archive_manifests RENAME TO unavailable_manifests`); err != nil {
		t.Fatal(err)
	}
	if err = captureArchiveBatch(ctx, pool, st, typed, raw, policy, stream); err == nil {
		t.Fatal("unavailable archive index admitted")
	}
	if _, err = pool.Exec(ctx, `ALTER TABLE indexer.unavailable_manifests RENAME TO delivery_archive_manifests`); err != nil {
		t.Fatal(err)
	}
	if err = captureArchiveBatch(ctx, pool, st, nil, raw, lifecycleArchiveDocument{FromHeight: 33963716, BatchBlocks: 1}, stream); err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("archive gap ignored: %v", err)
	}
	if !bytes.Contains(rawBody, []byte("33963717")) {
		t.Fatal("source height absent from retained manifest")
	}
}
