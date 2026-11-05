package bringup

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func repairM792Read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func repairM792Inspect(t *testing.T, args []string, input []byte) (string, error) {
	t.Helper()
	var output bytes.Buffer
	err := RunInspector(context.Background(), args, bytes.NewReader(input), &output)
	return output.String(), err
}

func repairM792Inventory(t *testing.T) PublicInventory {
	t.Helper()
	data := repairM792Read(t, "../../docs/site/docs/reference/public-rpc.md")
	inventory, err := ReadPublicInventory(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}

func repairM792Bundle(t *testing.T, generation string) (string, IdentityBundle) {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	bundle := IdentityBundle{}
	for _, file := range identityFiles {
		data := repairM792Read(t, filepath.Join("tests/fixtures/repair_m792", generation, file.File))
		bundle[file.File] = data
		if err := os.WriteFile(filepath.Join(directory, file.File), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return directory, bundle
}

func repairM792Railway(t *testing.T, bundle IdentityBundle, list bool) []byte {
	t.Helper()
	values := map[string]string{}
	for _, file := range identityFiles {
		if data, ok := bundle[file.File]; ok {
			values["IDENTITY_TLS_"+file.Suffix] = base64.StdEncoding.EncodeToString(data)
		}
	}
	var value any = values
	if list {
		rows := []map[string]string{}
		for name, data := range values {
			rows = append(rows, map[string]string{"name": name, "value": data})
		}
		value = rows
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRepairM_792_FoundationWrapperRetiredTarget(t *testing.T) {
	// The installed producer and qualification consumer exchange the actual
	// merged plan. Neither native artifacts nor a build tool are prerequisites.
	wire, err := repairM792Inspect(t, []string{"runtime-plan"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := ReadRuntimePlan(strings.NewReader(wire))
	if err != nil {
		t.Fatal(err)
	}
	var producer RuntimePlan
	if err := json.Unmarshal(repairM792Read(t, "runtime-bundle.json"), &producer); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan, producer) || !reflect.DeepEqual(plan.Names, []string{"paxd", "attestor", "bringup-inspect"}) {
		t.Fatalf("producer/consumer plan differs: %+v", plan)
	}
	for _, required := range []string{"Makefile", "chain.mk", "modules", "layerxproof", "src", "include", "programs", "services", "go.mod", "go.sum", "rust-toolchain.toml"} {
		found := false
		for _, path := range plan.GoPaths {
			found = found || path == required
		}
		if !found {
			t.Fatalf("missing merged dependency %s", required)
		}
	}
	wrapper := string(repairM792Read(t, "build-foundation-artifacts.sh"))
	producerSource := string(repairM792Read(t, "tests/foundation-artifacts.py"))
	for _, required := range []string{"--build --bundle merged-runtime", "\"$@\""} {
		if !strings.Contains(wrapper, required) {
			t.Fatalf("wrapper lost explicit build contract: %s", required)
		}
	}
	for _, required := range []string{"tools/bringup/runtime-bundle.json", `manifest.get("version") == PLAN["version"]`, `manifest.get("bundle") == PLAN["bundle"]`, `record.get("sha256") == sha256(path)`, `source_binding(selected_revision, PATHS[name]) == binding`, `output_path(destination)`, `go_metadata(stage / name, revision, tool)`} {
		if !strings.Contains(producerSource, required) {
			t.Fatalf("producer lost source/output proof %s", required)
		}
	}
	for _, retired := range []string{"layerxd", "layerx-genesis-build", "layerx-module-registry", "custodyproof", "checkpoint-settlement"} {
		if strings.Contains(producerSource, retired) {
			t.Fatalf("artifact producer retains %s", retired)
		}
	}
	for _, consumer := range []string{"tests/paxeer-x-fixture-foundation.py"} {
		if !strings.Contains(string(repairM792Read(t, consumer)), "ART.validate(") {
			t.Fatal("real fixture no longer validates producer manifest")
		}
	}
	for _, fault := range []string{"legacy-version", "retired-executable", "retired-source"} {
		t.Run(fault, func(t *testing.T) {
			value := producer
			value.Names = append([]string{}, producer.Names...)
			value.GoPaths = append([]string{}, producer.GoPaths...)
			switch fault {
			case "legacy-version":
				value.Version = 1
			case "retired-executable":
				value.Names = append(value.Names, "layerxd")
			case "retired-source":
				value.GoPaths = append(value.GoPaths, "custodyproof")
			}
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ReadRuntimePlan(bytes.NewReader(raw)); err == nil {
				t.Fatal("retired bundle accepted")
			}
		})
	}
}

func TestRepairM_792_CARetiredAuthorityAndRouter(t *testing.T) {
	wire, err := repairM792Inspect(t, []string{"services", "ca-services.tsv"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ReadServiceRoster(strings.NewReader(wire))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 30 {
		t.Fatalf("want all 30 live identities, got %d", len(rows))
	}
	names := map[string]bool{}
	scanner := strings.Split(string(repairM792Read(t, "railway-names.env")), "\n")
	for _, line := range scanner {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok || names[key] {
			t.Fatalf("invalid names row %q", line)
		}
		names[key] = true
	}
	foundGateway, foundRedis := false, false
	for _, row := range rows {
		key := strings.ToUpper(strings.ReplaceAll(row.Name, "-", "_"))
		if !names[key] {
			t.Fatalf("missing names row %s", row.Name)
		}
		delete(names, key)
		if row.Name == "gateway-client" {
			foundGateway = row.Target == "railway:gateway" && row.Custody == "ENDPOINT_CLIENT" && row.EKU == "clientAuth"
		}
		if row.Name == "gateway-redis" {
			foundRedis = row.Target == "railway:redis-gateway" && row.EKU == "serverAuth"
		}
	}
	if len(names) != 0 || !foundGateway || !foundRedis {
		t.Fatalf("roster/name ownership mismatch: extras=%v gateway=%t redis=%t", names, foundGateway, foundRedis)
	}
	if !strings.Contains(string(repairM792Read(t, "ca.sh")), `cat "$repo_root/tools/bringup/ca-services.tsv"`) {
		t.Fatal("shell issuer bypasses shared roster")
	}
	if strings.Contains(string(repairM792Read(t, "railway-names.env")), "redis-router") {
		t.Fatal("retired Redis SAN retained")
	}
}

func TestRepairM_792_CAExpiredRenewalRecovery(t *testing.T) {
	directory, original := repairM792Bundle(t, "expired")
	leaf, err := parsePEMCertificate(original["cert.pem"])
	if err != nil {
		t.Fatal(err)
	}
	now := leaf.NotAfter.Add(48 * time.Hour)
	custody, err := OpenIdentityCustody(directory)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := custody.Read()
	if err != nil {
		t.Fatal(err)
	}
	var report bytes.Buffer
	if err := BundleReport(bundle, &report); err != nil {
		t.Fatal(err)
	}
	if err := custody.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := custody.Read(); err == nil {
		t.Fatal("closed custody reader remained usable")
	}
	state := InspectBundleReport(&report, original["ca.pem"], now, "layerx-agentd", "serverAuth", "DNS:layerx-agentd,DNS:localhost,IP:127.0.0.1")
	if state.State != "expired" {
		t.Fatalf("same-issuer expired identity misclassified: %+v", state)
	}
	action, err := IdentityAction(state.State, state.Days, 30)
	if err != nil || action != "renew" {
		t.Fatalf("renewal refused: %q %v", action, err)
	}
	// The recorded renewal was emitted by ca.sh sign + derive_cmd under the
	// retained issuer and key; reopen custody only after the handoff is stored.
	_, renewed := repairM792Bundle(t, "renewed")
	if !bytes.Equal(original["key.pem"], renewed["key.pem"]) || !bytes.Equal(original["ca.pem"], renewed["ca.pem"]) {
		t.Fatal("renewal replaced the established key or authority")
	}
	for name, data := range renewed {
		if err := os.WriteFile(filepath.Join(directory, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	reopened, err := OpenIdentityCustody(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	recovered, err := reopened.Read()
	if err != nil {
		t.Fatal(err)
	}
	report.Reset()
	if err := BundleReport(recovered, &report); err != nil {
		t.Fatal(err)
	}
	state = InspectBundleReport(&report, original["ca.pem"], now, "layerx-agentd", "serverAuth", "DNS:layerx-agentd,DNS:localhost,IP:127.0.0.1")
	if state.State != "present" || state.Fingerprint == "" {
		t.Fatalf("renewed identity not recovered: %+v", state)
	}
	action, err = IdentityAction(state.State, state.Days, 30)
	if err != nil || action != "reuse" {
		t.Fatalf("renewed bundle not reusable: %s %v", action, err)
	}
	foreignKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	foreign := &x509.Certificate{SerialNumber: big.NewInt(42), Subject: pkix.Name{CommonName: "Foreign issuer"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), BasicConstraintsValid: true, IsCA: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, foreign, foreign, &foreignKey.PublicKey, foreignKey)
	if err != nil {
		t.Fatal(err)
	}
	wrongCA := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	state = CertificateInventory(original["cert.pem"], wrongCA, now, 0, "layerx-agentd", "serverAuth", "DNS:layerx-agentd,DNS:localhost,IP:127.0.0.1")
	if state.State != "foreign" {
		t.Fatalf("foreign authority not detected: %+v", state)
	}
	if _, err := IdentityAction(state.State, state.Days, 30); err == nil {
		t.Fatal("foreign authority eligible for renewal")
	}
}

func TestRepairM_792_CABundleConsistency(t *testing.T) {
	_, valid := repairM792Bundle(t, "renewed")
	if err := BundleConsistent(valid); err != nil {
		t.Fatalf("real ca.sh output rejected: %v", err)
	}
	_, old := repairM792Bundle(t, "expired")
	for _, fault := range []string{"none", "key.pem", "key.der", "cert.der", "ca.der", "ca.pem", "identity.p12", "password", "missing"} {
		t.Run(fault, func(t *testing.T) {
			directory, bundle := repairM792Bundle(t, "renewed")
			switch fault {
			case "none":
			case "key.pem", "key.der":
				key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				der, err := x509.MarshalPKCS8PrivateKey(key)
				if err != nil {
					t.Fatal(err)
				}
				if fault == "key.pem" {
					bundle[fault] = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
				} else {
					bundle[fault] = der
				}
			case "cert.der":
				bundle[fault] = old[fault]
			case "ca.der", "ca.pem":
				bundle[fault] = bundle[map[string]string{"ca.der": "cert.der", "ca.pem": "cert.pem"}[fault]]
			case "identity.p12":
				bundle[fault] = old[fault]
			case "password":
				bundle[fault] = []byte("wrong-password\n")
			case "missing":
				delete(bundle, "key.der")
				if err := os.Remove(filepath.Join(directory, "key.der")); err != nil {
					t.Fatal(err)
				}
			}
			for name, data := range bundle {
				if err := os.WriteFile(filepath.Join(directory, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			box, err := repairM792Inspect(t, []string{"bundle-local", directory}, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, list := range []bool{false, true} {
				railway, err := repairM792Inspect(t, []string{"bundle-railway", "IDENTITY_TLS"}, repairM792Railway(t, bundle, list))
				if err != nil {
					t.Fatal(err)
				}
				if box != railway {
					t.Fatalf("custody transports disagree: box=%q railway=%q", box, railway)
				}
			}
			expected := "inconsistent"
			if fault == "none" {
				expected = "consistent"
			}
			if fault == "missing" {
				expected = "partial"
			}
			if !strings.HasPrefix(box, "bundle="+expected+"\n") {
				t.Fatalf("fault %s accepted: %s", fault, box)
			}
			if strings.Contains(box, "PRIVATE KEY") || strings.Contains(box, string(valid["password"])) || strings.Contains(box, base64.StdEncoding.EncodeToString(valid["key.der"])) {
				t.Fatal("inventory revealed secret material")
			}
			if fault != "none" {
				state := InspectBundleReport(strings.NewReader(box), valid["ca.pem"], time.Now(), "layerx-agentd", "serverAuth", "DNS:layerx-agentd,DNS:localhost,IP:127.0.0.1")
				if _, err := IdentityAction(state.State, state.Days, 30); err == nil {
					t.Fatalf("%s allowed reuse or silent replacement", fault)
				}
				retained := repairM792Read(t, filepath.Join(directory, "cert.pem"))
				if !bytes.Equal(retained, valid["cert.pem"]) {
					t.Fatal("inventory replaced established identity")
				}
			}
		})
	}
	for _, fault := range []string{"permissions", "symlink", "hardlink"} {
		t.Run(fault, func(t *testing.T) {
			directory, _ := repairM792Bundle(t, "renewed")
			target := filepath.Join(directory, "key.der")
			switch fault {
			case "permissions":
				if err := os.Chmod(target, 0644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Rename(target, target+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target+".saved", target); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				if err := os.Link(target, target+".alias"); err != nil {
					t.Fatal(err)
				}
			}
			output, err := repairM792Inspect(t, []string{"bundle-local", directory}, nil)
			if err == nil || output != "" {
				t.Fatalf("unprotected custody accepted or leaked: %q %v", output, err)
			}
		})
	}
}

func TestRepairM_792_RPCRoster(t *testing.T) {
	inventory := repairM792Inventory(t)
	if len(inventory.Nodes) != 14 || len(inventory.Endpoints()) != 15 || inventory.Canonical != "https://api-mainnet-beta.paxeer.network" {
		t.Fatalf("wrong public roster %+v", inventory)
	}
	wire, err := repairM792Inspect(t, []string{"rpc-names", "../../docs/site/docs/reference/public-rpc.md"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := strings.Fields(wire)
	for n, name := range names {
		if name != fmt.Sprintf("api%d.mainnet-beta.paxeer.network", n+1) {
			t.Fatalf("wrong node %q", name)
		}
	}
	if len(names) != 14 {
		t.Fatalf("wrong CLI roster: %v", names)
	}
	if err := inventory.ValidatePool([]string{inventory.Canonical, inventory.Nodes[13]}); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"https://api15.mainnet-beta.paxeer.network", "https://api16.mainnet-beta.paxeer.network", inventory.Canonical + "/rpc"} {
		if err := inventory.ValidatePool([]string{inventory.Canonical, bad}); err == nil {
			t.Fatalf("retired endpoint admitted: %s", bad)
		}
	}
	source := string(repairM792Read(t, "check-live.sh"))
	if strings.Contains(source, "api{1..16}") || strings.Contains(source, "-le 16") || strings.Contains(source, "rpc_name_count=16") {
		t.Fatal("fleet consumer retains sixteen-node loop")
	}
	if !strings.Contains(source, "RPC_COUNT=%s") || !strings.Contains(source, "canonical chain_id=125") || !strings.Contains(string(repairM792Read(t, "search-front.sh")), "load_rpc_inventory") {
		t.Fatal("fleet consumer bypasses maintained inventory")
	}
}

func TestRepairM_792_GasRPCRoute(t *testing.T) {
	inventory := repairM792Inventory(t)
	config := map[string]any{"chain_id": 125, "token": "0x21f7b20a555199fa73A238B1a91FD0f549068fEe", "endpoints": []string{inventory.Canonical, inventory.Nodes[0], inventory.Nodes[13]}}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := repairM792Inspect(t, []string{"gas-config", "../../docs/site/docs/reference/public-rpc.md"}, raw)
	if err != nil || strings.TrimSpace(endpoint) != inventory.Canonical {
		t.Fatalf("gas config not consumed: %q %v", endpoint, err)
	}
	for _, bad := range []any{[]string{inventory.Canonical + "/rpc", inventory.Nodes[0]}, []string{inventory.Canonical, "https://api16.mainnet-beta.paxeer.network"}, []string{inventory.Canonical, inventory.Canonical}} {
		config["endpoints"] = bad
		raw, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ValidateGasConfig(inventory, bytes.NewReader(raw)); err == nil {
			t.Fatalf("drifted gas pool accepted: %v", bad)
		}
	}
	// Exercise the repaired gas_rpc consumer over real HTTP into geth's real
	// signing, transaction pool, execution and committed receipt path.
	chain := repair143NewChain(t)
	transaction, encoded := repair143Signed(t, chain.key, chain.chainID, 0, chain.recipient)
	params, _ := json.Marshal([]string{fmt.Sprintf("0x%x", encoded)})
	hash, err := repairM792Inspect(t, []string{"rpc", chain.url, "eth_sendRawTransaction", string(params)}, nil)
	if err != nil || strings.TrimSpace(hash) != transaction.Hash().Hex() {
		t.Fatalf("real node submission failed: %s %v", hash, err)
	}
	repair143ObservePayment(t, chain, transaction)
	params, _ = json.Marshal([]string{transaction.Hash().Hex()})
	receipt, err := repairM792Inspect(t, []string{"rpc", chain.url, "eth_getTransactionReceipt", string(params)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var committed struct {
		Status string `json:"status"`
		Hash   string `json:"transactionHash"`
		Block  string `json:"blockHash"`
	}
	if json.Unmarshal([]byte(receipt), &committed) != nil || committed.Status != "0x1" || committed.Hash != transaction.Hash().Hex() || committed.Block == "" {
		t.Fatalf("receipt path incomplete: %s", receipt)
	}
	source := string(repairM792Read(t, "check-live.sh"))
	for _, required := range []string{`gas_rpc_url="${CHECK_LIVE_GAS_RPC:-${node_endpoints[0]}}"`, `bringup_inspect gas-config`, "gas_post /quote", "SponsoredBatch(", "executeSponsored(", "gas_post /submit", "gas_rpc eth_getTransactionReceipt", "gas_transfer_topic"} {
		if !strings.Contains(source, required) {
			t.Fatalf("gas journey lost %s", required)
		}
	}
}

func TestRepairM_792_WalletDeletedSpec(t *testing.T) {
	spec := "../../spec/layerx-merge/spec.kvx"
	actual := repairM792Read(t, spec)
	name, err := WalletName(bytes.NewReader(actual))
	cli, cliErr := repairM792Inspect(t, []string{"wallet-name", spec}, nil)
	if err != nil {
		if !strings.Contains(err.Error(), "wallet_endpoint missing from ["+PublicNamesSection+"]") || cliErr == nil || cli != "" {
			t.Fatalf("missing repository declaration was not explicit: %q %v %v", cli, err, cliErr)
		}
	} else if cliErr != nil || strings.TrimSpace(cli) != name || name == "" {
		t.Fatalf("repository wallet declaration lost: %q %v", cli, cliErr)
	}
	source := string(repairM792Read(t, "check-live.sh"))
	if !strings.Contains(source, "local spec=spec/layerx-merge/spec.kvx") || !strings.Contains(source, `bringup_inspect wallet-name "$repo_root/$spec"`) || strings.Contains(source, "spec/paxeer-x-bringup/spec.kvx") {
		t.Fatal("wallet consumer does not read active repository declaration")
	}
	if !strings.Contains(source, `railway_service wallet-gateway`) || !strings.Contains(source, `CHECK_LIVE_GATEWAY_BASE="$answer"`) {
		t.Fatal("wallet rollout gateway check lost")
	}
}

func TestRepairM_792_RouterRetiredProtocol(t *testing.T) {
	account := "0x1111111111111111111111111111111111111111"
	var mu sync.Mutex
	var calls []string
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			t.Error("node request unreadable")
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		calls = append(calls, "node:"+r.URL.Path+":"+request.Method)
		mu.Unlock()
		if r.URL.Path != "/" || request.Method != "eth_chainId" || r.Header.Get("Authorization") != "" {
			t.Error("hosted method or credential reached node")
			w.WriteHeader(400)
			return
		}
		io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":"0x7d"}`)
	}))
	defer node.Close()
	broken := false
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string   `json:"method"`
			Params []string `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			t.Error("gateway request unreadable")
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		calls = append(calls, "gateway:"+r.URL.Path+":"+request.Method)
		mu.Unlock()
		if r.URL.Path != "/hosted" || r.Header.Get("Authorization") != "Bearer fixture-credential" {
			t.Error("gateway route or authorization changed")
			w.WriteHeader(400)
			return
		}
		var result any
		switch request.Method {
		case "px_getNetwork":
			result = map[string]any{"network_id": "125", "paxeer": map[string]string{"chain_id": "0x7d", "latest_block": "0x10"}, "consensus": map[string]any{"latest_height": "16", "latest_block_hash": "aa", "latest_app_hash": "bb", "catching_up": broken}}
		case "px_getAccount":
			if !reflect.DeepEqual(request.Params, []string{account}) {
				t.Error("account selector lost")
			}
			result = map[string]any{"account": map[string]any{"evm_address": account, "pax_address": nil, "layerx_did": nil, "layerx_account": nil, "bound": false}, "paxeer": map[string]any{"address": account, "balance": "0x0", "nonce": "0x0", "height": 16}}
		default:
			t.Error("retired method reached hosted gateway")
			w.WriteHeader(400)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
	defer gateway.Close()
	inventory := repairM792Inventory(t)
	inventory.Canonical = node.URL
	var output bytes.Buffer
	if err := ProbeRoutes(context.Background(), node.Client(), inventory, gateway.URL+"/hosted", account, "fixture-credential", &output); err != nil {
		t.Fatalf("real routed checker rejected peers: %v %s", err, output.String())
	}
	if !reflect.DeepEqual(calls, []string{"node:/:eth_chainId", "gateway:/hosted:px_getNetwork", "gateway:/hosted:px_getAccount"}) {
		t.Fatalf("retired topology called: %v", calls)
	}
	broken = true
	output.Reset()
	if err := ProbeRoutes(context.Background(), node.Client(), inventory, gateway.URL+"/hosted", account, "fixture-credential", &output); err == nil || !strings.Contains(output.String(), "fail px_getNetwork") {
		t.Fatal("catching-up gateway passed")
	}
	public := repairM792Inventory(t)
	pool := map[string]any{"components": map[string]any{"rpc_pool": map[string]any{"state": "up", "healthy": 2, "endpoints": []map[string]string{{"url": public.Canonical}, {"url": public.Nodes[0]}}}}}
	raw, err := json.Marshal(pool)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateWalletPool(public, bytes.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
	invalid := strings.Replace(string(raw), public.Canonical, gateway.URL, 1)
	if err := ValidateWalletPool(public, strings.NewReader(invalid)); err == nil {
		t.Fatal("wallet pool still accepts hosted router first")
	}
	now := time.Now()
	ready := map[string]any{"readiness_version": 1, "service": "layerx-gateway", "status": "ready", "observed_at_ms": now.UnixMilli(), "valid_until_ms": now.Add(30 * time.Second).UnixMilli(), "backends": map[string]any{"durable_store": map[string]string{"state": "ready"}, "paxeer_chain": map[string]string{"state": "ready"}, "consensus": map[string]string{"state": "ready"}}}
	raw, err = json.Marshal(ready)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateGatewayReadiness(bytes.NewReader(raw), now); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGatewayReadiness(bytes.NewReader(raw), now.Add(31*time.Second)); err == nil {
		t.Fatal("stale gateway readiness passed")
	}
	source := string(repairM792Read(t, "check-live.sh"))
	start := strings.Index(source, "check_router() {")
	end := strings.Index(source[start:], "# check_kernel_boundaries:")
	routeSource := source[start : start+end]
	if strings.Contains(routeSource, "lx_getAccount") || strings.Contains(routeSource, "railway_service router") || strings.Contains(routeSource, "$url/rpc") {
		t.Fatal("retired router consumer retained")
	}
}
