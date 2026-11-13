package l17

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestPortfolioUpdateCommitsExactProjection(t *testing.T) {
	for _, count := range []int{1, 2} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			repository := phase2Repository(t)
			ctx := context.Background()
			positions := phase2Positions(count, "1")
			for i := range positions {
				positions[i].MarkPrice = nil
				positions[i].PricingQuality = "unpriced"
				var err error
				positions[i], _, err = Derive(positions[i])
				if err != nil {
					t.Fatal(err)
				}
			}
			id, err := opaqueID()
			if err != nil {
				t.Fatal(err)
			}
			summary, err := Aggregate(wallet, id, positions[0].SourceWatermark, positions)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(summary)
			if err != nil {
				t.Fatal(err)
			}
			var wireSummary interfaces.PortfolioSummary
			if err = json.Unmarshal(raw, &wireSummary); err != nil {
				t.Fatal(err)
			}
			changes := make([]interfaces.PositionChange, 0, count)
			for _, position := range positions {
				raw, err = json.Marshal(position)
				if err != nil {
					t.Fatal(err)
				}
				var wirePosition interfaces.PortfolioPosition
				if err = json.Unmarshal(raw, &wirePosition); err != nil {
					t.Fatal(err)
				}
				changes = append(changes, interfaces.PositionChange{Operation: "upsert", Position: &wirePosition, PositionId: position.PositionID, Revision: "1"})
			}
			update := interfaces.PortfolioUpdate{Changes: changes, CompleteBatch: true, SnapshotId: id, Summary: wireSummary, SummaryRevision: "1"}
			raw, err = json.Marshal(update)
			if err != nil {
				t.Fatal(err)
			}
			if err = interfaces.ValidateJSON("PortfolioUpdate", raw); err != nil {
				t.Fatal(err)
			}
			schema := repository.Pool.Config().ConnConfig.RuntimeParams["search_path"]
			ddl, err := delivery.SchemaDDL(schema)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = repository.Pool.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
			subject := "portfolio.snapshot.v2"
			store, err := delivery.NewStore(repository.Pool, schema, delivery.Policy{Subjects: map[string]bool{subject: true}, EventTypes: map[string]bool{subject: true}})
			if err != nil {
				t.Fatal(err)
			}
			event := delivery.Envelope{EventID: "projection-event", EventType: subject, SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: "99", TransactionIndex: "0", LogIndex: "1"}, EmittedAt: time.Now().UTC(), TraceID: "projection-event", PayloadHash: delivery.Hash(raw), Payload: raw}
			assertProjectionBindings(t, repository, store, subject, event, update, summary, positions)
			tx, err := repository.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			committed, err := repository.CommitProjectionDeliveryTx(ctx, tx, store, subject, event, update, time.Now().Add(time.Hour))
			if err != nil || !reflect.DeepEqual(committed, summary) {
				t.Fatalf("committed summary = %+v, expected = %+v, error = %v", committed, summary, err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			var notices int
			if err = repository.Pool.QueryRow(ctx, "SELECT count(*) FROM delivery_outbox WHERE event_id=$1", event.EventID).Scan(&notices); err != nil || notices != 1 {
				t.Fatalf("delivery notice count = %d, error = %v", notices, err)
			}
			page, err := repository.Page(ctx, wallet, id, -1, count, time.Now())
			if err != nil || len(page.Positions) != count || page.Summary.SnapshotID != id {
				t.Fatalf("committed page = %+v, error = %v", page, err)
			}
		})
	}
}
