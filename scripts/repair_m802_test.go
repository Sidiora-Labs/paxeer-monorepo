package scripts_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	scriptops "github.com/Sidiora-Labs/Paxeer-X-Network/scripts"
	"github.com/go-git/go-billy/v5/memfs"
	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/spf13/viper"
)

func requireM802(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func readM802(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	requireM802(t, err)
	return data
}
func writeM802(t *testing.T, path string, data []byte) {
	t.Helper()
	requireM802(t, os.MkdirAll(filepath.Dir(path), 0700))
	requireM802(t, os.WriteFile(path, data, 0600))
}

func TestRepairM_802_Finding16_NodeExportArchive(t *testing.T) {
	// A committed, actual paxd export, not a constructed replacement for the node.
	transcript := readM802(t, "../node/testdata/v6120-halt-export.json")
	const height int64 = 34742400
	output := filepath.Join(t.TempDir(), "state.zip")
	args, err := scriptops.ExportArguments("/offline node", height)
	requireM802(t, err)
	expectedArgs := []string{"export", "--home", "/offline node", "--height", "34742400", "--streaming=false"}
	if !reflect.DeepEqual(args, expectedArgs) {
		t.Fatalf("node CLI arguments: %q", args)
	}
	requireM802(t, scriptops.PublishExportResult(bytes.NewReader(transcript), nil, height, output))
	archive, err := zip.OpenReader(output)
	requireM802(t, err)
	defer archive.Close()
	contents := map[string][]byte{}
	for _, entry := range archive.File {
		reader, openErr := entry.Open()
		requireM802(t, openErr)
		contents[entry.Name], err = io.ReadAll(reader)
		requireM802(t, err)
		requireM802(t, reader.Close())
	}
	if !bytes.Equal(contents["genesis.json"], transcript) {
		t.Fatal("exported application, validators or consensus state was omitted or modified")
	}
	var manifest scriptops.StateManifest
	requireM802(t, json.Unmarshal(contents["manifest.json"], &manifest))
	if manifest.Height != height || manifest.InitialHeight != height+1 || manifest.Interface != "paxd export" {
		t.Fatalf("wrong height/interface manifest: %+v", manifest)
	}
	var producer struct {
		AppState map[string]json.RawMessage `json:"app_state"`
	}
	requireM802(t, json.Unmarshal(transcript, &producer))
	expected := make([]string, 0, len(producer.AppState))
	for name := range producer.AppState {
		expected = append(expected, name)
	}
	sort.Strings(expected)
	if !reflect.DeepEqual(manifest.Stores, expected) {
		t.Fatalf("store inventory mismatch: got %q want %q", manifest.Stores, expected)
	}
	for _, name := range []string{"evm", "layerxcustody", "layerxanchor", "layerxbridge", "sequencer", "xweb"} {
		if _, ok := producer.AppState[name]; !ok {
			t.Fatalf("real export fixture lacks current store %s", name)
		}
	}
	if len(contents) != 2 || !strings.Contains(manifest.Scope, "not a physical/native database snapshot") {
		t.Fatal("archive must disclose the logical export boundary")
	}
}

func TestRepairM_802_Finding16_FailedExportsNeverPublish(t *testing.T) {
	transcript := readM802(t, "../node/testdata/v6120-halt-export.json")
	output := filepath.Join(t.TempDir(), "existing.zip")
	sentinel := []byte("previous qualified archive")
	writeM802(t, output, sentinel)
	failures := []struct {
		name   string
		data   []byte
		status error
		height int64
	}{
		{"node-failed-after-writing", transcript, errors.New("recorded node export failure"), 34742400},
		{"truncated", transcript[:len(transcript)/2], nil, 34742400},
		{"wrong-height", transcript, nil, 34742399},
		{"invalid-height", transcript, nil, 0},
		{"trailing-document", append(append([]byte{}, transcript...), []byte(" {}")...), nil, 34742400},
	}
	var state map[string]json.RawMessage
	requireM802(t, json.Unmarshal(transcript, &state))
	var modules map[string]json.RawMessage
	requireM802(t, json.Unmarshal(state["app_state"], &modules))
	delete(modules, "evm")
	state["app_state"], _ = json.Marshal(modules)
	unsupported, err := json.Marshal(state)
	requireM802(t, err)
	failures = append(failures, struct {
		name   string
		data   []byte
		status error
		height int64
	}{"legacy-layout", unsupported, nil, 34742400})
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			if err := scriptops.PublishExportResult(bytes.NewReader(tc.data), tc.status, tc.height, output); err == nil {
				t.Fatal("bad export published")
			}
			if !bytes.Equal(readM802(t, output), sentinel) {
				t.Fatal("failed export replaced previous archive")
			}
			entries, err := os.ReadDir(filepath.Dir(output))
			requireM802(t, err)
			if len(entries) != 1 {
				t.Fatalf("failed export leaked staging files: %v", entries)
			}
		})
	}
	closed, err := os.Open("../node/testdata/v6120-halt-export.json")
	requireM802(t, err)
	requireM802(t, closed.Close())
	if err := scriptops.PublishExportResult(closed, nil, 34742400, output); err == nil {
		t.Fatal("read failure must prevent publication")
	}
	if !bytes.Equal(readM802(t, output), sentinel) {
		t.Fatal("read failure changed archive")
	}
}

