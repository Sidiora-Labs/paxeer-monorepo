package holdings

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestHolderSnapshotMetadataIntegrity(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	const schema = "snapshot_integrity"
	ddl, err := SchemaSQL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool, schema)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	if batch, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10); err != nil || batch.Applied != 1 {
		t.Fatalf("catchup: %+v %v", batch, err)
	}
	cursor, err := repo.Materialize(ctx, mint.Token, addr(83), mint.To)
	if err != nil {
		t.Fatal(err)
	}
	page, err := repo.ReadPage(ctx, mint.Token, addr(83), cursor, 10)
	if err != nil || page.Snapshot.Supply != "100" || page.Stale || page.Next != nil {
		t.Fatalf("snapshot: %+v %v", page, err)
	}
	var original []byte
	if err := pool.QueryRow(ctx, "SELECT metadata FROM snapshot_integrity.snapshots WHERE snapshot_id=$1", page.SnapshotID).Scan(&original); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*snapshotMetadata)
		want   error
	}{
		{"missing provenance", func(m *snapshotMetadata) { m.Provenance = nil }, ErrIntegrity},
		{"changed provenance", func(m *snapshotMetadata) { m.Provenance.Recipient = addr(99) }, ErrIntegrity},
		{"changed token", func(m *snapshotMetadata) { m.Snapshot.Token = addr(99) }, ErrIntegrity},
		{"negative revision", func(m *snapshotMetadata) { m.Snapshot.Revision = -1 }, ErrIntegrity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var saved snapshotMetadata
			if err := json.Unmarshal(original, &saved); err != nil {
				t.Fatal(err)
			}
			tc.change(&saved)
			raw, err := json.Marshal(saved)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, "UPDATE snapshot_integrity.snapshots SET metadata=$1 WHERE snapshot_id=$2", raw, page.SnapshotID); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.ReadPage(ctx, mint.Token, addr(83), cursor, 10); !errors.Is(err, tc.want) {
				t.Fatalf("snapshot integrity: %v", err)
			}
		})
	}
	if _, err := pool.Exec(ctx, "UPDATE snapshot_integrity.snapshots SET metadata=$1 WHERE snapshot_id=$2", original, page.SnapshotID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE snapshot_integrity.tokens SET revision=revision+1 WHERE token=$1", mint.Token); err != nil {
		t.Fatal(err)
	}
	stale, err := repo.ReadPage(ctx, mint.Token, addr(83), cursor, 10)
	if err != nil || !stale.Stale || stale.StaleReason != "projection_changed_since_snapshot" || stale.Snapshot.Revision != page.Snapshot.Revision {
		t.Fatalf("frozen revision: %+v %v", stale, err)
	}
	changed := encodeSnapshotCursor(page.SnapshotID, "99", mint.To)
	if _, err := repo.ReadPage(ctx, mint.Token, addr(83), changed, 10); !errors.Is(err, ErrCursor) {
		t.Fatalf("cursor boundary: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE snapshot_integrity.snapshots SET expires_at=clock_timestamp()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.DeleteExpiredSnapshots(ctx, 1); err != nil || n != 1 {
		t.Fatalf("expired snapshot: %d %v", n, err)
	}
	if n, err := repo.DeleteExpiredSnapshots(ctx, 1); err != nil || n != 0 {
		t.Fatalf("expired snapshot repeated: %d %v", n, err)
	}
	if _, err := repo.ReadPage(ctx, mint.Token, addr(83), cursor, 10); !errors.Is(err, ErrRestartRequired) {
		t.Fatalf("deleted snapshot cursor: %v", err)
	}
}

