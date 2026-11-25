package l16

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5/pgxpool"
)

type incrementalNode struct {
	mu                                     sync.Mutex
	head, knownThrough, stateHeight, reorg int64
	primary                                bool
	hashes                                 map[string]int64
	blocks                                 map[int64]json.RawMessage
	receipts                               map[string]int64
	calls                                  map[string]int
	pages                                  [][2]int64
	oldAnswered, oldState, pruned          int
	origin                                 string
	client                                 *ethclient.Client
}

func incrementalHash(height int64) string { return common.BigToHash(big.NewInt(height)).Hex() }

func newIncrementalNode(t *testing.T, ctx context.Context, pool *pgxpool.Pool, origin, stream string, head, knownThrough, stateHeight int64, primary bool) *incrementalNode {
	t.Helper()
	n := &incrementalNode{head: head, knownThrough: knownThrough, stateHeight: stateHeight, primary: primary, hashes: map[string]int64{}, blocks: map[int64]json.RawMessage{}, receipts: map[string]int64{}, calls: map[string]int{}, origin: origin}
	direct, err := ethclient.DialContext(ctx, origin)
	if err != nil {
		t.Fatal(err)
	}
	defer direct.Close()
	rows, err := pool.Query(ctx, `SELECT height,block_hash FROM indexer.indexer_block_completeness_v1 WHERE stream_id=$1 AND height<=$2`, stream, knownThrough)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var height int64
		var hash string
		if err = rows.Scan(&height, &hash); err != nil {
			t.Fatal(err)
		}
		var raw json.RawMessage
		if err = direct.Client().CallContext(ctx, &raw, "eth_getBlockByNumber", hexutil.EncodeBig(big.NewInt(height)), false); err != nil {
			t.Fatal(err)
		}
		n.hashes[hash], n.blocks[height] = height, raw
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	rows, err = pool.Query(ctx, `SELECT DISTINCT transaction_hash,height FROM indexer.indexer_raw_logs_v1`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var hash string
		var height int64
		if err = rows.Scan(&hash, &height); err != nil {
			t.Fatal(err)
		}
		n.receipts[hash] = height
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { n.serve(t, w, r) }))
	t.Cleanup(server.Close)
	n.client, err = ethclient.DialContext(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(n.client.Close)
	return n
}

func (n *incrementalNode) block(height int64) json.RawMessage {
	if raw := n.blocks[height]; raw != nil {
		return raw
	}
	var block map[string]json.RawMessage
	if json.Unmarshal(n.blocks[n.knownThrough], &block) != nil {
		return nil
	}
	block["number"], _ = json.Marshal(hexutil.EncodeBig(big.NewInt(height)))
	block["hash"], _ = json.Marshal(incrementalHash(height))
	block["transactions"] = json.RawMessage(`[]`)
	var stamp hexutil.Uint64
	if json.Unmarshal(block["timestamp"], &stamp) != nil {
		return nil
	}
	block["timestamp"], _ = json.Marshal(hexutil.EncodeUint64(uint64(stamp) + uint64(height-n.knownThrough)))
	raw, _ := json.Marshal(block)
	return raw
}

func (n *incrementalNode) height(raw json.RawMessage) int64 {
	var hash string
	if json.Unmarshal(raw, &hash) == nil {
		if height, ok := n.hashes[hash]; ok {
			return height
		}
		if encoded, err := hexutil.Decode(hash); err == nil && len(encoded) == common.HashLength {
			value := new(big.Int).SetBytes(encoded)
			if value.IsInt64() && value.Int64() > n.knownThrough {
				return value.Int64()
			}
		}
		value, err := hexutil.DecodeBig(hash)
		if err == nil && value.IsInt64() {
			return value.Int64()
		}
	}
	var anchor struct {
		BlockHash string `json:"blockHash"`
	}
	if json.Unmarshal(raw, &anchor) == nil && anchor.BlockHash != "" {
		b, _ := json.Marshal(anchor.BlockHash)
		return n.height(b)
	}
	return 0
}

