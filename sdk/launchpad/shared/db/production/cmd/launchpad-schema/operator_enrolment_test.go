package main

import (
	"path/filepath"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestOperatorEnrolmentRegistryBindings(t *testing.T) {
	bundle, err := loadBundle(filepath.Join(coordinatortest.RepositoryRoot(t), "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	migration, ok := bundle.catalog[450002]
	if !ok || migration.Service != "security/policy" || migration.Schema != "security" || len(migration.PredecessorIDs) != 1 || migration.PredecessorIDs[0] != 450001 {
		t.Fatal("operator enrolment migration has no policy predecessor")
	}
	if migration.InterfaceDigest != bundle.interfaceDigest || migration.ApprovalManifestDigest != bundle.registry.ApprovalDigest || approvalManifestDigest(bundle.registry) != bundle.registry.ApprovalDigest {
		t.Fatal("operator migration approval or grants binding differs")
	}
	if len(migration.SourceInputs) != 1 || migration.SourceInputs[0].Path != "services/launchpad/security/control/enrolment/enrolment.go" {
		t.Fatal("operator migration does not bind the enrolment producer")
	}
	for _, role := range []string{"operator_runtime", "operator_registrar"} {
		var present bool
		for _, manifest := range bundle.roles {
			if manifest.RuntimeRole == role && manifest.Service == "security/operator" {
				present = true
			}
		}
		if !present {
			t.Fatal("operator migration role is unregistered")
		}
	}
}
