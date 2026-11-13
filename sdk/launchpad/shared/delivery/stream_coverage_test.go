package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func committedPriceFrame(t *testing.T) map[string]any {
	t.Helper()
	asset := func(kind string, chain, currency any, decimals int) map[string]any {
		return map[string]any{"kind": kind, "chainId": chain, "address": nil, "currencyCode": currency, "decimals": decimals}
	}
	return map[string]any{
		"eventId": "price_1", "topic": "market.price.0x" + strings.Repeat("a", 40),
		"sequence": "1", "revision": "1", "occurredAt": "2026-01-02T03:04:05.000Z",
		"watermark": nil, "coalesced": false, "sourceSequenceFirst": nil, "sourceSequenceLast": nil,
		"payload": map[string]any{
			"numerator": "1", "denominator": "2",
			"baseAsset":  asset("native", "125", nil, 18),
			"quoteAsset": asset("fiat_display", nil, "USD", 2),
			"source":     map[string]any{"kind": "external_display", "amountBasis": "provider_display", "sourceEventId": nil, "position": nil, "providerId": "fixture", "observedAt": "2026-01-02T03:04:05.000Z", "sourceReference": nil},
		},
	}
}

func TestCommittedPriceFrameAdmission(t *testing.T) {
	encode := func(frame map[string]any) []byte {
		t.Helper()
		raw, err := json.Marshal(frame)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	base := committedPriceFrame(t)
	actual, err := ParseCommittedStreamFrame(encode(base))
	if err != nil || actual.EventID != "price_1" || actual.Sequence != "1" || actual.Topic != base["topic"] {
		t.Fatalf("committed frame: %+v, %v", actual, err)
	}
	for _, tc := range []struct {
		name string
		key  string
		val  any
		want error
	}{
		{"coalesced", "coalesced", true, ErrSchema},
		{"source bound", "sourceSequenceFirst", "1", ErrIntegrity},
		{"zero sequence", "sequence", "0", ErrIntegrity},
		{"zero revision", "revision", "0", ErrSchema},
		{"unknown topic", "topic", "market.unknown.pool", ErrSchema},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame := committedPriceFrame(t)
			frame[tc.key] = tc.val
			if _, err := ParseCommittedStreamFrame(encode(frame)); !errors.Is(err, tc.want) {
				t.Fatalf("frame change %s: %v", tc.key, err)
			}
		})
	}
	if _, err := ParseCommittedStreamFrame(nil); !errors.Is(err, ErrSchema) {
		t.Fatalf("empty frame: %v", err)
	}
	if _, err := ParseCommittedStreamFrame([]byte(strings.Repeat("x", 256*1024+1))); !errors.Is(err, ErrSchema) {
		t.Fatalf("oversized frame: %v", err)
	}
}

func TestReplayDiskRunwayBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		backlog, free, reserve int64
		arrival, drain         int64
		seconds                int64
		valid                  bool
	}{
		{"growing backlog", 100, 1000, 100, 20, 10, 90, true},
		{"negative backlog", -1, 1000, 100, 20, 10, 0, false},
		{"no free capacity", 1, 100, 100, 20, 10, 0, false},
		{"nonpositive arrival", 1, 1000, 100, 0, 10, 0, false},
		{"draining backlog", 1, 1000, 100, 10, 20, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seconds, err := DiskRunwaySeconds(tc.backlog, tc.free, tc.reserve, tc.arrival, tc.drain)
			if (err == nil) != tc.valid || seconds != tc.seconds {
				t.Fatalf("disk runway: %d, %v", seconds, err)
			}
		})
	}
	if err := (StreamWriter{}).AppendTx(context.Background(), nil, StreamUpdate{}); err == nil {
		t.Fatal("stream update without a transaction was admitted")
	}
	if _, err := (StreamWriter{}).MarketProjectionHeadTx(context.Background(), nil, "market.price.0x"+strings.Repeat("a", 40)); err == nil {
		t.Fatal("market head read without a transaction was admitted")
	}
}

func TestStreamWriterChecksCommittedUpdateBeforeDatabaseDispatch(t *testing.T) {
	ctx, pool, _, _ := orderedPostgres(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	frame := committedPriceFrame(t)
	payload, err := json.Marshal(frame["payload"])
	if err != nil {
		t.Fatal(err)
	}
	update := StreamUpdate{
		EventID: frame["eventId"].(string), Topic: frame["topic"].(string),
		Revision: frame["revision"].(string), OccurredAt: frame["occurredAt"].(string),
		Watermark: json.RawMessage("null"), Payload: payload,
		Source: StreamSourceProof{Kind: "price_commit"},
	}
	writer := StreamWriter{Schema: os.Getenv("DELIVERY_TEST_SCHEMA") + "_ordered"}
	if err := writer.AppendTx(ctx, tx, StreamUpdate{Source: StreamSourceProof{Kind: "uncommitted"}}); err == nil || !strings.Contains(err.Error(), "unsupported committed source") {
		t.Fatalf("uncommitted stream source: %v", err)
	}
	invalid := update
	invalid.Topic = "invalid.topic"
	if err := writer.AppendTx(ctx, tx, invalid); err == nil || !strings.Contains(err.Error(), "invalid canonical public stream update") {
		t.Fatalf("invalid price update: %v", err)
	}
	if err := writer.AppendTx(ctx, tx, update); err == nil {
		t.Fatal("price update dispatched to an absent committed source function")
	} else {
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "42883" {
			t.Fatalf("price update dispatch: %v", err)
		}
	}
}
