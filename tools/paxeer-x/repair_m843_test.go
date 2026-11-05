package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func repairM843Exchange(t *testing.T, request contractRequest, result any) error {
	t.Helper()
	wire, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var response bytes.Buffer
	if err := runCandidateContract(bytes.NewReader(wire), &response); err != nil {
		if response.Len() != 0 {
			t.Fatalf("refusal emitted a success response: %s", response.Bytes())
		}
		return err
	}
	if err := json.Unmarshal(response.Bytes(), result); err != nil {
		t.Fatalf("invalid policy response: %v", err)
	}
	return nil
}

func repairM843ReadyService(id string, deps ...string) candidateService {
	if deps == nil {
		deps = []string{}
	}
	encoded, err := json.Marshal(deps)
	if err != nil {
		panic(err)
	}
	revision, digest := strings.Repeat("a", 40), "sha256:"+strings.Repeat("b", 64)
	bindings := map[string]string{}
	for _, key := range []string{"config_ref", "owner_ref", "authority_ref", "membership_ref", "storage_ref", "data_ref",
		"abi_ref", "providers_ref", "foreign_chains_ref", "funded_accounts_ref", "roles_ref", "certificates_ref", "policy_ref"} {
		bindings[key] = "secure://recorded/" + id + "/" + key
	}
	bindings["source_revision"], bindings["image_digest"], bindings["config_digest"] = revision, digest, digest
	return candidateService{
		ID: id, Declaration: declaration{Purpose: "Recorded " + id + " role", Route: "root RPC", Dependencies: encoded},
		Bindings: bindings, DependenciesRef: "secure://recorded/dependencies/" + id,
		Observations: []runtimeObservation{{Identity: id + "/running", State: "running", Readiness: "ready",
			ImageDigest: digest, SourceRevision: revision, ConfigDigest: digest,
			RoleIdentity: "secure://recorded/roles/" + id}},
	}
}

func repairM843Request(t *testing.T) contractRequest {
	t.Helper()
	rows := []candidateService{repairM843ReadyService("agent-mcp", "agentd", "platform-identity"),
		repairM843ReadyService("agentd"), repairM843ReadyService("platform-identity")}
	roster := map[string]declaration{}
	for _, row := range rows {
		roster[row.ID] = row.Declaration
	}
	var resolutions map[string]dependencyResolution
	if err := repairM843Exchange(t, contractRequest{Operation: "resolve", Roster: roster}, &resolutions); err != nil {
		t.Fatal(err)
	}
	for i := range rows {
		rows[i].DependencyIDs = resolutions[rows[i].ID].IDs
	}
	var network map[string]any
	if err := repairM843Exchange(t, contractRequest{Operation: "foundation"}, &network); err != nil {
		t.Fatal(err)
	}
	return contractRequest{Operation: "decisions", Roster: roster, Services: rows, Foundation: network}
}

func repairM843Decisions(t *testing.T, request contractRequest) map[string]string {
	t.Helper()
	var decisions map[string]string
	if err := repairM843Exchange(t, request, &decisions); err != nil {
		t.Fatal(err)
	}
	if len(decisions) != len(request.Services) {
		t.Fatalf("incomplete policy response: %v", decisions)
	}
	return decisions
}

