package debug

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/cli"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
)

func TestRepairM_1152_ArchiveRoundTrip(t *testing.T) {
	src := t.TempDir()
	if err := os.Mkdir(filepath.Join(src, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeStateJSONToFile(map[string]int{"height": 42}, src, "status.json"); err != nil {
		t.Fatal(err)
	}
	payload := []byte("goroutine profile\n")
	if err := os.WriteFile(filepath.Join(src, "nested", "goroutine.out"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "diagnostics.zip")
	if err := zipDir(src, dest); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	want := map[string][]byte{
		"diagnostics/":                     nil,
		"diagnostics/nested/":              nil,
		"diagnostics/nested/goroutine.out": payload,
		"diagnostics/status.json":          []byte("{\n  \"height\": 42\n}"),
	}
	for _, entry := range archive.File {
		expected, ok := want[entry.Name]
		if !ok {
			t.Fatalf("unexpected ZIP entry %q", entry.Name)
		}
		r, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(r)
		closeErr := r.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(body, expected) {
			t.Fatalf("entry %s: got %q, want %q", entry.Name, body, expected)
		}
		delete(want, entry.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing ZIP entries: %v", want)
	}
}

func TestRepairM_1152_ArchiveFinalizationFailure(t *testing.T) {
	name := filepath.Join(t.TempDir(), "readonly.zip")
	if err := os.WriteFile(name, nil, 0600); err != nil {
		t.Fatal(err)
	}
	output, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	// An empty directory fits in zip.Writer's buffer. Its first output write
	// occurs during Close, against a real file descriptor opened read-only.
	err = zipDirToFile(t.TempDir(), name, output)
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Op != "write" {
		t.Fatalf("ZIP finalization did not report its output failure: %v", err)
	}
	if _, err := output.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("failed ZIP output was not closed: %v", err)
	}
}

func TestRepairM_1152_ArchivePreservesAllErrors(t *testing.T) {
	name := filepath.Join(t.TempDir(), "closed.zip")
	output, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	err = zipDirToFile(filepath.Join(t.TempDir(), "missing"), name, output)
	if !errors.Is(err, os.ErrNotExist) || !errors.Is(err, os.ErrClosed) {
		t.Fatalf("lost traversal or finalization error: %v", err)
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok || len(joined.Unwrap()) != 3 {
		t.Fatalf("want traversal, ZIP-close and file-close errors: %v", err)
	}
}

func TestRepairM_1152_ProfileSizeLimit(t *testing.T) {
	for _, extra := range []int{0, 1} {
		t.Run(fmt.Sprintf("extra_%d", extra), func(t *testing.T) {
			body := strings.Repeat("p", maxProfileSize+extra)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/debug/pprof/heap" || r.URL.Query().Get("debug") != "2" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush() // No Content-Length: enforce the streaming bound.
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			dir := t.TempDir()
			filename := filepath.Join(dir, "heap.out")
			original := []byte("previous valid profile")
			if err := os.WriteFile(filename, original, 0600); err != nil {
				t.Fatal(err)
			}
			err := dumpProfile(t.Context(), dir, server.URL, "heap", 2)
			if extra == 1 {
				if !errors.Is(err, errProfileTooLarge) {
					t.Fatalf("oversized profile error: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			actual, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			if extra == 0 && string(actual) != body {
				t.Fatalf("profile at the size limit was not preserved: %d bytes", len(actual))
			}
			if extra == 1 && !bytes.Equal(actual, original) {
				t.Fatal("oversized response overwrote the previous profile")
			}
		})
	}
}

func TestRepairM_1152_ProfileRejectsHTTPFailures(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "HTTP error page")
			}))
			defer server.Close()
			dir := t.TempDir()
			if err := dumpProfile(t.Context(), dir, server.URL, "goroutine", 2); err == nil {
				t.Fatalf("accepted HTTP status %d", status)
			}
			if _, err := os.Stat(filepath.Join(dir, "goroutine.out")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unsuccessful response published a profile: %v", err)
			}
		})
	}
}

func TestRepairM_1152_ProfileCancellation(t *testing.T) {
	for _, phase := range []string{"headers", "body"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if phase == "body" {
					_, _ = io.WriteString(w, "partial profile")
					w.(http.Flusher).Flush()
				}
				cancel()
				<-r.Context().Done()
			}))
			defer server.Close()
			dir := t.TempDir()
			if err := dumpProfile(ctx, dir, server.URL, "heap", 2); !errors.Is(err, context.Canceled) {
				t.Fatalf("lost cancellation during %s: %v", phase, err)
			}
			if _, err := os.Stat(filepath.Join(dir, "heap.out")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("canceled response published a profile: %v", err)
			}
		})
	}
}

