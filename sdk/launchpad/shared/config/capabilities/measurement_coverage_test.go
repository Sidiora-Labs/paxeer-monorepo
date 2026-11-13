package capabilities

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

func TestMeasurementReportsRecordedPinnedCases(t *testing.T) {
	chain := repair38OpenChain(t, true)
	block := chain.peer.blocks[0]
	header := chain.peer.headers[block]
	request := chain.request(PlainQuote, block)
	request.Pool = chain.peer.epoch.Pool
	request.Recipient = chain.trader
	build := ConsumerBuildIdentity{
		BuildSHA256: strings.Repeat("a", 64), CandidateSHA256: strings.Repeat("b", 64),
		ManifestSHA256:       chain.engine.revision.ManifestSHA256,
		SourceManifestSHA256: chain.engine.manifest.SourceManifestSHA256,
		RevisionSHA256:       chain.engine.revision.RevisionSHA256,
		NetworkSHA256:        strings.Repeat("c", 64), Addresses: chain.engine.manifest.Addresses(),
	}
	plan := MeasurementPlan{
		SchemaVersion: 1, CandidateSHA256: build.CandidateSHA256,
		HarnessBuildSHA256: strings.Repeat("d", 64), AuthoritySHA256: strings.Repeat("e", 64),
		PolicySHA256: strings.Repeat("f", 64), GoBuild: build, FrontendBuild: build,
		SamplesPerCase: 1, Concurrency: 3, DeadlineMilliseconds: 60000,
		P95BudgetMicroseconds: 60000000, MaxErrorBasisPoints: 10000, MaxRawBytes: 64 << 20,
	}
	for _, kind := range measurementKinds {
		probe := MeasurementCase{Kind: kind, BlockHash: block, BlockNumber: header.Number.Uint64(), Request: request}
		if kind == valueQuotes {
			probe.OpticalRequest = request
			probe.OpticalRequest.Pool = chain.peer.epoch.Token
		}
		if kind == valueCreationFees {
			probe.ReceiptHashes = []common.Hash{{1}, {2}}
		}
		plan.Cases = append(plan.Cases, probe)
	}
	plan.PublicEndpoints = deployment.PublicEndpoints{
		RPC: "https://api1.mainnet-beta.paxeer.network", Explorer: "https://scan.example",
		AllowedRPCs: []string{"https://api1.mainnet-beta.paxeer.network"}, AllowedExplorers: []string{"https://scan.example"},
	}
	policy := deployment.Policy{Now: time.Now().UTC(), MaxAge: time.Hour, Timeout: 10 * time.Second}
	attestation, err := deployment.Attest(context.Background(), chain.reader, chain.engine.manifest, block, policy)
	if err != nil {
		t.Fatal(err)
	}
	export, err := deployment.ExportRevision(chain.engine.manifest, attestation, policy, plan.PublicEndpoints, chain.engine.revision, build.Addresses, build.Addresses)
	if err != nil {
		t.Fatal(err)
	}
	plan.GoBuild.NetworkSHA256 = export.NetworkSHA256
	plan.FrontendBuild.NetworkSHA256 = export.NetworkSHA256
	if err := plan.Validate(chain.engine); err != nil {
		t.Fatalf("recorded measurement plan: %v", err)
	}
	var raw bytes.Buffer
	report, err := chain.engine.Measure(context.Background(), plan, &raw)
	if err == nil || report.MeasurementPassed || report.CompletedSamples != len(measurementKinds) || len(report.Samples) != len(measurementKinds) || report.RawReadCount == 0 || report.RawBytes != int64(raw.Len()) || report.FailedSamples == 0 || len(report.Blockers) == 0 {
		t.Fatalf("recorded pinned cases: completed=%d reads=%d failed=%d blockers=%v err=%v", report.CompletedSamples, report.RawReadCount, report.FailedSamples, report.Blockers, err)
	}
	digest := sha256.Sum256(raw.Bytes())
	if report.RawSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatal("raw measurement digest did not bind captured reads")
	}
	for i, sample := range report.Samples {
		if sample.Index != i || sample.Kind != measurementKinds[i] || sample.ElapsedNanoseconds <= 0 {
			t.Fatalf("sample ordering or timing: %+v", sample)
		}
	}
	if !report.Samples[0].Passed || len(report.Coverage["D01_010.1"]) != 1 {
		t.Fatalf("approved public export was not measured: %+v", report.Samples[0])
	}
}