func TestRepairM_843_RPCFoundation(t *testing.T) {
	var maintained struct {
		RPCRecords      []string `json:"rpc_records"`
		CanonicalOrigin string   `json:"canonical_origin"`
	}
	data, err := os.ReadFile("route-catalogue.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &maintained); err != nil {
		t.Fatal(err)
	}
	expected := []string{}
	for i := 1; i <= 14; i++ {
		expected = append(expected, fmt.Sprintf("api%d", i))
	}
	expected = append(expected, "api-mainnet-beta")
	if !reflect.DeepEqual(maintained.RPCRecords, expected) {
		t.Fatalf("maintained roster = %v", maintained.RPCRecords)
	}
	if maintained.CanonicalOrigin != "https://api-mainnet-beta.paxeer.network" {
		t.Fatal("canonical root endpoint changed")
	}
	var network struct {
		RPCRecords     []string `json:"rpc_records"`
		ChainID        int      `json:"chain_id"`
		Nodes          string   `json:"nodes"`
		Validators     string   `json:"validators"`
		OriginalHost   string   `json:"api1_original_host"`
		Authentication string   `json:"wallet_authentication"`
		Placement      string   `json:"wallet_placement"`
		Relocation     bool     `json:"relocation"`
	}
	if err := repairM843Exchange(t, contractRequest{Operation: "foundation"}, &network); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(network.RPCRecords, maintained.RPCRecords) {
		t.Fatalf("producer/consumer roster drift: %v", network.RPCRecords)
	}
	if network.ChainID != 125 || network.Nodes != "preserved" || network.Validators != "preserved" ||
		network.OriginalHost != "preserved" || network.Authentication != "Supabase" ||
		network.Placement != "existing-docker-host" || network.Relocation {
		t.Fatalf("foundation preservation changed: %+v", network)
	}
	for _, records := range [][]string{expected[:14], append(append([]string{}, expected...), "api15"),
		{"api1", "api2", "api3", "api4", "api5", "api6", "api7", "api8", "api9", "api10", "api11", "api12", "api13", "api14", "api15", "api16"}} {
		request := repairM843Request(t)
		request.Foundation["rpc_records"] = records
		var decisions map[string]string
		if err := repairM843Exchange(t, request, &decisions); err == nil || !strings.Contains(err.Error(), "foundation") {
			t.Fatalf("changed RPC roster accepted: %v, %v", records, err)
		}
	}
}

func TestRepairM_843_CanonicalDependencies(t *testing.T) {
	request := repairM843Request(t)
	if got := request.Services[0].DependencyIDs; !reflect.DeepEqual(got, []string{"agentd", "platform-identity"}) {
		t.Fatalf("canonical ids = %v", got)
	}
	if got := repairM843Decisions(t, request)["agent-mcp"]; got != "preserved" {
		t.Fatalf("ready declared dependencies: %s", got)
	}
	indexer := repairM843ReadyService("platform-indexer")
	request.Roster[indexer.ID] = indexer.Declaration
	request.Services = append(request.Services, indexer)
	request.Services[0].Declaration.Dependencies = json.RawMessage(`["indexer", "platform-identity"]`)
	request.Roster["agent-mcp"] = request.Services[0].Declaration
	var resolutions map[string]dependencyResolution
	if err := repairM843Exchange(t, contractRequest{Operation: "resolve", Roster: request.Roster}, &resolutions); err != nil {
		t.Fatal(err)
	}
	request.Services[0].DependencyIDs = resolutions["agent-mcp"].IDs
	if !reflect.DeepEqual(request.Services[0].DependencyIDs, []string{"platform-identity", "platform-indexer"}) {
		t.Fatal(resolutions)
	}
	if got := repairM843Decisions(t, request)["agent-mcp"]; got != "preserved" {
		t.Fatalf("canonical alias was not consumed: %s", got)
	}
	request.Services[0].DependencyIDs = []string{"indexer", "platform-identity"}
	var decisions map[string]string
	if err := repairM843Exchange(t, request, &decisions); err == nil {
		t.Fatal("noncanonical runtime dependency accepted")
	}
}

func TestRepairM_843_MissingDependencies(t *testing.T) {
	for _, ids := range [][]string{nil, {}, {"agentd"}, {"platform-identity"}} {
		request := repairM843Request(t)
		request.Services[0].DependencyIDs = ids
		var decisions map[string]string
		err := repairM843Exchange(t, request, &decisions)
		if err == nil || !strings.Contains(err.Error(), "declared dependency omitted") {
			t.Fatalf("omitted dependency accepted: %v, %v", ids, err)
		}
	}
}

