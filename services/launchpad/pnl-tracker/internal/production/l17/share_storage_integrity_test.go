package l17

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestSelectedShareRejectsChangedStoredContentAndState(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
		want      error
	}{
		{"snapshot shape", `UPDATE portfolio_shares_v3 SET public_snapshot='[]' WHERE id=$1`, nil},
		{"snapshot digest", `UPDATE portfolio_shares_v3 SET public_snapshot=jsonb_set(public_snapshot,'{id}','"changed"') WHERE id=$1`, ErrConflict},
		{"revoked draft", `UPDATE portfolio_shares_v3 SET state='revoked' WHERE id=$1`, ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p, summary := sharingV3Fixture(t)
			ctx := context.Background()
			preview, err := s.Preview(ctx, p, ShareSelectionV3{SnapshotID: summary.SnapshotID, Fields: []string{"knownRealizedSubtotalUsdlRaw"}, ExpiresAt: time.Now().Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.repository.Pool.Exec(ctx, tc.sql, preview.ID); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Publish(ctx, p, SharePublishV3{ID: preview.ID, Intent: preview.Intent}, "publish"); err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
			var count int
			if err = s.repository.Pool.QueryRow(ctx, `SELECT count(*) FROM portfolio_share_outbox_v3`).Scan(&count); err != nil || count != 0 {
				t.Fatal(count, err)
			}
			if tc.name == "snapshot shape" {
				if _, err = s.Owner(ctx, p, preview.ID); err == nil {
					t.Fatal("invalid public snapshot decoded")
				}
			}
		})
	}
}

func TestSelectedSharePreviewWindowAndPublishKeyAreBound(t *testing.T) {
	s, p, summary := sharingV3Fixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	s.clock = func() time.Time { return now }
	selection := ShareSelectionV3{SnapshotID: summary.SnapshotID, Fields: []string{"knownRealizedSubtotalUsdlRaw"}, ExpiresAt: now.Add(time.Hour)}
	first, err := s.Preview(ctx, p, selection)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Preview(ctx, p, selection)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Publish(ctx, p, SharePublishV3{ID: first.ID, Intent: first.Intent}, "same-key"); err != nil || got.State != "pending" {
		t.Fatal(got, err)
	}
	if _, err = s.Publish(ctx, p, SharePublishV3{ID: second.ID, Intent: second.Intent}, "same-key"); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	now = now.Add(11 * time.Minute)
	if _, err = s.Publish(ctx, p, SharePublishV3{ID: second.ID, Intent: second.Intent}, "new-key"); !errors.Is(err, ErrGone) {
		t.Fatal(err)
	}
}

func TestSelectedPublicShareRequiresVisibleCanonicalSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name, state, raw string
		want             error
	}{
		{"draft", "draft", "", ErrNotFound}, {"shape", "public", "[]", nil}, {"digest", "public", `{"id":"changed"}`, ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p, summary := sharingV3Fixture(t)
			ctx := context.Background()
			preview, err := s.Preview(ctx, p, ShareSelectionV3{SnapshotID: summary.SnapshotID, Fields: []string{"knownRealizedSubtotalUsdlRaw"}, ExpiresAt: time.Now().Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.Publish(ctx, p, SharePublishV3{ID: preview.ID, Intent: preview.Intent}, "publish"); err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(preview.Share)
			if err != nil {
				t.Fatal(err)
			}
			if tc.raw != "" {
				raw = []byte(tc.raw)
			}
			if _, err = s.repository.Pool.Exec(ctx, `UPDATE portfolio_shares_v3 SET state=$2,public_snapshot=$3,image=$4 WHERE id=$1`, preview.ID, tc.state, raw, []byte("pending image")); err != nil {
				t.Fatal(err)
			}
			if _, _, err = s.Public(ctx, preview.ID); err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
}
