package ranker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
)

func boundMeasurementFile(t *testing.T, path string, raw []byte) BoundFile {
	t.Helper()
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return BoundFile{Path: path, SHA256: hex.EncodeToString(sum[:])}
}

func TestMeasurementSourceChecksBoundFiles(t *testing.T) {
	root := t.TempDir()
	leaf := boundMeasurementFile(t, filepath.Join(root, "source.go"), []byte("package sample\n"))
	manifestBytes, err := json.Marshal(map[string]string{"source.go": leaf.SHA256})
	if err != nil {
		t.Fatal(err)
	}
	manifest := boundMeasurementFile(t, filepath.Join(root, "manifest.json"), manifestBytes)
	binding := MeasurementBinding{Candidate: manifest.SHA256, Interface: leaf.SHA256, Topology: leaf.SHA256, Formula: leaf.SHA256, Image: leaf.SHA256, Workload: leaf.SHA256}
	if err := binding.Validate(); err != nil {
		t.Fatal(err)
	}
	files, err := ValidateMeasurementSource(root, manifest, binding)
	if err != nil || files["source.go"] != leaf.SHA256 || len(files) != 1 {
		t.Fatalf("validated sources = %v, %v", files, err)
	}
	if raw, err := ReadBoundFile(leaf, int64(len("package sample\n"))); err != nil || string(raw) != "package sample\n" {
		t.Fatalf("bound source = %q, %v", raw, err)
	}
	if _, err := ReadBoundFile(leaf, 1); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("oversized source: %v", err)
	}
	if _, err := ReadBoundFile(BoundFile{Path: leaf.Path, SHA256: manifest.SHA256}, 100); err == nil {
		t.Fatal("changed source checksum accepted")
	}
	link := filepath.Join(root, "link.go")
	if err := os.Symlink(leaf.Path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBoundFile(BoundFile{Path: link, SHA256: leaf.SHA256}, 100); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("symbolic link accepted: %v", err)
	}
	invalid := binding
	invalid.Image = "missing"
	if _, err := ValidateMeasurementSource(root, manifest, invalid); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("incomplete binding accepted: %v", err)
	}
	invalid = binding
	invalid.Candidate = leaf.SHA256
	if _, err := ValidateMeasurementSource(root, manifest, invalid); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unbound manifest accepted: %v", err)
	}
	for _, name := range []string{"../source.go", "/source.go"} {
		raw, err := json.Marshal(map[string]string{name: leaf.SHA256})
		if err != nil {
			t.Fatal(err)
		}
		bad := boundMeasurementFile(t, filepath.Join(root, "invalid.json"), raw)
		invalid = binding
		invalid.Candidate = bad.SHA256
		if _, err := ValidateMeasurementSource(root, bad, invalid); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("source path %q accepted: %v", name, err)
		}
	}
}

func TestLatencyHistogramQuantiles(t *testing.T) {
	h := NewLatencyHistogram()
	if _, err := h.QuantileUpper(50); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("empty histogram quantile: %v", err)
	}
	for _, duration := range []time.Duration{time.Nanosecond, time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond} {
		if err := h.Observe(duration); err != nil {
			t.Fatal(err)
		}
	}
	if h.Count != 4 || h.Maximum != int64(3*time.Millisecond) {
		t.Fatalf("histogram count and maximum = %d, %d", h.Count, h.Maximum)
	}
	for _, tc := range []struct {
		percent int
		want    int64
	}{{25, int64(time.Millisecond)}, {50, int64(time.Millisecond)}, {75, int64(2 * time.Millisecond)}, {100, int64(3 * time.Millisecond)}} {
		got, err := h.QuantileUpper(tc.percent)
		if err != nil || got != tc.want {
			t.Fatalf("quantile %d = %d, %v; want %d", tc.percent, got, err, tc.want)
		}
	}
	if err := h.Observe(-time.Nanosecond); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("negative duration accepted: %v", err)
	}
	if _, err := h.QuantileUpper(101); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("invalid percentile accepted: %v", err)
	}
}

