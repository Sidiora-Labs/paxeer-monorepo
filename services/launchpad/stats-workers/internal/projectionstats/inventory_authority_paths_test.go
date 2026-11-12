package projectionstats

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func inventoryPublication(t *testing.T, market accounting.PublicationV1) accounting.PublicationV1 {
	t.Helper()
	fill := *market.Fill
	wallet := fill.Attribution.TransactionSender
	attribution := accounting.Attribution{TransactionSender: wallet, Payer: &wallet, Recipient: &wallet, Trader: &wallet, Quality: "exact_receipt", Evidence: []accounting.EvidenceRef{{Kind: "receipt", ArtifactID: "canonical-transfer", ArtifactSHA256: fill.Provenance.ReceiptSHA256}}}
	enrichment := accounting.EnrichmentV1{SourceFillID: fill.SourceFillID, SourceFactDigest: market.SourceFactDigest, Facet: "inventory", EnrichmentVersion: "ownership-v1", ProjectionVersion: "inventory-v1", EvidenceDigest: accounting.Digest(fill.Provenance.ReceiptSHA256), Attribution: &attribution}
	raw, err := json.Marshal(enrichment)
	if err != nil {
		t.Fatal(err)
	}
	key := "125:" + wallet + ":" + fill.Token.Address
	position := accounting.PartitionPosition{ProjectionVersion: enrichment.ProjectionVersion, PartitionKind: "inventory", PartitionKey: key, Sequence: "1", SourceEventID: fill.SourceFillID, EffectIdentity: accounting.EffectIdentity(fill.SourceFillID, "inventory", enrichment.ProjectionVersion, enrichment.EvidenceDigest, "inventory", key), PayloadDigest: accounting.Digest(delivery.Hash(raw)), SourceOrder: fill.Position}
	publication, err := accounting.NewEnrichmentPublication(enrichment, []accounting.PartitionPosition{position})
	if err != nil {
		t.Fatal(err)
	}
	return publication
}

func TestInventoryPublicationRetainsSourceAuthority(t *testing.T) {
	ctx, repo := repository(t)
	market, _ := repairM1043Receipt(t)
	inventory := inventoryPublication(t, market)
	schema := strings.Trim(repo.schema, `"`)
	authority, err := NewAccountingSourceAuthority(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, "CREATE TABLE "+repo.schema+".indexer_accounting_fills_v1(source_fill_id text PRIMARY KEY,source_fact_digest text,payload jsonb); CREATE TABLE "+repo.schema+".indexer_accounting_publications_v1(publication_id text PRIMARY KEY,payload jsonb)"); err != nil {
		t.Fatal(err)
	}
	fillRaw, err := json.Marshal(market.Fill)
	if err != nil {
		t.Fatal(err)
	}
	inventoryRaw, err := json.Marshal(inventory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, "INSERT INTO "+repo.schema+".indexer_accounting_fills_v1 VALUES($1,$2,$3)", market.SourceFillID, string(market.SourceFactDigest), fillRaw); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.pool.Exec(ctx, "INSERT INTO "+repo.schema+".indexer_accounting_publications_v1 VALUES($1,$2)", inventory.PublicationID, inventoryRaw); err != nil {
		t.Fatal(err)
	}
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	original, err := authority.OriginalTx(ctx, tx, inventory)
	if err != nil || original.SourceFillID != market.SourceFillID || original.Position != market.Fill.Position {
		t.Fatalf("inventory source: %+v %v", original, err)
	}
	for i := 0; i < 2; i++ {
		if err := repo.RecordInventoryPublicationTx(ctx, tx, inventory); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if count(t, ctx, repo, "accounting_inventory_facets") != 1 {
		t.Fatal("inventory replay duplicated facet")
	}
	for _, tc := range []struct {
		name, query string
		args        []any
	}{
		{"missing publication", "DELETE FROM " + repo.schema + ".indexer_accounting_publications_v1", nil},
		{"malformed publication", "UPDATE " + repo.schema + ".indexer_accounting_publications_v1 SET payload='{}'", nil},
		{"changed publication", "UPDATE " + repo.schema + ".indexer_accounting_publications_v1 SET payload=jsonb_set(payload,'{positions,0,sequence}','\"2\"')", nil},
		{"missing source", "DELETE FROM " + repo.schema + ".indexer_accounting_fills_v1", nil},
		{"malformed fill", "UPDATE " + repo.schema + ".indexer_accounting_fills_v1 SET payload='{}'", nil},
		{"changed digest", "UPDATE " + repo.schema + ".indexer_accounting_fills_v1 SET source_fact_digest=$1", []any{strings.Repeat("a", 64)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := repo.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if _, err := tx.Exec(ctx, tc.query, tc.args...); err != nil {
				t.Fatal(err)
			}
			if _, err := authority.OriginalTx(ctx, tx, inventory); !errors.Is(err, ErrIntegrity) {
				t.Fatalf("source authority: %v", err)
			}
		})
	}
	tx, err = repo.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "UPDATE "+repo.schema+".accounting_inventory_facets SET source_fact_digest=$1", strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordInventoryPublicationTx(ctx, tx, inventory); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("inventory conflict: %v", err)
	}
	if err := repo.RecordInventoryPublicationTx(ctx, nil, inventory); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil inventory transaction: %v", err)
	}
	if err := repo.RecordInventoryPublicationTx(ctx, tx, market); !errors.Is(err, ErrInvalid) {
		t.Fatalf("market facet: %v", err)
	}
	if err := repo.RecordMarketPublicationTx(ctx, tx, market, ApplyResult{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unaccepted market result: %v", err)
	}
	if _, err := authority.ResolveTx(ctx, tx, inventory, "consumer-v1"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("inventory market projection: %v", err)
	}
}

