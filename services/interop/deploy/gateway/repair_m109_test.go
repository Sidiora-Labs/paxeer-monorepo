package gateway

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
)

func repairM109Read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func repairM109JSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := json.Unmarshal(repairM109Read(t, path), value); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

type repairM109Pin struct {
	KeyID       string `json:"keyId"`
	PublicKey   string `json:"publicKey"`
	FirstHeight uint64 `json:"firstHeight"`
	LastHeight  uint64 `json:"lastHeight"`
}

type repairM109Manifest struct {
	Adapters []struct {
		ID                  string `json:"id"`
		SpecificationSHA256 string `json:"specification_sha256"`
		Suite               string `json:"conformance_suite"`
		Count               int    `json:"conformance_vectors"`
		Digest              string `json:"conformance_sha256"`
	} `json:"adapters"`
	Transports []struct {
		ID                  string `json:"id"`
		SpecificationSHA256 string `json:"specification_sha256"`
		Digest              string `json:"conformance_sha256"`
	} `json:"transports"`
	Supported struct {
		Kinds []struct {
			Version int    `json:"x402Version"`
			Scheme  string `json:"scheme"`
			Network string `json:"network"`
		} `json:"kinds"`
		Signers map[string][]string `json:"signers"`
	} `json:"x402_supported"`
}

func repairM109Suite(t *testing.T, directory string) (int, string) {
	t.Helper()
	digest := sha256.New()
	count := 0
	var files []string
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	for _, path := range files {
		data := repairM109Read(t, path)
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			t.Fatal(err)
		}
		digest.Write([]byte(filepath.ToSlash(relative)))
		digest.Write([]byte{0})
		digest.Write(data)
		digest.Write([]byte{0})
		var records any
		if err := json.Unmarshal(data, &records); err != nil {
			t.Fatal(err)
		}
		if array, ok := records.([]any); ok {
			if len(array) == 0 {
				t.Fatalf("%s has no vectors", path)
			}
			count += len(array)
		} else {
			count++
		}
	}
	if len(files) == 0 || count == 0 {
		t.Fatalf("%s has no conformance suite", directory)
	}
	return count, hex.EncodeToString(digest.Sum(nil))
}

func TestRepairM_109_ExamplePins(t *testing.T) {
	var manifest repairM109Manifest
	repairM109JSON(t, "config.example.json", &manifest)
	root := filepath.Join("..", "..", "..", "..")
	documents := map[string]string{
		"x402":     "services/interop/specs/vendor/x402/x402-specification-v2.md",
		"ap2":      "services/interop/specs/vendor/ap2/specification.md",
		"ucp":      "services/interop/specs/vendor/ucp/specification-checkout.html",
		"visa-tap": "services/interop/specs/vendor/visa-tap/README.md",
		"fiat":     "docs/wiki/FiatRamps.md",
	}
	if len(manifest.Adapters) != len(documents) || len(manifest.Transports) != 3 {
		t.Fatal("the manifest must pin every adapter and transport")
	}
	seen := map[string]bool{}
	for _, adapter := range manifest.Adapters {
		if documents[adapter.ID] == "" || seen[adapter.ID] {
			t.Fatalf("unexpected adapter %q", adapter.ID)
		}
		seen[adapter.ID] = true
		count, digest := repairM109Suite(t, filepath.Join(root, "services/interop/specs/conformance", adapter.ID))
		if adapter.Count != count || adapter.Digest != digest || adapter.Suite != "layerx-"+adapter.ID+"-conformance-v1" {
			t.Errorf("%s pins (%d, %s); checkout has (%d, %s)", adapter.ID, adapter.Count, adapter.Digest, count, digest)
		}
		spec := sha256.Sum256(repairM109Read(t, filepath.Join(root, documents[adapter.ID])))
		if adapter.SpecificationSHA256 != hex.EncodeToString(spec[:]) {
			t.Errorf("%s specification pin drifted", adapter.ID)
		}
	}
	transports := map[string]bool{"http": false, "mcp": false, "a2a": false}
	for _, transport := range manifest.Transports {
		already, exists := transports[transport.ID]
		if !exists || already {
			t.Fatalf("unexpected transport %q", transport.ID)
		}
		transports[transport.ID] = true
		_, digest := repairM109Suite(t, filepath.Join(root, "services/interop/specs/conformance/transport-"+transport.ID))
		if transport.Digest != digest {
			t.Errorf("%s conformance pin drifted", transport.ID)
		}
		spec := sha256.Sum256(repairM109Read(t, filepath.Join(root, "services/interop/specs/vendor/x402/transports", transport.ID+".md")))
		if transport.SpecificationSHA256 != hex.EncodeToString(spec[:]) {
			t.Errorf("%s specification pin drifted", transport.ID)
		}
	}
	var document, inputs map[string]json.RawMessage
	repairM109JSON(t, "config.example.json", &document)
	repairM109JSON(t, "trust-inputs.example.json", &inputs)
	if len(inputs) != 5 {
		t.Fatal("all five external trust inputs must be explicit")
	}
	for _, name := range []string{"ap2_keys", "ap2_assets", "visa_agents", "visa_targets", "fiat_providers"} {
		var actual, expected any
		if err := json.Unmarshal(document[name], &actual); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(inputs[name], &expected); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("%s differs from example trust input", name)
		}
	}
}

