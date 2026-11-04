package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func repair74Archive(t *testing.T) ([]store.ArchiveManifest, []byte, []accounting.FillV1, string) {
	t.Helper()
	raw, err := os.ReadFile("testdata/repair74/manifests.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifests []store.ArchiveManifest
	if err = json.Unmarshal(raw, &manifests); err != nil || len(manifests) != 1 {
		t.Fatal("committed manifests", err)
	}
	payload, err := os.ReadFile("testdata/repair74/segment.json")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != "542512d9bb2586a22c852cf134cc11da5214d3ac5b0391f83437571650fca2aa" {
		t.Fatal("committed archive bytes changed")
	}
	fills, err := store.ValidateArchive(payload, manifests[0])
	if err != nil || len(fills) != 3 {
		t.Fatal("real archive codec", err)
	}
	digests := []string{}
	for _, manifest := range manifests {
		wire, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(wire)
		digests = append(digests, hex.EncodeToString(sum[:]))
	}
	wire, err := json.Marshal(digests)
	if err != nil {
		t.Fatal(err)
	}
	sum = sha256.Sum256(wire)
	return manifests, payload, fills, hex.EncodeToString(sum[:])
}

func repair74JSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func repair74Seed(t *testing.T) (retainedReconstructionCheckpoint, []QualifiedCandle, []accounting.FillV1) {
	t.Helper()
	_, _, fills, manifest := repair74Archive(t)
	from, err := BucketStart(1577923201, "1w")
	if err != nil {
		t.Fatal(err)
	}
	to := from + 604800
	// The live producer folds independently of the reconstruction algorithm.
	bars := []QualifiedCandle{}
	for _, interval := range ExactIntervals {
		buckets := map[int64]*QualifiedCandle{}
		actors := map[int64]map[string]bool{}
		for _, fill := range fills {
			copied, err := QualifiedFill(fill)
			if err != nil {
				t.Fatal(err)
			}
			start, err := BucketStart(copied.BlockTimestamp, interval)
			if err != nil {
				t.Fatal(err)
			}
			if actors[start] == nil {
				actors[start] = map[string]bool{}
			}
			fresh := copied.Sender != "" && !actors[start][copied.Sender]
			if copied.Sender != "" {
				actors[start][copied.Sender] = true
			}
			next, err := FoldQualified(buckets[start], fill, interval, fresh)
			if err != nil {
				t.Fatal(err)
			}
			buckets[start] = &next
		}
		for _, bar := range buckets {
			bars = append(bars, *bar)
		}
	}
	digest, err := projectionDigest(bars)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := retainedReconstructionCheckpoint{Pool: fills[0].PoolAddress, From: from, To: to, Expected: digest, Output: digest, Manifest: manifest, RetainedManifest: manifest, Epoch: 2, Status: "promoted"}
	return checkpoint, bars, fills
}

func repair74Reopen(t *testing.T, path string) []byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	return raw
}

