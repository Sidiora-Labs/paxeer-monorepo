package rest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestDevelopmentStreamRequiresCompleteProviders(t *testing.T) {
	db := internaltest.NewPostgres(t)
	options := DevelopmentStreamOptions{Pool: db, Store: store.New(db, nil), Authority: sharedauth.HTTPStreamAuthority{}, SigningKey: []byte(strings.Repeat("k", 32))}
	for _, tc := range []struct {
		name   string
		change func(*DevelopmentStreamOptions)
	}{
		{"pool", func(o *DevelopmentStreamOptions) { o.Pool = nil }},
		{"store", func(o *DevelopmentStreamOptions) { o.Store = nil }},
		{"authority", func(o *DevelopmentStreamOptions) { o.Authority = nil }},
		{"signing key", func(o *DevelopmentStreamOptions) { o.SigningKey = o.SigningKey[:31] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := options
			tc.change(&candidate)
			stream, err := NewDevelopmentStream(t.Context(), candidate)
			if stream != nil || err == nil || err.Error() != "development stream providers required" {
				t.Fatalf("incomplete providers: %v, %v", stream, err)
			}
		})
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if stream, err := NewDevelopmentStream(canceled, options); stream != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled schema initialization: %v, %v", stream, err)
	}
	options.ChainRPCURL = "://invalid"
	if stream, err := NewDevelopmentStream(t.Context(), options); stream != nil || err == nil || err.Error() != "development chain RPC URL invalid" {
		t.Fatalf("invalid RPC binding: %v, %v", stream, err)
	}
}

func TestDevelopmentStreamRejectsUnreadableTopicHeads(t *testing.T) {
	db := internaltest.NewPostgres(t)
	ctx := t.Context()
	if err := ensureDevelopmentStreamSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	const topic = "market.stats.0x1111111111111111111111111111111111111111"
	for _, frame := range []string{`{`, `{"revision":"invalid","payload":{}}`} {
		if _, err := db.Exec(ctx, `INSERT INTO api_stream_dev.stream_projection(topic,sequence,frame) VALUES($1,1,$2) ON CONFLICT(topic) DO UPDATE SET frame=EXCLUDED.frame`, topic, []byte(frame)); err != nil {
			t.Fatal(err)
		}
		tx, err := db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, readErr := readTopicHead(ctx, tx, topic)
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		if !errors.Is(readErr, broker.ErrFrame) {
			t.Fatalf("invalid topic head %s: %v", frame, readErr)
		}
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := readTopicHead(canceled, tx, topic); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled topic head: %v", err)
	}
}

func TestDevelopmentStreamSerializesProjectionAndReportsMissingTables(t *testing.T) {
	db := internaltest.NewPostgres(t)
	ctx := t.Context()
	d, err := NewDevelopmentStream(ctx, DevelopmentStreamOptions{Pool: db, Store: store.New(db, nil), Authority: sharedauth.HTTPStreamAuthority{}, SigningKey: []byte(strings.Repeat("k", 32))})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Rollback(ctx) }()
	if _, err := owner.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, developmentStreamLock); err != nil {
		t.Fatal(err)
	}
	contender, locked, err := d.beginProjection(ctx)
	if contender != nil || locked || err != nil {
		t.Fatalf("concurrent projector acquired lock: %v, %t, %v", contender, locked, err)
	}
	if err := owner.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if tx, locked, err := d.beginProjection(canceled); tx != nil || locked || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled projector: %v, %t, %v", tx, locked, err)
	}
	unknown, err := d.UnknownTopics(ctx, []string{"private.notifications.account", "market.stats.invalid", "market.rankings.trending"})
	if err != nil || len(unknown) != 0 {
		t.Fatalf("topics without pool scope: %v, %v", unknown, err)
	}
	if _, err := db.Exec(ctx, `DROP TABLE indexer.pools CASCADE; DROP TABLE api_stream_dev.stream_projection CASCADE; DROP TABLE api_stream_dev.stream_delivery_outbox`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		read func() error
	}{
		{"pool lookup", func() error {
			_, err := d.UnknownTopics(ctx, []string{"market.stats.0x1111111111111111111111111111111111111111"})
			return err
		}},
		{"stats sweep", func() error { return d.projectStats(ctx, nil, true) }},
		{"stats inputs", func() error {
			return d.projectStatsBatch(ctx, []string{"0x1111111111111111111111111111111111111111"}, nil)
		}},
		{"retention", func() error { return d.prune(ctx) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cause *pgconn.PgError
			if err := tc.read(); !errors.As(err, &cause) || cause.Code != "42P01" {
				t.Fatalf("missing table cause: %v", err)
			}
		})
	}
}
