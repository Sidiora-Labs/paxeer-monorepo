package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/urfave/cli/v3"
	"sigs.k8s.io/yaml"

	"github.com/sei-protocol/seictl/internal/cliutil"
	"github.com/sei-protocol/seictl/internal/patch"
	"github.com/sei-protocol/seictl/internal/releaseproof"
	sidecar "github.com/sei-protocol/seictl/sidecar/client"
	"github.com/sei-protocol/seictl/sidecar/engine"
	"github.com/sei-protocol/seictl/sidecar/rpc"
	"github.com/sei-protocol/seictl/sidecar/server"
	"github.com/sei-protocol/seictl/sidecar/tasks"
)

func repair839Read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func repair839Require(t *testing.T, text string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(text, needle) {
			t.Errorf("missing %q", needle)
		}
	}
}

// Copy the actual registrations, including flags/arguments, so independent CLI
// invocations do not reuse urfave's mutable parser state.
func repair839Command(source *cli.Command) *cli.Command {
	cmd := *source
	cmd.Commands = nil
	for _, child := range source.Commands {
		cmd.Commands = append(cmd.Commands, repair839Command(child))
	}
	cmd.Flags = nil
	for _, flag := range source.Flags {
		value := reflect.ValueOf(flag)
		copy := reflect.New(value.Elem().Type())
		copy.Elem().Set(value.Elem())
		cmd.Flags = append(cmd.Flags, copy.Interface().(cli.Flag))
	}
	cmd.Arguments = nil
	for _, arg := range source.Arguments {
		value := reflect.ValueOf(arg)
		copy := reflect.New(value.Elem().Type())
		copy.Elem().Set(value.Elem())
		cmd.Arguments = append(cmd.Arguments, copy.Interface().(cli.Argument))
	}
	return &cmd
}

func repair839Run(t *testing.T, ctx context.Context, args []string) ([]byte, error) {
	t.Helper()
	out, err := os.CreateTemp(t.TempDir(), "stdout-")
	if err != nil {
		return nil, err
	}
	defer out.Close()
	previous := os.Stdout
	os.Stdout = out
	defer func() { os.Stdout = previous }()
	cmd := repair839Command(&seictlCmd)
	cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}
	cmd.Writer = out
	cmd.ErrWriter = io.Discard
	err = cmd.Run(ctx, append([]string{"seictl"}, args...))
	data, readErr := os.ReadFile(out.Name())
	if err != nil {
		return data, err
	}
	return data, readErr
}

// Validate executable examples against the registered production command tree,
// rather than maintaining a separate list of supported verbs and flags.
func repair839DocCommands(t *testing.T, doc string) map[string]bool {
	t.Helper()
	found := map[string]bool{}
	inShell := false
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "```") {
			if inShell {
				inShell = false
			} else {
				inShell = line == "```sh" || line == "```bash"
			}
			continue
		}
		if !inShell {
			continue
		}
		index := strings.Index(line, "seictl ")
		if index < 0 || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		prefix := line[:index]
		if prefix != "" && prefix != "./" && prefix != "./build/" && !strings.HasSuffix(prefix, "$(") {
			continue
		}
		args := strings.Fields(strings.SplitN(line[index+len("seictl "):], "|", 2)[0])
		cmd := &seictlCmd
		flagNames := map[string]bool{"help": true, "h": true}
		addFlags := func(c *cli.Command) {
			for _, f := range c.Flags {
				for _, name := range f.Names() {
					flagNames[name] = true
				}
			}
		}
		addFlags(cmd)
		for _, arg := range args {
			if strings.HasPrefix(arg, "-") {
				name := strings.SplitN(strings.TrimLeft(arg, "-"), "=", 2)[0]
				if !flagNames[name] {
					t.Errorf("unregistered flag %q in %s", name, line)
				}
				continue
			}
			for _, child := range cmd.Commands {
				if child.Name == arg {
					cmd = child
					addFlags(cmd)
					found[arg] = true
					break
				}
			}
		}
		if cmd == &seictlCmd && len(args) > 0 && args[0] != "--help" {
			t.Errorf("unregistered command example: %s", line)
		}
	}
	return found
}

