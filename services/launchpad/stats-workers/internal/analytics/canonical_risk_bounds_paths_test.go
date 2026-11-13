package analytics_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/holdings"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/projectionstats"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCanonicalRiskInputsValidatePersistedSourceFacts(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	archive := repair1038Fixture(t)
	fill := archive.Source[0]
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE SCHEMA indexer; CREATE SCHEMA risk_authority")
	root := "../../../../../"
	for _, id := range []string{"60000", "190001"} {
		if id == "190001" {
			ddl, err := delivery.PartitionSchemaDDL("indexer")
			if err != nil {
				t.Fatal(err)
			}
			exec(ddl)
		}
		raw, err := os.ReadFile(filepath.Join(root, "launchpad/spec/production-system/integration/migrations", id+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		exec(string(raw))
	}
	authority := analytics.ReactionAuthority{Pool: db, Schema: "risk_authority"}
	for _, schema := range []func() (string, error){authority.SchemaSQL, authority.SnapshotSQL, authority.ProjectionSQL} {
		ddl, err := schema()
		if err != nil {
			t.Fatal(err)
		}
		exec(ddl)
	}
	repo, err := projectionstats.NewRepository(db, authority.Schema)
	if err != nil {
		t.Fatal(err)
	}
	exec(repo.MigrationSQL())
	holderDDL, err := holdings.SchemaSQL("risk_holders")
	if err != nil {
		t.Fatal(err)
	}
	exec(holderDDL)
	raw, err := os.ReadFile(filepath.Join(root, "sdk/launchpad/protocol/contracts/v1/chain/golden-fixtures.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tape struct {
		Receipts []struct {
			Logs []struct {
				Address, TransactionHash, BlockHash, Data string
				BlockNumber, TransactionIndex, Index      uint64
				Topics                                    []string
			}
		}
	}
	if err := json.Unmarshal(raw, &tape); err != nil {
		t.Fatal(err)
	}
	height, err := strconv.ParseInt(fill.Position.Height, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	for _, receipt := range tape.Receipts {
		for _, log := range receipt.Logs {
			if log.BlockNumber > uint64(height) {
				continue
			}
			topics, err := json.Marshal(log.Topics)
			if err != nil {
				t.Fatal(err)
			}
			data, err := hex.DecodeString(strings.TrimPrefix(log.Data, "0x"))
			if err != nil {
				t.Fatal(err)
			}
			exec(`INSERT INTO indexer.indexer_raw_logs VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8)`, strings.ToLower(log.TransactionHash), log.Index, log.BlockNumber, log.TransactionIndex, strings.ToLower(log.BlockHash), strings.ToLower(log.Address), topics, data)
		}
	}
	creation := archive.Creation
	exec(`INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$4,$4,1,$5,$6,$7,0,0,$8,'{}',$9)`, creation.Token, creation.Pool, creation.Creator, fill.ActualEmitter, creation.Height, creation.Hash, creation.Transaction, creation.CreatedAt, fmt.Sprintf(`{"metadata":{"decimals":6,"totalSupply":"%s"}}`, creation.Supply))
	exec(`INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'canonical',$1,$1)`, height)
	exec(`INSERT INTO indexer.indexer_ingest_commits(chain_id,stream_id,batch_id,previous_height,last_complete_height,block_hash,record_count,outbox_count) VALUES(125,'canonical','source',56,$1,$2,1,0)`, height, fill.Position.BlockHash)
	exec(`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) SELECT 125,'canonical',h,CASE WHEN h=$1 THEN $2 ELSE $3 END,0,true,true FROM generate_series(57,$1::bigint) h`, height, fill.Position.BlockHash, creation.Hash)
	exec(`UPDATE indexer.indexer_block_completeness b SET block_hash=r.hash,raw_count=r.total FROM (SELECT height,min(block_hash) AS hash,count(*)::integer AS total FROM indexer.indexer_raw_logs GROUP BY height) r WHERE r.height=b.height`)
	for _, transfer := range archive.Transfers {
		h, err := strconv.ParseInt(transfer.Height, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		if h > height {
			continue
		}
		payload, err := json.Marshal(transfer)
		if err != nil {
			t.Fatal(err)
		}
		exec(`INSERT INTO indexer.indexer_token_transfers(chain_id,transaction_hash,log_index,token,from_address,to_address,amount_raw,height,transaction_index,block_hash,provenance,payload) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8,$9,'{}',$10)`, transfer.TransactionHash, transfer.LogIndex, transfer.Token, transfer.From, transfer.To, transfer.Amount, transfer.Height, transfer.TransactionIndex, transfer.BlockHash, payload)
	}
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO indexer.indexer_accounting_fills VALUES($1,$2,125,$3,$4,$5,$6)`, fill.SourceFillID, string(digest), fill.Position.TransactionHash, fill.Position.LogIndex, fill.PoolAddress, payload)
	exec(`INSERT INTO risk_authority.accounting_market_commits(publication_id,source_fill_id,source_fact_digest,projection_version,pool,bucket_start,position,fill,sequence,revision) VALUES($1,$2,$3,'projection-v1',$4,0,$5,$6,1,1)`, "publication-"+fill.SourceFillID, fill.SourceFillID, string(digest), fill.PoolAddress, repair1038JSON(fill.Position), payload)
	holder, err := holdings.NewRepository(db, "risk_holders")
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.DiscoverCanonical(ctx, creation.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	for _, event := range archive.Transfers {
		h, err := strconv.ParseInt(event.Height, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		if h > height {
			continue
		}
		if result, err := holder.ApplyCanonicalTransfer(ctx, event, "canonical"); err != nil || result.Incident {
			t.Fatalf("canonical ownership: %+v %v", result, err)
		}
	}
	exec(`UPDATE risk_holders.tokens SET coverage='complete' WHERE token=$1`, creation.Token)

	source := analytics.CanonicalRiskInputs{Authority: authority, HolderSchema: "risk_holders", SourceSchema: "indexer", SourceStream: "canonical", ProjectionVersion: "projection-v1"}
	wm := analytics.Watermark{ChainID: "125", Height: fill.Position.Height, BlockHash: fill.Position.BlockHash, ProjectionRevision: "1", ObservedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), Integrity: "healthy"}
	now := archive.Now.Unix()
	for _, tc := range []struct {
		name, sql string
		want      error
		unknown   bool
		storage   bool
	}{
		{name: "canonical inputs"},
		{name: "source block relation", sql: `ALTER VIEW indexer.indexer_block_completeness_v1 RENAME TO saved_blocks`, storage: true},
		{name: "creation relation", sql: `ALTER VIEW indexer.indexer_canonical_creations_v1 RENAME TO saved_creations`, storage: true},
		{name: "holder token relation", sql: `ALTER TABLE risk_holders.tokens RENAME TO saved_tokens`, storage: true},
		{name: "fill relation", sql: `ALTER VIEW indexer.indexer_accounting_fills_v1 RENAME TO saved_fills`, storage: true},
		{name: "risk persistence relation", sql: `ALTER TABLE risk_authority.analytics_risk_inputs RENAME TO saved_inputs`, storage: true},

		{name: "uncommitted fill", sql: `DELETE FROM risk_authority.accounting_market_commits`, unknown: true},
		{name: "creation metadata", sql: `UPDATE indexer.indexer_canonical_creations SET payload='[]'`, want: analytics.ErrConflict},
		{name: "creation supply", sql: `UPDATE indexer.indexer_canonical_creations SET payload='{"metadata":{"totalSupply":"invalid"}}'`, want: analytics.ErrConflict},
		{name: "creation time", sql: `UPDATE indexer.indexer_canonical_creations SET block_timestamp=9223372036854775807`, want: analytics.ErrConflict},
		{name: "source payload", sql: `UPDATE indexer.indexer_accounting_fills SET payload='[]'`, want: analytics.ErrConflict},
		{name: "source digest", sql: `UPDATE indexer.indexer_accounting_fills SET source_fact_digest='changed'`, want: analytics.ErrConflict},
		{name: "committed payload", sql: `UPDATE risk_authority.accounting_market_commits SET fill='[]'`, want: analytics.ErrConflict},
		{name: "committed digest", sql: `UPDATE risk_authority.accounting_market_commits SET source_fact_digest='changed'`, want: analytics.ErrConflict},
		{name: "incomplete block", sql: `UPDATE indexer.indexer_block_completeness SET required_decode_complete=false`, want: analytics.ErrDependencyUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if tc.sql != "" {
				if _, err := tx.Exec(ctx, tc.sql); err != nil {
					t.Fatal(err)
				}
			}
			got, err := source.LoadRiskInputsAt(ctx, tx, fill.PoolAddress, wm, now)
			if tc.storage {
				var sqlerr *pgconn.PgError
				if !errors.As(err, &sqlerr) || sqlerr.Code != "42P01" {
					t.Fatalf("risk source storage: %v", err)
				}
				return
			}
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("canonical fact: %+v %v", got, err)
				}
				return
			}
			if err != nil || got.SupplyRaw == "" || got.HolderCount < 1 {
				t.Fatalf("canonical risk inputs: %+v %v", got, err)
			}
			wantTraders := 1
			if tc.unknown {
				wantTraders = -1
			}
			if got.UniqueTraders24h != wantTraders {
				t.Fatalf("trader coverage: %+v", got)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		source analytics.CanonicalRiskInputs
		pool   string
		now    int64
		want   error
	}{
		{"pool address", source, "invalid", now, analytics.ErrInvalidInput},
		{"observation time", source, fill.PoolAddress, 0, analytics.ErrInvalidInput},
		{"authority schema", analytics.CanonicalRiskInputs{}, fill.PoolAddress, now, analytics.ErrInvalidInput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			_, err = tc.source.LoadRiskInputsAt(ctx, tx, tc.pool, wm, tc.now)
			if err == nil {
				t.Fatal("invalid risk binding accepted")
			}
			if tc.name != "authority schema" && !errors.Is(err, tc.want) {
				t.Fatalf("risk binding: %v", err)
			}
		})
	}
	if _, err := source.LoadRiskInputs(ctx, nil, fill.PoolAddress, wm); !errors.Is(err, analytics.ErrDependencyUnavailable) {
		t.Fatalf("missing risk transaction: %v", err)
	}
	invalidSource := source
	invalidSource.Authority.Schema = "invalid-schema"
	if err := invalidSource.Validate(); err == nil {
		t.Fatal("invalid authority schema accepted")
	}

}
