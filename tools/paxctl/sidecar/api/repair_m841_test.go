package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"math"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	seiconfig "github.com/sei-protocol/sei-config"
	sidecar "github.com/sei-protocol/seictl/sidecar/client"
	"github.com/sei-protocol/seictl/sidecar/engine"
	"github.com/sei-protocol/seictl/sidecar/server"
	"github.com/sei-protocol/seictl/sidecar/tasks"
	"sigs.k8s.io/yaml"
)

type repair841Schema struct {
	Ref        string                      `json:"$ref"`
	Type       string                      `json:"type"`
	Format     string                      `json:"format"`
	Pattern    string                      `json:"pattern"`
	Minimum    *float64                    `json:"minimum"`
	Required   []string                    `json:"required"`
	Properties map[string]*repair841Schema `json:"properties"`
	Items      *repair841Schema            `json:"items"`
	Enum       []string                    `json:"enum"`
}

type repair841Operation struct {
	OperationID string                `json:"operationId"`
	Security    []map[string][]string `json:"security"`
	Parameters  []struct {
		Name   string          `json:"name"`
		In     string          `json:"in"`
		Schema repair841Schema `json:"schema"`
	} `json:"parameters"`
	Responses map[string]struct {
		Content map[string]struct {
			Schema repair841Schema `json:"schema"`
		} `json:"content"`
	} `json:"responses"`
}

type repair841Contract struct {
	Paths      map[string]map[string]repair841Operation `json:"paths"`
	Components struct {
		Schemas map[string]*repair841Schema `json:"schemas"`
	} `json:"components"`
}

func repair841Load(t *testing.T) *repair841Contract {
	t.Helper()
	data, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var contract repair841Contract
	var document map[string]any
	if err := yaml.UnmarshalStrict(data, &document); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	return &contract
}

// Check the schema against messages emitted by the real server, rather than
// manufacturing a second TaskResult type that could conceal the omitted field.
func (c *repair841Contract) check(t *testing.T, schema *repair841Schema, value any) {
	t.Helper()
	if schema == nil {
		t.Fatal("missing response schema")
	}
	if schema.Ref != "" {
		const prefix = "#/components/schemas/"
		if !strings.HasPrefix(schema.Ref, prefix) {
			t.Fatalf("unexpected schema reference %q", schema.Ref)
		}
		c.check(t, c.Components.Schemas[strings.TrimPrefix(schema.Ref, prefix)], value)
		return
	}
	switch schema.Type {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("expected object, got %T", value)
		}
		for _, name := range schema.Required {
			if _, ok := object[name]; !ok {
				t.Fatalf("response missing required property %s", name)
			}
		}
		for name, child := range schema.Properties {
			if v, ok := object[name]; ok {
				c.check(t, child, v)
			}
		}
	case "array":
		array, ok := value.([]any)
		if !ok {
			t.Fatalf("expected array, got %T", value)
		}
		for _, v := range array {
			c.check(t, schema.Items, v)
		}
	case "integer":
		n, ok := value.(float64)
		if !ok || n != math.Trunc(n) {
			t.Fatalf("expected integer, got %v", value)
		}
		if schema.Minimum != nil && n < *schema.Minimum {
			t.Fatalf("integer %v below minimum %v", n, *schema.Minimum)
		}
	case "string":
		s, ok := value.(string)
		if !ok {
			t.Fatalf("expected string, got %T", value)
		}
		if len(schema.Enum) > 0 {
			found := false
			for _, allowed := range schema.Enum {
				found = found || s == allowed
			}
			if !found {
				t.Fatalf("%q outside enum %v", s, schema.Enum)
			}
		}
		if schema.Pattern != "" && !regexp.MustCompile(schema.Pattern).MatchString(s) {
			t.Fatalf("%q violates %s", s, schema.Pattern)
		}
		switch schema.Format {
		case "uuid":
			if _, err := uuid.Parse(s); err != nil {
				t.Fatal(err)
			}
		case "date-time":
			if _, err := time.Parse(time.RFC3339Nano, s); err != nil {
				t.Fatal(err)
			}
		}
	default:
		t.Fatalf("unhandled declared schema type %q", schema.Type)
	}
}

