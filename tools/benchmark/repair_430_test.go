package benchmark_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	benchmarkhome "github.com/Sidiora-Labs/Paxeer-X-Network/tools/benchmark"
)

type repair430Fixture struct {
	home   string
	repo   string
	target string
	state  map[string]string
}

func repair430NewFixture(t *testing.T) repair430Fixture {
	t.Helper()
	base := t.TempDir()
	f := repair430Fixture{
		home:   filepath.Join(base, "user"),
		repo:   filepath.Join(base, "repository"),
		target: filepath.Join(base, "benchmark with spaces", "node"),
		state:  make(map[string]string),
	}
	for _, dir := range []string{f.home, f.repo} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"config/priv_validator_key.json", "data/priv_validator_state.json", "keyring-test/admin", "data/application.db/CURRENT"} {
		path := filepath.Join(f.home, ".pax", name)
		value := "unrelated node: " + name
		repair430Write(t, path, value)
		f.state[path] = value
	}
	return f
}

func repair430Write(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}

func (f repair430Fixture) run(target, reset string) (string, error) {
	var output bytes.Buffer
	err := benchmarkhome.RunHome([]string{"--home", target, "--user-home", f.home, "--protected", f.repo, "--reset", reset}, &output)
	return output.String(), err
}

func (f repair430Fixture) assertProtected(t *testing.T) {
	t.Helper()
	for path, want := range f.state {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("protected node state %s: got %q, %v; want %q", path, data, err, want)
		}
	}
}

