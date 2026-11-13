package release

// Release integrity checks cover compiler selection, source provenance,
// checksums, node process ownership and shared release locking.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// The compiler peers replay recorded -dumpfullversion responses only. They are
// never used to compile or to replace the pin checker under test.
func init() {
	if len(os.Args) == 2 && os.Args[1] == "-dumpfullversion" {
		switch filepath.Base(os.Args[0]) {
		case "gcc-13", "g++-13", "gcc", "g++":
			fmt.Println("13.3.0")
			os.Exit(0)
		case "cc", "cc-other", "gcc-other", "g++-other":
			fmt.Println("12.2.0")
			os.Exit(0)
		}
	}
}

func releaseSourceDirectory(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate release sources")
	}
	return filepath.Dir(file)
}

func writeReleaseTestFile(t *testing.T, path, value string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), mode); err != nil {
		t.Fatal(err)
	}
}

func readReleaseTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func releaseShellCommand(t *testing.T, dir, script string, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, "bash", append([]string{"-euo", "pipefail", "-c", script, "release-integrity"}, args...)...)
	cmd.Dir = dir
	// Own the environment of every peer; no operator environment is required.
	for _, entry := range os.Environ() {
		key := strings.SplitN(entry, "=", 2)[0]
		if key == "CC" || key == "CXX" || strings.HasPrefix(key, "PAX") || strings.HasPrefix(key, "BASH_FUNC_") || key == "BASH_ENV" || key == "ENV" {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	return cmd
}

func runReleaseCommand(t *testing.T, cmd *exec.Cmd, want bool, message string) string {
	t.Helper()
	out, err := cmd.CombinedOutput()
	if (err == nil) != want {
		t.Fatalf("%v: %v\n%s", cmd.Args, err, out)
	}
	if !strings.Contains(string(out), message) {
		t.Fatalf("missing %q in %s", message, out)
	}
	return string(out)
}

func releaseFileDigest(t *testing.T, path string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(readReleaseTestFile(t, path)))
	return hex.EncodeToString(sum[:])
}

func TestReleaseCompilerSelection(t *testing.T) {
	release := releaseSourceDirectory(t)
	dir := t.TempDir()
	writeReleaseTestFile(t, filepath.Join(dir, "tools/release/toolchain.env"), "CC_VERSION=13.3.0\n", 0600)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "drivers")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gcc-13", "g++-13", "cc-other", "gcc-other", "g++-other", "gcc", "g++"} {
		if err := os.Symlink(exe, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	// A differing cc alternative cannot validate a different gcc used downstream.
	if err := os.Symlink(exe, filepath.Join(bin, "cc")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		cc, cxx string
		pass    bool
	}{
		{"gcc-13", "g++-13", true}, {"gcc-other", "g++-13", false}, {"gcc-13", "g++-other", false}, {"missing", "g++-13", false},
	} {
		t.Run(tc.cc+"_"+tc.cxx, func(t *testing.T) {
			cmd := releaseShellCommand(t, dir, `source "$1/build-paxd.sh"; cc_pin; printf 'selected=%s,%s\n' "$CC" "$CXX"`, release)
			cmd.Env = append(cmd.Env, "CC="+filepath.Join(bin, tc.cc), "CXX="+filepath.Join(bin, tc.cxx), "PATH="+bin+":"+os.Getenv("PATH"))
			msg := "build-paxd:"
			if tc.pass {
				msg = "selected=" + filepath.Join(bin, tc.cc) + "," + filepath.Join(bin, tc.cxx)
			}
			runReleaseCommand(t, cmd, tc.pass, msg)
		})
	}
	cmd := releaseShellCommand(t, dir, `source "$1/build-paxd.sh"; cc_pin; printf 'selected=%s,%s\n' "$CC" "$CXX"`, release)
	cmd.Env = append(cmd.Env, "PATH="+bin+":"+os.Getenv("PATH"))
	runReleaseCommand(t, cmd, true, "selected="+filepath.Join(bin, "gcc")+","+filepath.Join(bin, "g++"))
	source := readReleaseTestFile(t, filepath.Join(release, "build-paxd.sh"))
	for _, required := range []string{`make --no-print-directory CC="$CC" CXX="$CXX"`, `export CC CXX`, `export CGO_ENABLED=1`} {
		if !strings.Contains(source, required) {
			t.Fatalf("selected drivers not transported to native/CGO build: %s", required)
		}
	}
	if strings.Contains(source, "CC=gcc") || strings.Contains(source, "cc -dumpfullversion") {
		t.Fatal("unvalidated compiler selection remains")
	}
}

