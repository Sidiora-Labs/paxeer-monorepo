package coordinatortest

import (
	"bytes"
	"os"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
)

func TestExpectedSchemaFilePreservesRuntimeAuthority(t *testing.T) {
	database := Bootstrap(t, os.Getenv("COORDINATOR_TEST_POSTGRES_DSN"), "recovery_runtime")
	path, digest := database.ExpectedSchemaFile(t, "recovery_runtime")
	wire, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(wire, database.Runtimes["recovery_runtime"].ExpectedSchema) || production.Checksum(wire) != digest {
		t.Fatalf("runtime schema binding: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("runtime schema permissions: %v", err)
	}
}
