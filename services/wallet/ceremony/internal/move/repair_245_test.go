package move

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/rehearsal"
)

func TestRepair_245_UnrelatedMissingTable(t *testing.T) {
	sourceLedger := []string{"001_init.sql", "005_agent_actions.sql"}
	source := []rehearsal.Table{{Name: "wallets"}, {Name: "wallet_signatures"}, {Name: "funded_tiers"}}
	target := []rehearsal.Table{{Name: "wallets"}}
	for _, filename := range []string{"006_attestor_signing.sql", "006_archive_funded.sql", "019_drop_retired_layerx_custody.sql", "999_unrelated.sql", ""} {
		t.Run(filename, func(t *testing.T) {
			later, err := migrationDifference(sourceLedger, append(append([]string{}, sourceLedger...), filename))
			if err != nil {
				t.Fatal(err)
			}
			present, err := checkTargetTables(source, target, later)
			if !errors.Is(err, rehearsal.ErrSchema) || !strings.Contains(err.Error(), "wallet_signatures") || present != nil {
				t.Fatalf("unexplained missing table accepted: present=%v error=%v", present, err)
			}
		})
	}
}

func TestRepair_245_ExactAppliedRetirements(t *testing.T) {
	cases := []struct {
		file   string
		tables []string
	}{
		{"006_archive_funded.sql", []string{"whitelist_entries", "funded_tiers"}},
		{"019_drop_retired_layerx_custody.sql", []string{"layerx_accounts", "wallet_custody_submissions", "wallet_custody_authorizations"}},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "gateway", "migrations", c.file)
			sql, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			drops := regexp.MustCompile(`(?im)^drop table (?:if exists )?([a-z_]+);`).FindAllStringSubmatch(string(sql), -1)
			var actual []string
			for _, drop := range drops {
				actual = append(actual, drop[1])
			}
			if !reflect.DeepEqual(actual, c.tables) {
				t.Fatalf("gateway retirement contract: got %v, want %v", actual, c.tables)
			}
			for _, name := range actual {
				source := []rehearsal.Table{{Name: "wallets"}, {Name: name}}
				target := []rehearsal.Table{{Name: "wallets"}}
				for _, applied := range []string{"", "006_attestor_signing.sql", "006_archive_funded.sql", "019_drop_retired_layerx_custody.sql"} {
					later, err := migrationDifference([]string{"001_init.sql"}, []string{applied, "001_init.sql"})
					if err != nil {
						t.Fatal(err)
					}
					present, err := checkTargetTables(source, target, later)
					if applied == c.file {
						if err != nil || !present["wallets"] || present[name] {
							t.Fatalf("applied %s retirement %s: present=%v error=%v", applied, name, present, err)
						}
					} else if !errors.Is(err, rehearsal.ErrSchema) || present != nil {
						t.Fatalf("%s retired by %q: present=%v error=%v", name, applied, present, err)
					}
				}
			}
		})
	}
}

func TestRepair_245_SourceAlreadyAppliedRetirement(t *testing.T) {
	for _, c := range []struct{ table, migration string }{
		{"funded_tiers", "006_archive_funded.sql"},
		{"layerx_accounts", "019_drop_retired_layerx_custody.sql"},
	} {
		later, err := migrationDifference([]string{"001_init.sql", c.migration}, []string{"999_unrelated.sql", c.migration, "001_init.sql"})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(later, []string{"999_unrelated.sql"}) {
			t.Fatalf("migrations applied since source: %v", later)
		}
		present, err := checkTargetTables([]rehearsal.Table{{Name: c.table}}, nil, later)
		if !errors.Is(err, rehearsal.ErrSchema) || present != nil {
			t.Fatalf("migration already in source authorized omission of %s: %v %v", c.table, present, err)
		}
	}
}

func TestRepair_245_MissingSourceMigration(t *testing.T) {
	later, err := migrationDifference([]string{"001_init.sql", "005_agent_actions.sql"}, []string{"001_init.sql", "006_archive_funded.sql"})
	if !errors.Is(err, ErrNotMoved) || !strings.Contains(err.Error(), "005_agent_actions.sql") || later != nil {
		t.Fatalf("incomplete moved ledger accepted: %v %v", later, err)
	}
}

func TestRepair_245_PresentTablesArePreserved(t *testing.T) {
	tables := []rehearsal.Table{{Name: "wallets"}, {Name: "funded_tiers"}, {Name: "wallet_signatures"}}
	for _, later := range [][]string{nil, {"006_archive_funded.sql", "019_drop_retired_layerx_custody.sql"}} {
		present, err := checkTargetTables(tables, tables, later)
		if err != nil || len(present) != len(tables) {
			t.Fatalf("present tables refused: %v %v", present, err)
		}
		for _, table := range tables {
			if !present[table.Name] {
				t.Fatalf("present table %s treated as retired", table.Name)
			}
		}
	}
}