func (c *repair841Contract) response(t *testing.T, method, path string, status int, header http.Header, body []byte) {
	t.Helper()
	op, ok := c.Paths[path][strings.ToLower(method)]
	if !ok {
		t.Fatalf("undocumented operation: %s %s", method, path)
	}
	resp, ok := op.Responses[strconv.Itoa(status)]
	if !ok {
		t.Fatalf("undocumented response: %s %s %d: %s", method, path, status, body)
	}
	if len(body) == 0 {
		if len(resp.Content) != 0 && !(path == "/v0/livez" && status == 503) {
			t.Fatalf("empty body for declared content: %s %s %d", method, path, status)
		}
		return
	}
	media, _, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	content, ok := resp.Content[media]
	if !ok {
		t.Fatalf("undocumented content type %q for %s %s %d", media, method, path, status)
	}
	var value any = string(body)
	if media == "application/json" {
		if err := json.Unmarshal(body, &value); err != nil {
			t.Fatal(err)
		}
	}
	c.check(t, &content.Schema, value)
}

func TestRepairM_841_RegisteredOperations(t *testing.T) {
	c := repair841Load(t)
	file, err := parser.ParseFile(token.NewFileSet(), "../server/server.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	registered := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 1 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Handle" && sel.Sel.Name != "HandleFunc") {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			t.Fatal("route registration must be accounted for")
		}
		pattern, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Fatal(err)
		}
		registered[pattern] = true
		return true
	})
	if len(registered) != 10 {
		t.Fatalf("expected 10 registered operations, got %v", registered)
	}
	ids := map[string]bool{}
	for path, methods := range c.Paths {
		for method, op := range methods {
			key := strings.ToUpper(method) + " " + path
			if !registered[key] {
				t.Fatalf("schema operation has no server registration: %s", key)
			}
			delete(registered, key)
			if op.OperationID == "" || ids[op.OperationID] {
				t.Fatalf("missing or duplicate operationId %q", op.OperationID)
			}
			ids[op.OperationID] = true
			bypass := path == "/v0/healthz" || path == "/v0/startupz" || path == "/v0/livez" || path == "/v0/metrics"
			if bypass {
				if op.Security == nil || len(op.Security) != 0 {
					t.Fatalf("probe/scrape requires explicit public security: %s", key)
				}
			} else {
				if len(op.Security) != 1 {
					t.Fatalf("missing security on %s", key)
				}
				if _, ok := op.Security[0]["remoteUserHeader"]; !ok {
					t.Fatalf("missing identity security on %s", key)
				}
				if _, ok := op.Responses["401"]; !ok {
					t.Fatalf("missing auth error on %s", key)
				}
			}
		}
	}
	if len(registered) != 0 {
		t.Fatalf("registered operations missing from schema: %v", registered)
	}
	run := c.Components.Schemas["TaskResult"].Properties["run"]
	if run == nil || run.Type != "integer" || run.Minimum == nil || *run.Minimum != 1 {
		t.Fatalf("run must be a positive integer: %+v", run)
	}
	found := false
	for _, name := range c.Components.Schemas["TaskResult"].Required {
		found = found || name == "run"
	}
	if !found {
		t.Fatal("run is emitted on every result and must be required")
	}
	verbose := c.Paths["/v0/livez"]["get"].Parameters
	if len(verbose) != 1 || verbose[0].Name != "verbose" || verbose[0].In != "query" {
		t.Fatalf("missing liveness verbose query: %+v", verbose)
	}
}

type repair841Server struct {
	base   string
	http   *http.Client
	engine *engine.Engine
	store  *engine.SQLiteStore
	home   string
}

func repair841Start(t *testing.T) *repair841Server {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	store, err := engine.NewMemoryStore()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	home := t.TempDir()
	eng := engine.NewEngine(ctx, map[engine.TaskType]engine.TaskHandler{
		engine.TaskConfigValidate: tasks.NewConfigValidator(home).Handler(),
		engine.TaskMarkReady:      tasks.MarkReadyHandler(),
	}, store)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	srv := server.NewServer(addr, eng, home, server.AuthnModeTrustedHeader)
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()
	hc := &http.Client{Timeout: time.Second}
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("server shutdown: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("server shutdown timed out")
		}
		hc.CloseIdleConnections()
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	fixture := &repair841Server{base: "http://" + addr, http: hc, engine: eng, store: store, home: home}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := hc.Get(fixture.base + "/v0/livez")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("startup liveness = %d", resp.StatusCode)
			}
			return fixture
		}
		select {
		case err := <-done:
			t.Fatalf("server startup: %v", err)
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("server did not start")
	return nil
}

