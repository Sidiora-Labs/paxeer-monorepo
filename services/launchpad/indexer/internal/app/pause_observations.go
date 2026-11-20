package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	pauseBaseCycle       = 60 * time.Second
	pauseBaseTimeout     = 90 * time.Second
	pauseReadTimeout     = 2 * time.Second
	pauseReadConcurrency = 32
	pauseRefreshTimeout  = 30 * time.Second
)

var (
	pauseTopic          = crypto.Keccak256Hash([]byte("PauseToggled(bool)"))
	pauseUpgradeTopic   = crypto.Keccak256Hash([]byte("Upgraded(address)"))
	pauseSelector       = crypto.Keccak256([]byte("paused()"))[:4]
	errPauseUnavailable = errors.New("indexer: pause observation unavailable")
)

// These queries use the canonical stream's durable completeness boundary.
const pauseHeadSQL = `SELECT w.typed_complete_height,b.block_hash,b.completed_at FROM indexer.indexer_ingest_watermarks w JOIN indexer.indexer_block_completeness b ON b.chain_id=w.chain_id AND b.stream_id=w.stream_id AND b.height=w.typed_complete_height WHERE w.chain_id=125 AND w.stream_id=$1 AND w.raw_complete_height=w.typed_complete_height AND b.fetch_complete AND b.required_decode_complete`
const pauseCutoffSQL = `SELECT w.typed_complete_height,b.block_hash,b.completed_at,b.fetch_complete AND b.required_decode_complete,w.raw_complete_height=w.typed_complete_height FROM indexer.indexer_ingest_watermarks w LEFT JOIN indexer.indexer_block_completeness b ON b.chain_id=w.chain_id AND b.stream_id=w.stream_id AND b.height=$2 WHERE w.chain_id=125 AND w.stream_id=$1`
const pausePoolsSQL = `SELECT pool,height,transaction_index,log_index FROM indexer.indexer_canonical_creations WHERE height<=$1 ORDER BY pool`
const pauseBlocksSQL = `SELECT height,block_hash FROM indexer.indexer_block_completeness WHERE chain_id=125 AND stream_id=$1 AND height BETWEEN $2 AND $3 AND fetch_complete AND required_decode_complete ORDER BY height`
const pauseLogsSQL = `SELECT emitter,height,transaction_index,log_index,block_hash,topics,data FROM indexer.indexer_raw_logs WHERE chain_id=125 AND height BETWEEN $1 AND $2 AND topics->>0 IN ($3,$4) ORDER BY height,transaction_index,log_index`

type pausePool struct {
	address                  string
	height, transaction, log int64
}
type pauseBlock struct {
	height    int64
	hash      string
	completed time.Time
}
type pauseBase struct {
	block pauseBlock
	pools map[string]bool
	built time.Time
}
type pauseSnapshot struct {
	base   *pauseBase
	head   pauseBlock
	pools  []pausePool
	blocks map[int64]string
	events []types.Log
}
type pauseBaseClient interface {
	CallContractAtHash(context.Context, ethereum.CallMsg, common.Hash) ([]byte, error)
}

type pauseObservations struct {
	pool             *pgxpool.Pool
	rpc              pauseBaseClient
	stream           string
	epochs           []source.AddressEpoch
	discoveredEpochs func() []receipt.RPCCodeEpoch
	mu               sync.Mutex
	snapshot         atomic.Pointer[pauseSnapshot]
}

type pausePoolEvidence struct {
	Paused        bool   `json:"paused"`
	PauseEvidence string `json:"pauseEvidence"`
}
type pauseRankingEvidence struct {
	Revision     string                       `json:"revision"`
	CutoffHeight uint64                       `json:"cutoffHeight"`
	CutoffTime   int64                        `json:"cutoffTime"`
	BlockHash    string                       `json:"blockHash"`
	Pools        map[string]pausePoolEvidence `json:"pools"`
}

func pauseHashValid(s string) bool {
	if len(s) != 66 || !strings.HasPrefix(s, "0x") {
		return false
	}
	raw, err := hex.DecodeString(s[2:])
	return err == nil && len(raw) == 32 && hex.EncodeToString(raw) == s[2:] && common.BytesToHash(raw) != (common.Hash{})
}

func pauseBool(raw []byte) (bool, error) {
	if len(raw) != 32 || !bytes.Equal(raw[:31], make([]byte, 31)) || raw[31] > 1 {
		return false, errPauseUnavailable
	}
	return raw[31] == 1, nil
}

