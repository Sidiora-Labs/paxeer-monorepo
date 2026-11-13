package capabilities

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
)

func TestMeasurementCasesRejectUnboundInputs(t *testing.T) {
	for _, mode := range []string{"pin", "export addresses", "export network", "creation paths", "unknown case"} {
		t.Run(mode, func(t *testing.T) {
			chain := repair38OpenChain(t, true)
			block := chain.peer.blocks[1]
			probe := MeasurementCase{Kind: valueExports, BlockHash: block, BlockNumber: chain.peer.headers[block].Number.Uint64()}
			plan := MeasurementPlan{
				GoBuild:       ConsumerBuildIdentity{Addresses: chain.engine.manifest.Addresses()},
				FrontendBuild: ConsumerBuildIdentity{Addresses: chain.engine.manifest.Addresses()},
				PublicEndpoints: deployment.PublicEndpoints{
					RPC: "https://api1.mainnet-beta.paxeer.network", Explorer: "https://scan.example",
					AllowedRPCs: []string{"https://api1.mainnet-beta.paxeer.network"}, AllowedExplorers: []string{"https://scan.example"},
				},
			}
			want := ""
			switch mode {
			case "pin":
				probe.BlockHash = common.Hash{1}
				want = "measurement_canonical_pin_unverified"
			case "export addresses":
				plan.FrontendBuild.Addresses[valueRouter] = chain.engine.manifest.Roles[valueQuoter].Address
			case "export network":
				want = "consumer_network_identity_mismatch"
			case "creation paths":
				probe.Kind = valueCreationFees
				want = "creation_paths_incomplete"
			case "unknown case":
				probe.Kind = "unknown"
				want = "measurement_case_unknown"
			}
			var raw bytes.Buffer
			capture := &measurementCapture{reader: chain.reader, writer: &raw, hash: sha256.New(), maxBytes: 8 << 20}
			sample := chain.engine.measureCase(context.Background(), plan, probe, capture)
			if sample.Passed || sample.Issue == nil || sample.ElapsedNanoseconds <= 0 || capture.sequence == 0 {
				t.Fatalf("unbound measurement input: %+v reads=%d", sample, capture.sequence)
			}
			if want != "" && sample.Issue.Code != want {
				t.Fatalf("measurement rejection: got %s want %s", sample.Issue.Code, want)
			}
			if mode == "export addresses" && !strings.Contains(sample.Issue.Error(), valueRouter) {
				t.Fatalf("address rejection lost role: %v", sample.Issue)
			}
		})
	}
}

func TestPinnedQuoteRejectsInvalidContextAndTuple(t *testing.T) {
	for _, mode := range []string{"amount", "context", "tuple", "zero output", "read unavailable"} {
		t.Run(mode, func(t *testing.T) {
			chain := repair38OpenChain(t, true)
			block, pool, quoter := chain.peer.blocks[1], chain.peer.epoch.Pool, chain.peer.address(valueQuoter)
			amount := "1000000"
			want := ""
			switch mode {
			case "amount":
				amount = "-1"
				want = "amount_invalid"
			case "context":
				pool = common.Address{}
				want = "quote_context_invalid"
			case "tuple":
				chain.peer.override(quoter, "quoteExactInput(address,uint256,bool)", []byte{1}, pool, big.NewInt(1000000), true)
				want = "quote_tuple_malformed"
			case "zero output":
				chain.peer.override(quoter, "quoteExactInput(address,uint256,bool)", make([]byte, 96), pool, big.NewInt(1000000), true)
			case "read unavailable":
				block = common.Hash{1}
				want = "quote_read_failed"
			}
			quote, err := ReadPinnedQuote(context.Background(), chain.reader, quoter, pool, chain.trader, amount, true, block)
			if err == nil || quote != (ContractQuote{}) {
				t.Fatalf("invalid quote admitted: %+v %v", quote, err)
			}
			if mode == "zero output" {
				var unavailable Unavailable
				if !errors.As(err, &unavailable) || unavailable.ReasonCode != UnexecutableQuote {
					t.Fatalf("zero output classification: %v", err)
				}
			} else {
				var issue deployment.Issue
				if !errors.As(err, &issue) || issue.Code != want {
					t.Fatalf("quote rejection: got %v want %s", err, want)
				}
			}
		})
	}
}