func (s *repair841Server) request(t *testing.T, c *repair841Contract, method, path, body string, identity bool, want int) []byte {
	t.Helper()
	req, err := http.NewRequest(method, s.base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if identity {
		req.Header.Set("X-Remote-User", "repair-841")
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, resp.StatusCode, want, data)
	}
	contractPath := strings.Split(path, "?")[0]
	if strings.HasPrefix(contractPath, "/v0/tasks/") {
		contractPath = "/v0/tasks/{id}"
	}
	c.response(t, method, contractPath, resp.StatusCode, resp.Header, data)
	return data
}

func repair841Client(t *testing.T, s *repair841Server) *sidecar.ClientWithResponses {
	t.Helper()
	client, err := sidecar.NewClientWithResponses(s.base, sidecar.WithHTTPClient(s.http), sidecar.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Set("X-Remote-User", "repair-841")
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func repair841Wait(t *testing.T, eng *engine.Engine, id string, run int, status engine.TaskStatus) *engine.TaskResult {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		result := eng.GetResult(id)
		if result != nil && result.Run == run && result.Status == status {
			return result
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("task never reached run %d, status %s: %+v", run, status, eng.GetResult(id))
	return nil
}

func TestRepairM_841_FailedTaskResubmission(t *testing.T) {
	c, s := repair841Load(t), repair841Start(t)
	client := repair841Client(t, s)
	ctx := context.Background()
	id := uuid.New()
	request := sidecar.TaskRequest{Id: &id, Type: string(engine.TaskConfigValidate)}
	for run, status := range []engine.TaskStatus{engine.TaskStatusFailed, engine.TaskStatusCompleted} {
		if run == 1 {
			if err := seiconfig.WriteConfigToDir(seiconfig.DefaultForMode(seiconfig.ModeValidator), s.home); err != nil {
				t.Fatal(err)
			}
		}
		submit, err := client.SubmitTaskWithResponse(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		if submit.JSON201 == nil || submit.JSON201.Id != id {
			t.Fatalf("submit: %+v", submit)
		}
		c.response(t, "POST", "/v0/tasks", submit.StatusCode(), submit.HTTPResponse.Header, submit.Body)
		stored := repair841Wait(t, s.engine, id.String(), run+1, status)
		get, err := client.GetTaskWithResponse(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if get.JSON200 == nil || get.JSON200.Id != id || string(get.JSON200.Status) != string(status) {
			t.Fatalf("generated get decoder: %+v", get)
		}
		if !get.JSON200.SubmittedAt.Equal(stored.SubmittedAt) || get.JSON200.CompletedAt == nil {
			t.Fatal("generated decoder lost lifecycle timestamps")
		}
		if status == engine.TaskStatusFailed && (get.JSON200.Error == nil || !strings.Contains(*get.JSON200.Error, "reading config")) {
			t.Fatalf("missing real config failure: %+v", get.JSON200)
		}
		if status == engine.TaskStatusCompleted && get.JSON200.Error != nil {
			t.Fatalf("retry retained stale error: %+v", get.JSON200)
		}
		c.response(t, "GET", "/v0/tasks/{id}", get.StatusCode(), get.HTTPResponse.Header, get.Body)
		var raw map[string]any
		if err := json.Unmarshal(get.Body, &raw); err != nil {
			t.Fatal(err)
		}
		if raw["run"] != float64(run+1) {
			t.Fatalf("wire run = %v, want %d", raw["run"], run+1)
		}
		// Every producer property must be declared; the audit's omitted run
		// would fail this assertion even though the old decoder drops it.
		for key := range raw {
			if c.Components.Schemas["TaskResult"].Properties[key] == nil {
				t.Fatalf("undeclared producer field %s", key)
			}
		}
		list, err := client.ListTasksWithResponse(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if list.JSON200 == nil || len(*list.JSON200) != 1 || (*list.JSON200)[0].Id != id || (*list.JSON200)[0].Status != get.JSON200.Status {
			t.Fatalf("generated list decoder: %+v", list)
		}
		c.response(t, "GET", "/v0/tasks", list.StatusCode(), list.HTTPResponse.Header, list.Body)
		var rawList []map[string]any
		if err := json.Unmarshal(list.Body, &rawList); err != nil {
			t.Fatal(err)
		}
		if len(rawList) != 1 || !reflect.DeepEqual(raw, rawList[0]) {
			t.Fatalf("get/list contract diverged: %s / %s", get.Body, list.Body)
		}
	}
	before := s.engine.GetResult(id.String())
	if _, err := client.SubmitTaskWithResponse(ctx, request); err != nil {
		t.Fatal(err)
	}
	after := s.engine.GetResult(id.String())
	if after.Run != 2 || !after.SubmittedAt.Equal(before.SubmittedAt) {
		t.Fatalf("completed resubmission changed run: %+v", after)
	}
	deleted, err := client.DeleteTaskWithResponse(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if deleted.StatusCode() != http.StatusNoContent || len(deleted.Body) != 0 {
		t.Fatalf("delete: %+v", deleted)
	}
	c.response(t, "DELETE", "/v0/tasks/{id}", deleted.StatusCode(), deleted.HTTPResponse.Header, deleted.Body)
	s.request(t, c, "GET", "/v0/tasks/"+id.String(), "", true, 404)
}

func TestRepairM_841_ProbeAndNodeIDResponses(t *testing.T) {
	c, s := repair841Load(t), repair841Start(t)
	for _, path := range []string{"/v0/healthz", "/v0/startupz"} {
		if body := s.request(t, c, "GET", path, "", false, 503); len(body) != 0 {
			t.Fatalf("probe returned body: %s", body)
		}
	}
	if body := s.request(t, c, "GET", "/v0/livez", "", false, 200); len(body) != 0 {
		t.Fatalf("probe returned body: %s", body)
	}
	metrics := s.request(t, c, "GET", "/v0/metrics", "", false, 200)
	if !bytes.Contains(metrics, []byte("# HELP")) || !bytes.Contains(metrics, []byte("go_goroutines")) {
		t.Fatal("missing real Prometheus exposition")
	}
	s.request(t, c, "GET", "/v0/status", "", false, 401)
	s.request(t, c, "GET", "/v0/node-id", "", false, 401)
	s.request(t, c, "POST", "/v0/tasks", `{"type":"mark-ready"}`, false, 401)
	s.request(t, c, "DELETE", "/v0/tasks/"+uuid.NewString(), "", false, 401)
	status := s.request(t, c, "GET", "/v0/status", "", true, 200)
	if !bytes.Contains(status, []byte(`"Initializing"`)) {
		t.Fatalf("initial status: %s", status)
	}
	s.request(t, c, "POST", "/v0/tasks", `{"type":"mark-ready"}`, true, 201)
	deadline := time.Now().Add(5 * time.Second)
	for !s.engine.Healthz() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	for _, path := range []string{"/v0/healthz", "/v0/startupz"} {
		if body := s.request(t, c, "GET", path, "", false, 200); len(body) != 0 {
			t.Fatalf("ready probe returned body: %s", body)
		}
	}
	s.request(t, c, "GET", "/v0/node-id", "", true, 500)
	keyPath := filepath.Join(s.home, "config", "node_key.json")
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"{", `{"priv_key":{"value":"!"}}`, `{"priv_key":{"value":"AA=="}}`} {
		if err := os.WriteFile(keyPath, []byte(invalid), 0600); err != nil {
			t.Fatal(err)
		}
		s.request(t, c, "GET", "/v0/node-id", "", true, 500)
	}
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x84}, ed25519.SeedSize))
	keyJSON := fmt.Sprintf(`{"priv_key":{"type":"tendermint/PrivKeyEd25519","value":%q}}`, base64.StdEncoding.EncodeToString(key))
	if err := os.WriteFile(keyPath, []byte(keyJSON), 0600); err != nil {
		t.Fatal(err)
	}
	nodeBody := s.request(t, c, "GET", "/v0/node-id", "", true, 200)
	var node map[string]string
	if err := json.Unmarshal(nodeBody, &node); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(key.Public().(ed25519.PublicKey))
	if node["nodeId"] != hex.EncodeToString(hash[:20]) || len(node) != 1 {
		t.Fatalf("unexpected public node ID: %s", nodeBody)
	}
	if err := s.store.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v0/livez", "/v0/livez?verbose=1"} {
		body := s.request(t, c, "GET", path, "", false, 503)
		if strings.Contains(path, "?") {
			var failure sidecar.ErrorResponse
			if err := json.Unmarshal(body, &failure); err != nil || failure.Error == "" {
				t.Fatalf("missing verbose store error: %s (%v)", body, err)
			}
		} else if len(body) != 0 {
			t.Fatalf("non-verbose liveness returned body: %s", body)
		}
	}
}
