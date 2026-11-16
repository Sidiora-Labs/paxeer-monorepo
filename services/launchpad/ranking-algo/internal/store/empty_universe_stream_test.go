package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	v1 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEmptyUniverseSharedCategoryTopics(t *testing.T) {
	db := internaltest.NewRankingRuntimeDatabase(t)
	pool, err := pgxpool.New(context.Background(), db.RuntimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	runID := uuid.NewString()
	cutoff := time.Now().UTC().Truncate(time.Second)
	blockHash := "0x" + strings.Repeat("a", 64)
	watermark := v1.Watermark{ChainId: "125", Height: "1", BlockHash: blockHash, ProjectionRevision: "1", ObservedAt: cutoff.Format("2006-01-02T15:04:05.000Z"), Integrity: "healthy"}
	provenance := ranker.Provenance{Watermark: watermark, UniverseDigest: strings.Repeat("b", 64), StatsRevision: "1", ModerationRevision: strings.Repeat("c", 64), ActivityRevision: "1", WindowStart: cutoff.Unix() - 86400, WindowEnd: cutoff.Unix()}
	inputDigest := strings.Repeat("d", 64)
	outputDigest := strings.Repeat("e", 64)
	poolAddress := "0x" + strings.Repeat("1", 40)
	categories := []string{"trending", "breakout"}

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, category := range categories {
		artifact := map[string]any{"category": category, "entries": []map[string]any{{"poolAddress": poolAddress, "score": "1", "rank": 1}}}
		publicHeader := map[string]any{"algorithmVersion": ranker.AlgorithmVersion, "cutoffTime": cutoff.Unix(), "provenance": provenance, "total": 1}
		artifactJSON, _ := json.Marshal(artifact)
		headerJSON, _ := json.Marshal(publicHeader)
		exec(`INSERT INTO ranking.snapshots(run_id,category,input_digest,output_digest,artifact,public_header) VALUES($1,$2,$3,$4,$5,$6)`, runID, category, inputDigest, outputDigest, artifactJSON, headerJSON)
		exec(`INSERT INTO ranking.snapshot_entries(run_id,category,rank,pool_address,score) VALUES($1,$2,1,$3,'1')`, runID, category, poolAddress)
		exec(`INSERT INTO ranking.publication_outbox(run_id,category,artifact,category_sequence) VALUES($1,$2,$3,1)`, runID, category, artifactJSON)
	}

	writer := delivery.StreamWriter{Schema: "api_stream"}
	updates := make([]delivery.StreamUpdate, 0, len(categories))
	for _, category := range categories {
		header := ranker.Header{RunID: runID, Category: category, InputDigest: inputDigest, OutputDigest: outputDigest, AlgorithmVersion: ranker.AlgorithmVersion, CutoffTime: cutoff.Unix(), CompletedAt: cutoff, Revision: 1, Total: 1, Provenance: provenance}
		payload, err := WireSnapshot(header, []ranker.RankedItem{{Address: poolAddress, Score: "1", Rank: 1}})
		if err != nil {
			t.Fatal(err)
		}
		payloadJSON, _ := json.Marshal(payload)
		watermarkJSON, _ := json.Marshal(watermark)
		updates = append(updates, delivery.StreamUpdate{EventID: uuid.NewSHA1(uuid.MustParse(runID), []byte(category)).String(), Topic: "market.rankings." + category, Revision: "1", OccurredAt: payload.ComputedAt, Watermark: watermarkJSON, Payload: payloadJSON, Source: delivery.StreamSourceProof{Kind: "ranking_commit", SourceID: runID, Sequence: "1", SourceDigest: outputDigest}})
	}
	for _, update := range updates {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err = writer.AppendTx(ctx, tx, update); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("append %s: %v", update.Topic, err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for _, update := range updates {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err = writer.AppendTx(ctx, tx, update); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("replay %s: %v", update.Topic, err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}

}
