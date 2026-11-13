package store

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestCanonicalAdmissionRejectsInvalidRawAndOutboxBounds(t *testing.T) {
	raw := lifecycleRaw()
	base := CanonicalBatch{ChainID: 125, StreamID: "bounded", Owner: "worker", BatchID: "batch", Fence: 1, PreviousHeight: 100, Blocks: []BlockCompleteness{{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []RawRecord{raw}, Outbox: []OutboxIntent{{TransactionHash: raw.TransactionHash, Kind: "Swap", Payload: json.RawMessage(`{}`)}}}
	if err := validateBatch(base); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*CanonicalBatch)
	}{
		{"overflow", func(b *CanonicalBatch) { b.PreviousHeight = math.MaxInt64 }},
		{"record hash", func(b *CanonicalBatch) { b.Records[0].TransactionHash = "invalid" }},
		{"topic", func(b *CanonicalBatch) { b.Records[0].Topics = []string{"invalid"} }},
		{"duplicate", func(b *CanonicalBatch) { b.Records = append(b.Records, b.Records[0]); b.Blocks[0].RawCount++ }},
		{"unbound height", func(b *CanonicalBatch) { b.Records[0].Height++ }},
		{"unbound block", func(b *CanonicalBatch) { b.Records[0].BlockHash = "0x" + strings.Repeat("f", 64) }},
		{"outbox identity", func(b *CanonicalBatch) { b.Outbox[0].LogIndex++ }},
		{"outbox payload", func(b *CanonicalBatch) { b.Outbox[0].Payload = json.RawMessage(`{`) }},
		{"raw aggregate", func(b *CanonicalBatch) {
			b.Records = nil
			data := make([]byte, 1<<20)
			for i := 0; i < 32; i++ {
				r := raw
				r.LogIndex = uint32(i)
				r.Data = data
				b.Records = append(b.Records, r)
			}
			b.Blocks[0].RawCount = 32
		}},
		{"outbox aggregate", func(b *CanonicalBatch) {
			b.Outbox = nil
			payload := json.RawMessage(`"` + strings.Repeat("x", (1<<20)-2) + `"`)
			for i := 0; i < 33; i++ {
				b.Outbox = append(b.Outbox, OutboxIntent{TransactionHash: raw.TransactionHash, Kind: "Swap", Payload: payload})
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			var b CanonicalBatch
			if err = json.Unmarshal(body, &b); err != nil {
				t.Fatal(err)
			}
			tc.edit(&b)
			if err = validateBatch(b); err == nil {
				t.Fatal("invalid canonical input admitted")
			}
		})
	}
}

func TestReplayAdmissionBindsEveryRawRecordAndTypedEvent(t *testing.T) {
	raw := lifecycleRaw()
	digest, err := RawDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	base := ReplaySegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: "bounded", FromHeight: 101, ToHeight: 101, Blocks: []BlockCompleteness{{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []RawRecord{raw}, Events: []ReplayEvent{{Raw: raw, RawDigest: digest, Kind: "Swap", Timestamp: 1700000000, Payload: json.RawMessage(`{}`)}}}
	if err = ValidateReplaySegment(base); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*ReplaySegment)
	}{
		{"bounds", func(s *ReplaySegment) { s.ToHeight = 1101 }},
		{"invalid raw", func(s *ReplaySegment) { s.Records[0].Emitter = "invalid" }},
		{"outside range", func(s *ReplaySegment) { s.Records[0].Height++ }},
		{"duplicate raw", func(s *ReplaySegment) { s.Records = append(s.Records, s.Records[0]) }},
		{"source order", func(s *ReplaySegment) {
			r := s.Records[0]
			r.TransactionHash = "0x" + strings.Repeat("f", 64)
			s.Records = append(s.Records, r)
			s.Blocks[0].RawCount++
		}},
		{"block identity", func(s *ReplaySegment) { s.Blocks[0].Hash = "0x" + strings.Repeat("f", 64) }},
		{"event raw", func(s *ReplaySegment) { s.Events[0].Raw.Emitter = "invalid" }},
		{"event digest", func(s *ReplaySegment) { s.Events[0].RawDigest = strings.Repeat("f", 64) }},
		{"duplicate event", func(s *ReplaySegment) { s.Events = append(s.Events, s.Events[0]) }},
		{"aggregate", func(s *ReplaySegment) {
			s.Records = nil
			s.Events = nil
			data := make([]byte, 1<<20)
			for i := 0; i < 32; i++ {
				r := raw
				r.LogIndex = uint32(i)
				r.Data = data
				s.Records = append(s.Records, r)
			}
			s.Blocks[0].RawCount = 32
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			var segment ReplaySegment
			if err = json.Unmarshal(body, &segment); err != nil {
				t.Fatal(err)
			}
			tc.edit(&segment)
			if err = ValidateReplaySegment(segment); err == nil {
				t.Fatal("invalid replay input admitted")
			}
		})
	}
}
