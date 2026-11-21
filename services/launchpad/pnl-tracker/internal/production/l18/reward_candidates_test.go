package l18

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5/pgxpool"
)

func rewardCandidateMarkets(d *rewardHistoryDatabase, count int) {
	base := d.peers.Markets[0]
	d.peers.Markets = nil
	d.peers.Transfers = nil
	d.peers.Claims = nil
	d.peers.Claimable = map[string]map[string]string{strconv.FormatUint(d.height, 10): {}}
	for i := 1; i <= count; i++ {
		m := base
		m.ID = strconv.Itoa(i)
		m.Pool = fmt.Sprintf("0x%040x", 100+i)
		m.LogIndex = strconv.Itoa(i)
		d.peers.Markets = append(d.peers.Markets, m)
		d.peers.Claimable[strconv.FormatUint(d.height, 10)][m.ID] = "0"
	}
}

func rewardCandidateTransfer(id, owner string, height uint64) rewardHistoryTransfer {
	return rewardHistoryTransfer{NFTID: id, Owner: owner, Height: height, BlockHash: rewardHistoryHash(height)}
}

func TestRewardCandidatesWalletHistoryAndCursor(t *testing.T) {
	f := rewardHistoryChain(t)
	f.db.mu.Lock()
	rewardCandidateMarkets(f.db, 6)
	f.db.peers.Transfers = []rewardHistoryTransfer{
		rewardCandidateTransfer("1", f.chain.Alice, 10),
		rewardCandidateTransfer("2", f.chain.Bob, 10),
		rewardCandidateTransfer("2", f.chain.Alice, 11),
		rewardCandidateTransfer("3", f.chain.Alice, 10),
		rewardCandidateTransfer("3", f.chain.Bob, 12),
		rewardCandidateTransfer("4", f.chain.Alice, 10),
		rewardCandidateTransfer("4", f.chain.Bob, 12),
		rewardCandidateTransfer("5", f.chain.Bob, 10),
		rewardCandidateTransfer("6", f.chain.Bob, 10),
	}
	f.db.peers.Claims = []rewardHistoryClaim{{Pool: f.db.peers.Markets[2].Pool, Wallet: f.chain.Alice, Height: 11, BlockHash: rewardHistoryHash(11), Data: "0x" + fmt.Sprintf("%064x", 7)}}
	f.db.mu.Unlock()
	ctx := context.Background()
	b := Checkpoint{f.db.peers.Initial, rewardHistoryHash(f.db.peers.Initial)}
	candidates, err := f.journey.rewardCandidates(ctx, f.chain.Alice, b, "0")
	if err != nil || len(candidates) != 4 {
		t.Fatalf("wallet candidates=%+v: %v", candidates, err)
	}
	for i, candidate := range candidates {
		if candidate.NFTID != strconv.Itoa(i+1) {
			t.Fatalf("candidate order=%+v", candidates)
		}
	}
	market, err := f.journey.market(ctx, "1")
	if err != nil || candidates[0] != market {
		t.Fatalf("candidate source=%+v market=%+v: %v", candidates[0], market, err)
	}
	page := readRewardHistory(t, f.handler(f.chain.Alice), "", 100)
	if len(page.Data.Items) != 3 || page.NextCursor != nil {
		t.Fatalf("wallet rewards=%+v", page)
	}
	for i, item := range page.Data.Items {
		if item.RewardId != "creator:"+strconv.Itoa(i+1) {
			t.Fatalf("reward order=%+v", page)
		}
	}
	if page.Data.Items[2].ClaimedRaw != "7" || page.Data.Items[2].ClaimableRaw != "0" {
		t.Fatalf("former owner history=%+v", page.Data.Items[2])
	}
	first := readRewardHistory(t, f.handler(f.chain.Alice), "", 1)
	if len(first.Data.Items) != 1 || first.Data.Items[0].RewardId != "creator:1" || first.NextCursor == nil {
		t.Fatalf("first reward=%+v", first)
	}
	cursor, err := f.journey.decodeCursor(*first.NextCursor, f.chain.Alice, "")
	if err != nil || cursor.After != "1" || cursor.Block != b {
		t.Fatalf("reward cursor=%+v: %v", cursor, err)
	}
	f.advance(f.db.peers.Advanced)
	next := readRewardHistory(t, f.handler(f.chain.Alice), *first.NextCursor, 1)
	if len(next.Data.Items) != 1 || next.Data.Items[0].RewardId != "creator:2" || next.NextCursor == nil || next.Watermark.Height != first.Watermark.Height || next.Watermark.BlockHash != first.Watermark.BlockHash {
		t.Fatalf("next reward=%+v", next)
	}
	last := readRewardHistory(t, f.handler(f.chain.Alice), *next.NextCursor, 1)
	if len(last.Data.Items) != 1 || last.Data.Items[0].RewardId != "creator:3" || last.NextCursor != nil {
		t.Fatalf("last reward=%+v", last)
	}
}

