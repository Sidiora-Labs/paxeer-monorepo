package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Run the actual qualification producer and workflow consumer, including their
// shared canonicalization and logical identity functions.
func repair314Python(mode string, input []byte) ([]byte, error) {
	path, err := filepath.Abs("../../qualification/paxeer_x_release.py")
	if err != nil {
		return nil, err
	}
	script := `import importlib.util,json,os,sys
sys.dont_write_bytecode=True
spec=importlib.util.spec_from_file_location('release',sys.argv[1])
m=importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
v=json.load(sys.stdin)
mode=sys.argv[2]
if mode=='produce':
 v['logical_id']=m.request_id(v['manifest'])
elif mode=='workflow':
 os.environ['PAXEER_X_REQUEST']=v['inputs']['request']
 os.environ['PAXEER_X_REQUEST_ID']=v['inputs']['request_id']
 import base64
 r=json.loads(base64.b64decode(v['inputs']['request']))
 os.environ['GITHUB_SHA']=r['manifest']['revision']
 os.environ['GITHUB_RUN_ATTEMPT']='1'
 v=m.workflow_request()
sys.stdout.buffer.write(m.canonical(v))`
	cmd := exec.Command("python3", "-B", "-c", script, path, mode)
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("qualification Python %s: %w: %s", mode, err, &stderr)
	}
	return out, nil
}

func TestRepair_314_CanonicalNumbersAndErrors(t *testing.T) {
	value := map[string]any{
		"numbers": []int64{math.MinInt64, math.MinInt64 + 1, -9007199254740993, 9007199254740992, 9007199254740993, math.MaxInt64 - 1, math.MaxInt64},
		"strings": "<>&\x7fé\u2028😀\b\f\n\r\t",
		"nested":  map[string]any{"z": true, "a": nil},
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	want, err := repair314Python("canonical", raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := qCanonical(value)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("canonical bytes differ from Python:\n%s\n%s", got, want)
	}
	var decoded struct {
		Numbers []int64 `json:"numbers"`
	}
	if err = json.Unmarshal(got, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Numbers, value["numbers"]) {
		t.Fatal("integer roundtrip changed values")
	}
	cycle := map[string]any{}
	cycle["cycle"] = cycle
	for _, invalid := range []any{math.NaN(), math.Inf(1), make(chan int), cycle, json.RawMessage("{"), time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)} {
		if b, err := qCanonical(invalid); err == nil || b != nil {
			t.Fatalf("serialization error suppressed for %T: %s %v", invalid, b, err)
		}
	}
	completed := qTestComplete(t)
	_, signer := qTestKeys(t)
	completed.ObservedAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err = qSign(&completed, signer); err == nil || completed.Signature != nil {
		t.Fatal("signing suppressed serialization failure")
	}
	if err = qValidateCompleted(completed, signer.Public().(ed25519.PublicKey), completed.Domain, time.Now().Add(time.Minute)); err == nil {
		t.Fatal("verification suppressed serialization failure")
	}
}

