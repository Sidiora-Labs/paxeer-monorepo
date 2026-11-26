package l18

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
)

type rewardNodeSingle struct {
	client     *rpc.Client
	singles    map[string]int
	methods    []string
	batches    [][]rpc.BatchElem
	proxy      bool
	after      func(any, string, ...any) error
	batchAfter func([]rpc.BatchElem) error
}

func (r *rewardNodeSingle) CallContext(ctx context.Context, result any, method string, args ...any) error {
	r.singles[method]++
	r.methods = append(r.methods, method)
	if err := r.client.CallContext(ctx, result, method, args...); err != nil {
		return err
	}
	r.implementation(result, method)
	if r.after != nil {
		return r.after(result, method, args...)
	}
	return nil
}
func (r *rewardNodeSingle) implementation(result any, method string) {
	if r.proxy && method == "eth_getStorageAt" {
		*result.(*hexutil.Bytes) = common.LeftPadBytes(common.HexToAddress(NFT).Bytes(), 32)
	}
}

type rewardNodeBatch struct{ *rewardNodeSingle }

func (r *rewardNodeBatch) BatchCallContext(ctx context.Context, batch []rpc.BatchElem) error {
	r.batches = append(r.batches, append([]rpc.BatchElem(nil), batch...))
	if err := r.client.BatchCallContext(ctx, batch); err != nil {
		return err
	}
	for _, elem := range batch {
		if elem.Error == nil {
			r.implementation(elem.Result, elem.Method)
		}
	}
	if r.batchAfter != nil {
		return r.batchAfter(batch)
	}
	return nil
}

func rewardNodeReader(t *testing.T, batching, proxy bool) (*rewardHistoryFixture, *rewardNodeSingle) {
	t.Helper()
	f := rewardHistoryChain(t)
	r := &rewardNodeSingle{client: f.rpc, singles: map[string]int{}, proxy: proxy}
	var transport RPC = r
	if batching {
		transport = &rewardNodeBatch{r}
	}
	digest := strings.ToLower(crypto.Keccak256Hash([]byte{0x60, 0, 0x60, 0}).Hex())
	hashes := map[string]string{NFT: digest, Accumulator: digest, FeesRouter: digest, USDL: digest}
	f.creator.rpc = transport
	f.creator.attest = func(ctx context.Context, b Checkpoint) error {
		return verifyRuntime(ctx, transport, b, hashes)
	}
	return f, r
}
func rewardNodePage(t *testing.T, f *rewardHistoryFixture) RewardPage {
	t.Helper()
	page, err := f.journey.Rewards(context.Background(), Actor{Wallet: f.chain.Alice}, "", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].RewardId != "creator:1" || page.Items[0].ClaimableRaw != "10000000" || page.NextCursor != nil || page.Block != (Checkpoint{f.db.peers.Initial, rewardHistoryHash(f.db.peers.Initial)}) {
		t.Fatalf("reward page=%+v", page)
	}
	return page
}
func rewardNodeMethods(batch []rpc.BatchElem) []string {
	methods := make([]string, len(batch))
	for i, elem := range batch {
		methods[i] = elem.Method
	}
	return methods
}

