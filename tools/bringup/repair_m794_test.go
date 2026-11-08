package bringup

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func m794Read(t *testing.T, path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func m794Write(t *testing.T, path string, b []byte) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func m794CLI(t *testing.T, args []string, input []byte) (string, error) {
	t.Helper()
	var out bytes.Buffer
	e := RunInspector(context.Background(), args, bytes.NewReader(input), &out)
	return out.String(), e
}
func m794Wire(t *testing.T, path string, terms ...string) {
	t.Helper()
	s := string(m794Read(t, path))
	for _, term := range terms {
		if !strings.Contains(s, term) {
			t.Fatalf("%s does not use %s", path, term)
		}
	}
}
func m794Bundle(t *testing.T, name string) (string, IdentityBundle) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	b := IdentityBundle{}
	for _, file := range identityFiles {
		data := m794Read(t, filepath.Join("tests/fixtures/repair_m794", name, file.File))
		m794Write(t, filepath.Join(dir, file.File), data)
		b[file.File] = data
	}
	return dir, b
}
func m794TLS(t *testing.T, role string, handler http.Handler) *httptest.Server {
	t.Helper()
	_, b := m794Bundle(t, role)
	cert, e := tls.X509KeyPair(b["cert.pem"], b["key.pem"])
	if e != nil {
		t.Fatal(e)
	}
	ca := x509.NewCertPool()
	if !ca.AppendCertsFromPEM(b["ca.pem"]) {
		t.Fatal("fixture CA")
	}
	s := httptest.NewUnstartedServer(handler)
	s.Config.ErrorLog = log.New(io.Discard, "", 0)
	s.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: ca}
	s.StartTLS()
	t.Cleanup(s.Close)
	return s
}
func m794ProducerPeer(t *testing.T, r *http.Request) {
	t.Helper()
	if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 {
		t.Error("mTLS producer was not verified")
		return
	}
	uris := r.TLS.PeerCertificates[0].URIs
	if len(uris) != 1 || uris[0].String() != "urn:layerx:webhooks:role:producer" {
		t.Error("producer role absent")
	}
}

