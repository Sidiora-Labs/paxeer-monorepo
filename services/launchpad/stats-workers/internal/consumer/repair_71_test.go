package consumer_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/consumer"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/projectionstats"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repair71Log struct {
	Address, BlockHash, TransactionHash, Data string
	BlockNumber, TransactionIndex, Index      int64
	Topics                                    []string
}
type repair71Decoded struct {
	Name, Signature, Address, TransactionHash, BlockHash string
	BlockNumber, TransactionIndex, LogIndex              int64
	Args                                                 []json.RawMessage
}
type repair71Fixture struct {
	dsn          string
	pg           *pgxpool.Pool
	st           *store.Store
	bridge       *consumer.CanonicalLegacyBridge
	expected     store.CrossTokenSwapRow
	routePayload []byte
	fills        []accounting.FillV1
}

func repair71JSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func repair71Text(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return strings.ToLower(s)
}
func (f *repair71Fixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.pg.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%v: %.180s", err, sql)
	}
}
func (f *repair71Fixture) open(t *testing.T) {
	t.Helper()
	f.st = store.New(f.pg)
	rdb := internaltest.NewRedis(t)
	market, err := consumer.NewMarketConsumer(f.st, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	holder := consumer.NewHolderTracker(f.st, rdb, discardLogger(), time.Hour)
	t.Cleanup(holder.Close)
	f.bridge = &consumer.CanonicalLegacyBridge{Pool: f.pg, ProjectionSchema: "stats", SourceSchema: "indexer", SourceStream: legacyStream, ProjectionVersion: legacyVersion, Market: market, Swap: consumer.NewSwapConsumer(f.st, rdb, discardLogger()), Holder: holder, Batch: 100}
}
func (f *repair71Fixture) restart(t *testing.T) {
	t.Helper()
	f.bridge.Holder.Close()
	f.pg.Close()
	pg, err := pgxpool.New(context.Background(), f.dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pg.Close)
	f.pg = pg
	f.open(t)
}
func (f *repair71Fixture) commitLeg(t *testing.T, fill accounting.FillV1, sequence int) {
	t.Helper()
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO stats.accounting_market_commits(publication_id,source_fill_id,source_fact_digest,projection_version,pool,bucket_start,position,fill,sequence,revision) VALUES($1,$2,$3,$4,$5,$6,'{}',$7,$8,1)`, "publication-"+fill.SourceFillID, fill.SourceFillID, string(digest), legacyVersion, fill.PoolAddress, int64(1791504060), string(repair71JSON(t, fill)), sequence)
}
func (f *repair71Fixture) deliverRoute(t *testing.T) {
	t.Helper()
	// Re-deliver the archived producer's identical route identity and payload.
	f.exec(t, `INSERT INTO indexer.indexer_ingest_outbox(chain_id,transaction_hash,log_index,event_kind,payload) VALUES(125,$1,12,'MultihopSwap',$2) ON CONFLICT DO NOTHING`, f.expected.TxHash, string(f.routePayload))
}

func newRepair71Fixture(t *testing.T) *repair71Fixture {
	t.Helper()
	ctx := context.Background()
	dir := filepath.Join("..", "..", "..", "..", "..", "sdk", "launchpad", "protocol", "contracts", "v1", "chain")
	// This authority checks the immutable receipt/ABI pins and admits only mined
	// historical core fills. No invented pair of fills supplies a route marker.
	authority, err := projectionstats.OpenHistoricalCoreAuthority(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "golden-fixtures.json"))
	if err != nil {
		t.Fatal(err)
	}
	var archive struct {
		ChainID        int
		ReplayVerified bool
		Addresses      map[string]string
		Receipts       []struct {
			Label, Hash, BlockHash, From string
			BlockNumber, Index           int64
			Logs                         []repair71Log
		}
		DecodedLogs []repair71Decoded
	}
	if err = json.Unmarshal(raw, &archive); err != nil || archive.ChainID != 125 || !archive.ReplayVerified {
		t.Fatal(err)
	}
	f := &repair71Fixture{}
	f.dsn, f.pg = internaltest.NewPostgresWithDSN(t)
	f.exec(t, "CREATE SCHEMA indexer")
	for _, id := range []string{"60000", "80000", "190001"} {
		ddl, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "launchpad", "spec", "production-system", "integration", "migrations", id+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		f.exec(t, string(ddl))
	}
	repo, err := projectionstats.NewRepository(f.pg, "stats")
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, repo.MigrationSQL())
	f.exec(t, `INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,$1,'archive-indexer',1,clock_timestamp()+interval '1 hour')`, legacyStream)
	f.exec(t, `INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,$1,68,68)`, legacyStream)
	var receiptHash, blockHash, origin string
	for _, r := range archive.Receipts {
		if r.Label != "multihop" {
			continue
		}
		receiptHash, blockHash, origin = strings.ToLower(r.Hash), strings.ToLower(r.BlockHash), strings.ToLower(r.From)
		for _, l := range r.Logs {
			data, err := hex.DecodeString(strings.TrimPrefix(l.Data, "0x"))
			if err != nil {
				t.Fatal(err)
			}
			topics := make([]string, len(l.Topics))
			for i, s := range l.Topics {
				topics[i] = strings.ToLower(s)
			}
			f.exec(t, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8)`, receiptHash, l.Index, r.BlockNumber, r.Index, blockHash, strings.ToLower(l.Address), string(repair71JSON(t, topics)), data)
		}
		f.exec(t, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,$4,true,true)`, legacyStream, r.BlockNumber, blockHash, len(r.Logs))
	}
	if receiptHash == "" {
		t.Fatal("mined multihop receipt missing")
	}
	var route repair71Decoded
	tokensByPool := map[string]string{}
	for _, d := range archive.DecodedLogs {
		if d.Name == "MarketCreated" && strings.EqualFold(d.Address, archive.Addresses["factory"]) && d.BlockNumber < 68 {
			token, pool, creator := repair71Text(t, d.Args[0]), repair71Text(t, d.Args[1]), repair71Text(t, d.Args[2])
			tokensByPool[pool] = token
			f.exec(t, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'{}',$13)`, token, pool, creator, strings.ToLower(d.Address), repair71Text(t, d.Args[4]), repair71Text(t, d.Args[3]), d.BlockNumber, strings.ToLower(d.BlockHash), strings.ToLower(d.TransactionHash), d.TransactionIndex, d.LogIndex, int64(1791504000)+d.BlockNumber, string(repair71JSON(t, d)))
		}
		if d.Name == "MultihopSwap" && strings.EqualFold(d.TransactionHash, receiptHash) {
			route = d
		}
	}
	if route.Name == "" || route.Signature != "MultihopSwap(address,address,address,uint256,uint256,uint256)" {
		t.Fatal("source route evidence missing")
	}
	args := map[string]string{}
	for i, k := range []string{"sender", "tokenIn", "tokenOut", "amountIn", "intermediateUsdl", "amountOut"} {
		args[k] = repair71Text(t, route.Args[i])
	}
	// Preserve the real indexer Event DTO, derived only from the recorded
	// producer's decoded log and its original receipt coordinates.
	f.routePayload = repair71JSON(t, struct {
		EventName, Sig string
		Args           map[string]string
		BlockNumber    int64
		TxHash         string
		LogIndex       int64
	}{route.Name, route.Signature, args, route.BlockNumber, strings.ToLower(route.TransactionHash), route.LogIndex})
	evidence := fmt.Sprintf("%x", sha256.Sum256(raw))
	for i, trade := range authority.Trades(receiptHash) {
		if _, err = authority.Account(trade); err != nil {
			t.Fatal(err)
		}
		var swapLog repair71Log
		for _, r := range archive.Receipts {
			if r.Label == "multihop" {
				for _, l := range r.Logs {
					if strconv.FormatInt(l.Index, 10) == trade.Position.LogIndex {
						swapLog = l
					}
				}
			}
		}
		data, err := hex.DecodeString(strings.TrimPrefix(swapLog.Data, "0x"))
		if err != nil || len(data) != 224 {
			t.Fatal("real Swap ABI evidence", err)
		}
		ts := new(big.Int).SetBytes(data[160:192]).String()
		fill := accounting.FillV1{
			Version:       accounting.FillVersion,
			Position:      accounting.SourcePosition{ChainID: trade.Position.ChainID, Height: trade.Position.Height, TransactionIndex: trade.Position.TransactionIndex, LogIndex: trade.Position.LogIndex, TransactionHash: trade.Position.TransactionHash, BlockHash: trade.Position.BlockHash},
			ActualEmitter: trade.EmittingContract, PoolAddress: trade.PoolAddress, Token: accounting.Asset{Address: tokensByPool[trade.PoolAddress], Decimals: 6}, Settlement: accounting.Asset{Address: strings.ToLower(archive.Addresses["usdl"]), Decimals: 6},
			Side: trade.Side, TokenAmountRaw: trade.TokenAmountRaw, USDLAmountRaw: trade.USDLAmountRaw, PaidInputRaw: *trade.PaidInputRaw, FeeRaw: trade.FeeRaw, InputDerivationQuality: trade.InputDerivationQuality, FeeDisposition: trade.FeeDisposition,
			Attribution: accounting.Attribution{TransactionSender: origin, Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Execution: accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
			Provenance: accounting.Provenance{DeploymentID: "isolated-hardhat-chain125", ABIRevision: "sidiora-v1", EmitterCodeEpoch: trade.CodeEpoch, PoolCodeEpoch: "archived-pool-v1", ReceiptSHA256: evidence, HeaderSHA256: evidence}, BlockTimestampSeconds: ts,
		}
		fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
		digest, err := accounting.FillDigest(fill)
		if err != nil {
			t.Fatal(err)
		}
		index, err := strconv.ParseInt(fill.Position.LogIndex, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		f.exec(t, `INSERT INTO indexer.indexer_accounting_fills(source_fill_id,source_fact_digest,chain_id,transaction_hash,log_index,pool_address,payload) VALUES($1,$2,125,$3,$4,$5,$6)`, fill.SourceFillID, string(digest), receiptHash, index, fill.PoolAddress, string(repair71JSON(t, fill)))
		f.commitLeg(t, fill, i+1)
		f.fills = append(f.fills, fill)
	}
	if len(f.fills) != 2 || f.fills[0].Side != "sell" || f.fills[1].Side != "buy" {
		t.Fatal("actual route legs", f.fills)
	}
	f.expected = store.CrossTokenSwapRow{ID: receiptHash + "-12", Sender: args["sender"], TokenIn: args["tokenIn"], TokenOut: args["tokenOut"], PoolIn: f.fills[0].PoolAddress, PoolOut: f.fills[1].PoolAddress, AmountIn: "14969972418576", IntermediateUsdl: "297812138", AmountOut: "28835684790084", FeeIn: "44909917255", FeeOut: "893436", BlockTimestamp: 1791504068, TxHash: receiptHash}
	f.open(t)
	if n, err := f.bridge.RunOnce(ctx); err != nil || n != 4 {
		t.Fatalf("ordinary canonical projections: n=%d err=%v", n, err)
	}
	f.assertHistory(t, 0)
	return f
}

func (f *repair71Fixture) assertHistory(t *testing.T, want int) {
	t.Helper()
	router := chi.NewRouter()
	httpapi.RegisterCrossTokenSwaps(router, f.st)
	for _, path := range []string{"/stats/cross-token-swaps/token/" + strings.ToUpper(f.expected.TokenIn), "/stats/cross-token-swaps/token/" + strings.ToUpper(f.expected.TokenOut), "/stats/cross-token-swaps/" + strings.ToUpper(f.expected.Sender)} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		var body struct{ Swaps []store.CrossTokenSwapRow }
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &body) != nil || len(body.Swaps) != want {
			t.Fatalf("%s: status=%d body=%s", path, response.Code, response.Body.String())
		}
		if want == 1 && body.Swaps[0] != f.expected {
			t.Fatalf("%s: got %+v want %+v", path, body.Swaps[0], f.expected)
		}
	}
	var count int
	if err := f.pg.QueryRow(context.Background(), `SELECT count(*) FROM stats.cross_token_swaps`).Scan(&count); err != nil || count != want {
		t.Fatal(count, err)
	}
}

