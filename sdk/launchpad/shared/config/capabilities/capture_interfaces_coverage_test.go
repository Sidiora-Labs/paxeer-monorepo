package capabilities

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type requiredMeasurementReader struct{ deployment.Reader }

func TestMeasurementRequiresOptionalReaderInterfaces(t *testing.T) {
	chain := repair38OpenChain(t, true)
	ctx := context.Background()
	block := chain.peer.blocks[1]
	address := chain.peer.epoch.Pool
	var raw bytes.Buffer
	capture := &measurementCapture{reader: requiredMeasurementReader{chain.reader}, writer: &raw, hash: sha256.New(), maxBytes: 1 << 20}
	for name, call := range map[string]func() error{
		"reported header":  func() error { _, e := capture.HeaderByReportedHash(ctx, block); return e },
		"canonical header": func() error { _, _, e := capture.ReportedHeaderByNumber(ctx, big.NewInt(1)); return e },
		"numeric code":     func() error { _, e := capture.CodeAt(ctx, address, big.NewInt(1)); return e },
		"numeric storage":  func() error { _, e := capture.StorageAt(ctx, address, common.Hash{}, big.NewInt(1)); return e },
		"numeric call": func() error {
			_, e := capture.CallContract(ctx, ethereum.CallMsg{To: &address}, big.NewInt(1))
			return e
		},
		"gas":        func() error { _, e := capture.EstimateGasAtHash(ctx, ethereum.CallMsg{To: &address}, block); return e },
		"price":      func() error { _, e := capture.SuggestGasPrice(ctx); return e },
		"full block": func() error { _, e := capture.ReportedBlockByHash(ctx, block); return e },
		"receipt":    func() error { _, e := capture.TransactionReceipt(ctx, block); return e },
	} {
		t.Run(name, func(t *testing.T) {
			var issue deployment.Issue
			if e := call(); !errors.As(e, &issue) || !strings.HasSuffix(issue.Code, "_missing") {
				t.Fatalf("optional interface: %v", e)
			}
		})
	}
	if capture.sequence != 9 {
		t.Fatalf("missing interface observations: %d", capture.sequence)
	}
	decoder := json.NewDecoder(&raw)
	for i := 0; i < 9; i++ {
		var record rawMeasurement
		if err := decoder.Decode(&record); err != nil || record.Issue == nil || record.Sequence != uint64(i+1) {
			t.Fatalf("raw interface refusal %+v %v", record, err)
		}
	}
	engine := *chain.engine
	engine.reader = requiredMeasurementReader{chain.reader}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	plan := pinnedMeasurementPlan(t, chain)
	if _, err := engine.SerialExport(cancelled, block, plan.PublicEndpoints, plan.GoBuild.Addresses, plan.FrontendBuild.Addresses); err == nil {
		t.Fatal("cancelled export admitted")
	}
	engine.reader = chain.reader
	exported, err := engine.SerialExport(ctx, block, plan.PublicEndpoints, plan.GoBuild.Addresses, plan.FrontendBuild.Addresses)
	if err != nil || exported.NetworkSHA256 != plan.GoBuild.NetworkSHA256 {
		t.Fatalf("serial export %+v %v", exported, err)
	}
	var absent *PreflightEngine
	if _, err := absent.SerialExport(ctx, block, plan.PublicEndpoints, nil, nil); err == nil {
		t.Fatal("absent export engine admitted")
	}
	if _, err := absent.Preflight(ctx, PreflightRequest{}); err == nil {
		t.Fatal("absent preflight engine admitted")
	}
	if _, err := absent.Revalidate(ctx, Preflight{}, PreflightRequest{}); err == nil {
		t.Fatal("absent revalidation engine admitted")
	}
	response := httptest.NewRecorder()
	engine.Handler(PlainQuote).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != 405 || response.Header().Get("Allow") != "POST" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("method response %+v", response)
	}
	for _, mode := range []string{"runtime", "manifest", "interface"} {
		manifest, runtime, serial := chain.engine.manifest, "0x"+strings.Repeat("a", 64), chain.engine.revision.SerialInterfaceSHA256
		switch mode {
		case "runtime":
			runtime = "0x" + strings.Repeat("z", 64)
		case "manifest":
			manifest.ChainID = 1
		case "interface":
			serial = "invalid"
		}
		if _, err := NewPreflightEngine(chain.reader, manifest, runtime, serial, time.Hour, time.Second); err == nil {
			t.Fatal("invalid engine configuration admitted")
		}
	}
}

func TestMeasurementCapturesBlockEncodingAndEncodingRefusal(t *testing.T) {
	chain := repair38OpenChain(t, true)
	ctx := context.Background()
	var raw bytes.Buffer
	capture := &measurementCapture{reader: chain.reader, writer: &raw, hash: sha256.New(), maxBytes: 1 << 20}
	block := types.NewBlockWithHeader(chain.peer.headers[chain.peer.blocks[1]])
	got, err := captureRead(ctx, capture, "block", nil, func() (*types.Block, error) { return block, nil })
	if err != nil || got.Hash() != block.Hash() {
		t.Fatalf("captured block %v %v", got, err)
	}
	var record rawMeasurement
	if err := json.Unmarshal(bytes.TrimSpace(raw.Bytes()), &record); err != nil || record.Encoding != "ethereum_block_rlp_base64" || record.Sequence != 1 || len(record.Result) == 0 {
		t.Fatalf("encoded block %+v %v", record, err)
	}
	failed := &measurementCapture{reader: chain.reader, writer: &raw, hash: sha256.New(), maxBytes: 1 << 20}
	if _, err := captureRead(ctx, failed, "non-json", make(chan int), func() (int, error) { return 1, nil }); err == nil || failed.err == nil || failed.sequence != 0 {
		t.Fatalf("unserializable capture admitted: %v", err)
	}
	request := chain.request(PlainQuote, chain.peer.blocks[1])
	request.Pool = chain.peer.epoch.Pool
	prior, err := chain.engine.Preflight(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"request", "revision", "observation", "fee", "cancellation"} {
		candidate := prior
		callCtx := ctx
		switch mode {
		case "request":
			candidate.RequestSHA256 = strings.Repeat("0", 64)
		case "revision":
			candidate.Revision = deployment.Revision{}
		case "observation":
			candidate.ObservedRevisionSHA256 = strings.Repeat("0", 64)
		case "fee":
			candidate.CreationCost = &CreationCost{ProtocolFeeRaw: "1"}
		case "cancellation":
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			callCtx = cancelled
		}
		if _, err := chain.engine.Revalidate(callCtx, candidate, request); err == nil {
			t.Fatalf("changed %s revalidation admitted", mode)
		}
	}
}
