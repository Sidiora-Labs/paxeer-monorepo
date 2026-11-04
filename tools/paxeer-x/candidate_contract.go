package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

//go:embed route-catalogue.json
var routeCatalogue []byte

const unknown = "unknown"

type declaration struct {
	Purpose      string          `json:"purpose"`
	Route        string          `json:"route"`
	Dependencies json.RawMessage `json:"dependencies"`
}

type runtimeObservation struct {
	Identity       string `json:"identity"`
	State          string `json:"state"`
	ImageDigest    string `json:"image_digest"`
	SourceRevision string `json:"source_revision"`
	ConfigDigest   string `json:"config_digest"`
	Readiness      string `json:"readiness"`
	RoleIdentity   string `json:"role_identity"`
	ImageKind      string `json:"image_kind"`
	EvidenceRef    string `json:"evidence_ref"`
	ExposureRef    string `json:"exposure_ref"`
	StorageRef     string `json:"storage_ref"`
}

type candidateService struct {
	ID              string               `json:"id"`
	Declaration     declaration          `json:"declaration"`
	Bindings        map[string]string    `json:"bindings"`
	Observations    []runtimeObservation `json:"observations"`
	DependencyIDs   []string             `json:"dependency_ids"`
	DependenciesRef string               `json:"dependencies_ref"`
	Action          string               `json:"action"`
	MutationAllowed bool                 `json:"mutation_allowed"`
}

type dependencyResolution struct {
	IDs        []string `json:"ids"`
	Unresolved []string `json:"unresolved"`
}

type contractRequest struct {
	Operation  string                 `json:"operation"`
	Roster     map[string]declaration `json:"roster"`
	Services   []candidateService     `json:"services"`
	Foundation map[string]any         `json:"foundation"`
}

func foundation() (map[string]any, error) {
	var routes struct {
		ChainID              int      `json:"chain_id"`
		RPCRecords           []string `json:"rpc_records"`
		PreserveAPI1         bool     `json:"preserve_api1_original_host"`
		WalletAuthentication string   `json:"wallet_authentication"`
		WalletPlacement      string   `json:"wallet_placement"`
	}
	if err := json.Unmarshal(routeCatalogue, &routes); err != nil {
		return nil, err
	}
	if routes.ChainID != 125 || !routes.PreserveAPI1 || len(routes.RPCRecords) == 0 {
		return nil, errors.New("invalid maintained network foundation")
	}
	return map[string]any{
		"chain_id": routes.ChainID, "nodes": "preserved", "validators": "preserved",
		"rpc_records": routes.RPCRecords, "api1_original_host": "preserved",
		"wallet_authentication": routes.WalletAuthentication,
		"wallet_placement":      routes.WalletPlacement, "relocation": false,
	}, nil
}

func resolveDependencies(value json.RawMessage, roster map[string]declaration) (dependencyResolution, error) {
	result := dependencyResolution{IDs: []string{}, Unresolved: []string{}}
	var names []string
	if err := json.Unmarshal(value, &names); err != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		var prose string
		if err := json.Unmarshal(value, &prose); err != nil || strings.TrimSpace(prose) == "" {
			return result, errors.New("invalid dependency declaration")
		}
		if strings.EqualFold(strings.TrimSpace(prose), "none") {
			return result, nil
		}
		// Prose is not an authoritative list of service identities. Keep it
		// unresolved unless the entire declaration identifies one service.
		names = []string{prose}
	}
	seen := map[string]bool{}
	for _, name := range names {
		token := strings.ToLower(strings.TrimSpace(name))
		if token == "" {
			return result, errors.New("empty dependency declaration")
		}
		canonical := token
		// Only aliases whose service identity is unambiguous are admitted.
		aliases := map[string]string{
			"indexer": "platform-indexer", "identity": "platform-identity",
			"identity service": "platform-identity", "receipt authority": "platform-receipt-authority",
			"core boundary": "platform-core-boundary", "gateway": "platform-gateway",
			"faucet": "platform-faucet", "agent boundary internal receipts": "platform-agent-boundary",
			"kms signing": "human-kms",
		}
		if _, exists := roster[canonical]; !exists {
			if alias, ok := aliases[token]; ok {
				canonical = alias
			}
		}
		if _, exists := roster[canonical]; !exists {
			result.Unresolved = append(result.Unresolved, name)
			continue
		}
		if seen[canonical] {
			return result, errors.New("duplicate declared dependency")
		}
		seen[canonical] = true
		result.IDs = append(result.IDs, canonical)
	}
	sort.Strings(result.IDs)
	return result, nil
}

func resolvedRoster(roster map[string]declaration) (map[string]dependencyResolution, error) {
	if len(roster) == 0 {
		return nil, errors.New("missing service catalogue")
	}
	result := map[string]dependencyResolution{}
	for name, row := range roster {
		resolved, err := resolveDependencies(row.Dependencies, roster)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for _, dep := range resolved.IDs {
			if dep == name {
				return nil, errors.New("self dependency")
			}
		}
		result[name] = resolved
	}
	return result, nil
}

func known(value string) bool { return value != "" && value != unknown }

