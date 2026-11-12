package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/migrations"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
)

func TestPublicProjectionAndSnapshotReadCommittedFrame(t *testing.T) {
	t.Setenv(internaltest.ExternalPostgresEnv, os.Getenv("API_TEST_DATABASE_URL"))
	pool := internaltest.NewPostgres(t)
	ctx := context.Background()
	migration, err := migrations.Migrations().ReadFile("00001_api_stream.sql")
	if err != nil {
		t.Fatal(err)
	}
	_, up, found := strings.Cut(string(migration), "-- +goose Up")
	if !found {
		t.Fatal("stream migration missing Up")
	}
	up, _, found = strings.Cut(up, "-- +goose Down")
	if !found {
		t.Fatal("stream migration missing Down")
	}
	if _, err = pool.Exec(ctx, up); err != nil {
		t.Fatal(err)
	}
	config := productionReadConfig()
	config.StreamSchema = "api_stream"
	model, err := store.NewAccountingReadModel(pool, config)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := broker.NewPostgresStream(pool, "api_stream", bytes.Repeat([]byte{9}, 32), sharedauth.HTTPStreamAuthority{})
	if err != nil {
		t.Fatal(err)
	}
	const address = productionAddress
	const watermark = `{"chainId":"125","height":"1","blockHash":"0x1111111111111111111111111111111111111111111111111111111111111111","projectionRevision":"1","observedAt":"2026-10-09T00:00:00.000Z","integrity":"healthy"}`
	snapshotID := uuid.NewString()
	payload, err := json.Marshal(map[string]any{"snapshotId": snapshotID, "category": "trending", "modelVersion": "fixture", "computedAt": "2026-10-09T00:00:00.000Z", "watermark": json.RawMessage(watermark), "items": []map[string]any{{"poolAddress": address, "category": "trending", "rank": 1, "score": "100", "modelVersion": "fixture", "computedAt": "2026-10-09T00:00:00.000Z"}}, "totalEligible": "1", "truncated": false, "replace": true})
	if err != nil {
		t.Fatal(err)
	}
	const topic = "market.rankings.trending"
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	frame := broker.Frame{EventID: uuid.NewString(), Topic: topic, Sequence: "1", Revision: "1", OccurredAt: "2026-10-12T00:00:00.000Z", Watermark: json.RawMessage(watermark), Payload: payload}
	if err = stream.AppendProjectionTx(ctx, tx, frame); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	projections, err := model.PublicProjections(ctx, []string{topic})
	if err != nil || len(projections) != 1 || projections[0].Topic != topic || projections[0].Sequence != "1" {
		t.Fatalf("committed projections %+v, %v", projections, err)
	}
	var partition string
	if err = pool.QueryRow(ctx, `SELECT partition_id::text FROM api_stream.stream_projection WHERE topic=$1`, topic).Scan(&partition); err != nil {
		t.Fatal(err)
	}
	raw, err := stream.PublicSnapshot(ctx, []string{topic})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		ResumeCursor string `json:"resumeCursor"`
	}
	if err = json.Unmarshal(raw, &snapshot); err != nil || snapshot.ResumeCursor == "" {
		t.Fatalf("cursor %s: %v", raw, err)
	}
	page, err := model.TopicSnapshotAt(ctx, []string{topic}, map[string]string{partition: "1"}, snapshotID, snapshot.ResumeCursor)
	if err != nil || !bytes.Contains(page, []byte(topic)) {
		t.Fatalf("snapshot %s: %v", page, err)
	}
	for _, tc := range []struct {
		name    string
		topics  []string
		offsets map[string]string
		id      string
		want    error
	}{
		{"invalid partition", []string{topic}, map[string]string{"invalid": "1"}, snapshotID, store.ErrDataIntegrity},
		{"invalid sequence", []string{topic}, map[string]string{partition: "0"}, snapshotID, store.ErrDataIntegrity},
		{"unlisted topic", []string{"market.rankings.volume"}, map[string]string{partition: "1"}, snapshotID, store.ErrDataIntegrity},
		{"invalid snapshot id", []string{topic}, map[string]string{partition: "1"}, "invalid", store.ErrDataIntegrity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := model.TopicSnapshotAt(ctx, tc.topics, tc.offsets, tc.id, snapshot.ResumeCursor); !errors.Is(err, tc.want) {
				t.Fatalf("snapshot error %v, want %v", err, tc.want)
			}
		})
	}
	if _, err := model.PublicProjections(ctx, []string{"private.notifications." + address}); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("private projection accepted: %v", err)
	}
	if _, err := model.PublicProjections(ctx, nil); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("empty projection scope: %v", err)
	}
	if _, err := model.TopicSnapshotAt(ctx, []string{topic}, nil, snapshotID, snapshot.ResumeCursor); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("empty offsets: %v", err)
	}
	if _, err := model.TopicSnapshotAt(ctx, []string{topic}, map[string]string{partition: "999"}, snapshotID, snapshot.ResumeCursor); err == nil {
		t.Fatal("missing outbox frame accepted")
	}
	if _, err := pool.Exec(ctx, `UPDATE api_stream.stream_projection SET frame='{}'::bytea WHERE topic=$1`, topic); err != nil {
		t.Fatal(err)
	}
	if _, err := model.PublicProjections(ctx, []string{topic}); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("malformed projection: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO api_stream.stream_delivery_outbox(partition_id,sequence,event_id,frame) VALUES($1,2,'invalid-frame','{}'::bytea)`, partition); err != nil {
		t.Fatal(err)
	}
	if _, err := model.TopicSnapshotAt(ctx, []string{topic}, map[string]string{partition: "2"}, snapshotID, snapshot.ResumeCursor); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("malformed snapshot: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := model.PublicProjections(canceled, []string{topic}); err == nil {
		t.Fatal("canceled projection read accepted")
	}
	if _, err := model.TopicSnapshotAt(canceled, []string{topic}, map[string]string{partition: "1"}, snapshotID, snapshot.ResumeCursor); err == nil {
		t.Fatal("canceled snapshot read accepted")
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE api_stream.stream_projection RENAME TO unavailable_projection`); err != nil {
		t.Fatal(err)
	}
	if _, err := model.PublicProjections(ctx, []string{topic}); err == nil {
		t.Fatal("missing projection source accepted")
	}

}
