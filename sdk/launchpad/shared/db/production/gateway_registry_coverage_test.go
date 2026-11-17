package production

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMigrationRegistryHonorsPredecessorOrderGateway(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("repository path unavailable")
	}
	root := filepath.Join(filepath.Dir(file), "../../../../../")
	registryBytes, err := os.ReadFile(filepath.Join(root, "migrations", "registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var registry MigrationRegistry
	if err = json.Unmarshal(registryBytes, &registry); err != nil {
		t.Fatal(err)
	}
	var gateway MigrationManifest
	for _, candidate := range registry.Registrations {
		if candidate.MigrationID == 410001 {
			gateway = candidate
			break
		}
	}
	if gateway.MigrationID != 410001 || len(gateway.PredecessorIDs) != 1 || gateway.PredecessorIDs[0] != 410000 {
		t.Fatalf("gateway migration predecessor = %#v", gateway.PredecessorIDs)
	}
	position := func(id int64) int {
		for i, candidate := range registry.ApplicationOrder {
			if candidate == id {
				return i
			}
		}
		return -1
	}
	if position(410000) < 0 || position(410001) <= position(410000) {
		t.Fatalf("gateway migration order = %v", registry.ApplicationOrder)
	}

	grantsBytes, err := os.ReadFile(filepath.Join(root, "migrations", "grants.json"))
	if err != nil {
		t.Fatal(err)
	}
	var grants struct {
		Roles []RoleManifest `json:"roles"`
	}
	if err = json.Unmarshal(grantsBytes, &grants); err != nil {
		t.Fatal(err)
	}
	var gatewayRole, rankingRole *RoleManifest
	for i := range grants.Roles {
		switch grants.Roles[i].RuntimeRole {
		case "identity_runtime":
			gatewayRole = &grants.Roles[i]
		case "ranking_runtime":
			rankingRole = &grants.Roles[i]
		}
	}
	if gatewayRole == nil || rankingRole == nil {
		t.Fatal("runtime roles are not registered")
	}
	hasGrant := func(role *RoleManifest, schema, object string, privileges ...string) bool {
		for _, grant := range role.Grants {
			if grant.Schema != schema || grant.Object != object || len(grant.Privileges) != len(privileges) {
				continue
			}
			seen := map[string]bool{}
			for _, privilege := range grant.Privileges {
				seen[privilege] = true
			}
			matched := true
			for _, privilege := range privileges {
				matched = matched && seen[privilege]
			}
			if matched {
				return true
			}
		}
		return false
	}
	if !hasGrant(gatewayRole, "gateway", "restrictions", "SELECT", "INSERT", "UPDATE") || !hasGrant(gatewayRole, "gateway", "inventory_cursors", "SELECT", "INSERT", "UPDATE") {
		t.Fatal("gateway runtime grants do not cover storage statements")
	}
	if !hasGrant(rankingRole, "ranking", "snapshots", "SELECT", "INSERT", "DELETE") {
		t.Fatal("ranking runtime grant does not cover prune row locking and deletion")
	}
	var lockGrant bool
	for _, grant := range rankingRole.Grants {
		if grant.Schema == "ranking" && grant.Object == "snapshots" && len(grant.Privileges) == 1 && grant.Privileges[0] == "UPDATE" && len(grant.Columns) == 1 && grant.Columns[0] == "completed_at" {
			lockGrant = true
		}
	}
	if !lockGrant {
		t.Fatal("ranking prune requires a column-scoped row lock privilege")
	}

}