func TestRewardNodeReadsBatchesAndSingles(t *testing.T) {
	batched, calls := rewardNodeReader(t, true, true)
	page := rewardNodePage(t, batched)
	if !reflect.DeepEqual(calls.singles, map[string]int{"eth_chainId": 1, "eth_getBlockByNumber": 1}) {
		t.Fatalf("single node reads=%v", calls.singles)
	}
	want := [][]string{
		{"eth_getCode", "eth_getStorageAt", "eth_getCode", "eth_getStorageAt", "eth_getCode", "eth_getStorageAt", "eth_getCode", "eth_getStorageAt"},
		{"eth_getCode", "eth_getCode", "eth_getCode", "eth_getCode"},
		{"eth_call", "eth_call", "eth_call"},
		{"eth_call", "eth_call"},
	}
	if len(calls.batches) != len(want) {
		t.Fatalf("batch count=%d", len(calls.batches))
	}
	for i, batch := range calls.batches {
		if got := rewardNodeMethods(batch); !reflect.DeepEqual(got, want[i]) {
			t.Fatalf("batch %d=%v, want %v", i, got, want[i])
		}
	}
	for i, target := range []string{NFT, Accumulator, FeesRouter, USDL} {
		if calls.batches[0][2*i].Args[0] != target || calls.batches[0][2*i+1].Args[0] != target {
			t.Fatalf("runtime target %d=%v", i, calls.batches[0])
		}
	}
	for i, names := range [][]string{{"ownerOf", "getPoolAddress", "getFeeStrategy"}, {"supportsAirdrop", "getAccumulatedFees"}} {
		for j, elem := range calls.batches[i+2] {
			call := elem.Args[0].(map[string]any)
			a := nft
			target := NFT
			if i == 1 {
				a, target = accumulator, Accumulator
			}
			data, err := hexutil.Decode(call["data"].(string))
			if err != nil {
				t.Fatal(err)
			}
			method, err := a.MethodById(data[:4])
			if err != nil || method.Name != names[j] || call["to"] != target {
				t.Fatalf("preview call=%v: %v", call, err)
			}
		}
	}
	single, singles := rewardNodeReader(t, false, true)
	if got := rewardNodePage(t, single); !reflect.DeepEqual(got, page) {
		t.Fatalf("single page=%+v batch page=%+v", got, page)
	}
	if len(singles.batches) != 0 || !reflect.DeepEqual(singles.singles, map[string]int{"eth_chainId": 1, "eth_getBlockByNumber": 1, "eth_getCode": 8, "eth_getStorageAt": 4, "eth_call": 5}) {
		t.Fatalf("single transport reads=%v batches=%d", singles.singles, len(singles.batches))
	}
	wantMethods := []string{"eth_chainId", "eth_getBlockByNumber"}
	for range []string{NFT, Accumulator, FeesRouter, USDL} {
		wantMethods = append(wantMethods, "eth_getCode", "eth_getStorageAt", "eth_getCode")
	}
	wantMethods = append(wantMethods, "eth_call", "eth_call", "eth_call", "eth_call", "eth_call")
	if !reflect.DeepEqual(singles.methods, wantMethods) {
		t.Fatalf("single read sequence=%v", singles.methods)
	}
	if batched.creator.checked != nil || single.creator.checked != nil {
		t.Fatal("request checkpoint retained on shared creator")
	}
}

func TestRewardNodeReadsMovingWatermark(t *testing.T) {
	f, calls := rewardNodeReader(t, true, false)
	calls.after = func(_ any, method string, _ ...any) error {
		if method == "eth_chainId" {
			f.advance(f.db.peers.Advanced)
		}
		return nil
	}
	rewardNodePage(t, f)
	f.db.mu.Lock()
	defer f.db.mu.Unlock()
	watermarks, completeness := 0, 0
	for _, sql := range f.db.queries {
		if strings.Contains(sql, "FROM indexer.indexer_ingest_watermarks_v1") {
			watermarks++
		}
		if strings.Contains(sql, "FROM indexer.indexer_block_completeness_v1") {
			completeness++
		}
	}
	if watermarks != 1 || completeness != 1 || f.db.height != f.db.peers.Advanced {
		t.Fatalf("source reads=%d,%d height=%d", watermarks, completeness, f.db.height)
	}
}