func TestHistoricalReplayMatchesBoundSnapshot(t *testing.T) {
	in := fixture(2)
	in.Provenance = &Provenance{
		Watermark: v1.Watermark{
			BlockHash: "0x" + strings.Repeat("a", 64), ChainId: "125", Height: "125",
			Integrity: "healthy", ObservedAt: time.Unix(in.CutoffTimeUTC, 0).UTC().Format("2006-01-02T15:04:05.000Z"),
			ProjectionRevision: "1",
		},
		UniverseDigest: CanonicalUniverse(in.Candidates), StatsRevision: "1", ModerationRevision: "1",
		ActivityRevision: "1", WindowStart: 1, WindowEnd: in.CutoffTimeUTC,
	}
	snapshot, err := BuildSnapshot(in, "trending", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	file := boundMeasurementFile(t, filepath.Join(t.TempDir(), "snapshot.json"), raw)
	result, err := MeasureHistoricalReplay(HistoricalReplaySample{Cutoff: in.CutoffTimeUTC, Category: "trending", Original: file})
	if err != nil || !result.Equal || result.InputDigest != snapshot.InputDigest() || result.ActualOutputDigest != snapshot.OutputDigest() {
		t.Fatalf("historical replay = %+v, %v", result, err)
	}
	if _, err := MeasureHistoricalReplay(HistoricalReplaySample{Cutoff: in.CutoffTimeUTC + 1, Category: "trending", Original: file}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("different cutoff accepted: %v", err)
	}
	trending := snapshot
	movers, err := BuildSnapshot(in, "movers", &trending)
	if err != nil {
		t.Fatal(err)
	}
	moversRaw, err := json.Marshal(movers)
	if err != nil {
		t.Fatal(err)
	}
	moversFile := boundMeasurementFile(t, filepath.Join(t.TempDir(), "movers.json"), moversRaw)
	moversResult, err := MeasureHistoricalReplay(HistoricalReplaySample{Cutoff: in.CutoffTimeUTC, Category: "movers", Original: moversFile})
	if err != nil || !moversResult.Equal || moversResult.ActualOutputDigest != movers.OutputDigest() {
		t.Fatalf("historical movers replay = %+v, %v", moversResult, err)
	}
}

func TestSnapshotMetadataAndHeaderIdentity(t *testing.T) {
	in := fixture(2)
	in.Provenance = &Provenance{StatsRevision: "source-1"}
	snapshot, err := BuildSnapshot(in, "trending", nil)
	if err != nil {
		t.Fatal(err)
	}
	metadata := snapshot.Metadata()
	if len(metadata.Candidates) != 0 || metadata.RunID != in.RunID || metadata.Provenance == nil || metadata.Provenance.StatsRevision != "source-1" {
		t.Fatalf("snapshot metadata = %+v", metadata)
	}
	metadata.Provenance.StatsRevision = "other"
	if snapshot.Metadata().Provenance.StatsRevision != "source-1" {
		t.Fatal("snapshot provenance changed through metadata")
	}
	h := Header{RunID: snapshot.RunID(), Category: snapshot.Category(), InputDigest: snapshot.InputDigest(), OutputDigest: snapshot.OutputDigest(), AlgorithmVersion: AlgorithmVersion, CutoffTime: snapshot.CutoffTime(), CompletedAt: time.Unix(1001, 0), Revision: 1, Total: int64(len(snapshot.Entries()))}
	if err := h.Validate(); err != nil {
		t.Fatal(err)
	}
	policy, err := Policy(h.Category)
	if err != nil {
		t.Fatal(err)
	}
	if h.Stale(time.Unix(h.CutoffTime, 0)) || !h.Stale(time.Unix(h.CutoffTime+policy.FreshnessSeconds+1, 0)) {
		t.Fatal("header freshness window changed")
	}
	if h.ETag() == "" {
		t.Fatal("empty header entity tag")
	}
	changed := h
	changed.OutputDigest = strings.Repeat("0", 64)
	if changed.ETag() == h.ETag() {
		t.Fatal("changed output retained the header entity tag")
	}
	if _, err := NewCoordinator(nil, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("coordinator accepted missing dependencies: %v", err)
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	if err := (&Coordinator{}).Run(canceled, false, func(error) {}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled coordinator run: %v", err)
	}
}

func TestComparisonArtifactRejectsChangedRankingData(t *testing.T) {
	snapshot, err := BuildSnapshot(fixture(2), "trending", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"unknown field", func(artifact map[string]any) { artifact["unknown"] = true }},
		{"category", func(artifact map[string]any) { artifact["category"] = "unknown" }},
		{"input digest", func(artifact map[string]any) { artifact["inputDigest"] = strings.Repeat("0", 64) }},
		{"duplicate entry", func(artifact map[string]any) {
			entries := artifact["entries"].([]any)
			entries[1].(map[string]any)["Address"] = entries[0].(map[string]any)["Address"]
			artifact["outputDigest"] = digest(entries)
		}},
		{"negative score", func(artifact map[string]any) {
			entries := artifact["entries"].([]any)
			entries[0].(map[string]any)["Score"] = float64(-1)
			artifact["outputDigest"] = digest(entries)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var artifact map[string]any
			if err := json.Unmarshal(raw, &artifact); err != nil {
				t.Fatal(err)
			}
			tc.change(artifact)
			changed, err := json.Marshal(artifact)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(changed)
			if _, err := ReadComparisonArtifact(changed, hex.EncodeToString(sum[:])); err == nil {
				t.Fatal("changed comparison artifact accepted")
			}
		})
	}
	trailing := append(append([]byte(nil), raw...), []byte(" {}")...)
	sum := sha256.Sum256(trailing)
	if _, err := ReadComparisonArtifact(trailing, hex.EncodeToString(sum[:])); err == nil {
		t.Fatal("trailing artifact value accepted")
	}
}

func TestComparisonArtifactRejectsInconsistentMembership(t *testing.T) {
	snapshot, err := BuildSnapshot(fixture(2), "trending", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*InputSnapshot, *[]Scored, *[]Exclusion)
	}{
		{"duplicate candidate", func(in *InputSnapshot, _ *[]Scored, _ *[]Exclusion) {
			in.Candidates[1].Pool.PoolAddress = in.Candidates[0].Pool.PoolAddress
		}},
		{"unrecognized entry", func(_ *InputSnapshot, entries *[]Scored, _ *[]Exclusion) {
			(*entries)[0].Address = "0x" + strings.Repeat("f", 40)
		}},
		{"oversized entry score", func(_ *InputSnapshot, entries *[]Scored, _ *[]Exclusion) {
			(*entries)[0].Score = 1e13
		}},
		{"unexplained exclusion", func(_ *InputSnapshot, entries *[]Scored, excluded *[]Exclusion) {
			*excluded = append(*excluded, Exclusion{Address: (*entries)[0].Address})
			*entries = (*entries)[1:]
		}},
		{"unrecognized exclusion", func(_ *InputSnapshot, entries *[]Scored, excluded *[]Exclusion) {
			*excluded = append(*excluded, Exclusion{Address: "0x" + strings.Repeat("f", 40), Reason: "policy"})
			*entries = (*entries)[1:]
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var artifact struct {
				Input        InputSnapshot `json:"input"`
				Category     string        `json:"category"`
				Entries      []Scored      `json:"entries"`
				Excluded     []Exclusion   `json:"excluded"`
				InputDigest  string        `json:"inputDigest"`
				OutputDigest string        `json:"outputDigest"`
			}
			if err := json.Unmarshal(raw, &artifact); err != nil {
				t.Fatal(err)
			}
			tc.change(&artifact.Input, &artifact.Entries, &artifact.Excluded)
			artifact.InputDigest = digest(artifact.Input)
			artifact.OutputDigest = digest(artifact.Entries)
			changed, err := json.Marshal(artifact)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(changed)
			if _, err := ReadComparisonArtifact(changed, hex.EncodeToString(sum[:])); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("inconsistent comparison artifact accepted: %v", err)
			}
		})
	}
}

