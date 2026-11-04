package rollout

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
)

func repairM729Read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func repairM729Ordered(t *testing.T, source string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		at := strings.Index(source, part)
		if at < 0 {
			t.Fatalf("missing or out-of-order production statement %q", part)
		}
		source = source[at+len(part):]
	}
}

func repairM729Function(t *testing.T, source, name string) string {
	t.Helper()
	start := strings.Index(source, "\ndef "+name+"(")
	if start < 0 {
		t.Fatalf("production function %s is missing", name)
	}
	body := source[start+1:]
	if end := strings.Index(body, "\n\n\n"); end >= 0 {
		body = body[:end]
	}
	return body
}

func TestRepairM_729_LedgerConcurrentRestartRecovery(t *testing.T) {
	// Inspect the actual shell program, as required by the continuation ruling.
	// Opening in append mode and retaining descriptor 9 to process exit keeps
	// allocation and the entire record on the same locked inode across retries.
	script := repairM729Read(t, "ledger.sh")
	repairM729Ordered(t, script,
		"set -euo pipefail\n",
		`file="$dir/${started%%T*}.kvx"`,
		`mkdir -p "$dir"`,
		`exec 9>>"$file"`,
		"flock -x 9\n",
		`section="rollout.${sha:0:12}.$svc"`,
		`if [ -f "$file" ] && grep -qF "[$section]" "$file"; then`,
		`n=$(grep -cE "^\[$section(\.[0-9]+)?\]$" "$file")`,
		`section="$section.$((n + 1))"`,
		"fi\n{\n",
		`[ -s "$file" ] && echo`,
		`printf '[%s]\nrevision = "%s"\ntarget = "%s"\nstarted_at = "%s"\noutcome = "%s"\nevidence = "%s"\n'`,
		`"$section" "$sha" "$target" "$started" "$outcome" "$evidence"`,
		"} >&9\n")
	critical := script[strings.Index(script, "flock -x 9\n")+len("flock -x 9\n"):]
	if strings.Contains(critical, "flock ") || strings.Contains(critical, "exec ") ||
		!strings.HasSuffix(critical, "} >&9\n") {
		t.Fatal("ledger lock must remain on descriptor 9 through the complete append and process exit")
	}
	if strings.Count(script, `exec 9>>"$file"`) != 1 || strings.Count(script, "flock -x 9\n") != 1 ||
		strings.Count(script, "printf ") != 1 {
		t.Fatal("ledger must have one append descriptor, one exclusive lock and one complete record writer")
	}

	// Exercise the real file-lock primitive used by flock -x, with independently
	// opened append descriptors. No deployment script or external binary runs.
	path := filepath.Join(t.TempDir(), "ledger.kvx")
	open := func() *os.File {
		t.Helper()
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		return f
	}
	owner := open()
	if err := syscall.Flock(int(owner.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	// Seed a complete prior-attempt record, then ensure contending append opens
	// and a new owner retain every byte of that persisted evidence.
	retained := []byte(`[rollout.0123456789ab.router]
revision = "0123456789abcdef0123456789abcdef01234567"
target = "railway"
started_at = "2026-10-07T10:00:00Z"
outcome = "blocked"
evidence = "/root/lx-ops/attempt-0.log"
`)
	if _, err := owner.Write(retained); err != nil {
		t.Fatal(err)
	}
	if err := owner.Sync(); err != nil {
		t.Fatal(err)
	}
	const count = 24
	writers := make([]*os.File, count)
	for i := range writers {
		writers[i] = open()
	}
	var wg sync.WaitGroup
	results := make(chan error, count)
	for _, writer := range writers {
		wg.Add(1)
		go func(f *os.File) {
			defer wg.Done()
			results <- syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		}(writer)
	}
	wg.Wait()
	close(results)
	for err := range results {
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			t.Fatalf("simultaneous ledger opener bypassed the held exclusive lock: %v", err)
		}
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	// Closing the prior owner, just as shell exit does, must release the lock.
	// Retry owners reopen the same inode rather than replacing or truncating it.
	for _, writer := range writers {
		if err := syscall.Flock(int(writer.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			t.Fatalf("ledger lock did not recover after the owner closed: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
	}
	reopened := open()
	if err := syscall.Flock(int(reopened.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("a restarted ledger owner could not acquire the file lock: %v", err)
	}
	recovered, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(recovered, retained) {
		t.Fatalf("append opens or ownership recovery changed persisted evidence: %q", recovered)
	}
}

func TestRepairM_729_SleepingBlocksRolloutAndPerpsConsumer(t *testing.T) {
	rollout := repairM729Read(t, "rollout.py")
	perps := repairM729Read(t, "../deploy/railway-perps-project.py")
	terminalLiteral := regexp.MustCompile(`(?m)^TERMINAL = (\{[^}]+\})`).FindStringSubmatch(rollout)
	if len(terminalLiteral) != 2 {
		t.Fatal("missing production terminal status table")
	}
	var terminal map[string]string
	if err := json.Unmarshal([]byte(terminalLiteral[1]), &terminal); err != nil {
		t.Fatal(err)
	}
	wantTerminal := map[string]string{
		"SUCCESS": "pass", "SLEEPING": "blocked", "FAILED": "fail", "CRASHED": "fail",
		"REMOVED": "fail", "REMOVING": "fail", "SKIPPED": "fail", "NEEDS_APPROVAL": "blocked",
	}
	if !reflect.DeepEqual(terminal, wantTerminal) {
		t.Fatalf("unsafe deployment status policy: got %v, want %v", terminal, wantTerminal)
	}

	// Check the production producer, wire status extraction, and consumers. This
	// verifies that the parsed policy is the one both actual waiters consume.
	repairM729Ordered(t, repairM729Function(t, rollout, "gql"),
		"urllib.request.Request(", "json.dumps({\"query\": query, \"variables\": variables})",
		"urllib.request.urlopen(req, timeout=30)", "body = json.load(resp)", `return body["data"]`)
	repairM729Ordered(t, repairM729Function(t, rollout, "wait_deployment"),
		`status = gql("query Deployment(`, `{"id": deployment})["deployment"]["status"]`,
		"if status not in IN_PROGRESS:", "if status in TERMINAL:", "return TERMINAL[status]",
		`return "fail"`)
	repairM729Ordered(t, repairM729Function(t, rollout, "apply_railway_image"),
		`if not change and runs(latest, ref, svc["health"]):`,
		`return wait_deployment(latest["id"], log)`,
		`deployment = gql("mutation Deploy(`, "return wait_deployment(deployment, log)")
	repairM729Ordered(t, repairM729Function(t, rollout, "apply_railway"),
		`deployment = gql("mutation DeployCommit(`, "return wait_deployment(deployment, log)")
	repairM729Ordered(t, repairM729Function(t, rollout, "main"),
		"for svc in work:", "outcome = apply_railway(", "outcome = apply_railway_image(",
		`subprocess.run([str(HERE / "ledger.sh"), sha, svc["name"], target, started,`,
		"outcome, str(path)], check=True)",
		`print(f"{outcome} {svc['name']} evidence={path}")`, "if outcome != \"pass\":\n            return 1")
	repairM729Ordered(t, perps,
		"from rollout import API, GHCR, IN_PROGRESS, TERMINAL, Malformed, load_digests",
		`SUCCESS = {s for s, v in TERMINAL.items() if v == "pass"}`)
	if strings.Count(perps, "SUCCESS =") != 1 || strings.Count(perps, "TERMINAL =") != 0 {
		t.Fatal("perps must derive its success policy solely from rollout's imported terminal table")
	}
	repairM729Ordered(t, repairM729Function(t, perps, "http_transport"),
		"urllib.request.Request(", "json.dumps({\"query\": query, \"variables\": variables})",
		"urllib.request.urlopen(req, timeout=30)", "body = json.load(resp)", `return body["data"]`)
	repairM729Ordered(t, repairM729Function(t, perps, "wait_deployment"),
		`status = gql("query DeploymentStatus(`, `{"id": deployment})["deployment"]["status"]`,
		"if status not in IN_PROGRESS:", "return status if status in TERMINAL else")
	repairM729Ordered(t, repairM729Function(t, perps, "main"),
		"status = wait_deployment(gql, deployment)", "if status not in SUCCESS:",
		"later waves were not applied", "return 1")

	// Decode the committed wire transcripts in Go, without an external runtime,
	// and check the repaired consumer outcomes and unchanged positive controls.
	var replay struct {
		Scenarios map[string]struct {
			Exit  int
			Out   []string
			Calls []struct {
				Op   string
				Data json.RawMessage
			}
		}
	}
	if err := json.Unmarshal([]byte(repairM729Read(t, "tests/replay.json")), &replay); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		name, status, outcome string
		exit                  int
	}{
		{"rerun-sleeping-is-unchanged", "SLEEPING", "blocked", 1},
		{"rerun-healthy-is-unchanged", "SUCCESS", "pass", 0},
		{"rerun-waits-for-running-deployment", "SUCCESS", "pass", 0},
		{"first-apply", "SUCCESS", "pass", 0},
		{"needs-approval-blocks", "NEEDS_APPROVAL", "blocked", 1},
	} {
		t.Run(check.name, func(t *testing.T) {
			case_, ok := replay.Scenarios[check.name]
			if !ok || case_.Exit != check.exit || len(case_.Calls) == 0 || len(case_.Out) == 0 {
				t.Fatalf("missing or incorrect replay case %s: %+v", check.name, case_)
			}
			last := case_.Calls[len(case_.Calls)-1]
			var wire struct {
				Deployment struct{ Status string }
			}
			if err := json.Unmarshal(last.Data, &wire); err != nil {
				t.Fatal(err)
			}
			if last.Op != "Deployment" || wire.Deployment.Status != check.status || terminal[wire.Deployment.Status] != check.outcome {
				t.Fatalf("recorded deployment must resolve to %s: %s", check.outcome, last.Data)
			}
			for _, out := range case_.Out {
				if !strings.HasPrefix(out, check.outcome+" ") {
					t.Fatalf("recorded output contradicts terminal policy: %q", out)
				}
			}
			if check.status == "SLEEPING" {
				if len(case_.Out) != 1 || case_.Out[0] != "blocked gateway" {
					t.Fatal("a sleeping gateway must block before another wave is reported")
				}
				for _, call := range case_.Calls {
					if strings.HasPrefix(call.Op, "Deploy") && call.Op != "Deployment" {
						t.Fatal("unchanged sleeping deployment should be blocked, not silently redeployed")
					}
				}
				if !bytes.Contains(case_.Calls[len(case_.Calls)-2].Data, []byte(`"status": "SLEEPING"`)) {
					t.Fatal("sleeping fixture must cover the unchanged instance as well as the deployment response")
				}
			}
		})
	}
}
