package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestCreationAndTransferBindCommittedRawRecords(t *testing.T) {
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	deliveryDDL, err := delivery.SchemaDDL("indexer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, deliveryDDL); err != nil {
		t.Fatal(err)
	}
	partitionDDL, err := delivery.PartitionSchemaDDL("indexer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, partitionDDL); err != nil {
		t.Fatal(err)
	}
	ddl, err := os.ReadFile(filepath.Join("../../../../..", "launchpad/spec/production-system/integration/migrations/190001.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(ddl)); err != nil {
		t.Fatal(err)
	}
	address := func(c string) string { return "0x" + strings.Repeat(c, 40) }
	hash := func(c string) string { return "0x" + strings.Repeat(c, 64) }
	factory, token, poolAddress := address("1"), address("2"), address("3")
	block, transaction := hash("4"), hash("5")
	for index, emitter := range []string{factory, token} {
		if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,101,0,$3,$4,'[]'::jsonb,$5)`, transaction, index, block, emitter, []byte{}); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	canonical := CanonicalTx{Tx: tx, Store: New(pool)}
	position := accounting.SourcePosition{ChainID: "125", Height: "101", TransactionIndex: "0", LogIndex: "0", TransactionHash: transaction, BlockHash: block}
	provenance := accounting.Provenance{DeploymentID: "current", ABIRevision: "revision", EmitterCodeEpoch: "epoch", PoolCodeEpoch: "pool-epoch", ReceiptSHA256: strings.Repeat("6", 64), HeaderSHA256: strings.Repeat("7", 64)}
	creation := CreationFact{Token: token, Pool: poolAddress, Creator: address("8"), Factory: factory, Optical: address("9"), NFTID: "12", Position: position, BlockTimestampSeconds: "1700000000", Provenance: provenance}
	metadataPosition := position
	metadataPosition.LogIndex = "1"
	creation.Metadata = &TokenCreationMetadata{Emitter: token, Name: "Launch token", Symbol: "LAUNCH", Decimals: 18, TotalSupply: "1000000000000000000", Position: metadataPosition, Provenance: provenance}
	for _, tc := range []struct {
		name string
		edit func(*CreationFact)
	}{
		{"zero token", func(f *CreationFact) { f.Token = address("0") }},
		{"pool epoch", func(f *CreationFact) { f.Provenance.PoolCodeEpoch = "" }},
		{"nft number", func(f *CreationFact) { f.NFTID = "-1" }},
		{"timestamp", func(f *CreationFact) { f.BlockTimestampSeconds = "0" }},
		{"source height", func(f *CreationFact) { f.Position.Height = "102" }},
		{"source missing", func(f *CreationFact) { f.Position.LogIndex = "2" }},
		{"source position", func(f *CreationFact) { f.Position.ChainID = "1" }},
		{"metadata emitter", func(f *CreationFact) { f.Metadata.Emitter = "invalid" }},
		{"metadata provenance", func(f *CreationFact) { f.Metadata.Provenance.ABIRevision = "" }},
		{"metadata name", func(f *CreationFact) { f.Metadata.Name = strings.Repeat("a", 65537) }},
		{"metadata transaction", func(f *CreationFact) { f.Metadata.Position.TransactionHash = hash("a") }},
		{"metadata supply", func(f *CreationFact) { f.Metadata.TotalSupply = "-1" }},
		{"metadata source", func(f *CreationFact) { f.Metadata.Position.LogIndex = "3" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(creation)
			if err != nil {
				t.Fatal(err)
			}
			var f CreationFact
			if err = json.Unmarshal(raw, &f); err != nil {
				t.Fatal(err)
			}
			tc.edit(&f)
			if err = canonical.InsertCreation(ctx, f); err == nil {
				t.Fatal("unbound creation accepted")
			}
		})
	}
	if err := (CanonicalTx{}).sourceBound(ctx, position, factory); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("missing transaction: %v", err)
	}
	if err := canonical.InsertCreation(ctx, creation); err != nil {
		t.Fatal(err)
	}
	if err := canonical.InsertCreation(ctx, creation); err != nil {
		t.Fatalf("identical creation replay: %v", err)
	}
	if found, queryErr := canonical.HasCanonicalToken(ctx, token); queryErr != nil || !found {
		t.Fatalf("committed creation identity: %t %v", found, queryErr)
	}
	changedCreation := creation
	changedCreation.Pool = address("a")
	if err := canonical.InsertCreation(ctx, changedCreation); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed creation identity: %v", err)
	}
	transferPosition := position
	transferPosition.LogIndex = "1"
	transfer := TransferFact{Token: token, From: address("b"), To: address("c"), AmountRaw: "15", Position: transferPosition, Provenance: provenance}
	if err := canonical.InsertTokenTransfer(ctx, transfer); err != nil {
		t.Fatal(err)
	}
	if err := canonical.InsertTokenTransfer(ctx, transfer); err != nil {
		t.Fatalf("identical transfer replay: %v", err)
	}
	changedTransfer := transfer
	changedTransfer.AmountRaw = "16"
	if err := canonical.InsertTokenTransfer(ctx, changedTransfer); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed transfer identity: %v", err)
	}
	for _, edit := range []func(*TransferFact){
		func(f *TransferFact) { f.Token = "invalid" }, func(f *TransferFact) { f.AmountRaw = "-1" }, func(f *TransferFact) { f.Position.Height = "102" },
	} {
		f := transfer
		edit(&f)
		if err := canonical.InsertTokenTransfer(ctx, f); err == nil {
			t.Fatal("unbound transfer accepted")
		}
	}
	var creations, transfers int
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM indexer.indexer_canonical_creations),(SELECT count(*) FROM indexer.indexer_token_transfers)`).Scan(&creations, &transfers); err != nil || creations != 1 || transfers != 1 {
		t.Fatalf("committed creation and transfer rows: %d %d %v", creations, transfers, err)
	}
}
