package processor

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestEventIntegerRanges(t *testing.T) {
	for _, tc := range []struct {
		name      string
		height    uint64
		index     uint
		wantError bool
	}{
		{"zero", 0, 0, false},
		{"height upper bound", math.MaxInt64, 0, false},
		{"height above upper bound", math.MaxInt64 + 1, 0, true},
		{"index upper bound", 1, math.MaxInt32, false},
		{"index above upper bound", 1, math.MaxInt32 + 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &Processor{}
			event, err := p.processEvent(context.Background(), &decode.Event{EventName: "Unknown", BlockNumber: tc.height, LogIndex: tc.index}, 1, nil)
			if (err != nil) != tc.wantError || event != nil {
				t.Fatalf("processEvent(%d, %d) = %v, %v; want error %v", tc.height, tc.index, event, err, tc.wantError)
			}
		})
	}
}

func TestReplayIntegerRefusals(t *testing.T) {
	for _, tc := range []struct {
		name     string
		from, to int64
		value    int64
	}{
		{"negative first height", -1, 0, -1},
		{"minimum first height", math.MinInt64, 0, math.MinInt64},
		{"negative last height", 0, -1, -1},
		{"minimum last height", 0, math.MinInt64, math.MinInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := new(ReplayRunner).fetch(context.Background(), store.ReplayRequest{}, tc.from, tc.to)
			want := fmt.Sprintf("integer %d is outside the range of uint64", tc.value)
			if err == nil || err.Error() != want || !reflect.DeepEqual(got, store.ReplaySegment{}) {
				t.Fatalf("fetch(%d, %d) = %+v, %v; want %q", tc.from, tc.to, got, err, want)
			}
		})
	}
	for _, bounds := range [][2]int64{{0, 64}, {1, 0}} {
		runner := &ReplayRunner{Source: new(receipt.RPCQualification)}
		segment, err := runner.fetch(context.Background(), store.ReplayRequest{}, bounds[0], bounds[1])
		if err != receipt.ErrBinding || !reflect.DeepEqual(segment, store.ReplaySegment{}) {
			t.Fatalf("fetch(%d, %d) = %+v, %v; want source binding refusal", bounds[0], bounds[1], segment, err)
		}
	}
}

func TestLegacyIntegerRefusals(t *testing.T) {
	ctx := context.Background()
	batch := fixtureBatch(t)
	raw := batch.Records[0]
	registry := sourceRegistry(t, common.HexToAddress(raw.Emitter))
	log := types.Log{Address: common.HexToAddress(raw.Emitter), BlockHash: common.HexToHash(raw.BlockHash), TxHash: common.HexToHash(raw.TransactionHash), BlockNumber: 101, TxIndex: uint(raw.TransactionIndex), Index: uint(raw.LogIndex), Data: raw.Data}
	for _, topic := range raw.Topics {
		log.Topics = append(log.Topics, common.HexToHash(topic))
	}
	envelope, err := registry.Decode(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, height := range []int64{math.MinInt64, -1} {
		t.Run(fmt.Sprintf("height %d", height), func(t *testing.T) {
			changed := raw
			changed.Height = height
			err := new(Processor).projectLegacyEnvelopes(ctx, store.CanonicalBatch{Records: []store.RawRecord{changed}}, []*decode.DecodedEnvelope{envelope}, map[string]int64{raw.TransactionHash: 123456}, nil, nil)
			want := fmt.Sprintf("integer %d is outside the range of uint64", height)
			if err == nil || err.Error() != want {
				t.Fatalf("projectLegacyEnvelopes = %v; want %q", err, want)
			}
		})
	}
	for _, index := range []uint32{math.MaxInt32 + 1, math.MaxUint32} {
		t.Run(fmt.Sprintf("reserve index %d", index), func(t *testing.T) {
			err := new(Processor).projectLegacyEnvelopes(ctx, store.CanonicalBatch{}, nil, map[string]int64{raw.TransactionHash: 123456}, nil, []reserveSnapshot{{TransactionHash: raw.TransactionHash, LogIndex: index}})
			want := fmt.Sprintf("integer %d is outside the range of int32", index)
			if err == nil || err.Error() != want {
				t.Fatalf("projectLegacyEnvelopes = %v; want %q", err, want)
			}
		})
	}
}

func TestStrategyRangeErrors(t *testing.T) {
	p := &Processor{}
	for _, value := range []int64{math.MinInt32 - 1, math.MaxInt32 + 1} {
		if err := p.handleFeeDistributed(context.Background(), map[string]any{"nftId": "1", "strategy": value}, "", 1, 1, 1); err == nil {
			t.Fatalf("fee distribution accepted strategy %d", value)
		}
		if err := p.handleFeeStrategyChanged(context.Background(), map[string]any{"nftId": "1", "oldStrategy": value, "newStrategy": "0"}, "", 1, 1, 1); err == nil {
			t.Fatalf("strategy change accepted old strategy %d", value)
		}
		if err := p.handleFeeStrategyChanged(context.Background(), map[string]any{"nftId": "1", "oldStrategy": "0", "newStrategy": value}, "", 1, 1, 1); err == nil {
			t.Fatalf("strategy change accepted new strategy %d", value)
		}
	}
}

func TestIntegerCoercionRanges(t *testing.T) {
	for _, tc := range []struct {
		value     any
		want      int64
		wantError bool
	}{
		{int(-1), -1, false},
		{uint8(math.MaxUint8), math.MaxUint8, false},
		{uint16(math.MaxUint16), math.MaxUint16, false},
		{uint32(math.MaxUint32), math.MaxUint32, false},
		{uint64(0), 0, false},
		{uint64(math.MaxInt64), math.MaxInt64, false},
		{uint64(math.MaxInt64) + 1, 0, true},
		{uint64(math.MaxUint64), 0, true},
	} {
		got, err := asInt64(tc.value)
		if got != tc.want || (err != nil) != tc.wantError {
			t.Fatalf("asInt64(%v) = %d, %v; want %d, error %v", tc.value, got, err, tc.want, tc.wantError)
		}
	}
}
