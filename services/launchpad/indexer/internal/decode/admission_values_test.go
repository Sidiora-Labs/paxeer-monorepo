package decode

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestRegistryRequiresCatalogIdentityAndCompleteProvenance(t *testing.T) {
	body, catalog := frozen(t)
	for _, mode := range []string{"JSON", "identity", "provenance", "duplicate", "ABI", "precompile", "enum"} {
		t.Run(mode, func(t *testing.T) {
			var c Catalog
			if err := json.Unmarshal(body, &c); err != nil {
				t.Fatal(err)
			}
			ds := []Deployment{}
			switch mode {
			case "identity":
				c.ChainID = 1
			case "provenance":
				c.Contracts[0].CompilerVersion = ""
			case "duplicate":
				c.Contracts = append(c.Contracts, c.Contracts[0])
			case "ABI":
				c.Contracts[0].ABI = json.RawMessage(`[{"type":"event","name":"Invalid","inputs":[{"type":"invalid"}]}]`)
				c.Contracts[0].ABIRevision = measurementDigestForAdmission(c.Contracts[0].ABI)
			case "precompile":
				ds = []Deployment{{Address: common.HexToAddress("0x123"), Role: LaunchpadPrecompileRole, ABIRevision: catalog.Contracts[0].ABIRevision}}
			case "enum":
				d, a := deployment(t, "EventEmitter", common.HexToAddress("0x123"))
				for _, e := range a.Events {
					d.EnumMax = map[string]map[string]uint64{e.Sig: {"absent": 1}}
					break
				}
				ds = []Deployment{d}
			}
			raw, err := json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "JSON" {
				raw = []byte("{")
			}
			if _, err = NewRegistry(raw, ds); err == nil {
				t.Fatal("unbound registry input admitted")
			}
		})
	}
}

