package bringup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func m797Read(t *testing.T, path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func m797JSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func m797Write(t *testing.T, path string, data []byte) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, data, 0600); e != nil {
		t.Fatal(e)
	}
}
func m797CLI(args []string, raw []byte) ([]byte, error) {
	var out bytes.Buffer
	e := RunInspector(context.Background(), args, bytes.NewReader(raw), &out)
	return out.Bytes(), e
}
func m797Wire(t *testing.T, path string, terms ...string) {
	t.Helper()
	text := string(m797Read(t, path))
	for _, term := range terms {
		if !strings.Contains(text, term) {
			t.Fatalf("%s lost production wiring %q", path, term)
		}
	}
}
func m797Digest(raw []byte) string { hash := sha256.Sum256(raw); return hex.EncodeToString(hash[:]) }

func TestRepairM_797_FoundationProvenanceShape(t *testing.T) {
	directory := t.TempDir()
	var source struct {
		Revision string   `json:"revision"`
		Binding  string   `json:"source_binding"`
		Paths    []string `json:"source_paths"`
	}
	if e := json.Unmarshal(m797Read(t, "tests/fixtures/repair_m797/source.json"), &source); e != nil {
		t.Fatal(e)
	}
	var invocations []FoundationInvocation
	if e := json.Unmarshal(m797Read(t, "tests/fixtures/repair_m797/invocations.json"), &invocations); e != nil {
		t.Fatal(e)
	}
	output := m797Read(t, "tests/fixtures/repair_m797/source-tree.bin")
	if len(invocations) != 1 || invocations[0].Stdout != m797Digest(output) || source.Binding != m797Digest(output) {
		t.Fatal("actual Foundation.producer recording changed")
	}
	m797Write(t, filepath.Join(directory, "generated/source-tree.bin"), output)
	artifact := filepath.Join(directory, "artifact-manifest.json")
	bundle := m797Read(t, "runtime-bundle.json")
	m797Write(t, artifact, bundle)
	manifest := map[string]any{"version": 1, "stage": "dependency-foundation", "purpose": "disposable-test-only", "source_revision": source.Revision, "source_binding": source.Binding, "source_paths": source.Paths, "producers": invocations, "inputs": map[string]string{"source": "generated/source-tree.bin"}, "generated_artifacts": map[string]FoundationArtifact{"generated/source-tree.bin": {m797Digest(output), int64(len(output))}}, "artifact_manifest": artifact, "artifact_manifest_sha256": m797Digest(bundle)}
	// The production producer and both consuming handoffs receive precisely the
	// same structured manifest, with source and byte bindings retained.
	raw := m797JSON(t, manifest)
	produced, e := m797CLI([]string{"foundation-provenance", directory, source.Revision, source.Binding, "foundation"}, raw)
	if e != nil || !bytes.Equal(raw, produced) {
		t.Fatalf("producer refused/rewrote its real argv recording: %v", e)
	}
	for _, consumer := range []string{"registry-material", "policy-graph"} {
		if _, e := m797CLI([]string{"foundation-provenance", directory, source.Revision, source.Binding, consumer}, produced); e == nil {
			t.Fatal("consumer accepted missing qualification")
		}
	}
	// This is the successful wire qualification just executed above, not a
	// claim that the native dependency foundation or Docker suite ran here.
	m797Write(t, filepath.Join(directory, "qualification.json"), m797JSON(t, map[string]any{"source_revision": source.Revision, "exit_code": 0, "tests": 1, "skipped": 0}))
	for _, consumer := range []string{"registry-material", "policy-graph"} {
		accepted, e := m797CLI([]string{"foundation-provenance", directory, source.Revision, source.Binding, consumer}, produced)
		if e != nil || !bytes.Equal(accepted, produced) {
			t.Fatalf("real %s consumer rejected structured producer output: %v", consumer, e)
		}
		for _, mutation := range []string{"command-only", "missing-digest", "failure", "source", "artifact", "unbound-input", "skipped"} {
			t.Run(consumer+"/"+mutation, func(t *testing.T) {
				var changed map[string]any
				if e := json.Unmarshal(produced, &changed); e != nil {
					t.Fatal(e)
				}
				record := changed["producers"].([]any)[0].(map[string]any)
				switch mutation {
				case "command-only":
					delete(record, "argv")
					record["command"] = "git ls-tree"
				case "missing-digest":
					delete(record, "stdout_sha256")
				case "failure":
					record["exit_code"] = 1
				case "source":
					changed["source_binding"] = strings.Repeat("0", 64)
				case "artifact":
					changed["generated_artifacts"].(map[string]any)["generated/source-tree.bin"].(map[string]any)["sha256"] = strings.Repeat("0", 64)
				case "unbound-input":
					changed["inputs"].(map[string]any)["foreign"] = "generated/absent"
				case "skipped":
					m797Write(t, filepath.Join(directory, "qualification.json"), m797JSON(t, map[string]any{"source_revision": source.Revision, "exit_code": 0, "tests": 1, "skipped": 1}))
				}
				out, e := m797CLI([]string{"foundation-provenance", directory, source.Revision, source.Binding, consumer}, m797JSON(t, changed))
				if e == nil || len(out) != 0 {
					t.Fatalf("%s admitted invalid provenance: %s", consumer, mutation)
				}
				m797Write(t, filepath.Join(directory, "qualification.json"), m797JSON(t, map[string]any{"source_revision": source.Revision, "exit_code": 0, "tests": 1, "skipped": 0}))
			})
		}
	}
	m797Wire(t, "tests/paxeer-x-fixture-foundation.py", "qualify_foundation(self.manifest", "'argv': [str(x) for x in args]", "'stdout_sha256'", "'stderr_sha256'")
	m797Wire(t, "tests/paxeer-x-runtime-contract.py", "qualify_foundation(foundation, manifest_path.parent, revision, 'registry-material')", "qualify_foundation(foundation, manifest_path.parent, revision, 'policy-graph')")
}

