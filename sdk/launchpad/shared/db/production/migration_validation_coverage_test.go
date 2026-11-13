package production

import (
	"strings"
	"testing"
)

func TestMigrationAdmissionChecksApprovedSourceAndSafety(t *testing.T) {
	sql := []byte("CREATE TABLE stats.coverage_entry(id bigint)")
	base := migration(sql)
	base.PredecessorIDs = nil
	base.MigrationID = 70001
	base.Path = "coverage_entry.sql"
	for _, tc := range []struct {
		name   string
		change func(*MigrationManifest)
		input  []byte
		want   string
	}{
		{"owner lane", func(m *MigrationManifest) { m.OwnerLane = "L08" }, sql, "owner range"},
		{"source path", func(m *MigrationManifest) { m.Path = "../coverage_entry.sql" }, sql, "checksum/path"},
		{"source checksum", func(m *MigrationManifest) { m.SHA256 = Checksum([]byte("different")) }, sql, "checksum/path"},
		{"interface", func(m *MigrationManifest) { m.InterfaceDigest = Checksum([]byte("different")) }, sql, "interface/manifest"},
		{"reader", func(m *MigrationManifest) { m.ReaderMinVersion = 2 }, sql, "safety metadata"},
		{"lock", func(m *MigrationManifest) { m.EstimatedLockMS = 2001 }, sql, "safety metadata"},
		{"mode", func(m *MigrationManifest) { m.TransactionMode = "unknown" }, sql, "transaction mode"},
		{"destructive", func(m *MigrationManifest) { m.Destructive = true }, sql, "compatibility evidence"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := base
			tc.change(&candidate)
			registry := map[int64]MigrationManifest{candidate.MigrationID: candidate}
			err := candidate.Validate(tc.input, registry, nil, base.InterfaceDigest, base.ApprovalManifestDigest, 1)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("migration admission = %v, want %s", err, tc.want)
			}
		})
	}
	registry := map[int64]MigrationManifest{base.MigrationID: base}
	if err := base.Validate(sql, registry, map[int64]string{base.MigrationID: Checksum([]byte("different"))}, base.InterfaceDigest, base.ApprovalManifestDigest, 1); err == nil || !strings.Contains(err.Error(), "applied checksum") {
		t.Fatalf("changed applied migration = %v", err)
	}
	if err := base.Validate(sql, registry, nil, base.InterfaceDigest, base.ApprovalManifestDigest, 1); err != nil {
		t.Fatalf("approved additive migration = %v", err)
	}
}

func TestTrustedMigrationDocumentsRequireExactContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"malformed", []byte(`{`)},
		{"unregistered", []byte(`{"schemaVersion":2}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadMigrationRegistry(tc.data, Checksum(tc.data)); err == nil {
				t.Fatal("invalid reviewed registry accepted")
			}
		})
	}
	if _, err := LoadMigrationRegistry([]byte(`{}`), strings.Repeat("0", 64)); err == nil {
		t.Fatal("unbound reviewed registry accepted")
	}
	for _, data := range [][]byte{[]byte(`{"unknown":true}`), []byte(`{} {}`), []byte(`{}`)} {
		if _, err := LoadExpectedSchema(data, Checksum(data)); err == nil {
			t.Fatalf("invalid expected schema accepted: %s", data)
		}
	}
	if _, err := LoadExpectedSchema(nil, strings.Repeat("0", 64)); err == nil {
		t.Fatal("unbound expected schema accepted")
	}
}
