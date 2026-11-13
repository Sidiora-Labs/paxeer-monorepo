package store

import (
	"errors"
	"strings"
	"testing"
)

func TestReplayRequestAndArchiveBounds(t *testing.T) {
	hash := "0x" + strings.Repeat("a", 64)
	request := ReplayRequest{
		JobID: "job-1", SourceStream: "live", Target: "market", Generation: "g1",
		ProjectionRevision: "v1", Mode: "replay", Requester: "operator",
		AuthorizationID: "approval-1", FromHeight: 12, ToHeight: 12,
		CheckpointHeight: 12, CheckpointHash: hash,
	}
	if err := ValidateReplayRequest(request); err != nil {
		t.Fatal(err)
	}
	if scope := request.Scope(); scope.From != 12 || scope.To != 12 || scope.AuthorizationID != "approval-1" {
		t.Fatalf("replay scope: %+v", scope)
	}
	invalid := request
	invalid.CheckpointHeight = 11
	if err := ValidateReplayRequest(invalid); err == nil {
		t.Fatal("checkpoint before replay range accepted")
	}
	invalid = request
	invalid.JobID = "job/1"
	if err := ValidateReplayRequest(invalid); err == nil {
		t.Fatal("unbounded replay identity accepted")
	}

	block := BlockCompleteness{Height: 12, Hash: hash, FetchComplete: true, RequiredDecodeComplete: true}
	segment := ReplaySegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: "live", FromHeight: 12, ToHeight: 12, Blocks: []BlockCompleteness{block}}
	if err := ValidateReplaySegment(segment); err != nil {
		t.Fatal(err)
	}
	manifest := ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: "live", FromHeight: 12, ToHeight: 12, ByteLength: 1, SHA256: strings.Repeat("a", 64), Bucket: "archive", ObjectKey: "indexer/raw/v2/live/" + strings.Repeat("a", 64) + ".json", Blocks: []BlockCompleteness{block}}
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
	invalidSegment := segment
	invalidSegment.Blocks = []BlockCompleteness{{Height: 12, Hash: hash}}
	if err := ValidateReplaySegment(invalidSegment); err == nil {
		t.Fatal("incomplete block accepted")
	}
	invalidManifest := manifest
	invalidManifest.Blocks = []BlockCompleteness{{Height: 12, Hash: hash}}
	if err := invalidManifest.Validate(); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("archive block error = %v", err)
	}
}