func TestRepairM_802_Finding11_DisposableHomeAcknowledgement(t *testing.T) {
	home := t.TempDir()
	operator := filepath.Join(home, ".pax")
	state := filepath.Join(operator, "data", "operator-state")
	writeM802(t, state, []byte("do not erase"))
	cfg, err := scriptops.NewStressConfig(home, "", "")
	requireM802(t, err)
	if cfg.Home == operator || !strings.HasPrefix(cfg.Home, home+string(os.PathSeparator)+".pax-evm-stress-") {
		t.Fatalf("unsafe default stress home: %s", cfg.Home)
	}
	normalized, err := scriptops.ValidateLocalHome(home, cfg.Home)
	requireM802(t, err)
	if normalized != cfg.Home {
		t.Fatal("stress producer and initializer disagree about selected home")
	}
	if _, err := scriptops.NewStressConfig(home, "", operator); err == nil {
		t.Fatal("existing operator home was accepted as disposable")
	}
	if string(readM802(t, state)) != "do not erase" {
		t.Fatal("operator state changed")
	}
	outside := t.TempDir()
	if _, err := scriptops.NewStressConfig(home, "", filepath.Join(outside, "chain")); err == nil {
		t.Fatal("outside-HOME reset target accepted")
	}
	requireM802(t, os.Symlink(outside, filepath.Join(home, "escape")))
	if _, err := scriptops.ValidateLocalHome(home, filepath.Join(home, "escape", "chain")); err == nil {
		t.Fatal("symlink escape accepted")
	}
	for _, unsafe := range []string{home, filepath.Dir(home)} {
		if _, err := scriptops.ValidateLocalHome(home, unsafe); err == nil {
			t.Fatalf("unsafe initializer target accepted: %s", unsafe)
		}
	}
	source := string(readM802(t, "evm_stress.sh"))
	expected := `PAXD_BIN="$PAXD_BIN" PAX_HOME="$PAX_HOME" PAX_LOCAL_RESET=1 NO_RUN=1 ./scripts/initialize_local_chain.sh`
	if !strings.Contains(source, expected) || !strings.Contains(source, `echo "==> disposable stress home: $PAX_HOME"`) {
		t.Fatal("stress entrypoint must disclose its selected home and explicitly acknowledge only its initializer")
	}
	initializer := string(readM802(t, "initialize_local_chain.sh"))
	if !strings.Contains(initializer, `if [ "${PAX_LOCAL_RESET:-0}" != "1" ]; then`) || strings.Index(initializer, "Refusing to erase") > strings.Index(initializer, `rm -rf -- "$PAX_HOME"`) {
		t.Fatal("initializer operator-state protection was removed or reordered")
	}
}

