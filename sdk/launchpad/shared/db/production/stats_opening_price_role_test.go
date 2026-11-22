package production

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStatsRoleOpeningPriceColumns(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "migrations", "grants.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Roles []RoleManifest `json:"roles"`
	}
	if err = json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	for _, role := range document.Roles {
		if role.RuntimeRole != "stats_runtime" {
			continue
		}
		statements, err := role.GrantSQL()
		if err != nil {
			t.Fatal(err)
		}
		for object, columns := range map[string][]string{
			"pool_state_snapshots_v1": {"pool_id", "virtual_reserve", "real_reserve", "token_reserve", "block_number", "log_index"},
			"pools_read_v1":           {"pool_address", "pool_id"},
		} {
			count := 0
			for _, grant := range role.Grants {
				if grant.Schema != "indexer" || grant.Object != object {
					continue
				}
				count++
				if !grant.View || grant.ObjectKind != "view" || grant.GrantOption || !reflect.DeepEqual(grant.Privileges, []string{"SELECT"}) || !reflect.DeepEqual(grant.Columns, columns) {
					t.Fatalf("opening price read grant: %+v", grant)
				}
			}
			if count != 1 {
				t.Fatalf("%s read grants: %d", object, count)
			}
			quoted := make([]string, len(columns))
			for i, column := range columns {
				quoted[i] = `"` + column + `"`
			}
			want := `GRANT SELECT (` + strings.Join(quoted, ", ") + `) ON TABLE "indexer"."` + object + `" TO "stats_runtime"`
			if !strings.Contains(strings.Join(statements, ";"), want) {
				t.Fatalf("opening price column grant is not rendered: %s", object)
			}
		}
		return
	}
	t.Fatal("stats_runtime role is missing")
}
