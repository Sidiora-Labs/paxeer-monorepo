package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestEnrichmentStagingRequiresOriginalDurableFill(t *testing.T) {
	ctx := context.Background()
	_, durable, pool := stagingProviders(t)
	original := stagingPublication(t)
	fill := *original.Fill
	stager, err := NewDurableStager(durable, fill.Provenance.DeploymentID, map[string]string{accounting.MarketSubject: accounting.MarketSubject})
	if err != nil {
		t.Fatal(err)
	}
	enrichment := accounting.EnrichmentV1{SourceFillID: fill.SourceFillID, SourceFactDigest: original.SourceFactDigest, Facet: "market", EnrichmentVersion: "actors-v2", ProjectionVersion: "market-v1", EvidenceDigest: accounting.Digest(strings.Repeat("a", 64)), Attribution: &fill.Attribution}
	position := original.Positions[0]
	position.EffectIdentity = accounting.EffectIdentity(fill.SourceFillID, "market", enrichment.ProjectionVersion, enrichment.EvidenceDigest, "market", position.PartitionKey)
	position.PayloadDigest, err = accounting.FacetDigest(accounting.PublicationV1{Enrichment: &enrichment})
	if err != nil {
		t.Fatal(err)
	}
	publication, err := accounting.NewEnrichmentPublication(enrichment, []accounting.PartitionPosition{position})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, sql string
		valid     bool
	}{
		{name: "bound", valid: true},
		{name: "missing", sql: `DELETE FROM indexer.indexer_accounting_fills`},
		{name: "malformed", sql: `UPDATE indexer.indexer_accounting_fills SET payload=jsonb_set(payload,'{version}','"unsupported"')`},
		{name: "digest", sql: `UPDATE indexer.indexer_accounting_fills SET source_fact_digest=repeat('b',64)`},
		{name: "position", sql: `UPDATE indexer.indexer_accounting_fills SET payload=jsonb_set(payload,'{blockTimestampSeconds}','"1700000001"')`},
		{name: "queue", sql: `ALTER TABLE indexer.delivery_outbox RENAME TO delivery_unavailable`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,block_hash,height,transaction_index,emitter,topics,data) VALUES(125,$1,0,$2,101,0,$3,'[]',decode('01','hex'))`, fill.Position.TransactionHash, fill.Position.BlockHash, fill.ActualEmitter); err != nil {
				t.Fatal(err)
			}
			if _, err = (store.CanonicalTx{Tx: tx}).InsertAccountingFill(ctx, fill); err != nil {
				t.Fatal(err)
			}
			if tc.sql != "" {
				if _, err = tx.Exec(ctx, tc.sql); err != nil {
					t.Fatal(err)
				}
			}
			err = stager.StagePublication(ctx, tx, accounting.MarketSubject, publication)
			if !tc.valid {
				if err == nil {
					t.Fatal("unbound enrichment acknowledged")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			subject, envelope, err := durable.OriginalEnvelopeTx(ctx, tx, publication.PublicationID)
			if err != nil {
				t.Fatal(err)
			}
			want, err := accounting.CanonicalPublicationV1(publication)
			if err != nil {
				t.Fatal(err)
			}
			if subject != accounting.MarketSubject || !bytes.Equal(envelope.Payload, want) || envelope.EmittedAt.Unix() != 1700000000 {
				t.Fatalf("original enrichment envelope %+v", envelope)
			}
			var stored accounting.PublicationV1
			if err = json.Unmarshal(envelope.Payload, &stored); err != nil || stored.Fill != nil || stored.Enrichment == nil || stored.SourceFactDigest != original.SourceFactDigest {
				t.Fatalf("source fill binding %+v %v", stored, err)
			}
		})
	}
	if _, err = NewDurableStagerWithWebhook(nil, "deployment", nil, stagingBridge(t, durable), "g1"); err == nil {
		t.Fatal("webhook without durable store admitted")
	}
}
