package production

import (
	"context"
	"testing"
)

func TestMigrationRegistryHonorsPredecessorOrder(t *testing.T) {
	first := migration([]byte("CREATE TABLE stats.first_entry(id bigint)"))
	first.MigrationID = 70001
	first.Path = "first.sql"
	first.PredecessorIDs = nil
	second := migration([]byte("CREATE TABLE stats.second_entry(id bigint)"))
	second.MigrationID = 70002
	second.Path = "second.sql"
	second.PredecessorIDs = []int64{first.MigrationID}
	base := MigrationRegistry{SchemaVersion: 2, ApprovalDigest: Checksum([]byte("approved registry")), ApplicationOrder: []int64{first.MigrationID, second.MigrationID}, Registrations: []MigrationManifest{first, second}}
	catalog, err := base.ValidateOrder()
	if err != nil || len(catalog) != 2 || catalog[first.MigrationID].Path != first.Path || catalog[second.MigrationID].Path != second.Path {
		t.Fatalf("ordered migration catalog = %+v, %v", catalog, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*MigrationRegistry)
	}{
		{"schema", func(r *MigrationRegistry) { r.SchemaVersion = 1 }},
		{"approval", func(r *MigrationRegistry) { r.ApprovalDigest = "other" }},
		{"missing order", func(r *MigrationRegistry) { r.ApplicationOrder = nil }},
		{"duplicate registration", func(r *MigrationRegistry) { r.Registrations[1] = first }},
		{"unknown order", func(r *MigrationRegistry) { r.ApplicationOrder[1] = 70003 }},
		{"repeated order", func(r *MigrationRegistry) { r.ApplicationOrder[1] = first.MigrationID }},
		{"predecessor after child", func(r *MigrationRegistry) {
			r.ApplicationOrder[0], r.ApplicationOrder[1] = r.ApplicationOrder[1], r.ApplicationOrder[0]
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := base
			candidate.ApplicationOrder = append([]int64(nil), base.ApplicationOrder...)
			candidate.Registrations = append([]MigrationManifest(nil), base.Registrations...)
			tc.change(&candidate)
			if catalog, err := candidate.ValidateOrder(); err == nil || catalog != nil {
				t.Fatalf("invalid migration order admitted: %+v, %v", catalog, err)
			}
		})
	}
	if err := ApplyRegisteredMigration(context.Background(), nil, first, []byte("SELECT 1"), catalog, first.InterfaceDigest, first.ApprovalManifestDigest, 1); err == nil {
		t.Fatal("migration without direct coordinator connection admitted")
	}
}