func TestRepairM_843_DependencyReadiness(t *testing.T) {
	for _, field := range []string{"owner_ref", "authority_ref", "membership_ref", "storage_ref", "data_ref"} {
		t.Run(field, func(t *testing.T) {
			request := repairM843Request(t)
			request.Services[1].Bindings[field] = unknown
			if got := repairM843Decisions(t, request)["agent-mcp"]; got != unknown {
				t.Fatalf("unowned dependency admitted: %s", got)
			}
		})
	}
	mutations := map[string]func(*candidateService){
		"stopped":                  func(row *candidateService) { row.Observations[0].State = "stopped" },
		"not-ready":                func(row *candidateService) { row.Observations[0].Readiness = "not-ready" },
		"no-observation":           func(row *candidateService) { row.Observations = nil },
		"missing-running-identity": func(row *candidateService) { row.Observations[0].Identity = "" },
		"unknown-role":             func(row *candidateService) { row.Observations[0].RoleIdentity = unknown },
		"unknown-source":           func(row *candidateService) { row.Observations[0].SourceRevision = unknown },
		"different-image":          func(row *candidateService) { row.Observations[0].ImageDigest = "sha256:" + strings.Repeat("c", 64) },
		"different-config":         func(row *candidateService) { row.Observations[0].ConfigDigest = "sha256:" + strings.Repeat("c", 64) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			request := repairM843Request(t)
			mutate(&request.Services[1])
			if got := repairM843Decisions(t, request)["agent-mcp"]; got != unknown {
				t.Fatalf("unproven dependency admitted: %s", got)
			}
		})
	}
}

func TestRepairM_843_UnresolvedDeclarations(t *testing.T) {
	for _, value := range []string{`["agentd", "unlisted-provider"]`, `"chain JSON-RPC, local node relay"`, `"agentd and unlisted-provider"`} {
		request := repairM843Request(t)
		request.Services[0].Declaration.Dependencies = json.RawMessage(value)
		request.Roster["agent-mcp"] = request.Services[0].Declaration
		var resolutions map[string]dependencyResolution
		if err := repairM843Exchange(t, contractRequest{Operation: "resolve", Roster: request.Roster}, &resolutions); err != nil {
			t.Fatal(err)
		}
		if len(resolutions["agent-mcp"].Unresolved) == 0 {
			t.Fatalf("unresolved requirement discarded: %s", value)
		}
		request.Services[0].DependencyIDs = resolutions["agent-mcp"].IDs
		if got := repairM843Decisions(t, request)["agent-mcp"]; got != unknown {
			t.Fatalf("opaque evidence hid unresolved declaration: %s", got)
		}
	}
	for _, invalid := range []string{`null`, `42`, `{"agentd":true}`, `[""]`, `""`, `["agentd", "agentd"]`} {
		request := repairM843Request(t)
		row := request.Roster["agent-mcp"]
		row.Dependencies = json.RawMessage(invalid)
		request.Roster["agent-mcp"] = row
		var resolutions map[string]dependencyResolution
		if err := repairM843Exchange(t, contractRequest{Operation: "resolve", Roster: request.Roster}, &resolutions); err == nil {
			t.Fatalf("invalid declaration accepted: %s", invalid)
		}
	}
}

func TestRepairM_843_DeploymentDecisions(t *testing.T) {
	cases := map[string]func(*candidateService){
		"preserved": func(*candidateService) {},
		"configure": func(row *candidateService) { row.Observations[0].ConfigDigest = "sha256:" + strings.Repeat("d", 64) },
		"update":    func(row *candidateService) { row.Observations[0].ImageDigest = "sha256:" + strings.Repeat("e", 64) },
		"deploy": func(row *candidateService) {
			row.Observations[0].State = "absent"
			row.Observations[0].Readiness = unknown
		},
		"unknown": func(row *candidateService) { row.Bindings["policy_ref"] = unknown },
	}
	for want, mutate := range cases {
		t.Run(want, func(t *testing.T) {
			request := repairM843Request(t)
			mutate(&request.Services[0])
			if got := repairM843Decisions(t, request)["agent-mcp"]; got != want {
				t.Fatalf("decision = %s, want %s", got, want)
			}
		})
	}
	request := repairM843Request(t)
	request.Services[1].DependencyIDs = []string{"agent-mcp"}
	var decisions map[string]string
	if err := repairM843Exchange(t, request, &decisions); err == nil || !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("dependency cycle not refused: %v", err)
	}
}

