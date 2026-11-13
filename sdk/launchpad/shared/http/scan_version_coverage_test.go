package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func scannerServiceFixture(t *testing.T) (string, int, string) {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{"COPYING": "Public domain test signatures.\n", "daily.ndb": "Test.Eicar:0:*:45494341522d5354414e444152442d414e544956495255532d544553542d46494c45\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	build := exec.Command("sigtool", "--build=daily.cud", "--unsigned", "--cvd-version=17", "--no-cdiff")
	build.Dir = root
	build.Stdin = strings.NewReader("test\n")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("signature database: %v %s", err, out)
	}
	// The compiled database contains the signature and its published revision.
	for _, name := range []string{"daily.ndb", "daily.info"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "clamd.conf")
	raw := fmt.Sprintf("Foreground yes\nTCPSocket %d\nTCPAddr 127.0.0.1\nDatabaseDirectory %s\nOfficialDatabaseOnly no\n", port, root)
	if err := os.WriteFile(config, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("clamd", "--config-file="+config)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	t.Cleanup(func() {
		_ = command.Process.Kill()
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			t.Error("scanner did not stop")
		}
	})
	deadline := time.Now().Add(15 * time.Second)
	for {
		connection, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 100*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			break
		}
		select {
		case err := <-exited:
			exited <- err
			t.Fatalf("scanner startup: %v %s", err, output.String())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("scanner startup deadline")
		}
		time.Sleep(20 * time.Millisecond)
	}
	version, err := exec.Command("clamd", "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	return "127.0.0.1", port, strings.TrimSpace(strings.SplitN(string(version), "/", 2)[0])
}

func TestScannerVersionBindsCleanScan(t *testing.T) {
	host, port, engine := scannerServiceFixture(t)
	opts, err := ScannerVersion(context.Background(), ScanOptions{Host: host, Port: port, Timeout: time.Second})
	if err != nil || opts.Engine != engine || opts.SignatureRevision != "17" || opts.Host != host || opts.Port != port {
		t.Fatalf("scanner authority: %+v, %v", opts, err)
	}
	payload := []byte("clean payload")
	result := ScanVerifiedBuffer(context.Background(), payload, ScanOptions{Host: host, Port: port, Timeout: time.Second})
	digest := sha256.Sum256(payload)
	if !result.Clean || result.Verdict != ScanClean || result.ContentHash != hex.EncodeToString(digest[:]) || result.Size != int64(len(payload)) || result.Engine != opts.Engine || result.SignatureRevision != opts.SignatureRevision {
		t.Fatalf("version-bound clean scan: %+v", result)
	}
	infected := ScanVerifiedBuffer(context.Background(), []byte("EICAR-STANDARD-ANTIVIRUS-TEST-FILE"), ScanOptions{Host: host, Port: port, Timeout: time.Second})
	if infected.Clean || infected.Verdict != ScanInfected || infected.Reason != "malware-detected" {
		t.Fatalf("signature verdict: %+v", infected)
	}
	t.Setenv("CLAMAV_HOST", host)
	t.Setenv("CLAMAV_PORT", strconv.Itoa(port))
	if configured, err := ScannerVersion(context.Background(), ScanOptions{}); err != nil || configured.Engine != engine || configured.SignatureRevision != "17" {
		t.Fatalf("configured scanner: %+v %v", configured, err)
	}
}

func TestScannerVersionRejectsUnboundReplies(t *testing.T) {
	t.Setenv("CLAMAV_HOST", "")
	for _, tc := range []struct{ name, reply string }{
		{"engine format", "ClamAV latest/17/2026-01-01\x00"},
		{"signature format", "ClamAV 1.4.2/0/2026-01-01\x00"},
		{"missing date", "ClamAV 1.4.2/17/\x00"},
		{"incomplete", "ClamAV 1.4.2/17/2026-01-01"},
		{"prefix", "Scanner 1.4.2/17/2026-01-01\x00"},
		{"layout", "ClamAV 1.4.2/17\x00"},
		{"byte limit", strings.Repeat("x", 1025) + "\x00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if engine, revision, err := parseScannerVersionReply(tc.reply); err == nil || engine != "" || revision != "" {
				t.Fatalf("unbound scanner version: %s %s %v", engine, revision, err)
			}
		})
	}
	for _, tc := range []struct {
		name string
		opts ScanOptions
	}{
		{"unconfigured", ScanOptions{}},
		{"invalid port", ScanOptions{Host: "127.0.0.1", Port: 65536}},
		{"unavailable", ScanOptions{Host: "127.0.0.1", Port: 1, Timeout: 100 * time.Millisecond}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CLAMAV_HOST", "")
			if opts, err := ScannerVersion(context.Background(), tc.opts); err == nil || opts.Engine != "" {
				t.Fatalf("unavailable scanner version: %+v, %v", opts, err)
			}
		})
	}
	if result := ScanVerifiedBuffer(context.Background(), []byte(strings.Repeat("x", 4)), ScanOptions{}); result.Clean || result.Verdict != ScanUnavailable || result.Reason != "clamav-version-unavailable" {
		t.Fatalf("scan without version: %+v", result)
	}
}
