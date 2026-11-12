package catalog_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/protocol/catalog"
	"github.com/Sidiora-Technologies/KindleLaunch/protocol/coverage"
	"github.com/ethereum/go-ethereum/common"
)

func TestReviewedCatalogDecoderBurstFromCommittedReceipt(t *testing.T) {
	root, err := filepath.Abs("../../../../contracts/launchpad/contracts")
	if err != nil {
		t.Fatal(err)
	}
	producerModule, err := os.ReadFile("../coverage/testdata/repair28producer/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	modfile := filepath.Join(t.TempDir(), "producer.mod")
	if err := os.WriteFile(modfile, producerModule, 0600); err != nil {
		t.Fatal(err)
	}
	producerContext, cancelProducer := context.WithTimeout(context.Background(), 8*time.Minute)
	t.Cleanup(cancelProducer)
	command := exec.CommandContext(producerContext, "go", "run", "-modfile="+modfile, "-mod=mod", ".", root)
	command.Dir = "../coverage/testdata/repair28producer"
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.Create(filepath.Join(t.TempDir(), "producer.log"))
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	stopProducer := func() {
		once.Do(func() {
			stdin.Close()
			if err := command.Wait(); err != nil {
				stderr.Close()
				output, _ := os.ReadFile(stderr.Name())
				t.Errorf("real contract producer: %v\n%s", err, output)
			} else {
				stderr.Close()
			}
		})
	}
	t.Cleanup(stopProducer)
	var producer struct {
		Endpoint  string
		Addresses map[string]common.Address
		Funded    common.Hash
	}
	if err := json.NewDecoder(stdout).Decode(&producer); err != nil {
		t.Fatal(err)
	}
	if producer.Funded == (common.Hash{}) || producer.Addresses["USDL"] == (common.Address{}) {
		t.Fatal("committed funding receipt or token address missing")
	}
	bundles, err := catalog.Bundles()
	if err != nil {
		t.Fatal(err)
	}
	var deployment catalog.Deployment
	for _, bundle := range bundles {
		if bundle.Name == "SidioraERC20" {
			deployment = catalog.Deployment{ChainID: 125, Address: producer.Addresses["USDL"], Family: bundle.Name, Version: bundle.ABIHash, Attested: true}
			break
		}
	}
	if deployment.Version == "" {
		t.Fatal("reviewed token catalog entry missing")
	}
	zeroDigest := strings.Repeat("0", 64)
	manifest := coverage.MeasurementManifest{
		SchemaVersion: 1, ChainID: 125, SourceManifestSHA256: zeroDigest, InterfaceSHA256: zeroDigest,
		Deployments: []catalog.Deployment{deployment}, EnabledFamilies: []string{"SidioraERC20"},
		Receipts:                 []coverage.ReceiptRequest{{TransactionHash: producer.Funded, Scenario: 2, ExpectedCanonicalSHA256: zeroDigest, ExpectedProjectionSHA256: zeroDigest}},
		ExpectedProjectionSHA256: zeroDigest,
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	input, err := coverage.ReadMeasurementManifest(raw, hex.EncodeToString(digest[:]))
	if err != nil {
		t.Fatal(err)
	}
	registry, _, err := coverage.MeasurementRegistry(input)
	if err != nil {
		t.Fatal(err)
	}
	client, err := coverage.DialMeasurementRPC(context.Background(), producer.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := coverage.CaptureHistorical(context.Background(), client, input, hex.EncodeToString(digest[:]))
	if err != nil {
		client.Close()
		t.Fatal(err)
	}
	if len(capture.Receipts) != 1 || capture.Receipts[0].Receipt.TxHash != producer.Funded || len(capture.Receipts[0].Receipt.Logs) != 1 || capture.Receipts[0].Receipt.Logs[0].Address != producer.Addresses["USDL"] {
		client.Close()
		t.Fatalf("committed funding transfer differs: %+v", capture)
	}
	client.Close()
	stopProducer()
	fullContext, finish := context.WithTimeout(context.Background(), 12*time.Minute)
	defer finish()
	full, err := coverage.MeasureDecoderBurst(fullContext, registry, capture)
	if err != nil || full == nil || full.Attempted != coverage.MeasurementRate*coverage.MeasurementSeconds || full.Decoded != full.Attempted || full.Matched != full.Attempted || full.Mismatched != 0 || len(full.Windows) != coverage.MeasurementSeconds || len(full.OutputSHA256) != 64 {
		t.Fatalf("complete decoder replay differs: %+v %v", full, err)
	}
	if _, err := hex.DecodeString(full.OutputSHA256); err != nil {
		t.Fatalf("complete decoder digest differs: %v", err)
	}
}
