package l17

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLatestSnapshotOrderPassesAThousand(t *testing.T) {
	ctx := context.Background()
	repository := phase2Repository(t)
	var latest Summary
	for _, revision := range []string{"998", "999", "1000", "1102"} {
		latest = phase2Commit(t, repository, "latest-order-"+revision, revision, 1)
	}
	now := time.Now().UTC()
	id, revision, expires, err := repository.Latest(ctx, wallet, now)
	if err != nil {
		t.Fatal(err)
	}
	if id != latest.SnapshotID || revision != "1102" || !now.Before(expires) {
		t.Fatalf("latest snapshot: id=%s revision=%s expires=%s; want id=%s revision=1102 and an unexpired snapshot", id, revision, expires, latest.SnapshotID)
	}
	service := Service{Repository: repository, CursorKey: []byte(strings.Repeat("k", 32))}
	response, err := service.Portfolio(ctx, wallet, "", 10, now)
	if err != nil {
		t.Fatal(err)
	}
	if response.Page.Summary.SnapshotID != latest.SnapshotID || response.Page.Summary.Watermark.ProjectionRevision != "1102" {
		t.Fatalf("portfolio snapshot: %+v; want id=%s revision=1102", response.Page.Summary, latest.SnapshotID)
	}
	if len(response.Page.Positions) != 1 || response.Page.Positions[0].SourceWatermark.ProjectionRevision != "1102" {
		t.Fatalf("portfolio positions: %+v; want one position at revision 1102", response.Page.Positions)
	}
}

func TestLatestSnapshotOrderNineBeforeTen(t *testing.T) {
	ctx := context.Background()
	repository := phase2Repository(t)
	phase2Commit(t, repository, "latest-order-9", "9", 1)
	latest := phase2Commit(t, repository, "latest-order-10", "10", 1)
	now := time.Now().UTC()
	id, revision, expires, err := repository.Latest(ctx, wallet, now)
	if err != nil {
		t.Fatal(err)
	}
	if id != latest.SnapshotID || revision != "10" || !now.Before(expires) {
		t.Fatalf("latest snapshot: id=%s revision=%s expires=%s; want id=%s revision=10 and an unexpired snapshot", id, revision, expires, latest.SnapshotID)
	}
}