func TestRepair_430_DedicatedHomePreservesDefaultNode(t *testing.T) {
	f := repair430NewFixture(t)
	output, err := f.run(f.target, "")
	if err != nil {
		t.Fatal(err)
	}
	if output != f.target+"\n" {
		t.Fatalf("consumer home = %q, want %q", output, f.target+"\n")
	}
	entries, err := os.ReadDir(strings.TrimSuffix(output, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != ".pax-benchmark-home" {
		t.Fatalf("new home is not empty of node state: %v", entries)
	}
	info, err := os.Stat(filepath.Join(f.target, ".pax-benchmark-home"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("marker permissions = %o, want 600", info.Mode().Perm())
	}
	f.assertProtected(t)
}

func TestRepair_430_RejectsUnsafeHomes(t *testing.T) {
	f := repair430NewFixture(t)
	link := filepath.Join(filepath.Dir(f.home), "default-link")
	if err := os.Symlink(filepath.Join(f.home, ".pax"), link); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"empty": "", "root": string(os.PathSeparator), "relative": "relative/benchmark",
		"user-home": f.home, "ancestor": filepath.Dir(f.home), "repository": f.repo,
		"default":       filepath.Join(f.home, ".pax"),
		"default-child": filepath.Join(f.home, ".pax", "benchmark"),
		"symlink":       link, "symlink-parent": filepath.Join(link, "benchmark"),
		"parent-traversal": f.target + "/../other", "dot-component": f.target + "/./other",
		"newline": f.target + "\nother",
	}
	for name, target := range cases {
		t.Run(name, func(t *testing.T) {
			for _, reset := range []string{"", target} {
				output, err := f.run(target, reset)
				if err == nil || output != "" {
					t.Fatalf("unsafe home %q reset %q: output %q, error %v", target, reset, output, err)
				}
				f.assertProtected(t)
			}
		})
	}
	file := filepath.Join(filepath.Dir(f.home), "not-a-directory")
	repair430Write(t, file, "protected file")
	for _, target := range []string{file, filepath.Join(file, "child")} {
		if output, err := f.run(target, target); err == nil || output != "" {
			t.Fatalf("accepted file as home: %q, %v", output, err)
		}
	}
	ordinary := filepath.Join(filepath.Dir(f.home), "ordinary-node")
	repair430Write(t, filepath.Join(ordinary, "data", "state"), "ordinary node")
	alias := filepath.Join(filepath.Dir(f.home), "ordinary-alias")
	if err := os.Symlink(ordinary, alias); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{alias, filepath.Join(alias, "child")} {
		if output, err := f.run(target, target); err == nil || output != "" {
			t.Fatalf("accepted linked home: %q, %v", output, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(ordinary, "data", "state"))
	if err != nil || string(data) != "ordinary node" {
		t.Fatalf("linked node changed: %q, %v", data, err)
	}
}

func TestRepair_430_ExistingStateRequiresBoundedReset(t *testing.T) {
	f := repair430NewFixture(t)
	if _, err := f.run(f.target, ""); err != nil {
		t.Fatal(err)
	}
	oldState := filepath.Join(f.target, "data", "application.db", "CURRENT")
	repair430Write(t, oldState, "old benchmark database")
	for _, reset := range []string{"", "true", filepath.Dir(f.target), f.target + "/data", f.target + "/"} {
		output, err := f.run(f.target, reset)
		if err == nil || output != "" {
			t.Fatalf("unbounded reset %q accepted: %q, %v", reset, output, err)
		}
		data, err := os.ReadFile(oldState)
		if err != nil || string(data) != "old benchmark database" {
			t.Fatalf("state changed before confirmed reset: %q, %v", data, err)
		}
	}
	// A new invocation reopens the production directory and validates its marker.
	output, err := f.run(f.target, f.target)
	if err != nil || output != f.target+"\n" {
		t.Fatalf("confirmed reset: %q, %v", output, err)
	}
	if _, err := os.Stat(oldState); !os.IsNotExist(err) {
		t.Fatalf("old benchmark state remains: %v", err)
	}
	entries, err := os.ReadDir(f.target)
	if err != nil || len(entries) != 1 || entries[0].Name() != ".pax-benchmark-home" {
		t.Fatalf("reset home: %v, %v", entries, err)
	}
	f.assertProtected(t)
}

func TestRepair_430_RefusesUnownedState(t *testing.T) {
	for _, marker := range []string{"absent", "wrong-path", "symlink", "directory"} {
		t.Run(marker, func(t *testing.T) {
			f := repair430NewFixture(t)
			state := filepath.Join(f.target, "keyring-test", "admin")
			repair430Write(t, state, "existing account key")
			markerPath := filepath.Join(f.target, ".pax-benchmark-home")
			switch marker {
			case "wrong-path":
				repair430Write(t, markerPath, "pax-benchmark-home-v1\n"+f.target+"-other\n")
			case "symlink":
				outside := filepath.Join(f.home, "marker")
				repair430Write(t, outside, "pax-benchmark-home-v1\n"+f.target+"\n")
				if err := os.Symlink(outside, markerPath); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(markerPath, 0700); err != nil {
					t.Fatal(err)
				}
			}
			output, err := f.run(f.target, f.target)
			if err == nil || output != "" {
				t.Fatalf("unowned home accepted: %q, %v", output, err)
			}
			data, err := os.ReadFile(state)
			if err != nil || string(data) != "existing account key" {
				t.Fatalf("unowned state changed: %q, %v", data, err)
			}
			f.assertProtected(t)
		})
	}
}

func TestRepair_430_ResetCannotFollowLinksOutsideHome(t *testing.T) {
	f := repair430NewFixture(t)
	if _, err := f.run(f.target, ""); err != nil {
		t.Fatal(err)
	}
	for _, link := range []string{"data", "nested/keys"} {
		path := filepath.Join(f.target, link)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(f.home, ".pax"), path); err != nil {
			t.Fatal(err)
		}
	}
	// A sibling marked benchmark is never included in the reset boundary.
	sibling := f.target + "-sibling"
	if _, err := f.run(sibling, ""); err != nil {
		t.Fatal(err)
	}
	repair430Write(t, filepath.Join(sibling, "data", "state"), "sibling database")
	output, err := f.run(f.target, f.target)
	if err != nil || output != f.target+"\n" {
		t.Fatalf("bounded reset: %q, %v", output, err)
	}
	for _, name := range []string{"data", "nested"} {
		if _, err := os.Lstat(filepath.Join(f.target, name)); !os.IsNotExist(err) {
			t.Fatalf("reset entry %s remains: %v", name, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(sibling, "data", "state"))
	if err != nil || string(data) != "sibling database" {
		t.Fatalf("sibling state changed: %q, %v", data, err)
	}
	f.assertProtected(t)
}

func TestRepair_430_DefaultHomeAliasIsProtected(t *testing.T) {
	f := repair430NewFixture(t)
	target := filepath.Join(f.home, ".pax", "data")
	otherUser := filepath.Join(filepath.Dir(f.home), "other-user")
	if err := os.Mkdir(otherUser, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.home, ".pax"), filepath.Join(otherUser, ".pax")); err != nil {
		t.Fatal(err)
	}
	f.home = otherUser
	for _, reset := range []string{"", target} {
		if output, err := f.run(target, reset); err == nil || output != "" {
			t.Fatalf("default-home alias accepted: %q, %v", output, err)
		}
	}
	f.assertProtected(t)
}

func TestRepair_430_ScriptConsumesValidatedHome(t *testing.T) {
	data, err := os.ReadFile("benchmark.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, forbidden := range []string{"rm -rf", "~/.pax", "$HOME/.pax/config", "--overwrite"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("unsafe legacy home path remains: %s", forbidden)
		}
	}
	for _, required := range []string{
		`PAX_HOME=${PAX_HOME-"$HOME/.pax-benchmark"}`,
		`PAX_HOME=$(cd "$REPO_ROOT" && go run ./tools/benchmark/home`,
		`--home "$PAX_HOME" --user-home "$HOME" --protected "$REPO_ROOT"`,
		`--reset "$BENCHMARK_RESET_HOME")`, `export PAX_HOME`,
		`"$PAXD" init demo --chain-id pax-chain --home "$PAX_HOME"`,
		`PAXD_BIN="$PAXD" python3`,
		`PAXD_ARGS=(start --chain-id pax-chain --home "$PAX_HOME")`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("missing production home wiring: %s", required)
		}
	}
	prepare := strings.Index(script, "PAX_HOME=$(")
	init := strings.Index(script, `"$PAXD" init`)
	if prepare < 0 || init <= prepare {
		t.Fatal("home must be prepared before initializing the node")
	}
	for _, line := range strings.Split(script, "\n") {
		if strings.Contains(line, `"$PAXD"`) && !strings.Contains(line, `"${PAXD_ARGS[@]}"`) && !strings.Contains(line, `PAXD_BIN=`) && !strings.Contains(line, `--home "$PAX_HOME"`) {
			t.Fatalf("node command does not consume validated home: %s", line)
		}
	}
	// Drive exactly the helper's CLI transport into the filesystem consumer.
	f := repair430NewFixture(t)
	output, err := f.run(f.target, "")
	if err != nil {
		t.Fatal(err)
	}
	repair430Write(t, filepath.Join(strings.TrimSuffix(output, "\n"), "config", "genesis.json"), "dedicated genesis")
	f.assertProtected(t)
}