func TestReleaseSourceProvenance(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	work, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "source.c")
	writeReleaseTestFile(t, path, "int source = 1;\n", 0600)
	if _, err := work.Add("source.c"); err != nil {
		t.Fatal(err)
	}
	signature := &object.Signature{Name: "release fixture", Email: "fixture@example.invalid", When: time.Unix(1700000000, 0)}
	first, err := work.Commit("recorded release source", &git.CommitOptions{Author: signature, Committer: signature})
	if err != nil {
		t.Fatal(err)
	}
	release := releaseSourceDirectory(t)
	invoke := func(want bool, msg, revision string) {
		cmd := releaseShellCommand(t, dir, `source "$1/build-paxd.sh"; release_source; printf '%s\n' "$commit"`, release)
		if revision != "" {
			cmd.Env = append(cmd.Env, "PAXD_COMMIT="+revision)
		}
		runReleaseCommand(t, cmd, want, msg)
	}
	invoke(true, first.String(), "")
	invoke(true, first.String(), "HEAD")
	// Deliberately modified tracked source, including a staged modification.
	writeReleaseTestFile(t, path, "int source = 2;\n", 0600)
	invoke(false, "tracked source changes", first.String())
	if _, err := work.Add("source.c"); err != nil {
		t.Fatal(err)
	}
	invoke(false, "tracked source changes", first.String())
	second, err := work.Commit("next recorded source", &git.CommitOptions{Author: signature, Committer: signature})
	if err != nil {
		t.Fatal(err)
	}
	invoke(false, "differs from the checkout revision", first.String())
	invoke(true, second.String(), second.String())
	writeReleaseTestFile(t, filepath.Join(dir, "untracked.go"), "package surprise\n", 0600)
	invoke(false, "untracked source files", "")
	source := readReleaseTestFile(t, filepath.Join(release, "build-paxd.sh"))
	if !strings.Contains(source, `printf '%s\n' "$commit" > "$out/source-revision.txt"`) {
		t.Fatal("resolved source revision missing from checksummed evidence")
	}
}

func TestReleaseRetiredAssetsAndChecksums(t *testing.T) {
	release := releaseSourceDirectory(t)
	dir := t.TempDir()
	cmd := releaseShellCommand(t, dir, `source "$1/build-paxd.sh"; release_assets`, release)
	cmd.Env = append(cmd.Env, "PAXD_RELEASE_LAYERXD=1")
	runReleaseCommand(t, cmd, false, "PAXD_RELEASE_LAYERXD is retired")
	cmd = releaseShellCommand(t, dir, `source "$1/build-paxd.sh"; release_assets`, release)
	runReleaseCommand(t, cmd, true, "")
	assets := []string{"paxd-v6.12.0-linux-amd64", "price-sidecar-v6.12.0-linux-amd64", "native-libs.sha256", "libwasmvm.x86_64.so", "source-revision.txt"}
	for _, asset := range assets {
		writeReleaseTestFile(t, filepath.Join(dir, asset), "recorded asset bytes: "+asset+"\n", 0600)
	}
	cmd = releaseShellCommand(t, dir, `source "$1/build-paxd.sh"; out=$PWD; write_sums; sha256sum --check --strict SHA256SUMS`, release)
	runReleaseCommand(t, cmd, true, "source-revision.txt: OK")
	lines := strings.Split(strings.TrimSpace(readReleaseTestFile(t, filepath.Join(dir, "SHA256SUMS"))), "\n")
	if len(lines) != len(assets) {
		t.Fatalf("wrong checksummed asset count: %v", lines)
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != releaseFileDigest(t, filepath.Join(dir, fields[1])) {
			t.Fatalf("bad checksum evidence: %q", line)
		}
	}
	writeReleaseTestFile(t, filepath.Join(dir, assets[0]), "tampered", 0600)
	runReleaseCommand(t, releaseShellCommand(t, dir, `sha256sum --check --strict SHA256SUMS`), false, "FAILED")
	source := readReleaseTestFile(t, filepath.Join(release, "build-paxd.sh"))
	for _, retired := range []string{"make layerxd", "layerxd-$version", "daemon/layerxd"} {
		if strings.Contains(source, retired) {
			t.Fatalf("retired asset path: %s", retired)
		}
	}
	for _, retained := range []string{"native-libs.sha256", "NativeArchiveDigest", "libwasmvm152", "libwasmvm155", "price-sidecar", "sha256sum --check --strict"} {
		if !strings.Contains(source, retained) {
			t.Fatalf("lost reproducibility check: %s", retained)
		}
	}
}

