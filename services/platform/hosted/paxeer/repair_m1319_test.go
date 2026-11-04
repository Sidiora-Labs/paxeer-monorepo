package paxeer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func repair1319Read(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func repair1319Absent(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("retired entrypoint %s remains: %v", path, err)
		}
	}
}

func repair1319Profile(t *testing.T) DeploymentProfile {
	t.Helper()
	d, err := OpenDeployment("deployment-profile.json")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	p, err := d.Profile()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Exercise the actual CLI handler through an OS pipe into the YAML consumer.
// Neither the profile reader nor the maintained manifest producer is replaced.
func repair1319Documents(t *testing.T) []map[string]any {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	done := make(chan error, 1)
	go func() {
		err := RunDeployment([]string{"manifest", "deployment-profile.json"}, writer)
		closeErr := writer.Close()
		if err == nil {
			err = closeErr
		}
		done <- err
	}()
	var documents []map[string]any
	decoder := yaml.NewDecoder(reader)
	for {
		var document map[string]any
		err := decoder.Decode(&document)
		if err == io.EOF {
			break
		}
		if err != nil {
			reader.Close()
			t.Fatal(err)
		}
		documents = append(documents, document)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(documents) == 0 {
		t.Fatal("native handoff produced no Kubernetes resources")
	}
	return documents
}

func repair1319Object(t *testing.T, value any) map[string]any {
	t.Helper()
	result, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected object, got %T", value)
	}
	return result
}

func repair1319Items(t *testing.T, value any) []any {
	t.Helper()
	result, ok := value.([]any)
	if !ok {
		t.Fatalf("expected list, got %T", value)
	}
	return result
}

func repair1319Resource(t *testing.T, documents []map[string]any, kind, name string) map[string]any {
	t.Helper()
	for _, d := range documents {
		metadata := repair1319Object(t, d["metadata"])
		if d["kind"] == kind && metadata["name"] == name {
			return d
		}
	}
	t.Fatalf("missing maintained %s %s", kind, name)
	return nil
}

func repair1319Pod(t *testing.T, documents []map[string]any) map[string]any {
	t.Helper()
	node := repair1319Resource(t, documents, "StatefulSet", "layerx-node")
	spec := repair1319Object(t, node["spec"])
	template := repair1319Object(t, spec["template"])
	return repair1319Object(t, template["spec"])
}

func repair1319Container(t *testing.T, pod map[string]any, name string) map[string]any {
	t.Helper()
	for _, row := range repair1319Items(t, pod["containers"]) {
		container := repair1319Object(t, row)
		if container["name"] == name {
			return container
		}
	}
	t.Fatalf("missing maintained container %s", name)
	return nil
}

func repair1319Provisioning(t *testing.T, pod map[string]any) (map[string]any, map[string]string) {
	t.Helper()
	// The inline Python dictionary is also a YAML flow mapping. Decode only
	// the paths assignment, rather than matching filenames anywhere in a script.
	assignment := regexp.MustCompile(`(?m)^\s*paths = (\{[^}]+\})`)
	var provisioner map[string]any
	var paths map[string]string
	for _, row := range repair1319Items(t, pod["initContainers"]) {
		container := repair1319Object(t, row)
		for _, value := range repair1319Items(t, container["args"]) {
			script, ok := value.(string)
			if !ok {
				t.Fatal("non-string native provisioning argument")
			}
			for _, match := range assignment.FindAllStringSubmatch(script, -1) {
				if provisioner != nil {
					t.Fatal("multiple native provisioning paths maps")
				}
				if err := yaml.Unmarshal([]byte(match[1]), &paths); err != nil {
					t.Fatalf("invalid native provisioning paths: %v", err)
				}
				provisioner = container
			}
		}
	}
	if provisioner == nil || len(paths) == 0 {
		t.Fatal("native provisioning paths map missing")
	}
	return provisioner, paths
}

func TestRepairM_1319_RelayWorkspaceRetirement(t *testing.T) {
	documents := repair1319Documents(t)
	count := 0
	for _, document := range documents {
		if document["kind"] == "StatefulSet" {
			count++
		}
		raw, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte("layerx-paxeer-boundary")) {
			t.Fatal("relay binary is still deployed")
		}
	}
	if count != 1 {
		t.Fatalf("wanted one maintained chain, got %d StatefulSets", count)
	}
	pod := repair1319Pod(t, documents)
	if containers := repair1319Items(t, pod["containers"]); len(containers) != 1 {
		t.Fatalf("wanted one native kernel container, got %d", len(containers))
	}
	node := repair1319Container(t, pod, "paxd")
	var configuration struct {
		Image string `json:"image"`
	}
	if err := json.Unmarshal(repair1319Read(t, "../node/tests/fixtures/repair_22_31_355/native_configuration.json"), &configuration); err != nil {
		t.Fatal(err)
	}
	if configuration.Image == "" || node["image"] != configuration.Image {
		t.Fatalf("node fixture image %q differs from maintained image %v", configuration.Image, node["image"])
	}
	provisioner, _ := repair1319Provisioning(t, pod)
	if provisioner["image"] != node["image"] {
		t.Fatal("native provisioning and node use different images")
	}
}

