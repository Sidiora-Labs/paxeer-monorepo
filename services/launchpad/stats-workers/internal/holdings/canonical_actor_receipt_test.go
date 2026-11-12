package holdings

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestCanonicalActorEnrichmentFromRecordedBuyReceipt(t *testing.T) {
	ctx, pool, _ := canonicalLocalFixture(t)
	raw, err := os.ReadFile(filepath.Join("..", "projectionstats", "testdata", "real_pool_125.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tape struct {
		Token, Pool, USDL, EventEmitter string
		Blocks                          map[string]struct{ Timestamp int64 }
		Transactions                    []struct {
			Hash, From, BlockHash         string
			BlockNumber, TransactionIndex int64
			Logs                          []struct {
				LogIndex      int64
				Address, Data string
				Topics        []string
			}
		}
	}
	if err := json.Unmarshal(raw, &tape); err != nil {
		t.Fatal(err)
	}
	if len(tape.Transactions) < 2 {
		t.Fatal("creation and buy receipts required")
	}
	mintReceipt, buyReceipt := tape.Transactions[0], tape.Transactions[1]
	decodeTransfer := func(transaction int, logIndex int64) Transfer {
		t.Helper()
		tx := tape.Transactions[transaction]
		for _, log := range tx.Logs {
			if log.LogIndex != logIndex {
				continue
			}
			data, err := hex.DecodeString(strings.TrimPrefix(log.Data, "0x"))
			if err != nil {
				t.Fatal(err)
			}
			topics := make([]common.Hash, len(log.Topics))
			for i, topic := range log.Topics {
				topics[i] = common.HexToHash(topic)
			}
			event, err := TransferFromLog(types.Log{Address: common.HexToAddress(log.Address), Topics: topics, Data: data, BlockNumber: uint64(tx.BlockNumber), TxHash: common.HexToHash(tx.Hash), TxIndex: uint(tx.TransactionIndex), BlockHash: common.HexToHash(tx.BlockHash), Index: uint(log.LogIndex)})
			if err != nil {
				t.Fatal(err)
			}
			return event
		}
		t.Fatal("recorded transfer log missing")
		return Transfer{}
	}
	mint := decodeTransfer(0, 2)
	poolFunding := decodeTransfer(0, 10)
	buy := decodeTransfer(1, 1)
	if mint.From != Zero || buy.TransactionHash != buyReceipt.Hash || buy.Token != tape.Token {
		t.Fatalf("receipt transfer identities: %+v %+v", mint, buy)
	}
	ddl, err := SchemaSQL("holdings_receipt_actor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repository, err := NewRepository(pool, "holdings_receipt_actor")
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Discover(ctx, tape.Token, 6, strconv.FormatInt(mintReceipt.BlockNumber, 10), true); err != nil {
		t.Fatal(err)
	}
	for _, transfer := range []Transfer{mint, poolFunding, buy} {
		if result, err := repository.Apply(ctx, transfer); err != nil || result.Duplicate || result.Revision == 0 {
			t.Fatalf("recorded transfer: %+v %v", result, err)
		}
	}
	var swapLog struct {
		LogIndex      int64
		Address, Data string
		Topics        []string
	}
	for _, log := range buyReceipt.Logs {
		if log.Address == tape.EventEmitter && len(log.Topics) == 3 {
			swapLog = log
			break
		}
	}
	data, err := hex.DecodeString(strings.TrimPrefix(swapLog.Data, "0x"))
	if err != nil || len(data) != 7*32 {
		t.Fatalf("recorded swap: %v", err)
	}
	word := func(i int) string { return new(big.Int).SetBytes(data[i*32 : (i+1)*32]).String() }
	if word(0) != "1" {
		t.Fatal("buy receipt side")
	}
	fill := accounting.FillV1{Version: accounting.FillVersion,
		Position:      accounting.SourcePosition{ChainID: "125", Height: strconv.FormatInt(buyReceipt.BlockNumber, 10), TransactionIndex: strconv.FormatInt(buyReceipt.TransactionIndex, 10), LogIndex: strconv.FormatInt(swapLog.LogIndex, 10), TransactionHash: buyReceipt.Hash, BlockHash: buyReceipt.BlockHash},
		ActualEmitter: tape.EventEmitter, PoolAddress: tape.Pool, Token: accounting.Asset{Address: tape.Token, Decimals: 6}, Settlement: accounting.Asset{Address: tape.USDL, Decimals: 6},
		Side: "buy", TokenAmountRaw: word(2), USDLAmountRaw: word(1), PaidInputRaw: word(1), FeeRaw: word(3), FeeDisposition: "distributed_usdl", InputDerivationQuality: "receipt_incomplete",
		Attribution: accounting.Attribution{TransactionSender: buyReceipt.From, Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Execution: accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Provenance: accounting.Provenance{DeploymentID: "paxeer125-a35ff7c6f532d2f9", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "event-emitter-1", PoolCodeEpoch: "pool-1", ReceiptSHA256: strings.Repeat("a", 64), HeaderSHA256: strings.Repeat("b", 64)}, BlockTimestampSeconds: strconv.FormatInt(tape.Blocks[strconv.FormatInt(buyReceipt.BlockNumber, 10)].Timestamp, 10)}
	fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
	if err := accounting.ValidateFillV1(fill); err != nil {
		t.Fatal(err)
	}
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	fillRaw, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	logTopics, err := json.Marshal(swapLog.Topics)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8)`, buyReceipt.Hash, swapLog.LogIndex, buyReceipt.BlockNumber, buyReceipt.TransactionIndex, buyReceipt.BlockHash, swapLog.Address, logTopics, data); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_accounting_fills(source_fill_id,source_fact_digest,chain_id,transaction_hash,log_index,pool_address,payload) VALUES($1,$2,125,$3,$4,$5,$6)`, fill.SourceFillID, string(digest), buyReceipt.Hash, swapLog.LogIndex, tape.Pool, fillRaw); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,'canonical',$1,$2,1,true,true)`, buyReceipt.BlockNumber, buyReceipt.BlockHash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_commits(chain_id,stream_id,batch_id,previous_height,last_complete_height,block_hash,record_count,outbox_count) VALUES(125,'canonical','recorded-buy',10,$1,$2,1,0)`, buyReceipt.BlockNumber, buyReceipt.BlockHash); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := repository.EnrichCanonicalActorTx(ctx, tx, buy, fill, "canonical"); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var saved []byte
	if err := pool.QueryRow(ctx, "SELECT payload FROM holdings_receipt_actor.provenance WHERE identity=$1", buy.Identity()).Scan(&saved); err != nil {
		t.Fatal(err)
	}
	var provenance Provenance
	if err := json.Unmarshal(saved, &provenance); err != nil || provenance.Actor == nil || *provenance.Actor != buyReceipt.From {
		t.Fatalf("recorded actor: %+v %v", provenance, err)
	}
	changed := fill
	changed.Position.BlockHash = "0x" + strings.Repeat("f", 64)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.EnrichCanonicalActorTx(ctx, tx, buy, changed, "canonical"); !errors.Is(err, ErrConflict) {
		_ = tx.Rollback(ctx)
		t.Fatalf("changed actor source: %v", err)
	}
	if err := tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
}