func (n *incrementalNode) serve(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	n.mu.Lock()
	defer n.mu.Unlock()
	var request struct {
		JSONRPC string            `json:"jsonrpc"`
		ID      json.RawMessage   `json:"id"`
		Method  string            `json:"method"`
		Params  []json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		t.Error(err)
		return
	}
	n.calls[request.Method]++
	reply := func(result any, err error) {
		envelope := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		if err != nil {
			envelope["error"] = map[string]any{"code": -32000, "message": err.Error()}
		} else {
			envelope["result"] = result
		}
		if e := json.NewEncoder(w).Encode(envelope); e != nil {
			t.Error(e)
		}
	}
	height := int64(0)
	state := false
	var filter map[string]json.RawMessage
	switch request.Method {
	case "eth_getBlockByNumber", "eth_getBlockByHash":
		height = n.height(request.Params[0])
	case "eth_getTransactionReceipt":
		var hash string
		_ = json.Unmarshal(request.Params[0], &hash)
		height = n.receipts[hash]
	case "eth_getLogs":
		if json.Unmarshal(request.Params[0], &filter) != nil {
			reply(nil, errors.New("invalid log filter"))
			return
		}
		if block := filter["blockHash"]; block != nil {
			height = n.height(block)
		} else {
			height = n.height(filter["fromBlock"])
			n.pages = append(n.pages, [2]int64{height, n.height(filter["toBlock"])})
		}
	case "eth_call", "eth_getCode", "eth_getBalance", "eth_getStorageAt":
		state = true
		height = n.height(request.Params[len(request.Params)-1])
	}
	if state && height < n.head {
		n.oldState++
		reply(nil, fmt.Errorf("state height %d below head %d", height, n.head))
		return
	}
	if n.primary && height > 0 && height < n.head-100000 {
		n.pruned++
		reply(nil, fmt.Errorf("height %d has been pruned; earliest available height %d", height, n.head-100000))
		return
	}
	if height > 0 && height < n.head-100000 && n.primary {
		n.oldAnswered++
	}
	if request.Method == "eth_getBlockByNumber" || request.Method == "eth_getBlockByHash" {
		raw := n.block(height)
		if n.reorg == height {
			var block map[string]json.RawMessage
			_ = json.Unmarshal(raw, &block)
			block["hash"], _ = json.Marshal("0x" + strings.Repeat("e", 64))
			raw, _ = json.Marshal(block)
		}
		reply(raw, nil)
		return
	}
	if request.Method == "eth_getLogs" && filter["blockHash"] == nil {
		if height > n.knownThrough {
			reply([]types.Log{}, nil)
			return
		}
		if n.height(filter["toBlock"]) > n.knownThrough {
			filter["toBlock"], _ = json.Marshal(hexutil.EncodeBig(big.NewInt(n.knownThrough)))
			request.Params[0], _ = json.Marshal(filter)
		}
	}
	if state {
		var hash string
		for candidate, h := range n.hashes {
			if h == n.stateHeight {
				hash = candidate
				break
			}
		}
		if hash == "" {
			reply(nil, errors.New("head state anchor unavailable"))
			return
		}
		request.Params[len(request.Params)-1], _ = json.Marshal(map[string]string{"blockHash": hash})
	}
	wire, err := json.Marshal(request)
	if err != nil {
		t.Error(err)
		return
	}
	response, err := http.Post(n.origin, "application/json", bytes.NewReader(wire))
	if err != nil {
		t.Error(err)
		return
	}
	defer response.Body.Close()
	if _, err = io.Copy(w, response.Body); err != nil {
		t.Error(err)
	}
}