func TestRepairM_1319_CustodyGenesisRetirement(t *testing.T) {
	repair467Restored(t, "custody-genesis.py")
	p := repair1319Profile(t)
	if p.Node.Bootstrap != "../node/bootstrap.sh" {
		t.Fatalf("wrong genesis owner: %s", p.Node.Bootstrap)
	}
	bootstrap := repair1319Read(t, p.Node.Bootstrap)
	if !bytes.Contains(bootstrap, []byte("native_runtime.py\" bootstrap")) {
		t.Fatal("bootstrap no longer uses maintained native initializer")
	}
	wrapper := repair1319Read(t, "init-chain.sh")
	if !bytes.Contains(wrapper, []byte(`exec "$SCRIPT_DIR/../node/bootstrap.sh" "$@"`)) {
		t.Fatal("compatibility entrypoint must hand off to native bootstrap")
	}
	for _, retired := range []string{"layerxcustody", "deposit_root_authority", "sequencer_authorizations", "custody-genesis.py"} {
		if bytes.Contains(wrapper, []byte(retired)) {
			t.Fatalf("retired genesis mutation remains: %s", retired)
		}
	}
	pod := repair1319Pod(t, repair1319Documents(t))
	_, paths := repair1319Provisioning(t, pod)
	for _, input := range []string{"genesis.json", "config.toml", "app.toml", "priv_validator_key.json", "receipt.key"} {
		if paths[input] == "" {
			t.Fatalf("native provisioning paths omit %s", input)
		}
	}
}

func TestRepairM_1319_RelayDeployerRetirement(t *testing.T) {
	repair467Restored(t, "deploy-contracts.sh")
	for _, phase := range []string{"deploy", "permissions", "activate", "bond", "finalize", "bootstrap", "authority-schedule", "authority-execute", "status", "check-profile"} {
		var output bytes.Buffer
		err := RunDeployment([]string{phase, "this-profile-does-not-exist"}, &output)
		if err == nil || !strings.Contains(err.Error(), "historical deployment phases are retired") || output.Len() != 0 {
			t.Fatalf("legacy phase %s was not refused before opening inputs: %v, %q", phase, err, output.String())
		}
	}
	var manifest bytes.Buffer
	if err := RunDeployment([]string{"manifest", "deployment-profile.json"}, &manifest); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(manifest.Bytes(), repair1319Read(t, "../node/deployment.yaml")) {
		t.Fatal("renderer diverged from maintained node deployment")
	}
}