func TestRepairM_797_RuntimeCARosterThirdShape(t *testing.T) {
	approved, e := ReadServiceRoster(bytes.NewReader(m797Read(t, "tests/fixtures/repair_m794/approved-roster.tsv")))
	if e != nil {
		t.Fatal(e)
	}
	declared, e := ReadServiceRoster(bytes.NewReader(m797Read(t, "ca-services.tsv")))
	if e != nil {
		t.Fatal(e)
	}
	if len(approved) != 30 || len(declared) != len(approved) {
		t.Fatal("merged inventory lost identities")
	}
	fingerprints := map[string]bool{}
	for i, row := range approved {
		if row.Name != declared[i].Name || row.Target != declared[i].Target || row.CN != declared[i].CN || row.EKU != declared[i].EKU || row.Custody != declared[i].Custody {
			t.Fatalf("issuer/approved inventory drift: %+v %+v", row, declared[i])
		}
		// These complete bundles were produced by actual ca.sh sign/derive_cmd.
		_, bundle := m794Bundle(t, row.Name)
		var transport bytes.Buffer
		if e := BundleReport(bundle, &transport); e != nil {
			t.Fatal(e)
		}
		state := InspectBundleReport(&transport, bundle["ca.pem"], time.Now(), row.CN, row.EKU, row.SANs)
		if state.State != "present" || state.Fingerprint == "" || fingerprints[state.Fingerprint] {
			t.Fatalf("issued identity not distinct/current: %s %+v", row.Name, state)
		}
		fingerprints[state.Fingerprint] = true
		other := approved[(i+1)%len(approved)]
		if CertificateInventory(bundle["cert.pem"], bundle["ca.pem"], time.Now(), 0, other.CN, other.EKU, other.SANs).State == "present" {
			t.Fatalf("%s can impersonate %s", row.Name, other.Name)
		}
	}
	_, attestor := m794Bundle(t, "human-attestor-client")
	_, ordinary := m794Bundle(t, "human-event-client")
	if CertificateInventory(attestor["cert.pem"], ordinary["ca.pem"], time.Now(), 0, "layerx-human-components", "clientAuth", "-").State != "foreign" {
		t.Fatal("attestor authority merged with internal issuer")
	}
	for _, name := range []string{"receipt-authority", "guarantor", "guarantor-1", "guarantor-2", "relay-archive", "layerxd"} {
		if _, e := ReadServiceRoster(strings.NewReader(name + " box:KERNEL_HOST volume obsolete clientAuth -\n")); e == nil {
			t.Fatalf("retired identity accepted: %s", name)
		}
	}
	m797Wire(t, "tests/paxeer-x-runtime-contract.py", "'human-kms-executor':", "'gateway-client': ('railway:gateway'", "every-pinned-identity-issued-and-inspected-row-by-row", "human-kms-roles-never-stand-in-for-one-another", "retired service regained an approved identity")
}