func TestRepairM_794_RegistryRetiredEvents(t *testing.T) {
	row, e := ReadRolloutService(bytes.NewReader(m794Read(t, "../../deploy/rollout.kvx")), "program-registry")
	if e != nil || row.Target != "box" || row.HostKey() != "PROGRAM_REGISTRY_HOST" || row.Unit != "layerx-registry.service" {
		t.Fatalf("actual rollout: %+v %v", row, e)
	}
	declaration, e := abi.JSON(bytes.NewReader(m794Read(t, "../../precompiles/layerxprograms/abi.json")))
	if e != nil {
		t.Fatal(e)
	}
	definition := declaration.Events["ProgramDeployed"]
	program := common.HexToHash("0x" + strings.Repeat("c", 64))
	codeHash := [32]byte{7}
	encoded, e := definition.Inputs.NonIndexed().Pack(codeHash)
	if e != nil {
		t.Fatal(e)
	}
	// Use the committed Programs ABI's real event codec and geth's RPC log
	// codec. The loopback peer supplies canonical inclusion, not a live node.
	event := types.Log{Address: common.HexToAddress(ProgramsAddress), Topics: []common.Hash{definition.ID, program}, Data: encoded, BlockNumber: 257, BlockHash: common.HexToHash("0x1234"), TxHash: common.HexToHash("0x5678"), Index: 2}
	if event.Topics[0] != ProgramDeployedTopic {
		t.Fatal("event topic drift")
	}
	var mu sync.Mutex
	var starts []string
	fault := ""
	head := uint64(300)
	chain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Method string
			Params []json.RawMessage
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || r.URL.Path != "/" || r.Header.Get("Authorization") != "" {
			t.Error("chain transport drift")
			w.WriteHeader(400)
			return
		}
		var result any
		switch q.Method {
		case "eth_chainId":
			result = "0x7d"
		case "eth_blockNumber":
			result = fmt.Sprintf("0x%x", head)
		case "eth_getLogs":
			var filter struct {
				Address string
				Topics  []string
				From    string `json:"fromBlock"`
				To      string `json:"toBlock"`
			}
			if len(q.Params) != 1 || json.Unmarshal(q.Params[0], &filter) != nil || !strings.EqualFold(filter.Address, ProgramsAddress) || len(filter.Topics) != 2 || filter.Topics[0] != definition.ID.Hex() || filter.Topics[1] != program.Hex() {
				t.Error("current event filter lost")
				w.WriteHeader(400)
				return
			}
			mu.Lock()
			starts = append(starts, filter.From)
			mu.Unlock()
			from, _ := quantity(filter.From)
			to, _ := quantity(filter.To)
			result = []types.Log{}
			if from.Uint64() <= 257 && to.Uint64() >= 257 && fault != "absent" {
				v := event
				switch fault {
				case "removed":
					v.Removed = true
				case "address":
					v.Address = common.Address{}
				case "topic":
					v.Topics = []common.Hash{common.Hash{}, program}
				case "empty":
					v.Data = nil
				case "range":
					v.BlockNumber = 999
				}
				result = []types.Log{v}
			}
		case "eth_getBlockByNumber":
			hash := event.BlockHash
			if fault == "canonical" {
				hash = common.Hash{1}
			}
			result = map[string]any{"hash": hash, "number": "0x101", "transactions": []common.Hash{event.TxHash}}
		case "eth_getTransactionReceipt":
			status := "0x1"
			if fault == "receipt" {
				status = "0x0"
			}
			logs := []types.Log{event}
			if fault == "receipt-log" {
				logs = nil
			}
			result = map[string]any{"status": status, "transactionHash": event.TxHash, "blockHash": event.BlockHash, "blockNumber": "0x101", "logs": logs}
		default:
			t.Errorf("retired or unexpected chain method %s", q.Method)
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
	defer chain.Close()
	cursor := filepath.Join(t.TempDir(), "registry.cursor")
	observed, e := ObserveRegistryEvents(context.Background(), chain.Client(), chain.URL, strings.TrimPrefix(program.Hex(), "0x"), cursor)
	if e != nil || observed.TxHash != event.TxHash {
		t.Fatalf("committed observation: %+v %v", observed, e)
	}
	if !reflect.DeepEqual(starts, []string{"0x0", "0x100"}) {
		t.Fatalf("pagination %v", starts)
	}
	// Close and reopen the durable cursor. A new consumer revalidates its
	// retained event and continues from the saved block, without replaying it.
	f, e := os.Open(cursor)
	if e != nil {
		t.Fatal(e)
	}
	var saved RegistryCursor
	if e = json.NewDecoder(f).Decode(&saved); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	if saved.Next != 301 || saved.Match == nil || saved.Match.TxHash != event.TxHash {
		t.Fatalf("cursor not durable: %+v", saved)
	}
	head = 600
	starts = nil
	observed, e = ObserveRegistryEvents(context.Background(), chain.Client(), chain.URL, saved.Program, cursor)
	if e != nil || observed.TxHash != event.TxHash || !reflect.DeepEqual(starts, []string{"0x12d", "0x22d"}) {
		t.Fatalf("reopened cursor replayed or lost event: %v %v", starts, e)
	}
	now := time.Now()
	publication := map[string]any{"program_id": saved.Program, "lifecycle": "active", "observed_sequence": 600, "observed_at": now.UnixMilli(), "valid_through": now.Add(time.Minute).UnixMilli(), "versions": []any{map[string]any{"code_hash": hex.EncodeToString(encoded), "source": map[string]any{"status": "verified"}}}, "receipt": map[string]string{"verification": "receipt-verified", "deployment_receipt_digest": strings.Repeat("a", 64)}}
	registry := m794TLS(t, "registry", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m794ProducerPeer(t, r)
		if r.Method != "GET" || r.URL.Path != "/v1/programs/registry/"+saved.Program || r.Header.Get("Authorization") != "Bearer registry-read-fixture" {
			w.WriteHeader(401)
			return
		}
		json.NewEncoder(w).Encode(publication)
	}))
	_, bundle := m794Bundle(t, "registry-event-client")
	client, e := BundleTLSClient(bundle)
	if e != nil {
		t.Fatal(e)
	}
	defer client.CloseIdleConnections()
	if e = ProbeRegistryPublication(context.Background(), client, registry.URL, "Bearer registry-read-fixture", saved.Program, observed, now); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{"stale", "unpublished", "wrong-program", "wrong-code"} {
		t.Run(bad, func(t *testing.T) {
			p := map[string]any{}
			for k, v := range publication {
				p[k] = v
			}
			switch bad {
			case "stale":
				p["valid_through"] = now.Add(-time.Millisecond).UnixMilli()
			case "unpublished":
				p["versions"] = []any{map[string]any{"code_hash": hex.EncodeToString(encoded), "source": map[string]string{"status": "unpublished"}}}
			case "wrong-program":
				p["program_id"] = strings.Repeat("e", 64)
			case "wrong-code":
				p["versions"] = []any{map[string]any{"code_hash": strings.Repeat("a", 64), "source": map[string]string{"status": "verified"}}}
			}
			old := publication
			publication = p
			defer func() { publication = old }()
			if e = ProbeRegistryPublication(context.Background(), client, registry.URL, "Bearer registry-read-fixture", saved.Program, observed, now); e == nil {
				t.Fatal("invalid publication accepted")
			}
		})
	}
	for _, bad := range []string{"removed", "address", "topic", "empty", "range", "canonical", "receipt", "receipt-log", "absent"} {
		t.Run(bad, func(t *testing.T) {
			fault = bad
			path := filepath.Join(t.TempDir(), "cursor")
			if _, e := ObserveRegistryEvents(context.Background(), chain.Client(), chain.URL, saved.Program, path); e == nil {
				t.Fatal("invalid committed event accepted")
			}
			if bad == "absent" {
				var c RegistryCursor
				if e = json.Unmarshal(m794Read(t, path), &c); e != nil || c.Next != 601 || c.Match != nil {
					t.Fatal("empty page lost durable progress")
				}
			}
		})
	}
	fault = ""
	m794Write(t, cursor, []byte(`{"Endpoint":"foreign","Program":"bad","Next":601}`))
	if _, e = ObserveRegistryEvents(context.Background(), chain.Client(), chain.URL, saved.Program, cursor); e == nil {
		t.Fatal("foreign cursor accepted")
	}
	m794Wire(t, "check-live.sh", `rollout-service "$repo_root/deploy/rollout.kvx" program-registry`, `registry-observe`, `registry-health`)
	registrySource := string(m794Read(t, "check-live.sh"))
	registrySource = registrySource[strings.Index(registrySource, "check_registry() {"):strings.Index(registrySource, "check_registry_bootstrap() {")]
	for _, retired := range []string{"lx_getProgramEvents", "fly.toml", "flyctl", "$router/rpc"} {
		if strings.Contains(registrySource, retired) {
			t.Fatalf("retired registry path %s", retired)
		}
	}
}