func TestRewardNodeReadsHeaderWait(t *testing.T) {
	for _, tc := range []struct {
		name    string
		missing int
		want    error
		reads   int
	}{
		{"eventually available", 2, nil, 3},
		{"unavailable", 6, ErrUnavailable, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, calls := rewardNodeReader(t, true, false)
			calls.after = func(result any, method string, _ ...any) error {
				if method == "eth_getBlockByNumber" && calls.singles[method] <= tc.missing {
					return json.Unmarshal([]byte("null"), result)
				}
				return nil
			}
			start := time.Now()
			page, err := f.journey.Rewards(context.Background(), Actor{Wallet: f.chain.Alice}, "", "", 1)
			if !errors.Is(err, tc.want) || calls.singles["eth_getBlockByNumber"] != tc.reads || time.Since(start) < time.Duration(tc.reads-1)*100*time.Millisecond {
				t.Fatalf("header reads=%v elapsed=%s error=%v", calls.singles, time.Since(start), err)
			}
			if err == nil && (len(page.Items) != 1 || page.Block.Number != f.db.peers.Initial) {
				t.Fatalf("reward page=%+v", page)
			}
		})
	}
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "canceled", true: "deadline"}[deadline], func(t *testing.T) {
			f, calls := rewardNodeReader(t, true, false)
			ctx, cancel := context.WithCancel(context.Background())
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 250*time.Millisecond)
			}
			defer cancel()
			calls.after = func(result any, method string, _ ...any) error {
				if method == "eth_getBlockByNumber" {
					if !deadline {
						cancel()
					}
					return json.Unmarshal([]byte("null"), result)
				}
				return nil
			}
			_, err := f.journey.Rewards(ctx, Actor{Wallet: f.chain.Alice}, "", "", 1)
			want := error(context.Canceled)
			if deadline {
				want = context.DeadlineExceeded
			}
			reads := calls.singles["eth_getBlockByNumber"]
			if !errors.Is(err, want) || reads < 1 || reads >= 6 || !deadline && reads != 1 {
				t.Fatalf("header wait reads=%v error=%v", calls.singles, err)
			}
		})
	}
}

func TestRewardNodeReadsBatchErrorsAndValidation(t *testing.T) {
	failure := errors.New("node read unavailable")
	for _, tc := range []struct {
		name           string
		batch, element int
		code           bool
		want           error
	}{
		{"runtime code element", 0, 0, false, failure},
		{"runtime slot element", 0, 1, false, failure},
		{"implementation element", 1, 0, false, failure},
		{"NFT element", 2, 1, false, failure},
		{"accumulator element", 3, 1, false, failure},
		{"runtime hash", 0, 0, true, ErrUnavailable},
		{"implementation hash", 1, 0, true, ErrUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, calls := rewardNodeReader(t, true, true)
			calls.batchAfter = func(batch []rpc.BatchElem) error {
				if len(calls.batches)-1 == tc.batch {
					if tc.code {
						*batch[tc.element].Result.(*hexutil.Bytes) = []byte{0x60, 1}
					} else {
						batch[tc.element].Error = failure
					}
				}
				return nil
			}
			_, err := f.journey.Rewards(context.Background(), Actor{Wallet: f.chain.Alice}, "", "", 1)
			if !errors.Is(err, tc.want) || len(calls.batches) != tc.batch+1 {
				t.Fatalf("batch error=%v batches=%d", err, len(calls.batches))
			}
		})
	}
}