func TestRepair_314_SubmitReopenRouteAndVerify(t *testing.T) {
	values := []int64{9007199254740991, 9007199254740992, 9007199254740993, 9007199254740994, -9007199254740992, -9007199254740993, math.MinInt64, math.MinInt64 + 1, math.MaxInt64 - 1, math.MaxInt64}
	identities := map[string]int64{}
	for i, value := range values {
		t.Run(strconv.FormatInt(value, 10), func(t *testing.T) {
			registry, req := qTestRegistry(t)
			key, signer := qTestKeys(t)
			if err := qWrite(filepath.Join(registry.root, "artifact-key.der"), x509.MarshalPKCS1PrivateKey(key)); err != nil {
				t.Fatal(err)
			}
			if err := qWrite(filepath.Join(registry.root, "signing-key"), signer); err != nil {
				t.Fatal(err)
			}
			req.Manifest.Cells[0].Thresholds["accepted"] = qualificationThreshold{Operator: "eq", Value: value, Unit: "count"}
			// Non-ASCII and HTML characters must hash identically on both sides too.
			req.Manifest.Cells[0].Matrix["description"] = "exact <integer> é 😀"
			producer, err := repair314Python("produce", qTestCanonical(t, req))
			if err != nil {
				t.Fatal(err)
			}
			if err = qStrict(producer, &req); err != nil {
				t.Fatal(err)
			}
			if previous, exists := identities[req.LogicalID]; exists {
				t.Fatalf("logical identity collision: %d and %d", previous, value)
			}
			identities[req.LogicalID] = value
			control, err := json.Marshal(qualificationControl{Operation: "submit", Request: &req})
			if err != nil {
				t.Fatal(err)
			}
			submitted := qHandle(registry, control)
			if submitted.Error != "" || submitted.Record == nil {
				t.Fatalf("submit: %+v", submitted)
			}
			if submitted.Record.Request.Manifest.Cells[0].Thresholds["accepted"].Value != value {
				t.Fatal("submit changed threshold")
			}
			root := registry.root
			registry = nil
			reopened, err := newQualificationRegistry(root)
			if err != nil {
				t.Fatal(err)
			}
			status := qHandle(reopened, qTestCanonical(t, qualificationControl{Operation: "status", LogicalID: req.LogicalID}))
			if status.Error != "" || status.Record == nil {
				t.Fatalf("reopened status: %+v", status)
			}
			if !reflect.DeepEqual(status.Record.Request, req) {
				t.Fatal("durable request changed")
			}

			type delivery struct {
				request qualificationRequest
				err     error
			}
			delivered := make(chan delivery, 1)
			// The real dispatcher sends HTTP bytes to the real Python workflow consumer.
			receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload json.RawMessage
				err := json.NewDecoder(r.Body).Decode(&payload)
				if err == nil && (r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/actions/workflows/paxeer-x-qualification.yml/dispatches")) {
					err = fmt.Errorf("unexpected route %s %s", r.Method, r.URL.Path)
				}
				var routed qualificationRequest
				if err == nil {
					var output []byte
					output, err = repair314Python("workflow", payload)
					if err == nil {
						err = qStrict(output, &routed)
					}
				}
				delivered <- delivery{routed, err}
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer receiver.Close()
			client, err := newGitHubClient(config{GitHubAPIURL: receiver.URL, Owner: "local", Repo: "qualification"}, receiver.Client())
			if err != nil {
				t.Fatal(err)
			}
			if err = client.qDispatch(context.Background(), status.Record.Request); err != nil {
				t.Fatal(err)
			}
			routed := <-delivered
			if routed.err != nil {
				t.Fatal(routed.err)
			}
			if !reflect.DeepEqual(routed.request, req) {
				t.Fatal("workflow consumer received changed request")
			}

			run, attempt := int64(math.MaxInt64-int64(i)), int64(9007199254740993+int64(i))
			if _, err = reopened.advance(req.LogicalID, "prepared", "dispatch_intent", 0, 0, ""); err != nil {
				t.Fatal(err)
			}
			if _, err = reopened.advance(req.LogicalID, "dispatch_intent", "run_bound", run, attempt, ""); err != nil {
				t.Fatal(err)
			}
			reopened = nil
			reopened, err = newQualificationRegistry(root)
			if err != nil {
				t.Fatal(err)
			}
			recovered, err := reopened.read(req.LogicalID)
			if err != nil {
				t.Fatal(err)
			}
			if recovered.RunID != run || recovered.Attempt != attempt || !reopened.protects(run) {
				t.Fatal("run identity changed after reopen")
			}
			if reopened.protects(run - 1) {
				t.Fatal("adjacent run identity confused")
			}

			completed := qTestComplete(t)
			completed.Request = recovered.Request
			completed.RunID, completed.Attempt = recovered.RunID, recovered.Attempt
			completed.Domain = "github-actions"
			result := &completed.Results[0]
			result.LogicalID = req.LogicalID
			result.ManifestSHA256 = qHash(qTestCanonical(t, req.Manifest))
			result.RunID, result.Attempt = run, attempt
			result.Measurements["accepted"] = value
			completed.Jobs[0].ID = run
			completed.Jobs[0].RunnerID = run
			completed.Jobs[0].RunID = run
			if err = qSign(&completed, signer); err != nil {
				t.Fatal(err)
			}
			signed := qTestCanonical(t, completed)
			var roundtrip qualificationCompleted
			if err = qStrict(signed, &roundtrip); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(roundtrip, completed) {
				t.Fatal("signed evidence lost an integer")
			}
			evidence := filepath.Join(root, req.LogicalID+".completed.json")
			trust := filepath.Join(root, "trust-key")
			if err = qWrite(evidence, signed); err != nil {
				t.Fatal(err)
			}
			if err = qWrite(trust, signer.Public().(ed25519.PublicKey)); err != nil {
				t.Fatal(err)
			}
			if err = qVerifyFile(evidence, trust, time.Now().Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			for _, state := range []struct{ old, next string }{{"run_bound", "completed_uncollected"}, {"completed_uncollected", "completed_validated"}} {
				if _, err = reopened.advance(req.LogicalID, state.old, state.next, run, attempt, qHash(signed)); err != nil {
					t.Fatal(err)
				}
			}
			exported := qHandle(reopened, qTestCanonical(t, qualificationControl{Operation: "export", LogicalID: req.LogicalID}))
			if exported.Error != "" || exported.Completed == nil || !reflect.DeepEqual(*exported.Completed, completed) {
				t.Fatalf("export did not retain exact evidence: %s", exported.Error)
			}
			adjacent := value + 1
			if value == math.MaxInt64 {
				adjacent = value - 1
			}
			roundtrip.Results[0].Measurements["accepted"] = adjacent
			if err = qValidateCompleted(roundtrip, signer.Public().(ed25519.PublicKey), "github-actions", time.Now().Add(time.Minute)); err == nil {
				t.Fatal("adjacent measurement reused signature")
			}
			if err = qSign(&roundtrip, signer); err != nil {
				t.Fatal(err)
			}
			if err = qValidateCompleted(roundtrip, signer.Public().(ed25519.PublicKey), "github-actions", time.Now().Add(time.Minute)); err == nil {
				t.Fatal("signed adjacent measurement satisfied exact threshold")
			}
		})
	}
}
