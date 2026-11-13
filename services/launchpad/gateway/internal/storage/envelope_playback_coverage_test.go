package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestEnvelopePlaybackDecodesBoundMedia(t *testing.T) {
	const player = "/usr/bin/ffmpeg"
	program, err := os.ReadFile(player)
	if err != nil {
		t.Fatal(err)
	}
	programHash := sha256.Sum256(program)
	media := filepath.Join(t.TempDir(), "movie.mp4")
	command := exec.Command(player, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=64x64:rate=15", "-t", "2", "-c:v", "mpeg4", "-y", media)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("media generation: %v: %s", err, output)
	}
	video, err := os.ReadFile(media)
	if err != nil || len(video) == 0 {
		t.Fatalf("generated media: bytes=%d err=%v", len(video), err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie.mp4" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.ServeContent(w, r, "movie.mp4", time.Now(), bytes.NewReader(video))
	}))
	defer server.Close()
	ca := filepath.Join(t.TempDir(), "root.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	runner := envelopeTestRunner(t, server, EnvelopeConfig{
		PlaybackBinary:   EnvelopeFile{Path: player, SHA256: hex.EncodeToString(programHash[:])},
		PlaybackSeconds:  1,
		PlaybackTargetID: "cdn",
		TLSRootCA:        ca,
		Targets:          []EnvelopeTarget{{ID: "cdn", Class: cdnServiceName, URL: server.URL + "/movie.mp4", ExpectedStatus: http.StatusOK}},
	})
	if err := runner.playback(context.Background()); err != nil {
		t.Fatalf("bound media playback: %v", err)
	}
	if runner.writeError != nil {
		t.Fatalf("playback event: %v", runner.writeError)
	}
	if _, err := runner.events.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	events, err := io.ReadAll(runner.events)
	if err != nil || !bytes.Contains(events, []byte(`"cleanEnd":true`)) || !bytes.Contains(events, []byte(`"cdn_decoded_playback"`)) {
		t.Fatalf("decoded playback event: %s, %v", events, err)
	}
}
