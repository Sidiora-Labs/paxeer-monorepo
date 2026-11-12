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

func TestCommittedStreamFamiliesBindTopicsAndRevisions(t *testing.T) {
	wire, err := os.ReadFile("../../../../services/launchpad/api/internal/broker/frozen_stream_fixtures_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[string]json.RawMessage
	if err := json.Unmarshal(wire, &fixtures); err != nil {
		t.Fatal(err)
	}
	ctx, pool, _, _ := orderedPostgres(t)
	for family, kind := range map[string]string{"CandleEvent": "candle_commit", "MarketPriceEvent": "price_commit", "MarketTradesEvent": "market_commit", "PortfolioEvent": "portfolio_commit", "RankingEvent": "ranking_commit", "StatsEvent": "stats_commit"} {
		t.Run(family, func(t *testing.T) {
			var frame map[string]json.RawMessage
			if err := json.Unmarshal(fixtures[family], &frame); err != nil {
				t.Fatal(err)
			}
			parsed, err := ParseCommittedStreamFrame(fixtures[family])
			if err != nil {
				t.Fatalf("canonical %s frame: %v", family, err)
			}
			if parsed.EventID != "value" || parsed.Sequence != "9007199254740993" {
				t.Fatalf("canonical identity: %+v", parsed)
			}
			for _, change := range []struct {
				key   string
				value any
			}{{"topic", strings.Replace(parsed.Topic, "1111111111111111111111111111111111111111", "2222222222222222222222222222222222222222", 1)}, {"sequence", "9223372036854775808"}, {"sourceSequenceLast", "1"}} {
				if change.key == "topic" && (change.value == parsed.Topic || family == "MarketPriceEvent") {
					continue
				}
				previous := frame[change.key]
				frame[change.key], err = json.Marshal(change.value)
				if err != nil {
					t.Fatal(err)
				}
				changed, err := json.Marshal(frame)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := ParseCommittedStreamFrame(changed); err == nil {
					t.Fatalf("unbound %s frame admitted: %s", family, change.key)
				}
				frame[change.key] = previous
			}
			var u StreamUpdate
			if err := json.Unmarshal(fixtures[family], &u); err != nil {
				t.Fatal(err)
			}
			u.Source = StreamSourceProof{Kind: kind}
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = (StreamWriter{Schema: os.Getenv("DELIVERY_TEST_SCHEMA") + "_ordered"}).AppendTx(ctx, tx, u)
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != "42883" {
				t.Fatalf("canonical family dispatch: %v", err)
			}
			if err := tx.Rollback(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
