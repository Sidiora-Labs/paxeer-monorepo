package paxeer

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// SHA-256 digests of the restored blobs, independent of local Git history.
// Decision 2533 reconciles deployment.yaml with the maintained native runtime.
var repair467OriginalSHA256 = map[string]string{
	"evm.py":                     "7bb0ad0fe92b1cbb0d97ff6a6913a5116943866b5978aae3dbb3827af9d053d7",
	"deployment.yaml":            "ba84aff353486b744f3b9759fb158219b9de90558366485b483e4793c60dc9a2",
	"deployment-input.beta.json": "2c718a52f3d1e6bf1add92ef3ab54389a97c1d294d5a5563ec2649399481463b",
}

func repair467Restored(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		want, ok := repair467OriginalSHA256[name]
		if !ok {
			t.Fatalf("no historical digest for %s", name)
		}
		info, err := os.Lstat(name)
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("restored %s is not a regular file: %v", name, info.Mode())
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != want {
			t.Fatalf("%s differs from its restored content: SHA-256 %s, want %s", name, got, want)
		}
	}
}

func TestRepair467_RestoredContent(t *testing.T) {
	for _, name := range []string{"evm.py", "deployment.yaml", "deployment-input.beta.json"} {
		t.Run(name, func(t *testing.T) {
			repair467Restored(t, name)
		})
	}
}

func TestRepair467_PythonCompile(t *testing.T) {
	for _, name := range []string{"evm.py"} {
		t.Run(name, func(t *testing.T) {
			command := exec.Command("python3", "-m", "py_compile", name)
			command.Env = append(os.Environ(), "PYTHONPYCACHEPREFIX="+t.TempDir())
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("py_compile %s: %v\n%s", name, err, output)
			}
		})
	}
}

func TestRepair467_MergedOwnerLoader(t *testing.T) {
	directory, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	// Import the actual consumer so its own line-19 loader resolves the sibling file.
	command := exec.Command("python3", "-B", "-c", `
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]).parent / 'human'))
import merged_owner
expected = Path(sys.argv[1]) / 'evm.py'
assert Path(merged_owner.evm.__file__).resolve() == expected.resolve()
assert Path(merged_owner._spec.origin).resolve() == expected.resolve()
assert merged_owner.evm.keccak(b'').hex() == 'c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470'
assert merged_owner.rlp_decode(merged_owner.evm.rlp([1, b'paxeer'])) == [b'\x01', b'paxeer']
print(Path(merged_owner.evm.__file__).resolve())
`, directory)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("merged_owner EVM loader: %v\n%s", err, output)
	}
	if got, want := strings.TrimSpace(string(output)), filepath.Join(directory, "evm.py"); got != want {
		t.Fatalf("merged_owner loaded %q, want %q", got, want)
	}
}
