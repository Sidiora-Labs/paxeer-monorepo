package production

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestChartsRoleCanonicalCreationColumns(t *testing.T) {
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
		if role.RuntimeRole != "candles_runtime" {
			continue
		}
		statements, err := role.GrantSQL()
		if err != nil {
			t.Fatal(err)
		}
		for _, grant := range role.Grants {
			if grant.Schema != "indexer" || grant.Object != "indexer_canonical_creations_v1" {
				continue
			}
			want := []string{"pool", "factory", "height", "block_hash", "transaction_hash", "transaction_index", "log_index", "payload"}
			if !grant.View || grant.ObjectKind != "view" || grant.GrantOption || !reflect.DeepEqual(grant.Privileges, []string{"SELECT"}) || !reflect.DeepEqual(grant.Columns, want) {
				t.Fatalf("canonical creation read grant: %+v", grant)
			}
			if !strings.Contains(strings.Join(statements, ";"), `GRANT SELECT ("pool", "factory", "height", "block_hash", "transaction_hash", "transaction_index", "log_index", "payload") ON TABLE "indexer"."indexer_canonical_creations_v1" TO "candles_runtime"`) {
				t.Fatal("canonical creation column grant is not rendered")
			}
			return
		}
	}
	t.Fatal("canonical creations read grant for candles_runtime is missing")
}
