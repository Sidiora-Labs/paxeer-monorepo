package source

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func copyParityInput(t *testing.T, input ParityInput) ParityInput {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var out ParityInput
	if err = json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestParityRejectsIncompleteAndMalformedSourceInputs(t *testing.T) {
	node := newCapacityChain(t)
	node.head.Store(102)
	live := repair997Open(t, fopts(node.server.URL))
	result, err := live.FetchCommitted(context.Background(), FetchOptions{FromBlock: 100, ToBlock: 102}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	left := ParityInput{SourceID: "rpc", TimestampUnit: "seconds", Result: result}
	for _, tc := range []struct {
		name string
		edit func(*ParityInput)
	}{
		{"source", func(p *ParityInput) { p.SourceID = "" }},
		{"completion", func(p *ParityInput) { p.Result.Complete = false }},
		{"manifest", func(p *ParityInput) { p.Result.ManifestDigest = "" }},
		{"unit", func(p *ParityInput) { p.TimestampUnit = "minutes" }},
		{"range start", func(p *ParityInput) { p.Result.CoveredRanges[0].FromHeight++ }},
		{"range end", func(p *ParityInput) { p.Result.CoveredRanges[0].ToHeight-- }},
		{"range missing", func(p *ParityInput) { p.Result.CoveredRanges = nil }},
		{"block count", func(p *ParityInput) { delete(p.Result.Blocks, 100) }},
		{"block identity", func(p *ParityInput) { b := p.Result.Blocks[100]; b.ChainID = 1; p.Result.Blocks[100] = b }},
		{"timestamp", func(p *ParityInput) { b := p.Result.Blocks[100]; b.TimestampSeconds = 0; p.Result.Blocks[100] = b }},
		{"timestamp map", func(p *ParityInput) { p.Result.BlockTimestamps[100]++ }},
		{"parent", func(p *ParityInput) {
			b := p.Result.Blocks[102]
			b.ParentHash = common.Hash{}
			p.Result.Blocks[102] = b
		}},
		{"fractional millis", func(p *ParityInput) { p.TimestampUnit = "milliseconds" }},
		{"removed", func(p *ParityInput) { p.Result.Logs[0].Removed = true }},
		{"emitter", func(p *ParityInput) { p.Result.Logs[0].Address = common.Address{} }},
		{"transaction index", func(p *ParityInput) { p.Result.Logs[0].TxIndex = 1 }},
		{"topics", func(p *ParityInput) { p.Result.Logs[0].Topics = make([]common.Hash, 5) }},
		{"oversize", func(p *ParityInput) { p.Result.Logs[0].Data = make([]byte, 65537) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := copyParityInput(t, left)
			tc.edit(&p)
			if err := validateParityInput(p, 100, 102); err == nil {
				t.Fatal("malformed parity input accepted")
			}
		})
	}
	right := copyParityInput(t, left)
	right.SourceID = "explorer"
	for _, bounds := range [][2]uint64{{102, 100}, {0, math.MaxUint64}, {0, 100000}} {
		if _, err = CompareParity(bounds[0], bounds[1], left, right); err == nil {
			t.Fatal("invalid parity bounds accepted")
		}
	}
	right.Result.ManifestDigest = "other"
	if _, err = CompareParity(100, 102, left, right); err == nil {
		t.Fatal("different manifests compared")
	}
}

func TestParityRetainsBoundedDisputesAndHistoricalMergeIdentity(t *testing.T) {
	node := newCapacityChain(t)
	node.head.Store(102)
	live := repair997Open(t, fopts(node.server.URL))
	result, err := live.FetchCommitted(context.Background(), FetchOptions{FromBlock: 100, ToBlock: 102}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	left := ParityInput{SourceID: "rpc", TimestampUnit: "seconds", Result: result}
	for _, mode := range []string{"left missing", "right missing", "duplicate", "payload", "block", "many disputes"} {
		t.Run(mode, func(t *testing.T) {
			a, b := copyParityInput(t, left), copyParityInput(t, left)
			b.SourceID = "explorer"
			switch mode {
			case "left missing":
				a.Result.Logs = nil
			case "right missing":
				b.Result.Logs = nil
			case "duplicate":
				a.Result.Logs = append(a.Result.Logs, a.Result.Logs[0])
			case "payload":
				b.Result.Logs[0].Data = []byte{2}
			case "block":
				v := b.Result.Blocks[102]
				v.TransactionCount = 1
				b.Result.Blocks[102] = v
			case "many disputes":
				a.Result.Logs = nil
				b.Result.Logs = nil
				for i := 0; i < 300; i++ {
					l := result.Logs[0]
					l.Index = uint(i)
					a.Result.Logs = append(a.Result.Logs, l)
				}
			}
			report, err := CompareParity(100, 102, a, b)
			if err != nil || report.Certified {
				t.Fatalf("parity report %+v %v", report, err)
			}
			switch mode {
			case "left missing", "right missing":
				if len(report.Missing) != 1 || len(report.Disputes) != 1 {
					t.Fatalf("missing identities %+v", report)
				}
			case "duplicate":
				if len(report.Duplicates) != 1 || report.Duplicates[0].Count != 2 {
					t.Fatalf("duplicate identity %+v", report)
				}
			case "payload":
				if len(report.Different) != 1 || report.Disputes[0].Left == nil || report.Disputes[0].Right == nil {
					t.Fatalf("payload dispute %+v", report)
				}
			case "block":
				if len(report.BlockDifferences) != 1 {
					t.Fatalf("block dispute %+v", report)
				}
			case "many disputes":
				if !report.DisputesTruncated || len(report.Disputes) != 256 || len(report.Missing) != 300 {
					t.Fatalf("dispute bounds %+v", report)
				}
			}
			if err = WriteParityReport(t.TempDir(), report); err != nil {
				t.Fatal(err)
			}
			if err = ValidateParityAdmission(&report, "fixture", "explorer"); err == nil {
				t.Fatal("disputed range admitted")
			}
		})
	}
	report, err := CompareParity(100, 102, left, ParityInput{SourceID: "explorer", TimestampUnit: "seconds", Result: result})
	if err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err = os.WriteFile(blocked, []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = WriteParityReport(filepath.Join(blocked, "directory"), report); err == nil {
		t.Fatal("report directory collision accepted")
	}
	bad := report
	bad.ReportID = strings.Repeat("z", 64)
	if err = WriteParityReport(t.TempDir(), bad); err == nil {
		t.Fatal("invalid report identity accepted")
	}
	for _, mode := range []string{"incomplete", "gap", "duplicate block", "origin conflict", "explorer identity", "logs count", "logs bytes"} {
		t.Run(mode, func(t *testing.T) {
			var out, page FetchResult
			out = FetchResult{ManifestDigest: "fixture", Blocks: map[uint64]BlockRecord{}, BlockTimestamps: map[uint64]int64{}, TxFromMap: map[string]string{}}
			page = copyParityInput(t, left).Result
			switch mode {
			case "incomplete":
				page.Complete = false
			case "gap":
				out.CoveredRanges = []CoveredRange{{1, 98}}
			case "duplicate block":
				out.Blocks[100] = page.Blocks[100]
			case "origin conflict":
				page.TxFromMap[strings.ToLower(page.Logs[0].TxHash.Hex())] = "original"
				out.TxFromMap[strings.ToLower(page.Logs[0].TxHash.Hex())] = "different"
			case "explorer identity":
				out.Explorer = &ExplorerProvenance{SourceID: "a"}
				page.Explorer = &ExplorerProvenance{SourceID: "b"}
			case "logs count":
				out.Logs = make([]ethtypes.Log, 131073)
			case "logs bytes":
				page.Logs[0].Data = make([]byte, 128<<20)
			}
			if err := mergeFetch(&out, page); err == nil {
				t.Fatal("conflicting historical page merged")
			}
		})
	}
}