func TestRepairM_843_CurrentCatalogue(t *testing.T) {
	data, err := os.ReadFile("../../spec/layerx-merge/spec.kvx")
	if err != nil {
		t.Fatal(err)
	}
	// The committed catalogue is the peer fixture, rather than a live service.
	roster := map[string]declaration{}
	name := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "[") {
			name = ""
			if strings.HasPrefix(line, "[service.") && strings.HasSuffix(line, "]") {
				name = strings.TrimSuffix(strings.TrimPrefix(line, "[service."), "]")
				roster[name] = declaration{}
			}
			continue
		}
		if name == "" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		row := roster[name]
		switch strings.TrimSpace(key) {
		case "purpose":
			err = json.Unmarshal([]byte(strings.TrimSpace(value)), &row.Purpose)
		case "route":
			err = json.Unmarshal([]byte(strings.TrimSpace(value)), &row.Route)
		case "dependencies":
			row.Dependencies = json.RawMessage(strings.TrimSpace(value))
		}
		if err != nil {
			t.Fatal(err)
		}
		roster[name] = row
	}
	if len(roster) != 40 {
		t.Fatalf("committed catalogue fixture incomplete: %d", len(roster))
	}
	var resolutions map[string]dependencyResolution
	if err := repairM843Exchange(t, contractRequest{Operation: "resolve", Roster: roster}, &resolutions); err != nil {
		t.Fatal(err)
	}
	request := repairM843Request(t)
	request.Roster, request.Services = roster, nil
	for name, declared := range roster {
		row := repairM843ReadyService(name)
		row.Declaration = declared
		row.DependencyIDs = resolutions[name].IDs
		for field := range row.Bindings {
			row.Bindings[field] = unknown
		}
		row.Observations = nil
		request.Services = append(request.Services, row)
	}
	decisions := repairM843Decisions(t, request)
	for name, action := range decisions {
		if action != unknown {
			t.Fatalf("unobserved current service %s got %s", name, action)
		}
	}
	if !reflect.DeepEqual(resolutions["agent-mcp"].IDs, []string{"agentd"}) {
		t.Fatal("real agent dependency was not resolved")
	}
	if !reflect.DeepEqual(resolutions["human-service"].IDs, []string{"embedded-wallet-gateway", "gas-station", "platform-indexer"}) {
		t.Fatalf("real catalogue alias did not resolve: %+v", resolutions["human-service"])
	}
	if len(resolutions["sequencer"].Unresolved) != 2 {
		t.Fatal("non-catalogue dependencies lost")
	}
}

func TestRepairM_843_TransportRefusals(t *testing.T) {
	for _, input := range []string{`{`, `{"operation":"unknown"}`, `{"operation":"foundation","extra":true}`,
		`{"operation":"foundation"} {"operation":"foundation"}`, `{"operation":"decisions"}`} {
		var response bytes.Buffer
		if err := runCandidateContract(strings.NewReader(input), &response); err == nil || response.Len() != 0 {
			t.Fatalf("unsafe transport input accepted: %s, %v", input, err)
		}
	}
}

func TestRepairM_843_LNIGuidance(t *testing.T) {
	data, err := os.ReadFile("retired_callers.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalogue struct {
		Components map[string]struct {
			Symbols      []string `json:"symbols"`
			DeletedTrees []string `json:"deleted_trees"`
			Replacement  string   `json:"replacement"`
		} `json:"components"`
	}
	if err := json.Unmarshal(data, &catalogue); err != nil {
		t.Fatal(err)
	}
	lni, exists := catalogue.Components["lni"]
	if !exists {
		t.Fatal("LNI retirement record removed")
	}
	symbols := []string{"lni", "LNI", "layerx_lni", "LniArchiveSource", "LniFailure", "lxp_daemon_lni", "lni_"}
	trees := []string{"services/agent/crates/layerx-client/src/lni", "services/agent/include/layerx_lni_abi.h", "cmd/layerxd/lxp_daemon_lni.c", "cmd/layerxd"}
	if !reflect.DeepEqual(lni.Symbols, symbols) || !reflect.DeepEqual(lni.DeletedTrees, trees) {
		t.Fatal("historical retirement scan identities changed")
	}
	for _, required := range []string{"eth_sendRawTransaction", "directly", "public node root JSON-RPC", "chain ID 125", "eth_call", "gRPC", "committed block height", "Block commitment is final"} {
		if !strings.Contains(lni.Replacement, required) {
			t.Fatalf("replacement lacks %q: %s", required, lni.Replacement)
		}
	}
	if strings.Contains(strings.ToLower(lni.Replacement), "router") {
		t.Fatal("replacement still directs callers through a router")
	}
}
