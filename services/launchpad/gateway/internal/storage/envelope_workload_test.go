package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestEnvelopeConcurrentUploadAndReadWorkload(t *testing.T) {
	body := make([]byte, 6<<20)
	for i := range body {
		body[i] = byte(i)
	}
	fixture := envelopeTestFile(t, "upload.bin", body)
	videoPath := filepath.Join(t.TempDir(), "owned-playback.mp4")
	generate := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=red:s=16x16:r=10", "-t", "2", "-c:v", "mpeg4", "-movflags", "+faststart", "-y", videoPath)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("generate owned playback bytes: %v: %s", err, output)
	}
	readBody, err := os.ReadFile(videoPath)
	if err != nil {
		t.Fatal(err)
	}
	readDigest := sha256.Sum256(readBody)
	var uploads, reads, metrics atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/upload":
			prefix := make([]byte, 32768)
			if _, err := io.ReadFull(r.Body, prefix); err != nil || r.ContentLength != int64(len(body)) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			uploads.Add(1)
			time.Sleep(4 * time.Second)
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
		case "/read":
			reads.Add(1)
			_, _ = w.Write(readBody)
		case "/metrics":
			metrics.Add(1)
			_, _ = io.WriteString(w, "process_resident_memory_bytes 1048576\ngo_goroutines 8\n")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	config := EnvelopeConfig{
		DurationSeconds:  8,
		ReadWorkers:      3,
		ResourceURL:      server.URL + "/metrics",
		MaxResidentBytes: 2 << 20,
		MaxGoroutines:    16,
		Release: MediaReleaseInputs{
			FixtureRoot: filepath.Dir(fixture.Path),
			Fixtures:    []LicensedFixture{{Path: "upload.bin", SHA256: fixture.SHA256, Ownership: "owned"}},
		},
		PlaybackBinary: EnvelopeFile{Path: filepath.Join(t.TempDir(), "absent-player"), SHA256: strings.Repeat("0", 64)},
	}
	for i := 0; i < 200; i++ {
		config.SlowUploads = append(config.SlowUploads, EnvelopeTarget{ID: fmt.Sprintf("upload-%d", i), Class: uploadKind, URL: server.URL + "/upload", ExpectedStatus: http.StatusTooManyRequests, FixturePath: "upload.bin"})
	}
	for _, class := range []string{originServiceName, metadataServiceName, cdnServiceName} {
		config.Targets = append(config.Targets, EnvelopeTarget{ID: class, Class: class, URL: server.URL + "/read", ExpectedStatus: http.StatusOK, ExpectedSHA256: hex.EncodeToString(readDigest[:])})
	}
	runner := envelopeTestRunner(t, server, config)
	runner.allocation = EnvelopeAllocation{OriginRPS: 2, MetadataRPS: 2, CDNRPS: 2}
	err = runner.run(context.Background())
	if err == nil || err.Error() != "evidence_source_missing" {
		t.Fatalf("workload terminal playback file: %v", err)
	}
	if runner.result.HeldUploadConnections != 200 || uploads.Load() != 200 {
		t.Fatalf("held uploads=%d received=%d", runner.result.HeldUploadConnections, uploads.Load())
	}
	if reads.Load() < 3 || metrics.Load() == 0 || runner.result.MaximumResidentBytes != 1048576 || runner.result.MaximumGoroutines != 8 {
		t.Fatalf("reads=%d resources=%d memory=%d goroutines=%d", reads.Load(), metrics.Load(), runner.result.MaximumResidentBytes, runner.result.MaximumGoroutines)
	}
	if distributions := runner.aggregate.SnapshotDistributions(); distributions[uploadKind].Count != 200 || distributions[uploadKind].Failed != 200 || distributions[uploadKind].Statuses[http.StatusTooManyRequests] != 200 || runner.writeError != nil {
		t.Fatalf("upload distributions=%v write error=%v", distributions, runner.writeError)
	}
	program, err := os.ReadFile("/usr/bin/ffmpeg")
	if err != nil {
		t.Fatal(err)
	}
	programHash := sha256.Sum256(program)
	caPath := filepath.Join(t.TempDir(), "playback-root.pem")
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	config.PlaybackBinary = EnvelopeFile{Path: "/usr/bin/ffmpeg", SHA256: hex.EncodeToString(programHash[:])}
	config.PlaybackSeconds = 1
	config.PlaybackTargetID = cdnServiceName
	config.TLSRootCA = caPath
	measured := envelopeTestRunner(t, server, config)
	measured.allocation = runner.allocation
	if err := measured.run(context.Background()); err != nil {
		t.Fatalf("licensed playback workload: %v", err)
	}
	if measured.result.HeldUploadConnections != 200 || measured.result.ScenarioResults[5] == "" || measured.writeError != nil || reads.Load() < 6 || metrics.Load() < 2 {
		t.Fatalf("licensed playback: held=%d scenario=%q writes=%v reads=%d resources=%d", measured.result.HeldUploadConnections, measured.result.ScenarioResults[5], measured.writeError, reads.Load(), metrics.Load())
	}
}
