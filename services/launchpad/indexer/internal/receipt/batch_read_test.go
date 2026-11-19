package receipt

import (
	"bytes"
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestReadBatchRetainsRecordedBlockReceipts(t *testing.T) {
	ctx := context.Background()
	input := receiptReplayInputsFromProducer(t)
	authority, closeClient := receiptReplayOpen(t, ctx, input)
	defer closeClient()
	bundle, err := authority.Bundle(ctx, input.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	height := bundle.Header.Number.Uint64()
	expectedBlock, expectedReceipts, err := authority.pinnedBlock(ctx, bundle.BlockHash)
	if err != nil {
		t.Fatal(err)
	}
	reads, err := authority.ReadBatch(ctx, height, height)
	if err != nil || len(reads) != 1 {
		t.Fatalf("recorded batch: %d, %v", len(reads), err)
	}
	read := reads[0]
	if read.Height != height || read.Hash != bundle.BlockHash || read.ParentHash != bundle.Header.ParentHash || !reflect.DeepEqual(read.Header, expectedBlock.Header) || len(read.Transactions) != len(expectedBlock.Transactions) || !reflect.DeepEqual(read.Receipts, expectedReceipts) {
		t.Fatal("batch changed recorded block or receipt material")
	}
	for i, transaction := range read.Transactions {
		got, err := transaction.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		want, err := expectedBlock.Transactions[i].MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) || transaction.Hash() != expectedBlock.Transactions[i].Hash() {
			t.Fatalf("recorded signed transaction %d changed", i)
		}
	}

	cached, ok := authority.reads.height(height)
	if !ok || cached.block.Hash != read.Hash || !reflect.DeepEqual(cached.receipts, read.Receipts) {
		t.Fatal("batch did not retain height-bound receipt material")
	}
	repeated, err := authority.ReadBatch(ctx, height, height)
	if err != nil || !reflect.DeepEqual(reads, repeated) {
		t.Fatalf("repeated recorded batch: %v", err)
	}
	for _, bounds := range [][2]uint64{{height, height - 1}, {height, height + 64}, {0, math.MaxUint64}} {
		if reads, err := authority.ReadBatch(ctx, bounds[0], bounds[1]); reads != nil || !errors.Is(err, ErrQualification) {
			t.Fatalf("batch range %v: %v, %v", bounds, reads, err)
		}
	}
	for _, unavailable := range []*RPCQualification{nil, {}} {
		if reads, err := unavailable.ReadBatch(ctx, height, height); reads != nil || !errors.Is(err, ErrQualification) {
			t.Fatalf("unavailable authority: %v, %v", reads, err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if reads, err := authority.ReadBatch(canceled, height, height); reads != nil || !errors.Is(err, ErrQualification) {
		t.Fatalf("canceled batch: %v, %v", reads, err)
	}
}
