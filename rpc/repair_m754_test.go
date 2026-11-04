package evmrpc_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

// Repair record: 22.30.172/geth-reference-pin is proved by
// TestRepairM_754_ExactReference, TestRepairM_754_IsolatedInstall and
// TestRepairM_754_RejectUnverifiedReference. No out-of-path wiring is needed.
// The ordered gate forbids a baseline verify before implementation; none ran.
// These command transcripts are peers of the production selector, not a
// replacement selector. Its actual JavaScript runs in-process, and its report
// crosses the real filesystem into the JSON consumer below.
const repairM754ReleaseTranscript = "Geth\nVersion: 1.17.0-stable\nArchitecture: amd64\nGo Version: go1.24.0\nOperating System: linux\n"

const repairM754Isolated = "<isolated>"

type repairM754Command struct {
	binary string
	args   []string
	env    map[string]string
	status int
	stdout string
	stderr string
}

type repairM754Report struct {
	Requested     string `json:"requested"`
	Version       string `json:"version"`
	Binary        string `json:"binary"`
	Source        string `json:"source"`
	VersionOutput string `json:"version_output"`
}

func repairM754Script(t *testing.T) string {
	t.Helper()
	contents, err := os.ReadFile("../tests/integration/rpc_tests/scripts/run-ci.sh")
	require.NoError(t, err)
	return string(contents)
}

func repairM754Probe(binary, output string) repairM754Command {
	return repairM754Command{binary: binary, args: []string{"version"}, env: map[string]string{}, stdout: output}
}

func repairM754Install() repairM754Command {
	return repairM754Command{
		binary: "go", args: []string{"install", "github.com/ethereum/go-ethereum/cmd/geth@v1.17.0"},
		env: map[string]string{"GOBIN": repairM754Isolated, "GOFLAGS": "-mod=mod", "GOWORK": "off"},
	}
}

func repairM754Select(t *testing.T, requested, installed string, transcript []repairM754Command, reportIsDirectory bool) (string, *repairM754Report, error) {
	t.Helper()
	script := repairM754Script(t)
	_, source, found := strings.Cut(script, "<<'GETH_REFERENCE_JS'\n")
	require.True(t, found, "the production selector must be present")
	source, _, found = strings.Cut(source, "\nGETH_REFERENCE_JS\n")
	require.True(t, found, "the production selector must have a complete heredoc")
	vm := goja.New()
	_, err := vm.RunString(source)
	require.NoError(t, err)
	selectReference, ok := goja.AssertFunction(vm.Get("ensureGethReference"))
	require.True(t, ok)

	root := t.TempDir()
	isolatedDir := filepath.Join(root, "isolated install")
	require.NoError(t, os.Mkdir(isolatedDir, 0700))
	reportFile := filepath.Join(root, "geth-reference.json")
	if reportIsDirectory {
		require.NoError(t, os.Mkdir(reportFile, 0700))
	}
	consumed := 0
	run := func(binary string, args []string, env map[string]string) map[string]interface{} {
		require.Less(t, consumed, len(transcript), "unexpected command %s %v", binary, args)
		entry := transcript[consumed]
		consumed++
		require.Equal(t, strings.ReplaceAll(entry.binary, repairM754Isolated, isolatedDir), binary)
		require.Equal(t, entry.args, args)
		expectedEnv := make(map[string]string, len(entry.env))
		for name, value := range entry.env {
			expectedEnv[name] = strings.ReplaceAll(value, repairM754Isolated, isolatedDir)
		}
		require.Equal(t, expectedEnv, env, "installation must override inherited Go configuration")
		return map[string]interface{}{"status": entry.status, "stdout": entry.stdout, "stderr": entry.stderr}
	}
	writeFile := func(path, contents string) {
		require.Equal(t, reportFile, path)
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			panic(vm.NewGoError(err))
		}
	}
	selected, selectionErr := selectReference(goja.Undefined(), vm.ToValue(requested), vm.ToValue(installed),
		vm.ToValue(isolatedDir), vm.ToValue(reportFile), vm.ToValue(run), vm.ToValue(writeFile))
	require.Equal(t, len(transcript), consumed, "every recorded command must execute")
	if selectionErr != nil {
		if !reportIsDirectory {
			_, err = os.Stat(reportFile)
			require.True(t, os.IsNotExist(err), "a failed selection must not publish a reference report: %v", err)
		}
		return "", nil, selectionErr
	}

	contents, err := os.ReadFile(reportFile)
	require.NoError(t, err)
	var report repairM754Report
	require.NoError(t, json.Unmarshal(contents, &report))
	require.Equal(t, requested, report.Requested)
	require.Equal(t, selected.String(), report.Binary, "launch and report must identify the same executable")
	return selected.String(), &report, nil
}

func TestRepairM_754_ExactReference(t *testing.T) {
	for _, output := range []string{repairM754ReleaseTranscript, "Geth\nVersion: 1.17.0\n"} {
		t.Run(strings.Split(output, "\n")[1], func(t *testing.T) {
			binary := "/opt/geth release/geth"
			selected, report, err := repairM754Select(t, "v1.17.0", binary,
				[]repairM754Command{repairM754Probe(binary, output)}, false)
			require.NoError(t, err)
			require.Equal(t, binary, selected)
			require.Equal(t, "installed", report.Source)
			require.Equal(t, strings.TrimPrefix(strings.Split(output, "\n")[1], "Version: "), report.Version)
			require.Equal(t, output, report.VersionOutput)
		})
	}
}