func repair74Retain(t *testing.T, path string, checkpoint retainedReconstructionCheckpoint) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(repair74JSON(t, checkpoint)); err != nil {
		t.Fatal(err)
	}
	if err = file.Sync(); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRepair_74_RetainedParitySurvivesRestart(t *testing.T) {
	checkpoint, live, fills := repair74Seed(t)
	dir := t.TempDir()
	retained := filepath.Join(dir, "retained.json")
	repair74Retain(t, retained, checkpoint)
	projectionPath := filepath.Join(dir, "projection.json")
	if err := os.WriteFile(projectionPath, repair74JSON(t, live), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(projectionPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(projectionPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("projection was not deleted", err)
	}
	// A damaged surviving projection is deliberately different from its commitment.
	live[0].VolumeUsdlRaw = "999999"
	damaged, err := projectionDigest(live)
	if err != nil || damaged == checkpoint.Expected {
		t.Fatal("damage did not alter economic parity", err)
	}
	live = nil
	expected, err := decodeReconstructionCheckpoint(repair74Reopen(t, retained), checkpoint.Pool, checkpoint.From, checkpoint.To, checkpoint.Manifest, checkpoint.Epoch)
	if err != nil || expected != checkpoint.Expected {
		t.Fatal("lost independent commitment", expected, err)
	}
	recovered, err := buildReconstructionProjection(fills, checkpoint.From, checkpoint.To, SeriesCheckpoint{Height: 103, Timestamp: checkpoint.To}, time.Unix(checkpoint.To+1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyReconstructionParity(expected, recovered.Digest); err != nil {
		t.Fatal(err)
	}
	if len(recovered.Bars) != 8 || len(recovered.Candles) != 8 || len(recovered.Traders) != 7 {
		t.Fatal("lost intervals or actor membership", len(recovered.Bars), len(recovered.Traders))
	}
	if err = os.WriteFile(projectionPath, repair74JSON(t, recovered.Candles), 0600); err != nil {
		t.Fatal(err)
	}
	var snapshots []store.StagedCandle
	if err = json.Unmarshal(repair74Reopen(t, projectionPath), &snapshots); err != nil {
		t.Fatal(err)
	}
	recoveredBars := []QualifiedCandle{}
	counts := map[string]int{}
	rawSnapshots := make([]json.RawMessage, 0, len(snapshots))
	for _, row := range snapshots {
		rawSnapshots = append(rawSnapshots, row.Snapshot)
	}
	consumed, err := decodeSeriesSnapshots(rawSnapshots)
	if err != nil || len(consumed) != len(snapshots) {
		t.Fatal("series consumer", err)
	}
	for i, row := range snapshots {
		bar := consumed[i]
		if bar.Synthetic || bar.AlgorithmVersion != SeriesAlgorithm || bar.CorrectionReason != "archive_reconstruction" || bar.Interval != row.Interval || bar.Start != row.Start || bar.SourceWatermarkHeight != 103 || bar.CompletenessStatus != "complete_through_watermark" {
			t.Fatal("invalid consumer snapshot", bar)
		}
		counts[bar.Interval]++
		recoveredBars = append(recoveredBars, bar.QualifiedCandle)
	}
	for _, interval := range ExactIntervals {
		want := 1
		if interval == "1m" {
			want = 2
		}
		if counts[interval] != want {
			t.Fatal("missing interval", interval, counts[interval])
		}
	}
	actual, err := projectionDigest(recoveredBars)
	if err != nil || actual != expected {
		t.Fatal("economic data not restored", actual, err)
	}
	t.Log("finding 21: retained checkpoint codec survives deletion and file close/reopen independently of the damaged projection; shared reconstruction restores all intervals")
}

func TestRepair_74_RejectsInvalidRetainedAuthority(t *testing.T) {
	checkpoint, _, _ := repair74Seed(t)
	cases := []struct {
		name  string
		alter func(*retainedReconstructionCheckpoint)
	}{
		{"unequal_output", func(c *retainedReconstructionCheckpoint) { c.Output = strings.Repeat("b", 64) }},
		{"invalid_digest", func(c *retainedReconstructionCheckpoint) { c.Expected = "bad"; c.Output = "bad" }},
		{"uppercase_digest", func(c *retainedReconstructionCheckpoint) {
			c.Expected = strings.ToUpper(c.Expected)
			c.Output = c.Expected
		}},
		{"different_manifest", func(c *retainedReconstructionCheckpoint) { c.Manifest = strings.Repeat("b", 64) }},
		{"unpromoted_run", func(c *retainedReconstructionCheckpoint) { c.Status = "validated" }},
		{"different_range", func(c *retainedReconstructionCheckpoint) { c.From += 60 }},
		{"different_range_end", func(c *retainedReconstructionCheckpoint) { c.To -= 60 }},
		{"different_pool", func(c *retainedReconstructionCheckpoint) { c.Pool = "0x" + strings.Repeat("a", 40) }},
		{"missing_range_checkpoint", func(c *retainedReconstructionCheckpoint) { c.RetainedManifest = "" }},
		{"different_checkpoint_manifest", func(c *retainedReconstructionCheckpoint) { c.RetainedManifest = strings.Repeat("b", 64) }},
		{"advanced_series", func(c *retainedReconstructionCheckpoint) { c.Epoch-- }},
		{"zero_epoch", func(c *retainedReconstructionCheckpoint) { c.Epoch = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			invalid := checkpoint
			tc.alter(&invalid)
			if _, err := decodeReconstructionCheckpoint(repair74JSON(t, invalid), checkpoint.Pool, checkpoint.From, checkpoint.To, checkpoint.Manifest, checkpoint.Epoch); !errors.Is(err, ErrReconstructionCheckpoint) {
				t.Fatal("invalid retained authority accepted", err)
			}
		})
	}
	valid := repair74JSON(t, checkpoint)
	for _, wire := range [][]byte{nil, []byte("{}"), []byte("null"), append(append([]byte(nil), valid...), valid...), append(append([]byte(nil), valid[:len(valid)-1]...), []byte(",\"unknown\":true}")...), bytes.Repeat([]byte("x"), 4097)} {
		if _, err := decodeReconstructionCheckpoint(wire, checkpoint.Pool, checkpoint.From, checkpoint.To, checkpoint.Manifest, checkpoint.Epoch); !errors.Is(err, ErrReconstructionCheckpoint) {
			t.Fatal("malformed checkpoint accepted", err)
		}
	}
	t.Log("finding 21: checksum, manifest, pool, exact range, promotion and epoch guards preserved")
}

func TestRepair_74_DeleteCloseReopenReconstructArchive(t *testing.T) {
	checkpoint, live, _ := repair74Seed(t)
	dir := t.TempDir()
	retained := filepath.Join(dir, "checkpoint.json")
	repair74Retain(t, retained, checkpoint)
	// Retention only proceeds after the independent commitment and floor qualify.
	expected, err := decodeReconstructionCheckpoint(repair74Reopen(t, retained), checkpoint.Pool, checkpoint.From, checkpoint.To, checkpoint.Manifest, checkpoint.Epoch)
	if err != nil {
		t.Fatal(err)
	}
	floor, err := store.RetentionFloor(time.Unix(checkpoint.To+366*86400, 0), "1m")
	if err != nil || checkpoint.To > floor.Unix() {
		t.Fatal("retention floor", err)
	}
	path := filepath.Join(dir, "bars.json")
	if err = os.WriteFile(path, repair74JSON(t, live), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	live = nil
	// Reopen both transports; there is no live projection available to checksum.
	manifests, payload, fills, _ := repair74Archive(t)
	if _, err = os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("retained-away projection survived", err)
	}
	expected, err = decodeReconstructionCheckpoint(repair74Reopen(t, retained), checkpoint.Pool, checkpoint.From, checkpoint.To, checkpoint.Manifest, checkpoint.Epoch)
	if err != nil {
		t.Fatal(err)
	}
	watermark := SeriesCheckpoint{Height: 103, Timestamp: checkpoint.To}
	now := time.Unix(checkpoint.To+1, 0)
	recovered, err := buildReconstructionProjection(fills, checkpoint.From, checkpoint.To, watermark, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyReconstructionParity(expected, recovered.Digest); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, repair74JSON(t, recovered.Candles), 0600); err != nil {
		t.Fatal(err)
	}
	if len(repair74Reopen(t, path)) == 0 || len(recovered.Bars) != 8 {
		t.Fatal("archive did not restore nonempty history")
	}
	// Equal re-emission produces exactly the same staged content after restart.
	repeated, err := buildReconstructionProjection(fills, checkpoint.From, checkpoint.To, watermark, now)
	if err != nil || !reflect.DeepEqual(recovered.Candles, repeated.Candles) || !reflect.DeepEqual(recovered.Traders, repeated.Traders) {
		t.Fatal("non-idempotent reconstruction content", err)
	}
	corrupt := append([]byte(nil), payload...)
	corrupt[len(corrupt)/2] ^= 1
	if accepted, err := store.ValidateArchive(corrupt, manifests[0]); err == nil || accepted != nil {
		t.Fatal("corrupt archive accepted", err)
	}
	altered := append([]accounting.FillV1(nil), fills...)
	altered[0].USDLAmountRaw = "9000000"
	altered[0].PaidInputRaw = "9000000"
	wrong, err := buildReconstructionProjection(altered, checkpoint.From, checkpoint.To, watermark, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyReconstructionParity(expected, wrong.Digest); err == nil || !strings.Contains(err.Error(), "reconstruction_economic_checksum_mismatch") {
		t.Fatal("changed economics bypassed retained authority", err)
	}
	empty, err := buildReconstructionProjection(nil, checkpoint.From, checkpoint.To, watermark, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyReconstructionParity(expected, empty.Digest); err == nil {
		t.Fatal("empty reconstruction bypassed nonempty commitment")
	}
	// An internally consistent, but wrong, retained digest must still reject output.
	checkpoint.Expected = strings.Repeat("b", 64)
	checkpoint.Output = checkpoint.Expected
	wrongExpected, err := decodeReconstructionCheckpoint(repair74JSON(t, checkpoint), checkpoint.Pool, checkpoint.From, checkpoint.To, checkpoint.Manifest, checkpoint.Epoch)
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyReconstructionParity(wrongExpected, recovered.Digest); err == nil {
		t.Fatal("authentic output replaced retained checksum authority")
	}
	t.Log("finding 21: committed archive codec -> shared reconstruction -> staged snapshot consumer survives file deletion/reopen; corrupt bytes and wrong economic checksums are refused")
}