func TestHolderRepositoryInputBounds(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	repo, err := NewRepository(pool, "bounds")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewRepository(nil, "bounds"); err == nil {
		t.Fatal("nil pool accepted")
	}
	if _, err := NewRepository(pool, "Invalid"); err == nil {
		t.Fatal("invalid schema accepted")
	}
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"discovery token", func() error { return repo.Discover(ctx, "invalid", 6, "10", true) }},
		{"discovery height", func() error { return repo.Discover(ctx, mint.Token, 6, "01", true) }},
		{"snapshot limit", func() error { _, e := repo.Snapshot(ctx, mint.Token, 0); return e }},
		{"snapshot token", func() error { _, e := repo.Snapshot(ctx, "invalid", 10); return e }},
		{"materialize token", func() error { _, e := repo.Materialize(ctx, "invalid", addr(83), ""); return e }},
		{"materialize pool", func() error { _, e := repo.Materialize(ctx, mint.Token, "invalid", ""); return e }},
		{"page limit", func() error { _, e := repo.ReadPage(ctx, mint.Token, addr(83), "", 101); return e }},
		{"page token", func() error { _, e := repo.ReadPage(ctx, "invalid", addr(83), "", 10); return e }},
		{"page pool", func() error { _, e := repo.ReadPage(ctx, mint.Token, "invalid", "", 10); return e }},
		{"page bytes", func() error {
			_, e := repo.ReadPage(ctx, mint.Token, addr(83), base64.RawURLEncoding.EncodeToString([]byte("short")), 10)
			return e
		}},
		{"snapshot cleanup limit", func() error { _, e := repo.DeleteExpiredSnapshots(ctx, 0); return e }},
		{"nil transaction", func() error { _, e := repo.ApplyTx(ctx, nil, mint); return e }},
		{"nil internal transaction", func() error { _, e := repo.applyTx(ctx, nil, mint); return e }},
		{"nil classification transaction", func() error {
			return repo.ClassifyTx(ctx, nil, mint.Token, mint.To, "external_wallet", strings.Repeat("a", 64), 1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil {
				t.Fatal("invalid repository input accepted")
			}
		})
	}
	for _, tc := range []struct{ id, balance, address string }{
		{"invalid", "1", mint.To}, {"00000000-0000-0000-0000-000000000001", "01", mint.To}, {"00000000-0000-0000-0000-000000000001", "1", "invalid"},
	} {
		if got := encodeSnapshotCursor(tc.id, tc.balance, tc.address); got != "" {
			t.Fatalf("invalid cursor encoded: %s", got)
		}
	}
	pool.Close()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"apply", func() error { _, e := repo.Apply(ctx, mint); return e }},
		{"snapshot", func() error { _, e := repo.Snapshot(ctx, mint.Token, 10); return e }},
		{"materialize", func() error { _, e := repo.Materialize(ctx, mint.Token, addr(83), ""); return e }},
		{"page", func() error {
			_, e := repo.ReadPage(ctx, mint.Token, addr(83), encodeSnapshotCursor("00000000-0000-0000-0000-000000000001", "0", Zero), 10)
			return e
		}},
		{"checkpoint", func() error { _, e := repo.Checkpoint(ctx, mint.Token, "10", mint.BlockHash, "holdings_v1"); return e }},
		{"canonical apply", func() error { _, e := repo.ApplyCanonicalTransfer(ctx, mint, "canonical"); return e }},
		{"canonical discovery", func() error { return repo.DiscoverCanonical(ctx, mint.Token, "canonical") }},
		{"canonical catchup", func() error { _, e := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10); return e }},
		{"rebuild", func() error {
			_, e := repo.RebuildFromCanonical(ctx, pool, "canonical", mint.Token, 6, "10", "10", "holdings_v1")
			return e
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil || !strings.Contains(err.Error(), "closed") {
				t.Fatalf("closed repository: %v", err)
			}
		})
	}
}

