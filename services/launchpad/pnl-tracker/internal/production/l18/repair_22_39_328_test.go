package l18

import (
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

func TestRepair_22_39_328(t *testing.T) {
	current := deployment.Current()
	if ManifestDigest != current.SourceManifestSHA256 {
		t.Fatalf("tracker manifest digest = %s, current deployment = %s", ManifestDigest, current.SourceManifestSHA256)
	}
	for _, capability := range []Capability{StrategyCapability(0, true), SharingCapability(), RewardSettlementCapability()} {
		if capability.DeploymentVersion != current.SourceManifestSHA256 {
			t.Fatalf("capability retained a stale manifest digest: %+v", capability)
		}
	}
}