func m797NativeHome(t *testing.T) (string, string, *privval.FilePV) {
	t.Helper()
	home := t.TempDir()
	m797Write(t, filepath.Join(home, "config/config.toml"), []byte("node-key-file = \"config/node_key.json\"\ngenesis-file = \"config/genesis.json\"\n[priv-validator]\nkey-file = \"config/priv_validator_key.json\"\nstate-file = \"data/priv_validator_state.json\"\n"))
	m797Write(t, filepath.Join(home, "config/app.toml"), []byte("[sequencer]\nenable = true\nreceipt-key-file = \"config/receipt.key\"\n"))
	m797Write(t, filepath.Join(home, "config/receipt.key"), []byte(strings.Repeat("ab", 32)))
	key := tmtypes.GenNodeKey()
	if e := key.SaveAs(filepath.Join(home, "config/node_key.json")); e != nil {
		t.Fatal(e)
	}
	if e := os.MkdirAll(filepath.Join(home, "data"), 0700); e != nil {
		t.Fatal(e)
	}
	pv, e := privval.GenFilePV(filepath.Join(home, "config/priv_validator_key.json"), filepath.Join(home, "data/priv_validator_state.json"), "")
	if e != nil {
		t.Fatal(e)
	}
	if e = pv.Save(); e != nil {
		t.Fatal(e)
	}
	genesis, e := tmtypes.GenesisDocFromJSON(m797Read(t, "../../services/platform/hosted/node/tests/fixtures/repair_22_31_355/genesis.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = genesis.SaveAs(filepath.Join(home, "config/genesis.json")); e != nil {
		t.Fatal(e)
	}
	return home, genesis.ChainID, pv
}

func TestRepairM_797_RuntimeRetiredServiceAssertions(t *testing.T) {
	home, chain, pv := m797NativeHome(t)
	identity, e := RetainPaxdIdentity(home, chain, true)
	if e != nil {
		t.Fatal(e)
	}
	pv.LastSignState.Height = 9
	if e = pv.Save(); e != nil {
		t.Fatal(e)
	}
	chainPeer := m793RecordedChain(t)
	peer := m793RPCServer(t, chainPeer, nil)
	process := KernelProcess{Executable: "paxd", PID: 400, UID: 4020, Owner: 4020, Home: home, Mounted: true, Genesis: true, State: true, Running: true, StartTicks: 900, BootID: "boot-current"}
	var out bytes.Buffer
	if e = CheckKernelNode(context.Background(), peer.Client(), peer.URL, peer.URL, process, &out); e != nil || !strings.Contains(out.String(), "committed-height=11") {
		t.Fatal("actual committed RPC path", e, out.String())
	}
	peer.Close()
	// Close/reopen the transport and retained native codecs. Current process
	// identity must change across restart; validator signing state must not reset.
	reopened, e := RetainPaxdIdentity(home, chain, false)
	if e != nil || !reflect.DeepEqual(identity, reopened) {
		t.Fatal("restart replaced native identity", e)
	}
	recovered, e := privval.LoadFilePV(filepath.Join(home, "config/priv_validator_key.json"), filepath.Join(home, "data/priv_validator_state.json"))
	if e != nil || recovered.LastSignState.Height != 9 {
		t.Fatal("restart reset signing state", e)
	}
	peer = m793RPCServer(t, chainPeer, nil)
	process.PID++
	process.StartTicks++
	out.Reset()
	if e = CheckKernelNode(context.Background(), peer.Client(), peer.URL, peer.URL, process, &out); e != nil {
		t.Fatal(e)
	}
	for _, variant := range []string{"dead", "uid", "owner", "mount", "genesis", "state", "pid-reuse", "retired"} {
		t.Run(variant, func(t *testing.T) {
			changed := process
			switch variant {
			case "dead":
				changed.Running = false
			case "uid":
				changed.UID = 0
			case "owner":
				changed.Owner = 4021
			case "mount":
				changed.Mounted = false
			case "genesis":
				changed.Genesis = false
			case "state":
				changed.State = false
			case "pid-reuse":
				changed.StartTicks = 0
			case "retired":
				changed.Executable = "layerxd"
			}
			var out bytes.Buffer
			if e := CheckKernelNode(context.Background(), peer.Client(), peer.URL, peer.URL, changed, &out); e == nil || out.Len() != 0 {
				t.Fatal("invalid role established readiness", variant)
			}
		})
	}
	m797Wire(t, "tests/paxeer-x-runtime-contract.py", "from merged_kernel_contract import RoleDirectoryEvidenceRead", "def service_roster():", "--reuid=4020 --regid=4020 --clear-groups --no-new-privs")
	m797Wire(t, "tests/merged_kernel_contract.py", "self.runtime.CommittedReadiness", "self.runtime.readiness_server", "self.assertEqual(self.checkpoint.read_bytes(), before)", "os.setuid(4021)", "PID reuse established readiness")
}

func TestRepairM_797_RegistryMissingCreateWindow(t *testing.T) {
	directory := t.TempDir()
	if e := os.Chmod(directory, 0700); e != nil {
		t.Fatal(e)
	}
	m797Write(t, filepath.Join(directory, "lifecycle.json"), m797JSON(t, map[string]any{"schema_version": 1, "id": strings.Repeat("7", 32), "owner_uid": os.Getuid(), "directory": directory, "phase": "prepared", "containers": []any{}, "mount": nil}))
	engine := &m795DockerAPI{t: t, directory: directory, objects: map[string]dockerContainer{}}
	created := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			held := httptest.NewRecorder()
			engine.serve(held, r)
			if held.Code != 201 {
				t.Error("real lifecycle create configuration refused")
				w.WriteHeader(held.Code)
				return
			}
			once.Do(func() { close(created) })
			<-r.Context().Done() // Engine created it; owner never receives the ID.
			return
		}
		engine.serve(w, r)
	}))
	defer server.Close()
	peer := DockerPeer{server.Client(), server.URL + "/v1.41"}
	owner, e := OpenRegistryLifecycle(directory, peer)
	if e != nil {
		t.Fatal(e)
	}
	name, e := owner.Plan()
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, e := owner.Create(ctx, name, m797JSON(t, RegistryCreateConfiguration("sha256:"+strings.Repeat("1", 64), filepath.Join(directory, name), filepath.Join(directory, "quota"), filepath.Join(directory, "builder"))))
		finished <- e
	}()
	select {
	case <-created:
	case <-time.After(5 * time.Second):
		t.Fatal("created resource never observed")
	}
	var state map[string]any
	if e := json.Unmarshal(m797Read(t, filepath.Join(directory, "lifecycle.json")), &state); e != nil {
		t.Fatal(e)
	}
	record := state["containers"].([]any)[0].(map[string]any)
	if record["id"] != nil || record["phase"] != "planned" || record["cgroup"] != nil {
		t.Fatal("test missed created-before-publication window")
	}
	engine.mu.Lock()
	owned := len(engine.objects)
	engine.mu.Unlock()
	if owned != 1 {
		t.Fatal("engine resource did not exist before publication")
	}
	cancel()
	select {
	case e := <-finished:
		if e == nil {
			t.Fatal("lost create response accepted")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("interrupted owner retained lifecycle lock")
	}
	owner.Close()
	cleanup, e := OpenRegistryLifecycle(directory, peer)
	if e != nil {
		t.Fatal(e)
	}
	defer cleanup.Close()
	if e = cleanup.Cleanup(context.Background(), ""); e != nil {
		t.Fatal(e)
	}
	engine.mu.Lock()
	remaining, deletes := len(engine.objects), engine.deletes
	engine.mu.Unlock()
	if remaining != 0 || deletes != 1 {
		t.Fatal("owned orphan survived reopen", remaining, deletes)
	}
	state = nil
	if e := json.Unmarshal(m797Read(t, filepath.Join(directory, "lifecycle.json")), &state); e != nil {
		t.Fatal(e)
	}
	record = state["containers"].([]any)[0].(map[string]any)
	if record["id"] != strings.Repeat("a", 64) || record["container_absent"] != true {
		t.Fatal("recovered identity and absence not durable")
	}
	if e = cleanup.Cleanup(context.Background(), ""); e != nil {
		t.Fatal("cleanup was not idempotent", e)
	}
	m797Wire(t, "tests/registry-artifacts.py", "def test_98_created_resource_before_id_publication", "observed['id'] = json.loads(raw)['Id']", "self.assertIsNone(record['id']", "os.killpg(process.pid, signal.SIGKILL)", "'containers_absent', 'cgroups_absent', 'mount_absent', 'loop_absent'")
}

