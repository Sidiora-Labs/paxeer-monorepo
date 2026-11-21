package store

import (
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
)

func TestBlockContentBoundsCanonicalBatch(t *testing.T) {
	raw := lifecycleRaw()
	base := CanonicalBatch{ChainID: 125, StreamID: "bounded", Owner: "worker", BatchID: "batch", Fence: 1, PreviousHeight: 100, Blocks: []BlockCompleteness{{Height: 101, Hash: raw.BlockHash, FetchComplete: true, RequiredDecodeComplete: true}}}
	for _, tc := range []struct {
		name             string
		count, dataBytes int
	}{{"records", 100001, 0}, {"log data", 1, decode.MaxEnvelopeBytes + 1}, {"single block bytes", 33, 1 << 20}} {
		t.Run(tc.name, func(t *testing.T) {
			batch := base
			batch.Blocks = append([]BlockCompleteness(nil), base.Blocks...)
			batch.Blocks[0].RawCount = tc.count
			batch.Records = make([]RawRecord, tc.count)
			data := make([]byte, tc.dataBytes)
			for i := range batch.Records {
				batch.Records[i] = raw
				batch.Records[i].LogIndex = uint32(i)
				batch.Records[i].Data = data
			}
			if err := validateBatch(batch); err != nil {
				t.Fatal(err)
			}
			if tc.name != "records" {
				if _, err := RawDigest(batch.Records[0]); err != nil {
					t.Fatal(err)
				}
			}
			if tc.name != "log data" {
				batch.Blocks = append(batch.Blocks, BlockCompleteness{Height: 102, Hash: raw.BlockHash, FetchComplete: true, RequiredDecodeComplete: true})
				if err := validateBatch(batch); err == nil {
					t.Fatal("multi-block admission bound")
				}
			}
		})
	}
}
