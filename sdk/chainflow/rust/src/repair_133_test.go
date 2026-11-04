package src

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var repair133Driver string

func TestMain(m *testing.M) {
	os.Exit(repair133Main(m))
}

func repair133Main(m *testing.M) int {
	target, err := os.MkdirTemp("", "repair-133-target-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(target)
	source, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	// Keep Cargo's generated manifest and lockfile outside the candidate tree.
	manifest := filepath.Join(target, "Cargo.toml")
	config := fmt.Sprintf(`[package]
name = "repair-133-driver"
version = "0.1.0"
edition = "2024"

[[bin]]
name = "repair-133-driver"
path = %q

[dependencies]
chainflow-data-gateway = { path = %q }
tokio = { version = "1", features = ["macros", "rt-multi-thread"] }
`, filepath.Join(source, "repair_133_driver", "main.rs"), filepath.Dir(source))
	if err := os.WriteFile(manifest, []byte(config), 0600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	lock, err := os.ReadFile(filepath.Join(filepath.Dir(source), "Cargo.lock"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.WriteFile(filepath.Join(target, "Cargo.lock"), lock, 0600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	// This compiles and links the real Rust SDK inside the declared Go gate.
	cmd := exec.Command("cargo", "build", "--manifest-path", manifest, "--target-dir", target)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "compile SDK qualification driver:", err)
		return 1
	}
	repair133Driver = filepath.Join(target, "debug", "repair-133-driver")
	return m.Run()
}

type repair133Request struct {
	method string
	path   string
	query  string
	header http.Header
	body   []byte
	err    error
}

func repair133Capture(t *testing.T, operation, rest, mint, token string) repair133Request {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	received := make(chan repair133Request, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			received <- repair133Request{err: err}
			return
		}
		defer conn.Close()
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			received <- repair133Request{err: err}
			return
		}
		req, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			received <- repair133Request{err: err}
			return
		}
		defer req.Body.Close()
		body, err := io.ReadAll(req.Body)
		received <- repair133Request{req.Method, req.URL.Path, req.URL.RawQuery, req.Header, body, err}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, repair133Driver, operation, "http://"+listener.Addr().String(), rest, mint, token)
	// Local wire qualification must not depend on the worker's proxy setup.
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		switch strings.ToLower(name) {
		case "http_proxy", "https_proxy", "all_proxy", "no_proxy":
		default:
			cmd.Env = append(cmd.Env, entry)
		}
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("real SDK %s operation: %v\n%s", operation, err, output)
	}
	request := <-received
	if request.err != nil {
		t.Fatal(request.err)
	}
	return request
}

func repair133AssertKey(t *testing.T, request repair133Request, method, path, key string) {
	t.Helper()
	if request.method != method || request.path != path {
		t.Fatalf("operation = %s %s, want %s %s", request.method, request.path, method, path)
	}
	keys := request.header.Values("X-API-Key")
	if key == "" {
		if len(keys) != 0 {
			t.Fatal("operation leaked an API key")
		}
	} else if len(keys) != 1 || keys[0] != key {
		t.Fatal("operation did not send exactly its selected API key")
	}
}

func TestRepair_133_RESTCredentialSelection(t *testing.T) {
	request := repair133Capture(t, "rest", "rest-key-133", "mint-key-133", "scoped-token-133")
	repair133AssertKey(t, request, http.MethodGet, "/api/btc/time", "rest-key-133")
	if request.query != "" {
		t.Fatal("REST read leaked a query credential")
	}
}

func TestRepair_133_MintCredentialSelection(t *testing.T) {
	request := repair133Capture(t, "mint", "rest-key-133", "mint-key-133", "scoped-token-133")
	repair133AssertKey(t, request, http.MethodPost, "/auth/ws-token", "mint-key-133")
	if request.query != "" {
		t.Fatal("mint operation leaked a query credential")
	}
	var body map[string]string
	if err := json.Unmarshal(request.body, &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body["symbol"] != "btc" {
		t.Fatalf("SDK mint body = %s", request.body)
	}
}

func TestRepair_133_UpgradeCredentialSelection(t *testing.T) {
	for operation, path := range map[string]string{
		"price": "/ws/btc", "markets": "/ws/markets", "explorer": "/ws/explorer", "alias": "/explorer",
	} {
		t.Run(operation, func(t *testing.T) {
			request := repair133Capture(t, operation, "rest-key-133", "mint-key-133", "scope+token/133?&=")
			repair133AssertKey(t, request, http.MethodGet, path, "")
			if request.query != "token=scope%2Btoken%2F133%3F%26%3D" {
				t.Fatal("upgrade did not encode exactly one scoped token")
			}
			if !strings.EqualFold(request.header.Get("Upgrade"), "websocket") || request.header.Get("Sec-WebSocket-Key") == "" {
				t.Fatal("request did not pass through the real WebSocket handshake")
			}
		})
	}
}

func TestRepair_133_NoCredentialSubstitution(t *testing.T) {
	for _, operation := range []string{"rest", "mint", "price"} {
		t.Run(operation, func(t *testing.T) {
			rest, mint, token := "rest-key-133", "mint-key-133", "scoped-token-133"
			path, method := "/api/btc/time", http.MethodGet
			switch operation {
			case "rest":
				rest = "unset"
			case "mint":
				mint, path, method = "unset", "/auth/ws-token", http.MethodPost
			case "price":
				token, path = "unset", "/ws/btc"
			}
			request := repair133Capture(t, operation, rest, mint, token)
			repair133AssertKey(t, request, method, path, "")
			if request.query != "" {
				t.Fatal("operation substituted an unrelated credential")
			}
		})
	}
}
