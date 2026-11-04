package chainflowdatagateway_test

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	sdk "chainflow-data-gateway"
	"chainflow-data-gateway/option"
	consumer "example.com/chainflow-example"
	"github.com/gorilla/websocket"
)

func repairM1306Read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func repairM1306Install(t *testing.T, path string) {
	t.Helper()
	doc := repairM1306Read(t, path)
	for _, required := range []string{
		"module chainflow-data-gateway", "not a downloadable public module",
		"CHAINFLOW_SDK=/absolute/path/to/paxeer-monorepo/sdk/chainflow/go",
		"mkdir chainflow-example\ncd chainflow-example\ngo mod init example.com/chainflow-example",
		`go mod edit -replace="chainflow-data-gateway=$CHAINFLOW_SDK"`,
		"go get chainflow-data-gateway@v0.0.0", "go mod tidy\ngo build ./...",
	} {
		if !strings.Contains(doc, required) {
			t.Errorf("%s lacks local consumer instruction %q", path, required)
		}
	}
	if strings.Contains(doc, "\ngo get chainflow-data-gateway\n") {
		t.Fatalf("%s advertises a hostname-free public installation", path)
	}
	module := repairM1306Read(t, "go.mod")
	if !strings.HasPrefix(module, "module chainflow-data-gateway\n") {
		t.Fatal("documented module differs from production go.mod")
	}
	manifestPath := "testdata/repair_m1306_consumer/go.mod"
	manifest := repairM1306Read(t, manifestPath)
	for _, directive := range []string{
		"module example.com/chainflow-example\n", "chainflow-data-gateway v0.0.0\n",
		"replace chainflow-data-gateway => ../..\n",
	} {
		if !strings.Contains(manifest, directive) {
			t.Fatalf("consumer manifest lacks %q", directive)
		}
	}
	replaced, err := os.ReadFile(filepath.Join(filepath.Dir(manifestPath), "../..", "go.mod"))
	if err != nil || string(replaced) != module {
		t.Fatalf("consumer replacement does not resolve the SDK: %v", err)
	}

	// Check the actual documented program's imports, rather than a prose import list.
	start := strings.Index(doc, "```go\npackage main\n")
	if start < 0 {
		t.Fatalf("%s has no complete consumer program", path)
	}
	source := strings.SplitN(doc[start+len("```go\n"):], "\n```", 2)[0]
	program, err := parser.ParseFile(token.NewFileSet(), "main.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	imports := map[string]string{}
	for _, item := range program.Imports {
		importPath, err := strconv.Unquote(item.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		alias := ""
		if item.Name != nil {
			alias = item.Name.Name
		}
		imports[importPath] = alias
	}
	if alias, ok := imports["chainflow-data-gateway"]; !ok || alias != "sdk" {
		t.Fatal("consumer must import the declared module with the documented sdk alias")
	}
	if _, ok := imports["chainflow-data-gateway/option"]; !ok {
		t.Fatal("consumer must import options from the same module")
	}
	calls := map[string]bool{}
	ast.Inspect(program, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok {
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
				calls[selector.Sel.Name] = true
			}
		}
		return true
	})
	for _, name := range []string{"NewClient", "WithRestAPIKey", "WithQueryDel", "ListHealth"} {
		if !calls[name] {
			t.Fatalf("documented consumer does not call %s", name)
		}
	}

	// The separately compiled consumer module imports and calls the real SDK.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/health" || r.URL.RawQuery != "" {
			t.Errorf("unexpected health request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("X-API-Key") != "fixture-rest-key" {
			t.Error("consumer did not apply the explicit REST key")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("gateway fixture ready"))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	health, err := consumer.CheckHealth(ctx, server.Client(), server.URL, "fixture-rest-key")
	if err != nil || health != "gateway fixture ready" {
		t.Fatalf("new consumer health = %q, error = %v", health, err)
	}
}

func TestRepairM_1306_InstallREADME(t *testing.T) {
	repairM1306Install(t, "README.md")
}

func TestRepairM_1306_InstallSkill(t *testing.T) {
	repairM1306Install(t, "SKILL.md")
}

func TestRepairM_1306_InstallAgentSkill(t *testing.T) {
	repairM1306Install(t, ".claude/skills/chainflow-data-gateway-go-sdk/SKILL.md")
}

