package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestArchiveValidationBindsRawEventsAndBlockOrder(t *testing.T) {
	newSegment := func() ArchiveSegment {
		blockHash := "0x" + strings.Repeat("a", 64)
		raw := ArchiveRaw{Height: 10, TransactionHash: "0x" + strings.Repeat("b", 64), BlockHash: blockHash, Emitter: "0x" + strings.Repeat("c", 40), Topics: []string{}, Data: []byte{}}
		digest, err := archiveRawDigest(raw)
		if err != nil {
			t.Fatal(err)
		}
		return ArchiveSegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 10, ToHeight: 10,
			Blocks:  []ArchiveBlock{{Height: 10, Hash: blockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}},
			Records: []ArchiveRaw{raw}, Events: []ArchiveEvent{{RawDigest: digest, Kind: "market.other.v1", Timestamp: 1700000000, Payload: json.RawMessage(`{"kind":"other"}`), Raw: raw}}}
	}
	check := func(segment ArchiveSegment, change func(*ArchiveManifest)) ([]byte, ArchiveManifest) {
		t.Helper()
		payload, err := json.Marshal(segment)
		if err != nil {
			t.Fatal(err)
		}
		manifest := ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: "canonical", FromHeight: segment.FromHeight, ToHeight: segment.ToHeight, EventCount: len(segment.Records), TypedCount: len(segment.Events), ByteLength: int64(len(payload)), SHA256: bytesDigest(payload), Bucket: "charts", Blocks: segment.Blocks}
		manifest.ObjectKey = "indexer/raw/v2/canonical/" + manifest.SHA256 + ".json"
		if change != nil {
			change(&manifest)
		}
		return payload, manifest
	}
	valid, manifest := check(newSegment(), nil)
	if fills, err := ValidateArchive(valid, manifest); err != nil || len(fills) != 0 {
		t.Fatalf("non-market archive event: %+v, %v", fills, err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*ArchiveSegment)
	}{
		{"invalid raw transaction", func(s *ArchiveSegment) { s.Records[0].TransactionHash = "0xbroken" }},
		{"invalid raw topic", func(s *ArchiveSegment) { s.Records[0].Topics = []string{"0xbroken"} }},
		{"duplicate raw identity", func(s *ArchiveSegment) { s.Records = append(s.Records, s.Records[0]); s.Blocks[0].RawCount = 2 }},
		{"reversed transaction order", func(s *ArchiveSegment) {
			first := s.Records[0]
			first.TransactionIndex = 1
			second := first
			second.TransactionIndex = 0
			second.TransactionHash = "0x" + strings.Repeat("e", 64)
			s.Records = []ArchiveRaw{first, second}
			s.Blocks[0].RawCount = 2
		}},
		{"record outside archive height", func(s *ArchiveSegment) { s.Records[0].Height = 11 }},
		{"record block mismatch", func(s *ArchiveSegment) { s.Records[0].BlockHash = "0x" + strings.Repeat("d", 64) }},
		{"raw block count mismatch", func(s *ArchiveSegment) { s.Blocks[0].RawCount = 2 }},
		{"typed event has no raw record", func(s *ArchiveSegment) {
			s.Events[0].Raw.TransactionHash = "0x" + strings.Repeat("e", 64)
			digest, err := archiveRawDigest(s.Events[0].Raw)
			if err != nil {
				t.Fatal(err)
			}
			s.Events[0].RawDigest = digest
		}},
		{"invalid typed raw topic", func(s *ArchiveSegment) { s.Events[0].Raw.Topics = []string{"0xbroken"} }},
		{"event digest mismatch", func(s *ArchiveSegment) { s.Events[0].RawDigest = strings.Repeat("0", 64) }},
		{"event timestamp absent", func(s *ArchiveSegment) { s.Events[0].Timestamp = 0 }},
		{"event kind absent", func(s *ArchiveSegment) { s.Events[0].Kind = "" }},
		{"duplicate typed event", func(s *ArchiveSegment) { s.Events = append(s.Events, s.Events[0]) }},
		{"malformed market fill", func(s *ArchiveSegment) {
			s.Events[0].Kind = accounting.MarketSubject
			s.Events[0].Payload = json.RawMessage(`{"version":"accounting-fill-v1"}`)
		}},
		{"segment stream disagreement", func(s *ArchiveSegment) { s.SourceStream = "other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			segment := newSegment()
			tc.mutate(&segment)
			payload, m := check(segment, nil)
			if fills, err := ValidateArchive(payload, m); err == nil || len(fills) != 0 {
				t.Fatalf("invalid archive event admitted: %+v, %v", fills, err)
			}
		})
	}
	canonical := newSegment()
	canonical.Records[0].Topics = nil
	canonical.Records[0].Data = nil
	canonical.Events[0].Raw = canonical.Records[0]
	canonical.Events[0].RawDigest, _ = archiveRawDigest(canonical.Records[0])
	payload, noPayloadManifest := check(canonical, nil)
	if fills, err := ValidateArchive(payload, noPayloadManifest); err != nil || len(fills) != 0 {
		t.Fatalf("canonical empty raw fields: %+v, %v", fills, err)
	}
	if fills, err := ValidateArchive(valid, func() ArchiveManifest { invalid := manifest; invalid.ChainID = 1; return invalid }()); err == nil || len(fills) != 0 {
		t.Fatalf("foreign chain manifest admitted: %+v, %v", fills, err)
	}
	trailing := append(append([]byte(nil), valid...), []byte(`{}`)...)
	trailingManifest := manifest
	trailingManifest.ByteLength = int64(len(trailing))
	trailingManifest.SHA256 = bytesDigest(trailing)
	trailingManifest.ObjectKey = "indexer/raw/v2/canonical/" + trailingManifest.SHA256 + ".json"
	if fills, err := ValidateArchive(trailing, trailingManifest); err == nil || len(fills) != 0 {
		t.Fatalf("trailing archive document admitted: %+v, %v", fills, err)
	}
}
