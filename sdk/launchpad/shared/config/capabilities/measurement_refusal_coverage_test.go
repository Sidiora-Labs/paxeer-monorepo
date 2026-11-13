package capabilities

import (
	"bytes"
	"context"
	"crypto/sha256"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestMeasurementCasesRejectUnobservedConditions(t *testing.T) {
	chain := repair38OpenChain(t, true)
	base := pinnedMeasurementPlan(t, chain)
	for _, mode := range []string{"pin", "pending governance", "network export", "network identity", "quote without oracle", "quote request", "virtual sell", "ordinary quote", "optical request", "creation receipt", "revocation"} {
		t.Run(mode, func(t *testing.T) {
			plan := base
			engine := *chain.engine
			probe := base.Cases[0]
			want := ""
			switch mode {
			case "pin":
				probe.BlockHash = common.Hash{1}
				want = "measurement_canonical_pin_unverified"
			case "pending governance":
				probe.Kind = valuePendingGovernance
				want = "pending_governance_fixture_not_observed"
			case "network export":
				probe.Kind = valueExports
				plan.PublicEndpoints.RPC = "http://localhost"
				want = "public_endpoint_not_approved"
			case "network identity":
				probe.Kind = valueExports
				plan.GoBuild.NetworkSHA256 = strings.Repeat("0", 64)
				want = "consumer_network_identity_mismatch"
			case "quote without oracle":
				probe.Kind = valueNoOracle
				probe.Request.Pool = common.Address{}
				want = "core_quote_without_oracle_unavailable"
			case "quote request":
				probe.Kind = valueQuotes
				probe.Request.Trader = common.Address{}
				want = "preflight_context_invalid"
			case "virtual sell":
				probe.Kind = valueZeroRealSell
				want = "virtual_liquidity_sell_not_blocked"
			case "ordinary quote":
				probe.Kind = valueQuotes
				probe.Request.IsBuy = false
				want = "quote_read_failed"
			case "optical request":
				probe.Kind = valueQuotes
				probe.OpticalRequest.Trader = common.Address{}
				want = "preflight_context_invalid"
			case "creation receipt":
				probe.Kind = valueCreationFees
				probe.ReceiptHashes = []common.Hash{{1}}
				want = "creation_fee_reconciliation_unverified"
			case "revocation":
				probe.Kind = valueQuoterRevoked
				want = "dependency_revocation_not_isolated"
			}
			var raw bytes.Buffer
			capture := &measurementCapture{reader: chain.reader, writer: &raw, hash: sha256.New(), maxBytes: 64 << 20}
			sample := engine.measureCase(context.Background(), plan, probe, capture)
			if sample.Passed || sample.Issue == nil || sample.Issue.Code != want {
				t.Fatalf("condition %s: %+v", mode, sample)
			}
			if raw.Len() == 0 || capture.sequence == 0 {
				t.Fatal("refused condition omitted observations")
			}
		})
	}
}