func TestRepairM_1306_TokenHandshake(t *testing.T) {
	// Compile and execute exactly the function printed in the API reference.
	source := repairM1306Read(t, "testdata/repair_m1306_consumer/websocket.go")
	function := strings.TrimSpace(source[strings.Index(source, "func ConnectBTC"):])
	doc := repairM1306Read(t, "api.md")
	if !strings.Contains(doc, "```go\n"+function+"\n```") {
		t.Fatal("API token example differs from the compiled consumer function")
	}
	if strings.Contains(function, "fmt.Println(credential)") || strings.Contains(function, "fmt.Println(gateway)") {
		t.Fatal("token example prints a credential response")
	}

	const secret = "fixture-opaque+/=&? token"
	const welcome = `{"symbol":"BTC","price":123}`
	expires := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
	received := make(chan string, 1)
	upgrader := websocket.Upgrader{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/ws-token":
			if r.Method != http.MethodPost || r.Header.Get("X-API-Key") != "fixture-mint-key" || r.URL.RawQuery != "" {
				t.Error("mint request must use only the mint key, without a token query")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body) != 1 || body["symbol"] != "btc" {
				t.Error("SDK mint body must contain only the matching symbol")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(sdk.WsToken{
				Token: secret, ExpiresAt: expires, ExpiresIn: sdk.WsTokenExpiresIn300, Symbol: sdk.WsScopeBtc,
			}); err != nil {
				t.Error(err)
			}
		case "/ws/btc":
			query := r.URL.Query()
			if len(query) != 1 || len(query["token"]) != 1 || query.Get("token") != secret {
				t.Error("handshake did not URL-encode the minted credential as its sole query value")
				w.WriteHeader(http.StatusForbidden)
				return
			}
			if r.Header.Get("X-API-Key") != "" || r.Header.Get("Authorization") != "" {
				t.Error("handshake leaked a long-lived API key")
				w.WriteHeader(http.StatusForbidden)
				return
			}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Error("fixture could not upgrade the WebSocket")
				return
			}
			defer conn.Close()
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, []byte(welcome)); err != nil {
				t.Error(err)
				return
			}
			_, message, err := conn.ReadMessage()
			if err != nil {
				t.Error(err)
				return
			}
			received <- string(message)
		default:
			t.Error("unexpected fixture endpoint")
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := sdk.NewClient(option.WithBaseURL(server.URL), option.WithHTTPClient(server.Client()),
		option.WithRestAPIKey("fixture-rest-key"), option.WithWsToken("stale-token"), option.WithMaxRetries(0))
	dialer := websocket.Dialer{TLSClientConfig: server.Client().Transport.(*http.Transport).TLSClientConfig.Clone(), HandshakeTimeout: 5 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var output bytes.Buffer
	conn, err := consumer.ConnectBTC(ctx, client, "fixture-mint-key", "wss"+strings.TrimPrefix(server.URL, "https")+"/ws/btc?token=stale", &dialer, &output)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	kind, message, err := conn.ReadMessage()
	if err != nil || kind != websocket.TextMessage || string(message) != welcome {
		t.Fatalf("consumer did not receive the fixture stream: kind=%d message=%q err=%v", kind, message, err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte("consumer-received")); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-received:
		if got != "consumer-received" {
			t.Fatalf("upgraded transport received %q", got)
		}
	case <-ctx.Done():
		t.Fatal("fixture did not receive the consumer's WebSocket frame")
	}
	want := "scope=btc expires_at=2030-01-02T03:04:05Z expires_in=300\n"
	if output.String() != want {
		t.Fatalf("example output contains unexpected data: %q", output.String())
	}
	for _, sensitive := range []string{secret, "fixture-mint-key", "fixture-rest-key", "token=", "wss://"} {
		if strings.Contains(output.String(), sensitive) {
			t.Fatal("example output exposed sensitive data")
		}
	}
}

func TestRepairM_1306_TokenFailuresDoNotExposeSecrets(t *testing.T) {
	for _, stage := range []string{"mint", "handshake"} {
		t.Run(stage, func(t *testing.T) {
			const secret = "fixture-secret-never-print"
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/auth/ws-token" && stage == "handshake" {
					_ = json.NewEncoder(w).Encode(sdk.WsToken{Token: secret, ExpiresAt: time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC), ExpiresIn: sdk.WsTokenExpiresIn300, Symbol: sdk.WsScopeBtc})
					return
				}
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"` + secret + `"}`))
			}))
			defer server.Close()
			client := sdk.NewClient(option.WithBaseURL(server.URL), option.WithHTTPClient(server.Client()), option.WithMaxRetries(0))
			dialer := websocket.Dialer{TLSClientConfig: server.Client().Transport.(*http.Transport).TLSClientConfig.Clone(), HandshakeTimeout: 5 * time.Second}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var output bytes.Buffer
			conn, err := consumer.ConnectBTC(ctx, client, "fixture-mint-key", "wss"+strings.TrimPrefix(server.URL, "https")+"/ws/btc", &dialer, &output)
			if conn != nil || err == nil {
				t.Fatal("rejected mint or upgrade must return an error without a connection")
			}
			want := "WebSocket handshake failed"
			if stage == "mint" {
				want = "WebSocket token mint failed"
			}
			if err.Error() != want || output.Len() != 0 {
				t.Fatalf("failure must return only a safe message and no output: error=%q output=%q", err.Error(), output.String())
			}
		})
	}
}
