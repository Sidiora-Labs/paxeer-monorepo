package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/ethereum/go-ethereum/common"
)

func TestRecordedRangeReadAheadRechecksCurrentBloomAdmission(t *testing.T) {
	ctx := context.Background()
	input := repair53InputsFromProducer(t)
	a, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	if err := a.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	a = a.current()
	bundle, err := a.Bundle(ctx, input.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	height := bundle.Header.Number.Uint64()
	block, receipts, err := a.pinnedBlock(ctx, bundle.BlockHash)
	if err != nil {
		t.Fatal(err)
	}
	// These are the original pinned block and receipts, read before a gate refresh.
	done := make(chan struct{})
	close(done)
	a.reads.mu.Lock()
	a.reads.ahead = map[uint64]*pinnedAhead{height: {done: done, pinned: pinnedHeight{header: block.Header, block: block, skipped: true}}}
	a.reads.mu.Unlock()
	batch, authority, err := a.sourceBlocks(ctx, height, height)
	if err != nil || len(batch.Blocks) != 1 || len(batch.Records) == 0 || authority == nil || a.ArchiveReadMetrics().ReadaheadHits != 1 || a.ArchiveReadMetrics().ReceiptReads == 0 {
		t.Fatalf("refreshed bloom admission %+v: %v", batch, err)
	}
	if len(receipts) == 0 {
		t.Fatal("recorded block contains no receipts")
	}
	emptyGate := bloomGate{}
	skipped, gotReceipts, didSkip, err := a.pinnedBlockAtHeight(ctx, height, emptyGate)
	if err != nil || !didSkip || len(gotReceipts) != 0 || skipped.Hash != block.Hash {
		t.Fatalf("irrelevant block pin %s/%v: %v", skipped.Hash, didSkip, err)
	}
	for _, bounds := range [][2]uint64{{0, 0}, {height, height - 1}, {height, height + 64}, {height, math.MaxUint64}} {
		if _, _, err = a.sourceBlocks(ctx, bounds[0], bounds[1]); !errors.Is(err, ErrBinding) {
			t.Fatalf("invalid range %v: %v", bounds, err)
		}
	}
	for _, mode := range []string{"invalid raw identity", "too many transactions", "unavailable transaction", "unbound record"} {
		t.Run(mode, func(t *testing.T) {
			changed := batch
			changed.Records = append([]store.RawRecord(nil), batch.Records...)
			switch mode {
			case "invalid raw identity":
				changed.Records[0].TransactionHash = "invalid"
			case "too many transactions":
				changed.Records = make([]store.RawRecord, 4097)
				for i := range changed.Records {
					changed.Records[i].TransactionHash = fmt.Sprintf("0x%064x", i+1)
				}
			case "unavailable transaction":
				changed.Records[0].TransactionHash = common.HexToHash("0xbad").Hex()
			case "unbound record":
				changed.Records[0].Data = append(changed.Records[0].Data, 1)
			}
			if _, e := a.issueBatch(ctx, changed); e == nil {
				t.Fatal("unqualified batch admitted")
			}
		})
	}
}

func TestRecordedHeightPinsRejectUnavailableAndMisboundResponses(t *testing.T) {
	ctx := context.Background()
	original := repair53InputsFromProducer(t)
	base, closeBase := repair53Open(t, ctx, original)
	defer closeBase()
	bundle, err := base.Bundle(ctx, original.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	height := bundle.Header.Number.Uint64()
	for _, mode := range []string{"pruned", "unavailable", "malformed", "height", "timestamp", "parent linkage"} {
		t.Run(mode, func(t *testing.T) {
			input, e := clone(original)
			if e != nil {
				t.Fatal(e)
			}
			changed := 0
			for key, response := range input.Responses {
				if !strings.HasPrefix(key, "eth_getBlockByNumber ") || !strings.Contains(key, fmt.Sprintf(`"0x%x"`, height)) {
					continue
				}
				switch mode {
				case "pruned", "unavailable":
					response.Result = nil
					response.Error = json.RawMessage(`{"code":-32000,"message":"` + mode + `"}`)
				case "malformed":
					response.Result = json.RawMessage(`{}`)
				default:
					var value map[string]json.RawMessage
					if e = json.Unmarshal(response.Result, &value); e != nil {
						t.Fatal(e)
					}
					switch mode {
					case "height":
						value["number"] = json.RawMessage(`"0x1"`)
					case "timestamp":
						value["timestamp"] = json.RawMessage(`"0x0"`)
					case "parent linkage":
						value["parentHash"], e = json.Marshal(common.HexToHash("0x1"))
						if e != nil {
							t.Fatal(e)
						}
					}
					response.Result, e = json.Marshal(value)
					if e != nil {
						t.Fatal(e)
					}
				}
				input.Responses[key] = response
				changed++
			}
			if changed == 0 {
				t.Fatal("recorded numbered block absent")
			}
			input = repair53Replay(t, input)
			a, closeClient := repair53Open(t, ctx, input)
			defer closeClient()
			if e = a.Rehydrate(ctx, input.Markets); e != nil {
				t.Fatal(e)
			}
			a = a.current()
			if mode == "parent linkage" {
				_, _, e = a.sourceBlocks(ctx, height-1, height)
			} else {
				_, _, _, e = a.pinnedBlockAtHeight(ctx, height, nil)
			}
			if e == nil {
				t.Fatal("unavailable or misbound range admitted")
			}
			if mode == "pruned" && !IsPruned(e) {
				t.Fatalf("pruned classification: %v", e)
			}
		})
	}
	for _, q := range []*RPCQualification{nil, {}} {
		if q.ArchiveReadMetrics() != (ArchiveReadMetrics{}) {
			t.Fatal("unconfigured archive metrics")
		}
	}
	var readState *rpcReadState
	if _, ok := readState.code("epoch"); ok {
		t.Fatal("absent code epoch present")
	}
	readState.storeCode("epoch", "digest")
}
