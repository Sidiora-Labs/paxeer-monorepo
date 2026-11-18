package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func envelopePlaybackServer(t *testing.T, handler http.Handler) (*httptest.Server, string) {
	t.Helper()
	pki := testpki.New(t)
	leaf := pki.Leaf("cdn", "localhost")
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{leaf.Pair}}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server, pki.CAFile
}

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
	server, ca := envelopePlaybackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie.mp4" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.ServeContent(w, r, "movie.mp4", time.Now(), bytes.NewReader(video))
	}))
	defer server.Close()
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