func TestRepairM_797_SearchRetiredEndpoints(t *testing.T) {
	inventory := m795Inventory(t)
	names := []string{}
	for _, endpoint := range inventory.Nodes {
		names = append(names, strings.TrimPrefix(endpoint, "https://"))
	}
	listing := "validator " + names[0] + "\nserve " + names[1] + "\nserve " + names[2] + "\n"
	selection, e := SelectSearchServing(inventory, strings.NewReader(listing), "", "")
	if e != nil {
		t.Fatal(e)
	}
	pair, certificate, _ := m795TLSIdentity(t, selection.Primary)
	var mu sync.Mutex
	ready := true
	reads := 0
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		reads++
		if r.URL.Path != "/readyz" || r.Host != selection.Primary {
			t.Error("unknown readiness target")
		}
		if !ready {
			w.WriteHeader(503)
		}
		json.NewEncoder(w).Encode(m795Readiness(time.Now(), ready))
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	client := m795NamedTLSClient(t, server, certificate)
	var rendered bytes.Buffer
	if e = RenderSearchFront(context.Background(), client, selection, &rendered); e != nil {
		t.Fatal(e)
	}
	accepted, e := m797CLI([]string{"search-pin", selection.Primary, selection.Backup}, rendered.Bytes())
	if e != nil || string(accepted) != selection.Primary+"\n" {
		t.Fatal("renderer->live consumer", e, string(accepted))
	}
	if e = ProbeSearchReady(context.Background(), client, "https://"+strings.TrimSpace(string(accepted)), time.Now()); e != nil {
		t.Fatal("live consumer readiness", e)
	}
	for _, bad := range []string{strings.ReplaceAll(rendered.String(), selection.Primary, names[0]), strings.ReplaceAll(rendered.String(), selection.Primary, "api15.mainnet-beta.paxeer.network"), strings.Replace(rendered.String(), selection.Primary+" 1;", selection.Primary+" 0;", 1), rendered.String() + "map $other $xweb_node {default api1;}"} {
		if out, e := m797CLI([]string{"search-pin", selection.Primary, selection.Backup}, []byte(bad)); e == nil || len(out) != 0 {
			t.Fatal("live consumer admitted obsolete/ineligible backend")
		}
	}
	mu.Lock()
	ready = false
	mu.Unlock()
	rendered.Reset()
	if e = RenderSearchFront(context.Background(), client, selection, &rendered); e != nil {
		t.Fatal(e)
	}
	if _, e = SearchFrontPin(selection, rendered.Bytes()); e == nil {
		t.Fatal("unready output admitted")
	}
	mu.Lock()
	seen := reads
	mu.Unlock()
	if seen < 3 {
		t.Fatal("real readiness path not exercised")
	}
	m797Wire(t, "search-front.sh", "search-pin", "search_front_candidate \"$pin\"")
	m797Wire(t, "check-live.sh", `search-front.sh" qualify <<<"$mounted"`)
	m797Wire(t, "tests/search-front.test.sh", "live_consumer_refuses_unadmitted_render", "layerx-gateway.layerx-testnet.svc.cluster.local/rpc", "config_receipt_key_ids_are_hex32", "https://api1.mainnet-beta.paxeer.network/")
}

