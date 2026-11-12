package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDeterministicReaderValidatesRequestAndSourceRows(t *testing.T) {
	r, db := deterministicFixture(t)
	ctx := context.Background()
	for _, schema := range []string{"", strings.Repeat("a", 64), "invalid-name"} {
		if _, err := NewDeterministicReader(db, r.keys, schema); err == nil {
			t.Fatalf("invalid schema: %q", schema)
		}
	}
	if _, err := r.Snapshot(ctx, "invalid"); !errors.Is(err, ErrAddressInvalid) {
		t.Fatalf("snapshot address: %v", err)
	}
	snapshot, err := r.Snapshot(ctx, fixtureAddress)
	if err != nil || snapshot.Watermark.ProjectionRevision != "1" {
		t.Fatalf("snapshot: %+v %v", snapshot, err)
	}
	page, err := r.Trades(ctx, fixtureAddress, "", "", 0)
	if err != nil || len(page.Trades) != 0 || page.HasMore {
		t.Fatalf("default trade page: %+v %v", page, err)
	}
	for _, tc := range []struct {
		pool, filter, token string
		size                int
		want                error
	}{
		{"invalid", "", "", 1, ErrAddressInvalid},
		{fixtureAddress, "", "", 101, ErrCursorInvalid},
		{fixtureAddress, "unknown", "", 1, ErrCursorInvalid},
		{fixtureAddress, "", "invalid", 1, ErrCursorInvalid},
	} {
		if _, err := r.Trades(ctx, tc.pool, tc.filter, tc.token, tc.size); !errors.Is(err, tc.want) {
			t.Fatalf("request %+v: %v", tc, err)
		}
	}
	for _, tc := range []struct{ name, change, restore string }{
		{"missing hash", `UPDATE ` + fixtureSchema + `.market_snapshots SET block_hash=NULL`, `UPDATE ` + fixtureSchema + `.market_snapshots SET block_hash='0x` + strings.Repeat("a", 64) + `'`},
		{"negative height", `UPDATE ` + fixtureSchema + `.market_snapshots SET chain_height=-1`, `UPDATE ` + fixtureSchema + `.market_snapshots SET chain_height=3`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := db.Exec(ctx, tc.change); err != nil {
				t.Fatal(err)
			}
			if _, err := r.Snapshot(ctx, fixtureAddress); !errors.Is(err, ErrDataIntegrity) {
				t.Fatalf("invalid source accepted: %v", err)
			}
			if _, err := db.Exec(ctx, tc.restore); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := db.Exec(ctx, `INSERT INTO `+fixtureSchema+`.market_trades VALUES('125',$1,1,-1,0,'invalid-order',1,'{}')`, fixtureAddress); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Trades(ctx, fixtureAddress, "", "", 1); !errors.Is(err, ErrDataIntegrity) {
		t.Fatalf("invalid order accepted: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE `+fixtureSchema+`.market_trades SET transaction_index=0,event_id=$1`, strings.Repeat("x", 161)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Trades(ctx, fixtureAddress, "", "", 1); !errors.Is(err, ErrDataIntegrity) {
		t.Fatalf("oversized identity accepted: %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := r.Snapshot(cancelled, fixtureAddress); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled snapshot: %v", err)
	}
	if _, err := db.Exec(ctx, `DROP TABLE `+fixtureSchema+`.market_trades`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Trades(ctx, fixtureAddress, "", "", 1); err == nil {
		t.Fatal("missing trades source accepted")
	}
	if _, err := db.Exec(ctx, `DROP TABLE `+fixtureSchema+`.market_snapshots`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Snapshot(ctx, fixtureAddress); err == nil {
		t.Fatal("missing snapshot source accepted")
	}
}