func TestRepairM_794_EventsUpstreamPlatformDrift(t *testing.T) {
	kinds := []struct{ kind, identity string }{{"journey", "human-event-client"}, {"approval", "human-event-client"}, {"payment", "gateway-client"}, {"program", "registry-event-client"}}
	for _, item := range kinds {
		t.Run(item.kind, func(t *testing.T) {
			dir, _ := m794Bundle(t, item.identity)
			fault := ""
			var calls []string
			handler := func(role string) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					m794ProducerPeer(t, r)
					path := "/internal/v1/producer-readiness"
					token := "Bearer source-fixture"
					if role == "webhook-trigger" {
						path = "/internal/v1/readiness/" + item.kind
						token = "Bearer trigger-fixture"
					}
					if r.Method != "GET" || r.URL.Path != path || r.Header.Get("Authorization") != token {
						w.WriteHeader(401)
						return
					}
					calls = append(calls, role)
					kind := item.kind
					ready := true
					if fault == role+"-kind" {
						kind = "foreign"
					}
					if fault == role+"-ready" {
						ready = false
					}
					json.NewEncoder(w).Encode(map[string]any{"schema": "layerx.event-admission.v1", "role": role, "kind": kind, "ready": ready, "generation": 7, "principals": 1, "principal_digest": true, "fresh_for_ms": 10000})
				})
			}
			source := m794TLS(t, "internal-"+map[string]string{"journey": "journeys", "approval": "approvals", "payment": "payments", "program": "programs"}[item.kind], handler("source-producer"))
			trigger := m794TLS(t, "developer", handler("webhook-trigger"))
			env := map[string]string{}
			for prefix, endpoint := range map[string]string{"LAYERX_EVENTS_" + strings.ToUpper(item.kind): source.URL, "LAYERX_EVENTS_WEBHOOKS": trigger.URL} {
				env[prefix+"_UPSTREAM_URL"] = endpoint
				env[prefix+"_UPSTREAM_CA_DER"] = filepath.Join(dir, "ca.der")
				env[prefix+"_UPSTREAM_CLIENT_IDENTITY_PKCS12"] = filepath.Join(dir, "identity.p12")
				env[prefix+"_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE"] = filepath.Join(dir, "password")
				token := []byte("source-fixture")
				if prefix == "LAYERX_EVENTS_WEBHOOKS" {
					token = []byte("trigger-fixture")
				}
				path := filepath.Join(dir, prefix+".token")
				m794Write(t, path, token)
				env[prefix+"_UPSTREAM_TOKEN_FILE"] = path
			}
			var out bytes.Buffer
			if e := ProbeEventHandoff(context.Background(), env, item.kind, &out); e != nil || !strings.Contains(out.String(), "kind="+item.kind) || !reflect.DeepEqual(calls, []string{"source-producer", "webhook-trigger"}) {
				t.Fatalf("real producer handoff: %s %v %v", out.String(), calls, e)
			}
			for _, bad := range []string{"source-producer-kind", "source-producer-ready", "webhook-trigger-kind", "webhook-trigger-ready"} {
				fault = bad
				out.Reset()
				if e := ProbeEventHandoff(context.Background(), env, item.kind, &out); e == nil || out.Len() != 0 {
					t.Fatal("invalid handoff produced a success")
				}
			}
			fault = ""
			for _, suffix := range []string{"_UPSTREAM_CLIENT_IDENTITY_PKCS12", "_UPSTREAM_TOKEN_FILE", "_UPSTREAM_CLIENT_IDENTITY_PASSWORD_FILE", "_UPSTREAM_CA_DER"} {
				copyEnv := map[string]string{}
				for k, v := range env {
					copyEnv[k] = v
				}
				copyEnv["LAYERX_EVENTS_WEBHOOKS"+suffix] = filepath.Join(dir, "absent")
				out.Reset()
				if e := ProbeEventHandoff(context.Background(), copyEnv, item.kind, &out); e == nil || out.Len() != 0 {
					t.Fatal("missing producer credential accepted")
				}
			}
		})
	}
	deployment := []byte(`{"latestDeployment":{"id":"deploy-current","status":"SUCCESS","instances":[{"id":"replica-one","status":"RUNNING"}],"meta":{"serviceManifest":{"deploy":{"numReplicas":1}}}}}`)
	wire, e := m794CLI(t, []string{"role-instances"}, deployment)
	if e != nil || wire != "replica-one\n" {
		t.Fatalf("role selection: %q %v", wire, e)
	}
	for _, bad := range []string{strings.Replace(string(deployment), "RUNNING", "STOPPED", 1), strings.Replace(string(deployment), `"numReplicas":1`, `"numReplicas":2`, 1), strings.Replace(string(deployment), `"SUCCESS"`, `"FAILED"`, 1)} {
		if _, e = m794CLI(t, []string{"role-instances"}, []byte(bad)); e == nil {
			t.Fatal("stale deployment selected")
		}
	}
	for _, name := range []string{"internal-journeys", "internal-approvals", "internal-payments", "internal-programs", "webhooks-ingress", "router"} {
		row, e := ReadRolloutService(bytes.NewReader(m794Read(t, "../../deploy/rollout.kvx")), name)
		if e != nil || row.Target != "railway" || row.Service != name {
			t.Fatalf("current role lookup %s: %v", name, e)
		}
	}
	m794Wire(t, "check-live.sh", `role_instances "$app"`, `railway_ssh "$app" "$command" "$instance"`, `event-handoff`, `event-role`, `event-trigger`)
}

