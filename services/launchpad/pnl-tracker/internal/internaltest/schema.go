package internaltest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func RegisteredObjectSQL(t *testing.T, migration int64, kind, name string) string {
	t.Helper()
	root := coordinatortest.RepositoryRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "migrations/registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := production.LoadMigrationRegistry(raw, production.Checksum(raw))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := registry.ValidateOrder()
	if err != nil {
		t.Fatal(err)
	}
	manifest, ok := catalog[migration]
	if !ok {
		t.Fatalf("migration %d is unregistered", migration)
	}
	raw, err = coordinatortest.ReadMigrationSource(root, manifest.Path, manifest.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`(?is)\bCREATE\s+` + regexp.QuoteMeta(kind) + `\s+` + regexp.QuoteMeta(name) + `\s[^;]*;`)
	matches := pattern.FindAllString(string(raw), -1)
	if len(matches) != 1 {
		t.Fatalf("migration %d declares %d instances of %s %s", migration, len(matches), kind, name)
	}
	return matches[0]
}

func RewardRegistryFixture(t *testing.T, root string) ([]byte, []byte, []byte) {
	t.Helper()
	read := func(path string) []byte {
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	source := read("migrations/registry.json")
	approval := read("migrations/pnl-reward-approval.json")
	reader := read("sdk/launchpad/shared/interfaces/v2/validators.gen.go")
	var registry production.MigrationRegistry
	if err := json.Unmarshal(source, &registry); err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(approval, &fields); err != nil {
		t.Fatal(err)
	}
	fields["interface_digest"] = json.RawMessage(fmt.Sprintf("%q", production.Checksum(reader)))
	approval, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	registry.ApprovalDigest = production.Checksum(approval)
	for i := range registry.Registrations {
		manifest := &registry.Registrations[i]
		if manifest.MigrationID == 180001 {
			manifest.ApprovalManifestDigest = registry.ApprovalDigest
			manifest.InterfaceDigest = production.Checksum(reader)
		}
	}
	source, err = json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	return source, approval, reader
}
