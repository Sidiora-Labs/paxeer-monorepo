package capabilities

import (
	"bytes"
	"context"
	"crypto/sha256"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func pinnedMeasurementPlan(t *testing.T, chain *repair38Chain) MeasurementPlan {
	t.Helper()
	build := ConsumerBuildIdentity{BuildSHA256: strings.Repeat("a", 64), CandidateSHA256: strings.Repeat("b", 64), ManifestSHA256: chain.engine.revision.ManifestSHA256, SourceManifestSHA256: chain.engine.manifest.SourceManifestSHA256, RevisionSHA256: chain.engine.revision.RevisionSHA256, NetworkSHA256: strings.Repeat("c", 64), Addresses: chain.engine.manifest.Addresses()}
	plan := MeasurementPlan{SchemaVersion: 1, CandidateSHA256: build.CandidateSHA256, HarnessBuildSHA256: strings.Repeat("d", 64), AuthoritySHA256: strings.Repeat("e", 64), PolicySHA256: strings.Repeat("f", 64), GoBuild: build, FrontendBuild: build, SamplesPerCase: 1, Concurrency: 1, DeadlineMilliseconds: 60000, P95BudgetMicroseconds: 60000000, MaxErrorBasisPoints: 0, MaxRawBytes: 64 << 20}
	plan.PublicEndpoints = deployment.PublicEndpoints{RPC: "https://api1.mainnet-beta.paxeer.network", Explorer: "https://scan.example", AllowedRPCs: []string{"https://api1.mainnet-beta.paxeer.network"}, AllowedExplorers: []string{"https://scan.example"}}
	block := chain.peer.blocks[1]
	for _, kind := range measurementKinds {
		request := chain.request(PlainQuote, block)
		request.Pool = chain.peer.epoch.Pool
		probe := MeasurementCase{Kind: kind, BlockHash: block, BlockNumber: chain.peer.headers[block].Number.Uint64(), Request: request}
		probe.OpticalRequest = request
		probe.OpticalRequest.Pool = chain.peer.epoch.Token
		probe.ReceiptHashes = []common.Hash{{1}, {2}}
		plan.Cases = append(plan.Cases, probe)
	}
	att, err := deployment.Attest(context.Background(), chain.reader, chain.engine.manifest, block, deployment.Policy{Now: time.Now(), MaxAge: time.Hour, Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	export, err := deployment.ExportRevision(chain.engine.manifest, att, deployment.Policy{Now: time.Now(), MaxAge: time.Hour, Timeout: 10 * time.Second}, plan.PublicEndpoints, chain.engine.revision, build.Addresses, build.Addresses)
	if err != nil {
		t.Fatal(err)
	}
	plan.GoBuild.NetworkSHA256 = export.NetworkSHA256
	plan.FrontendBuild.NetworkSHA256 = export.NetworkSHA256
	return plan
}

func TestMeasurementCasesBindPinnedFixtureConditions(t *testing.T) {
	for _, kind := range []string{valueExports, valueNoOracle, valuePrivateExport, valueZeroRealSell, valueQuotes, valueQuoterRevoked, valuePendingGovernance, valueUnknownImplementation, "unknown"} {
		t.Run(kind, func(t *testing.T) {
			native := kind != valuePendingGovernance && kind != valueUnknownImplementation
			chain := repair38OpenChain(t, native)
			plan := pinnedMeasurementPlan(t, chain)
			probe := plan.Cases[0]
			probe.Kind = kind
			expected := ""
			switch kind {
			case valuePendingGovernance:
				chain.peer.override(chain.peer.address("PoolBeacon"), "owner()", repair38Address(chain.peer.epoch.Trader))
				chain.peer.override(chain.peer.address("PoolBeacon"), "pendingOwner()", repair38Address(common.HexToAddress(chain.engine.manifest.Governance.ExpectedBeaconOwner)))
			case valueUnknownImplementation:
				chain.peer.mu.Lock()
				chain.peer.codes[common.HexToAddress(chain.engine.manifest.Roles["Quoter"].ImplementationAddress)] = []byte{0}
				chain.peer.mu.Unlock()
			case valueQuoterRevoked:
				chain.peer.mu.Lock()
				chain.peer.codes[chain.peer.address("Quoter")] = []byte{0}
				chain.peer.mu.Unlock()
			case valueZeroRealSell:
				expected = "quote_read_failed"
				probe.Request.IsBuy = false
				chain.peer.override(chain.peer.address("Quoter"), "quoteExactInput(address,uint256,bool)", repair38Tuple(t, []string{"1000000", "1", "0"}), probe.Request.Pool, big.NewInt(1000000), false)
			case valueQuotes:
				expected = "optical_quote_not_typed"
			case "unknown":
				expected = "measurement_case_unknown"
			}
			var raw bytes.Buffer
			capture := &measurementCapture{reader: chain.reader, writer: &raw, hash: sha256.New(), maxBytes: 64 << 20}
			sample := chain.engine.measureCase(context.Background(), plan, probe, capture)
			if expected == "" {
				if !sample.Passed || sample.Issue != nil {
					t.Fatalf("pinned condition %s: %+v", kind, sample)
				}
			} else if sample.Passed || sample.Issue == nil || sample.Issue.Code != expected {
				t.Fatalf("pinned refusal %s: %+v", kind, sample)
			}
			if sample.ElapsedNanoseconds <= 0 || capture.sequence == 0 || raw.Len() == 0 {
				t.Fatal("measurement omitted raw observations")
			}
		})
	}
}

func TestMeasurementPlanChecksConsumerAndCaseBindings(t *testing.T) {
	chain := repair38OpenChain(t, true)
	base := pinnedMeasurementPlan(t, chain)
	for name, edit := range map[string]func(*MeasurementPlan){
		"consumer build":   func(p *MeasurementPlan) { p.GoBuild.BuildSHA256 = "invalid" },
		"consumer address": func(p *MeasurementPlan) { p.GoBuild.Addresses = map[string]string{} },
		"consumer network": func(p *MeasurementPlan) { p.FrontendBuild.NetworkSHA256 = strings.Repeat("0", 64) },
		"unknown case":     func(p *MeasurementPlan) { p.Cases[0].Kind = "unknown" },
		"quote context": func(p *MeasurementPlan) {
			for i := range p.Cases {
				if p.Cases[i].Kind == valueNoOracle {
					p.Cases[i].Request.Pool = common.Address{}
				}
			}
		},
		"optical context": func(p *MeasurementPlan) {
			for i := range p.Cases {
				if p.Cases[i].Kind == valueQuotes {
					p.Cases[i].OpticalRequest = p.Cases[i].Request
				}
			}
		},
		"receipt context": func(p *MeasurementPlan) {
			for i := range p.Cases {
				if p.Cases[i].Kind == valueCreationFees {
					p.Cases[i].ReceiptHashes = nil
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			candidate.Cases = append([]MeasurementCase(nil), base.Cases...)
			edit(&candidate)
			if err := candidate.Validate(chain.engine); err == nil {
				t.Fatal("unbound measurement admitted")
			}
		})
	}
	for _, wire := range []string{`[1,]`, `{"a":}`, `{"a":{"b":1,"b":2}}`, `{"a":[`, `{"a":1} {}`, `"unterminated`} {
		if err := measurementUniqueJSON([]byte(wire)); err == nil {
			t.Fatal("malformed measurement JSON admitted")
		}
	}
	for _, wire := range []string{`{"a":[1,true,null,{"b":2}]}`, `[1]`} {
		if err := measurementUniqueJSON([]byte(wire)); err != nil {
			t.Fatal(err)
		}
	}
	engine := *chain.engine
	engine.reader = pinnedBoundaryReader{PinnedRPCReader: chain.reader, address: chain.peer.epoch.Pool, selector: crypto.Keccak256([]byte("getReserves()"))[:4]}
	var raw bytes.Buffer
	report, err := engine.Measure(context.Background(), base, &raw)
	if err == nil || report.MeasurementPassed || report.FailedSamples == 0 || len(report.Blockers) == 0 {
		t.Fatalf("failed read measurement qualified: %+v %v", report, err)
	}
}