func TestRepairM_794_CAObsoleteRoster(t *testing.T) {
	expected := strings.Fields("pending-core pending-core-admin agent-boundary agentd agentd-client paxeer-boundary-loopback paxeer-boundary-public human human-event-client human-attestor-client human-kms human-kms-client human-kms-executor gateway-redis gateway-client identity internal-kms internal-journeys internal-payments internal-approvals internal-programs internal-redis registry registry-event-client indexer interop-client developer developer-client dashboard-client ramp-client")
	rows, e := ReadServiceRoster(bytes.NewReader(m794Read(t, "ca-services.tsv")))
	if e != nil || len(rows) != len(expected) {
		t.Fatal("approved identity inventory differs", e)
	}
	approved, e := ReadServiceRoster(bytes.NewReader(m794Read(t, "tests/fixtures/repair_m794/approved-roster.tsv")))
	if e != nil {
		t.Fatal(e)
	}
	_, ordinary := m794Bundle(t, "human-event-client")
	for i, row := range rows {
		if row.Name != expected[i] || approved[i].Name != row.Name || approved[i].Target != row.Target || approved[i].CN != row.CN || approved[i].EKU != row.EKU {
			t.Fatalf("unapproved row %+v", row)
		}
		dir, bundle := m794Bundle(t, row.Name)
		if e = BundleConsistent(bundle); e != nil {
			t.Fatalf("real issuance %s: %v", row.Name, e)
		}
		var report bytes.Buffer
		if e = BundleReport(bundle, &report); e != nil {
			t.Fatal(e)
		}
		state := InspectBundleReport(&report, bundle["ca.pem"], time.Now(), row.CN, row.EKU, approved[i].SANs)
		if state.State != "present" {
			t.Fatalf("issuance %s: %+v", row.Name, state)
		}
		if row.Name == "human-attestor-client" {
			state = CertificateInventory(bundle["cert.pem"], ordinary["ca.pem"], time.Now(), 0, row.CN, row.EKU, approved[i].SANs)
			if state.State != "foreign" {
				t.Fatal("attestor authority merged into internal CA")
			}
		}
		if row.Name == "human-event-client" {
			for _, bad := range []string{"cert.der", "key.der", "ca.der", "identity.p12", "password", "missing"} {
				t.Run(bad, func(t *testing.T) {
					copyBundle := IdentityBundle{}
					for k, v := range bundle {
						copyBundle[k] = v
					}
					if bad == "missing" {
						delete(copyBundle, "key.der")
					} else {
						copyBundle[bad] = []byte("malformed")
					}
					if e = BundleConsistent(copyBundle); e == nil {
						t.Fatal("malformed material accepted")
					}
				})
			}
			out, e := m794CLI(t, []string{"bundle-local", dir}, nil)
			if e != nil || !strings.HasPrefix(out, "bundle=consistent") {
				t.Fatalf("custody transport: %s %v", out, e)
			}
		}
	}
	for _, retired := range []string{"guarantor", "relay-archive", "receipt-authority"} {
		if _, e = ReadServiceRoster(strings.NewReader(retired + " box:KERNEL_HOST volume retired serverAuth -\n")); e == nil {
			t.Fatal("retired success assumption retained")
		}
	}
	m794Wire(t, "check-live.test.sh", "expected_ca_services=(pending-core", `fail human-kms target=box:KERNEL_HOST`, `target=railway:gateway`)
}

