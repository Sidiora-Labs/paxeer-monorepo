package cosmovisor_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/cosmovisor"
	"github.com/stretchr/testify/require"
)

// The child is this gate's own executable, with no shell or installed daemon.
func TestSupervisorLifecycleChildProcess(t *testing.T) {
	mode := ""
	for i, arg := range os.Args {
		if arg == "--supervisor-child" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
		}
	}
	if mode == "" {
		return
	}
	switch mode {
	case "normal":
		fmt.Fprintln(os.Stdout, "finished stdout")
		fmt.Fprintln(os.Stderr, "finished stderr")
	case "exit-error":
		os.Exit(7)
	case "stdout-upgrade", "stderr-upgrade", "error-upgrade":
		out, other := io.Writer(os.Stdout), io.Writer(os.Stderr)
		if mode == "stderr-upgrade" {
			out, other = other, out
		}
		fmt.Fprintln(other, "other stream finished")
		// Recorded x/upgrade/abci.go:134 message, with the logger's suffix.
		fmt.Fprintln(out, `ERR UPGRADE "v2" NEEDED at height: 123: {"binaries": {"any": "https://example.com/paxd"}} module=x/upgrade`)
		if mode == "error-upgrade" {
			os.Exit(7)
		}
	case "signals":
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGTERM)
		fmt.Fprintln(os.Stdout, "ready")
		<-sigs
		fmt.Fprintln(os.Stdout, "received first signal")
		<-sigs
		fmt.Fprintln(os.Stdout, "received second signal")
		signal.Stop(sigs)
	case "oversize":
		fmt.Fprintln(os.Stdout, strings.Repeat("x", 2*bufio.MaxScanTokenSize))
		time.Sleep(time.Hour)
	case "writer-error":
		fmt.Fprintln(os.Stdout, "cannot copy this output")
		time.Sleep(time.Hour)
	default:
		os.Exit(99)
	}
	os.Exit(0)
}

func supervisorConfig(t *testing.T) *cosmovisor.Config {
	t.Helper()
	cfg := &cosmovisor.Config{Home: t.TempDir(), Name: "paxd"}
	executable, err := os.Executable()
	require.NoError(t, err)
	for _, bin := range []string{cfg.GenesisBin(), cfg.UpgradeBin("v2")} {
		require.NoError(t, os.MkdirAll(filepath.Dir(bin), 0755))
		require.NoError(t, os.Symlink(executable, bin))
	}
	return cfg
}

func supervisorChildArgs(mode string) []string {
	return []string{"-test.run=^TestSupervisorLifecycleChildProcess$", "--", "--supervisor-child", mode}
}

type supervisorResult struct {
	upgrade bool
	err     error
}

func launchSupervisor(t *testing.T, cfg *cosmovisor.Config, mode string, out, errOut io.Writer) <-chan supervisorResult {
	t.Helper()
	result := make(chan supervisorResult, 1)
	go func() {
		upgrade, err := cosmovisor.LaunchProcess(cfg, supervisorChildArgs(mode), out, errOut)
		result <- supervisorResult{upgrade, err}
	}()
	return result
}

func awaitSupervisor(t *testing.T, result <-chan supervisorResult) supervisorResult {
	t.Helper()
	select {
	case res := <-result:
		return res
	case <-time.After(10 * time.Second):
		t.Fatal("supervisor did not finish and join its workers")
		return supervisorResult{}
	}
}

type blockedOutputWriter struct {
	bytes.Buffer
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (w *blockedOutputWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return w.Buffer.Write(p)
}

func TestSupervisorLifecycleFinalUpgrade(t *testing.T) {
	for _, mode := range []string{"stdout-upgrade", "stderr-upgrade", "error-upgrade"} {
		t.Run(mode, func(t *testing.T) {
			cfg := supervisorConfig(t)
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			out := &blockedOutputWriter{entered: make(chan struct{}), release: release}
			errOut := &blockedOutputWriter{entered: make(chan struct{}), release: release}
			result := launchSupervisor(t, cfg, mode, out, errOut)
			for _, entered := range []<-chan struct{}{out.entered, errOut.entered} {
				select {
				case <-entered:
				case <-time.After(10 * time.Second):
					t.Fatal("child output was not consumed")
				}
			}
			select {
			case res := <-result:
				t.Fatalf("returned while both output writers were still active: %+v", res)
			case <-time.After(100 * time.Millisecond):
			}
			unblock()
			res := awaitSupervisor(t, result)
			require.NoError(t, res.err)
			require.True(t, res.upgrade)
			require.Contains(t, out.String()+errOut.String(), `UPGRADE "v2"`)
			require.Contains(t, out.String()+errOut.String(), "other stream finished")
			// Reopen the supervisor's configuration and run its persisted selection.
			reopened := &cosmovisor.Config{Home: cfg.Home, Name: cfg.Name}
			current, err := reopened.CurrentBin()
			require.NoError(t, err)
			require.Equal(t, cfg.UpgradeBin("v2"), current)
			var nextOut, nextErr bytes.Buffer
			next := awaitSupervisor(t, launchSupervisor(t, reopened, "normal", &nextOut, &nextErr))
			require.NoError(t, next.err)
			require.False(t, next.upgrade)
			require.Equal(t, "finished stdout\n", nextOut.String())
			require.Equal(t, "finished stderr\n", nextErr.String())
		})
	}
}

func signalForwarderCount() int {
	stack := make([]byte, 1<<20)
	n := runtime.Stack(stack, true)
	return strings.Count(string(stack[:n]), "cosmovisor.forwardSignals(") +
		strings.Count(string(stack[:n]), "cosmovisor.LaunchProcess.func1(")
}

func TestSupervisorLifecycleSignalLifetime(t *testing.T) {
	cfg := supervisorConfig(t)
	before := signalForwarderCount()
	for i := 0; i < 12; i++ {
		res := awaitSupervisor(t, launchSupervisor(t, cfg, "normal", io.Discard, io.Discard))
		require.NoError(t, res.err)
		require.False(t, res.upgrade)
		require.Equal(t, before, signalForwarderCount(), "launch retained a signal forwarder")
	}
	// Protect the gate while proving that completed launches no longer forward.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)
	defer signal.Stop(sigs)
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))
	select {
	case <-sigs:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown signal was not delivered")
	}
	require.Equal(t, before, signalForwarderCount())
}