func TestExactIndexedArgumentsRetainHashesAndRejectNoncanonicalWords(t *testing.T) {
	for _, tc := range []struct {
		kind     string
		word     common.Hash
		approved bool
		want     any
		bad      bool
	}{
		{"string", common.HexToHash("0x123"), false, common.HexToHash("0x123").Hex(), false},
		{"bool", common.HexToHash("0x2"), true, true, false},
		{"bool", common.HexToHash("0x2"), false, nil, true},
		{"address", common.HexToHash("0x100000000000000000000000000000000000000000000000000"), false, nil, true},
	} {
		t.Run(tc.kind+tc.word.Hex(), func(t *testing.T) {
			typ, err := abi.NewType(tc.kind, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			event := abi.NewEvent("Indexed", "Indexed", false, abi.Arguments{{Name: "value", Type: typ, Indexed: true}})
			approved := []string{}
			if tc.approved {
				approved = []string{"value"}
			}
			out, reason, err := exactArgs(event, []common.Hash{event.ID, tc.word}, nil, approved)
			if tc.bad {
				if err == nil || reason != "type" {
					t.Fatalf("noncanonical indexed word: %v %s", err, reason)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(out["value"], tc.want) {
				t.Fatalf("indexed value %+v: %v", out, err)
			}
			if _, reason, err = exactArgs(event, []common.Hash{event.ID, tc.word}, []byte{1}, approved); err == nil || reason != valueArity {
				t.Fatalf("unexpected event data: %v %s", err, reason)
			}
		})
	}
	for _, value := range []any{nil, (*string)(nil)} {
		if err := boundedValue(reflect.ValueOf(value)); err != nil {
			t.Fatal(err)
		}
	}
	if err := boundedValue(reflect.ValueOf(make([]byte, MaxFieldBytes+1))); err == nil {
		t.Fatal("unbounded bytes admitted")
	}
	typ, err := abi.NewType("uint256[2]", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := humanizeExact([2]*big.Int{big.NewInt(1), big.NewInt(2)}, typ); !reflect.DeepEqual(got, []any{"1", "2"}) {
		t.Fatalf("exact array %+v", got)
	}
}

func TestDecoderMeasurementOutputRejectsUnavailableDestinations(t *testing.T) {
	root := t.TempDir()
	if err := WriteDecoderMeasurement(filepath.Join(root, "report.json"), nil); err == nil {
		t.Fatal("empty report admitted")
	}
	report := &DecoderMeasurementReport{}
	for _, path := range []string{root, filepath.Join(root, "missing", "report.json")} {
		if err := WriteDecoderMeasurement(path, report); err == nil {
			t.Fatal("unavailable report destination admitted")
		}
	}
	if _, err := expectedArgs(nil); err == nil {
		t.Fatal("absent expectation admitted")
	}
	if _, err := expectedArgs(json.RawMessage("{")); err == nil {
		t.Fatal("malformed expectation admitted")
	}
	if err := os.WriteFile(filepath.Join(root, "input.json"), []byte("{} {}"), 0600); err != nil {
		t.Fatal(err)
	}
	var out any
	if _, err := measurementJSON(filepath.Join(root, "input.json"), measurementDigestForAdmission([]byte("{} {}")), &out); err == nil {
		t.Fatal("trailing measurement input admitted")
	}
	if _, err := DecoderCandidateSourceDigest(root); err == nil {
		t.Fatal("incomplete source tree admitted")
	}
}

func measurementDigestForAdmission(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestOptionalBatchRetainsDisabledEventsAndRejectsCoreFailures(t *testing.T) {
	d, a := deployment(t, "PECOR", common.HexToAddress("0x125"))
	d.Optional = true
	d.Enabled = false
	r := registry(t, d)
	var event abi.Event
	for _, e := range a.Events {
		event = e
		break
	}
	log, _ := syntheticLog(t, event, d.Address)
	out, err := r.DecodeOptionalBatch([]types.Log{log})
	if err != nil || len(out.Optional) != 1 || len(out.Failures) != 1 || len(out.Core) != 0 || out.Failures[0].Family != "pecor" {
		t.Fatalf("disabled optional event %+v: %v", out, err)
	}
	if _, err = (*Registry)(nil).DecodeOptionalBatch(nil); err == nil {
		t.Fatal("absent registry admitted")
	}
	core, coreABI := deployment(t, "SidioraERC20", common.HexToAddress("0x126"))
	r = registry(t, core)
	broken, _ := syntheticLog(t, coreABI.Events["Transfer"], core.Address)
	broken.Data = []byte{1}
	if _, err = r.DecodeOptionalBatch([]types.Log{broken}); err == nil {
		t.Fatal("malformed core event admitted")
	}
	ordinary, _ := syntheticLog(t, coreABI.Events["Transfer"], core.Address)
	conflict := ordinary
	conflict.Data = []byte{1}
	if _, err = r.DecodeOptionalBatch([]types.Log{ordinary, conflict}); err == nil {
		t.Fatal("conflicting raw identities admitted")
	}
	unknown := ordinary
	unknown.Topics = []common.Hash{common.HexToHash("0xfeed")}
	decoded, err := r.Decode(unknown)
	if err != nil || decoded.Status != "opaque" || decoded.FailureReason != valueUnknownSignature {
		t.Fatalf("unknown approved-emitter event %+v: %v", decoded, err)
	}
	ordinary.Removed = true
	decoded, err = r.Decode(ordinary)
	if err == nil || decoded.Status != valueQuarantined || decoded.FailureReason != "removed_log" {
		t.Fatalf("removed committed event %+v: %v", decoded, err)
	}
}

func TestDecoderCandidateDigestRejectsSymlinksAndMissingModuleInputs(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{"services/launchpad/indexer/internal/decode", "services/launchpad/indexer/internal/chainreader", "services/launchpad/indexer/internal/receipt", "sdk/launchpad/protocol", "sdk/launchpad/shared/accounting"} {
		if err := os.MkdirAll(filepath.Join(root, relative), 0700); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(root, "services/launchpad/indexer/internal/decode/current.go")
	if err := os.Symlink("missing.go", link); err != nil {
		t.Fatal(err)
	}
	if _, err := DecoderCandidateSourceDigest(root); err == nil {
		t.Fatal("symlink source admitted")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if _, err := DecoderCandidateSourceDigest(root); err == nil {
		t.Fatal("absent module inputs admitted")
	}
}
