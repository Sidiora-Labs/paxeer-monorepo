package production

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOperatorRoleCapabilitiesAndColumnGrants(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "migrations", "grants.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Roles []RoleManifest `json:"roles"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"operator_runtime", "operator_registrar"} {
		var manifest RoleManifest
		for _, candidate := range document.Roles {
			if candidate.RuntimeRole == name {
				manifest = candidate
			}
		}
		if manifest.Service != "security/operator" || manifest.Schema != "security" || manifest.OwnerRole != "security_owner" || len(manifest.SchemaCapabilities) != 1 {
			t.Fatal("operator authority registration missing")
		}
		statements, err := manifest.GrantSQL()
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(statements, ";")
		if !strings.Contains(joined, `GRANT USAGE ON SCHEMA "pnl"`) || !strings.Contains(joined, `GRANT UPDATE ("revoked_at") ON TABLE "pnl"."authority_credentials"`) {
			t.Fatal("operator grants omitted its credential authority")
		}
		for _, grant := range manifest.Grants {
			if grant.ObjectKind == "table" && (len(grant.Columns) == 0 || len(grant.Privileges) != 1) {
				t.Fatal("operator table privileges must be column-scoped")
			}
			if grant.ObjectKind == "table" && grant.Privileges[0] == "INSERT" && !strings.Contains(joined, "GRANT INSERT (") {
				t.Fatal("column insertion privilege was not rendered")
			}
		}
		for _, mutate := range []func(*RoleManifest){
			func(m *RoleManifest) { m.Service = "security/control" },
			func(m *RoleManifest) { m.RuntimeRole = "control_runtime" },
			func(m *RoleManifest) { m.SchemaCapabilities[0].OwnerRole = "security_owner" },
			func(m *RoleManifest) { m.SchemaCapabilities[0].Schema = "users" },
			func(m *RoleManifest) { m.SchemaCapabilities[0].Service = "media/user" },
		} {
			candidate := manifest
			candidate.SchemaCapabilities = append([]SchemaCapability(nil), manifest.SchemaCapabilities...)
			mutate(&candidate)
			if candidate.Validate() == nil {
				t.Fatal("unreviewed operator authority admitted")
			}
		}
	}
	foreign := manifest()
	foreign.Grants[1].Privileges = []string{"INSERT"}
	if foreign.Validate() == nil {
		t.Fatal("column INSERT admitted on a foreign interface")
	}
}
