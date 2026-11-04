package releaseproof

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Runner executes the actual artifact. Tests invoke the same CLI in process;
// release automation executes the downloaded binary on its native platform.
type Runner func(context.Context, []string) ([]byte, error)

type object struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name        string            `json:"name"`
		Namespace   string            `json:"namespace"`
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
}

func checkObject(data []byte, kind, expected string) error {
	var obj object
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	if obj.APIVersion != "sei.io/v1alpha1" || obj.Kind != kind || obj.Metadata.Name != "release-probe" || obj.Metadata.Namespace != "release-probe" {
		return fmt.Errorf("unexpected resource identity: %s %s %s/%s", obj.APIVersion, obj.Kind, obj.Metadata.Namespace, obj.Metadata.Name)
	}
	if got := obj.Metadata.Annotations["seictl.sei.io/version"]; got != expected {
		return fmt.Errorf("%s provenance %q, expected %q", kind, got, expected)
	}
	return nil
}

// Verify drives all three production renderers through discovery and dry-run
// server-side apply, checking both the HTTP request and the CLI's returned CR.
// The fixture never persists resources and needs no cluster or credentials.
func Verify(ctx context.Context, expected string, run Runner) error {
	if err := ValidateVersion(expected); err != nil {
		return err
	}
	var mu sync.Mutex
	received := map[string]int{}
	var fixtureErr error
	kinds := map[string]string{"seinetworks": "SeiNetwork", "seinodes": "SeiNode", "seinodetaskworkflows": "SeiNodeTaskWorkflow"}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			switch r.URL.Path {
			case "/api":
				io.WriteString(w, `{"kind":"APIVersions","apiVersion":"v1","versions":[],"serverAddressByClientCIDRs":[]}`)
			case "/apis":
				io.WriteString(w, `{"kind":"APIGroupList","apiVersion":"v1","groups":[{"name":"sei.io","versions":[{"groupVersion":"sei.io/v1alpha1","version":"v1alpha1"}],"preferredVersion":{"groupVersion":"sei.io/v1alpha1","version":"v1alpha1"}}]}`)
			case "/apis/sei.io/v1alpha1":
				io.WriteString(w, discovery)
			default:
				http.Error(w, "unexpected discovery path", http.StatusNotFound)
			}
			return
		}
		resource := strings.TrimPrefix(r.URL.Path, "/apis/sei.io/v1alpha1/namespaces/release-probe/")
		resource = strings.TrimSuffix(resource, "/release-probe")
		kind, ok := kinds[resource]
		data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err == nil && (!ok || r.Method != http.MethodPatch || r.URL.Query().Get("dryRun") != "All" || r.URL.Query().Get("fieldManager") != "seictl" || r.URL.Query().Get("force") != "true" || r.Header.Get("Content-Type") != "application/apply-patch+yaml") {
			err = fmt.Errorf("unexpected SSA request: %s %s", r.Method, r.URL)
		}
		if err == nil {
			err = checkObject(data, kind, expected)
		}
		mu.Lock()
		if err != nil {
			fixtureErr = err
		} else {
			received[kind]++
		}
		mu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Write(data)
	}))
	defer fixture.Close()
	dir, err := os.MkdirTemp("", "paxctl-release-proof-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	kubeconfig := filepath.Join(dir, "config")
	config := fmt.Sprintf("apiVersion: v1\nkind: Config\nclusters:\n- name: probe\n  cluster:\n    server: %s\ncontexts:\n- name: probe\n  context:\n    cluster: probe\n    user: probe\n    namespace: release-probe\ncurrent-context: probe\nusers:\n- name: probe\n  user: {}\n", fixture.URL)
	if err := os.WriteFile(kubeconfig, []byte(config), 0600); err != nil {
		return err
	}
	cases := []struct {
		tree, kind string
		flags      []string
	}{
		{"network", "SeiNetwork", []string{"--preset", "genesis-chain", "--chain-id", "paxeer-release-probe", "--image", "registry.example.invalid/paxd:probe"}},
		{"node", "SeiNode", []string{"--preset", "rpc", "--chain-id", "paxeer-release-probe", "--image", "registry.example.invalid/paxd:probe", "--network", "release-probe"}},
		{"workflow", "SeiNodeTaskWorkflow", []string{"--preset", "state-sync", "--target", "release-probe"}},
	}
	for _, c := range cases {
		args := []string{c.tree, "apply", "--kubeconfig", kubeconfig, "-n", "release-probe", "--dry-run"}
		args = append(args, c.flags...)
		args = append(args, "release-probe")
		data, err := run(ctx, args)
		if err != nil {
			return fmt.Errorf("%s artifact probe: %w", c.tree, err)
		}
		if err := checkObject(data, c.kind, expected); err != nil {
			return fmt.Errorf("%s CLI output: %w", c.tree, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if fixtureErr != nil {
		return fixtureErr
	}
	for _, kind := range kinds {
		if received[kind] != 1 {
			return fmt.Errorf("%s: expected one real SSA request, got %d", kind, received[kind])
		}
	}
	return nil
}

const discovery = `{"kind":"APIResourceList","apiVersion":"v1","groupVersion":"sei.io/v1alpha1","resources":[{"name":"seinetworks","singularName":"seinetwork","namespaced":true,"kind":"SeiNetwork","verbs":["get","list","patch"]},{"name":"seinodes","singularName":"seinode","namespaced":true,"kind":"SeiNode","verbs":["get","list","patch"]},{"name":"seinodetaskworkflows","singularName":"seinodetaskworkflow","namespaced":true,"kind":"SeiNodeTaskWorkflow","verbs":["get","list","patch"]}]}`
