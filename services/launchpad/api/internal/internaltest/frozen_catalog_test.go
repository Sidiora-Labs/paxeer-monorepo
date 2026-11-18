package internaltest

import (
	"net/http"
	"os/exec"
	"strings"
	"testing"
)

func TestFrozenV2OperationsDecodeCatalog(t *testing.T) {
	operations := FrozenV2Operations(t)
	const generate = `import importlib.util, json, pathlib, sys
root = pathlib.Path(sys.argv[1])
package = root / "sdk/launchpad/packages/sdk"
spec = importlib.util.spec_from_file_location("sdk_generate", package / "scripts/generate.py")
generator = importlib.util.module_from_spec(spec)
spec.loader.exec_module(generator)
recorded = json.loads((root / generator.RECORDED).read_bytes())
print(generator.current_operations(package, recorded), end="")
`
	raw, err := exec.Command("python3", "-c", generate, RepositoryRoot(t)).CombinedOutput()
	if err != nil {
		t.Fatalf("generate operation catalog: %v\n%s", err, raw)
	}
	produced := decodeFrozenV2Operations(t, raw)
	if len(operations) != len(produced) {
		t.Fatalf("decoded %d operations, want %d", len(operations), len(produced))
	}
	byID := make(map[string]FrozenOperation, len(operations))
	for _, op := range operations {
		if op.ID == "" || op.Method == "" || op.Service == "" {
			t.Errorf("operation missing identity, method or service: %+v", op)
		}
		if !strings.HasPrefix(op.Path, "/v") {
			t.Errorf("%s path %q does not start with /v", op.ID, op.Path)
		}
		if _, exists := byID[op.ID]; exists {
			t.Errorf("duplicate operation ID %q", op.ID)
		}
		byID[op.ID] = op
	}
	for _, want := range produced {
		if got := byID[want.ID]; got != want {
			t.Errorf("%s = %+v, want %+v", want.ID, got, want)
		}
	}
	want := FrozenOperation{ID: "IF_RANKINGS", Method: http.MethodGet, Path: "/v2/rankings/{category}", Service: "api", Response: "RankingsResponse"}
	if got := byID[want.ID]; got != want {
		t.Errorf("%s = %+v, want %+v", want.ID, got, want)
	}
	want = FrozenOperation{ID: "IF_ACCOUNT_DELETE", Method: http.MethodPost, Path: "/v2/me/deletion-requests", Service: "api", Body: "AccountDeleteBody", Response: "AccountDeleteResponse", Authenticated: true}
	if got := byID[want.ID]; got != want {
		t.Errorf("%s = %+v, want %+v", want.ID, got, want)
	}
}