func TestQualifiedPolicyUsesAttributionAndExecutionEvidence(t *testing.T) {
	market, _ := repairM1043Receipt(t)
	inventory := inventoryPublication(t, market)
	fill := *market.Fill
	fill.Attribution = *inventory.Enrichment.Attribution
	bps := uint16(30)
	digest := accounting.Digest(strings.Repeat("b", 64))
	fill.Execution = accounting.ExecutionPolicy{Quality: "exact", FeeBPS: &bps, ConfigDigest: &digest, PreTransactionStateDigest: &digest, ReplayEvidenceDigest: &digest, Evidence: []accounting.EvidenceRef{{Kind: "replay", ArtifactID: "execution", ArtifactSHA256: string(digest)}}}
	in, err := qualifiedInput(fill, "consumer-v1", "projection-v1")
	if err != nil {
		t.Fatal(err)
	}
	if in.Trade.Recipient == nil || *in.Trade.Recipient != *fill.Attribution.Recipient || len(in.Trade.RecipientEvidenceRefs) != 1 || in.Policy.FeeBPS == nil || *in.Policy.FeeBPS != 30 || in.Policy.FeeEvidenceDigest == nil || *in.Policy.FeeEvidenceDigest != string(digest) {
		t.Fatalf("qualified evidence: %+v", in)
	}
	if _, err := AccountQualified(in.Trade, in.Assets, in.Policy); err != nil {
		t.Fatal(err)
	}
	bad := in.Policy
	bad.assets.Token = bad.assets.USDL
	if _, err := AccountQualified(in.Trade, in.Assets, bad); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed bound assets: %v", err)
	}
	fill.BlockTimestampSeconds = strings.Repeat("9", 30)
	if _, err := qualifiedInput(fill, "consumer-v1", "projection-v1"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("timestamp overflow: %v", err)
	}
	var authority *CoreReceiptAuthority
	if got := authority.Trades("unknown"); got != nil {
		t.Fatalf("nil authority trades: %+v", got)
	}
	if _, err := authority.Account(in.Trade); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil authority account: %v", err)
	}
}