func TestRepairM_802_Finding12_OneBinaryAndHome(t *testing.T) {
	home := t.TempDir()
	binary := filepath.Join(home, "custom ' bin", "paxd")
	target := filepath.Join(home, "custom ' chain")
	cfg, err := scriptops.NewStressConfig(home, binary, target)
	requireM802(t, err)
	if cfg.Binary != binary || cfg.Home != target {
		t.Fatalf("overrides drifted: %+v", cfg)
	}
	accepted, err := scriptops.ValidateLocalHome(home, cfg.Home)
	requireM802(t, err)
	writeM802(t, filepath.Join(accepted, "config", "genesis.json"), []byte(`{"app_state":{"bank":{"balances":[]}}}`))
	// The same config drives the initializer target, genesis file and startup home.
	var genesis map[string]json.RawMessage
	requireM802(t, json.Unmarshal(readM802(t, filepath.Join(cfg.Home, "config", "genesis.json")), &genesis))
	args := cfg.StartArgs()
	if !reflect.DeepEqual(args, []string{"start", "--trace", "--chain-id", "pax-chain", "--home", accepted}) {
		t.Fatalf("startup home drift: %q", args)
	}
	env := cfg.ShellEnvironment()
	if !strings.Contains(env, `export PAXD_BIN='`+strings.ReplaceAll(binary, "'", "'\"'\"'")+"'\n") || !strings.Contains(env, "PAX_START_ARGS=(") {
		t.Fatalf("unsafe or missing shell transport: %q", env)
	}
	source := string(readM802(t, "evm_stress.sh"))
	for _, required := range []string{`STRESS_ENV=$(bash "$REPO_ROOT/scripts/maintenance.sh" stress-env)`, `readonly PAXD_BIN PAX_HOME`, `"$PAX_HOME/config/genesis.json"`, `"$PAXD_BIN" "${PAX_START_ARGS[@]}"`, `kill "$PAXD_PID"`, `wait "$PAXD_PID"`} {
		if !strings.Contains(source, required) {
			t.Fatalf("stress consumer lost canonical wiring %q", required)
		}
	}
	if strings.Contains(source, `"$HOME/.pax/config/genesis.json"`) || strings.Contains(source, `PAXD="$HOME/go/bin/paxd"`) {
		t.Fatal("legacy independent home/binary remains")
	}
}

func TestRepairM_802_Finding13_NativeConsensusKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := tmconfig.DefaultConfig()
	original.Moniker = "preserve me"
	requireM802(t, original.WriteToTemplate(path))
	before := readM802(t, path)
	requireM802(t, scriptops.ConfigureConsensus(path))
	data := readM802(t, path)
	v := viper.New()
	v.SetConfigType("toml")
	requireM802(t, v.ReadConfig(bytes.NewReader(data)))
	recovered := tmconfig.DefaultConfig()
	requireM802(t, v.Unmarshal(recovered))
	if recovered.Consensus.UnsafeVoteTimeoutOverride != 2*time.Second || recovered.Consensus.UnsafeCommitTimeoutOverride != 2*time.Second || recovered.Consensus.UnsafeBypassCommitTimeoutOverride == nil || *recovered.Consensus.UnsafeBypassCommitTimeoutOverride {
		t.Fatalf("native consensus decoder ignored produced settings: %+v", recovered.Consensus)
	}
	if recovered.Moniker != original.Moniker || recovered.Consensus.UnsafeProposeTimeoutOverride != original.Consensus.UnsafeProposeTimeoutOverride {
		t.Fatal("unrelated consensus/config values changed")
	}
	requireM802(t, recovered.ValidateBasic())
	first := append([]byte{}, data...)
	requireM802(t, scriptops.ConfigureConsensus(path))
	if !bytes.Equal(first, readM802(t, path)) {
		t.Fatal("consensus edit is not idempotent")
	}
	for _, key := range []string{"unsafe-vote-timeout-override", "unsafe-commit-timeout-override", "unsafe-bypass-commit-timeout-override"} {
		t.Run("missing-"+key, func(t *testing.T) {
			lines := strings.Split(string(before), "\n")
			for i, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#")), key+" =") {
					lines[i] = ""
				}
			}
			invalid := []byte(strings.Join(lines, "\n"))
			writeM802(t, path, invalid)
			if err := scriptops.ConfigureConsensus(path); err == nil {
				t.Fatalf("missing section key %s did not fail", key)
			}
			if !bytes.Equal(readM802(t, path), invalid) {
				t.Fatal("missing-key failure partially edited config")
			}
		})
	}
	source := string(readM802(t, "initialize_local_chain.sh"))
	if strings.Count(source, `bash "$SCRIPT_DIR/maintenance.sh" consensus-config "$CONFIG_PATH"`) != 2 || strings.Contains(source, "s/timeout_prevote") {
		t.Fatal("Linux and macOS must use the checked canonical section editor")
	}
}

func commitM802(t *testing.T, repo *git.Repository, message string) plumbing.Hash {
	t.Helper()
	wt, err := repo.Worktree()
	requireM802(t, err)
	file, err := wt.Filesystem.Create("history.txt")
	requireM802(t, err)
	_, err = io.WriteString(file, message)
	requireM802(t, err)
	requireM802(t, file.Close())
	_, err = wt.Add("history.txt")
	requireM802(t, err)
	hash, err := wt.Commit(message, &git.CommitOptions{Author: &object.Signature{Name: "Release fixture", Email: "fixture@example.invalid", When: time.Unix(1700000000, 0)}})
	requireM802(t, err)
	return hash
}