func TestSnapshotRejectsOversizedCandidateMetadata(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*InputSnapshot)
	}{
		{"depth", func(in *InputSnapshot) { in.Candidates[0].Depth = strings.Repeat("1", 513) }},
		{"amount", func(in *InputSnapshot) { in.Candidates[0].Pool.Volume24h = strings.Repeat("1", 513) }},
		{"algorithm", func(in *InputSnapshot) { in.AlgorithmVersion = "unknown" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := fixture(1)
			tc.change(&in)
			if _, err := BuildSnapshot(in, "trending", nil); err == nil {
				t.Fatal("invalid snapshot input accepted")
			}
		})
	}
}

func TestRankingArithmeticKeepsInvalidAmountsOutOfScores(t *testing.T) {
	for _, tc := range []struct {
		name        string
		numerator   string
		denominator string
	}{
		{"numerator", "invalid", "1"},
		{"denominator", "1", "invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := BoundedRatio(tc.numerator, tc.denominator, 10); err == nil {
				t.Fatal("invalid amount accepted in ranking ratio")
			}
		})
	}
	if ratio, err := BoundedRatio("100", "1", 10); err != nil || ratio != 10 {
		t.Fatalf("capped ranking ratio = %v, %v", ratio, err)
	}
	for _, tc := range []struct{ raw, self string }{{"invalid", "0"}, {"1", "invalid"}} {
		if _, err := AdjustActivity(tc.raw, tc.self, true, false, 1, 2); err == nil {
			t.Fatalf("invalid activity amounts %q, %q accepted", tc.raw, tc.self)
		}
	}
	if _, err := ReadBoundFile(BoundFile{Path: filepath.Join(t.TempDir(), "absent"), SHA256: strings.Repeat("a", 64)}, 100); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("absent bound source accepted: %v", err)
	}
}