// A process peer supplies actual Linux argv/environment/cwd/executable identity.
// The component under repair is the release ownership resolver, not a node.
func TestReleaseProcessPeer(t *testing.T) {
	if filepath.Base(os.Args[0]) != "paxd" {
		return
	}
	// Signal readiness only after the testing runtime has initialized this peer.
	if _, err := os.Stdout.Write([]byte("ready\n")); err != nil {
		os.Exit(2)
	}
	select {}
}

type releaseProcessPeer struct {
	cmd  *exec.Cmd
	done bool
}

func (p *releaseProcessPeer) stop() {
	if !p.done {
		_ = p.cmd.Process.Kill()
		_ = p.cmd.Wait()
		p.done = true
	}
}

func startReleaseProcessPeer(t *testing.T, dir, selection, home string) *releaseProcessPeer {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(t.TempDir(), "paxd")
	if err := os.Symlink(exe, name); err != nil {
		t.Fatal(err)
	}
	args := []string{"-test.run=^TestReleaseProcessPeer$", "--", "start"}
	switch selection {
	case "explicit":
		args = append(args, "--home", home)
	case "equals":
		args = append(args, "--home="+home)
	case "relative":
		args = append(args, "--home", "./"+filepath.Base(home))
	case "empty":
		args = append(args, "--home=")
	case "duplicate":
		args = append(args, "--home", home, "--home", home)
	case "environment", "default":
	default:
		t.Fatal(selection)
	}
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "PAX_HOME=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	if selection == "environment" {
		cmd.Env = append(cmd.Env, "PAX_HOME="+home)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	peer := &releaseProcessPeer{cmd: cmd}
	t.Cleanup(peer.stop)
	ready := make(chan error, 1)
	go func() {
		buf := make([]byte, 6)
		_, err := io.ReadFull(stdout, buf)
		if err == nil && string(buf) != "ready\n" {
			err = fmt.Errorf("unexpected readiness %q", buf)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatalf("process peer readiness: %v %s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("process peer readiness timed out")
	}
	return peer
}

func TestReleaseHomeResolver(t *testing.T) {
	release := releaseSourceDirectory(t)
	for _, selection := range []string{"explicit", "equals", "relative", "environment", "default", "empty", "duplicate"} {
		t.Run(selection, func(t *testing.T) {
			dir := t.TempDir()
			home := filepath.Join(dir, "home")
			if err := os.Mkdir(home, 0700); err != nil {
				t.Fatal(err)
			}
			peer := startReleaseProcessPeer(t, dir, selection, home)
			list := func(target string, want bool, msg string) {
				runReleaseCommand(t, releaseShellCommand(t, dir, `source "$1/stage.sh"; release_node_process list "$2"`, release, target), want, msg)
			}
			ambiguous := selection == "default" || selection == "empty" || selection == "duplicate"
			if ambiguous {
				list(home, false, "release:")
				list(filepath.Join(dir, "unrelated"), false, "release:")
			} else {
				list(home, true, fmt.Sprint(peer.cmd.Process.Pid))
				list(filepath.Join(dir, "unrelated"), true, "")
				alias := filepath.Join(dir, "alias")
				if err := os.Symlink(home, alias); err != nil {
					t.Fatal(err)
				}
				list(alias, true, fmt.Sprint(peer.cmd.Process.Pid))
				digest := releaseFileDigest(t, "/proc/"+fmt.Sprint(peer.cmd.Process.Pid)+"/exe")
				runReleaseCommand(t, releaseShellCommand(t, dir, `source "$1/stage.sh"; release_node_process verify "$2" "$3"`, release, home, digest), true, "")
				runReleaseCommand(t, releaseShellCommand(t, dir, `source "$1/stage.sh"; release_node_process verify "$2" "$3"`, release, home, strings.Repeat("0", 64)), false, "sha256 mismatch")
			}
			runReleaseCommand(t, releaseShellCommand(t, dir, `source "$1/stage.sh"; release_node_process stopped "$2"`, release, home), false, "release:")
			if !ambiguous {
				runReleaseCommand(t, releaseShellCommand(t, dir, `source "$1/stage.sh"; release_node_process stop "$2"`, release, home), true, "")
				if err := peer.cmd.Wait(); err == nil {
					t.Fatal("process was not terminated by the resolver")
				}
				peer.done = true
			} else {
				peer.stop()
			}
			runReleaseCommand(t, releaseShellCommand(t, dir, `source "$1/stage.sh"; release_node_process stopped "$2"`, release, home), true, "")
		})
	}
}

func createReleaseNodeHome(t *testing.T, dir string) string {
	t.Helper()
	home := filepath.Join(dir, "home")
	writeReleaseTestFile(t, filepath.Join(home, "config/app.toml"), "halt-height = 20\n", 0600)
	writeReleaseTestFile(t, filepath.Join(home, "data/priv_validator_state.json"), `{"height":"31","round":2,"step":3}`, 0600)
	writeReleaseTestFile(t, filepath.Join(home, "data/committer.db/commit"), "committed state", 0600)
	return home
}

func readReleaseHomeTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			result[strings.TrimPrefix(path, dir)] = readReleaseTestFile(t, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func writeRollbackEnvironment(t *testing.T, dir, home string) string {
	t.Helper()
	bin := filepath.Join(dir, "bin")
	snap := filepath.Join(dir, "snapshots")
	for _, path := range []string{bin, snap} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "rollback.env")
	writeReleaseTestFile(t, path, fmt.Sprintf("RELEASE_ID=r1\nBIN_DIR=%s\nSNAPSHOT_DIR=%s\nHALT_HEIGHT=20\nVALIDATOR_HOMES=%s\nVALIDATOR_UNITS=validator.service\nFULLNODE_HOMES=\nFULLNODE_UNITS=\n", bin, snap, home), 0600)
	return path
}

func rollbackCommand(t *testing.T, dir, env, phase string) *exec.Cmd {
	t.Helper()
	return releaseShellCommand(t, dir, `exec bash "$1/rollback.sh" --env-file "$2" --local --execute --phase "$3" --service-manager foreground`, releaseSourceDirectory(t), env, phase)
}

func TestReleaseRollbackLiveHomes(t *testing.T) {
	for _, selection := range []string{"explicit", "relative", "environment", "default"} {
		t.Run(selection, func(t *testing.T) {
			dir := t.TempDir()
			home := createReleaseNodeHome(t, dir)
			env := writeRollbackEnvironment(t, dir, home)
			peer := startReleaseProcessPeer(t, dir, selection, home)
			before := readReleaseHomeTree(t, home)
			for _, phase := range []string{"prepare", "restore"} {
				runReleaseCommand(t, rollbackCommand(t, dir, env, phase), false, "node ownership is not stopped")
			}
			if !reflect.DeepEqual(before, readReleaseHomeTree(t, home)) {
				t.Fatal("live home changed during refused rollback")
			}
			if _, err := os.Stat(filepath.Join(dir, "bin/paxd.new")); !os.IsNotExist(err) {
				t.Fatalf("binary staging changed: %v", err)
			}
			peer.stop()
		})
	}
}

func swapCommand(t *testing.T, dir, home string) *exec.Cmd {
	t.Helper()
	cmd := releaseShellCommand(t, dir, `exec bash "$1/swap.sh" --role validator --service-manager foreground`, releaseSourceDirectory(t))
	cmd.Env = append(cmd.Env, "BIN_DIR="+filepath.Join(dir, "bin"), "RELEASE_SHA256="+strings.Repeat("a", 64), "HALT_HEIGHT=20", "VALIDATOR_HOMES="+home)
	return cmd
}

func TestReleaseSwapLiveHomes(t *testing.T) {
	for _, selection := range []string{"explicit", "equals", "relative", "environment", "default"} {
		t.Run(selection, func(t *testing.T) {
			dir := t.TempDir()
			home := createReleaseNodeHome(t, dir)
			if err := os.Mkdir(filepath.Join(dir, "bin"), 0700); err != nil {
				t.Fatal(err)
			}
			peer := startReleaseProcessPeer(t, dir, selection, home)
			before := readReleaseHomeTree(t, home)
			message := "still running"
			if selection == "default" {
				message = "process inspection failed"
			}
			runReleaseCommand(t, swapCommand(t, dir, home), false, message)
			if !reflect.DeepEqual(before, readReleaseHomeTree(t, home)) {
				t.Fatal("live home changed during refused swap")
			}
			peer.stop()
		})
	}
}

func createStageFixture(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, "bin")
	dist := filepath.Join(dir, "dist")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dist, 0700); err != nil {
		t.Fatal(err)
	}
	// ELF fixtures exercise the actual checksum and dynamic-loader path; these
	// programs are never invoked as substitute node/sidecar implementations.
	for dst, src := range map[string]string{filepath.Join(bin, "paxd"): "/bin/bash", filepath.Join(dist, "paxd-v6.12.0-linux-amd64"): "/usr/bin/env", filepath.Join(dist, "price-sidecar-v6.12.0-linux-amd64"): "/usr/bin/printf"} {
		writeReleaseTestFile(t, dst, readReleaseTestFile(t, src), 0755)
	}
	template := readReleaseTestFile(t, filepath.Join(releaseSourceDirectory(t), "env/rollout.env.example"))
	values := map[string]string{"RELEASE_ID": "r1", "RELEASE_TAG": "paxeer-network/v6.12.0", "HALT_HEIGHT": "20", "BIN_DIR": bin, "RELEASE_SHA256": releaseFileDigest(t, filepath.Join(dist, "paxd-v6.12.0-linux-amd64")), "SIDECAR_SHA256": releaseFileDigest(t, filepath.Join(dist, "price-sidecar-v6.12.0-linux-amd64"))}
	var env strings.Builder
	for _, line := range strings.Split(template, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key := strings.TrimSuffix(line, "=")
		value := values[key]
		if value == "" {
			value = "fixture"
		}
		fmt.Fprintf(&env, "%s='%s'\n", key, value)
	}
	path := filepath.Join(dir, "stage.env")
	writeReleaseTestFile(t, path, env.String(), 0600)
	return path
}