func TestRepairM_802_Finding14_CanonicalChangelogLinks(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	requireM802(t, err)
	base := commitM802(t, repo, "Before (#1)")
	_ = commitM802(t, repo, "First feature (#42)")
	_ = commitM802(t, repo, "An ordinary commit")
	head := commitM802(t, repo, "Second feature (#99)\n\nBody (#777)")
	result, err := scriptops.ChangelogBetween(repo, "Sidiora-Labs/paxeer-monorepo", base, head, "v7.0")
	requireM802(t, err)
	expected := "## v7.0\npax-chain\n* [#99](https://github.com/Sidiora-Labs/paxeer-monorepo/pull/99) Second feature\n* [#42](https://github.com/Sidiora-Labs/paxeer-monorepo/pull/42) First feature\n"
	if result != expected {
		t.Fatalf("canonical release links/range:\n%s", result)
	}
	for _, repository := range []string{"", "https://github.com/owner/repo", "owner/repo/extra", "-owner/repo", "owner/repo.git", "owner/../repo"} {
		if _, err := scriptops.GenerateChangelog(t.TempDir(), repository, "base", "head"); err == nil {
			t.Fatalf("unvalidated release repository accepted: %q", repository)
		}
	}
}

func TestRepairM_802_Finding15_ResolvedLocalAndFetchedRevisions(t *testing.T) {
	remotePath := "m802://fixture/upstream"
	remote, err := git.Init(memory.NewStorage(), memfs.New())
	requireM802(t, err)
	client.InstallProtocol("m802", server.NewClient(server.MapLoader{remotePath: remote.Storer}))
	t.Cleanup(func() { client.InstallProtocol("m802", nil) })
	base := commitM802(t, remote, "Base (#1)")
	requireM802(t, remote.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("release/base"), base)))
	head := commitM802(t, remote, "Remote feature (#88)")
	requireM802(t, remote.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("release/head"), head)))
	localPath := filepath.Join(t.TempDir(), "local")
	local, err := git.PlainInit(localPath, false)
	requireM802(t, err)
	_, err = local.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{remotePath}})
	requireM802(t, err)
	// Fetches are real repository-to-repository transport; no git executable or network.
	resolvedBase, err := scriptops.ResolveRelease(local, "release/base")
	requireM802(t, err)
	resolvedHead, err := scriptops.ResolveRelease(local, "release/head")
	requireM802(t, err)
	if resolvedBase != base || resolvedHead != head {
		t.Fatal("fetch did not resolve the exact published commits")
	}
	ref, err := local.Reference(plumbing.NewRemoteReferenceName("origin", "release/head"), true)
	requireM802(t, err)
	if ref.Hash() != head {
		t.Fatal("fetched revision was left only in FETCH_HEAD")
	}
	// A local-only branch is valid even when no matching remote-tracking name exists.
	requireM802(t, local.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("release/local-only"), head)))
	resolvedLocal, err := scriptops.ResolveRelease(local, "release/local-only")
	requireM802(t, err)
	if resolvedLocal != head {
		t.Fatal("local branch was replaced by a remote-tracking revision")
	}
	if _, err = local.Reference(plumbing.NewRemoteReferenceName("origin", "release/local-only"), true); !errors.Is(err, plumbing.ErrReferenceNotFound) {
		t.Fatalf("local-only fixture accidentally has a remote counterpart: %v", err)
	}
	result, err := scriptops.GenerateChangelog(localPath, "owner/project", "release/base", "release/local-only")
	requireM802(t, err)
	if !strings.Contains(result, "/owner/project/pull/88") || strings.Contains(result, "pull/1)") {
		t.Fatalf("log did not consume resolved revisions unchanged: %s", result)
	}
	// Resolve the raw commit and fully qualified ref through the same consumer.
	for _, requested := range []string{head.String(), "refs/heads/release/local-only"} {
		resolved, err := scriptops.ResolveRelease(local, requested)
		requireM802(t, err)
		if resolved != head {
			t.Fatalf("revision changed for %q", requested)
		}
	}
	for _, requested := range []string{"", "--all", "release/../bad", "refs/heads/absent"} {
		if _, err := scriptops.ResolveRelease(local, requested); err == nil {
			t.Fatalf("invalid or absent ref accepted: %q", requested)
		}
	}
}

