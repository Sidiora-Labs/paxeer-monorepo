package internaltest

import (
	"encoding/json"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestRewardRegistryFixtureBindsReaderAndApproval(t *testing.T) {
	source, approval, reader := RewardRegistryFixture(t, coordinatortest.RepositoryRoot(t))
	registry, err := production.LoadMigrationRegistry(source, production.Checksum(source))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := registry.ValidateOrder()
	if err != nil {
		t.Fatal(err)
	}
	var approved struct {
		InterfaceDigest string `json:"interface_digest"`
	}
	if err = json.Unmarshal(approval, &approved); err != nil {
		t.Fatal(err)
	}
	if len(reader) == 0 || approved.InterfaceDigest != production.Checksum(reader) {
		t.Fatal("reward approval does not bind the interface reader")
	}
	if registry.ApprovalDigest != production.Checksum(approval) {
		t.Fatal("migration registry does not bind the reward approval")
	}
	manifest, ok := catalog[180001]
	if !ok || manifest.ApprovalManifestDigest != registry.ApprovalDigest || manifest.InterfaceDigest != approved.InterfaceDigest {
		t.Fatalf("reward migration binding = %+v, present = %v", manifest, ok)
	}
}
