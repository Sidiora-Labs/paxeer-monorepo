package rest_test

import (
	"os"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
)

// TestProductionReadsWithPrivateDatabase exercises the registered read routes
// against independent databases created from the module's PostgreSQL fixture.
func TestProductionReadsWithPrivateDatabase(t *testing.T) {
	admin := os.Getenv(internaltest.ExternalPostgresEnv)
	if admin == "" {
		t.Fatal("PostgreSQL fixture is required")
	}
	t.Setenv("API_TEST_DATABASE_URL", admin)
	for _, check := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"pool detail", TestPoolDetailProductionRead},
		{"pool list", TestPoolListProductionRead},
		{"holders validation", TestHoldersProductionReadValidation},
		{"platform metrics", TestPlatformMetricsProductionRead},
		{"holder rewards", TestHolderRewardsProductionRead},
		{"canonical stats and snapshot", TestProductionCanonicalStatsAndSnapshotResume},
		{"frozen response schemas", TestFrozenV2ResponseSchemasMatchHandlerBytes},
		{"private snapshot", TestProductionPrivateSnapshotDenialIsUncacheableAndRedactsTokens},
		{"admission surge", TestProductionHTTPAdmissionSurgePreservesReserveAndRejectsExplicitly},
		{"registered operations", TestProductionRegisteredOperationsHaveFrozenContracts},
	} {
		t.Run(check.name, check.run)
	}
}