func TestComparisonReportsRankingMovement(t *testing.T) {
	old, err := BuildSnapshot(fixture(101), "trending", nil)
	if err != nil {
		t.Fatal(err)
	}
	next := old
	next.entries = append([]Scored(nil), old.entries...)
	next.entries[0], next.entries[70] = next.entries[70], next.entries[0]
	next.entries[1], next.entries[20] = next.entries[20], next.entries[1]
	next.entries[2], next.entries[3] = next.entries[3], next.entries[2]
	identity := strings.Repeat("a", 64)
	report, err := Compare(old, next, 1, identity, identity, "reviewed")
	if err != nil {
		t.Fatal(err)
	}
	if report.Top100Overlap != 100 || report.MembershipChanges != 0 || report.Movement["1-10"] == 0 || report.Movement["11-50"] == 0 || report.Movement["51-99"] == 0 || !report.WithinReviewedThreshold || report.PromotionAllowed {
		t.Fatalf("comparison movement = %+v", report)
	}
}

func TestFiniteScoresBoundLargeAndIncompleteMetrics(t *testing.T) {
	base := fixture(1).Candidates[0].Pool
	for _, tc := range []struct {
		name     string
		category string
		change   func(*PoolStat)
	}{
		{"negative holders", "trending", func(p *PoolStat) { p.HolderCount = -1 }},
		{"zero baseline", "trending", func(p *PoolStat) { p.Volume24h = "0" }},
		{"invalid change", "trending", func(p *PoolStat) { p.PriceChange24h = "not-a-decimal" }},
		{"zero market cap", "movers", func(p *PoolStat) { p.MarketCap = "0" }},
		{"tiny market cap", "movers", func(p *PoolStat) { p.MarketCap = "0.0000000000001" }},
		{"zero short baseline", "unusual", func(p *PoolStat) { p.Volume1h = "0" }},
		{"unknown category", "unknown", func(*PoolStat) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := base
			tc.change(&pool)
			if _, err := finiteScore(tc.category, pool); err == nil {
				t.Fatal("incomplete finite score accepted")
			}
		})
	}
	pool := base
	pool.PriceChange24h = "1000000000001"
	if score, err := finiteScore("gainers", pool); err != nil || score != 1000000000000 {
		t.Fatalf("bounded gain = %v, %v", score, err)
	}
	pool.PriceChange24h = "-1000000000001"
	if score, err := finiteScore("losers", pool); err != nil || score != 1000000000000 {
		t.Fatalf("bounded loss = %v, %v", score, err)
	}
	pool = base
	pool.MarketCap = "10000000000000"
	if score, err := finiteScore("movers", pool); err != nil || score <= 0 {
		t.Fatalf("bounded market cap score = %v, %v", score, err)
	}
}

