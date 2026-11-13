package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestCapacityCaptureRetainsBoundedRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.jsonl")
	capture, err := newCapacityCapture(path, 120)
	if err != nil {
		t.Fatal(err)
	}
	if err := capture.write("block", map[string]uint64{"height": 101}); err != nil {
		t.Fatal(err)
	}
	if err := capture.write("large", strings.Repeat("x", 120)); err == nil {
		t.Fatal("capture accepted bytes beyond its bound")
	}
	digest, count, err := capture.finish()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var row struct {
		Kind  string
		Value map[string]uint64
	}
	sum := sha256.Sum256(body)
	if err := json.Unmarshal(body, &row); err != nil || row.Kind != "block" || row.Value["height"] != 101 || count != uint64(len(body)) || digest != hex.EncodeToString(sum[:]) {
		t.Fatalf("capture record=%+v count=%d digest=%q: %v", row, count, digest, err)
	}
	if _, err := newCapacityCapture(path, 120); !os.IsExist(err) {
		t.Fatalf("capture replaced a prior record: %v", err)
	}
}

func TestCapacityMeasurementsUseCommittedSourceLimits(t *testing.T) {
	srv := fixture(t, "0x7d")
	s, err := NewCommittedSource(context.Background(), fopts(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, err := s.ObserveHead(context.Background(), 100); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FetchLogs(context.Background(), FetchOptions{FromBlock: 100, ToBlock: 101}); err != nil {
		t.Fatal(err)
	}
	sample := s.capacityMeasurement()
	if sample.ObservedAt.IsZero() || len(sample.EndpointHealth) != 1 || len(sample.EndpointInFlight) != 1 || sample.HeapBytes == 0 {
		t.Fatalf("source measurement: %+v", sample)
	}
	if err := s.validateCapacityMeasurement(sample, sample.HeapBytes); err != nil {
		t.Fatal(err)
	}
	invalid := []struct {
		name string
		edit func(*CapacityMeasurement)
	}{
		{"negative in flight", func(m *CapacityMeasurement) { m.InFlight = -1 }},
		{"pending ranges", func(m *CapacityMeasurement) { m.PendingRanges = 1025 }},
		{"header cache", func(m *CapacityMeasurement) { m.HeaderCacheEntries = 16385 }},
		{"unknown endpoint", func(m *CapacityMeasurement) { m.EndpointInFlight = map[string]int{"unknown": 1} }},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			changed := sample
			tc.edit(&changed)
			if err := s.validateCapacityMeasurement(changed, sample.HeapBytes); err == nil {
				t.Fatal("source measurement exceeded an admitted limit")
			}
		})
	}
	if err := s.validateCapacityMeasurement(sample, sample.HeapBytes-1); err == nil {
		t.Fatal("source measurement exceeded its heap budget")
	}
	lag, err := s.capacityLagSample(time.Now().UTC(), 100)
	if err != nil || lag.Head != 101 || lag.Durable != 100 || lag.LagNanoseconds != 0 {
		t.Fatalf("committed lag: %+v %v", lag, err)
	}
	if _, err := s.capacityLagSample(time.Now().UTC(), 99); err == nil {
		t.Fatal("lag accepted a missing durable block")
	}
}

func TestCapacityBindingPinsCurrentSourceAndEndpoints(t *testing.T) {
	ctx := context.Background()
	server := fixture(t, "0x7d")
	live, err := NewCommittedSource(ctx, fopts(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.Close() })
	options := capacityBoundOptions(t, live)
	files := options.Binding.Files
	providers := CapacityProviders{Live: live, Historical: &HistoricalSource{RPC: live}}
	if err := verifyCapacityBinding(ctx, options, providers); err != nil {
		t.Fatalf("current source binding: %v", err)
	}
	changed := options
	changed.Binding = options.Binding
	changed.Binding.Files = append([]CapacityBoundFile(nil), files...)
	changed.Binding.Files[0].SHA256 = strings.Repeat("0", 64)
	if err := verifyCapacityBinding(ctx, changed, providers); err == nil {
		t.Fatal("changed source file digest accepted")
	}
	changed = options
	changed.Binding = options.Binding
	changed.Binding.EndpointConfigurationSHA256 = strings.Repeat("0", 64)
	if err := verifyCapacityBinding(ctx, changed, providers); err == nil {
		t.Fatal("changed endpoint digest accepted")
	}
}

func capacityFileDigest(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func capacityBoundOptions(t *testing.T, live *CommittedSource) CapacityOptions {
	t.Helper()
	ctx := context.Background()
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	revision, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	boundDocument := func(name string) CapacityBoundFile {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		if writeErr := os.WriteFile(path, []byte(name), 0600); writeErr != nil {
			t.Fatal(writeErr)
		}
		return CapacityBoundFile{Path: path, SHA256: capacityFileDigest(t, path)}
	}
	options := CapacityOptions{SourceRoot: root, ApprovalDocument: boundDocument("approval"), InterfaceDocument: boundDocument("interface")}
	files := []CapacityBoundFile{}
	for _, directory := range []string{"services/launchpad/indexer/internal/source", "services/launchpad/indexer/internal/processor", "services/launchpad/indexer/internal/app", "services/launchpad/indexer/internal/store", "services/launchpad/indexer/internal/receipt"} {
		entries, readErr := os.ReadDir(filepath.Join(root, directory))
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
				path := filepath.Join(directory, entry.Name())
				files = append(files, CapacityBoundFile{Path: path, SHA256: capacityFileDigest(t, filepath.Join(root, path))})
			}
		}
	}
	for _, path := range []string{"services/launchpad/indexer/go.mod", "services/launchpad/indexer/go.sum"} {
		files = append(files, CapacityBoundFile{Path: path, SHA256: capacityFileDigest(t, filepath.Join(root, path))})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	sourceDigest, err := capacityHash(files)
	if err != nil {
		t.Fatal(err)
	}
	endpointDigest, err := capacityHash(live.opts.Endpoints)
	if err != nil {
		t.Fatal(err)
	}
	options.Binding = CapacityBinding{Revision: strings.TrimSpace(string(revision)), SourceSHA256: sourceDigest, InterfaceSHA256: options.InterfaceDocument.SHA256, ApprovalSHA256: options.ApprovalDocument.SHA256, ManifestDigest: live.opts.ManifestDigest, EndpointConfigurationSHA256: endpointDigest, Files: files}
	return options
}