func TestRepairM_794_EdgeThirdShape(t *testing.T) {
	var manifest, sites strings.Builder
	for _, item := range [][]string{{"ramp.paxeer.network", "railway", "ramp.up.railway.app:443", "-"}, {"hooks.paxeer.network", "box", "private.box.test:8443", "hooks.box.test"}, {"plain.paxeer.network", "box", "10.20.0.2:8080", "-"}, {"machine.paxeer.network:443", "stream", "10.20.0.3:9420", "-"}} {
		line, e := m794CLI(t, append([]string{"edge-map"}, item...), nil)
		if e != nil {
			t.Fatal(e)
		}
		manifest.WriteString(line)
		rows, e := ReadEdgeManifest(strings.NewReader(line), false)
		if e != nil {
			t.Fatal(e)
		}
		r := rows[0]
		if r.Mode == "http" {
			out, e := m794CLI(t, []string{"edge-http", r.Name, r.Kind, r.Upstream, r.SNI, "tls", "/var/www/certbot", "/etc/letsencrypt/live", "127.0.0.53", "1"}, nil)
			if e != nil {
				t.Fatal(e)
			}
			sites.WriteString(out)
		}
	}
	stream, e := m794CLI(t, []string{"edge-stream", "443", "127.0.0.53"}, []byte(manifest.String()))
	if e != nil {
		t.Fatal(e)
	}
	sites.WriteString(stream)
	if strings.Contains(stream, `\t`) {
		t.Fatal("nginx stream output contains literal tab escapes")
	}
	pool := filepath.Join(t.TempDir(), "pool")
	down := filepath.Join(t.TempDir(), "down")
	m794Write(t, pool, []byte("api1.mainnet-beta.paxeer.network\napi2.mainnet-beta.paxeer.network\n"))
	m794Write(t, down, []byte("api2.mainnet-beta.paxeer.network\n"))
	rpc, e := m794CLI(t, []string{"edge-rpc", "rpc.paxeer.network", "tls", "/var/www/certbot", "/etc/letsencrypt/live", "127.0.0.53", "1", pool, down}, nil)
	if e != nil {
		t.Fatal(e)
	}
	manifest.WriteString("rpc.paxeer.network rpc - 443 rpc -\n")
	sites.WriteString(rpc)
	frame := manifest.String() + "@@sites\n" + sites.String()
	out, e := m794CLI(t, []string{"edge-contract"}, []byte(frame))
	if e != nil || out != manifest.String() {
		t.Fatalf("renderer -> transport -> real decoder: %v %s", e, out)
	}
	selected, e := m794CLI(t, []string{"edge-ramp", "ramp.paxeer.network"}, []byte(frame))
	if e != nil || !strings.HasPrefix(selected, "ramp.paxeer.network http ramp.up.railway.app:443 443 railway -") {
		t.Fatal(selected, e)
	}
	members, e := m794CLI(t, []string{"edge-contract", "rpc.paxeer.network", "443"}, []byte(frame))
	if e != nil || members != "api1.mainnet-beta.paxeer.network\n" {
		t.Fatal(members, e)
	}
	for _, bad := range []string{strings.Replace(frame, "ramp.up.railway.app:443 443 railway -", "ramp-app 443", 1), manifest.String() + strings.Split(manifest.String(), "\n")[0] + "\n@@sites\n" + sites.String(), strings.Replace(frame, "proxy_ssl_verify on;", "proxy_ssl_verify off;", 1), strings.Replace(frame, "set $edge_upstream ramp.up.railway.app:443;", "set $edge_upstream unregistered.up.railway.app:443;", 1), strings.ReplaceAll(frame, "max_fails=2 fail_timeout=10s;", "max_fails=2 fail_timeout=10s down;")} {
		if _, e = m794CLI(t, []string{"edge-contract"}, []byte(bad)); e == nil {
			t.Fatal("malformed or mismatched rendered route accepted")
		}
	}
	if _, e = m794CLI(t, []string{"edge-ramp", "unregistered.paxeer.network"}, []byte(frame)); e == nil {
		t.Fatal("unregistered ramp admitted")
	}
	if _, e = m794CLI(t, []string{"edge-map", "api15.mainnet-beta.paxeer.network", "railway", "node.up.railway.app:443", "-"}, nil); e == nil {
		t.Fatal("independent node claimed by edge")
	}
	m794Wire(t, "edge.sh", "edge-map", "edge-http", "edge-stream", "edge-rpc")
	m794Wire(t, "check-live.test.sh", `edge_fixture`, `"$root/tools/bringup/edge.sh" set`)
}