func TestRepairM_1152_ProfileClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "partial profile")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), profileRequestTimeout+5*time.Second)
	defer cancel()
	dir := t.TempDir()
	err := dumpProfile(ctx, dir, server.URL, "heap", 2)
	var timeoutErr net.Error
	if !errors.As(err, &timeoutErr) || !timeoutErr.Timeout() {
		t.Fatalf("stalled response did not time out: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatalf("parent deadline fired before the HTTP client timeout: %v", ctx.Err())
	}
	if _, err := os.Stat(filepath.Join(dir, "heap.out")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("timed-out response published a profile: %v", err)
	}
}

func repairM1152RPCServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"status": rpcserver.NewRPCFunc(func(context.Context) (*coretypes.ResultStatus, error) {
			status := &coretypes.ResultStatus{}
			status.NodeInfo.Network = "repair-m1152"
			return status, nil
		}),
		"net_info": rpcserver.NewRPCFunc(func(context.Context) (*coretypes.ResultNetInfo, error) {
			return &coretypes.ResultNetInfo{Listening: true}, nil
		}),
		"dump_consensus_state": rpcserver.NewRPCFunc(func(context.Context) (*coretypes.ResultDumpConsensusState, error) {
			return &coretypes.ResultDumpConsensusState{RoundState: json.RawMessage(`{"height":42}`)}, nil
		}),
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func repairM1152Config(t *testing.T) *config.Config {
	t.Helper()
	conf := config.DefaultConfig().SetRoot(t.TempDir())
	wal := conf.Consensus.WalFile()
	if err := os.MkdirAll(filepath.Dir(wal), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wal, []byte("recorded WAL fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return conf
}

func TestRepairM_1152_CommandContextReachesProfiles(t *testing.T) {
	for _, profile := range []string{"goroutine", "heap"} {
		t.Run(profile, func(t *testing.T) {
			rpc := repairM1152RPCServer(t)
			conf := repairM1152Config(t)
			previousHome := viper.Get(cli.HomeFlag)
			viper.Set(cli.HomeFlag, conf.RootDir)
			defer viper.Set(cli.HomeFlag, previousHome)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			requests := make(chan string, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests <- r.URL.Path
				if r.URL.Path == "/debug/pprof/"+profile {
					_, _ = io.WriteString(w, "partial profile")
					w.(http.Flusher).Flush()
					cancel()
					<-r.Context().Done()
					return
				}
				_, _ = io.WriteString(w, "complete goroutine profile")
			}))
			defer server.Close()
			outDir := t.TempDir()
			cmd := GetDebugCommand()
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"--" + flagNodeRPCAddr, rpc.URL, "dump", outDir, "--" + flagProfAddr, server.URL})
			if err := cmd.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("command cancellation at %s: %v", profile, err)
			}
			want := []string{"/debug/pprof/goroutine"}
			if profile == "heap" {
				want = append(want, "/debug/pprof/heap")
			}
			for _, endpoint := range want {
				select {
				case got := <-requests:
					if got != endpoint {
						t.Fatalf("got profile request %q, want %q", got, endpoint)
					}
				default:
					t.Fatalf("missing profile request %q", endpoint)
				}
			}
			entries, err := os.ReadDir(outDir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("canceled dump left output: %v, %v", entries, err)
			}
		})
	}
}

func TestRepairM_1152_DumpArchiveContainsProfiles(t *testing.T) {
	rpcServer := repairM1152RPCServer(t)
	rpc, err := rpchttp.New(rpcServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "profile fixture: "+r.URL.Path)
	}))
	defer server.Close()
	outDir := t.TempDir()
	dumpDebugData(t.Context(), rpc, dumpDebugDataArgs{conf: repairM1152Config(t), outDir: outDir, profAddr: server.URL})
	entries, err := os.ReadDir(outDir)
	if err != nil || len(entries) != 1 || filepath.Ext(entries[0].Name()) != ".zip" {
		t.Fatalf("dump did not produce exactly one archive: %v, %v", entries, err)
	}
	archive, err := zip.OpenReader(filepath.Join(outDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	want := map[string]string{
		"goroutine.out": "profile fixture: /debug/pprof/goroutine",
		"heap.out":      "profile fixture: /debug/pprof/heap",
	}
	for _, entry := range archive.File {
		expected, ok := want[filepath.Base(entry.Name)]
		if !ok {
			continue
		}
		r, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(r)
		closeErr := r.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			t.Fatal(err)
		}
		if string(body) != expected {
			t.Fatalf("archived profile %q = %q, want %q", entry.Name, body, expected)
		}
		delete(want, filepath.Base(entry.Name))
	}
	if len(want) != 0 {
		t.Fatalf("missing archived profiles: %v", want)
	}
}
