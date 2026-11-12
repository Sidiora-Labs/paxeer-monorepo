package migrate_test

import (
	"context"
	"encoding/json"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/migrate"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestRewardRegistryBindsReviewedMigrationAndReader(t *testing.T) {
	root := coordinatortest.RepositoryRoot(t)
	read := func(path string) []byte {
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	source := read("launchpad/spec/production-system/integration/migrations/registry.json")
	approval := read("launchpad/spec/production-system/integration/migrations/pnl-reward-approval.json")
	reader := read("sdk/launchpad/shared/interfaces/v2/validators.gen.go")
	var document production.MigrationRegistry
	if err := json.Unmarshal(source, &document); err != nil {
		t.Fatal(err)
	}
	var admitted map[string]json.RawMessage
	if err := json.Unmarshal(approval, &admitted); err != nil {
		t.Fatal(err)
	}
	readerDigest, err := json.Marshal(production.Checksum(reader))
	if err != nil {
		t.Fatal(err)
	}
	admitted["interface_digest"] = readerDigest
	approval, err = json.Marshal(admitted)
	if err != nil {
		t.Fatal(err)
	}
	document.ApprovalDigest = production.Checksum(approval)
	for i := range document.Registrations {
		m := &document.Registrations[i]
		if m.MigrationID == 180001 {
			m.ApprovalManifestDigest = document.ApprovalDigest
			m.InterfaceDigest = production.Checksum(reader)
			m.ReaderMinVersion = 1
			m.EstimatedLockMS = 2000
		}
	}
	source, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := migrate.LoadRewardRegistry(source, approval, reader)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := registry.ValidateOrder()
	if err != nil || catalog[180001].Service != "core/pnl-tracker" {
		t.Fatal(catalog[180001], err)
	}
	for _, tc := range []struct {
		name                     string
		source, approval, reader []byte
	}{
		{"registry schema", []byte(`{}`), approval, reader},
		{"approval schema", source, []byte(`{`), reader},
		{"reader identity", source, approval, []byte("unbound")},
		{"approval identity", source, []byte(`{}`), reader},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := migrate.LoadRewardRegistry(tc.source, tc.approval, tc.reader); err == nil {
				t.Fatal("unbound migration registry accepted")
			}
		})
	}

	for _, name := range []string{"predecessor set", "predecessor checksum"} {
		t.Run(name, func(t *testing.T) {
			var candidate production.MigrationRegistry
			if err := json.Unmarshal(source, &candidate); err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(approval, &fields); err != nil {
				t.Fatal(err)
			}
			if name == "predecessor set" {
				fields["predecessors"] = json.RawMessage(`{}`)
			} else {
				for i := range candidate.Registrations {
					if candidate.Registrations[i].MigrationID == 80003 {
						candidate.Registrations[i].SHA256 = strings.Repeat("0", 64)
					}
				}
			}
			bound, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			candidate.ApprovalDigest = production.Checksum(bound)
			for i := range candidate.Registrations {
				if candidate.Registrations[i].MigrationID == 180001 {
					candidate.Registrations[i].ApprovalManifestDigest = candidate.ApprovalDigest
				}
			}
			raw, err := json.Marshal(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = migrate.LoadRewardRegistry(raw, bound, reader); err == nil || !strings.Contains(err.Error(), "predecessor") && !strings.Contains(err.Error(), "prerequisite") {
				t.Fatal(err)
			}
		})
	}
	if err := migrate.Up(context.Background(), "invalid"); err == nil || !strings.Contains(err.Error(), "migrate: up: cannot parse") {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := migrate.Up(canceled, os.Getenv("PNL_TEST_POSTGRES_DSN")); err == nil {
		t.Fatal("cancelled migration accepted")
	}
}
