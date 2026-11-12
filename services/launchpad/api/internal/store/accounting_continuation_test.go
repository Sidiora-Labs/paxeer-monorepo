package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestCanonicalTradeContinuationPinsCommittedSnapshot(t *testing.T) {
	model, db := productionAccountingFixture(t)
	previous := ""
	for sequence := 1; sequence <= 3; sequence++ {
		previous = insertProductionFill(t, db, sequence, previous)
	}
	ctx := context.Background()
	request := store.AccountingPageRequest{Pool: productionAddress, Limit: 2}
	first, err := model.Trades(ctx, request)
	if err != nil || len(first.Items) != 2 || first.NextSequence == nil || *first.NextSequence != 2 || first.NextOrder == nil || first.NextEventID == "" || first.Revision != "3" {
		t.Fatalf("first committed page: %+v, %v", first, err)
	}
	var watermark struct {
		ObservedAt string `json:"observedAt"`
	}
	if err := json.Unmarshal(first.Watermark, &watermark); err != nil || watermark.ObservedAt == "" {
		t.Fatalf("first watermark: %s, %v", first.Watermark, err)
	}
	pinned := int64(3)
	request.SnapshotSequence = &pinned
	request.ExpectedSnapshot = first.SnapshotID
	request.SnapshotObservedAt = watermark.ObservedAt
	request.AfterSequence = first.NextSequence
	request.AfterOrder = first.NextOrder
	request.AfterEventID = first.NextEventID
	last, err := model.Trades(ctx, request)
	if err != nil || len(last.Items) != 1 || last.NextSequence != nil || last.SnapshotID != first.SnapshotID || string(last.Watermark) != string(first.Watermark) {
		t.Fatalf("continued committed page: %+v, %v", last, err)
	}
	request.AfterEventID = "missing-event"
	if _, err := model.Trades(ctx, request); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("unknown continuation event accepted: %v", err)
	}
	request.AfterEventID = first.NextEventID
	request.SnapshotObservedAt = "invalid"
	if _, err := model.Trades(ctx, request); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid pinned observation accepted: %v", err)
	}
	request.SnapshotObservedAt = watermark.ObservedAt
	request.ExpectedSnapshot = "different-snapshot"
	if _, err := model.Trades(ctx, request); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("changed snapshot accepted: %v", err)
	}
}