func TestRepairM_802_Finding17_EntireGoDurationGrammar(t *testing.T) {
	cases := []string{"0", "+0", "-0", "1h2m3.004005006s", "+2.5s", "-.5h", "1.s", ".5s", "1us", "1µs", "1μs", "-9223372036854775808ns", "9223372036854775807ns", "1s2s", "0.0000000009s", "1.0000000000000000000000000000001s"}
	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			expected, err := time.ParseDuration(value)
			requireM802(t, err)
			actual, err := scriptops.DurationSeconds(value)
			requireM802(t, err)
			if actual != expected.Seconds() || math.IsNaN(actual) || math.IsInf(actual, 0) {
				t.Fatalf("duration %q: got %v want %v", value, actual, expected.Seconds())
			}
			var stdout, stderr bytes.Buffer
			if code := scriptops.RunMaintenance([]string{"duration", value}, strings.NewReader(""), &stdout, &stderr); code != 0 {
				t.Fatalf("duration transport failed: %d %s", code, &stderr)
			}
			var response struct {
				Valid   bool    `json:"valid"`
				Seconds float64 `json:"seconds"`
			}
			requireM802(t, json.Unmarshal(stdout.Bytes(), &response))
			if !response.Valid || response.Seconds != actual {
				t.Fatalf("duration protocol lost value: %s", &stdout)
			}
		})
	}
	for _, value := range []string{"", "garbage", "3sgarbage", "junk3s", "1s-2s", "1s+2s", "1S", "1e3s", "NaNs", "Infs", " 3s", "3s ", "3s\n", "3s\x00", "1h 2m", "9223372036854775808ns", strings.Repeat("9", 400) + "h"} {
		t.Run("invalid-"+value, func(t *testing.T) {
			if _, err := scriptops.DurationSeconds(value); err == nil {
				t.Fatalf("malformed or overflowing duration accepted: %q", value)
			}
			var stdout, stderr bytes.Buffer
			if code := scriptops.RunMaintenance([]string{"duration", value}, strings.NewReader(""), &stdout, &stderr); code != 0 {
				t.Fatalf("invalid configuration should have a typed response, got %d: %s", code, &stderr)
			}
			var response struct {
				Valid bool   `json:"valid"`
				Error string `json:"error"`
			}
			requireM802(t, json.Unmarshal(stdout.Bytes(), &response))
			if response.Valid || !strings.Contains(response.Error, "configuration failure") {
				t.Fatalf("malformed input was reported as a healthy duration: %s", &stdout)
			}
		})
	}
	source := string(readM802(t, "paxd_problems_detector.py"))
	for _, required := range []string{`["bash", str(helper), "duration", s]`, `if parsed["valid"] is False:`, `math.isfinite(seconds)`, `Configuration failure: invalid mempool.ttl-duration`} {
		if !strings.Contains(source, required) {
			t.Fatalf("detector does not consume the checked parser protocol: %s", required)
		}
	}
}

func TestRepairM_802_Finding18_StableSafelyEncodedManifest(t *testing.T) {
	root := filepath.Join(t.TempDir(), "input")
	requireM802(t, os.Mkdir(root, 0700))
	files := map[string]string{"z-last": "z", "a-first": "a", "sub/space name": "space", "sub/back\\slash": "backslash", "sub/new\nline": "newline", "sub/carriage\rreturn": "carriage", "-option": "option"}
	for name, value := range files {
		writeM802(t, filepath.Join(root, name), []byte(value))
	}
	output := filepath.Join(root, "checksum.list")
	writeM802(t, output, []byte("old mutable manifest"))
	digest, err := scriptops.PublishChecksums(root, output)
	requireM802(t, err)
	first := readM802(t, output)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var expected strings.Builder
	for _, name := range names {
		sum := sha256.Sum256([]byte(files[name]))
		escaped, prefix := "./"+name, ""
		if strings.ContainsAny(escaped, "\\\n\r") {
			prefix = "\\"
			escaped = strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\r", "\\r").Replace(escaped)
		}
		fmt.Fprintf(&expected, "%s%x  %s\n", prefix, sum, escaped)
	}
	if string(first) != expected.String() {
		t.Fatalf("manifest ordering, filenames or digests changed:\n%q\nwant:\n%q", first, expected.String())
	}
	if digest != fmt.Sprintf("%x", sha256.Sum256(first)) {
		t.Fatal("aggregate does not hash the published manifest")
	}
	secondDigest, err := scriptops.PublishChecksums(root, output)
	requireM802(t, err)
	if digest != secondDigest || !bytes.Equal(first, readM802(t, output)) {
		t.Fatal("unchanged input is not reproducible or manifest hashed itself")
	}
	if strings.Contains(string(first), "checksum.list") {
		t.Fatal("final manifest included in input inventory")
	}
	writeM802(t, filepath.Join(root, "a-first"), []byte("changed"))
	changed, err := scriptops.PublishChecksums(root, output)
	requireM802(t, err)
	if changed == digest {
		t.Fatal("input mutation did not change directory digest")
	}
	entries, err := os.ReadDir(filepath.Dir(root))
	requireM802(t, err)
	if len(entries) != 1 {
		t.Fatalf("staging files were not cleaned: %v", entries)
	}
}

