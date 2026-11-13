package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestChainReadTablesMatchIndexerMigration(t *testing.T) {
	raw, err := os.ReadFile("../../indexer/internal/migrate/chainlaunch.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(raw)
	for _, name := range []string{
		"indexer.launched_chains", "chain_id numeric(20,0)", "fee_right text", "status varchar(16)",
		"indexer.chain_revenue_epochs", "eligible_supply text", "snapshot_block numeric(20,0)",
		"indexer.chain_revenue_claims", "holder varchar(42)", "home_chain bigint NOT NULL DEFAULT 0",
	} {
		if !strings.Contains(schema, name) {
			t.Fatalf("chain read schema lacks %q", name)
		}
	}
}
