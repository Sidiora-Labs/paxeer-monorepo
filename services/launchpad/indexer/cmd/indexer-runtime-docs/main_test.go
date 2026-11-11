package main

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/runtimedocs"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
)

func TestInputPathsResolveCommittedInputs(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source path unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", "..", ".."))
	reviewed, artifact := inputPaths(root)
	if want := filepath.Join(root, "sdk", "launchpad", "protocol", "catalog", "reviewed.json"); reviewed != want {
		t.Fatalf("reviewed path = %q, want %q", reviewed, want)
	}
	if want := filepath.Join(root, "contracts", "launchpad", "artifacts", "contracts", "core", "SidioraERC20.sol", "SidioraERC20.json"); artifact != want {
		t.Fatalf("artifact path = %q, want %q", artifact, want)
	}
	if _, err := os.ReadFile(reviewed); err != nil {
		t.Fatalf("read committed reviewed catalog: %v", err)
	}
	data, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read committed artifact: %v", err)
	}
	var compiled struct {
		DeployedBytecode string `json:"deployedBytecode"`
	}
	if err := json.Unmarshal(data, &compiled); err != nil {
		t.Fatalf("decode committed artifact: %v", err)
	}
	code, err := hex.DecodeString(strings.TrimPrefix(compiled.DeployedBytecode, "0x"))
	if err != nil {
		t.Fatalf("decode deployed bytecode: %v", err)
	}
	if len(code) == 0 {
		t.Fatal("committed artifact has empty deployed bytecode")
	}
}

func TestRepair_22_39_359_lag_budget_is_required(t *testing.T) {
	bound := int64(math.MaxInt64 / int64(time.Millisecond))
	for _, value := range []string{"", "0", "-1", strconv.FormatInt(bound+1, 10), "60000.5", "invalid", "9223372036854775808"} {
		t.Run("reject_"+value, func(t *testing.T) {
			_, err := deliveryParameters(value, "")
			if err == nil || !strings.Contains(err.Error(), "-max-delivery-lag-ms") {
				t.Fatalf("budget %q must fail naming its flag: %v", value, err)
			}
			originalFlags, originalArgs := flag.CommandLine, os.Args
			t.Cleanup(func() { flag.CommandLine, os.Args = originalFlags, originalArgs })
			flag.CommandLine = flag.NewFlagSet("indexer-runtime-docs", flag.ContinueOnError)
			os.Args = []string{"indexer-runtime-docs", "-start-height", "18446744073709551615", "-anchor-rpc", ":invalid-url"}
			if value != "" {
				os.Args = append(os.Args, "-max-delivery-lag-ms", value)
			}
			if runErr := run(); runErr == nil || runErr.Error() != err.Error() {
				t.Fatalf("command must reject the budget before accessing the anchor: got %v, want %v", runErr, err)
			}
		})
	}
	want := app.DefaultDeliveryParameters()
	want.MaxDeliveryLagMS = 60000
	parameters, err := deliveryParameters("60000", "")
	if err != nil || !reflect.DeepEqual(parameters, want) || parameters.DeadLetters != nil {
		t.Fatalf("lag budget must preserve every default and omit the policy: got %+v, want %+v, error %v", parameters, want, err)
	}
	for _, value := range []int64{1, bound} {
		parameters, err := deliveryParameters(strconv.FormatInt(value, 10), "")
		if err != nil || parameters.MaxDeliveryLagMS != value {
			t.Fatalf("valid boundary %d refused: %+v, %v", value, parameters, err)
		}
	}
	policy := app.DeadLetterParameters{
		SecuritySchema: "security", Audience: "indexer", PolicyVersion: "v1", Issuer: "gateway",
		PeerCertSHA256: strings.Repeat("a", 64), PublicKey: []byte("01234567890123456789012345678901"),
		AssertionHeader: "X-Service-Assertion", SignatureHeader: "X-Service-Signature",
	}
	raw, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "dead-letters.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	want.DeadLetters = &policy
	parameters, err = deliveryParameters("60000", path)
	if err != nil || !reflect.DeepEqual(parameters, want) {
		t.Fatalf("policy must preserve all declared members and defaults: got %+v, want %+v, error %v", parameters, want, err)
	}
	for name, data := range map[string]string{
		"unknown_member": strings.TrimSuffix(string(raw), "}") + `,"unknown":true}`,
		"malformed":      `{"audience":`, "wrong_type": `{"publicKey":42}`,
		"null": "null", "empty": "", "array": "[]", "trailing_object": string(raw) + " {}", "trailing_garbage": string(raw) + " garbage",
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := deliveryParameters("60000", path); err == nil || !strings.Contains(err.Error(), "-dead-letters") {
				t.Fatalf("invalid policy must fail naming its flag: %v", err)
			}
		})
	}
	if _, err := deliveryParameters("60000", filepath.Join(t.TempDir(), "missing.json")); err == nil || !strings.Contains(err.Error(), "-dead-letters") {
		t.Fatalf("unreadable policy must fail naming its flag: %v", err)
	}
}