func TestRepairM_802_Finding18_FailedHashingDoesNotPublish(t *testing.T) {
	parent := t.TempDir()
	output := filepath.Join(parent, "checksum.list")
	prior := []byte("previous manifest")
	writeM802(t, output, prior)
	if _, err := scriptops.PublishChecksums(filepath.Join(parent, "absent"), output); err == nil {
		t.Fatal("failed inventory reported success")
	}
	if !bytes.Equal(readM802(t, output), prior) {
		t.Fatal("failed inventory truncated published manifest")
	}
	root := filepath.Join(parent, "input")
	writeM802(t, filepath.Join(root, "file"), []byte("data"))
	// A destination directory is a real filesystem publication failure.
	directoryOutput := filepath.Join(parent, "directory-output")
	requireM802(t, os.Mkdir(directoryOutput, 0700))
	if _, err := scriptops.PublishChecksums(root, directoryOutput); err == nil {
		t.Fatal("failed manifest publication reported success")
	}
	if !bytes.Equal(readM802(t, output), prior) {
		t.Fatal("failed publication changed existing manifest")
	}
	entries, err := os.ReadDir(parent)
	requireM802(t, err)
	if len(entries) != 3 {
		t.Fatalf("failed publication leaked staging files: %v", entries)
	}
	// The empty tree has a stable, empty manifest and the standard empty SHA256.
	empty := filepath.Join(parent, "empty")
	requireM802(t, os.Mkdir(empty, 0700))
	emptyOutput := filepath.Join(empty, "checksum.list")
	digest, err := scriptops.PublishChecksums(empty, emptyOutput)
	requireM802(t, err)
	if digest != fmt.Sprintf("%x", sha256.Sum256(nil)) || len(readM802(t, emptyOutput)) != 0 {
		t.Fatal("empty-directory digest is not stable")
	}
}

func TestRepairM_802_EntrypointsUseProductionOperations(t *testing.T) {
	for _, tc := range []struct{ path, operation string }{
		{"dump_app_state_for_height.sh", `maintenance.sh" export "$PAXD_BIN" "$PAX_HOME" "$1"`},
		{"generate-changelog.sh", `maintenance.sh" changelog "${CHANGELOG_REPO:-}" "$1" "$2"`},
		{"run-sha256sum.sh", `maintenance.sh" checksums "$1"`},
	} {
		source := string(readM802(t, tc.path))
		if !strings.Contains(source, "set -euo pipefail") || !strings.Contains(source, tc.operation) {
			t.Fatalf("%s bypasses the tested production operation", tc.path)
		}
	}
	// Drive the actual command dispatcher against the filesystem, including cwd.
	cwd, err := os.Getwd()
	requireM802(t, err)
	root := t.TempDir()
	writeM802(t, filepath.Join(root, "input", "payload"), []byte("real dispatcher input"))
	t.Chdir(root)
	var stdout, stderr bytes.Buffer
	code := scriptops.RunMaintenance([]string{"checksums", "input"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dispatcher failed: %d %s", code, &stderr)
	}
	manifest := readM802(t, filepath.Join(root, "checksum.list"))
	sum := sha256.Sum256([]byte("real dispatcher input"))
	if string(manifest) != fmt.Sprintf("%x  ./payload\n", sum) {
		t.Fatalf("dispatcher ignored caller directory: %q", manifest)
	}
	requireM802(t, os.Chdir(cwd))
	// Bad command inputs fail visibly, without needing any external program.
	for _, args := range [][]string{nil, {"missing"}, {"duration"}, {"checksums"}, {"changelog"}, {"export"}, {"consensus-config"}, {"validate-local-home"}} {
		stdout.Reset()
		stderr.Reset()
		if code := scriptops.RunMaintenance(args, strings.NewReader(""), &stdout, &stderr); code == 0 || stderr.Len() == 0 {
			t.Fatalf("invalid CLI accepted: %q", args)
		}
	}
}