func TestRepairM_754_IsolatedInstall(t *testing.T) {
	for _, observed := range []string{
		"missing", "1.16.8-stable", "1.17.00", "1.17.0-unstable", "1.17.0-stable-extra", "1.17.0-rc.1", "1.17.1-stable",
		"unknown", "duplicate", "probe-failed",
	} {
		t.Run(observed, func(t *testing.T) {
			installed := "/usr/local/bin/geth"
			var transcript []repairM754Command
			if observed == "missing" {
				installed = ""
			} else {
				probe := repairM754Probe(installed, "Geth\nVersion: "+observed+"\n")
				switch observed {
				case "unknown":
					probe.stdout = "Geth\nArchitecture: amd64\n"
				case "duplicate":
					probe.stdout = repairM754ReleaseTranscript + "Version: 1.16.8-stable\n"
				case "probe-failed":
					probe.status = 1
					probe.stdout = repairM754ReleaseTranscript
				}
				transcript = append(transcript, probe)
			}
			transcript = append(transcript, repairM754Install(),
				repairM754Probe(repairM754Isolated+"/geth", repairM754ReleaseTranscript))
			selected, report, err := repairM754Select(t, "v1.17.0", installed, transcript, false)
			require.NoError(t, err)
			require.True(t, filepath.IsAbs(selected))
			require.NotEqual(t, installed, selected)
			require.Equal(t, "isolated-install", report.Source)
			require.Equal(t, "1.17.0-stable", report.Version)
			require.Equal(t, repairM754ReleaseTranscript, report.VersionOutput)
		})
	}
}

func TestRepairM_754_RejectUnverifiedReference(t *testing.T) {
	t.Run("install-failure", func(t *testing.T) {
		install := repairM754Install()
		install.status, install.stderr = 1, "module download failed"
		selected, _, err := repairM754Select(t, "v1.17.0", "", []repairM754Command{install}, false)
		require.ErrorContains(t, err, "pinned geth installation failed: module download failed")
		require.Empty(t, selected)
	})
	for _, probe := range []repairM754Command{
		repairM754Probe(repairM754Isolated+"/geth", "Geth\nVersion: 1.17.00\n"),
		{binary: repairM754Isolated + "/geth", args: []string{"version"}, env: map[string]string{}, status: 127},
	} {
		t.Run("post-install-validation", func(t *testing.T) {
			selected, _, err := repairM754Select(t, "v1.17.0", "", []repairM754Command{repairM754Install(), probe}, false)
			require.ErrorContains(t, err, "installed geth does not match v1.17.0")
			require.Empty(t, selected)
		})
	}
	for _, pin := range []string{"", "latest", "v1.17", "v1.17.0-extra", "1.17.0", "v1.17.0\n"} {
		t.Run("invalid-pin-"+pin, func(t *testing.T) {
			selected, _, err := repairM754Select(t, pin, "/usr/bin/geth", nil, false)
			require.ErrorContains(t, err, "GETH_VERSION must be an exact release tag")
			require.Empty(t, selected)
		})
	}
	t.Run("report-write-failure", func(t *testing.T) {
		selected, _, err := repairM754Select(t, "v1.17.0", "/usr/bin/geth",
			[]repairM754Command{repairM754Probe("/usr/bin/geth", repairM754ReleaseTranscript)}, true)
		require.Error(t, err)
		require.Empty(t, selected, "comparison cannot start without its reference evidence")
	})
}

func TestRepairM_754_LaunchReportedBinary(t *testing.T) {
	script := repairM754Script(t)
	require.Contains(t, script, "GETH_BINARY=\"$(node - \"$GETH_VERSION\" \"$(command -v geth || true)\"")
	require.Contains(t, script, `"$GETH_INSTALL_DIR" "$RPC_DIR/reports/geth-reference.json"`)
	require.Contains(t, script, `)" || die "cannot select the pinned geth reference"`)
	selection := strings.Index(script, "\nensure_geth\n")
	launch := strings.Index(script, `"$GETH_BINARY" --dev --http --http.addr 127.0.0.1 --http.port "$GETH_PORT"`)
	require.Positive(t, selection)
	require.Greater(t, launch, selection, "only a successfully validated binary may serve the reference endpoint")
	require.Contains(t, script[launch:], "--http.api eth,net,web3,debug,txpool --dev.period 0 > \"$GETH_LOG\" 2>&1 &\nGETH_PID=$!")
	require.NotContains(t, script, "$REPORT_DIR/geth-reference.json", "reference metadata must stay outside mochawesome report globs")
	require.NotContains(t, script, "npm run --silent rpc:geth", "npm must not resolve geth again through another PATH")
	require.Contains(t, script, `[ -z "$GETH_INSTALL_DIR" ] || rm -rf -- "$GETH_INSTALL_DIR"`)
}
