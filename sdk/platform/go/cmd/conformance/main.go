package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	layerx "github.com/Sidiora-Labs/LayerX-Network/sdk/platform/go"
)

type operationDescriptor struct {
	Plane       string `json:"plane"`
	Name        string `json:"name"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Request     string `json:"request"`
	Response    string `json:"response"`
	Idempotency bool   `json:"idempotency"`
	Bodyless    bool   `json:"bodyless"`
}

type operations map[string][]operationDescriptor

func (catalog *operations) UnmarshalJSON(encoded []byte) error {
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	opening, err := decoder.Token()
	if err != nil {
		return err
	}
	if opening != json.Delim('{') {
		return fmt.Errorf("operations must be keyed by contract ID")
	}
	result := make(operations)
	validID := regexp.MustCompile(`^O([1-9]|[1-7][0-9])$`)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		id, ok := token.(string)
		if !ok || !validID.MatchString(id) {
			return fmt.Errorf("invalid operation ID %v", token)
		}
		if _, exists := result[id]; exists {
			return fmt.Errorf("duplicate operation ID %s", id)
		}
		var descriptors []operationDescriptor
		if err := decoder.Decode(&descriptors); err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
		if len(descriptors) == 0 {
			return fmt.Errorf("%s has no SDK operations", id)
		}
		result[id] = descriptors
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("unexpected data after operations: %v", err)
	}
	if len(result) == 0 {
		return fmt.Errorf("operation catalogue is empty")
	}
	*catalog = result
	return nil
}

type requestVector struct {
	Method  string                     `json:"method"`
	Path    string                     `json:"path"`
	Headers map[string]string          `json:"headers"`
	Body    map[string]json.RawMessage `json:"body"`
}

type responseVector struct {
	Status int `json:"status"`
	Body   struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code  layerx.HumanErrorCode `json:"code"`
			Retry string                `json:"retry"`
		} `json:"error"`
		Trace string `json:"trace"`
	} `json:"body"`
}

func main() {
	repo := flag.String("repo", "../../..", "LayerX repository root")
	flag.Parse()
	ids, err := verifyOperationParity(*repo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "layerx-go-conformance:", err)
		os.Exit(1)
	}
	if err := verifyHumanVectors(*repo); err != nil {
		fmt.Fprintln(os.Stderr, "layerx-go-conformance:", err)
		os.Exit(1)
	}
	metadata := layerx.PlatformSDKGo()
	fmt.Printf("agent_operations=%d human_operations=%d human_golden_triplets=%d operation_ids=%s\n", metadata.AgentOperations, metadata.HumanOperations, metadata.HumanOperations, strings.Join(ids, ","))
}

func verifyOperationParity(repo string) ([]string, error) {
	catalog, err := readJSON[struct {
		Schema     int        `json:"schema"`
		Operations operations `json:"operations"`
	}](filepath.Join(repo, "sdk", "platform", "conformance", "operations.json"))
	if err != nil {
		return nil, err
	}
	if catalog.Schema != 1 || len(catalog.Operations) == 0 {
		return nil, fmt.Errorf("invalid operation catalogue schema or missing operations")
	}
	seen := make(map[string]string)
	ids := make([]string, 0, len(catalog.Operations))
	for id := range catalog.Operations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		for _, entry := range catalog.Operations[id] {
			key := entry.Plane + ":" + entry.Name
			if previous, exists := seen[key]; exists {
				return nil, fmt.Errorf("%s: operation %s already belongs to %s", id, key, previous)
			}
			seen[key] = id
			switch entry.Plane {
			case "agent":
				operation := layerx.AgentOperation(entry.Name)
				if !operation.Valid() || operation.RequiresIdempotency() != entry.Idempotency {
					return nil, fmt.Errorf("%s: agent SDK diverges for %s", id, entry.Name)
				}
			case "human":
				operation := layerx.HumanOperation(entry.Name)
				metadata, ok := operation.Metadata()
				if !ok || metadata.Method != entry.Method || metadata.Path != entry.Path || metadata.Request != entry.Request || metadata.Response != entry.Response || operation.RequiresIdempotency() != entry.Idempotency || entry.Bodyless != (metadata.Request == "Empty") {
					return nil, fmt.Errorf("%s: human SDK diverges for %s", id, entry.Name)
				}
			default:
				return nil, fmt.Errorf("%s: unknown SDK plane %q", id, entry.Plane)
			}
		}
	}
	for _, operation := range layerx.AllAgentOperations() {
		if _, exists := seen["agent:"+string(operation)]; !exists {
			return nil, fmt.Errorf("agent SDK operation %s has no contract ID", operation)
		}
	}
	for _, operation := range layerx.AllHumanOperations() {
		if _, exists := seen["human:"+string(operation)]; !exists {
			return nil, fmt.Errorf("human SDK operation %s has no contract ID", operation)
		}
	}
	return ids, nil
}

func verifyHumanVectors(repo string) error {
	goldenRoot := filepath.Join(repo, "services", "human", "schema", "human-api", "golden")
	for _, operation := range layerx.AllHumanOperations() {
		metadata, ok := operation.Metadata()
		if !ok {
			return fmt.Errorf("generated metadata missing for %s", operation)
		}
		prefix := filepath.Join(goldenRoot, string(operation))
		request, err := readJSON[requestVector](prefix + ".request.json")
		if err != nil {
			return err
		}
		if request.Method != metadata.Method || !pathMatches(metadata.Path, request.Path) {
			return fmt.Errorf("request vector for %s diverges from generated method/path", operation)
		}
		if operation.RequiresIdempotency() && request.Headers["Idempotency-Key"] == "" {
			return fmt.Errorf("request vector for %s omits Idempotency-Key", operation)
		}
		for name, value := range request.Body {
			if err := verifyMoneyEncoding(name, value); err != nil {
				return fmt.Errorf("request vector for %s: %w", operation, err)
			}
		}
		success, err := readJSON[responseVector](prefix + ".response.json")
		if err != nil {
			return err
		}
		if success.Status < 200 || success.Status >= 300 || !success.Body.OK || success.Body.Trace == "" || len(success.Body.Result) == 0 || success.Body.Error != nil {
			return fmt.Errorf("success vector for %s is not a typed success envelope", operation)
		}
		if err := verifyMoneyEncoding("result", success.Body.Result); err != nil {
			return fmt.Errorf("success vector for %s: %w", operation, err)
		}
		failure, err := readJSON[responseVector](prefix + ".failure.json")
		if err != nil {
			return err
		}
		if failure.Status >= 200 && failure.Status < 300 || failure.Body.OK || failure.Body.Trace == "" || failure.Body.Error == nil || !failure.Body.Error.Code.Valid() {
			return fmt.Errorf("failure vector for %s is not a typed failure envelope", operation)
		}
		switch failure.Body.Error.Retry {
		case "retriable", "retriable-after", "structural", "final":
		default:
			return fmt.Errorf("failure vector for %s has unknown retry class", operation)
		}
	}
	return nil
}

func verifyMoneyEncoding(name string, encoded json.RawMessage) error {
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	return walkMoney(name, value)
}

func walkMoney(name string, value any) error {
	switch typed := value.(type) {
	case map[string]any:
		for childName, child := range typed {
			if err := walkMoney(childName, child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := walkMoney(name, child); err != nil {
				return err
			}
		}
	case json.Number:
		if name == "amount" || name == "amounts" {
			return fmt.Errorf("%s is encoded as a JSON number", name)
		}
	}
	return nil
}

func readJSON[T any](path string) (T, error) {
	var result T
	encoded, err := os.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(encoded, &result); err != nil {
		return result, fmt.Errorf("decode %s: %w", path, err)
	}
	return result, nil
}

func pathMatches(template string, actual string) bool {
	templateParts := strings.Split(strings.Trim(template, "/"), "/")
	actualParts := strings.Split(strings.Trim(actual, "/"), "/")
	if len(templateParts) != len(actualParts) {
		return false
	}
	for index, expected := range templateParts {
		if strings.HasPrefix(expected, "{") && strings.HasSuffix(expected, "}") {
			if actualParts[index] == "" {
				return false
			}
			continue
		}
		if expected != actualParts[index] {
			return false
		}
	}
	return true
}