func TestRepair_71_CrossTokenHistoryDuplicateAndRestart(t *testing.T) {
	f := newRepair71Fixture(t)
	f.deliverRoute(t)
	if n, err := f.bridge.RunOnce(context.Background()); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	f.assertHistory(t, 1)
	f.deliverRoute(t)
	if n, err := f.bridge.RunOnce(context.Background()); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	f.assertHistory(t, 1)
	f.restart(t)
	if n, err := f.bridge.RunOnce(context.Background()); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	f.assertHistory(t, 1)
}

func TestRepair_71_WaitsForBothCommittedLegs(t *testing.T) {
	f := newRepair71Fixture(t)
	f.deliverRoute(t)
	f.exec(t, `DELETE FROM stats.accounting_market_commits WHERE source_fill_id=$1`, f.fills[1].SourceFillID)
	if n, err := f.bridge.RunOnce(context.Background()); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	f.assertHistory(t, 0)
	f.restart(t)
	f.commitLeg(t, f.fills[1], 2)
	if n, err := f.bridge.RunOnce(context.Background()); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	f.assertHistory(t, 1)
}

func TestRepair_71_RejectsUnboundRouteEvidence(t *testing.T) {
	for _, caseName := range []string{"route-amount", "route-token", "typed-route", "leg-fee", "leg-position", "token-pool", "receipt-provenance", "unqualified-marker"} {
		t.Run(caseName, func(t *testing.T) {
			f := newRepair71Fixture(t)
			f.deliverRoute(t)
			switch caseName {
			case "route-amount":
				f.exec(t, `UPDATE indexer.indexer_raw_logs SET data=overlay(data placing '\x01'::bytea from 32 for 1) WHERE transaction_hash=$1 AND log_index=12`, f.expected.TxHash)
			case "route-token":
				f.exec(t, `UPDATE indexer.indexer_raw_logs SET topics=jsonb_set(topics,'{1}',topics->2) WHERE transaction_hash=$1 AND log_index=12`, f.expected.TxHash)
			case "typed-route":
				f.exec(t, `UPDATE indexer.indexer_ingest_outbox SET payload=jsonb_set(payload,'{Args,amountOut}','"1"') WHERE transaction_hash=$1`, f.expected.TxHash)
			case "leg-fee":
				f.exec(t, `UPDATE indexer.indexer_raw_logs SET data=overlay(data placing '\x01'::bytea from 128 for 1) WHERE transaction_hash=$1 AND log_index=2`, f.expected.TxHash)
			case "leg-position":
				fill := f.fills[0]
				fill.Position.LogIndex = "1"
				fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
				digest, err := accounting.FillDigest(fill)
				if err != nil {
					t.Fatal(err)
				}
				f.exec(t, `UPDATE indexer.indexer_accounting_fills SET payload=$2,source_fact_digest=$3 WHERE source_fill_id=$1`, f.fills[0].SourceFillID, string(repair71JSON(t, fill)), string(digest))
			case "token-pool":
				f.exec(t, `UPDATE indexer.indexer_canonical_creations SET token=$2 WHERE pool=$1`, f.expected.PoolOut, "0x"+strings.Repeat("f", 40))
			case "receipt-provenance":
				fill := f.fills[1]
				fill.Provenance.ReceiptSHA256 = strings.Repeat("e", 64)
				digest, err := accounting.FillDigest(fill)
				if err != nil {
					t.Fatal(err)
				}
				f.exec(t, `UPDATE indexer.indexer_accounting_fills SET payload=$2,source_fact_digest=$3 WHERE source_fill_id=$1`, fill.SourceFillID, string(repair71JSON(t, fill)), string(digest))
				f.exec(t, `UPDATE stats.accounting_market_commits SET fill=$2,source_fact_digest=$3 WHERE source_fill_id=$1`, fill.SourceFillID, string(repair71JSON(t, fill)), string(digest))
			case "unqualified-marker":
				f.exec(t, `DELETE FROM indexer.indexer_ingest_outbox WHERE transaction_hash=$1`, f.expected.TxHash)
				if n, err := f.bridge.RunOnce(context.Background()); err != nil || n != 0 {
					t.Fatal(n, err)
				}
				f.assertHistory(t, 0)
				return
			}
			if _, err := f.bridge.RunOnce(context.Background()); !errors.Is(err, consumer.ErrLegacyBridgeIntegrity) {
				t.Fatalf("accepted %s: %v", caseName, err)
			}
			f.assertHistory(t, 0)
		})
	}
}