func TestRepairM_1319_DeploymentKeyArgv(t *testing.T) {
	repair467Restored(t, "deploy-contracts.sh", "evm.py")
	secret := strings.Repeat("ac", 32)
	for _, args := range [][]string{
		{"manifest", "deployment-profile.json", "--private-key", secret},
		{"deploy", "--private-key", secret},
		{"send", "--key-file", "protected.key"},
		{"wallet", "address", "--private-key", secret},
	} {
		var output bytes.Buffer
		err := RunDeployment(args, &output)
		if err == nil || output.Len() != 0 || strings.Contains(err.Error(), secret) {
			t.Fatalf("signing request accepted or key exposed: %v", err)
		}
	}
	pod := repair1319Pod(t, repair1319Documents(t))
	node := repair1319Container(t, pod, "paxd")
	argv := append([]any{}, repair1319Items(t, node["command"])...)
	if args, exists := node["args"]; exists {
		argv = append(argv, repair1319Items(t, args)...)
	}
	rawKey := regexp.MustCompile(`(?i)(?:0x)?[0-9a-f]{64}|[A-Za-z0-9+/]{43}=|[A-Za-z0-9+/]{86}==|-----BEGIN [A-Z ]*PRIVATE KEY-----`)
	for _, value := range argv {
		argument, ok := value.(string)
		if !ok {
			t.Fatal("non-string native node argument")
		}
		for _, flag := range []string{"--private-key", "--private_key", "--priv-key", "--secret", "--seed", "--mnemonic", "--receipt-key", "--sequencer.receipt-key"} {
			if strings.Contains(strings.ToLower(argument), flag) {
				t.Fatal("native node command exposes a signing-key argument")
			}
		}
		if rawKey.MatchString(argument) {
			t.Fatal("native node command exposes raw signing material")
		}
	}
	_, paths := repair1319Provisioning(t, pod)
	if paths["receipt.key"] != "config/receipt.key" {
		t.Fatalf("receipt key is not provisioned in the native config: %q", paths["receipt.key"])
	}
}

