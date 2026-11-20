package main

import (
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
)

func TestPoolRuntimeOptionUsesManifestAcceptance(t *testing.T) {
	manifest := deployment.Current()
	factory := manifest.Roles["SidioraFactory"]
	for _, value := range []string{"", factory.RuntimeHash, factory.StagedRuntimeHash} {
		got, err := poolRuntimeHash(manifest, value)
		if err != nil || got != common.HexToHash(value) {
			t.Fatalf("accepted pool runtime %q: %s %v", value, got, err)
		}
	}
	for _, value := range []string{common.HexToHash("0x01").Hex(), "invalid", "0x01"} {
		_, err := poolRuntimeHash(manifest, value)
		if err == nil || !strings.Contains(err.Error(), value) || !strings.Contains(err.Error(), factory.RuntimeHash) || !strings.Contains(err.Error(), factory.StagedRuntimeHash) {
			t.Fatalf("unaccepted runtime error must identify supplied and manifest hashes: %v", err)
		}
		if err := invokeRuntimeDocuments(t, "-max-delivery-lag-ms", "60000", "-pool-runtime", value); err == nil || !strings.Contains(err.Error(), value) || !strings.Contains(err.Error(), factory.RuntimeHash) {
			t.Fatalf("command accepted incompatible pool runtime %q: %v", value, err)
		}
	}
	factory.StagedUpgrade = ""
	manifest.Roles["SidioraFactory"] = factory
	if _, err := poolRuntimeHash(manifest, factory.StagedRuntimeHash); err == nil {
		t.Fatal("staged hash accepted without a staged upgrade")
	}
	factory.StagedRuntimeHash = ""
	manifest.Roles["SidioraFactory"] = factory
	if _, err := poolRuntimeHash(manifest, factory.RuntimeHash); err != nil {
		t.Fatal(err)
	}
}

func TestPoolRuntimeOptionRequiresLegacyHash(t *testing.T) {
	manifest, ok := deployment.Approved(32226215)
	if !ok {
		t.Fatal("approved legacy deployment unavailable")
	}
	for _, value := range []string{"", "invalid", "0x01", (common.Hash{}).Hex(), "0x" + strings.Repeat("ab", 33)} {
		if _, err := poolRuntimeHash(manifest, value); err == nil {
			t.Fatalf("invalid legacy runtime %q accepted", value)
		}
	}
	value := common.HexToHash("0xa9b304d6d0aa562f283b45978b84ced8840a9331f8ac946742670b1b02a2872d")
	if got, err := poolRuntimeHash(manifest, value.Hex()); err != nil || got != value {
		t.Fatalf("legacy runtime %s: %s %v", value, got, err)
	}
}
