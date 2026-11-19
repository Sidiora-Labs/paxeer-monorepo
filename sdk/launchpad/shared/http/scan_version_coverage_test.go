package http

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func scannerServiceFixture(t *testing.T) (string, int, string, string) {
	t.Helper()
	host := os.Getenv("CLAMAV_HOST")
	port, err := strconv.Atoi(os.Getenv("CLAMAV_PORT"))
	if host == "" || err != nil || port < 1 || port > 65535 {
		t.Fatal("CLAMAV_HOST and CLAMAV_PORT must name a running scanner")
	}
	connection, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(connection, "nVERSION\n"); err != nil {
		t.Fatal(err)
	}
	version, err := bufio.NewReader(io.LimitReader(connection, 1025)).ReadString('\n')
	if err != nil || len(version) > 1024 {
		t.Fatalf("scanner version: %q: %v", version, err)
	}
	parts := strings.Split(strings.TrimSuffix(version, "\n"), "/")
	if len(parts) != 3 || !strings.HasPrefix(parts[0], "ClamAV ") || strings.TrimSpace(parts[2]) == "" {
		t.Fatalf("scanner version: %q", version)
	}
	revision, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil || revision == 0 {
		t.Fatalf("scanner signature revision: %q: %v", parts[1], err)
	}
	return host, port, parts[0], parts[1]
}

func TestScannerVersionBindsCleanScan(t *testing.T) {
	host, port, engine, revision := scannerServiceFixture(t)
	opts, err := ScannerVersion(context.Background(), ScanOptions{Host: host, Port: port, Timeout: time.Second})
	if err != nil || opts.Engine != engine || opts.SignatureRevision != revision || opts.Host != host || opts.Port != port {
		t.Fatalf("scanner authority: %+v, %v", opts, err)
	}
	payload := []byte("clean payload")
	result := ScanVerifiedBuffer(context.Background(), payload, ScanOptions{Host: host, Port: port, Timeout: time.Second})
	digest := sha256.Sum256(payload)
	if !result.Clean || result.Verdict != ScanClean || result.ContentHash != hex.EncodeToString(digest[:]) || result.Size != int64(len(payload)) || result.Engine != opts.Engine || result.SignatureRevision != opts.SignatureRevision {
		t.Fatalf("version-bound clean scan: %+v", result)
	}
	infected := ScanVerifiedBuffer(context.Background(), []byte(`X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`), ScanOptions{Host: host, Port: port, Timeout: time.Second})
	if infected.Clean || infected.Verdict != ScanInfected || infected.Reason != "malware-detected" {
		t.Fatalf("signature verdict: %+v", infected)
	}
	t.Setenv("CLAMAV_HOST", host)
	t.Setenv("CLAMAV_PORT", strconv.Itoa(port))
	if configured, err := ScannerVersion(context.Background(), ScanOptions{}); err != nil || configured.Engine != engine || configured.SignatureRevision != revision {
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
