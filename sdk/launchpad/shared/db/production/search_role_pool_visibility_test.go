package production

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSearchRolePoolVisibilityColumns(t *testing.T) {
	assertPoolVisibilityColumns(t, "stats_search_runtime")
}

func TestStatsRolePoolVisibilityColumns(t *testing.T) {
	assertPoolVisibilityColumns(t, "stats_runtime")
}

func assertPoolVisibilityColumns(t *testing.T, runtimeRole string) {
	t.Helper()
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
	for _, role := range document.Roles {
		if role.RuntimeRole != runtimeRole {
			continue
		}
		statements, err := role.GrantSQL()
		if err != nil {
			t.Fatal(err)
		}
		matches := 0
		for _, grant := range role.Grants {
			if grant.Schema != "chat" {
				continue
			}
			matches++
			if grant.Object != "pool_search_visibility_v2" || !grant.View || grant.ObjectKind != "view" || grant.GrantOption || len(grant.ArgumentTypes) != 0 || !reflect.DeepEqual(grant.Privileges, []string{"SELECT"}) || !reflect.DeepEqual(grant.Columns, []string{"pool", "token", "public", "revision"}) {
				t.Fatalf("pool visibility read grant: %+v", grant)
			}
		}
		if matches != 1 {
			t.Fatalf("chat read grants = %d, want one", matches)
		}
		sql := strings.Join(statements, ";")
		for _, want := range []string{
			`GRANT USAGE ON SCHEMA "chat" TO "` + runtimeRole + `"`,
			`GRANT SELECT ("pool", "token", "public", "revision") ON TABLE "chat"."pool_search_visibility_v2" TO "` + runtimeRole + `"`,
		} {
			if !strings.Contains(sql, want) {
				t.Fatalf("missing rendered grant: %s", want)
			}
		}
		return
	}
	t.Fatalf("%s role is missing", runtimeRole)
}