func TestRepairM_1319_DeploymentPhaseRecovery(t *testing.T) {
	// The native handoff remains read-only beside the restored compatibility runner.
	// Reopen the real handoff beside recorded chain effects: no phase or legacy
	// record may be inferred or published on either side of this restart.
	root := t.TempDir()
	directory := filepath.Join(root, "paxeer")
	node := filepath.Join(root, "node")
	for _, path := range []string{directory, node} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	original := repair1319Read(t, "../node/deployment.yaml")
	if err := os.WriteFile(filepath.Join(node, "deployment.yaml"), original, 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "deployment-profile.json")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteProfile(file, repair1319Profile(t)); err != nil {
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	transcript := repair1319Read(t, "../node/tests/fixtures/repair_22_31_355/native_configuration.json")
	var capture struct {
		Funding struct {
			TransactionID string `json:"transaction_id"`
		} `json:"funding"`
	}
	if err := json.Unmarshal(transcript, &capture); err != nil {
		t.Fatal(err)
	}
	if capture.Funding.TransactionID != "0xcfc62b10e2cba44c2809dfcb89821ed9111bb92bc689afbe6330ef88fdfbe293" {
		t.Fatal("recorded broadcast evidence missing")
	}
	broadcast := filepath.Join(directory, "broadcast.json")
	if err := os.WriteFile(broadcast, transcript, 0600); err != nil {
		t.Fatal(err)
	}
	d, err := OpenDeployment(path)
	if err != nil {
		t.Fatal(err)
	}
	var before bytes.Buffer
	if err := d.WriteManifest(&before); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Profile(); err == nil {
		t.Fatal("closed deployment remained usable")
	}
	if err := d.WriteManifest(io.Discard); err == nil {
		t.Fatal("closed deployment rendered configuration")
	}
	reopened, err := OpenDeployment(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var after bytes.Buffer
	if err := reopened.WriteManifest(&after); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before.Bytes(), after.Bytes()) || !bytes.Equal(after.Bytes(), original) {
		t.Fatal("restart changed the native deployment")
	}
	if !bytes.Equal(repair1319Read(t, broadcast), transcript) {
		t.Fatal("restart rewrote recorded chain effects")
	}
	repair1319Absent(t, filepath.Join(directory, "deployment.json"), filepath.Join(directory, "checkpoint-settlement.json"))
	if legacy, err := OpenDeployment(broadcast); err == nil {
		legacy.Close()
		t.Fatal("recorded transaction was reinterpreted as a deployment phase")
	}
	var output bytes.Buffer
	if err := RunDeployment([]string{"bootstrap", path}, &output); err == nil || output.Len() != 0 {
		t.Fatal("restart exposed a rebroadcast path")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("unexpected restart publications: %v", entries)
	}
}

func TestRepairM_1319_SettlementInputRetirement(t *testing.T) {
	repair467Restored(t, "deployment-input.beta.json")
	p := repair1319Profile(t)
	var encoded bytes.Buffer
	if err := WriteProfile(&encoded, p); err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(encoded.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if len(document) != 3 {
		t.Fatalf("native inputs include more than schema, node and tenant: %v", document)
	}
	for _, field := range []string{"checkpoint_threshold_numerator", "checkpoint_threshold_denominator", "checkpoint_maximum_age", "challenge_window", "unbonding_delay", "timelock_delay", "timelock_profile", "phases", "addresses", "guarantors"} {
		t.Run(field, func(t *testing.T) {
			var candidate map[string]any
			if err := json.Unmarshal(encoded.Bytes(), &candidate); err != nil {
				t.Fatal(err)
			}
			candidate[field] = 1
			raw, err := json.Marshal(candidate)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "legacy-profile.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if d, err := OpenDeployment(path); err == nil {
				d.Close()
				t.Fatalf("retired settlement input %s accepted", field)
			}
		})
	}
}

func TestRepairM_1319_NativeDeployment(t *testing.T) {
	repair467Restored(t, "deployment.yaml")
	documents := repair1319Documents(t)
	service := repair1319Resource(t, documents, "Service", "layerx-native-chain")
	spec := repair1319Object(t, service["spec"])
	selector := repair1319Object(t, spec["selector"])
	statefulSet := repair1319Resource(t, documents, "StatefulSet", "layerx-node")
	nodeSpec := repair1319Object(t, statefulSet["spec"])
	template := repair1319Object(t, nodeSpec["template"])
	metadata := repair1319Object(t, template["metadata"])
	labels := repair1319Object(t, metadata["labels"])
	if len(selector) == 0 {
		t.Fatal("native service has no node selector")
	}
	for name, value := range selector {
		if labels[name] != value {
			t.Fatalf("native service selector %s does not match node labels", name)
		}
	}
	if selector["app"] != repair1319Object(t, statefulSet["metadata"])["name"] {
		t.Fatalf("RPC routes to wrong node: %v", selector)
	}
	pod := repair1319Pod(t, documents)
	native := repair1319Container(t, pod, "paxd")
	containerPorts := make(map[string]int)
	for _, raw := range repair1319Items(t, native["ports"]) {
		port := repair1319Object(t, raw)
		name, named := port["name"].(string)
		number, numbered := port["containerPort"].(int)
		if !named || name == "" || !numbered || number <= 0 || number > 65535 || containerPorts[name] != 0 {
			t.Fatalf("invalid or duplicate native container port: %v", port)
		}
		containerPorts[name] = number
	}
	ports := repair1319Items(t, spec["ports"])
	expected := make(map[string]int)
	for _, name := range []string{"rpc", "consensus", "health"} {
		if containerPorts[name] == 0 {
			t.Fatalf("native container is missing %s port", name)
		}
		expected[name] = containerPorts[name]
	}
	for _, raw := range ports {
		port := repair1319Object(t, raw)
		name, ok := port["name"].(string)
		if !ok {
			t.Fatal("unnamed native port")
		}
		want, ok := expected[name]
		if !ok || port["port"] != want || port["targetPort"] != name {
			t.Fatalf("unexpected native service port: %v", port)
		}
		delete(expected, name)
	}
	if len(expected) != 0 {
		t.Fatalf("native service is missing ports: %v", expected)
	}
	for _, row := range repair1319Items(t, pod["containers"]) {
		container := repair1319Object(t, row)
		if container["name"] == "boundary" {
			t.Fatal("retired relay sidecar remains")
		}
	}
	raw, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}
	for _, retired := range []string{"paxeer-boundary-tls", "custody-genesis", "init-chain.sh", "layerx-paxeer-boundary"} {
		if bytes.Contains(raw, []byte(retired)) {
			t.Fatalf("retired deployment wiring remains: %s", retired)
		}
	}
	readiness := repair1319Object(t, native["readinessProbe"])
	health := repair1319Object(t, readiness["httpGet"])
	startup := repair1319Object(t, native["startupProbe"])
	if health["path"] != "/readyz" || health["port"] != "health" || !reflect.DeepEqual(health, startup["httpGet"]) {
		t.Fatal("native committed health check is missing")
	}
	var profile bytes.Buffer
	if err := RunDeployment([]string{"profile", "deployment-profile.json"}, &profile); err != nil {
		t.Fatal(err)
	}
	var delivered DeploymentProfile
	if err := json.NewDecoder(&profile).Decode(&delivered); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(delivered, repair1319Profile(t)) {
		t.Fatal("handoff changed native or tenant inputs")
	}
	for name, endpoint := range map[string]string{"rpc": delivered.Node.RPC, "consensus": delivered.Node.ConsensusRPC} {
		u, err := url.Parse(endpoint)
		if err != nil || u.Port() != fmt.Sprint(containerPorts[name]) {
			t.Fatalf("profile %s endpoint disagrees with native container port: %s (%v)", name, endpoint, err)
		}
	}
	for _, endpoint := range []string{delivered.Node.RPC, delivered.Node.ConsensusRPC, delivered.Tenant.GatewayRPC} {
		u, err := url.Parse(endpoint)
		if err != nil || u.Path != "/" {
			t.Fatalf("RPC must use the root origin: %s (%v)", endpoint, err)
		}
	}
}