func TestRepair_71_RouteRollbackAndRestartRecovery(t *testing.T) {
	f := newRepair71Fixture(t)
	f.deliverRoute(t)
	f.exec(t, `ALTER TABLE stats.cross_token_swaps ADD CONSTRAINT repair71_block CHECK(fee_in <> '44909917255')`)
	if _, err := f.bridge.RunOnce(context.Background()); err == nil {
		t.Fatal("database constraint did not abort route")
	}
	f.assertHistory(t, 0)
	f.restart(t)
	f.exec(t, `ALTER TABLE stats.cross_token_swaps DROP CONSTRAINT repair71_block`)
	if n, err := f.bridge.RunOnce(context.Background()); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	f.assertHistory(t, 1)
}

func TestRepair_71_RequiresCompleteCanonicalSource(t *testing.T) {
	f := newRepair71Fixture(t)
	f.deliverRoute(t)
	f.exec(t, `UPDATE indexer.indexer_block_completeness SET required_decode_complete=false`)
	if _, err := f.bridge.RunOnce(context.Background()); !errors.Is(err, consumer.ErrAccountingSourceUnavailable) {
		t.Fatal("incomplete source admitted", err)
	}
	f.assertHistory(t, 0)
	f.exec(t, `UPDATE indexer.indexer_block_completeness SET required_decode_complete=true`)
	f.exec(t, `UPDATE indexer.indexer_ingest_leases SET expires_at=clock_timestamp()-interval '1 second'`)
	if _, err := f.bridge.RunOnce(context.Background()); !errors.Is(err, consumer.ErrAccountingSourceUnavailable) {
		t.Fatal("expired source lease admitted", err)
	}
	f.assertHistory(t, 0)
	f.exec(t, `UPDATE indexer.indexer_ingest_leases SET expires_at=clock_timestamp()+interval '1 hour'`)
	f.exec(t, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=67,typed_complete_height=67`)
	if n, err := f.bridge.RunOnce(context.Background()); err != nil || n != 0 {
		t.Fatal("route ahead of source watermark", n, err)
	}
	f.assertHistory(t, 0)
	f.exec(t, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=68,typed_complete_height=68`)
	if n, err := f.bridge.RunOnce(context.Background()); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	f.assertHistory(t, 1)
}