func TestRepairM_839_RetiredSmokeCommands(t *testing.T) {
	doc := repair839Read(t, ".agent/runbooks/seictl-smoke.md")
	found := repair839DocCommands(t, doc)
	for _, tree := range []string{"network", "node", "workflow", "task"} {
		if !found[tree] {
			t.Errorf("smoke does not execute %s", tree)
		}
	}
	for _, retired := range []string{"seictl onboard", "seictl bench", "seinodedeploy"} {
		if strings.Contains(strings.ToLower(doc), retired) {
			t.Errorf("retired smoke operation: %s", retired)
		}
	}
	repair839Require(t, doc, "seinetworks.sei.io", "seinodes.sei.io", "seinodetaskworkflows.sei.io", "eth_chainId", ".result.node_info.network", "EXPECTED_EVM_CHAIN_ID", "latest_block_height", "--proxy-ca-file", "--dry-run", "config-validate", "status=completed", "two-minute timeout", "registry.example.invalid", "--until Ready", "--until Running")
	// The same documented read-only task must traverse the real wire codec and
	// production handler against a config written by the production generator.
	home := t.TempDir()
	if _, err := tasks.NewConfigApplier(home).Handler()(context.Background(), map[string]any{"mode": "full", "incremental": false}); err != nil {
		t.Fatal(err)
	}
	req := (sidecar.ConfigValidateTask{}).ToTaskRequest()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var received server.TaskRequest
	if err := json.Unmarshal(data, &received); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.NewConfigValidator(home).Handler()(context.Background(), received.Params); err != nil {
		t.Fatalf("documented config-validate: %v", err)
	}
}

func TestRepairM_839_ReleaseProvenance(t *testing.T) {
	version, err := releaseproof.ReadVersion("version.json", "")
	if err != nil {
		t.Fatal(err)
	}
	previous := cliutil.Version
	t.Cleanup(func() { cliutil.Version = previous })
	cliutil.Version = version
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := releaseproof.Verify(ctx, version, func(ctx context.Context, args []string) ([]byte, error) { return repair839Run(t, ctx, args) }); err != nil {
		t.Fatal(err)
	}
	// A missing linker stamp is rejected by the same verifier used for releases.
	cliutil.Version = "dev"
	if err := releaseproof.Verify(ctx, version, func(ctx context.Context, args []string) ([]byte, error) { return repair839Run(t, ctx, args) }); err == nil {
		t.Fatal("unstamped artifact passed provenance verification")
	}
	cliutil.Version = version
	if err := releaseproof.Verify(ctx, "v999.888.777", func(ctx context.Context, args []string) ([]byte, error) { return repair839Run(t, ctx, args) }); err == nil {
		t.Fatal("wrong release stamp passed provenance verification")
	}

	for _, bad := range []string{"", "dev", "1.2.3", "v1.2.3\n", "v1.2.3 -X main.injected=1", "v1.2.3'", "$(echo injected)"} {
		if err := releaseproof.ValidateVersion(bad); err == nil {
			t.Errorf("accepted unsafe version %q", bad)
		}
	}
	if _, err := releaseproof.ReadVersion("version.json", "v999.888.777"); err == nil {
		t.Fatal("release tag mismatch accepted")
	}
	for _, value := range []string{"v1.2.3", "v1.2.3-rc.1"} {
		if err := releaseproof.ValidateVersion(value); err != nil {
			t.Fatal(err)
		}
	}
	var config struct {
		Before struct {
			Hooks []string `json:"hooks"`
		} `json:"before"`
		Builds []struct {
			LDFlags []string `json:"ldflags"`
			Goos    []string `json:"goos"`
			Goarch  []string `json:"goarch"`
		} `json:"builds"`
	}
	if err := yaml.Unmarshal([]byte(repair839Read(t, ".goreleaser.yaml")), &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Builds) != 1 {
		t.Fatal("expected one release build definition")
	}
	if !slices.Equal(config.Builds[0].Goos, []string{"linux", "darwin"}) || !slices.Equal(config.Builds[0].Goarch, []string{"amd64", "arm64"}) {
		t.Fatalf("documented release platforms differ: %+v", config.Builds[0])
	}
	repair839Require(t, strings.Join(config.Before.Hooks, "\n"), "go run ./internal/release version --expected {{ .Tag }}")
	flags := strings.Join(config.Builds[0].LDFlags, " ")
	repair839Require(t, flags, "-X github.com/sei-protocol/seictl/internal/cliutil.Version={{ .Tag }}")
	if strings.Contains(flags, "main.") {
		t.Fatal("release still stamps nonexistent main symbols")
	}
	makefile := repair839Read(t, "Makefile")
	repair839Require(t, makefile, "VERSION   := $(shell go run ./internal/release version)", "github.com/sei-protocol/seictl/internal/cliutil.Version=$(VERSION)", "go run ./internal/release verify --binary $(BUILD_DIR)/$(BIN)")
	// The Makefile's defense-in-depth shell guard must match the shared validator.
	repair839Require(t, makefile, strings.TrimSuffix(releaseproof.VersionPattern, "$")+"$$")
	docker := repair839Read(t, "Dockerfile")
	repair839Require(t, docker, `VERSION="$(/go/bin/releaseverify version)"`, "github.com/sei-protocol/seictl/internal/cliutil.Version=${VERSION}", `/go/bin/releaseverify verify --binary /go/bin/seictl --expected "$VERSION"`, "COPY . /src")
	workflow := repair839Read(t, ".github/workflows/release-provenance.yml")
	for _, archive := range []string{"seictl_Linux_x86_64.tar.gz", "seictl_Linux_arm64.tar.gz", "seictl_Darwin_x86_64.tar.gz", "seictl_Darwin_arm64.tar.gz"} {
		repair839Require(t, workflow, archive)
	}
	repair839Require(t, workflow, "ubuntu-24.04-arm", "macos-15-intel", "macos-14", "shasum -a 256 -c selected-checksum.txt", `go run ./internal/release verify --expected "$RELEASE_TAG" --binary "$RUNNER_TEMP/paxctl-artifact/seictl"`)
}