func incrementalEmptyCoverage(t *testing.T, ctx context.Context, pool *pgxpool.Pool, stream string, from, to int64) {
	t.Helper()
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) SELECT 125,$1,h,'0x'||lpad(to_hex(h),64,'0'),0,true,true FROM generate_series($2::bigint,$3::bigint) h`, stream, from, to); err != nil {
		t.Fatal(err)
	}
}

func incrementalHead(t *testing.T, ctx context.Context, pool *pgxpool.Pool, stream string, height int64, hash string) {
	t.Helper()
	if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=$2,typed_complete_height=$2 WHERE stream_id=$1`, stream, height); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) VALUES(125,$1,$2,$3,'healthy','canonical-node')`, stream, height, hash); err != nil {
		t.Fatal(err)
	}
}

func incrementalWorker(t *testing.T, service *Service, wallet string) (*LiveTransferWorker, auth.IdentityPrincipal) {
	t.Helper()
	identity, p := inventoryIdentityFixture(t, wallet)
	worker, err := NewLiveTransferWorker(service, l17.Repository{Pool: service.authority.pool}, identity, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = worker.EnqueueOwned(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	return worker, p
}

func TestLiveWalletIncrementalEnrollmentRestart(t *testing.T) {
	fixture := newLocalCanonicalFixture(t, "incremental")
	ctx := fixture.ctx
	a := fixture.authority
	admin, err := pgxpool.New(ctx, fixture.db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	var creation int64
	if err = admin.QueryRow(ctx, `SELECT height FROM indexer.indexer_canonical_creations_v1 WHERE token=$1`, fixture.chain.Token).Scan(&creation); err != nil {
		t.Fatal(err)
	}
	known := int64(fixture.chain.Height)
	head := creation + 1200000
	second, err := ethclient.DialContext(ctx, fixture.chain.RPC)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	outbox, err := delivery.NewStoreWithEpochGuard(a.pool, "pnl", delivery.Policy{Subjects: map[string]bool{RebuildSubject: true, "portfolio.snapshot.v2": true}, EventTypes: map[string]bool{RebuildSubject: true, "portfolio.snapshot.v2": true}}, a.epochGuard)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(a, second, [2]string{"primary", "archive"}, strings.Repeat("c", 64), outbox)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := service.RebuildArchive(ctx, fixture.chain.Wallet, fixture.chain.Token, known)
	if err != nil {
		t.Fatal(err)
	}
	incrementalEmptyCoverage(t, ctx, admin, fixture.chain.Stream, known+1, head)
	incrementalHead(t, ctx, admin, fixture.chain.Stream, head, incrementalHash(head))
	primary := newIncrementalNode(t, ctx, admin, fixture.chain.RPC, fixture.chain.Stream, head, known, known, true)
	archive := newIncrementalNode(t, ctx, admin, fixture.chain.RPC, fixture.chain.Stream, head, known, known, false)
	a.rpc, a.liveFallback, service.second = primary.client, archive.client, archive.client
	worker, p := incrementalWorker(t, service, fixture.chain.Wallet)
	var accepted, progressHeight int64
	for pass := 0; pass < 40; pass++ {
		before := len(primary.pages)
		if changed, e := worker.processWallet(ctx, p.Wallet); e != nil || !changed {
			t.Fatal(pass, changed, e)
		}
		if pages := len(primary.pages) - before; pages > int(liveEnrollmentPages*10000/liveLogRangeBlocks) {
			t.Fatalf("pass %d read %d pages", pass, pages)
		}
		if err = admin.QueryRow(ctx, `SELECT accepted_height FROM pnl.inventory_l16_live_owners_v1 WHERE wallet=$1`, p.Wallet).Scan(&accepted); err != nil {
			t.Fatal(err)
		}
		if accepted == head {
			if pass < 2 {
				t.Fatal("enrollment was not bounded across passes")
			}
			break
		}
		var next int64
		if err = admin.QueryRow(ctx, `SELECT max(height) FROM pnl.inventory_l16_active_v2 WHERE partition LIKE 'live-enrollment:%'`).Scan(&next); err != nil || next <= progressHeight {
			t.Fatal(next, progressHeight, err)
		}
		progressHeight = next
		if pass == 0 {
			restarted := *service
			authority := *a
			restarted.authority = &authority
			worker, err = NewLiveTransferWorker(&restarted, l17.Repository{Pool: a.pool}, worker.identity, 1)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if accepted != head {
		t.Fatal("enrollment did not reach the accepted head", accepted)
	}
	active, err := service.Active(ctx, p.Wallet, fixture.chain.Token)
	if err != nil || active.State != reference.State {
		t.Fatal(active.State, reference.State, err)
	}
	var fills int
	if err = admin.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_accounting_fills_v1`).Scan(&fills); err != nil || fills != 3 {
		t.Fatal(fills, err)
	}
	if primary.oldState != 0 || archive.oldState != 0 || primary.oldAnswered != 0 || primary.pruned == 0 || archive.calls["eth_getLogs"] == 0 {
		t.Fatal(primary.oldState, archive.oldState, primary.oldAnswered, primary.pruned, archive.calls)
	}
	pages := len(primary.pages) + len(archive.pages)
	started := time.Now()
	if changed, e := worker.processWallet(ctx, p.Wallet); e != nil || !changed {
		t.Fatal(changed, e)
	}
	t.Logf("unchanged head pass: %s", time.Since(started))
	if len(primary.pages)+len(archive.pages) != pages {
		t.Fatal("unchanged head read movement pages")
	}
}

