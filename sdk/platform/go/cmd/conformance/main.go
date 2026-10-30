package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
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
	Transport   string `json:"transport"`
	PerLeg      bool   `json:"per_leg"`
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
	if err := runVectors(*repo); err != nil {
		fmt.Fprintln(os.Stderr, "layerx-go-conformance:", err)
		os.Exit(1)
	}
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
	fmt.Printf("agent_operations=%d human_operations=%d human_golden_triplets=%d operation_ids=%s receipt_vector_files=5\n", metadata.AgentOperations, metadata.HumanOperations, metadata.HumanOperations, strings.Join(ids, ","))
}

type receiptVectorCase struct {
	ID             string          `json:"id"`
	RawTransaction string          `json:"rawTransaction"`
	ReceiptJSON    json.RawMessage `json:"receiptJson"`
	SignedReceipt  string          `json:"signedReceipt"`
	ReceiptDigest  string          `json:"receiptDigest"`
	PrevSeqHash    string          `json:"prevSeqHash"`
	PinnedKey      struct {
		KeyID       string `json:"keyId"`
		PublicKey   string `json:"publicKey"`
		FirstHeight uint64 `json:"firstHeight"`
		LastHeight  uint64 `json:"lastHeight"`
	} `json:"pinnedKey"`
	LXPCode  string `json:"lxpCode"`
	Expected struct {
		Rung      string `json:"rung"`
		RefusedAt string `json:"refusedAt"`
		Check     string `json:"check"`
	} `json:"expected"`
}

func (c receiptVectorCase) keys() (layerx.ReceiptKeys, error) {
	var pin layerx.ReceiptKey
	keys := layerx.ReceiptKeys{}
	for _, field := range []struct {
		name, value string
		target      []byte
	}{
		{"keyId", c.PinnedKey.KeyID, pin.KeyID[:]},
		{"publicKey", c.PinnedKey.PublicKey, pin.PublicKey[:]},
		{"prevSeqHash", c.PrevSeqHash, keys.PreviousSequenceHash[:]},
	} {
		decoded, err := vectorHex(field.value)
		if err != nil || len(decoded) != len(field.target) {
			return keys, fmt.Errorf("%s: invalid 32-byte %s", c.ID, field.name)
		}
		copy(field.target, decoded)
	}
	pin.FirstHeight, pin.LastHeight = c.PinnedKey.FirstHeight, c.PinnedKey.LastHeight
	keys.Pins = []layerx.ReceiptKey{pin}
	return keys, nil
}

func vectorHex(value string) ([]byte, error) {
	if !strings.HasPrefix(value, "0x") || len(value) <= 2 {
		return nil, fmt.Errorf("expected nonempty 0x-prefixed hex")
	}
	return hex.DecodeString(value[2:])
}

func verifyVectorReceipt(c receiptVectorCase) (layerx.SequencerReceiptVerification, error) {
	keys, err := c.keys()
	if err != nil {
		return layerx.SequencerReceiptVerification{}, err
	}
	raw, err := vectorHex(c.RawTransaction)
	if err != nil {
		return layerx.SequencerReceiptVerification{}, fmt.Errorf("rawTransaction: %w", err)
	}
	return layerx.VerifySequencerReceipt(c.ReceiptJSON, raw, keys)
}

func matchVectorReceipt(c receiptVectorCase, result layerx.SequencerReceiptVerification, rung string) error {
	if rung == "" || result.Rung != rung || result.Level != rung {
		return fmt.Errorf("expected rung %q, got rung %q and level %q", rung, result.Rung, result.Level)
	}
	if c.SignedReceipt != "0x"+hex.EncodeToString(result.CanonicalBytes) || c.ReceiptDigest != "0x"+hex.EncodeToString(result.ReceiptDigest[:]) {
		return fmt.Errorf("verified receipt bytes or digest differ from vector")
	}
	if c.LXPCode != "" {
		if result.Status != 0 || result.Revert == nil || result.Revert.Code != c.LXPCode {
			return fmt.Errorf("expected receipted revert %s, got %+v", c.LXPCode, result.Revert)
		}
	} else if result.Status != 1 || result.Revert != nil {
		return fmt.Errorf("expected successful execution")
	}
	return nil
}

