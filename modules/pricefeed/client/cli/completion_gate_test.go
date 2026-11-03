package cli_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestMarketsCompletionRetiredSurfacesAbsent(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("locate repository root: %v", err)
	}

	for _, retired := range []struct {
		id     string
		path   string
		symbol string
	}{
		{
			"interface.3.go-pcspot-newprecompileset-j26",
			"precompiles/spot/spot.go",
			"NewPrecompileSet",
		},
		{
			"interface.3.http-perps-data-graphql-query-spotbatchclearings-j27",
			"services/platform/perps-data/internal/graphql/schema.go",
			"spotBatchClearings",
		},
		{
			"interface.3.http-perps-data-graphql-query-spotepochsettlements-j27",
			"services/platform/perps-data/internal/graphql/schema.go",
			"spotEpochSettlements",
		},
		{
			"interface.3.http-perps-data-graphql-query-spotfastsettlements-j27",
			"services/platform/perps-data/internal/graphql/schema.go",
			"spotFastSettlements",
		},
		{
			"interface.3.http-perps-data-graphql-query-spotmarketmodechanges-j27",
			"services/platform/perps-data/internal/graphql/schema.go",
			"spotMarketModeChanges",
		},
		{
			"interface.3.http-perps-data-graphql-query-spotpofqupdates-j27",
			"services/platform/perps-data/internal/graphql/schema.go",
			"spotPofqUpdates",
		},
		{
			"interface.3.http-perps-data-graphql-query-spotplvs-j27",
			"services/platform/perps-data/internal/graphql/schema.go",
			"spotPLVs",
		},
		{
			"interface.3.http-perps-data-graphql-query-spotkeepercycles-j27",
			"services/platform/perps-data/internal/graphql/schema.go",
			"spotKeeperCycles",
		},
		{
			"interface.3.http-perps-data-graphql-query-spotbatchclearfailures-j27",
			"services/platform/perps-data/internal/graphql/schema.go",
			"spotBatchClearFailures",
		},
	} {
		t.Run("retired/"+retired.id, func(t *testing.T) {
			path := filepath.Join(root, filepath.FromSlash(retired.path))
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read retired interface %s source %s: %v", retired.id, retired.path, err)
			}
			if retired.symbol == "NewPrecompileSet" {
				file, err := parser.ParseFile(token.NewFileSet(), path, source, parser.AllErrors)
				if err != nil {
					t.Fatalf("parse retired interface %s source %s: %v", retired.id, retired.path, err)
				}
				if file.Scope.Lookup(retired.symbol) != nil {
					t.Errorf("retired interface %s remains declared at %s", retired.id, retired.path)
				}
			} else {
				field := regexp.MustCompile(`\b` + regexp.QuoteMeta(retired.symbol) + `\s*(\(|:)`)
				if field.Match(source) {
					t.Errorf("retired interface %s remains declared at %s", retired.id, retired.path)
				}
			}
		})
	}

	for _, planned := range []struct {
		task string
		path string
	}{
		{"3.3.2", "precompiles/perps/quote_market_test.go"},
		{"3.7.1", "precompiles/perps/testdata/views_account_golden.json"},
		{"3.7.1", "precompiles/perps/views_golden_account_test.go"},
		{"3.7.2", "precompiles/perps/testdata/views_market_golden.json"},
		{"3.7.2", "precompiles/perps/views_golden_market_test.go"},
		{"3.7.3", "precompiles/perps/testdata/views_funding_golden.json"},
		{"3.7.3", "precompiles/perps/views_golden_funding_test.go"},
		{"3.7.4", "precompiles/perps/testdata/views_quotes_golden.json"},
		{"3.7.4", "precompiles/perps/views_golden_quotes_test.go"},
		{"3.7.5", "precompiles/perps/testdata/views_insurance_golden.json"},
		{"3.7.5", "precompiles/perps/views_golden_insurance_test.go"},
		{"3.8.1", "modules/perps/migration_test.go"},
		{"3.8.2", "modules/perps/keeper/seed_governed_test.go"},
		{"3.22.8", "modules/spot/keeper/migrations_test.go"},
		{"3.23.6", "precompiles/spot/listing_test.go"},
		{"3.43.1", "services/platform/perps-data/internal/graphql/retired_test.go"},
		{"3.47.1", "apps/perps/tests/perps-abi-parity.test.ts"},
		{"3.47.2", "apps/perps/tests/spot-abi-parity.test.ts"},
		{"3.54.2", "apps/perps/tests/local-order-tick.test.ts"},
		{"3.54.3", "apps/perps/tests/local-market-config.test.ts"},
		{"3.54.5", "apps/perps/tests/local-fee-tier.test.ts"},
		{"3.54.6", "apps/perps/tests/local-spot-token-state.test.ts"},
		{"3.59.1", "apps/perps/tests/position-pnl-tiers.test.ts"},
		{"3.97.1", "modules/oracle/keeper/slash_metrics_test.go"},
	} {
		t.Run("planned/"+planned.path, func(t *testing.T) {
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(planned.path)))
			if err != nil {
				t.Fatalf("task %s required planned path %s: %v", planned.task, planned.path, err)
			}
			if !info.Mode().IsRegular() {
				t.Errorf("task %s required planned path %s is not a regular file", planned.task, planned.path)
			}
		})
	}
}