func TestRepair_71_CanonicalTransactionAtomicity(t *testing.T) {
	f := newRepair71Fixture(t)
	ctx := context.Background()
	c := consumer.NewMultihopConsumer(f.st, discardLogger())
	ev := consumer.MultihopEvent{Sender: f.expected.Sender, TokenIn: f.expected.TokenIn, TokenOut: f.expected.TokenOut, PoolIn: f.expected.PoolIn, PoolOut: f.expected.PoolOut, AmountIn: f.expected.AmountIn, IntermediateUsdl: f.expected.IntermediateUsdl, AmountOut: f.expected.AmountOut, FeeIn: f.expected.FeeIn, FeeOut: f.expected.FeeOut, BlockTimestamp: f.expected.BlockTimestamp, TxHash: f.expected.TxHash, LogIndex: 12}
	tx, err := f.pg.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if inserted, err := c.ProjectCanonicalTx(ctx, tx, ev); err != nil || !inserted {
		t.Fatal(inserted, err)
	}
	f.assertHistory(t, 0)
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	f.assertHistory(t, 0)
	tx, err = f.pg.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if inserted, err := c.ProjectCanonicalTx(ctx, tx, ev); err != nil || !inserted {
		t.Fatal(inserted, err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	f.assertHistory(t, 1)
	tx, err = f.pg.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if inserted, err := c.ProjectCanonicalTx(ctx, tx, ev); err != nil || inserted {
		t.Fatal(inserted, err)
	}
	ev.FeeOut = "1"
	if _, err = c.ProjectCanonicalTx(ctx, tx, ev); !errors.Is(err, consumer.ErrLegacyBridgeIntegrity) {
		t.Fatal("conflicting canonical identity accepted", err)
	}
	if err = tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatal(err)
	}
	f.assertHistory(t, 1)
}