func TestRepairM_1319_EVMToolRetirement(t *testing.T) {
	repair467Restored(t, "evm.py")
	helper := repair1319Read(t, "settlement-domain.py")
	if bytes.Contains(helper, []byte("from evm import")) || !bytes.Contains(helper, []byte("from layerx_sdk.account_derivation import keccak256 as keccak")) {
		t.Fatal("historical public-data helper no longer uses the maintained SDK Keccak")
	}
	for _, action := range []string{"address", "public-key", "calldata", "chain-id", "balance", "call", "send"} {
		var output bytes.Buffer
		if err := RunDeployment([]string{action, "deployment-profile.json"}, &output); err == nil || output.Len() != 0 {
			t.Fatalf("standalone EVM operation %s was admitted", action)
		}
	}
	if err := repair1319Profile(t).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRepairM_1319_ProfileRefusals(t *testing.T) {
	original := repair1319Read(t, "deployment-profile.json")
	for name, raw := range map[string][]byte{
		"old schema":       bytes.Replace(original, []byte("paxeer/native-deployment/1"), []byte("layerx/checkpoint-settlement/1"), 1),
		"relay manifest":   bytes.Replace(original, []byte("../node/deployment.yaml"), []byte("deployment.yaml"), 1),
		"legacy bootstrap": bytes.Replace(original, []byte("../node/bootstrap.sh"), []byte("init-chain.sh"), 1),
		"wrong chain":      bytes.Replace(original, []byte("hyperpax_125-1"), []byte("hyperpax_126-1"), 1),
		"wrong EVM chain":  bytes.Replace(original, []byte(`"evm_chain_id": 125`), []byte(`"evm_chain_id": 126`), 1),
		"relay suffix":     bytes.Replace(original, []byte(":8545/"), []byte(":8545/evm"), 1),
		"invalid port":     bytes.Replace(original, []byte(":8545/"), []byte(":65536/"), 1),
		"zero port":        bytes.Replace(original, []byte(":8545/"), []byte(":0/"), 1),
		"cleartext tenant": bytes.Replace(original, []byte("https://"), []byte("http://"), 1),
		"trailing JSON":    append(append([]byte{}, original...), []byte(`{}`)...),
		"oversize":         append(append([]byte{}, original...), bytes.Repeat([]byte(" "), 16*1024)...),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if d, err := OpenDeployment(path); err == nil {
				d.Close()
				t.Fatal("invalid or legacy profile accepted")
			}
		})
	}
	d, err := OpenDeployment("deployment-profile.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Profile()
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	p.Node.Manifest = "evm.py"
	reopened, err := OpenDeployment("deployment-profile.json")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	recovered, err := reopened.Profile()
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Node.Manifest != "../node/deployment.yaml" {
		t.Fatal("caller mutation changed recovered handoff")
	}
}
