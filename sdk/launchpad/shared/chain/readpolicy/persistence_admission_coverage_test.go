package readpolicy

import (
	"context"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestMeasurementFinalizationRequiresReadableRawCapture(t *testing.T) {
	for _, mode := range []string{"missing raw", "raw directory", "histogram bound", "empty histogram", "request capacity"} {
		t.Run(mode, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "capture")
			capture, err := NewMeasurementCapture(directory, measuredTestBinding(t), 1000000, 1<<30)
			if err != nil {
				t.Fatal(err)
			}
			sample := MeasurementSample{RequestID: "request", Scenario: "read", ChainID: "125", ObservedUTCNS: 1, FinishedUTCNS: 2, ElapsedNS: 1, Outcome: "success"}
			switch mode {
			case "histogram bound":
				sample.ElapsedNS = int64(31 * time.Second)
			case "request capacity":
				for i := 0; i < 1000000; i++ {
					capture.requestIDs[big.NewInt(int64(i)).String()] = struct{}{}
				}
			}
			err = capture.Append(sample)
			if mode == "histogram bound" || mode == "request capacity" {
				if err == nil {
					t.Fatal("unbounded observation admitted")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			raw := filepath.Join(directory, "chain-samples.ndjson")
			if mode == "missing raw" || mode == "raw directory" {
				if err := os.Remove(raw); err != nil {
					t.Fatal(err)
				}
				if mode == "raw directory" {
					if err := os.Mkdir(raw, 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			if mode == "empty histogram" {
				capture.histograms["empty"] = &latencyHistogram{}
			}
			result, err := capture.Finish(true, "")
			if mode == "request capacity" {
				if err != nil || result.LaneHarnessPassed {
					t.Fatalf("unbounded capture: %+v %v", result, err)
				}
			} else if err == nil {
				t.Fatal("incomplete raw capture finalized")
			}
		})
	}
}

func TestReceiptStateRejectsForeignAndUnorderedLogs(t *testing.T) {
	state, err := Submitted(common.Hash{1}, "deployment")
	if err != nil {
		t.Fatal(err)
	}
	header := &types.Header{Number: big.NewInt(1)}
	hash := common.Hash{2}
	base := types.Log{TxHash: state.SubmittedHash, BlockHash: hash, BlockNumber: 1, TxIndex: 0, Index: 1}
	for _, mode := range []string{"foreign log", "unordered logs"} {
		receipt := &types.Receipt{Status: 1, TxHash: state.SubmittedHash, BlockHash: hash, BlockNumber: big.NewInt(1), Logs: []*types.Log{&base}}
		if mode == "foreign log" {
			changed := base
			changed.Removed = true
			receipt.Logs = []*types.Log{&changed}
		} else {
			receipt.Logs = append(receipt.Logs, &base)
		}
		if _, err := state.WithReceipt(receipt, header, hash); err == nil {
			t.Fatal("unbound receipt logs admitted")
		}
	}
	for _, bounds := range [][3]uint64{{2, 1, 1}, {1, 1, 0}, {0, 1000000, 1}} {
		if err := WalkRanges(context.Background(), bounds[0], bounds[1], bounds[2], func(context.Context, Range) error { t.Fatal("invalid range read"); return nil }, func(error) bool { return false }); err == nil {
			t.Fatal("invalid range admitted")
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := WalkRanges(cancelled, 1, 1, 1, func(context.Context, Range) error { t.Fatal("cancelled range read"); return nil }, func(error) bool { return false }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	boundary := errors.New("range limit")
	if err := WalkRanges(context.Background(), 1, 2, 2, func(context.Context, Range) error { return boundary }, func(err error) bool { return errors.Is(err, boundary) }); !errors.Is(err, boundary) {
		t.Fatal(err)
	}
}