func TestLiveWalletIncrementalExtendsAcceptedHeightAndQuarantinesReorg(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	service, admin := recordedRebuildService(t, ctx)
	referenceTx, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := service.liveRebuildTx(ctx, referenceTx, realMarketCreator, realMarketToken, 33963749)
	referenceRollback := referenceTx.Rollback(ctx)
	if err != nil || referenceRollback != nil {
		t.Fatal(err, referenceRollback)
	}
	var firstHash string
	if err = admin.QueryRow(ctx, `SELECT block_hash FROM indexer.indexer_block_completeness_v1 WHERE height=33963733`).Scan(&firstHash); err != nil {
		t.Fatal(err)
	}
	incrementalHead(t, ctx, admin, realMarketStream, 33963733, firstHash)
	origin := coordinatorReplayRecordedRPC(t)
	primary := newIncrementalNode(t, ctx, admin, origin, realMarketStream, 33963733, 33963749, 33963733, true)
	archive := newIncrementalNode(t, ctx, admin, origin, realMarketStream, 33963733, 33963749, 33963733, false)
	a := service.authority
	a.rpc, a.liveFallback, service.second = primary.client, archive.client, archive.client
	worker, p := incrementalWorker(t, service, realMarketCreator)
	first, err := worker.RebuildOwned(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	incrementalEmptyCoverage(t, ctx, admin, realMarketStream, 33963750, 33963783)
	incrementalHead(t, ctx, admin, realMarketStream, 33963783, incrementalHash(33963783))
	primary.head, primary.stateHeight = 33963783, 33963749
	archive.head, archive.stateHeight = 33963783, 33963749
	before, archivePages := len(primary.pages), len(archive.pages)
	second, err := worker.RebuildOwned(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if second.SnapshotID == first.SnapshotID || second.Watermark.Height != "33963783" {
		t.Fatal(first, second)
	}
	if len(primary.pages)-before != 1 || len(archive.pages) != archivePages || primary.pages[before] != [2]int64{33963734, 33963783} {
		t.Fatal(primary.pages[before:], archive.pages[archivePages:])
	}
	active, err := service.Active(ctx, p.Wallet, realMarketToken)
	if err != nil || active.State != reference.State || active.InputDigest != reference.InputDigest {
		t.Fatal(active, reference, err)
	}
	// A full fold at the same head preserves state, inputs and checkpoints.
	referenceAuthority := *a
	referenceAuthority.generation = "full-live-replay"
	referenceService := *service
	referenceService.authority = &referenceAuthority
	fullTx, e := admin.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	full, e := referenceService.liveContinueTx(ctx, fullTx, p.Wallet, realMarketToken, 33963783, nil)
	rollbackErr := fullTx.Rollback(ctx)
	if e != nil || rollbackErr != nil || full.State != active.State || full.InputDigest != active.InputDigest || full.MovementDigest != active.MovementDigest || !reflect.DeepEqual(full.Checkpoints, active.Checkpoints) {
		t.Fatal(full, active, e, rollbackErr)
	}
	// The continuation digest includes the original incomplete movement page.
	tx, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inventoryRollback(tx)
	_, digest, _, err := a.liveMovementPages(ctx, tx, realMarketToken, 33963716, 33963783)
	if err != nil || digest != active.MovementDigest {
		t.Fatal(digest, active.MovementDigest, err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	before = len(primary.pages)
	started := time.Now()
	if _, err = worker.RebuildOwned(ctx, p); err != nil {
		t.Fatal(err)
	}
	t.Logf("unchanged head pass: %s", time.Since(started))
	if len(primary.pages) != before {
		t.Fatal("unchanged head read a page")
	}
	primary.reorg = 33963783
	if changed, e := worker.processWallet(ctx, p.Wallet); changed || !errors.Is(e, ErrParity) {
		t.Fatal(changed, e)
	}
	var accepted int64
	var commits, quarantines int
	if err = admin.QueryRow(ctx, `SELECT accepted_height,(SELECT count(*) FROM pnl.inventory_l16_live_commits_v1),(SELECT count(*) FROM pnl.inventory_l16_quarantine_v2) FROM pnl.inventory_l16_live_owners_v1 WHERE wallet=$1`, p.Wallet).Scan(&accepted, &commits, &quarantines); err != nil || accepted != 33963783 || commits != 2 || quarantines != 1 {
		t.Fatal(accepted, commits, quarantines, err)
	}
	if len(primary.pages) != before || primary.oldState != 0 || archive.oldState != 0 {
		t.Fatal(primary.pages[before:], primary.oldState, archive.oldState)
	}
	if value, e := strconv.ParseInt(second.Watermark.Height, 10, 64); e != nil || value != accepted {
		t.Fatal(value, e)
	}
}