func TestRepair_22_39_359_rendered_delivery_document_carries_the_budget(t *testing.T) {
	parameters, err := deliveryParameters("60000", "")
	if err != nil {
		t.Fatal(err)
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source path unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", "..", ".."))
	manifest, ok := deployment.Approved(32226215)
	if !ok {
		t.Fatal("recorded anchor requires the approved historical deployment")
	}
	reviewed, err := os.ReadFile(filepath.Join(root, "sdk", "launchpad", "protocol", "catalog", "history", "reviewed-a35ff7c6f532d2f9.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, artifactPath := inputPaths(root)
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	var compiled struct {
		DeployedBytecode string `json:"deployedBytecode"`
	}
	if err := json.Unmarshal(artifact, &compiled); err != nil {
		t.Fatal(err)
	}
	code, err := hex.DecodeString(strings.TrimPrefix(compiled.DeployedBytecode, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := os.Open(filepath.Join(root, "services", "launchpad", "indexer", "internal", "internaltest", "chainfixture", "testdata", "paxeer125-market.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer corpus.Close()
	reader, err := gzip.NewReader(corpus)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	decoder := json.NewDecoder(reader)
	anchor := receipt.BlockAnchor{Height: 33963715}
	for {
		// Results of other RPC methods have different shapes; decode the
		// recorded block only after matching its request key.
		var recorded struct {
			Key    string          `json:"k"`
			Result json.RawMessage `json:"result"`
		}
		if err := decoder.Decode(&recorded); err != nil {
			t.Fatalf("read recorded anchor: %v", err)
		}
		if recorded.Key != fmt.Sprintf(`eth_getBlockByNumber ["0x%x",false]`, anchor.Height) {
			continue
		}
		var block struct {
			Number string      `json:"number"`
			Hash   common.Hash `json:"hash"`
		}
		if err := json.Unmarshal(recorded.Result, &block); err != nil {
			t.Fatal(err)
		}
		if block.Number != fmt.Sprintf("0x%x", anchor.Height) || block.Hash == (common.Hash{}) {
			t.Fatal("recorded anchor has invalid block identity")
		}
		anchor.Hash = block.Hash
		break
	}
	documents, err := app.RenderRuntimeDocuments(runtimedocs.Inputs{
		Manifest: manifest, ReviewedCatalog: reviewed, ERC20RuntimeCode: code,
		PoolRuntimeHash: common.HexToHash("0xa9b304d6d0aa562f283b45978b84ced8840a9331f8ac946742670b1b02a2872d"), Anchor: anchor,
	}, parameters)
	if err != nil {
		t.Fatal(err)
	}
	var delivery map[string]json.RawMessage
	if err := json.Unmarshal(documents["delivery.json"].Bytes, &delivery); err != nil {
		t.Fatal(err)
	}
	var lag int64
	if err := json.Unmarshal(delivery["maxDeliveryLagMs"], &lag); err != nil || lag != 60000 {
		t.Fatalf("rendered delivery lag = %d, want 60000: %v", lag, err)
	}
	if _, present := delivery["deadLetters"]; present {
		t.Fatal("delivery document must omit an unrequested dead letter policy")
	}
}
