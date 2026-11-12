package production

import (
	"slices"
	"strings"
	"testing"
)

func TestProvisionRolesSQLBindsOwnersAndRuntimeRoles(t *testing.T) {
	first := manifest()
	second := first
	second.RuntimeRole = "l07_stats_reader"
	statements, err := ProvisionRolesSQL([]RoleManifest{first, second})
	if err != nil || len(statements) == 0 {
		t.Fatalf("role provisioning statements: %d, %v", len(statements), err)
	}
	joined := strings.Join(statements, ";")
	for _, fragment := range []string{
		`CREATE ROLE "l07_stats_owner" NOLOGIN NOSUPERUSER`,
		`CREATE SCHEMA IF NOT EXISTS "l07_stats" AUTHORIZATION "l07_stats_owner"`,
		`CREATE ROLE "l07_stats_runtime" NOLOGIN NOSUPERUSER`,
		`CREATE ROLE "l07_stats_reader" NOLOGIN NOSUPERUSER`,
		`REVOKE "l07_stats_owner" FROM "l07_stats_runtime"`,
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("role provisioning omitted %s", fragment)
		}
	}
	if count := strings.Count(joined, `CREATE SCHEMA IF NOT EXISTS "l07_stats"`); count != 1 {
		t.Fatalf("shared schema created %d times", count)
	}
	if slices.Index(statements, `CREATE ROLE "l07_stats_owner" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS`) > slices.Index(statements, `CREATE ROLE "l07_stats_runtime" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS`) {
		t.Fatal("runtime role provisioned before its owner")
	}
	if out, err := ProvisionRolesSQL(nil); err == nil || out != nil {
		t.Fatalf("empty role registry: %v, %v", out, err)
	}
	if out, err := ProvisionRolesSQL([]RoleManifest{first, first}); err == nil || out != nil {
		t.Fatalf("duplicate runtime role: %v, %v", out, err)
	}
	conflicting := first
	conflicting.RuntimeRole = "l07_stats_other"
	conflicting.OwnerRole = "l07_stats_other_owner"
	if out, err := ProvisionRolesSQL([]RoleManifest{first, conflicting}); err == nil || out != nil {
		t.Fatalf("conflicting schema owner: %v, %v", out, err)
	}
}