func TestMeasurementCaptureBindsPinnedReads(t *testing.T) {
	chain := repair38OpenChain(t, true)
	block := chain.peer.blocks[0]
	ctx := context.Background()
	var raw bytes.Buffer
	hash := sha256.New()
	capture := &measurementCapture{reader: chain.reader, writer: &raw, hash: hash, maxBytes: 1 << 20}
	id, err := capture.ChainID(ctx)
	if err != nil || id.Cmp(big.NewInt(125)) != 0 {
		t.Fatalf("captured chain identity: %v, %v", id, err)
	}
	header, err := capture.HeaderByReportedHash(ctx, block)
	if err != nil || header.Number.Cmp(chain.peer.headers[block].Number) != 0 {
		t.Fatalf("captured committed header: %+v, %v", header, err)
	}
	address := chain.peer.epoch.Pool
	code, err := capture.CodeAtHash(ctx, address, block)
	if err != nil || !bytes.Equal(code, chain.peer.codes[address]) {
		t.Fatalf("captured pool code: %x, %v", code, err)
	}
	price, err := capture.SuggestGasPrice(ctx)
	if err != nil || price.Cmp(big.NewInt(1_000_000_000)) != 0 {
		t.Fatalf("captured gas price: %v, %v", price, err)
	}
	call := ethereum.CallMsg{To: &address, Data: repair38Call(t, "getReserves()")}
	result, err := capture.CallContractAtHash(ctx, call, block)
	if err != nil || !bytes.Equal(result, chain.peer.calls[repair38Key(address, call.Data, block)]) {
		t.Fatalf("captured reserve call: %x, %v", result, err)
	}
	if _, err := capture.CodeAt(ctx, address, header.Number); err == nil {
		t.Fatal("unrecorded numeric block read accepted")
	}
	if _, err := capture.StorageAt(ctx, address, common.Hash{}, header.Number); err == nil {
		t.Fatal("unrecorded numeric storage read accepted")
	}
	if _, err := capture.CallContract(ctx, call, header.Number); err == nil {
		t.Fatal("unrecorded numeric call accepted")
	}
	if capture.sequence != 8 || capture.bytes != int64(raw.Len()) || raw.Len() == 0 {
		t.Fatalf("captured read count or size: %d, %d, %d", capture.sequence, capture.bytes, raw.Len())
	}
	want := sha256.Sum256(raw.Bytes())
	if !bytes.Equal(hash.Sum(nil), want[:]) {
		t.Fatal("raw read digest differs from captured bytes")
	}
}

func TestMeasurementCaptureStopsAtRawByteBudget(t *testing.T) {
	chain := repair38OpenChain(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var raw bytes.Buffer
	capture := &measurementCapture{reader: chain.reader, writer: &raw, hash: sha256.New(), maxBytes: 1, cancel: cancel}
	if _, err := capture.ChainID(ctx); err == nil {
		t.Fatal("raw byte budget admitted a full chain observation")
	} else {
		var issue deployment.Issue
		if !errors.As(err, &issue) || issue.Code != "measurement_raw_budget_exceeded" {
			t.Fatalf("bounded capture: %v", err)
		}
	}
	if ctx.Err() != context.Canceled || capture.sequence != 0 || capture.bytes != 0 || raw.Len() != 0 {
		t.Fatalf("capture after exhausted budget: sequence=%d bytes=%d output=%d context=%v", capture.sequence, capture.bytes, raw.Len(), ctx.Err())
	}
	if _, err := capture.ChainID(ctx); err == nil {
		t.Fatal("capture resumed after raw byte budget")
	}
	path := filepath.Join(t.TempDir(), "observations.ndjson")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	readOnly, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	writeContext, stop := context.WithCancel(context.Background())
	defer stop()
	writeCapture := &measurementCapture{reader: chain.reader, writer: readOnly, hash: sha256.New(), maxBytes: 1 << 20, cancel: stop}
	if _, err := writeCapture.ChainID(writeContext); err == nil {
		t.Fatal("read-only raw observation file accepted writes")
	} else {
		var issue deployment.Issue
		if !errors.As(err, &issue) || issue.Code != "measurement_capture_write_failed" {
			t.Fatalf("read-only capture: %v", err)
		}
	}
	if writeContext.Err() != context.Canceled || writeCapture.sequence != 0 || writeCapture.bytes != 0 {
		t.Fatalf("capture after write refusal: sequence=%d bytes=%d context=%v", writeCapture.sequence, writeCapture.bytes, writeContext.Err())
	}
	for _, tc := range []struct {
		err  error
		code string
	}{
		{context.Canceled, "measurement_canceled"},
		{context.DeadlineExceeded, "measurement_deadline_exceeded"},
		{errors.New("upstream unavailable"), "measurement_read_unavailable"},
	} {
		if issue := measurementIssue(tc.err); issue.Code != tc.code {
			t.Fatalf("measurement issue for %v = %s", tc.err, issue.Code)
		}
	}
}
