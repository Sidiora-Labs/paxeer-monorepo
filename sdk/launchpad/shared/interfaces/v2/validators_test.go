package v2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type validationCase struct {
	Name     string          `json:"name"`
	Rules    []string        `json:"rules"`
	Contract string          `json:"contract"`
	Pointer  string          `json:"pointer"`
	Schema   json.RawMessage `json:"schema"`
	Raw      string          `json:"raw"`
	Accepted bool            `json:"accepted"`
	Error    string          `json:"error"`
}

type validationCorpus struct {
	SchemaDocumentSHA256 string           `json:"schemaDocumentSha256"`
	Cases                []validationCase `json:"cases"`
	LargeDocument        struct {
		Contract string `json:"contract"`
		Raw      string `json:"raw"`
	} `json:"largeDocument"`
}

func readValidationCorpus(t *testing.T) validationCorpus {
	t.Helper()
	data, err := os.ReadFile("testdata/validation-results.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus validationCorpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(schemaDocument))
	if hex.EncodeToString(digest[:]) != corpus.SchemaDocumentSHA256 {
		t.Fatal("schema document digest differs")
	}
	for i := range corpus.Cases {
		c := &corpus.Cases[i]
		if c.Pointer == "" {
			continue
		}
		node := json.RawMessage(schemaDocument)
		for _, name := range strings.Split(c.Pointer, "/")[1:] {
			var object map[string]json.RawMessage
			if err := json.Unmarshal(node, &object); err != nil {
				t.Fatal(err)
			}
			var ok bool
			node, ok = object[name]
			if !ok {
				t.Fatalf("unknown schema pointer %s", c.Pointer)
			}
		}
		c.Schema = node
	}
	return corpus
}

func checkValidationResult(c validationCase) error {
	var err error
	if c.Contract != "" {
		err = ValidateJSON(c.Contract, []byte(c.Raw))
	} else {
		err = validateNode(c.Schema, []byte(c.Raw), "$")
	}
	message := ""
	if err != nil {
		message = err.Error()
	}
	if (err == nil) != c.Accepted || message != c.Error {
		return fmt.Errorf("%s: accepted=%t error=%q; want accepted=%t error=%q", c.Name, err == nil, message, c.Accepted, c.Error)
	}
	return nil
}

func TestValidationResults(t *testing.T) {
	corpus := readValidationCorpus(t)
	coverage := make(map[string]map[bool]bool)
	for _, c := range corpus.Cases {
		t.Run(c.Name, func(t *testing.T) {
			if err := checkValidationResult(c); err != nil {
				t.Fatal(err)
			}
		})
		for _, rule := range c.Rules {
			if coverage[rule] == nil {
				coverage[rule] = make(map[bool]bool)
			}
			coverage[rule][c.Accepted] = true
		}
	}
	rule := reflect.TypeOf(schemaRule{})
	for i := 0; i < rule.NumField(); i++ {
		name := rule.Field(i).Tag.Get("json")
		if !coverage[name][true] || !coverage[name][false] {
			t.Errorf("rule %s requires both accepted and refused documents", name)
		}
	}
	if err := ValidateJSONVersion(2, "UInt256", []byte(`"1"`)); err != nil {
		t.Fatal(err)
	}
	if err := ValidateJSONVersion(1, "UInt256", []byte(`"1"`)); err == nil || err.Error() != "unsupported schema major 1" {
		t.Fatalf("version error: %v", err)
	}
}

func TestValidationCompilesOnce(t *testing.T) {
	corpus := readValidationCorpus(t)
	coldSchema := json.RawMessage(`{"type":"array","minItems":1,"items":{"type":"string","pattern":"^(?:concurrent-rule){1}$"}}`)
	coldRaw := json.RawMessage(`["concurrent-rule"]`)
	decodes, compiles := schemaDecodes.Load(), expressionCompiles.Load()
	var group sync.WaitGroup
	for i := 0; i < 32; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := validateNode(coldSchema, coldRaw, "$"); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	if got := schemaDecodes.Load() - decodes; got != 2 {
		t.Fatalf("concurrent first use decoded %d schema nodes; want 2", got)
	}
	if got := expressionCompiles.Load() - compiles; got != 1 {
		t.Fatalf("concurrent first use compiled %d expressions; want 1", got)
	}
	var proof InputWalletProof
	proofJSON := []byte(`{"kind":"erc1271","wallet":"0x1111111111111111111111111111111111111111","signature":"0x"}`)
	validateAll := func() {
		for _, c := range corpus.Cases {
			if err := checkValidationResult(c); err != nil {
				t.Error(err)
			}
		}
		var value InputWalletProof
		if err := json.Unmarshal(proofJSON, &value); err != nil {
			t.Error(err)
			return
		}
		if _, err := json.Marshal(value); err != nil {
			t.Error(err)
		}
	}
	if err := json.Unmarshal(proofJSON, &proof); err != nil {
		t.Fatal(err)
	}
	validateAll()
	decodes, compiles = schemaDecodes.Load(), expressionCompiles.Load()
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 4; j++ {
				validateAll()
			}
		}()
	}
	group.Wait()
	if got := schemaDecodes.Load(); got != decodes {
		t.Errorf("repeated validation decoded %d additional schema nodes", got-decodes)
	}
	if got := expressionCompiles.Load(); got != compiles {
		t.Errorf("repeated validation compiled %d additional expressions", got-compiles)
	}
	large := corpus.LargeDocument
	const runs = 10
	started := time.Now()
	for i := 0; i < runs; i++ {
		if err := ValidateJSON(large.Contract, []byte(large.Raw)); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("document_bytes=%d runs=%d ns_per_validation=%d", len(large.Raw), runs, time.Since(started).Nanoseconds()/runs)
}