func TestRepairM_797_RetirementRetainsAuthority(t *testing.T) {
	rows, e := ReadServiceRoster(bytes.NewReader(m797Read(t, "ca-services.tsv")))
	if e != nil || len(rows) != 30 {
		t.Fatal(e)
	}
	for _, row := range rows {
		for _, retired := range []string{"receipt-authority", "guarantor", "relay-archive", "layerxd"} {
			if strings.Contains(row.Name, retired) || strings.Contains(row.Target, retired) {
				t.Fatal("retired authority owns identity", row)
			}
		}
	}
	c := m793RecordedChain(t)
	peer := m793RPCServer(t, c, nil)
	receipt, e := ReadCommittedTransaction(context.Background(), peer.Client(), peer.URL, c.hash)
	if e != nil || receipt.Height != c.height || receipt.Block != c.block {
		t.Fatal("finality lacks real canonical receipt", e)
	}
	// Retired authority status cannot substitute for a governed committed receipt.
	weak := m793RPCServer(t, c, func(method string, v any) any {
		if method == "pax_sequencerReceipt" {
			v.(map[string]any)["rung"] = "receipted"
		}
		return v
	})
	if _, e = ReadCommittedTransaction(context.Background(), weak.Client(), weak.URL, c.hash); e == nil {
		t.Fatal("precommit authority evidence accepted as finality")
	}
	m797Wire(t, "tests/test_kernel_genesis_retired.py", "tools/paxeer-x/tests/retired_callers_test.py", "self.assertEqual(services.returncode, 0", `self.assertNotIn("receipt-authority", names)`, "latest_committed_height", "committed-transaction")
}