func matchVectorRefusal(result layerx.SequencerReceiptVerification, err error, check layerx.ReceiptCheck) error {
	var failure *layerx.VerificationError
	if check == "" || !errors.Is(err, layerx.ErrVerification) || !errors.As(err, &failure) || failure.Check != check {
		return fmt.Errorf("expected refusal %q, got %+v: %v", check, failure, err)
	}
	if result.Rung != "" || result.Level != "" || len(result.CanonicalBytes) != 0 {
		return fmt.Errorf("refusal returned verified evidence")
	}
	return nil
}

func runVectors(repo string) error {
	root := filepath.Join(repo, "sdk", "platform", "conformance", "fixtures")
	refusals := map[string]struct {
		rung  string
		check layerx.ReceiptCheck
	}{
		"R3: keyId is not in the pinned ReceiptKeys":                  {"R3", layerx.ReceiptCheckKeyID},
		"R3: batch_id height is outside the pinned key range":         {"R3", layerx.ReceiptCheckKeyHeight},
		"R4: activity_id is not keccak256 of the raw transaction":     {"R4", layerx.ReceiptCheckRawTransaction},
		"R6: seqHash does not chain prevSeqHash, seq and activity_id": {"R6", layerx.ReceiptCheckSequenceHash},
		"sequencer receipt: JSON view differs from receipt bytes":     {"R5", layerx.ReceiptCheckLogs},
		"layerx codec: non-canonical":                                 {"R2", layerx.ReceiptCheckCanonicalEncoding},
	}
	var portableReceiptedKeys layerx.ReceiptKeys
	for index, group := range []struct {
		name  string
		count int
	}{
		{"vector-v1-receipted-positive", 4},
		{"vector-v2-receipted-revert", 1},
		{"vector-v3-receipt-refusals", 6},
	} {
		vector, err := readJSON[struct {
			Name  string              `json:"name"`
			Cases []receiptVectorCase `json:"cases"`
		}](filepath.Join(root, group.name+".json"))
		if err != nil {
			return err
		}
		if vector.Name != group.name || len(vector.Cases) != group.count {
			return fmt.Errorf("%s: unexpected vector name or case count", group.name)
		}
		for _, c := range vector.Cases {
			result, err := verifyVectorReceipt(c)
			if index == 2 {
				want, ok := refusals[c.Expected.Check]
				if !ok || c.Expected.RefusedAt != want.rung || c.Expected.Rung != "" {
					return fmt.Errorf("%s: %s: unknown refusal expectation %+v", group.name, c.ID, c.Expected)
				}
				err = matchVectorRefusal(result, err, want.check)
			} else if err == nil {
				if c.Expected.RefusedAt != "" || c.Expected.Check != "" {
					return fmt.Errorf("%s: %s: unexpected refusal expectation", group.name, c.ID)
				}
				err = matchVectorReceipt(c, result, c.Expected.Rung)
			}
			if err != nil {
				return fmt.Errorf("%s: %s: %w", group.name, c.ID, err)
			}
		}
		if index == 0 {
			portableReceiptedKeys, err = vector.Cases[0].keys()
			if err != nil {
				return err
			}
		}
	}
	committed, err := readJSON[struct {
		Name     string `json:"name"`
		Positive struct {
			receiptVectorCase
			BlockHeader      layerx.CommittedHeader  `json:"blockHeader"`
			ReceiptInclusion layerx.ReceiptInclusion `json:"receiptInclusion"`
			Commit           layerx.CometCommit      `json:"commit"`
			ValidatorSet     []layerx.CometValidator `json:"validatorSet"`
		} `json:"positive"`
		Refusals []struct {
			ID            string              `json:"id"`
			NextCommit    layerx.CometCommit  `json:"nextCommit"`
			ExpectedCheck layerx.ReceiptCheck `json:"expectedCheck"`
		} `json:"refusals"`
	}](filepath.Join(root, "vector-v4-committed.json"))
	if err != nil {
		return err
	}
	if committed.Name != "vector-v4-committed" || len(committed.Refusals) != 1 {
		return fmt.Errorf("vector-v4-committed: unexpected name or refusal count")
	}
	c := committed.Positive
	receipt, err := verifyVectorReceipt(c.receiptVectorCase)
	if err == nil {
		err = matchVectorReceipt(c.receiptVectorCase, receipt, "receipted")
	}
	if err != nil {
		return fmt.Errorf("%s: %s: %w", committed.Name, c.ID, err)
	}
	result, err := layerx.VerifyCommittedReceipt(receipt, c.BlockHeader, c.ReceiptInclusion, c.Commit, c.ValidatorSet)
	if err == nil {
		err = matchVectorReceipt(c.receiptVectorCase, result.SequencerReceiptVerification, c.Expected.Rung)
	}
	if err != nil {
		return fmt.Errorf("%s: %s: %w", committed.Name, c.ID, err)
	}
	for _, refusal := range committed.Refusals {
		inclusion := c.ReceiptInclusion
		inclusion.NextCommit = refusal.NextCommit
		result, err := layerx.VerifyCommittedReceipt(receipt, c.BlockHeader, inclusion, c.Commit, c.ValidatorSet)
		if err := matchVectorRefusal(result.SequencerReceiptVerification, err, refusal.ExpectedCheck); err != nil {
			return fmt.Errorf("%s: %s: %w", committed.Name, refusal.ID, err)
		}
	}
	portable, err := readJSON[struct {
		Name    string            `json:"name"`
		Bundles []json.RawMessage `json:"bundles"`
	}](filepath.Join(root, "vector-v5-portable.json"))
	if err != nil {
		return err
	}
	if portable.Name != "vector-v5-portable" || len(portable.Bundles) != 2 {
		return fmt.Errorf("vector-v5-portable: unexpected name or bundle count")
	}
	committedKeys, err := c.keys()
	if err != nil {
		return err
	}
	for i, encoded := range portable.Bundles {
		var bundle layerx.PortableReceipt
		if err := json.Unmarshal(encoded, &bundle); err != nil {
			return fmt.Errorf("%s: bundle %d: %w", portable.Name, i, err)
		}
		keys, level := portableReceiptedKeys, "receipted"
		if i == 1 {
			keys, level = committedKeys, "committed"
		}
		result, err := layerx.VerifyPortableReceipt(encoded, keys)
		if err != nil {
			return fmt.Errorf("%s: bundle %d: %w", portable.Name, i, err)
		}
		if bundle.VerificationLevel != level || result.Rung != bundle.VerificationLevel || result.Level != bundle.VerificationLevel {
			return fmt.Errorf("%s: bundle %d: expected %s, got rung %s and level %s", portable.Name, i, bundle.VerificationLevel, result.Rung, result.Level)
		}
	}
	return nil
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
	catalog, err := readJSON[struct {
		Operations operations `json:"operations"`
	}](filepath.Join(repo, "sdk", "platform", "conformance", "operations.json"))
	if err != nil {
		return err
	}
	humanOperations := make(map[layerx.HumanOperation]operationDescriptor)
	for _, entries := range catalog.Operations {
		for _, entry := range entries {
			if entry.Plane == "human" {
				humanOperations[layerx.HumanOperation(entry.Name)] = entry
			}
		}
	}
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
		if strings.HasPrefix(metadata.Method, "eth_") {
			entry, exists := humanOperations[operation]
			if !exists || entry.Transport != "evm" || metadata.Method != entry.Method || metadata.PerLeg != entry.PerLeg || metadata.Path != "" || entry.Path != "" {
				return fmt.Errorf("generated EVM routing for %s diverges from operations.json", operation)
			}
		} else if request.Method != metadata.Method || !pathMatches(metadata.Path, request.Path) {
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
