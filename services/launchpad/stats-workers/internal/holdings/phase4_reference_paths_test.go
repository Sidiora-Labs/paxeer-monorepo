package holdings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestPhase4ReferenceRejectsChangedPublicHolderEvidence(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	const schema = "holdings_reference_checks"
	ddl, err := SchemaSQL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repository, err := NewRepository(pool, schema)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	if batch, err := repository.CatchUpCanonical(ctx, mint.Token, "canonical", 10); err != nil || batch.Coverage != "complete" {
		t.Fatalf("canonical transfer: %+v %v", batch, err)
	}
	poolAddress := addr(83)
	cursor, err := repository.Materialize(ctx, mint.Token, poolAddress, "")
	if err != nil {
		t.Fatal(err)
	}
	page, err := repository.ReadPage(ctx, mint.Token, poolAddress, cursor, 100)
	if err != nil {
		t.Fatal(err)
	}
	wire := phase4HolderWire{Revision: strconv.FormatInt(page.Snapshot.Revision, 10), Quality: "complete"}
	wire.Watermark.Height, wire.Watermark.BlockHash, wire.Watermark.ProjectionRevision, wire.Watermark.Integrity = mint.Height, mint.BlockHash, wire.Revision, "healthy"
	wire.Data.Token, wire.Data.Pool, wire.Data.Supply, wire.Data.Coverage = mint.Token, poolAddress, page.Snapshot.Supply, page.Snapshot.Coverage
	wire.Data.SnapshotID, wire.Data.Position, wire.Data.Distribution, wire.Data.Decimals, wire.Data.Provenance = page.SnapshotID, page.Snapshot.Position, page.Distribution, int(page.Snapshot.Decimals), page.Provenance
	for _, balance := range page.Snapshot.Balances {
		wire.Data.Items = append(wire.Data.Items, struct {
			Wallet string `json:"wallet"`
			Raw    string `json:"balanceRaw"`
		}{balance.Address, balance.Raw})
	}
	plan := Phase4Plan{SourceStream: "canonical", HolderSchema: schema}
	market := Phase4Pool{Pool: poolAddress, Token: mint.Token}
	valid := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
	if err := phase4Reference(ctx, pool, plan, market, wire, valid); err != nil {
		t.Fatalf("canonical holder reference: %v", err)
	}
	if err := valid.close(); err != nil {
		t.Fatal(err)
	}
	if artifact, err := os.ReadFile(valid.directory + "/arithmetic.ndjson"); err != nil || !strings.Contains(string(artifact), `"method":"independent_signed_integer_transfer_sums"`) {
		t.Fatalf("independent arithmetic: %s %v", artifact, err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/pools/"+market.Pool+"/holders" || request.URL.Query().Get("limit") != "100" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := json.NewEncoder(w).Encode(wire); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	plan.Pools, plan.Origin = []Phase4Pool{market}, server.URL
	reconciled := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
	if err := phase4Reconcile(ctx, pool, server.Client(), plan, reconciled); err != nil {
		t.Fatalf("canonical holder reconciliation: %v", err)
	}
	if err := reconciled.close(); err != nil {
		t.Fatal(err)
	}
	if artifact, err := os.ReadFile(reconciled.directory + "/reconciliation-api.ndjson"); err != nil || !strings.Contains(string(artifact), market.Pool) {
		t.Fatalf("reconciliation evidence: %s %v", artifact, err)
	}
	for _, tc := range []struct {
		name, want string
		change     func(*phase4HolderWire)
	}{
		{"position integer", "invalid uint256", func(w *phase4HolderWire) {
			p := *w.Data.Position
			p.Height = "01"
			w.Data.Position = &p
			w.Watermark.Height = "01"
		}},
		{"recipient evidence", "integrity incident", func(w *phase4HolderWire) { p := *w.Data.Provenance; p.Recipient = Zero; w.Data.Provenance = &p }},
		{"actor source", "canonical authenticated fill", func(w *phase4HolderWire) {
			p := *w.Data.Provenance
			actor := addr(82)
			p.Actor = &actor
			p.ActorSource = ActorChainEvent
			w.Data.Provenance = &p
		}},
		{"creator absence", "missing creator fabricated", func(w *phase4HolderWire) { bps := "0"; w.Data.Distribution.Creator.BasisPoints = &bps }},
		{"category arithmetic", "category arithmetic", func(w *phase4HolderWire) { w.Data.Distribution.UnknownCount++ }},
		{"distribution band", "distribution band", func(w *phase4HolderWire) { w.Data.Distribution.Bands[2].Count++ }},
		{"population", "holder population", func(w *phase4HolderWire) { w.Data.Distribution.AllPositiveCount++ }},
		{"watermark", "canonical watermark", func(w *phase4HolderWire) { w.Watermark.Height = "9" }},
		{"provenance", "provenance missing", func(w *phase4HolderWire) { w.Data.Provenance = nil }},
		{"decimals", "decimals differ", func(w *phase4HolderWire) { w.Data.Decimals++ }},
		{"supply", "integer conservation differs", func(w *phase4HolderWire) { w.Data.Supply = "101" }},
		{"snapshot", "snapshot differs", func(w *phase4HolderWire) { w.Data.SnapshotID = "00000000-0000-0000-0000-000000000001" }},
		{"top rows", "row count differs", func(w *phase4HolderWire) { w.Data.Items = nil }},
		{"balance", "numeric ordering/balance differs", func(w *phase4HolderWire) { w.Data.Items[0].Raw = "99" }},
		{"concentration", "top10 basis points differs", func(w *phase4HolderWire) { changed := "9999"; w.Data.Distribution.Top10.BasisPoints = &changed }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := wire
			changed.Data.Items = append(changed.Data.Items[:0:0], wire.Data.Items...)
			changed.Data.Distribution = wire.Data.Distribution
			tc.change(&changed)
			a := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
			err := phase4Reference(ctx, pool, plan, market, changed, a)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("changed evidence: %v", err)
			}
			if err := a.close(); err != nil {
				t.Fatal(err)
			}
		})
	}
	creatorMarket := market
	creatorMarket.Creator = addr(82)
	creatorWire := wire
	creatorBPS := "10000"
	creatorWire.Data.Distribution.Creator.BasisPoints = &creatorBPS
	a := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
	if err := phase4Reference(ctx, pool, plan, creatorMarket, creatorWire, a); err != nil {
		t.Fatalf("creator concentration: %v", err)
	}
	if err := a.close(); err != nil {
		t.Fatal(err)
	}
	creatorWire.Data.Distribution.Creator.BasisPoints = nil
	a = &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
	if err := phase4Reference(ctx, pool, plan, creatorMarket, creatorWire, a); err == nil || !strings.Contains(err.Error(), "creator concentration") {
		t.Fatalf("missing creator concentration accepted: %v", err)
	}
	if err := a.close(); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ name, change, restore, want string }{
		{"creation metadata absent", `UPDATE indexer.indexer_canonical_creations SET payload='{}'`, `UPDATE indexer.indexer_canonical_creations SET payload='{"metadata":{"decimals":6,"totalSupply":"100"}}'`, "verified token creation metadata absent"},
		{"creation metadata shape", `UPDATE indexer.indexer_canonical_creations SET payload='"invalid"'`, `UPDATE indexer.indexer_canonical_creations SET payload='{"metadata":{"decimals":6,"totalSupply":"100"}}'`, "cannot unmarshal"},
		{"genesis issuance", `UPDATE indexer.indexer_token_transfers SET amount_raw=101`, `UPDATE indexer.indexer_token_transfers SET amount_raw=100`, "independent genesis issuance differs"},
		{"raw event emitter", `UPDATE indexer.indexer_raw_logs SET emitter='` + addr(90) + `' WHERE log_index=1`, `UPDATE indexer.indexer_raw_logs SET emitter='` + mint.Token + `' WHERE log_index=1`, "public position has no canonical raw event"},
		{"creation relation", `ALTER VIEW indexer.indexer_canonical_creations_v1 RENAME TO saved_source`, `ALTER VIEW indexer.saved_source RENAME TO indexer_canonical_creations_v1`, "does not exist"},
		{"transfer relation", `ALTER VIEW indexer.indexer_token_transfers_v1 RENAME TO saved_source`, `ALTER VIEW indexer.saved_source RENAME TO indexer_token_transfers_v1`, "does not exist"},
		{"snapshot balances", `ALTER TABLE ` + schema + `.snapshot_balances RENAME TO saved_balances`, `ALTER TABLE ` + schema + `.saved_balances RENAME TO snapshot_balances`, "does not exist"},
		{"snapshot metadata", `ALTER TABLE ` + schema + `.snapshots RENAME TO saved_snapshots`, `ALTER TABLE ` + schema + `.saved_snapshots RENAME TO snapshots`, "does not exist"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, tc.change); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := pool.Exec(ctx, tc.restore); err != nil {
					t.Error(err)
				}
			}()
			a := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
			defer a.close()
			if err := phase4Reference(ctx, pool, plan, market, wire, a); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("reference authority: %v, want %s", err, tc.want)
			}
		})
	}
	var metadata []byte
	if err := pool.QueryRow(ctx, "SELECT metadata FROM "+schema+".snapshots WHERE snapshot_id=$1", wire.Data.SnapshotID).Scan(&metadata); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, value, want string }{
		{"invalid saved metadata", `"invalid"`, "cannot unmarshal"},
		{"changed saved watermark", `{}`, "persisted snapshot metadata differs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, "UPDATE "+schema+".snapshots SET metadata=$1 WHERE snapshot_id=$2", tc.value, wire.Data.SnapshotID); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := pool.Exec(ctx, "UPDATE "+schema+".snapshots SET metadata=$1 WHERE snapshot_id=$2", metadata, wire.Data.SnapshotID); err != nil {
					t.Error(err)
				}
			}()
			a := &phase4Artifacts{directory: t.TempDir(), files: map[string]*os.File{}}
			defer a.close()
			if err := phase4Reference(ctx, pool, plan, market, wire, a); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("saved reference: %v, want %s", err, tc.want)
			}
		})
	}

}
