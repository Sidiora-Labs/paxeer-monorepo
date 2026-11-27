package l18

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
)

func rewardHistoryPlanInputs(t *testing.T) (production.MigrationRegistry, production.RoleManifest, rewardHistoryCatalog) {
	t.Helper()
	var registry production.MigrationRegistry
	var roles struct {
		Roles []production.RoleManifest `json:"roles"`
	}
	for name, dst := range map[string]any{"registry.json": &registry, "grants.json": &roles} {
		raw, err := os.ReadFile(filepath.Join(rewardHistoryRoot(), "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, dst); err != nil {
			t.Fatal(err)
		}
	}
	var transcript rewardHistoryCatalog
	rewardHistoryLoad(t, "catalog.json", &transcript)
	for _, role := range roles.Roles {
		if role.RuntimeRole == "pnl_runtime" {
			return registry, role, transcript
		}
	}
	t.Fatal("pnl_runtime role is required")
	return registry, production.RoleManifest{}, transcript
}

func rewardHistoryPlanRegistration(registry *production.MigrationRegistry, id int64, schema string, predecessors ...int64) {
	registration := registry.Registrations[0]
	registration.MigrationID = id
	registration.Schema = schema
	registration.PredecessorIDs = predecessors
	registry.Registrations = append(registry.Registrations, registration)
	registry.ApplicationOrder = append(registry.ApplicationOrder, id)
}

func TestRewardHistoryPlanBindingUnrelatedRegistryChanges(t *testing.T) {
	for _, change := range []string{"unread schema", "approval digest", "migration control"} {
		t.Run(change, func(t *testing.T) {
			registry, role, transcript := rewardHistoryPlanInputs(t)
			switch change {
			case "unread schema":
				rewardHistoryPlanRegistration(&registry, 990001, "reward_history_unread")
			case "approval digest":
				registry.ApprovalDigest = production.Checksum([]byte(registry.ApprovalDigest))
			case "migration control":
				rewardHistoryPlanRegistration(&registry, 990001, "migration_control")
			}
			plan, err := rewardHistoryPlanBinding(registry, role, transcript)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(plan, transcript.Applied) {
				t.Fatal("unrelated registration or approval changed the runtime dependency plan")
			}
		})
	}
}

func TestRewardHistoryPlanBindingMissingMigrations(t *testing.T) {
	for _, change := range []string{"owned schema", "granted schema", "transitive predecessor"} {
		t.Run(change, func(t *testing.T) {
			registry, role, transcript := rewardHistoryPlanInputs(t)
			missing := "missing migrations [990001]"
			switch change {
			case "owned schema":
				rewardHistoryPlanRegistration(&registry, 990001, role.Schema)
			case "granted schema":
				var schema string
				for _, grant := range role.Grants {
					if grant.Schema != role.Schema && grant.Schema != "migration_control" {
						schema = grant.Schema
						break
					}
				}
				if schema == "" {
					t.Fatal("runtime requires a foreign schema grant")
				}
				rewardHistoryPlanRegistration(&registry, 990001, schema)
			case "transitive predecessor":
				rewardHistoryPlanRegistration(&registry, 990003, "reward_history_unread")
				rewardHistoryPlanRegistration(&registry, 990002, "reward_history_unread", 990003)
				rewardHistoryPlanRegistration(&registry, 990001, role.Schema, 990002)
				missing = "missing migrations [990001 990002 990003]"
			}
			_, err := rewardHistoryPlanBinding(registry, role, transcript)
			if err == nil {
				t.Fatal("runtime dependency addition requires a renewed catalog")
			}
			for _, detail := range []string{missing, "unexpected migrations []", "checksum mismatches []", "services/launchpad/pnl-tracker/internal/production/l18/testdata/reward_history/record_catalog.py"} {
				if !strings.Contains(err.Error(), detail) {
					t.Fatalf("plan diagnostic %q lacks %q", err, detail)
				}
			}
		})
	}
}

func TestRewardHistoryPlanBindingChangedTranscript(t *testing.T) {
	for _, change := range []string{"checksum", "empty checksum", "missing applied migration", "unexpected registered migration", "unexpected unregistered migration", "combined differences"} {
		t.Run(change, func(t *testing.T) {
			registry, role, transcript := rewardHistoryPlanInputs(t)
			var appliedID int64
			for _, id := range registry.ApplicationOrder {
				if _, ok := transcript.Applied[id]; ok {
					appliedID = id
					break
				}
			}
			if appliedID == 0 {
				t.Fatal("runtime requires an applied migration")
			}
			missing, unexpected, mismatched := []int64{}, []int64{}, []int64{}
			switch change {
			case "checksum":
				for i := range registry.Registrations {
					if registry.Registrations[i].MigrationID == appliedID {
						registry.Registrations[i].SHA256 = production.Checksum([]byte(registry.Registrations[i].SHA256))
					}
				}
				mismatched = []int64{appliedID}
			case "empty checksum":
				transcript.Applied[appliedID] = ""
				mismatched = []int64{appliedID}
			case "missing applied migration":
				delete(transcript.Applied, appliedID)
				missing = []int64{appliedID}
			case "unexpected registered migration":
				rewardHistoryPlanRegistration(&registry, 990002, "reward_history_unread")
				transcript.Applied[990002] = registry.Registrations[len(registry.Registrations)-1].SHA256
				unexpected = []int64{990002}
			case "unexpected unregistered migration":
				transcript.Applied[990002] = transcript.Applied[appliedID]
				unexpected = []int64{990002}
			case "combined differences":
				rewardHistoryPlanRegistration(&registry, 990001, role.Schema)
				transcript.Applied[990002] = transcript.Applied[appliedID]
				transcript.Applied[appliedID] = production.Checksum([]byte(transcript.Applied[appliedID]))
				missing, unexpected, mismatched = []int64{990001}, []int64{990002}, []int64{appliedID}
			}
			_, err := rewardHistoryPlanBinding(registry, role, transcript)
			if err == nil {
				t.Fatal("catalog must match every runtime migration and checksum")
			}
			want := fmt.Sprintf("reward history catalog differs from the runtime dependency plan: missing migrations %v; unexpected migrations %v; checksum mismatches %v; renew with services/launchpad/pnl-tracker/internal/production/l18/testdata/reward_history/record_catalog.py", missing, unexpected, mismatched)
			if err.Error() != want {
				t.Fatalf("plan diagnostic = %q, want %q", err, want)
			}
		})
	}
}

func TestRewardHistoryPlanBindingRepository(t *testing.T) {
	registry, role, transcript := rewardHistoryPlanInputs(t)
	plan, err := rewardHistoryPlanBinding(registry, role, transcript)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan, transcript.Applied) {
		t.Fatal("catalog must contain exactly the runtime dependency plan")
	}
	bootstrap := rewardHistoryBootstrap(t)
	if !reflect.DeepEqual(bootstrap.plan, plan) {
		t.Fatal("bootstrap must use the bound runtime dependency plan")
	}
}