func TestRewardCandidatesStatementCount(t *testing.T) {
	var counts []int
	for _, inventory := range []int{3, 300} {
		t.Run(strconv.Itoa(inventory), func(t *testing.T) {
			f := rewardHistoryChain(t)
			f.db.mu.Lock()
			rewardCandidateMarkets(f.db, inventory)
			last := strconv.Itoa(inventory)
			for _, m := range f.db.peers.Markets {
				owner := f.chain.Bob
				if m.ID == "1" || m.ID == last {
					owner = f.chain.Alice
				}
				f.db.peers.Transfers = append(f.db.peers.Transfers, rewardCandidateTransfer(m.ID, owner, 10))
			}
			f.db.queries, f.db.queryArgs = nil, nil
			f.db.mu.Unlock()
			page, err := f.journey.Rewards(context.Background(), Actor{Wallet: f.chain.Alice}, "", "", 100)
			if err != nil || len(page.Items) != 2 || page.Items[0].RewardId != "creator:1" || page.Items[1].RewardId != "creator:"+last || page.NextCursor != nil {
				t.Fatalf("reward page=%+v: %v", page, err)
			}
			f.db.mu.Lock()
			defer f.db.mu.Unlock()
			counts = append(counts, len(f.db.queries))
			owners, claims, candidates, source := 0, 0, 0, 0
			for i, sql := range f.db.queries {
				args := f.db.queryArgs[i]
				switch {
				case strings.HasPrefix(sql, "SELECT l.topics->>2"):
					owners++
					if args[3] != common.BigToHash(big.NewInt(1)).Hex() && args[3] != common.BigToHash(big.NewInt(int64(inventory))).Hex() {
						t.Fatalf("ownership read for unrelated token: %v", args)
					}
				case strings.HasPrefix(sql, "SELECT l.height,l.transaction_index,l.log_index,l.data,l.topics"):
					claims++
					if args[3] != addressTopic(f.db.peers.Markets[0].Pool) && args[3] != addressTopic(f.db.peers.Markets[inventory-1].Pool) {
						t.Fatalf("claim read for unrelated pool: %v", args)
					}
				case strings.HasPrefix(sql, "SELECT c.nft_id::text"):
					candidates++
				case strings.Contains(sql, "FROM indexer.indexer_ingest_watermarks_v1"), strings.Contains(sql, "FROM indexer.indexer_block_completeness_v1"):
					source++
				}
			}
			if owners != 2 || claims != 2 || candidates != 1 || source != 2 {
				t.Fatalf("reads: owners=%d claims=%d candidates=%d source=%d", owners, claims, candidates, source)
			}
		})
	}
	if len(counts) != 2 || counts[0] != counts[1] || counts[0] != 31 {
		t.Fatalf("statement counts=%v", counts)
	}
}

func TestRewardCandidatesPageBounds(t *testing.T) {
	f := rewardHistoryChain(t)
	f.db.mu.Lock()
	rewardCandidateMarkets(f.db, 101)
	for _, m := range f.db.peers.Markets {
		f.db.peers.Transfers = append(f.db.peers.Transfers, rewardCandidateTransfer(m.ID, f.chain.Alice, 10))
	}
	f.db.mu.Unlock()
	b := Checkpoint{f.db.peers.Initial, rewardHistoryHash(f.db.peers.Initial)}
	first, err := f.journey.rewardCandidates(context.Background(), f.chain.Alice, b, "0")
	if err != nil || len(first) != 100 {
		t.Fatalf("candidate page=%+v: %v", first, err)
	}
	for i, m := range first {
		if m.NFTID != strconv.Itoa(i+1) {
			t.Fatalf("candidate %d=%+v", i, m)
		}
	}
	next, err := f.journey.rewardCandidates(context.Background(), f.chain.Alice, b, first[99].NFTID)
	if err != nil || len(next) != 1 || next[0].NFTID != "101" {
		t.Fatalf("candidate continuation=%+v: %v", next, err)
	}
}

