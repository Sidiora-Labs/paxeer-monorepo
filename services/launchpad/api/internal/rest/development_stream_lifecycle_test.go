package rest

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
)

func TestDevelopmentStreamPrunesExpiredDurableState(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	var logs bytes.Buffer
	d, err := NewDevelopmentStream(ctx, DevelopmentStreamOptions{Pool: db, Store: store.New(db, nil), SigningKey: []byte(strings.Repeat("k", 32)), Authority: sharedauth.HTTPStreamAuthority{}, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	if err != nil {
		t.Fatal(err)
	}
	d.report(errors.New("source unavailable"))
	if d.lastErr != "source unavailable" || !strings.Contains(logs.String(), "source unavailable") {
		t.Fatalf("degraded projection status: %q, %s", d.lastErr, logs.String())
	}
	d.report(nil)
	if d.lastErr != "" || !strings.Contains(logs.String(), "healthy") {
		t.Fatalf("recovered projection status: %q, %s", d.lastErr, logs.String())
	}
	if _, err := db.Exec(ctx, `INSERT INTO api_stream_dev.stream_projection(topic,sequence,frame) VALUES('market.price.0x1111111111111111111111111111111111111111',1,'{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO api_stream_dev.stream_delivery_outbox(partition_id,sequence,event_id,frame,created_at) SELECT partition_id,1,'expired-event','{}',clock_timestamp()-interval '25 hours' FROM api_stream_dev.stream_projection`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO api_stream_dev.stream_snapshots(snapshot_id,account_id,session_id,auth_epoch,filter_digest,generation,scope,initial_offsets,created_at,expires_at) SELECT '00000000-0000-7000-8000-000000000001','00000000-0000-7000-8000-000000000002','00000000-0000-7000-8000-000000000003',0,$1,generation,'{}','{}',clock_timestamp()-interval '26 hours',clock_timestamp()-interval '25 hours' FROM api_stream_dev.stream_generation`, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if err := d.prune(ctx); err != nil {
		t.Fatal(err)
	}
	var deliveries, snapshots int
	if err := db.QueryRow(ctx, `SELECT (SELECT count(*) FROM api_stream_dev.stream_delivery_outbox),(SELECT count(*) FROM api_stream_dev.stream_snapshots)`).Scan(&deliveries, &snapshots); err != nil || deliveries != 0 || snapshots != 0 {
		t.Fatalf("expired records remain: deliveries=%d snapshots=%d err=%v", deliveries, snapshots, err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	finished := make(chan struct{})
	go func() { d.Run(canceled); close(finished) }()
	select {
	case <-finished:
		if d.lastErr != "NETWORK_RPC_URL is not configured" {
			t.Fatalf("run status: %q", d.lastErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled projector did not stop")
	}
}

func TestDevelopmentStreamComparesStatsAndClassifiesTransport(t *testing.T) {
	prior := map[string]any{"poolAddress": "0x1111111111111111111111111111111111111111", "volume24hRaw": "10", "watermark": map[string]any{"height": "1"}}
	next := map[string]any{"poolAddress": prior["poolAddress"], "volume24hRaw": "10", "watermark": map[string]any{"height": "2"}}
	if !sameStatsContent(prior, next) {
		t.Fatal("watermark alone changed the stats document")
	}
	next["volume24hRaw"] = "11"
	if sameStatsContent(prior, next) {
		t.Fatal("changed volume was treated as the same stats document")
	}
	for _, tc := range []struct {
		err  error
		want bool
	}{{nil, false}, {errChainReferenceUnavailable, false}, {errors.New("connection closed"), true}} {
		if got := isTransport(tc.err); got != tc.want {
			t.Fatalf("transport classification of %v = %t, want %t", tc.err, got, tc.want)
		}
	}
}