func dependencyReady(service candidateService) bool {
	for _, field := range []string{"owner_ref", "authority_ref", "membership_ref", "storage_ref", "data_ref"} {
		if !known(service.Bindings[field]) {
			return false
		}
	}
	if len(service.Observations) == 0 {
		return false
	}
	for _, row := range service.Observations {
		if (row.State != "running" && row.State != "started") || row.Readiness != "ready" ||
			!known(row.Identity) || !known(row.RoleIdentity) || !known(row.SourceRevision) ||
			!known(row.ConfigDigest) || !known(row.ImageDigest) {
			return false
		}
		if row.SourceRevision != service.Bindings["source_revision"] ||
			row.ConfigDigest != service.Bindings["config_digest"] ||
			row.ImageDigest != service.Bindings["image_digest"] {
			return false
		}
	}
	return true
}

func deploymentDecision(service candidateService, services map[string]candidateService, resolution dependencyResolution) string {
	if len(resolution.Unresolved) != 0 {
		return unknown
	}
	for _, field := range []string{"source_revision", "image_digest", "config_digest", "config_ref",
		"owner_ref", "authority_ref", "membership_ref", "storage_ref", "data_ref", "abi_ref",
		"providers_ref", "foreign_chains_ref", "funded_accounts_ref", "roles_ref", "certificates_ref", "policy_ref"} {
		if !known(service.Bindings[field]) {
			return unknown
		}
	}
	if len(service.DependencyIDs) == 0 && !known(service.DependenciesRef) {
		return unknown
	}
	for _, name := range service.DependencyIDs {
		dependency, ok := services[name]
		if !ok || !dependencyReady(dependency) {
			return unknown
		}
	}
	if len(service.Observations) == 0 {
		return unknown
	}
	absent := true
	for _, row := range service.Observations {
		if row.State != "absent" {
			absent = false
		}
	}
	if absent {
		return "deploy"
	}
	for _, row := range service.Observations {
		if (row.State != "running" && row.State != "started") || row.Readiness != "ready" ||
			!known(row.RoleIdentity) || !known(row.SourceRevision) || !known(row.ConfigDigest) || !known(row.ImageDigest) {
			return unknown
		}
	}
	for _, row := range service.Observations {
		if row.ImageDigest != service.Bindings["image_digest"] || row.SourceRevision != service.Bindings["source_revision"] {
			return "update"
		}
	}
	for _, row := range service.Observations {
		if row.ConfigDigest != service.Bindings["config_digest"] {
			return "configure"
		}
	}
	return "preserved"
}

func candidateDecisions(roster map[string]declaration, rows []candidateService) (map[string]string, error) {
	resolutions, err := resolvedRoster(roster)
	if err != nil {
		return nil, err
	}
	services := map[string]candidateService{}
	for _, row := range rows {
		declared, exists := roster[row.ID]
		if _, duplicate := services[row.ID]; !exists || duplicate {
			return nil, errors.New("unexpected or duplicate service")
		}
		declaredJSON, _ := json.Marshal(declared)
		actualJSON, _ := json.Marshal(row.Declaration)
		if !bytes.Equal(declaredJSON, actualJSON) {
			return nil, errors.New("catalogue contract changed")
		}
		services[row.ID] = row
		provided := map[string]bool{}
		for _, dep := range row.DependencyIDs {
			if _, exists := roster[dep]; !exists || dep == row.ID || provided[dep] {
				return nil, errors.New("invalid dependency identities")
			}
			provided[dep] = true
		}
		for _, required := range resolutions[row.ID].IDs {
			if !provided[required] {
				return nil, errors.New("declared dependency omitted")
			}
		}
	}
	if len(services) != len(roster) {
		return nil, errors.New("incomplete service catalogue")
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			return errors.New("cyclic operational dependencies")
		}
		if visited[name] {
			return nil
		}
		visiting[name] = true
		for _, dep := range services[name].DependencyIDs {
			if err := visit(dep); err != nil {
				return err
			}
		}
		delete(visiting, name)
		visited[name] = true
		return nil
	}
	for name := range services {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	decisions := map[string]string{}
	for name, service := range services {
		decisions[name] = deploymentDecision(service, services, resolutions[name])
	}
	return decisions, nil
}

// runCandidateContract is the JSON transport used by candidate.py and the Go gate.
func runCandidateContract(input io.Reader, output io.Writer) error {
	decoder := json.NewDecoder(io.LimitReader(input, 4*1024*1024+1))
	decoder.DisallowUnknownFields()
	var request contractRequest
	if err := decoder.Decode(&request); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("trailing contract input")
	}
	var result any
	var err error
	switch request.Operation {
	case "foundation":
		result, err = foundation()
	case "resolve":
		result, err = resolvedRoster(request.Roster)
	case "decisions":
		expected, foundationErr := foundation()
		if foundationErr != nil {
			return foundationErr
		}
		want, _ := json.Marshal(expected)
		actual, marshalErr := json.Marshal(request.Foundation)
		if marshalErr != nil || !bytes.Equal(want, actual) {
			return errors.New("foundation preservation contract changed")
		}
		result, err = candidateDecisions(request.Roster, request.Services)
	default:
		return errors.New("unknown contract operation")
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(result)
}

func main() {
	if err := runCandidateContract(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "candidate contract: refused invalid or unsafe input")
		os.Exit(2)
	}
}
