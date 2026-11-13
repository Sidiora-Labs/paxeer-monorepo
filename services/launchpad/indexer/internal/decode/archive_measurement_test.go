package decode_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func measurementDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
func writeMeasurementInput(t *testing.T, path string, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	return measurementDigest(body)
}

func nativeArchiveMeasurement(t *testing.T) decode.DecoderMeasurementInput {
	t.Helper()
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	contractsRoot := filepath.Join(root, "contracts", "launchpad")
	body, err := os.ReadFile(filepath.Join(root, "sdk/launchpad/protocol/catalog/reviewed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		SchemaVersion  int                          `json:"schemaVersion"`
		ChainID        uint64                       `json:"chainId"`
		ManifestSHA256 string                       `json:"manifestSha256"`
		Contracts      []map[string]json.RawMessage `json:"contracts"`
	}
	if err = json.Unmarshal(body, &catalog); err != nil {
		t.Fatal(err)
	}
	selected := []map[string]json.RawMessage{}
	revisions := map[string]string{}
	for _, contract := range catalog.Contracts {
		var name, artifactPath string
		if err = json.Unmarshal(contract["name"], &name); err != nil {
			t.Fatal(err)
		}
		switch name {
		case "EventEmitter", "Router", "SidioraPool", "SidioraERC20":
		default:
			continue
		}
		if err = json.Unmarshal(contract["artifactPath"], &artifactPath); err != nil {
			t.Fatal(err)
		}
		artifact, err := os.ReadFile(filepath.Join(contractsRoot, artifactPath))
		if err != nil {
			t.Fatal(err)
		}
		var generated struct{ ABI json.RawMessage }
		if err = json.Unmarshal(artifact, &generated); err != nil {
			t.Fatal(err)
		}
		var approvedABI, actualABI bytes.Buffer
		if err = json.Compact(&approvedABI, contract["abi"]); err != nil {
			t.Fatal(err)
		}
		if err = json.Compact(&actualABI, generated.ABI); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(approvedABI.Bytes(), actualABI.Bytes()) {
			t.Fatalf("compiled %s ABI differs from the source catalog", name)
		}
		var debug struct {
			BuildInfo string `json:"buildInfo"`
		}
		debugBody, err := os.ReadFile(filepath.Join(contractsRoot, strings.TrimSuffix(artifactPath, ".json")+".dbg.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(debugBody, &debug); err != nil {
			t.Fatal(err)
		}
		build, err := os.ReadFile(filepath.Join(contractsRoot, filepath.Dir(artifactPath), debug.BuildInfo))
		if err != nil {
			t.Fatal(err)
		}
		contract["artifactSha256"], err = json.Marshal(measurementDigest(artifact))
		if err != nil {
			t.Fatal(err)
		}
		contract["buildInfoSha256"], err = json.Marshal(measurementDigest(build))
		if err != nil {
			t.Fatal(err)
		}
		var revision string
		if err = json.Unmarshal(contract["abiSha256"], &revision); err != nil {
			t.Fatal(err)
		}
		revisions[name] = revision
		selected = append(selected, contract)
	}
	if len(selected) != 4 {
		t.Fatal("native decoder compiler artifacts absent")
	}
	catalog.Contracts = selected
	directory := t.TempDir()
	input := decode.DecoderMeasurementInput{SchemaVersion: 1, SourceRoot: root, ContractsRoot: contractsRoot, CatalogPath: filepath.Join(directory, "catalog.json"), DeploymentPath: filepath.Join(directory, "deployments.json"), ArchivePath: filepath.Join(directory, "archive.json"), Workers: 2, MaxP99Nanoseconds: int64(50 * time.Millisecond), MaxHeapGrowthBytes: 512 << 20, MaxAllocatedBytesPerDelivery: 1 << 20}
	input.CatalogSHA256 = writeMeasurementInput(t, input.CatalogPath, catalog)
	producer := repair995Load(t)
	tx, receipt := repair995Bundle(t, producer)
	market := producer.Markets[0]
	deployments := decode.DecoderDeploymentInput{SchemaVersion: 1, ManifestSHA256: catalog.ManifestSHA256, Markets: repair995Markets(producer), Deployments: []decode.Deployment{
		{Address: decode.LaunchpadPrecompileAddress, Role: decode.LaunchpadPrecompileRole, ABIRevision: revisions["EventEmitter"]},
		{Address: decode.LaunchpadPrecompileAddress, Role: "SidioraPool", ABIRevision: revisions["SidioraPool"]},
		{Address: market.Pool, Role: "SidioraPool", ABIRevision: revisions["SidioraPool"]},
		{Address: market.Token, Role: "SidioraERC20", ABIRevision: revisions["SidioraERC20"]},
		{Address: common.HexToAddress(producer.Options.Manifest.Roles["USDL"].Address), Role: "SidioraERC20", ABIRevision: revisions["SidioraERC20"]},
	}}
	for _, family := range []string{"pecor", "orders", "vault", "routing", "tracker", "oracle"} {
		deployments.Capabilities = append(deployments.Capabilities, decode.CapabilityPolicy{Family: family})
	}
	input.DeploymentSHA256 = writeMeasurementInput(t, input.DeploymentPath, deployments)
	corpus := decode.ArchivedDecoderCorpus{SchemaVersion: 1, ChainID: 125, CatalogSHA256: input.CatalogSHA256}
	for _, log := range receipt.Logs[:2] {
		args, err := json.Marshal(map[string]any{"from": strings.ToLower(common.BytesToAddress(log.Topics[1].Bytes()).Hex()), "to": strings.ToLower(common.BytesToAddress(log.Topics[2].Bytes()).Hex()), "value": new(big.Int).SetBytes(log.Data).String()})
		if err != nil {
			t.Fatal(err)
		}
		corpus.Logs = append(corpus.Logs, decode.ArchivedDecoderLog{Log: *log, Expected: decode.ArchivedDecodeExpectation{Role: "SidioraERC20", ABIRevision: revisions["SidioraERC20"], EventName: "Transfer", Signature: "Transfer(address,address,uint256)", Status: "decoded", Args: args}})
	}
	transaction, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	origin, err := types.Sender(types.LatestSignerForChainID(big.NewInt(125)), tx)
	if err != nil {
		t.Fatal(err)
	}
	sender := strings.ToLower(origin.Hex())
	caller := strings.ToLower(common.BytesToAddress(receipt.Logs[4].Topics[2].Bytes()).Hex())
	recipient := strings.ToLower(common.BytesToAddress(receipt.Logs[1].Topics[2].Bytes()).Hex())
	identity := func(log *types.Log) decode.RawIdentity {
		return decode.RawIdentity{ChainID: 125, BlockHash: strings.ToLower(log.BlockHash.Hex()), TransactionHash: strings.ToLower(log.TxHash.Hex()), LogIndex: log.Index}
	}
	corpus.ActorReceipts = []decode.ArchivedActorReceipt{{TransactionHex: hex.EncodeToString(transaction), Receipt: *receipt, ExpectedActors: map[uint]decode.ActorContext{receipt.Logs[4].Index: {TransactionOrigin: &sender, EventCaller: &caller, Recipient: &recipient, Beneficiary: &recipient, AttributionConfidence: "receipt_evidence", AttributionMethod: "unique_pool_output_transfer", Revision: "actor-v1", EvidenceIdentity: []decode.RawIdentity{identity(receipt.Logs[4]), identity(receipt.Logs[1])}}}}}
	input.ArchiveSHA256 = writeMeasurementInput(t, input.ArchivePath, corpus)
	input.CandidateSourceSHA256, err = decode.DecoderCandidateSourceDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func TestDecoderArchiveMeasurementReportsIncompleteBurstWithoutCertification(t *testing.T) {
	input := nativeArchiveMeasurement(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	report, err := decode.RunDecoderMeasurement(ctx, input)
	if !errors.Is(err, context.DeadlineExceeded) || report.Passed || report.RestartParity != true || report.ArchivedLogCount != 2 || report.Compatibility.ActorReceipts != 1 || !report.Compatibility.CoreParityWithoutOracle || report.Burst == nil || report.Burst.Completed == 0 || report.Burst.MissingDeliveries == 0 || report.Burst.IdentityFailures != 0 || report.Failure == "" || report.FinishedAt.Before(report.StartedAt) {
		t.Fatalf("bounded archived measurement: %+v %v", report, err)
	}
	path := filepath.Join(t.TempDir(), "report.json")
	if err = decode.WriteDecoderMeasurement(path, report); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var persisted decode.DecoderMeasurementReport
	if err = json.Unmarshal(body, &persisted); err != nil || persisted.ArchivedIdentitySHA256 != report.ArchivedIdentitySHA256 || persisted.Burst.InputIdentitySHA256 != report.Burst.InputIdentitySHA256 || persisted.Passed {
		t.Fatalf("persisted measurement: %v", err)
	}
}

func TestDecoderMeasurementRejectsChangedArchiveAndDeploymentInputs(t *testing.T) {
	original := nativeArchiveMeasurement(t)
	for _, tc := range []struct {
		name   string
		change func(*decode.DecoderMeasurementInput)
	}{
		{"schema", func(i *decode.DecoderMeasurementInput) { i.SchemaVersion = 2 }},
		{"workers", func(i *decode.DecoderMeasurementInput) { i.Workers = 33 }},
		{"contracts", func(i *decode.DecoderMeasurementInput) { i.ContractsRoot = i.SourceRoot }},
		{"source", func(i *decode.DecoderMeasurementInput) { i.CandidateSourceSHA256 = strings.Repeat("0", 64) }},
		{"missing catalog", func(i *decode.DecoderMeasurementInput) { i.CatalogPath += ".absent" }},
		{"catalog digest", func(i *decode.DecoderMeasurementInput) { i.CatalogSHA256 = strings.Repeat("0", 64) }},
		{"deployment digest", func(i *decode.DecoderMeasurementInput) { i.DeploymentSHA256 = strings.Repeat("0", 64) }},
		{"archive digest", func(i *decode.DecoderMeasurementInput) { i.ArchiveSHA256 = strings.Repeat("0", 64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := original
			tc.change(&changed)
			report, err := decode.RunDecoderMeasurement(context.Background(), changed)
			if err == nil || report.Passed || report.Burst != nil || report.Failure == "" {
				t.Fatalf("changed input accepted: %+v %v", report, err)
			}
		})
	}
	for _, field := range []string{"schema", "manifest", "deployments", "capabilities"} {
		t.Run("deployment-"+field, func(t *testing.T) {
			changed := original
			deployment, err := decode.LoadDecoderMeasurementDeployments(original)
			if err != nil {
				t.Fatal(err)
			}
			switch field {
			case "schema":
				deployment.SchemaVersion = 0
			case "manifest":
				deployment.ManifestSHA256 = strings.Repeat("0", 64)
			case "deployments":
				deployment.Deployments = nil
			case "capabilities":
				deployment.Capabilities = nil
			}
			changed.DeploymentPath = filepath.Join(t.TempDir(), "deployment.json")
			changed.DeploymentSHA256 = writeMeasurementInput(t, changed.DeploymentPath, deployment)
			report, err := decode.RunDecoderMeasurement(context.Background(), changed)
			if !errors.Is(err, decode.ErrMeasurementInputs) || report.Passed || report.Burst != nil {
				t.Fatalf("changed deployment accepted: %+v %v", report, err)
			}
		})
	}
	for _, field := range []string{"schema", "chain", "catalog", "logs", "actors", "identity", "removed", "duplicate", "role", "args"} {
		t.Run("archive-"+field, func(t *testing.T) {
			changed := original
			corpus, err := decode.LoadDecoderMeasurementCorpus(original)
			if err != nil {
				t.Fatal(err)
			}
			switch field {
			case "schema":
				corpus.SchemaVersion = 0
			case "chain":
				corpus.ChainID = 1
			case "catalog":
				corpus.CatalogSHA256 = strings.Repeat("0", 64)
			case "logs":
				corpus.Logs = nil
			case "actors":
				corpus.ActorReceipts = nil
			case "identity":
				corpus.Logs[0].Log.TxHash = common.Hash{}
			case "removed":
				corpus.Logs[0].Log.Removed = true
			case "duplicate":
				corpus.Logs = append(corpus.Logs, corpus.Logs[0])
			case "role":
				corpus.Logs[0].Expected.Role = "Router"
			case "args":
				corpus.Logs[0].Expected.Args = json.RawMessage(`{}`)
			}
			changed.ArchivePath = filepath.Join(t.TempDir(), "archive.json")
			changed.ArchiveSHA256 = writeMeasurementInput(t, changed.ArchivePath, corpus)
			report, err := decode.RunDecoderMeasurement(context.Background(), changed)
			if err == nil || report.Passed || report.Burst != nil {
				t.Fatalf("changed archive accepted: %+v %v", report, err)
			}
		})
	}
}

func TestDecoderArchiveRejectsMalformedActorAndOptionalReceipts(t *testing.T) {
	original := nativeArchiveMeasurement(t)
	for _, mode := range []string{"actor hex", "actor transaction", "actor coordinates", "actor expectation", "actor missing", "optional bounds", "optional receipt", "optional expectation"} {
		t.Run(mode, func(t *testing.T) {
			input := original
			corpus, err := decode.LoadDecoderMeasurementCorpus(input)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "actor hex":
				corpus.ActorReceipts[0].TransactionHex = "gg"
			case "actor transaction":
				corpus.ActorReceipts[0].TransactionHex = "01"
			case "actor coordinates":
				corpus.ActorReceipts[0].Receipt.TxHash = common.Hash{}
			case "actor expectation":
				for k, v := range corpus.ActorReceipts[0].ExpectedActors {
					v.Revision = "other"
					corpus.ActorReceipts[0].ExpectedActors[k] = v
				}
			case "actor missing":
				corpus.ActorReceipts[0].ExpectedActors = map[uint]decode.ActorContext{}
			case "optional bounds":
				corpus.OptionalReceipts = make([]decode.ArchivedOptionalReceipt, 65)
			case "optional receipt":
				corpus.OptionalReceipts = []decode.ArchivedOptionalReceipt{{}}
			case "optional expectation":
				corpus.OptionalReceipts = []decode.ArchivedOptionalReceipt{{Receipt: corpus.ActorReceipts[0].Receipt, ExpectedEffects: []decode.ArchivedOptionalEffect{{Family: "pecor"}}}}
			}
			input.ArchivePath = filepath.Join(t.TempDir(), "archive.json")
			input.ArchiveSHA256 = writeMeasurementInput(t, input.ArchivePath, corpus)
			report, err := decode.RunDecoderMeasurement(context.Background(), input)
			if err == nil || report.Passed || report.Burst != nil {
				t.Fatalf("invalid archived receipt admitted %+v %v", report, err)
			}
		})
	}
}

func TestArchivedDecoderQuarantinesBoundsAndArityAndRetainsOptionalAccounting(t *testing.T) {
	input := nativeArchiveMeasurement(t)
	corpus, err := decode.LoadDecoderMeasurementCorpus(input)
	if err != nil {
		t.Fatal(err)
	}
	malformed := corpus.Logs[0]
	malformed.Log.Index += 100
	malformed.Log.Topics = append([]common.Hash(nil), malformed.Log.Topics[:2]...)
	malformed.Expected.Status = "quarantined"
	malformed.Expected.Opaque = true
	malformed.Expected.FailureReason = "arity"
	malformed.Expected.Args = nil
	corpus.Logs = append(corpus.Logs, malformed)
	oversized := corpus.Logs[0]
	oversized.Log.Index += 101
	oversized.Log.Data = make([]byte, decode.MaxEnvelopeBytes+1)
	oversized.Expected = decode.ArchivedDecodeExpectation{Status: "quarantined", FailureReason: "bounds", Opaque: true}
	corpus.Logs = append(corpus.Logs, oversized)
	producer := repair995Load(t)
	var optional types.Receipt
	for key, response := range producer.Responses {
		if !strings.HasPrefix(key, "eth_getTransactionReceipt ") || len(response.Result) == 0 {
			continue
		}
		var candidate types.Receipt
		if err := json.Unmarshal(response.Result, &candidate); err != nil {
			continue
		}
		if candidate.Status != 1 || len(candidate.Logs) != 1 {
			continue
		}
		log := candidate.Logs[0]
		if log != nil && log.Address == producer.Markets[0].Token {
			optional = candidate
			break
		}
	}
	if optional.TxHash == (common.Hash{}) {
		t.Fatal("single-transfer producer receipt absent")
	}
	corpus.OptionalReceipts = []decode.ArchivedOptionalReceipt{{Receipt: optional, ExpectedEffects: []decode.ArchivedOptionalEffect{}, ExpectedUnmatched: []decode.RawIdentity{}}}
	input.ArchivePath = filepath.Join(t.TempDir(), "archive.json")
	input.ArchiveSHA256 = writeMeasurementInput(t, input.ArchivePath, corpus)
	report, run, err := decode.PrepareDecoderMeasurement(context.Background(), input)
	if err != nil {
		t.Fatalf("archive preparation: %+v %v", report, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	report, err = run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || report.Passed || report.ArchivedLogCount != 4 || report.Compatibility.OptionalReceiptCount != 1 || report.Burst == nil || report.Burst.PreservedQuarantined == 0 {
		t.Fatalf("bounded archive count=%d optional=%d burst=%+v err=%v", report.ArchivedLogCount, report.Compatibility.OptionalReceiptCount, report.Burst, err)
	}
	for _, gap := range report.Gaps {
		if gap == "archive_case_missing:over_envelope_quarantined" || gap == "archive_case_missing:malformed_arity_quarantined" {
			t.Fatalf("present admission case marked missing: %s", gap)
		}
	}
}

func TestReceiptActorsEnforceMarketAndComparisonAdmission(t *testing.T) {
	input := repair995Load(t)
	r := repair995Registry(t, input, true, true)
	for _, mode := range []string{"market bound", "market identity", "market duplicate", "comparison bound"} {
		t.Run(mode, func(t *testing.T) {
			tx, rec := repair995Bundle(t, input)
			markets := repair995Markets(input)
			switch mode {
			case "market bound":
				markets = make([]decode.ActorMarketBinding, 65)
			case "market identity":
				markets[0].Token = markets[0].Settlement
			case "market duplicate":
				markets = append(markets, markets[0])
			case "comparison bound":
				var swap *types.Log
				for _, l := range rec.Logs {
					if l.Address != decode.LaunchpadPrecompileAddress {
						continue
					}
					e, err := r.Decode(*l)
					if err != nil {
						t.Fatal(err)
					}
					if e.Family != nil && e.Family.EconomicFill {
						swap = l
						break
					}
				}
				if swap == nil {
					t.Fatal("economic source event absent")
				}
				rec.Logs = make([]*types.Log, 1025)
				for i := range rec.Logs {
					l := *swap
					l.Index = uint(i)
					rec.Logs[i] = &l
				}
			}
			out, err := r.DecodeTransactionReceipt(tx, rec, markets)
			if mode != "comparison bound" {
				if err == nil {
					t.Fatal("unbound actor market admitted")
				}
				return
			}
			if err != nil || len(out) != 1025 {
				t.Fatalf("comparison admission count=%d: %v", len(out), err)
			}
			for _, e := range out {
				if e.Actors == nil || e.Actors.Recipient != nil || len(e.Alerts) != 1 || e.Alerts[0] != "actor_unavailable:comparison_bound" {
					t.Fatal("comparison bound did not preserve unknown attribution")
				}
			}
		})
	}
}