func (p *pauseObservations) head(ctx context.Context, tx pgx.Tx) (pauseBlock, error) {
	var b pauseBlock
	err := tx.QueryRow(ctx, pauseHeadSQL, p.stream).Scan(&b.height, &b.hash, &b.completed)
	if err != nil {
		return b, err
	}
	if b.height < 1 || !pauseHashValid(b.hash) {
		return b, errPauseUnavailable
	}
	return b, nil
}

func readPausePools(ctx context.Context, tx pgx.Tx, height int64) ([]pausePool, error) {
	rows, err := tx.Query(ctx, pausePoolsSQL, height)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pools := []pausePool{}
	for rows.Next() {
		var pool pausePool
		if err = rows.Scan(&pool.address, &pool.height, &pool.transaction, &pool.log); err != nil {
			return nil, err
		}
		if !common.IsHexAddress(pool.address) || pool.address != strings.ToLower(pool.address) || common.HexToAddress(pool.address) == (common.Address{}) || pool.height < 0 || pool.transaction < 0 || pool.log < 0 {
			return nil, errPauseUnavailable
		}
		pools = append(pools, pool)
	}
	sort.Slice(pools, func(i, j int) bool { return pools[i].address < pools[j].address })
	return pools, rows.Err()
}

func (p *pauseObservations) buildBase(ctx context.Context, block pauseBlock, pools []pausePool) *pauseBase {
	base := &pauseBase{block: block, pools: make(map[string]bool), built: time.Now()}
	jobs := make(chan pausePool)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for range pauseReadConcurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pool := range jobs {
				bounded, cancel := context.WithTimeout(ctx, pauseReadTimeout)
				// EIP-1898 binds this read to the complete block, never to latest.
				data, err := p.rpc.CallContractAtHash(bounded, ethereum.CallMsg{To: pauseAddressPointer(pool.address), Data: pauseSelector, Gas: 150000}, common.HexToHash(block.hash))
				cancel()
				if err != nil {
					continue
				}
				value, err := pauseBool(data)
				if err != nil {
					continue
				}
				mu.Lock()
				base.pools[pool.address] = value
				mu.Unlock()
			}
		}()
	}
send:
	for _, pool := range pools {
		select {
		case jobs <- pool:
		case <-ctx.Done():
			break send
		}
	}
	close(jobs)
	wg.Wait()
	return base
}

func pauseAddressPointer(value string) *common.Address {
	a := common.HexToAddress(value)
	return &a
}

// rawCoverage requires the complete pool and beacon emitter ranges. A stored
// event alone cannot establish coverage of an address with no events.
func (p *pauseObservations) rawCoverage(pools []pausePool, discovered []receipt.RPCCodeEpoch, from, to int64) bool {
	covered := make(map[string]bool)
	beacon := false
	for _, epoch := range p.epochs {
		if epoch.ActivationHeight > uint64(from) || epoch.RetirementHeight != nil && *epoch.RetirementHeight <= uint64(to) {
			continue
		}
		if epoch.Role == "PoolBeacon" {
			beacon = true
		}
		if epoch.Role == "SidioraPool" {
			covered[strings.ToLower(epoch.Address)] = true
		}
	}
	if !beacon {
		return false
	}
	for _, pool := range pools {
		if covered[pool.address] {
			continue
		}
		for _, epoch := range discovered {
			d := epoch.Deployment
			if d.Role != "SidioraPool" || strings.ToLower(d.Address.Hex()) != pool.address || d.Until != nil && d.Until.Block <= uint64(to) {
				continue
			}
			if d.From.Block < uint64(pool.height) || d.From.Block == uint64(pool.height) && (uint64(d.From.Transaction) < uint64(pool.transaction) || uint64(d.From.Transaction) == uint64(pool.transaction) && uint64(d.From.Log) <= uint64(pool.log)) {
				covered[pool.address] = true
				break
			}
		}
		if !covered[pool.address] {
			return false
		}
	}
	return true
}

