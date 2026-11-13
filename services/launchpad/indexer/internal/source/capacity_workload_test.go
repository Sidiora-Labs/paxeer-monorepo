package source

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func capacityOutputCommand(t *testing.T, output string) CapacityCommand {
	t.Helper()
	executable, err := exec.LookPath("printf")
	if err != nil {
		t.Fatal(err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	return CapacityCommand{Executable: executable, SHA256: capacityFileDigest(t, executable), Args: []string{"%s\\n", output}}
}

func capacityObservedOutputCommand(t *testing.T, output string) CapacityCommand {
	t.Helper()
	executable, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	const script = "import sys,json,datetime\nfor line in sys.argv[1].splitlines():\n record=json.loads(line)\n fault=record.get('Fault')\n if fault and fault['ObservedAt']=='0001-01-01T00:00:00Z':\n  fault['ObservedAt']=datetime.datetime.now(datetime.timezone.utc).isoformat()\n print(json.dumps(record),flush=True)\n"
	return CapacityCommand{Executable: executable, SHA256: capacityFileDigest(t, executable), Args: []string{"-c", script, output}}
}

func TestCapacityWorkloadRetainsProviderOutputAndFaultCoordinates(t *testing.T) {
	ctx := context.Background()
	fault := CapacityFaultObservation{Kind: "throttle", SourceID: "primary", State: "injected", ObservedAt: time.Now().UTC()}
	raw, err := json.Marshal(capacityWorkloadRecord{Kind: "fault", Fault: &fault})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, output string
		valid        bool
	}{
		{"event", `{"Kind":"event","Value":{"height":101}}`, true},
		{"fault", string(raw), true},
		{"malformed", `{`, false},
		{"missing-kind", `{}`, false},
		{"missing-fault", `{"Kind":"fault"}`, false},
		{"invalid-source", `{"Kind":"fault","Fault":{"SourceID":"bad source"}}`, false},
		{"empty", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workload.jsonl")
			capture, err := newCapacityCapture(path, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			faults := make(chan CapacityFaultObservation, 1)
			err = capacityWorkload(ctx, capacityOutputCommand(t, tc.output), capture, faults)
			if tc.valid && err != nil || !tc.valid && err == nil {
				t.Fatalf("provider output accepted=%v err=%v", tc.valid, err)
			}
			digest, count, finishErr := capture.finish()
			if finishErr != nil || len(digest) != 64 {
				t.Fatalf("capture finish %v", finishErr)
			}
			if tc.valid && count == 0 {
				t.Fatal("provider output absent from capture")
			}
			if tc.name == "fault" {
				select {
				case got := <-faults:
					if got != fault {
						t.Fatalf("fault coordinates %+v", got)
					}
				default:
					t.Fatal("fault observation absent")
				}
			}
		})
	}
	command := capacityOutputCommand(t, `{"Kind":"event"}`)
	for _, change := range []func(*CapacityCommand){
		func(c *CapacityCommand) { c.Executable = "relative" },
		func(c *CapacityCommand) { c.SHA256 = strings.Repeat("0", 64) },
		func(c *CapacityCommand) { c.Args = []string{string([]byte{0})} },
		func(c *CapacityCommand) { c.Args = []string{strings.Repeat("x", 32769)} },
		func(c *CapacityCommand) { c.Environment = make([]string, 257) },
	} {
		changed := command
		change(&changed)
		if err := capacityWorkload(ctx, changed, nil, nil); err == nil {
			t.Fatal("invalid command binding accepted")
		}
	}
	capture, err := newCapacityCapture(filepath.Join(t.TempDir(), "bounded.jsonl"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = capacityWorkload(ctx, command, capture, nil); err == nil {
		t.Fatal("provider exceeded capture bound")
	}
	if _, _, err = capture.finish(); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err = capacityWorkload(cancelled, command, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled provider %v", err)
	}
}

func TestCapacityInputIdentityAndBindingRefusals(t *testing.T) {
	ctx := context.Background()
	node := repair997Serve(t)
	live := repair997Open(t, fopts(node.server.URL))
	opts := capacityBoundOptions(t, live)
	providers := CapacityProviders{Live: live, Historical: &HistoricalSource{RPC: live}}
	for _, change := range []func(*CapacityOptions){
		func(o *CapacityOptions) { o.Binding.Revision = strings.Repeat("f", 40) },
		func(o *CapacityOptions) { o.Binding.Files = nil },
		func(o *CapacityOptions) {
			o.Binding.Files = append(append([]CapacityBoundFile(nil), o.Binding.Files...), o.Binding.Files[0])
		},
		func(o *CapacityOptions) {
			o.Binding.Files = append([]CapacityBoundFile(nil), o.Binding.Files...)
			o.Binding.Files[0].Path = "../outside.go"
		},
		func(o *CapacityOptions) { o.Binding.SourceSHA256 = strings.Repeat("0", 64) },
		func(o *CapacityOptions) { o.ApprovalDocument.Path = "relative" },
		func(o *CapacityOptions) { o.InterfaceDocument.Path = filepath.Join(t.TempDir(), "missing") },
		func(o *CapacityOptions) { o.Binding.ApprovalSHA256 = strings.Repeat("0", 64) },
		func(o *CapacityOptions) { o.Binding.ManifestDigest = "other" },
	} {
		changed := opts
		change(&changed)
		if err := verifyCapacityBinding(ctx, changed, providers); err == nil {
			t.Fatal("changed capacity identity accepted")
		}
	}
	for _, v := range []string{"", strings.Repeat("a", 129), "bad stream", "line\n"} {
		if capacityStreamID(v) {
			t.Fatalf("invalid stream %q", v)
		}
	}
	if !capacityStreamID("chain125.canonical:v1-prod_1") {
		t.Fatal("valid stream identity rejected")
	}
	if err := os.WriteFile(opts.ApprovalDocument.Path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCapacityBinding(ctx, opts, providers); err == nil {
		t.Fatal("changed approval bytes accepted")
	}
}

func TestCapacityWorkloadDrainsCompleteOutputBeforeWaiting(t *testing.T) {
	const events = 64
	output := strings.Repeat(`{"Kind":"event","Value":{"height":101}}`+"\n", events)
	command := capacityOutputCommand(t, strings.TrimSuffix(output, "\n"))
	for i := 0; i < 32; i++ {
		path := filepath.Join(t.TempDir(), "workload.jsonl")
		capture, err := newCapacityCapture(path, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		if err = capacityWorkload(context.Background(), command, capture, nil); err != nil {
			t.Fatal(err)
		}
		_, size, err := capture.finish()
		if err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(path)
		if err != nil || uint64(len(body)) != size {
			t.Fatalf("capture size %d/%d %v", len(body), size, err)
		}
		lines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
		if len(lines) != events {
			t.Fatalf("workload records %d", len(lines))
		}
		for _, line := range lines {
			var record struct {
				Kind  string
				Value capacityWorkloadRecord
			}
			if err = json.Unmarshal(line, &record); err != nil || record.Kind != "workload_provider" || record.Value.Kind != "event" || string(record.Value.Value) != `{"height":101}` {
				t.Fatalf("captured output %s: %v", line, err)
			}
		}
	}
}