func TestFrozenHolderCursorValidation(t *testing.T) {
	now := time.Unix(100, 0)
	frozen, err := Freeze(Snapshot{Token: addr(1), Revision: 2, Balances: []Balance{{Address: addr(2), Raw: "10"}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	encode := func(c Cursor) string {
		raw, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	for _, tc := range []struct {
		name, cursor string
		limit        int
		at           time.Time
	}{
		{"oversized", strings.Repeat("a", 513), 1, now}, {"encoding", "%", 1, now}, {"json", base64.RawURLEncoding.EncodeToString([]byte("{")), 1, now},
		{"snapshot", encode(Cursor{SnapshotID: "other", Balance: "10", Address: addr(2)}), 1, now},
		{"balance", encode(Cursor{SnapshotID: frozen.id, Balance: "01", Address: addr(2)}), 1, now},
		{"address", encode(Cursor{SnapshotID: frozen.id, Balance: "10", Address: "invalid"}), 1, now},
		{"boundary", encode(Cursor{SnapshotID: frozen.id, Balance: "9", Address: addr(2)}), 1, now},
		{"limit", "", 0, now}, {"expiry", "", 1, now.Add(15 * time.Minute)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := frozen.Page(tc.cursor, tc.limit, tc.at); err == nil {
				t.Fatal("invalid frozen page accepted")
			}
		})
	}
	if _, err := Freeze(Snapshot{Balances: make([]Balance, 10001)}, now); err == nil {
		t.Fatal("oversized frozen snapshot accepted")
	}
	if _, err := Freeze(Snapshot{Balances: []Balance{{Address: "invalid", Raw: "1"}}}, now); err == nil {
		t.Fatal("invalid balance accepted")
	}
	if _, err := Freeze(Snapshot{Balances: []Balance{{Address: addr(2), Raw: "1"}, {Address: addr(2), Raw: "2"}}}, now); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("duplicate frozen rows: %v", err)
	}
	page, err := frozen.Page(encode(Cursor{SnapshotID: frozen.id, Balance: "10", Address: addr(2)}), 1, now)
	if err != nil || len(page.Rows) != 0 || page.Next != "" || page.Revision != 2 {
		t.Fatalf("terminal page: %+v %v", page, err)
	}
}

func TestHolderClassificationAuthorityBounds(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	ddl, err := SchemaSQL("classification_bounds")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool, "classification_bounds")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Discover(ctx, mint.Token, 6, "10", true); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, token, address, category, evidence string
		version                                  int64
	}{
		{"token", "invalid", mint.To, "external_wallet", strings.Repeat("a", 64), 1},
		{"address", mint.Token, "invalid", "external_wallet", strings.Repeat("a", 64), 1},
		{"zero address", mint.Token, Zero, "external_wallet", strings.Repeat("a", 64), 1},
		{"version", mint.Token, mint.To, "external_wallet", strings.Repeat("a", 64), 0},
		{"evidence length", mint.Token, mint.To, "external_wallet", "a", 1},
		{"evidence encoding", mint.Token, mint.To, "external_wallet", strings.Repeat("z", 64), 1},
		{"category", mint.Token, mint.To, "other", strings.Repeat("a", 64), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if err := repo.ClassifyTx(ctx, tx, tc.token, tc.address, tc.category, tc.evidence, tc.version); err == nil {
				t.Fatal("invalid classification authority accepted")
			}
		})
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	evidence := strings.Repeat("a", 64)
	for i := 0; i < 2; i++ {
		if err := repo.ClassifyTx(ctx, tx, mint.Token, mint.To, "external_wallet", evidence, 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.ClassifyTx(ctx, tx, mint.Token, mint.To, "treasury", evidence, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("classification conflict: %v", err)
	}
	if err := repo.ClassifyTx(ctx, tx, mint.Token, mint.To, "treasury", evidence, 2); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var category string
	var version, revision int64
	if err := pool.QueryRow(ctx, "SELECT category,version FROM classification_bounds.classifications WHERE token=$1 AND address=$2", mint.Token, mint.To).Scan(&category, &version); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT revision FROM classification_bounds.tokens WHERE token=$1", mint.Token).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if category != "treasury" || version != 2 || revision != 2 {
		t.Fatalf("classification: %s version=%d revision=%d", category, version, revision)
	}
	if err := repo.Discover(ctx, mint.Token, 7, "10", true); !errors.Is(err, ErrConflict) {
		t.Fatalf("metadata conflict: %v", err)
	}
	if _, err := repo.Snapshot(ctx, addr(99), 10); !errors.Is(err, ErrUnknownToken) {
		t.Fatalf("unknown token: %v", err)
	}
	if _, err := repo.Materialize(ctx, addr(99), addr(83), ""); !errors.Is(err, ErrUnknownToken) {
		t.Fatalf("unknown materialization: %v", err)
	}
	if _, err := repo.Apply(ctx, event(addr(99), Zero, mint.To, "1", 1)); !errors.Is(err, ErrUnknownToken) {
		t.Fatalf("unknown mutation: %v", err)
	}
	if _, err := repo.Apply(ctx, Transfer{}); err == nil {
		t.Fatal("invalid transfer accepted")
	}
	tx, err = pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ApplyTx(ctx, tx, mint); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("closed transaction: %v", err)
	}
}