func TestSnapshotCursorRejectsChangedPageIdentity(t *testing.T) {
	if _, err := NewCursorCodec([]byte("short")); err == nil {
		t.Fatal("short cursor key accepted")
	}
	codec, err := NewCursorCodec([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := BuildSnapshot(fixture(3), "trending", nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(snapshot.CutoffTime(), 0)
	first, err := codec.Page(snapshot, 1, "", now)
	if err != nil || first.NextCursor == "" || len(first.Items) != 1 {
		t.Fatalf("first cursor page = %+v, %v", first, err)
	}
	if _, err := codec.Page(snapshot, 0, "", now); err == nil {
		t.Fatal("zero page limit accepted")
	}
	if _, err := codec.Page(snapshot, 1, "", now.Add(-time.Second)); err == nil {
		t.Fatal("page before frozen cutoff accepted")
	}
	if _, err := codec.Page(snapshot, 1, "", now.Add(900*time.Second)); !errors.Is(err, ErrCursorExpired) {
		t.Fatalf("expired page accepted: %v", err)
	}
	for _, token := range []string{strings.Repeat("x", 257), "!", first.NextCursor[:len(first.NextCursor)-1] + "!"} {
		if _, err := codec.Page(snapshot, 1, token, now); !errors.Is(err, ErrCursorInvalid) {
			t.Fatalf("changed cursor accepted: %v", err)
		}
	}
	other, err := BuildSnapshot(fixture(3), "volume", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := codec.Page(other, 1, first.NextCursor, now); !errors.Is(err, ErrCursorInvalid) {
		t.Fatalf("cursor from another category accepted: %v", err)
	}
	header := Header{RunID: snapshot.RunID(), Category: snapshot.Category(), InputDigest: snapshot.InputDigest(), OutputDigest: snapshot.OutputDigest(), AlgorithmVersion: AlgorithmVersion, CutoffTime: snapshot.CutoffTime(), CompletedAt: now, Revision: 1, Total: 3}
	last := RankedItem{Address: first.Items[0].Address, Score: "1", Rank: 1}
	publicCursor, err := codec.EncodePage(header, last)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := codec.ResolvePage("volume", publicCursor, now); !errors.Is(err, ErrCursorInvalid) {
		t.Fatalf("cross-category public cursor accepted: %v", err)
	}
	if _, err := codec.ResolvePage("trending", publicCursor, now.Add(900*time.Second)); !errors.Is(err, ErrCursorExpired) {
		t.Fatalf("expired public cursor accepted: %v", err)
	}
	last.Score = "NaN"
	if _, err := codec.EncodePage(header, last); !errors.Is(err, ErrCursorInvalid) {
		t.Fatalf("nondecimal public cursor accepted: %v", err)
	}
}