func TestRepairM_797_ValueLoopRetainsLNI(t *testing.T) {
	submission := m793Chain125Peer(t)
	submitted := m793RPCServer(t, submission, nil)
	var journey bytes.Buffer
	if e := CheckArchiveDelivery(context.Background(), submitted.Client(), submitted.URL, submission.raw, &journey); e != nil {
		t.Fatal("signed producer -> raw transaction transport -> committed read-back", e)
	}
	if !strings.Contains(journey.String(), submission.hash) {
		t.Fatal("transaction identity lost on committed read-back")
	}
	c := m793RecordedChain(t)
	peer := m793RPCServer(t, c, nil)
	// Real production transaction/receipt codecs and transport validate the
	// recorder's signed SEND and its canonical inclusion, without any LNI peer.
	tx, e := DecodeSignedTransaction(c.raw)
	if e == nil {
		t.Fatal("dev-chain recording unexpectedly qualified as chain 125", tx)
	}
	accepted, e := m797CLI([]string{"committed-transaction", peer.URL, c.hash, "-"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	var result map[string]string
	if e = json.Unmarshal(accepted, &result); e != nil || result["hash"] != c.hash || result["height"] != c.height || result["block"] != c.block {
		t.Fatal("committed read-back changed identity", e)
	}
	for _, variant := range []string{"receipt", "block", "key", "signature", "range"} {
		t.Run(variant, func(t *testing.T) {
			original := m793RecordedChain(t)
			bad := m793RPCServer(t, original, func(method string, v any) any {
				switch variant {
				case "receipt":
					if method == "eth_getTransactionReceipt" {
						v.(map[string]any)["status"] = "0x0"
					}
				case "block":
					if method == "eth_getBlockByNumber" {
						v.(map[string]any)["transactions"] = []string{}
					}
				case "key":
					if method == "px_getReceiptKeys" {
						v.(map[string]any)["keys"] = []any{}
					}
				case "signature":
					if method == "pax_sequencerReceipt" {
						v.(map[string]any)["receipt"] = "0x00"
					}
				case "range":
					if method == "px_getReceiptKeys" {
						v.(map[string]any)["keys"].([]any)[0].(map[string]any)["firstHeight"] = 11
					}
				}
				return v
			})
			if out, e := m797CLI([]string{"committed-transaction", bad.URL, original.hash, "-"}, nil); e == nil || len(out) != 0 {
				t.Fatal("invalid committed read-back accepted", variant)
			}
		})
	}
	source := string(m797Read(t, "value-loop.sh"))
	for _, retired := range []string{"layerxd.lni.sock", "--socket", "receipt-authority", "as_lni"} {
		if strings.Contains(source, retired) {
			t.Fatal("retired value-loop dependency", retired)
		}
	}
	m797Wire(t, "value-loop.sh", `committed-transaction "$rpc" "0x$activity"`, "send_journal reconcile", "eth_sendRawTransaction", "eth_getTransactionByHash", "latest_committed_height", `"$committed_height" -ge "$((inclusion_height))"`)
	m797Wire(t, "tests/test_value_loop_probe_contract.py", "--trust-anchor", "committed-transaction", "eth_sendRawTransaction")
}

func TestRepairM_797_XwebMissingLoader(t *testing.T) {
	c := m793RecordedChain(t)
	history := m797JSON(t, map[string]any{"keys": []any{c.key}})
	encoded, e := m797CLI([]string{"xweb-receipt-keys"}, history)
	if e != nil {
		t.Fatal(e)
	}
	var keys []XwebReceiptKey
	if e = json.Unmarshal(encoded, &keys); e != nil || len(keys) != 1 {
		t.Fatal("real trust transport failed", e)
	}
	key := keys[0]
	if key.ID != strings.TrimPrefix(c.key["keyId"].(string), "0x") || key.Public != strings.TrimPrefix(c.key["publicKey"].(string), "0x") || key.First != 10 || key.Last != 10 {
		t.Fatal("config lost produced key/range")
	}
	// Drive the transported key back through the real inclusion/signature
	// consumer. Loader adapter source below calls the actual Rust loader, not
	// an independent Go imitation of Config::parse.
	transported := m797JSON(t, map[string]any{"keys": []any{map[string]any{"keyId": "0x" + key.ID, "publicKey": "0x" + key.Public, "firstHeight": key.First, "lastHeight": key.Last}}})
	if e = VerifyCommittedReceipt(m797JSON(t, c.native), transported, c.hash); e != nil {
		t.Fatal("transported trust no longer authenticates native receipt", e)
	}
	for _, bad := range []string{`{"keys":[]}`, `{"keys":[{"keyId":"0x` + key.ID + `","publicKey":"0x` + strings.Repeat("0", 64) + `","firstHeight":10,"lastHeight":10}]}`, `{"keys":[{"keyId":"0x` + key.ID + `","publicKey":"0x` + key.Public + `","firstHeight":11,"lastHeight":10}]}`, string(bytes.Replace(history, []byte(`"keys":`), []byte(`"unsupported":true,"keys":`), 1))} {
		if out, e := m797CLI([]string{"xweb-receipt-keys"}, []byte(bad)); e == nil || len(out) != 0 {
			t.Fatal("invalid trust history admitted", bad)
		}
	}
	// A different valid key is refused by the actual receipt verifier too.
	key.Public = "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a"
	altered := m797JSON(t, map[string]any{"keys": []any{map[string]any{"keyId": "0x" + key.ID, "publicKey": "0x" + key.Public, "firstHeight": 10, "lastHeight": 10}}})
	if e = VerifyCommittedReceipt(m797JSON(t, c.native), altered, c.hash); e == nil {
		t.Fatal("altered receipt key accepted")
	}
	m797Wire(t, "xweb-config.sh", "xweb-receipt-keys", "px_getReceiptKeys")
	m797Wire(t, "tests/xweb-loader/src/main.rs", `services/interop/crates/x-websearch/src/config.rs`, "config::load", "key.public_key()", "key.first_batch_number()", "key.last_batch_number()")
	m797Wire(t, "tests/test_xweb_config.py", "load_config(path)", "load_config(altered, accepted=False)", "px_getReceiptKeys", "render\", \"attestor", "\"history\", \"unsupported\", \"legacy\", \"weak-key\"")
}