func TestRepairM_794_InternalPlatformShape(t *testing.T) {
	clientDir, bundle := m794Bundle(t, "human-event-client")
	calls := 0
	for _, group := range []string{"kms", "journeys", "payments", "approvals", "programs"} {
		t.Run(group, func(t *testing.T) {
			row, e := ReadRolloutService(bytes.NewReader(m794Read(t, "../../deploy/rollout.kvx")), "internal-"+group)
			if e != nil {
				t.Fatal(e)
			}
			notReady := false
			peer := m794TLS(t, "internal-"+group, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				m794ProducerPeer(t, r)
				if r.URL.Path != "/readyz" || r.Method != "GET" || r.Host != row.Service+".railway.internal:9443" {
					t.Error("readiness role route drift")
					w.WriteHeader(400)
					return
				}
				calls++
				if notReady {
					w.WriteHeader(503)
					io.WriteString(w, `{"ready":false,"state":"waiting-principals"}`)
				} else {
					io.WriteString(w, `{"ready":true}`)
				}
			}))
			connect := strings.TrimPrefix(peer.URL, "https://")
			url := "https://" + row.Service + ".railway.internal:9443/readyz"
			out, e := m794CLI(t, []string{"internal-readiness", url, clientDir, connect}, nil)
			if e != nil || out != "pass readiness http=200 ready=true identity=human-event-client\n" {
				t.Fatalf("real CA -> box custody -> service TLS: %s %v", out, e)
			}
			notReady = true
			if out, e = m794CLI(t, []string{"internal-readiness", url, clientDir, connect}, nil); e == nil || out != "" {
				t.Fatal("unready role accepted")
			}
			_, wrong := m794Bundle(t, "human-attestor-client")
			wrongClient, e := BundleTLSClient(wrong)
			if e != nil {
				t.Fatal(e)
			}
			defer wrongClient.CloseIdleConnections()
			if e = SelectInternalOrigin(wrongClient, url, connect); e != nil {
				t.Fatal(e)
			}
			if e = ProbeInternalReady(context.Background(), wrongClient, url); e == nil {
				t.Fatal("foreign authority admitted")
			}
		})
	}
	if calls != 10 {
		t.Fatalf("role coverage: %d", calls)
	}
	anonymous, e := liveHTTPClient(filepath.Join(clientDir, "ca.pem"))
	if e != nil {
		t.Fatal(e)
	}
	peer := m794TLS(t, "internal-kms", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("absent credential passed TLS") }))
	if e = ProbeInternalReady(context.Background(), anonymous, peer.URL+"/readyz"); e == nil {
		t.Fatal("absent client identity admitted")
	}
	anonymous.CloseIdleConnections()
	if e = os.Remove(filepath.Join(clientDir, "identity.p12")); e != nil {
		t.Fatal(e)
	}
	if out, e := m794CLI(t, []string{"internal-readiness", peer.URL + "/readyz", clientDir}, nil); e == nil || out != "" {
		t.Fatal("absent custody accepted")
	}
	if bytes.Contains(bundle["cert.pem"], []byte("PRIVATE KEY")) {
		t.Fatal("public certificate contains private material")
	}
	m794Wire(t, "check-live.sh", `ssh_read "$KERNEL_HOST" "$command"`, `internal-readiness %q %q %q`)
	m794Wire(t, "check-live.test.sh", "check_live_internal_absent_credentials", "internal-kms.railway.internal")
	m794Wire(t, "tests/fixtures/repair_m794/peers.py", "ssl.CERT_REQUIRED")
}

func TestRepairM_794_DocsNamePolicy(t *testing.T) {
	inventory, e := ReadPublicInventory(bytes.NewReader(m794Read(t, "../../docs/site/docs/reference/public-rpc.md")))
	if e != nil {
		t.Fatal(e)
	}
	allowed, e := DocumentationNames(inventory, bytes.NewReader(m794Read(t, "../../spec/layerx-merge/spec.kvx")))
	if e != nil {
		t.Fatal(e)
	}
	if len(inventory.Nodes) != 14 || !allowed["api-mainnet-beta.paxeer.network"] || !allowed["mainnet-beta.paxeer.network"] || allowed["api15.mainnet-beta.paxeer.network"] || allowed["api16.mainnet-beta.paxeer.network"] {
		t.Fatal("merged public-name set differs")
	}
	var text strings.Builder
	for _, endpoint := range inventory.Endpoints() {
		text.WriteString(endpoint + "\n")
	}
	text.WriteString("https://hooks.paxeer.network\nhttps://dev.paxeer.network\n")
	if e = ValidateDocumentNames(inventory, allowed, text.String()); e != nil {
		t.Fatal(e)
	}
	for _, good := range []string{"LAYERX_FAUCET_URL", "LAYERX_GATEWAY_FAUCET_URL", "LAYERX_SCHEDULED_FAUCET_URL", "FAUCET_URL_SUFFIX", "1FAUCET_URL", "FAUCET_URL1", "mainnet-beta.paxeer.network", "https://<node>.mainnet-beta.paxeer.network"} {
		if e = ValidateDocumentNames(inventory, allowed, good); e != nil {
			t.Fatalf("published documentation refused: %s: %v", good, e)
		}
	}
	for _, good := range []string{"http://127.0.0.1:$COMET_RPC_PORT/status", "http://127.0.0.1:${COMET_RPC_PORT}/validators?page=1&per_page=100"} {
		if e = ValidateDocumentNames(inventory, allowed, good); e != nil {
			t.Fatalf("local shell URL refused: %s: %v", good, e)
		}
	}
	for _, bad := range []string{inventory.Nodes[0] + ":$PORT", inventory.Nodes[0] + ":${PORT}/", inventory.Nodes[0] + ":$PORT/rpc", "http://127.0.0.1:invalid/status", "http://127.0.0.1:${}/status"} {
		if e = ValidateDocumentNames(inventory, allowed, bad); e == nil {
			t.Fatalf("routed or malformed documentation accepted: %s", bad)
		}
	}
	for _, bad := range []string{"FAUCET_URL", "`FAUCET_URL`", "(FAUCET_URL)", "FAUCET_URL=value", "https://unpublished.mainnet-beta.paxeer.network"} {
		if e = ValidateDocumentNames(inventory, allowed, bad); e == nil {
			t.Fatalf("unpublished documentation accepted: %s", bad)
		}
	}
	for _, bad := range []string{"https://api15.mainnet-beta.paxeer.network", "https://api16.mainnet-beta.paxeer.network", inventory.Canonical + "/rpc", inventory.Nodes[0] + "/rpc", inventory.Nodes[0] + ":443/rpc", inventory.Nodes[0] + "?route=rpc", "https://unpublished.paxeer.network", "FAUCET_URL=https://faucet.paxeer.network"} {
		if e = ValidateDocumentNames(inventory, allowed, bad); e == nil {
			t.Fatalf("unpublished or retired documentation accepted: %s", bad)
		}
	}
	for _, endpoint := range inventory.Endpoints() {
		if e = ValidateDocumentNames(inventory, allowed, endpoint+"/"); e != nil {
			t.Fatal("root slash refused", e)
		}
	}
	m794Wire(t, "docs-names.sh", "docs/site/docs/reference/public-rpc.md", `docs-names "$inventory" "$spec"`)
}