func (p *pauseObservations) readEvents(ctx context.Context, tx pgx.Tx, from, to int64) ([]types.Log, error) {
	if from > to {
		return nil, nil
	}
	rows, err := tx.Query(ctx, pauseLogsSQL, from, to, strings.ToLower(pauseTopic.Hex()), strings.ToLower(pauseUpgradeTopic.Hex()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := []types.Log{}
	for rows.Next() {
		var address, hash string
		var height, transaction, index int64
		var topics []string
		var raw json.RawMessage
		var data []byte
		if err = rows.Scan(&address, &height, &transaction, &index, &hash, &raw, &data); err != nil {
			return nil, err
		}
		if json.Unmarshal(raw, &topics) != nil || height < 0 || transaction < 0 || index < 0 || !common.IsHexAddress(address) || !pauseHashValid(hash) {
			return nil, errPauseUnavailable
		}
		event := types.Log{Address: common.HexToAddress(address), BlockNumber: uint64(height), TxIndex: uint(transaction), Index: uint(index), BlockHash: common.HexToHash(hash), Data: data}
		for _, topic := range topics {
			if !pauseHashValid(topic) {
				return nil, errPauseUnavailable
			}
			event.Topics = append(event.Topics, common.HexToHash(topic))
		}
		logs = append(logs, event)
	}
	return logs, rows.Err()
}

func (p *pauseObservations) refresh(parent context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rpc == nil || p.pool == nil || p.stream == "" {
		return errPauseUnavailable
	}
	ctx, cancel := context.WithTimeout(parent, pauseBaseTimeout+pauseRefreshTimeout)
	defer cancel()
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	head, err := p.head(ctx, tx)
	if err != nil {
		return err
	}
	pools, err := readPausePools(ctx, tx, head.height)
	if err != nil {
		return err
	}
	previous := p.snapshot.Load()
	var base *pauseBase
	if previous != nil {
		base = previous.base
	}
	var discovered []receipt.RPCCodeEpoch
	if p.discoveredEpochs != nil {
		discovered = p.discoveredEpochs()
	}
	if base == nil || time.Since(base.built) >= pauseBaseCycle || !p.rawCoverage(pools, discovered, base.block.height+1, head.height) {
		bounded, stop := context.WithTimeout(ctx, pauseBaseTimeout)
		base = p.buildBase(bounded, head, pools)
		stop()
		if parent.Err() != nil {
			return parent.Err()
		}
	}
	if head.height < base.block.height {
		return errPauseUnavailable
	}
	bounded, stop := context.WithTimeout(ctx, pauseRefreshTimeout)
	defer stop()
	blocks := make(map[int64]string)
	rows, err := tx.Query(bounded, pauseBlocksSQL, p.stream, base.block.height, head.height)
	if err != nil {
		return err
	}
	next := base.block.height
	for rows.Next() {
		var height int64
		var hash string
		if err = rows.Scan(&height, &hash); err != nil {
			rows.Close()
			return err
		}
		if height != next || !pauseHashValid(hash) {
			rows.Close()
			return errPauseUnavailable
		}
		blocks[height] = hash
		next++
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if next != head.height+1 || blocks[base.block.height] != base.block.hash {
		return errPauseUnavailable
	}
	events, err := p.readEvents(bounded, tx, base.block.height+1, head.height)
	if err != nil {
		return err
	}
	for _, event := range events {
		if event.BlockNumber > math.MaxInt64 || event.BlockNumber <= uint64(base.block.height) || event.BlockNumber > uint64(head.height) || event.Removed || blocks[int64(event.BlockNumber)] != strings.ToLower(event.BlockHash.Hex()) || len(event.Topics) == 0 {
			return errPauseUnavailable
		}
		switch event.Topics[0] {
		case pauseTopic:
			if len(event.Topics) != 1 {
				return errPauseUnavailable
			}
			if _, err = pauseBool(event.Data); err != nil {
				return err
			}
		case pauseUpgradeTopic:
			if len(event.Topics) != 2 || len(event.Data) != 0 {
				return errPauseUnavailable
			}
		default:
			return errPauseUnavailable
		}
	}
	sort.Slice(events, func(i, j int) bool {
		a, b := events[i], events[j]
		if a.BlockNumber != b.BlockNumber {
			return a.BlockNumber < b.BlockNumber
		}
		if a.TxIndex != b.TxIndex {
			return a.TxIndex < b.TxIndex
		}
		return a.Index < b.Index
	})
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	p.snapshot.Store(&pauseSnapshot{base: base, head: head, pools: pools, blocks: blocks, events: events})
	return nil
}

func (s *pauseSnapshot) evidence(height, cutoff int64, hash string) (pauseRankingEvidence, error) {
	out := pauseRankingEvidence{CutoffHeight: uint64(height), CutoffTime: cutoff, BlockHash: hash, Pools: make(map[string]pausePoolEvidence)}
	if s == nil || s.base == nil || height < s.base.block.height || height > s.head.height || s.blocks[height] != hash {
		return out, errPauseUnavailable
	}
	states := make(map[string]bool, len(s.pools))
	creations := make(map[string]pausePool, len(s.pools))
	for _, pool := range s.pools {
		if pool.height > height {
			continue
		}
		creations[pool.address] = pool
		if pool.height > s.base.block.height {
			states[pool.address] = false
		} else if value, ok := s.base.pools[pool.address]; ok {
			states[pool.address] = value
		}
	}
	for _, event := range s.events {
		if event.BlockNumber > uint64(height) {
			break
		}
		if event.Topics[0] == pauseUpgradeTopic {
			return out, errPauseUnavailable
		}
		pool := strings.ToLower(event.Address.Hex())
		if _, known := states[pool]; !known {
			continue
		}
		creation := creations[pool]
		// NFT receiver callbacks can pause before the factory announces creation.
		if event.BlockNumber < uint64(creation.height) || event.BlockNumber == uint64(creation.height) && event.TxIndex < uint(creation.transaction) {
			continue
		}
		value, err := pauseBool(event.Data)
		if err != nil {
			return out, err
		}
		states[pool] = value
	}
	revision := sha256.New()
	fmt.Fprintf(revision, "launchpad.pause.revision.v1\n125\n%d\n%d\n%s\n", height, cutoff, hash)
	// The inventory query supplies the canonical lexicographic order.
	for _, pool := range s.pools {
		state, ok := states[pool.address]
		if !ok {
			continue
		}
		digest := sha256.Sum256([]byte(fmt.Sprintf("launchpad.pause.v1\n125\n%s\n%s\n%t\n", pool.address, hash, state)))
		out.Pools[pool.address] = pausePoolEvidence{Paused: state, PauseEvidence: hex.EncodeToString(digest[:])}
		fmt.Fprintf(revision, "%s\n%t\n", pool.address, state)
	}
	out.Revision = hex.EncodeToString(revision.Sum(nil))
	return out, nil
}

func pauseQuery(q url.Values) (int64, int64, string, error) {
	if len(q) != 3 || len(q["cutoffHeight"]) != 1 || len(q["cutoffTime"]) != 1 || len(q["blockHash"]) != 1 {
		return 0, 0, "", errPauseUnavailable
	}
	height, err := strconv.ParseInt(q.Get("cutoffHeight"), 10, 64)
	if err != nil || height < 1 || strconv.FormatInt(height, 10) != q.Get("cutoffHeight") {
		return 0, 0, "", errPauseUnavailable
	}
	cutoff, err := strconv.ParseInt(q.Get("cutoffTime"), 10, 64)
	hash := q.Get("blockHash")
	if err != nil || cutoff < 86400 || strconv.FormatInt(cutoff, 10) != q.Get("cutoffTime") || !pauseHashValid(hash) {
		return 0, 0, "", errPauseUnavailable
	}
	return height, cutoff, hash, nil
}

func (p *pauseObservations) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, "invalid cutoff", http.StatusBadRequest)
		return
	}
	height, cutoff, hash, err := pauseQuery(q)
	if err != nil {
		http.Error(w, "invalid cutoff", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
	defer cancel()
	if p == nil || p.pool == nil {
		http.Error(w, "pause observation unavailable", http.StatusServiceUnavailable)
		return
	}
	var head int64
	var blockHash *string
	var completed *time.Time
	var complete *bool
	var coherent bool
	err = p.pool.QueryRow(ctx, pauseCutoffSQL, p.stream, height).Scan(&head, &blockHash, &completed, &complete, &coherent)
	if err != nil {
		http.Error(w, "pause observation unavailable", http.StatusServiceUnavailable)
		return
	}
	if height > head || blockHash == nil || complete == nil || !*complete || *blockHash != hash || completed == nil || completed.Unix() > cutoff {
		http.Error(w, "cutoff is not a complete canonical block", http.StatusConflict)
		return
	}
	if !coherent {
		http.Error(w, "pause observation unavailable", http.StatusServiceUnavailable)
		return
	}
	evidence, err := p.snapshot.Load().evidence(height, cutoff, hash)
	if err != nil {
		http.Error(w, "pause observation unavailable", http.StatusServiceUnavailable)
		return
	}
	wire, err := json.Marshal(evidence)
	if err != nil || ctx.Err() != nil {
		http.Error(w, "pause observation unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(wire)
}