func stageCommand(t *testing.T, dir, env, receipt string) *exec.Cmd {
	t.Helper()
	return releaseShellCommand(t, filepath.Join(dir, "dist"), `exec bash "$1/stage.sh" --env-file "$2" --local --receipt "$3"`, releaseSourceDirectory(t), env, receipt)
}

func TestReleaseSharedLockAndRecovery(t *testing.T) {
	dir := t.TempDir()
	home := createReleaseNodeHome(t, dir)
	stageEnv := createStageFixture(t, dir)
	rollbackEnv := writeRollbackEnvironment(t, dir, home)
	release := releaseSourceDirectory(t)
	receipt := filepath.Join(dir, "stage-receipt.json")
	// Pause a real staging command immediately before its first install, while
	// it owns the production lock. DEBUG controls scheduling only; every file and
	// checksum operation still executes the production implementation.
	ownerScript := `source "$1/stage.sh"
set -T
trap 'if [[ $BASH_COMMAND == "install -m 0755 -- ./paxd-v6.12.0-linux-amd64"* ]]; then printf "locked\n"; read -r; trap - DEBUG; fi' DEBUG
stage_main --env-file "$2" --local --receipt "$3"`
	holder := releaseShellCommand(t, filepath.Join(dir, "dist"), ownerScript, release, stageEnv, receipt)
	stdin, err := holder.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := holder.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var ownerErr bytes.Buffer
	holder.Stderr = &ownerErr
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	held := true
	t.Cleanup(func() {
		if held {
			_ = holder.Process.Kill()
			_ = holder.Wait()
		}
	})
	ready := make([]byte, 7)
	if _, err := io.ReadFull(stdout, ready); err != nil || string(ready) != "locked\n" {
		t.Fatalf("lock holder: %q %v %s", ready, err, ownerErr.String())
	}
	before := readReleaseHomeTree(t, home)
	binaryBefore := releaseFileDigest(t, filepath.Join(dir, "bin/paxd"))
	commands := []*exec.Cmd{stageCommand(t, dir, stageEnv, receipt), swapCommand(t, dir, home), rollbackCommand(t, dir, rollbackEnv, "restore")}
	type outcome struct {
		out string
		err error
	}
	results := make([]outcome, len(commands))
	var wg sync.WaitGroup
	for i, cmd := range commands {
		wg.Add(1)
		go func(i int, cmd *exec.Cmd) {
			defer wg.Done()
			out, err := cmd.CombinedOutput()
			results[i] = outcome{string(out), err}
		}(i, cmd)
	}
	wg.Wait()
	for i, result := range results {
		if result.err == nil || !strings.Contains(result.out, "another release operation") {
			t.Fatalf("mutator %d bypassed shared lock: %v %s", i, result.err, result.out)
		}
	}
	if !reflect.DeepEqual(before, readReleaseHomeTree(t, home)) || binaryBefore != releaseFileDigest(t, filepath.Join(dir, "bin/paxd")) {
		t.Fatal("contending command mutated state")
	}
	if _, err := os.Stat(filepath.Join(dir, "bin/paxd.v6.12.0")); !os.IsNotExist(err) {
		t.Fatalf("contender staged binary: %v", err)
	}
	// Let the sole mutator complete and publish its evidence, then reopen it.
	if _, err := io.WriteString(stdin, "continue\n"); err != nil {
		t.Fatal(err)
	}
	_ = stdin.Close()
	if err := holder.Wait(); err != nil {
		t.Fatalf("staging owner: %v %s", err, ownerErr.String())
	}
	held = false
	// Reacquire from another staging process and simulate termination while it
	// owns the lock. A fresh invocation must recover without replacing previous.
	interrupted := releaseShellCommand(t, filepath.Join(dir, "dist"), ownerScript, release, stageEnv, receipt)
	interruptIn, err := interrupted.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	interruptOut, err := interrupted.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := interrupted.Start(); err != nil {
		t.Fatal(err)
	}
	defer interruptIn.Close()
	if _, err := io.ReadFull(interruptOut, ready); err != nil {
		_ = interrupted.Process.Kill()
		_ = interrupted.Wait()
		t.Fatal(err)
	}
	_ = interrupted.Process.Kill()
	_ = interrupted.Wait()
	runReleaseCommand(t, stageCommand(t, dir, stageEnv, receipt), true, "")
	staged := releaseFileDigest(t, filepath.Join(dir, "bin/paxd.v6.12.0"))
	if staged != releaseFileDigest(t, filepath.Join(dir, "dist/paxd-v6.12.0-linux-amd64")) {
		t.Fatal("staged evidence differs from binary")
	}
	if binaryBefore != releaseFileDigest(t, filepath.Join(dir, "bin/paxd.pre-r1")) {
		t.Fatal("retained previous binary identity lost")
	}
	var evidence struct {
		Hosts  int    `json:"hosts_staged"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(readReleaseTestFile(t, receipt)), &evidence); err != nil || evidence.Hosts != 1 || evidence.Result != "pass" {
		t.Fatalf("bad receipt: %+v %v", evidence, err)
	}
	// A separate process reopens material and validates it under the same lock.
	verify := releaseShellCommand(t, dir, `exec bash "$1/stage.sh" --env-file "$2" --local --verify --receipt "$3"`, release, stageEnv, receipt)
	runReleaseCommand(t, verify, true, "")
	if staged != releaseFileDigest(t, filepath.Join(dir, "bin/paxd.v6.12.0")) {
		t.Fatal("reopened release material changed")
	}
	entries, err := os.ReadDir(filepath.Join(dir, "bin"))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if strings.Contains(strings.Join(names, " "), ".stage.lock") || strings.Contains(strings.Join(names, " "), ".rollback.lock") || strings.Contains(strings.Join(names, " "), ".paxd-swap.lock") {
		t.Fatalf("disjoint lock recreated: %v", names)
	}
}
