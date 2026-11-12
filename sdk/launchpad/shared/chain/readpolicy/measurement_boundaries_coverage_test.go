package readpolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestMeasurementArtifactsAndSampleBounds(t *testing.T) {
	binding := measuredTestBinding(t)
	for _, invalid := range []MeasurementBinding{
		{},
		func() MeasurementBinding { b := binding; b.WorkloadSeed = ""; return b }(),
	} {
		if err := invalid.Validate(); err == nil {
			t.Fatal("invalid measurement binding accepted")
		}
	}
	if _, err := NewMeasurementCapture("", binding, 1000, 1<<20); err == nil {
		t.Fatal("measurement without a destination accepted")
	}
	for _, value := range []string{"", "-1", "1.5", "18446744073709551616"} {
		if _, err := ParseHeight(value); err == nil {
			t.Fatalf("invalid measured height accepted: %q", value)
		}
	}
	if height, err := ParseHeight("18446744073709551615"); err != nil || height != ^uint64(0) {
		t.Fatalf("maximum measured height: %d, %v", height, err)
	}
	root := t.TempDir()
	content := []byte("recorded source artifact")
	path := filepath.Join(root, "source")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	artifact := BoundMeasurementArtifact{Path: path, SHA256: hex.EncodeToString(digest[:]), MaxBytes: int64(len(content))}
	capture, err := NewMeasurementCapture(filepath.Join(root, "capture"), binding, 1000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	file, err := capture.OpenArtifact("source", artifact)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := capture.OpenArtifact("source", artifact); err == nil {
		t.Fatal("duplicate artifact identity accepted")
	}
	if _, err := capture.OpenArtifact("another", BoundMeasurementArtifact{Path: path, SHA256: binding.BuildSHA256, MaxBytes: int64(len(content))}); err == nil {
		t.Fatal("changed artifact digest accepted")
	}
	now := time.Now().UnixNano()
	sample := MeasurementSample{RequestID: "sample-0", Scenario: "scenario-0", ChainID: "125", Height: "100", BlockHash: "0x" + binding.BuildSHA256, ObservedUTCNS: now, FinishedUTCNS: now + 1, ElapsedNS: 1, Outcome: "success"}
	if err := capture.Append(sample); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < 32; i++ {
		next := sample
		next.RequestID = "sample-" + strconv.Itoa(i)
		next.Scenario = "scenario-" + strconv.Itoa(i)
		if err := capture.Append(next); err != nil {
			t.Fatal(err)
		}
	}
	next := sample
	next.RequestID = "sample-32"
	next.Scenario = "scenario-32"
	if err := capture.Append(next); err == nil {
		t.Fatal("unbounded measurement scenarios accepted")
	}
	if err := capture.Append(sample); err == nil {
		t.Fatal("failed measurement capture admitted another sample")
	}
	result, err := capture.Finish(true, "")
	if err != nil || result.LaneHarnessPassed || result.RawRows != 33 || result.Inputs["source"].SHA256 != artifact.SHA256 {
		t.Fatalf("bounded capture result: %+v, %v", result, err)
	}
	if _, err := capture.OpenArtifact("late", artifact); err == nil {
		t.Fatal("finalized measurement accepted another artifact")
	}
}

func TestMeasurementRejectsMalformedPinnedSample(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*MeasurementSample)
	}{
		{"height", func(s *MeasurementSample) { s.Height = "-1" }},
		{"hash", func(s *MeasurementSample) { s.BlockHash = "0x1234" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture, err := NewMeasurementCapture(filepath.Join(t.TempDir(), "capture"), measuredTestBinding(t), 1000, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UnixNano()
			sample := MeasurementSample{RequestID: "pinned-sample", Scenario: "pinned-read", ChainID: "125", ObservedUTCNS: now, FinishedUTCNS: now + 1, ElapsedNS: 1, Outcome: "success"}
			tc.edit(&sample)
			if err := capture.Append(sample); err == nil {
				t.Fatal("malformed pinned sample accepted")
			}
			result, err := capture.Finish(true, "")
			if err != nil || result.LaneHarnessPassed || result.RawRows != 0 {
				t.Fatalf("malformed sample result: %+v, %v", result, err)
			}
		})
	}
}
