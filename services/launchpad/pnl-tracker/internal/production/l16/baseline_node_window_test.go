package l16

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func nodeWindowAdmin(t *testing.T, fixture localCanonicalFixture) *pgxpool.Pool {
	t.Helper()
	admin, err := pgxpool.New(fixture.ctx, fixture.db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	return admin
}

func nodeWindowIndexedMovements(t *testing.T, ctx context.Context, tx pgx.Tx, token string, start, end int64) []movement {
	t.Helper()
	rows, err := tx.Query(ctx, `SELECT transaction_hash,log_index,height,transaction_index,block_hash,from_address,to_address,amount_raw FROM indexer.indexer_token_transfers_v1 WHERE chain_id=125 AND token=$1 AND height BETWEEN $2 AND $3 ORDER BY height,transaction_index,log_index`, token, start, end)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []movement{}
	for rows.Next() {
		var m movement
		if err = rows.Scan(&m.TransactionHash, &m.LogIndex, &m.Height, &m.TransactionIndex, &m.BlockHash, &m.From, &m.To, &m.Amount); err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestNodeWindowWideRangeReadsTheNodeInsideTheBound(t *testing.T) {
	parallelDatabaseTest(t)
	fixture := newLocalCanonicalFixture(t, "incremental")
	ctx, a := fixture.ctx, fixture.authority
	admin := nodeWindowAdmin(t, fixture)
	var creation int64
	if err := admin.QueryRow(ctx, `SELECT height FROM indexer.indexer_canonical_creations_v1 WHERE token=$1`, fixture.chain.Token).Scan(&creation); err != nil {
		t.Fatal(err)
	}
	end := creation + 10001
	incrementalEmptyCoverage(t, ctx, admin, fixture.chain.Stream, int64(fixture.chain.Height)+1, end)
	incrementalHead(t, ctx, admin, fixture.chain.Stream, end, incrementalHash(end))
	origin := newIncrementalNode(t, ctx, admin, fixture.chain.RPC, fixture.chain.Stream, end, int64(fixture.chain.Height), int64(fixture.chain.Height), false)
	var mu sync.Mutex
	var ranges [][2]int64
	a.rpc = liveNodeProxy(t, origin.client, func(request liveNodeRequest) (any, error, bool) {
		if request.Method != "eth_getLogs" {
			return nil, nil, false
		}
		from, to := liveNodeFilter(t, request)
		mu.Lock()
		ranges = append(ranges, [2]int64{from, to})
		mu.Unlock()
		if to-from+1 > 2000 {
			return nil, fmt.Errorf("block range too large (%d), maximum allowed is 2000 blocks", to-from+1), true
		}
		return nil, nil, false
	})
	tx, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	want := nodeWindowIndexedMovements(t, ctx, tx, fixture.chain.Token, creation, end)
	got, digest, blockHash, err := a.verifiedMovements(ctx, tx, fixture.chain.Token, creation, end)
	if err != nil || len(want) == 0 || !reflect.DeepEqual(got, want) {
		t.Fatal(got, want, err)
	}
	raw, err := json.Marshal(want)
	if err != nil || digest != delivery.Hash(raw) || blockHash != incrementalHash(end) {
		t.Fatal(digest, blockHash, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if a.nodeWindowBlocks != 8 || len(ranges) != 1 {
		t.Fatal(a.nodeWindowBlocks, ranges)
	}
	for _, bounds := range ranges {
		if bounds[0] != end-a.nodeWindowBlocks+1 || bounds[1] != end || bounds[1]-bounds[0]+1 > a.nodeWindowBlocks {
			t.Fatal(bounds)
		}
	}
	for _, statement := range []string{
		`DELETE FROM indexer.indexer_block_completeness WHERE stream_id=$1 AND height=$2`,
		`UPDATE indexer.indexer_block_completeness SET raw_count=raw_count+1 WHERE stream_id=$1 AND height=$2`,
	} {
		nested, err := tx.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		tag, err := nested.Exec(ctx, statement, fixture.chain.Stream, creation+1)
		if err != nil || tag.RowsAffected() != 1 {
			t.Fatal(tag, err)
		}
		if _, _, _, err = a.verifiedMovements(ctx, nested, fixture.chain.Token, creation, end); !errors.Is(err, delivery.ErrGap) {
			t.Fatal(err)
		}
		if err = nested.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNodeWindowGivesTheSameMovementsWithAnyBound(t *testing.T) {
	parallelDatabaseTest(t)
	fixture := newLocalCanonicalFixture(t, "incremental")
	ctx, a := fixture.ctx, fixture.authority
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	var start int64
	if err = tx.QueryRow(ctx, `SELECT height FROM indexer.indexer_canonical_creations_v1 WHERE token=$1`, fixture.chain.Token).Scan(&start); err != nil {
		t.Fatal(err)
	}
	end := int64(fixture.chain.Height)
	a.nodeWindowBlocks = end - start + 1
	want, wantDigest, wantHash, err := a.verifiedMovements(ctx, tx, fixture.chain.Token, start, end)
	if err != nil || len(want) < 3 || !reflect.DeepEqual(want, nodeWindowIndexedMovements(t, ctx, tx, fixture.chain.Token, start, end)) {
		t.Fatal(want, err)
	}
	for _, bound := range []int64{1, 2, 8} {
		a.nodeWindowBlocks = bound
		got, digest, blockHash, err := a.verifiedMovements(ctx, tx, fixture.chain.Token, start, end)
		if err != nil || !reflect.DeepEqual(got, want) || digest != wantDigest || blockHash != wantHash {
			t.Fatal(bound, got, digest, blockHash, err)
		}
	}
}

func nodeWindowInventoryFixture(t *testing.T) (localCanonicalFixture, *pgxpool.Pool, []delivery.Envelope) {
	t.Helper()
	fixture := newLocalCanonicalFixture(t, "inventory")
	ctx, a := fixture.ctx, fixture.authority
	a.nodeWindowBlocks = 1
	if _, err := a.Capture(ctx, fixture.chain.Wallet, fixture.chain.Token); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.control, "trade"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		raw, err := os.ReadFile(filepath.Join(fixture.control, "traded.json"))
		if err == nil {
			if err = json.Unmarshal(raw, &fixture.chain); err != nil {
				t.Fatal(err)
			}
			break
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
	admin := nodeWindowAdmin(t, fixture)
	rows, err := admin.Query(ctx, `SELECT envelope FROM indexer.delivery_outbox WHERE subject=$1 ORDER BY (convert_from(envelope,'UTF8')::jsonb->'payload'->'positions'->0->>'sequence')::bigint`, accounting.InventorySubject)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	frames := []delivery.Envelope{}
	for rows.Next() {
		var raw []byte
		var frame delivery.Envelope
		if err = rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, &frame); err != nil {
			t.Fatal(err)
		}
		frames = append(frames, frame)
	}
	if err = rows.Err(); err != nil || len(frames) != 3 {
		t.Fatal(len(frames), err)
	}
	return fixture, admin, frames
}

func TestNodeWindowOlderMovementThatDiffersIsRefusedByTheBalance(t *testing.T) {
	parallelDatabaseTest(t)
	fixture, admin, frames := nodeWindowInventoryFixture(t)
	ctx, a := fixture.ctx, fixture.authority
	publication, err := accounting.DecodePublicationV1(frames[len(frames)-1].Payload)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := NewAccountingConsumerWithEpochGuard(a.pool, "pnl", a.consumerID, a.generation, a.owner, a.projectionVersion, a, a.epochGuard)
	if err != nil {
		t.Fatal(err)
	}
	if err = consumer.PreparePartition(ctx, publication.Positions[0].PartitionKey); err != nil {
		t.Fatal(err)
	}
	tx, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	fill, err := ReadSourceFillTx(ctx, tx, publication.SourceFillID, publication.SourceFactDigest)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.qualifyFreshTx(ctx, tx, a.generation, fill, publication.Positions[0], nil); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT payload FROM indexer.indexer_accounting_fills_v1 WHERE payload->'token'->>'address'=$1 ORDER BY (payload->'position'->>'height')::bigint,(payload->'position'->>'transactionIndex')::bigint,(payload->'position'->>'logIndex')::bigint LIMIT 1`, fixture.chain.Token).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	older, err := accounting.DecodeFillV1(raw)
	if err != nil || older.Position.Height == fill.Position.Height {
		t.Fatal(older, err)
	}
	amount, err := accounting.Uint(older.TokenAmountRaw)
	if err != nil {
		t.Fatal(err)
	}
	amount.Add(amount, big.NewInt(1))
	older.TokenAmountRaw = amount.String()
	changed, err := json.Marshal(older)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := accounting.FillDigest(older)
	if err != nil {
		t.Fatal(err)
	}
	tag, err := tx.Exec(ctx, `UPDATE indexer.indexer_accounting_fills SET payload=$1,source_fact_digest=$2 WHERE source_fill_id=$3`, changed, digest, older.SourceFillID)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatal(tag, err)
	}
	tag, err = tx.Exec(ctx, `UPDATE indexer.indexer_token_transfers SET amount_raw=$1 WHERE token=$2 AND transaction_hash=$3 AND to_address=$4`, amount.String(), fixture.chain.Token, older.Position.TransactionHash, fixture.chain.Wallet)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatal(tag, err)
	}
	tag, err = tx.Exec(ctx, `UPDATE indexer.indexer_raw_logs r SET data=$1 FROM indexer.indexer_token_transfers_v1 t WHERE t.chain_id=r.chain_id AND t.transaction_hash=r.transaction_hash AND t.log_index=r.log_index AND t.token=$2 AND t.transaction_hash=$3 AND t.to_address=$4`, common.LeftPadBytes(amount.Bytes(), 32), fixture.chain.Token, older.Position.TransactionHash, fixture.chain.Wallet)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatal(tag, err)
	}
	var start int64
	if err = tx.QueryRow(ctx, `SELECT height FROM indexer.indexer_canonical_creations_v1 WHERE token=$1`, fixture.chain.Token).Scan(&start); err != nil {
		t.Fatal(err)
	}
	moves, _, _, err := a.verifiedMovements(ctx, tx, fixture.chain.Token, start, int64(fixture.chain.Height))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range moves {
		if m.TransactionHash == older.Position.TransactionHash && m.To == fixture.chain.Wallet {
			found = m.Amount == amount.String() && m.Height < int64(fixture.chain.Height)
		}
	}
	if !found {
		t.Fatal("older indexed movement absent")
	}
	var balanceReads atomic.Int64
	a.rpc = liveNodeProxy(t, a.rpc, func(request liveNodeRequest) (any, error, bool) {
		if request.Method == "eth_call" && strings.Contains(string(request.Params[0]), common.Bytes2Hex(balanceSelector)) {
			balanceReads.Add(1)
		}
		return nil, nil, false
	})
	if err = a.qualifyFreshTx(ctx, tx, a.generation, fill, publication.Positions[0], nil); !errors.Is(err, delivery.ErrIntegrity) || balanceReads.Load() != 1 {
		t.Fatal(err, balanceReads.Load())
	}
}

func TestNodeWindowFillBeyondTheBoundWritesAPosition(t *testing.T) {
	parallelDatabaseTest(t)
	fixture, admin, frames := nodeWindowInventoryFixture(t)
	ctx, a := fixture.ctx, fixture.authority
	var creation, firstFill int64
	if err := admin.QueryRow(ctx, `SELECT (SELECT height FROM indexer.indexer_canonical_creations_v1 WHERE token=$1),(SELECT min((payload->'position'->>'height')::bigint) FROM indexer.indexer_accounting_fills_v1 WHERE payload->'token'->>'address'=$1)`, fixture.chain.Token).Scan(&creation, &firstFill); err != nil {
		t.Fatal(err)
	}
	if a.nodeWindowBlocks != 1 || firstFill-creation < a.nodeWindowBlocks {
		t.Fatal(creation, firstFill, a.nodeWindowBlocks)
	}
	consumeCurrentInventoryFrames(t, ctx, &Service{authority: a}, frames)
	var stateRaw []byte
	var revision, uses, commits int64
	partition := "125:" + fixture.chain.Wallet + ":" + fixture.chain.Token
	if err := admin.QueryRow(ctx, `SELECT state,revision,(SELECT count(*) FROM pnl.inventory_l16_baseline_uses WHERE partition=$1),(SELECT count(*) FROM pnl.delivery_partition_commits WHERE partition_key=$1) FROM pnl.inventory_l16 WHERE partition LIKE '%'||$1`, partition).Scan(&stateRaw, &revision, &uses, &commits); err != nil {
		t.Fatal(err)
	}
	state, err := DecodeInventoryState(stateRaw)
	if err != nil {
		t.Fatal(err)
	}
	var blockHash string
	if err = admin.QueryRow(ctx, `SELECT block_hash FROM indexer.indexer_block_completeness_v1 WHERE stream_id=$1 AND height=$2`, fixture.chain.Stream, int64(fixture.chain.Height)).Scan(&blockHash); err != nil {
		t.Fatal(err)
	}
	observed, err := a.observedBalance(ctx, fixture.chain.Token, fixture.chain.Wallet, blockHash)
	if err != nil || observed.Sign() <= 0 || state.Known != observed.String() || state.Unknown != "0" || revision != 3 || uses != 3 || commits != 3 {
		t.Fatal(state, observed, revision, uses, commits, err)
	}
}