func TestRewardCandidatesCanonicalLogBindings(t *testing.T) {
	ctx := context.Background()
	database := coordinatortest.Bootstrap(t, os.Getenv("PNL_TEST_POSTGRES_DSN"), "pnl_runtime")
	admin, err := pgxpool.New(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	pool, err := pgxpool.New(ctx, database.Runtimes["pnl_runtime"].DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	ledger, err := NewLedger(pool, "pnl", "reward-candidates", nil)
	if err != nil {
		t.Fatal(err)
	}
	s := &JourneyService{ledger: ledger, sourceStream: rewardHistoryStream}
	maxID := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String()
	for i := 1; i <= 10; i++ {
		id := strconv.Itoa(i)
		if i == 10 {
			id = maxID
		}
		poolAddress := fmt.Sprintf("0x%040x", 200+i)
		if i == 2 {
			poolAddress = "0x" + strings.ToUpper(poolAddress[2:])
		}
		creationHeight := uint64(1)
		if i == 9 {
			creationHeight = 31
		}
		_, err = admin.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload)
 VALUES($1,$1,$2,$2,$2,$3::numeric,$4,$5,$6,0,0,1,'{}','{}')`, poolAddress, actorA, id, creationHeight, rewardHistoryHash(creationHeight), rewardHistoryHash(uint64(i)))
		if err != nil {
			t.Fatal(err)
		}
		height := uint64(10 + i)
		if i == 7 {
			height = 31
		}
		stream := rewardHistoryStream
		if i == 3 {
			stream = "other"
		}
		_, err = admin.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,1,$4,$5)`, stream, height, rewardHistoryHash(height), i != 4, i != 5)
		if err != nil {
			t.Fatal(err)
		}
		n, _ := new(big.Int).SetString(id, 10)
		topics := []string{nft.Events["Transfer"].ID.Hex(), addressTopic(actorB), addressTopic(actorA), common.BigToHash(n).Hex()}
		emitter := NFT
		if i == 2 || i == 4 || i == 6 {
			emitter = Accumulator
			topics = []string{accumulator.Events["FeesClaimed"].ID.Hex(), addressTopic(poolAddress), addressTopic(actorA)}
		}
		if i == 8 {
			topics[3] = "invalid"
		}
		blockHash := rewardHistoryHash(height)
		if i == 6 {
			blockHash = rewardHistoryHash(999)
		}
		raw, err := json.Marshal(topics)
		if err != nil {
			t.Fatal(err)
		}
		_, err = admin.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,0,$2,0,$3,$4,$5,$6)`, rewardHistoryHash(uint64(100+i)), height, blockHash, emitter, raw, make([]byte, 32))
		if err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.rewardCandidates(ctx, actorA, Checkpoint{30, rewardHistoryHash(30)}, "0")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, m := range page {
		ids = append(ids, m.NFTID)
	}
	if !reflect.DeepEqual(ids, []string{"1", "2", maxID}) {
		t.Fatalf("canonical candidates=%v", ids)
	}
	if _, err = pool.Exec(ctx, "SELECT 1 FROM indexer.indexer_raw_logs"); err == nil {
		t.Fatal("runtime read private raw logs")
	}
	remaining, err := s.rewardCandidates(ctx, actorA, Checkpoint{30, rewardHistoryHash(30)}, "2")
	if err != nil || len(remaining) != 1 || remaining[0].NFTID != maxID {
		t.Fatalf("numeric continuation=%+v: %v", remaining, err)
	}
	empty, err := s.rewardCandidates(ctx, actorB, Checkpoint{30, rewardHistoryHash(30)}, "0")
	if err != nil || len(empty) != 0 {
		t.Fatalf("unrelated wallet candidates=%+v: %v", empty, err)
	}
}
