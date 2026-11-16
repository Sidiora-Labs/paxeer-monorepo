package production

import "testing"

func TestExpectedSchemaPrivilegeMatrixStatsSearchCapability(t *testing.T) {
	manifest := RoleManifest{Service: "core/stats-workers", Schema: "stats", OwnerRole: "stats_owner", RuntimeRole: "stats_runtime", SchemaCapabilities: []SchemaCapability{{Service: "core/stats-workers/search", Schema: "stats_search", OwnerRole: "stats_search_owner"}}, Grants: []Grant{{Schema: "stats_search", Object: "documents_v2", ObjectKind: "table", Privileges: []string{"SELECT", "INSERT"}}, {Schema: "stats_search", Object: "revision_v2", ObjectKind: "sequence", Privileges: []string{"USAGE"}}}}
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, capability := range []SchemaCapability{
		{Service: "core/stats-workers/search", Schema: "stats", OwnerRole: "stats_owner"},
		{Service: "core/stats-workers/search", Schema: "stats_search", OwnerRole: "stats_owner"},
		{Service: "core/stats-workers/other", Schema: "stats_search", OwnerRole: "stats_search_owner"},
	} {
		candidate := manifest
		candidate.SchemaCapabilities = []SchemaCapability{capability}
		if err := candidate.Validate(); err == nil {
			t.Fatalf("unreviewed search capability admitted: %+v", capability)
		}
	}
	candidate := manifest
	candidate.RuntimeRole = "ranking_runtime"
	if err := candidate.Validate(); err == nil {
		t.Fatal("search write capability admitted for another runtime")
	}
	candidate = manifest
	candidate.Service = "core/ranking-algo"
	if err := candidate.Validate(); err == nil {
		t.Fatal("search write capability admitted for another service")
	}
}