func TestRepairM_109_ReadmeManifest(t *testing.T) {
	readme := strings.Join(strings.Fields(string(repairM109Read(t, "README.md"))), " ")
	for _, required := range []string{
		"all five adapters and three transport bindings", "LAYERX_INTEROP_RECEIPT_KEYS_FILE",
		"px_getReceiptKeys", "complete signer set", "eth_sendRawTransaction",
		"pax_sequencerReceipt", "eth_getBlockByNumber", "Every `keyId` must be unique",
		"does not establish that the conformance suites passed", "consensus commit proof",
		"authority-token and single-key mounts still present in that template are retired",
		"--network-id eip155-125", "--receipt-keys-file", "env -i",
	} {
		if !strings.Contains(readme, required) {
			t.Errorf("README omits %q", required)
		}
	}
	if strings.Contains(readme, "counts are stale") {
		t.Fatal("README still describes the obsolete manifest")
	}
	for _, path := range []string{"config.example.json", "trust-inputs.example.json", "receipt-keys.example.json"} {
		if !strings.Contains(readme, "["+path+"]("+path+")") {
			t.Errorf("README omits input link %s", path)
		}
		var value any
		repairM109JSON(t, path, &value)
	}
}

func repairM109Hex(t *testing.T, value string, length int) []byte {
	t.Helper()
	if !strings.HasPrefix(value, "0x") {
		t.Fatal("recorded value lacks hex prefix")
	}
	data, err := hex.DecodeString(value[2:])
	if err != nil {
		t.Fatal(err)
	}
	if length > 0 && len(data) != length {
		t.Fatalf("expected %d bytes, got %d", length, len(data))
	}
	return data
}

func TestRepairM_109_ReceiptKeyWindows(t *testing.T) {
	var pins []repairM109Pin
	var manifest repairM109Manifest
	repairM109JSON(t, "receipt-keys.example.json", &pins)
	repairM109JSON(t, "config.example.json", &manifest)
	if len(pins) != 2 {
		t.Fatal("example must cover both recorded receipt windows")
	}
	if pins[0].FirstHeight > pins[0].LastHeight || pins[1].FirstHeight > pins[1].LastHeight || pins[0].LastHeight >= pins[1].FirstHeight || pins[0].KeyID == pins[1].KeyID {
		t.Fatal("example keys need unique ids and valid disjoint windows")
	}
	signers := []string{}
	for _, pin := range pins {
		signers = append(signers, "did:layerx:"+pin.PublicKey[2:])
	}
	sort.Strings(signers)
	if !reflect.DeepEqual(manifest.Supported.Signers, map[string][]string{"eip155:125": signers}) {
		t.Fatal("manifest must advertise the complete signer set")
	}
	if len(manifest.Supported.Kinds) != 1 || manifest.Supported.Kinds[0].Network != "eip155:125" || manifest.Supported.Kinds[0].Scheme != "exact" || manifest.Supported.Kinds[0].Version != 2 {
		t.Fatal("manifest must advertise exact x402 v2 on eip155:125")
	}
	fixtures := filepath.Join("..", "..", "..", "..", "sdk/platform/conformance/fixtures")
	var old struct {
		Cases []struct {
			Receipt string        `json:"signedReceipt"`
			Pin     repairM109Pin `json:"pinnedKey"`
		} `json:"cases"`
	}
	var current struct {
		Vectors []struct {
			Receipt string          `json:"receipt"`
			Keys    []repairM109Pin `json:"receiptKeys"`
		} `json:"vectors"`
	}
	repairM109JSON(t, filepath.Join(fixtures, "sequencer-receipt-v1.json"), &old)
	repairM109JSON(t, filepath.Join(fixtures, "receipt-sequencer-v1.json"), &current)
	if len(old.Cases) == 0 || len(current.Vectors) == 0 || len(current.Vectors[0].Keys) == 0 {
		t.Fatal("missing producer recordings")
	}
	for index, recording := range []string{old.Cases[0].Receipt, current.Vectors[0].Receipt} {
		pin := pins[index]
		producerPin := old.Cases[0].Pin
		if index == 1 {
			producerPin = current.Vectors[0].Keys[0]
		}
		if pin.KeyID != producerPin.KeyID || pin.PublicKey != producerPin.PublicKey {
			t.Fatal("example key differs from the real producer's recorded public key")
		}
		var publicKey [32]byte
		copy(publicKey[:], repairM109Hex(t, pin.PublicKey, 32))
		receipt := repairM109Hex(t, recording, 0)
		verified, err := verify.ReceiptSignature(receipt, publicKey)
		if err != nil {
			t.Fatalf("window %d: real proof verifier refused recording: %v", index, err)
		}
		height := binary.BigEndian.Uint64(verified.Receipt.BatchID[24:])
		if height < pin.FirstHeight || height > pin.LastHeight || !bytes.Equal(verified.Receipt.ContextHash[:], repairM109Hex(t, pin.KeyID, 32)) {
			t.Fatalf("window %d does not authorize the recorded key id and height", index)
		}
		if height != []uint64{1000, 1200}[index] {
			t.Fatal("recorded receipt height changed")
		}
		foreign := publicKey
		foreign[0] ^= 1
		if _, err := verify.ReceiptSignature(receipt, foreign); !errors.Is(err, verify.ErrSequencerSignature) {
			t.Fatalf("foreign signer must fail signature verification: %v", err)
		}
		tampered := append([]byte(nil), receipt...)
		tampered[len(tampered)-1] ^= 1
		if _, err := verify.ReceiptSignature(tampered, publicKey); !errors.Is(err, verify.ErrSequencerSignature) {
			t.Fatalf("tampered signature must be refused: %v", err)
		}
	}
}