type signalOutputWriter struct{ bytes.Buffer }

func (w *signalOutputWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if bytes.Contains(p, []byte("ready\n")) || bytes.Contains(p, []byte("received first signal\n")) {
		if signalErr := syscall.Kill(os.Getpid(), syscall.SIGTERM); signalErr != nil {
			return n, signalErr
		}
	}
	return n, err
}

func TestSupervisorLifecycleSignalForwarding(t *testing.T) {
	cfg := supervisorConfig(t)
	var out signalOutputWriter
	res := awaitSupervisor(t, launchSupervisor(t, cfg, "signals", &out, io.Discard))
	require.NoError(t, res.err)
	require.False(t, res.upgrade)
	require.Equal(t, "ready\nreceived first signal\nreceived second signal\n", out.String())
}

type failingOutputWriter struct{ err error }

func (w failingOutputWriter) Write([]byte) (int, error) { return 0, w.err }

func TestSupervisorLifecycleScannerFailureStopsChild(t *testing.T) {
	t.Run("oversize", func(t *testing.T) {
		res := awaitSupervisor(t, launchSupervisor(t, supervisorConfig(t), "oversize", io.Discard, io.Discard))
		require.ErrorIs(t, res.err, bufio.ErrTooLong)
		require.False(t, res.upgrade)
	})
	t.Run("writer failure", func(t *testing.T) {
		writeErr := errors.New("output sink failed")
		res := awaitSupervisor(t, launchSupervisor(t, supervisorConfig(t), "writer-error", failingOutputWriter{writeErr}, io.Discard))
		require.ErrorIs(t, res.err, writeErr)
		require.False(t, res.upgrade)
	})
	t.Run("child failure", func(t *testing.T) {
		res := awaitSupervisor(t, launchSupervisor(t, supervisorConfig(t), "exit-error", io.Discard, io.Discard))
		require.ErrorContains(t, res.err, "exit status 7")
		require.False(t, res.upgrade)
	})
}

func TestSupervisorLifecycleSpacedJSONDownload(t *testing.T) {
	binary, err := os.ReadFile("testdata/repo/raw_binary/autod")
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/paxd" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(binary)
	}))
	defer server.Close()
	for _, suffix := range []string{"", " module=x/upgrade height=123"} {
		t.Run(fmt.Sprintf("suffix=%q", suffix), func(t *testing.T) {
			cfg := &cosmovisor.Config{Home: t.TempDir(), Name: "paxd", AllowDownloadBinaries: true}
			infoJSON := fmt.Sprintf(`{"binaries": {"%s": "%s/paxd", "other/arch": "https://unused.invalid"}}`, cosmovisor.OSArch(), server.URL)
			line := fmt.Sprintf(`ERR UPGRADE "v2" NEEDED at height: 123: %s%s`, infoJSON, suffix)
			info, err := cosmovisor.WaitForUpdate(bufio.NewScanner(strings.NewReader(line + "\n")))
			require.NoError(t, err)
			require.Equal(t, &cosmovisor.UpgradeInfo{Name: "v2", Info: infoJSON}, info)
			url, err := cosmovisor.GetDownloadURL(info)
			require.NoError(t, err)
			require.Equal(t, server.URL+"/paxd", url)
			require.NoError(t, cosmovisor.DownloadBinary(cfg, info))
			require.NoError(t, cosmovisor.EnsureBinary(cfg.UpgradeBin("v2")))
			got, err := os.ReadFile(cfg.UpgradeBin("v2"))
			require.NoError(t, err)
			require.Equal(t, binary, got)
		})
	}
}

func TestSupervisorLifecycleScannerCompatibility(t *testing.T) {
	t.Run("original scanner table", TestWaitForInfo)
	for _, tc := range []struct{ name, due, text, want string }{
		{"full text", "height: 123", "DownloadData here!", "DownloadData here!"},
		{"empty", "height: 123", "", ""},
		{"reference", "time: 2026-01-01T00:00:00Z", "https://example.com/info?checksum=sha256:abc module=x/upgrade", "https://example.com/info?checksum=sha256:abc"},
		{"nested JSON", "height: 1", `{"binaries": {"any": "https://example.com/paxd"}, "note": "a } bracket and \"quote\""} module=x/upgrade`, `{"binaries": {"any": "https://example.com/paxd"}, "note": "a } bracket and \"quote\""}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := fmt.Sprintf("UPGRADE %q NEEDED at %s: %s\n", "v2", tc.due, tc.text)
			info, err := cosmovisor.WaitForUpdate(bufio.NewScanner(strings.NewReader(line)))
			require.NoError(t, err)
			require.Equal(t, &cosmovisor.UpgradeInfo{Name: "v2", Info: tc.want}, info)
		})
	}
}

func TestSupervisorLifecycleDownloadCases(t *testing.T) {
	var existing upgradeTestSuite
	existing.SetT(t)
	existing.TestDownloadBinary()
}
