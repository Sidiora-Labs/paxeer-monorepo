package notifications

import (
	"go/version"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func repair461GoVersion(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	directive := regexp.MustCompile(`(?m)^\s*go\s+(\S+)\s*(?://[^\n]*)?$`).FindSubmatch(raw)
	if len(directive) != 2 {
		t.Fatalf("%s has no Go directive", path)
	}
	v := "go" + string(directive[1])
	if !version.IsValid(v) {
		t.Fatalf("%s has invalid Go version %q", path, v)
	}
	return v
}

func TestRepair461SharedWorkspace(t *testing.T) {
	workspacePath := filepath.Join("..", "go.work")
	raw, err := os.ReadFile(workspacePath)
	if err != nil {
		t.Fatal(err)
	}
	workspaceVersion := repair461GoVersion(t, workspacePath)
	useBlock := regexp.MustCompile(`(?ms)^use\s*\((.*?)^\)`).FindSubmatch(raw)
	if len(useBlock) != 2 {
		t.Fatal("shared workspace has no use block")
	}
	var members []string
	for _, line := range strings.Split(string(useBlock[1]), "\n") {
		line, _, _ = strings.Cut(line, "//")
		member := strings.TrimSpace(line)
		if member == "" {
			continue
		}
		if strings.HasPrefix(member, `"`) || strings.HasPrefix(member, "`") {
			member, err = strconv.Unquote(member)
			if err != nil {
				t.Fatalf("invalid workspace member %q: %v", line, err)
			}
		}
		members = append(members, member)
		t.Run(member, func(t *testing.T) {
			dir := filepath.Join("..", member)
			moduleVersion := repair461GoVersion(t, filepath.Join(dir, "go.mod"))
			if version.Compare(workspaceVersion, moduleVersion) < 0 {
				t.Errorf("shared workspace %s is below member %s requirement %s", workspaceVersion, member, moduleVersion)
			}
			nested := filepath.Join(dir, "go.work")
			if _, err := os.Lstat(nested); !os.IsNotExist(err) {
				t.Errorf("member %s must not carry its own go.work (stat error: %v)", member, err)
			}
		})
	}
	if len(members) == 0 {
		t.Fatal("shared workspace has no members to check")
	}
	t.Logf("checked shared workspace %s against %d members: %s", workspaceVersion, len(members), strings.Join(members, ", "))
}