func TestRewardNodeReadsCursorCheckpoint(t *testing.T) {
	for _, advance := range []bool{false, true} {
		t.Run(map[bool]string{false: "same source", true: "new source"}[advance], func(t *testing.T) {
			f, calls := rewardNodeReader(t, true, true)
			block := f.advance(f.db.peers.Aged)
			first, err := f.journey.Rewards(context.Background(), Actor{Wallet: f.chain.Alice}, "", "", 1)
			if err != nil || first.NextCursor == nil {
				t.Fatalf("first page=%+v: %v", first, err)
			}
			if advance {
				f.advance(f.db.peers.Advanced)
			}
			calls.singles, calls.batches = map[string]int{}, nil
			next, err := f.journey.Rewards(context.Background(), Actor{Wallet: f.chain.Alice}, "", *first.NextCursor, 1)
			if err != nil || next.Block != block || len(next.Items) != 1 || next.Items[0].RewardId != "creator:4" {
				t.Fatalf("cursor page=%+v: %v", next, err)
			}
			checks := 1
			if advance {
				checks = 2
			}
			if calls.singles["eth_chainId"] != checks || calls.singles["eth_getBlockByNumber"] != checks || len(calls.batches) != 2*checks+2 {
				t.Fatalf("cursor reads=%v batches=%d", calls.singles, len(calls.batches))
			}
			cursorChecks := 0
			for _, batch := range calls.batches {
				if batch[0].Method == "eth_getCode" && len(batch) == 8 {
					if batch[0].Args[1].(map[string]any)[fieldBlockHash] == block.Hash {
						cursorChecks++
					}
				}
			}
			if cursorChecks != 1 {
				t.Fatalf("cursor runtime checks=%d", cursorChecks)
			}
		})
	}
}

func TestRewardNodeReadsHeaderMismatchAndErrors(t *testing.T) {
	failure := errors.New("header unavailable")
	for _, tc := range []struct {
		name   string
		result any
		want   error
	}{
		{"hash", map[string]any{"number": "0xf", "hash": rewardHistoryHash(14)}, ErrUnavailable},
		{"number", map[string]any{"number": "0xe", "hash": rewardHistoryHash(15)}, ErrUnavailable},
		{"read error", nil, failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, calls := rewardNodeReader(t, true, false)
			calls.after = func(result any, method string, _ ...any) error {
				if method != "eth_getBlockByNumber" {
					return nil
				}
				if tc.result == nil {
					return failure
				}
				raw, err := json.Marshal(tc.result)
				if err != nil {
					return err
				}
				return json.Unmarshal(raw, result)
			}
			_, err := f.journey.Rewards(context.Background(), Actor{Wallet: f.chain.Alice}, "", "", 1)
			if !errors.Is(err, tc.want) || calls.singles["eth_getBlockByNumber"] != 1 || len(calls.batches) != 0 {
				t.Fatalf("header reads=%v batches=%d error=%v", calls.singles, len(calls.batches), err)
			}
		})
	}
}

func TestRewardNodeReadsBatchCheckOrder(t *testing.T) {
	later := errors.New("later node read unavailable")
	for _, tc := range []struct {
		name           string
		batch, element int
	}{
		{"code before slot", 0, 0},
		{"implementation before next target", 1, 0},
		{"NFT decoding before next result", 2, 0},
		{"accumulator decoding before next result", 3, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, calls := rewardNodeReader(t, true, true)
			calls.batchAfter = func(batch []rpc.BatchElem) error {
				index := len(calls.batches) - 1
				if index == tc.batch {
					*batch[tc.element].Result.(*hexutil.Bytes) = []byte{1}
					if tc.batch != 1 {
						batch[tc.element+1].Error = later
					}
				}
				if tc.batch == 1 && index == 0 {
					batch[2].Error = later
				}
				return nil
			}
			_, err := f.journey.Rewards(context.Background(), Actor{Wallet: f.chain.Alice}, "", "", 1)
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("ordered validation error=%v", err)
			}
		})
	}
}

func TestRewardNodeReadsBatchResponseElements(t *testing.T) {
	f := rewardHistoryChain(t)
	var first, last hexutil.Big
	var unused string
	batch := []rpc.BatchElem{
		{Method: "eth_chainId", Result: &first},
		{Method: "unavailable_method", Result: &unused},
		{Method: "eth_chainId", Result: &last},
	}
	if err := f.rpc.BatchCallContext(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	if batch[0].Error != nil || batch[1].Error == nil || batch[2].Error != nil || first.String() != "0x7d" || last.String() != "0x7d" {
		t.Fatalf("batch responses=%v first=%s last=%s", batch, first.String(), last.String())
	}
}
