package l18

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rewardMeasuredNode struct {
	client              *rpc.Client
	mu                  sync.Mutex
	delay               time.Duration
	calls, active, peak int
	batches             []int
	reads               []rewardMeasuredRead
	hangMethod          string
	hangEntered         time.Time
	failID              string
	failure             error
	block               string
}

type rewardMeasuredRead struct {
	method string
	batch  bool
}

func (n *rewardMeasuredNode) enter(method string, batch bool) func() {
	n.mu.Lock()
	n.calls++
	n.reads = append(n.reads, rewardMeasuredRead{method, batch})
	n.active++
	n.peak = max(n.peak, n.active)
	n.mu.Unlock()
	return func() {
		n.mu.Lock()
		n.active--
		n.mu.Unlock()
	}
}

func (n *rewardMeasuredNode) wait(ctx context.Context, method string) error {
	n.mu.Lock()
	delay, hang := n.delay, n.hangMethod == method
	if hang && n.hangEntered.IsZero() {
		n.hangEntered = time.Now()
	}
	n.mu.Unlock()
	if hang {
		<-ctx.Done()
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (n *rewardMeasuredNode) hangEnteredAt() time.Time {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.hangEntered
}

func (n *rewardMeasuredNode) check(method string, args []any) error {
	if method != "eth_call" {
		return nil
	}
	bound := args[1].(map[string]any)
	if bound[fieldBlockHash] != n.block || bound[fieldRequireCanonical] != true {
		return fmt.Errorf("contract read requires the page checkpoint")
	}
	call := args[0].(map[string]any)
	data, err := hexutil.Decode(call[fieldData].(string))
	if err != nil {
		return err
	}
	if call["to"] == NFT && n.failID != "" {
		id, _ := amount(n.failID)
		if len(data) == 36 && new(big.Int).SetBytes(data[4:]).Cmp(id) == 0 {
			return n.failure
		}
	}
	return nil
}

func (n *rewardMeasuredNode) CallContext(ctx context.Context, out any, method string, args ...any) error {
	leave := n.enter(method, false)
	defer leave()
	if err := n.wait(ctx, method); err != nil {
		return err
	}
	if err := n.check(method, args); err != nil {
		return err
	}
	return n.client.CallContext(ctx, out, method, args...)
}

type rewardMeasuredBatch struct{ *rewardMeasuredNode }

func (n *rewardMeasuredBatch) BatchCallContext(ctx context.Context, batch []rpc.BatchElem) error {
	leave := n.enter(batch[0].Method, true)
	defer leave()
	n.mu.Lock()
	n.batches = append(n.batches, len(batch))
	n.mu.Unlock()
	if err := n.wait(ctx, batch[0].Method); err != nil {
		return err
	}
	if err := n.client.BatchCallContext(ctx, batch); err != nil {
		return err
	}
	for i := range batch {
		if err := n.check(batch[i].Method, batch[i].Args); err != nil {
			batch[i].Error = err
		}
	}
	return nil
}

type boundedRewardsFixture struct {
	service *JourneyService
	node    *rewardMeasuredNode
	owner   string
	block   Checkpoint
	hashes  map[string]string
}

func rewardsBoundedFixture(t *testing.T) boundedRewardsFixture {
	t.Helper()
	ctx := context.Background()
	client, chain := internaltest.NewAnvilFixture(t, ctx, fixtureRoot())
	mint := parseABI(`[{"type":"function","name":"mint","inputs":[{"type":"address"},{"type":"address"},{"type":"uint8"}],"outputs":[]}]`)
	writer, err := newFixtureRPC(chain.RPC)
	if err != nil {
		t.Fatal(err)
	}
	for i := 3; i <= 12; i++ {
		mustSend(t, writer, chain.Admin, NFT, mint, "mint", common.HexToAddress(chain.Alice), common.BigToAddress(big.NewInt(int64(100+i))), uint8(0))
	}
	latest := NewCreator(client, time.Now)
	block, err := latest.checkpoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bound := map[string]any{fieldBlockHash: block.Hash, fieldRequireCanonical: true}
	hashes := map[string]string{}
	for _, target := range []string{NFT, Accumulator, FeesRouter, USDL} {
		var code, slot hexutil.Bytes
		if err = client.CallContext(ctx, &code, "eth_getCode", target, bound); err != nil {
			t.Fatal(err)
		}
		hashes[target] = strings.ToLower(crypto.Keccak256Hash(code).Hex())
		if err = client.CallContext(ctx, &slot, "eth_getStorageAt", target, "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc", bound); err != nil {
			t.Fatal(err)
		}
		if implementation := common.BytesToAddress(slot); implementation != (common.Address{}) {
			if err = client.CallContext(ctx, &code, "eth_getCode", implementation.Hex(), bound); err != nil {
				t.Fatal(err)
			}
			hashes[strings.ToLower(implementation.Hex())] = strings.ToLower(crypto.Keccak256Hash(code).Hex())
		}
	}
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
	var logs []struct {
		Topics           []string
		Data             hexutil.Bytes
		BlockNumber      hexutil.Uint64
		BlockHash        string
		TransactionHash  string
		TransactionIndex hexutil.Uint64
		LogIndex         hexutil.Uint64
	}
	if err = client.CallContext(ctx, &logs, "eth_getLogs", map[string]any{"address": NFT, "fromBlock": "0x0", "toBlock": hexutil.EncodeUint64(block.Number), "topics": []any{nft.Events["Transfer"].ID.Hex(), addressTopic("0x0000000000000000000000000000000000000000")}}); err != nil {
		t.Fatal(err)
	}
	if len(logs) != 12 {
		t.Fatalf("mint logs=%d", len(logs))
	}
	for _, log := range logs {
		id := new(big.Int).SetBytes(common.HexToHash(log.Topics[3]).Bytes()).String()
		value, err := latest.read(ctx, block, NFT, nft, "getPoolAddress", new(big.Int).SetBytes(common.HexToHash(log.Topics[3]).Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		poolAddress := strings.ToLower(value.(common.Address).Hex())
		topics, err := json.Marshal(log.Topics)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = admin.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$1,$2,$2,$2,$3::numeric,$4,$5,$6,$7,$8,1,'{}','{}')`, poolAddress, chain.Alice, id, uint64(log.BlockNumber), log.BlockHash, log.TransactionHash, uint64(log.TransactionIndex), uint64(log.LogIndex)); err != nil {
			t.Fatal(err)
		}
		if _, err = admin.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8)`, log.TransactionHash, uint64(log.LogIndex), uint64(log.BlockNumber), uint64(log.TransactionIndex), log.BlockHash, NFT, topics, append([]byte{}, log.Data...)); err != nil {
			t.Fatal(err)
		}
		if _, err = admin.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,'bounded_rewards',$1,$2,1,true,true) ON CONFLICT DO NOTHING`, uint64(log.BlockNumber), log.BlockHash); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = admin.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,'bounded_rewards',$1,$1)`, block.Number); err != nil {
		t.Fatal(err)
	}
	node := &rewardMeasuredNode{client: client, block: block.Hash}
	creator, err := NewIndexedCreator(ctx, &rewardMeasuredBatch{node}, pool, "bounded_rewards", time.Now, ManifestDigest, hashes)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := queue.NewEpochGuard("recovery", coordinatortest.Epoch, coordinatortest.Region)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := NewRuntimeLedger(pool, "pnl", "bounded-rewards", nil, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewJourneyService(ledger, creator, []byte(strings.Repeat("r", 32)), time.Now, strings.Repeat("a", 64), strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	service, err = service.WithSourceStream("bounded_rewards")
	if err != nil {
		t.Fatal(err)
	}
	node.calls, node.batches, node.peak = 0, nil, 0
	node.reads = nil
	return boundedRewardsFixture{service, node, chain.Alice, block, hashes}
}

func (f boundedRewardsFixture) http(timeout time.Duration) HTTP {
	return HTTP{Service: f.service, Timeout: timeout, Authenticate: func(*http.Request) (Actor, error) { return Actor{Wallet: f.owner}, nil }}
}

func boundedRewardsResponse(t *testing.T, f boundedRewardsFixture) iface.RewardsResponse {
	t.Helper()
	response := httptest.NewRecorder()
	start := time.Now()
	f.http(0).Rewards(response, httptest.NewRequest(http.MethodGet, "/v2/me/rewards", nil))
	if response.Code != http.StatusOK || time.Since(start) >= 5*time.Second {
		t.Fatalf("rewards status=%d elapsed=%s body=%s", response.Code, time.Since(start), response.Body.String())
	}
	var page iface.RewardsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if err := iface.ValidateJSON("RewardsResponse", response.Body.Bytes()); err != nil {
		t.Fatal(err)
	}
	if len(page.Data.Items) != 12 {
		t.Fatalf("rewards=%+v", page)
	}
	return page
}

func TestRewardsBoundedPage(t *testing.T) {
	f := rewardsBoundedFixture(t)
	expected := make([]iface.Reward, 12)
	pinned := *f.service.creator
	pinned.checked = &f.block
	for i := range expected {
		id := strconv.Itoa(i + 1)
		m, err := f.service.market(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		p, err := pinned.Preview(context.Background(), id, m.Creator)
		if err != nil {
			t.Fatal(err)
		}
		expected[i] = iface.Reward{AssetAddress: USDL, ClaimableRaw: iface.UInt256(p.ClaimableRaw), ClaimedRaw: "0", EarnedRaw: iface.AggregateUInt(p.ClaimableRaw), ProgramId: "creator_fees", RewardId: "creator:" + id, Wallet: f.owner, SourcePosition: &iface.ChainPosition{ChainId: "125", Height: iface.UInt256(strconv.FormatUint(m.Height, 10)), BlockHash: m.BlockHash, TransactionHash: m.TxHash, TransactionIndex: iface.UInt256(m.TxIndex), LogIndex: iface.UInt256(m.LogIndex)}}
	}
	f.node.calls, f.node.batches, f.node.peak, f.node.delay = 0, nil, 0, 300*time.Millisecond
	page := boundedRewardsResponse(t, f)
	if !reflect.DeepEqual(page.Data.Items, expected) || f.node.calls != 6 || !reflect.DeepEqual(f.node.batches, []int{8, 3, 36, 24}) {
		t.Fatalf("page=%+v expected=%+v calls=%d batches=%v", page.Data.Items, expected, f.node.calls, f.node.batches)
	}
	f.service.creator.rpc = f.node
	f.node.calls, f.node.peak, f.node.batches = 0, 0, nil
	f.service.creator.attest = func(ctx context.Context, b Checkpoint) error {
		return verifyRuntime(ctx, f.node, b, f.hashes)
	}
	single := boundedRewardsResponse(t, f)
	if !reflect.DeepEqual(single.Data.Items, expected) || f.node.peak != 8 || f.node.active != 0 || len(f.node.batches) != 0 {
		t.Fatalf("single-call page=%+v peak=%d active=%d", single, f.node.peak, f.node.active)
	}
}

func TestRewardsBoundedTimeoutAndReadError(t *testing.T) {
	f := rewardsBoundedFixture(t)
	var messages bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&messages, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	f.node.hangMethod = "eth_call"
	for _, route := range []string{"rewards", "prepare"} {
		response := httptest.NewRecorder()
		start := time.Now()
		h := f.http(time.Second)
		if route == "rewards" {
			h.Rewards(response, httptest.NewRequest(http.MethodGet, "/v2/me/rewards", nil))
		} else {
			request := httptest.NewRequest(http.MethodPost, "/v2/me/rewards:prepare-claim", strings.NewReader(fmt.Sprintf(`{"rewardIds":["creator:1"],"recipient":%q}`, f.owner)))
			request.Header.Set("Idempotency-Key", uuid.NewString())
			h.PrepareClaim(response, request)
		}
		elapsed := time.Since(start)
		if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "5" || !strings.Contains(response.Body.String(), `"retryable":true`) || strings.Contains(response.Body.String(), `"items"`) || elapsed < time.Second || elapsed > 1500*time.Millisecond {
			t.Fatalf("%s status=%d elapsed=%s body=%s", route, response.Code, elapsed, response.Body.String())
		}
	}
	if !strings.Contains(messages.String(), `"markets":12`) || !strings.Contains(messages.String(), `"group":"nft"`) || !strings.Contains(messages.String(), `"markets":1,`) {
		t.Fatalf("timeout diagnostics=%s", messages.String())
	}
	f.node.hangMethod = ""
	f.node.failID, f.node.failure = "7", errors.New("market read unavailable")
	_, err := f.service.Rewards(context.Background(), Actor{Wallet: f.owner}, "", "", 50)
	if !errors.Is(err, f.node.failure) {
		t.Fatalf("market error=%v", err)
	}
	response := httptest.NewRecorder()
	f.http(0).Rewards(response, httptest.NewRequest(http.MethodGet, "/v2/me/rewards", nil))
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), `"items"`) {
		t.Fatalf("market read HTTP %d: %s", response.Code, response.Body.String())
	}
}

func TestRewardsBoundedConfiguration(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":        "postgres://localhost/rewards",
		"REDIS_URL":           "redis://localhost",
		"REDIS_BULL_URL":      "redis://localhost",
		"RPC_URL":             "http://localhost",
		"WEBHOOK_HMAC_SECRET": strings.Repeat("s", 32),
	}
	for name, role := range map[string]string{
		"EVENT_EMITTER_ADDRESS": "EventEmitter", "POOL_REGISTRY_ADDRESS": "PoolRegistry",
		"ROUTER_ADDRESS": "Router", "FACTORY_ADDRESS": "SidioraFactory", "QUOTER_ADDRESS": "Quoter",
		"PROTOCOL_CONFIG_ADDRESS": "ProtocolConfig", "FEE_ACCUMULATOR_ADDRESS": "FeeAccumulator",
		"SIDIORA_NFT_ADDRESS": "SidioraNFT", "FEES_ROUTER_ADDRESS": "FeesRouter", "POOL_BEACON_ADDRESS": "PoolBeacon",
	} {
		values[name] = deployment.Current().Roles[role].Address
	}
	for name, value := range values {
		t.Setenv(name, value)
	}
	t.Setenv("PNL_REWARDS_TIMEOUT_SECONDS", "")
	cfg, err := config.Load()
	if err != nil || cfg.RewardsTimeoutSeconds != 5 {
		t.Fatalf("default timeout=%d error=%v", cfg.RewardsTimeoutSeconds, err)
	}
	for _, seconds := range []int{-1, 0, 1, 5, 15, 16} {
		t.Setenv("PNL_REWARDS_TIMEOUT_SECONDS", strconv.Itoa(seconds))
		cfg, err := config.Load()
		valid := seconds >= 1 && seconds <= 15
		if (err == nil) != valid {
			t.Fatalf("bound=%d error=%v", seconds, err)
		}
		if !valid {
			if !strings.Contains(err.Error(), "PNL_REWARDS_TIMEOUT_SECONDS") {
				t.Fatal(err)
			}
			continue
		}
		timeout, err := cfg.RewardsTimeout()
		if err != nil || timeout != time.Duration(seconds)*time.Second {
			t.Fatalf("bound=%d timeout=%s error=%v", seconds, timeout, err)
		}
	}
}

func TestRewardsBoundedRoundTrip(t *testing.T) {
	f := rewardsBoundedFixture(t)
	if !f.node.hangEnteredAt().IsZero() {
		t.Fatal("hanging read entered before request")
	}
	f.node.hangMethod = "eth_call"
	response := httptest.NewRecorder()
	start := time.Now()
	f.http(0).Rewards(response, httptest.NewRequest(http.MethodGet, "/v2/me/rewards", nil))
	end := time.Now()
	entered := f.node.hangEnteredAt()
	elapsed := end.Sub(start)
	var before, after time.Duration
	if !entered.IsZero() {
		before, after = entered.Sub(start), end.Sub(entered)
	}
	if entered.IsZero() || response.Code != http.StatusServiceUnavailable || elapsed < 2*time.Second || after > 2500*time.Millisecond || elapsed >= 5*time.Second {
		t.Fatalf("round trip status=%d elapsed=%s before=%s after=%s hangingRead=%t", response.Code, elapsed, before, after, !entered.IsZero())
	}
	t.Logf("round trip status=%d elapsed=%s before=%s after=%s reads=%+v", response.Code, elapsed, before, after, f.node.reads)
}

func TestRewardsBoundedRoundTripAfterSlowReads(t *testing.T) {
	f := rewardsBoundedFixture(t)
	if !f.node.hangEnteredAt().IsZero() {
		t.Fatal("hanging read entered before request")
	}
	f.node.delay = 200 * time.Millisecond
	f.node.hangMethod = "eth_call"
	response := httptest.NewRecorder()
	start := time.Now()
	f.http(0).Rewards(response, httptest.NewRequest(http.MethodGet, "/v2/me/rewards", nil))
	end := time.Now()
	entered := f.node.hangEnteredAt()
	elapsed := end.Sub(start)
	var before, after time.Duration
	if !entered.IsZero() {
		before, after = entered.Sub(start), end.Sub(entered)
	}
	if entered.IsZero() || response.Code != http.StatusServiceUnavailable || elapsed < 2*time.Second || before < 600*time.Millisecond || after > 2500*time.Millisecond || elapsed >= 5*time.Second {
		t.Fatalf("round trip status=%d elapsed=%s before=%s after=%s hangingRead=%t", response.Code, elapsed, before, after, !entered.IsZero())
	}
	t.Logf("round trip status=%d elapsed=%s before=%s after=%s reads=%+v", response.Code, elapsed, before, after, f.node.reads)
}

func TestRewardsBoundedBatchChunks(t *testing.T) {
	f := rewardsBoundedFixture(t)
	market, err := f.service.market(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	markets := make([]marketSource, 100)
	for i := range markets {
		markets[i] = market
	}
	expected, err := f.service.creator.Preview(context.Background(), "1", market.Creator)
	if err != nil {
		t.Fatal(err)
	}
	f.node.calls, f.node.batches = 0, nil
	previews, err := f.service.creator.previewPage(context.Background(), f.block, markets)
	if err != nil || len(previews) != 100 || !reflect.DeepEqual(f.node.batches, []int{100, 100, 100, 100, 100}) {
		t.Fatalf("page previews=%d batches=%v error=%v", len(previews), f.node.batches, err)
	}
	for _, preview := range previews {
		if preview != expected {
			t.Fatalf("preview=%+v expected=%+v", preview, expected)
		}
	}
}

func TestRewardsBoundedRejectsBeforeNodeReads(t *testing.T) {
	node := &rewardMeasuredNode{}
	h := HTTP{Service: &JourneyService{creator: NewCreator(node, time.Now)}, Authenticate: func(*http.Request) (Actor, error) { return Actor{Wallet: actorA}, nil }}
	for _, query := range []string{"?limit=0", "?limit=101", "?limit=bad", "?programId=other", "?extra=1", "?limit=1&limit=2"} {
		response := httptest.NewRecorder()
		h.Rewards(response, httptest.NewRequest(http.MethodGet, "/v2/me/rewards"+query, nil))
		if response.Code != http.StatusUnprocessableEntity || node.calls != 0 {
			t.Fatalf("query=%s status=%d calls=%d", query, response.Code, node.calls)
		}
	}
	h.Authenticate = func(*http.Request) (Actor, error) { return Actor{}, ErrUnauthorized }
	response := httptest.NewRecorder()
	h.Rewards(response, httptest.NewRequest(http.MethodGet, "/v2/me/rewards", nil))
	if response.Code != http.StatusUnauthorized || node.calls != 0 {
		t.Fatalf("authentication status=%d calls=%d", response.Code, node.calls)
	}
}
