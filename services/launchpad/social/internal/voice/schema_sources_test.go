package voice

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/migrations"
)

type schemaSource struct {
	name string
	up   string
}

func schemaSources(t *testing.T) []schemaSource {
	t.Helper()
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no embedded social migrations")
	}
	sources := make([]schemaSource, 0, len(names))
	for _, name := range names {
		raw, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		_, up, ok := strings.Cut(string(raw), "-- +goose Up")
		if !ok {
			t.Fatalf("migration %s has no Up section", name)
		}
		up, _, ok = strings.Cut(up, "-- +goose Down")
		if !ok || strings.TrimSpace(up) == "" {
			t.Fatalf("migration %s has no complete Up/Down sections", name)
		}
		sources = append(sources, schemaSource{name: name, up: up})
	}
	return sources
}

func TestSchemaSourcesExist(t *testing.T) {
	sources := schemaSources(t)
	files := make(map[string]bool, len(sources))
	tables := make(map[string]bool)
	declaration := regexp.MustCompile(`(?im)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?chat\.([a-z_][a-z_0-9]*)\s*\(`)
	for _, source := range sources {
		files[source.name] = true
		for _, match := range declaration.FindAllStringSubmatch(source.up, -1) {
			tables[strings.ToLower(match[1])] = true
		}
	}
	for _, name := range []string{
		"00001_init.sql",
		"00002_dev_v2_social.sql",
		"00003_safety.sql",
		"00004_safety_bans_v1.sql",
		"00005_dev_v2_social_graph.sql",
	} {
		if !files[name] {
			t.Errorf("required social migration %s is absent", name)
		}
	}
	for _, table := range []string{
		"safety_bans",
		"voice_consents",
		"voice_ejections",
		"voice_participants",
		"voice_recording_events",
		"voice_recordings",
		"voice_rooms",
	} {
		if !tables[table] {
			t.Errorf("social migration Up sections do not declare chat.%s required by voice queries", table)
		}
	}
}