func TestRepairM_839_UpstreamInstallInstructions(t *testing.T) {
	doc := repair839Read(t, "README.md")
	repair839DocCommands(t, doc)
	versionLine := regexp.MustCompile(`(?m)^go ([0-9.]+)$`).FindStringSubmatch(repair839Read(t, "go.mod"))
	if len(versionLine) != 2 {
		t.Fatal("module toolchain is missing")
	}
	repair839Require(t, doc, "Go "+versionLine[1], "Paxeer-Network/paxctl/releases", "Sidiora-Labs/paxeer-monorepo.git", `git checkout --detach "$SOURCE_REVISION"`, "make build", "local", "replace", "--proxy-ca-file", "TokenReview", "SubjectAccessReview", "Windows and ARMv7 archives are unsupported")
	for _, archive := range []string{"seictl_Linux_x86_64.tar.gz", "seictl_Linux_arm64.tar.gz", "seictl_Darwin_x86_64.tar.gz", "seictl_Darwin_arm64.tar.gz"} {
		repair839Require(t, doc, archive)
	}
	for _, retired := range []string{"https://github.com/sei-protocol/seictl/releases", "go install github.com/sei-protocol/seictl@latest", "Go 1.25.4"} {
		if strings.Contains(doc, retired) {
			t.Errorf("obsolete installation: %s", retired)
		}
	}
	// Real CLI help is generated from main.go's command registrations.
	data, err := repair839Run(t, context.Background(), []string{"--help"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"network", "node", "workflow", "task"} {
		repair839Require(t, string(data), name)
	}
}

type repair839Server struct {
	url    string
	cancel context.CancelFunc
	done   chan error
	store  *server.TaskStore
	eng    *engine.Engine
	once   sync.Once
}

func repair839Open(t *testing.T, path, home string, handlers map[engine.TaskType]engine.TaskHandler) *repair839Server {
	t.Helper()
	store, err := server.NewTaskStore(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	eng := engine.NewEngine(ctx, handlers, store)
	eng.Config.Checkpointer = store
	eng.RehydrateStaleTasks()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		store.Close()
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	srv := server.NewServer(addr, eng, home, server.AuthnModeTrustedHeader)
	node := &repair839Server{url: "http://" + addr, cancel: cancel, done: make(chan error, 1), store: store, eng: eng}
	go func() { node.done <- srv.ListenAndServe(ctx) }()
	t.Cleanup(func() { node.stop(t) })
	deadline := time.Now().Add(3 * time.Second)
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, node.url+"/v0/livez", nil)
		response, err := (&http.Client{Timeout: time.Second}).Do(req)
		if err == nil {
			response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("livez status %d", response.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("sidecar listener: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return node
}

func (n *repair839Server) stop(t *testing.T) {
	t.Helper()
	n.once.Do(func() {
		n.cancel()
		select {
		case err := <-n.done:
			if err != nil {
				t.Errorf("sidecar shutdown: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("sidecar shutdown timed out")
		}
		if err := n.store.Close(); err != nil {
			t.Errorf("store close: %v", err)
		}
	})
}

func repair839Client(t *testing.T, url string) *sidecar.ClientWithResponses {
	t.Helper()
	client, err := sidecar.NewClientWithResponses(url, sidecar.WithHTTPClient(&http.Client{Timeout: time.Second}), sidecar.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
		r.Header.Set("X-Remote-User", "isolated-fixture-proxy")
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func repair839Wait(t *testing.T, store *server.TaskStore, id string) *engine.TaskResult {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		result, err := store.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if result != nil && result.Status != engine.TaskStatusRunning && string(result.Status) != "queued" {
			return result
		}
		if time.Now().After(deadline) {
			t.Fatalf("task %s did not finish: %+v", id, result)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRepairM_839_QueryHandlerSignatures(t *testing.T) {
	home := t.TempDir()
	node := repair839Open(t, filepath.Join(home, "sidecar.db"), home, nil)
	client := repair839Client(t, node.url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id := uuid.New()
	list, err := client.ListTasksWithResponse(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if list.StatusCode() != http.StatusOK || list.JSON200 == nil || len(*list.JSON200) != 0 || strings.TrimSpace(string(list.Body)) != "[]" {
		t.Fatalf("healthy empty store: %+v, body=%s", list, list.Body)
	}
	missing, err := client.GetTaskWithResponse(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if missing.StatusCode() != http.StatusNotFound {
		t.Fatalf("confirmed absence: %d %s", missing.StatusCode(), missing.Body)
	}
	// Closing the production SQLite store makes both real engine queries fail.
	// Keep the listener open to prove those errors survive the HTTP transport.
	if err := node.store.Close(); err != nil {
		t.Fatal(err)
	}
	list, err = client.ListTasksWithResponse(ctx)
	if err != nil {
		t.Fatal(err)
	}
	unavailable, err := client.GetTaskWithResponse(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, response := range []struct {
		status int
		header http.Header
		body   []byte
	}{
		{list.StatusCode(), list.HTTPResponse.Header, list.Body},
		{unavailable.StatusCode(), unavailable.HTTPResponse.Header, unavailable.Body},
	} {
		if response.status != http.StatusServiceUnavailable || response.header.Get("Retry-After") != "1" {
			t.Fatalf("store failure was hidden: %d %v %s", response.status, response.header, response.body)
		}
		var envelope server.ErrorResponse
		if err := json.Unmarshal(response.body, &envelope); err != nil || envelope.Error == "" {
			t.Fatalf("missing error envelope: %s, err=%v", response.body, err)
		}
	}
}

func TestRepairM_839_ArchitectureAndDurableRecovery(t *testing.T) {
	doc := repair839Read(t, "CLAUDE.md")
	repair839Require(t, doc, "one-shot", "no cron scheduler", "RehydrateStaleTasks", "sidecar.db", "workflow state-sync", "task snapshot-upload", "trusted-header", "127.0.0.1:7777", ":7777", "SEI_SIDECAR_ISOLATED_DEVELOPMENT=true", "--proxy-ca-file", "TokenReview", "SubjectAccessReview", "hostNetwork", "hostPID", "hostIPC", "413/429/503")
	t.Setenv("SEI_SIDECAR_AUTHN_MODE", "")
	t.Setenv("SEI_SIDECAR_ISOLATED_DEVELOPMENT", "")
	mode, err := server.AuthnMode()
	if err != nil || mode != server.AuthnModeTrustedHeader {
		t.Fatalf("default mode: %q %v", mode, err)
	}
	if got := server.BindAddress("7777", mode); got != "127.0.0.1:7777" {
		t.Fatalf("trusted listener: %s", got)
	}
	t.Setenv("SEI_SIDECAR_AUTHN_MODE", server.AuthnModeUnauthenticated)
	if _, err := server.AuthnMode(); err == nil {
		t.Fatal("unauthenticated mode accepted without isolation")
	}
	t.Setenv("SEI_SIDECAR_ISOLATED_DEVELOPMENT", "true")
	if mode, err := server.AuthnMode(); err != nil || server.BindAddress("7777", mode) != ":7777" {
		t.Fatalf("isolated development bind: %s %v", mode, err)
	}
	if _, err := repair839Run(t, context.Background(), []string{"serve"}); err == nil || !strings.Contains(err.Error(), "requires trusted-header") {
		t.Fatalf("serve accepted unauthenticated destructive handlers: %v", err)
	}
	t.Setenv("SEI_SIDECAR_AUTHN_MODE", server.AuthnModeTrustedHeader)

	home := t.TempDir()
	if err := tasks.EnsureDefaultConfig(home); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "sidecar.db")
	var height atomic.Int64
	seen := make(chan struct{})
	var once sync.Once
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			t.Errorf("unexpected RPC %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		once.Do(func() { close(seen) })
		fmt.Fprintf(w, `{"jsonrpc":"2.0","result":{"sync_info":{"latest_block_height":"%d","catching_up":false}}}`, height.Load())
	}))
	defer endpoint.Close()
	handlers := map[engine.TaskType]engine.TaskHandler{
		engine.TaskConfigPatch:    tasks.NewConfigPatcher(home).Handler(),
		engine.TaskAwaitCondition: tasks.NewConditionWaiter(rpc.NewStatusClient(endpoint.URL, endpoint.Client())).Handler(),
	}
	node := repair839Open(t, path, home, handlers)
	client := repair839Client(t, node.url)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Exercise the actual authenticated middleware, not just listener constants.
	for _, headers := range [][]string{nil, {""}, {"one", "two"}} {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, node.url+"/v0/tasks", nil)
		for _, value := range headers {
			req.Header.Add("X-Remote-User", value)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("headers %v: status %d", headers, response.StatusCode)
		}
	}
	if !slices.Equal(server.BypassPaths(), []string{"/v0/healthz", "/v0/livez", "/v0/metrics", "/v0/startupz"}) {
		t.Fatalf("probe boundary drifted: %v", server.BypassPaths())
	}
	patchReq := (sidecar.ConfigPatchTask{Files: map[string]map[string]any{"config.toml": {"moniker": "paxeer-smoke"}}}).ToTaskRequest()
	submitted, err := client.SubmitTaskWithResponse(ctx, patchReq)
	if err != nil || submitted.JSON201 == nil {
		t.Fatalf("real task submit: %+v %v", submitted, err)
	}
	patchID := submitted.JSON201.Id
	result := repair839Wait(t, node.store, patchID.String())
	if result.Status != engine.TaskStatusCompleted || result.Run != 1 {
		t.Fatalf("patch outcome: %+v", result)
	}
	config, err := patch.ReadTOML(filepath.Join(home, "config", "config.toml"))
	if err != nil || config["moniker"] != "paxeer-smoke" {
		t.Fatalf("real config consumer: %+v %v", config, err)
	}

	waitReq := (sidecar.AwaitConditionTask{Condition: sidecar.ConditionHeight, TargetHeight: 42}).ToTaskRequest()
	waiting, err := client.SubmitTaskWithResponse(ctx, waitReq)
	if err != nil || waiting.JSON201 == nil {
		t.Fatalf("real wait submit: %+v %v", waiting, err)
	}
	waitID := waiting.JSON201.Id
	select {
	case <-seen:
	case <-ctx.Done():
		t.Fatal("production handler never reached RPC fixture")
	}
	// Shutdown leaves running work durable. Reopen both the real SQLite store
	// and engine and let the production waiter finish at the recovered height.
	node.stop(t)
	height.Store(42)
	reopened := repair839Open(t, path, home, handlers)
	recovered := repair839Wait(t, reopened.store, waitID.String())
	if recovered.Status != engine.TaskStatusCompleted || recovered.Run != 1 {
		t.Fatalf("recovered task: %+v", recovered)
	}
	if recovered.Generation == "" || recovered.Params == nil {
		t.Fatalf("recovery metadata lost: %+v", recovered)
	}
	cli := repair839Client(t, reopened.url)
	for _, id := range []uuid.UUID{patchID, waitID} {
		response, err := cli.GetTaskWithResponse(ctx, id)
		if err != nil || response.StatusCode() != http.StatusOK || response.JSON200 == nil {
			t.Fatalf("recovered HTTP result: %+v %v", response, err)
		}
		var wireResult engine.TaskResult
		if err := json.Unmarshal(response.Body, &wireResult); err != nil {
			t.Fatal(err)
		}
		if string(response.JSON200.Status) != "completed" || wireResult.Run != 1 {
			t.Fatalf("one-shot result changed: %+v", wireResult)
		}
	}
	// Completed mutations remain inspectable after reopen without rerunning.
	persisted, err := reopened.store.Get(patchID.String())
	if err != nil || persisted == nil || !persisted.SubmittedAt.Equal(result.SubmittedAt) || persisted.Run != 1 {
		t.Fatalf("completed history changed: %+v %v", persisted, err)
	}
}