func m794Stage(t *testing.T, root string, names ...string) {
	t.Helper()
	var manifest strings.Builder
	for _, name := range names {
		line, e := m794CLI(t, []string{"edge-map", name, "railway", "app.up.railway.app:443", "-"}, nil)
		if e != nil {
			t.Fatal(e)
		}
		manifest.WriteString(line)
		body, e := m794CLI(t, []string{"edge-http", name, "railway", "app.up.railway.app:443", "-", "tls", "/var/www/certbot", "/etc/letsencrypt/live", "127.0.0.53", "0"}, nil)
		if e != nil {
			t.Fatal(e)
		}
		available := filepath.Join(root, "sites-available", name+".conf")
		m794Write(t, available, []byte(body))
		if e = os.MkdirAll(filepath.Join(root, "sites-enabled"), 0755); e != nil {
			t.Fatal(e)
		}
		enabled := filepath.Join(root, "sites-enabled", name+".conf")
		if e = os.Remove(enabled); e != nil && !os.IsNotExist(e) {
			t.Fatal(e)
		}
		if e = os.Symlink(available, enabled); e != nil {
			t.Fatal(e)
		}
	}
	m794Write(t, filepath.Join(root, "edge/manifest"), []byte(manifest.String()))
}
func m794Open(t *testing.T, root string) *EdgeState {
	t.Helper()
	s, e := OpenEdgeState(root)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Recover(); e != nil {
		t.Fatal(e)
	}
	return s
}
func m794Sync(t *testing.T, primary, secondary *EdgeState) bool {
	t.Helper()
	var wire bytes.Buffer
	if e := primary.Export(&wire); e != nil {
		t.Fatal(e)
	}
	changed, e := secondary.Import(&wire)
	if e != nil {
		t.Fatal(e)
	}
	if changed {
		if e = secondary.Finish(true); e != nil {
			t.Fatal(e)
		}
	}
	return changed
}
func TestRepairM_794_EdgeSecondaryStaleSites(t *testing.T) {
	primaryRoot, secondaryRoot := t.TempDir(), t.TempDir()
	primary := m794Open(t, primaryRoot)
	secondary := m794Open(t, secondaryRoot)
	if e := primary.Begin(); e != nil {
		t.Fatal(e)
	}
	m794Stage(t, primaryRoot, "ramp.paxeer.network", "hooks.paxeer.network")
	if e := primary.Finish(true); e != nil {
		t.Fatal(e)
	}
	m794Write(t, filepath.Join(secondaryRoot, "sites-available/operator.conf"), []byte("# operator-owned\nserver {}\n"))
	if e := os.MkdirAll(filepath.Join(secondaryRoot, "sites-enabled"), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink("../sites-available/operator.conf", filepath.Join(secondaryRoot, "sites-enabled/operator.conf")); e != nil {
		t.Fatal(e)
	}
	if !m794Sync(t, primary, secondary) {
		t.Fatal("initial route transfer unchanged")
	}
	first, e := EdgeGeneration(secondaryRoot)
	if e != nil {
		t.Fatal(e)
	}
	if m794Sync(t, primary, secondary) {
		t.Fatal("identical sync produced a new generation")
	}
	m794Write(t, filepath.Join(secondaryRoot, "edge/rpc-down"), []byte("node-a.example.org\n"))
	if e = primary.Begin(); e != nil {
		t.Fatal(e)
	}
	m794Stage(t, primaryRoot, "hooks.paxeer.network")
	if e = primary.Reconcile(); e != nil {
		t.Fatal(e)
	}
	if e = primary.Finish(true); e != nil {
		t.Fatal(e)
	}
	if !m794Sync(t, primary, secondary) {
		t.Fatal("route revocation unchanged")
	}
	for _, dir := range []string{"sites-available", "sites-enabled"} {
		if _, e = os.Lstat(filepath.Join(secondaryRoot, dir, "ramp.paxeer.network.conf")); !os.IsNotExist(e) {
			t.Fatal("removed secondary site or symlink survived", e)
		}
	}
	if e = secondary.Close(); e != nil {
		t.Fatal(e)
	}
	secondary = m794Open(t, secondaryRoot)
	defer secondary.Close()
	generation, e := EdgeGeneration(secondaryRoot)
	if e != nil || generation == first {
		t.Fatal("revocation generation not recovered")
	}
	if got := string(m794Read(t, filepath.Join(secondaryRoot, "edge/manifest"))); strings.Contains(got, "ramp.paxeer.network") || !strings.Contains(got, "hooks.paxeer.network") {
		t.Fatal("reopen revived retired route")
	}
	if got := string(m794Read(t, filepath.Join(secondaryRoot, "edge/rpc-down"))); got != "node-a.example.org\n" {
		t.Fatal("receiver health was overwritten")
	}
	if string(m794Read(t, filepath.Join(secondaryRoot, "sites-enabled/operator.conf"))) != "# operator-owned\nserver {}\n" {
		t.Fatal("unrelated nginx site changed")
	}
	// The ownership receipt also revokes a dangling marker-owned link.
	if e = secondary.Begin(); e != nil {
		t.Fatal(e)
	}
	m794Stage(t, secondaryRoot, "dangling.paxeer.network")
	if e = secondary.Finish(true); e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(filepath.Join(secondaryRoot, "sites-available/dangling.paxeer.network.conf")); e != nil {
		t.Fatal(e)
	}
	m794Write(t, filepath.Join(secondaryRoot, "edge/manifest"), nil)
	if e = secondary.Begin(); e != nil {
		t.Fatal(e)
	}
	if e = secondary.Reconcile(); e != nil {
		t.Fatal(e)
	}
	if e = secondary.Finish(true); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Lstat(filepath.Join(secondaryRoot, "sites-enabled/dangling.paxeer.network.conf")); !os.IsNotExist(e) {
		t.Fatal("dangling owned link survived")
	}
	m794Wire(t, "edge-sync.sh", "edge-export", "edge-import", "rerender --imported", ".edge-operation.lock")
}

func TestRepairM_794_EdgeReloadDurableState(t *testing.T) {
	root := t.TempDir()
	state := m794Open(t, root)
	if e := state.Begin(); e != nil {
		t.Fatal(e)
	}
	m794Stage(t, root, "serving.paxeer.network")
	m794Write(t, filepath.Join(root, "edge/rpc-pool"), []byte("node-a.example.org\n"))
	if e := state.Finish(true); e != nil {
		t.Fatal(e)
	}
	generation, e := EdgeGeneration(root)
	if e != nil {
		t.Fatal(e)
	}
	original := m794Read(t, filepath.Join(root, "sites-available/serving.paxeer.network.conf"))
	m794Write(t, filepath.Join(root, "sites-available/operator.conf"), []byte("# keep me\n"))
	if e = state.Begin(); e != nil {
		t.Fatal(e)
	}
	m794Stage(t, root, "unacknowledged.paxeer.network")
	m794Write(t, filepath.Join(root, "edge/rpc-pool"), []byte("node-b.example.org\n"))
	if e = state.Reconcile(); e != nil {
		t.Fatal(e)
	}
	// The production apply consumer maps a refused nginx reload to the real
	// rollback operation; no reload daemon or subprocess toolchain is required.
	if out, e := m794CLI(t, []string{"edge-reload", root, "0", "1"}, nil); e == nil || out != "" {
		t.Fatal("refused reload not reported", out, e)
	}
	if e = state.Close(); e != nil {
		t.Fatal(e)
	}
	state = m794Open(t, root)
	assertServing := func() {
		t.Helper()
		g, e := EdgeGeneration(root)
		if e != nil || g != generation {
			t.Fatal("last serving generation lost")
		}
		if !bytes.Equal(original, m794Read(t, filepath.Join(root, "sites-enabled/serving.paxeer.network.conf"))) {
			t.Fatal("last serving route lost")
		}
		if string(m794Read(t, filepath.Join(root, "edge/rpc-pool"))) != "node-a.example.org\n" {
			t.Fatal("pool rollback lost")
		}
		if _, e = os.Lstat(filepath.Join(root, "sites-enabled/unacknowledged.paxeer.network.conf")); !os.IsNotExist(e) {
			t.Fatal("unacknowledged route enabled")
		}
		if string(m794Read(t, filepath.Join(root, "sites-available/operator.conf"))) != "# keep me\n" {
			t.Fatal("unrelated file changed")
		}
	}
	assertServing()
	if e = state.Begin(); e != nil {
		t.Fatal(e)
	}
	m794Stage(t, root, "unacknowledged.paxeer.network")
	if e = state.Close(); e != nil {
		t.Fatal(e)
	}
	state = m794Open(t, root)
	defer state.Close()
	assertServing()
	if e = state.Begin(); e != nil {
		t.Fatal(e)
	}
	m794Stage(t, root, "unacknowledged.paxeer.network")
	if _, e = m794CLI(t, []string{"edge-reload", root, "1", "-1"}, nil); e == nil {
		t.Fatal("validation refusal not reported")
	}
	assertServing()
	if e = state.Begin(); e != nil {
		t.Fatal(e)
	}
	m794Stage(t, root, "acknowledged.paxeer.network")
	if e = state.Reconcile(); e != nil {
		t.Fatal(e)
	}
	if e = state.Finish(true); e != nil {
		t.Fatal(e)
	}
	if e = state.Close(); e != nil {
		t.Fatal(e)
	}
	state = m794Open(t, root)
	if g, e := EdgeGeneration(root); e != nil || g == generation {
		t.Fatal("successful generation did not advance")
	}
	if !strings.Contains(string(m794Read(t, filepath.Join(root, "edge/manifest"))), "acknowledged.paxeer.network") {
		t.Fatal("successful routes not recovered")
	}
	if e = state.Close(); e != nil {
		t.Fatal(e)
	}
	m794Wire(t, "edge.sh", `out="$(nginx -s reload 2>&1)" || reload=$?`, `previous generation restored`, `edge-reload "$nginx_dir" "$validation" "$reload"`, `edge-state rollback`)
}
