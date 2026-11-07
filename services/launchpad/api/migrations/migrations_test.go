package migrations

import (
	"bufio"
	"bytes"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestEmbeddedMigrationsIncludeAPIStream(t *testing.T) {
	files, err := fs.Glob(Migrations(), "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(files, "00001_api_stream.sql") {
		t.Fatalf("api stream migration missing from embedded set: %v", files)
	}
	raw, err := FS.ReadFile("00001_api_stream.sql")
	if err != nil {
		t.Fatal(err)
	}
	sections := migrationSections(t, raw)
	if sections["Up"][0] != "CREATE SCHEMA api_stream;" || sections["Down"][len(sections["Down"])-1] != "DROP SCHEMA api_stream;" {
		t.Fatal("api_stream schema must bracket its tracked tables")
	}
	creates := map[string]int{}
	pattern := regexp.MustCompile(`(?s)^CREATE TABLE api_stream\.([a-z_]+) \(`)
	for _, statement := range sections["Up"] {
		if match := pattern.FindStringSubmatch(statement); match != nil {
			creates[match[1]]++
		}
	}
	tables := []string{"stream_generation", "stream_projection", "stream_delivery_outbox", "stream_snapshots"}
	if len(creates) != len(tables) {
		t.Fatalf("stream table coverage: %v", creates)
	}
	for _, table := range tables {
		if creates[table] != 1 || !slices.Contains(sections["Down"], "DROP TABLE api_stream."+table+";") {
			t.Errorf("missing stream table create/drop: %s", table)
		}
	}
	up := strings.Join(sections["Up"], "\n")
	for _, required := range []string{
		"partition_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY",
		"topic text NOT NULL UNIQUE", "PRIMARY KEY (partition_id, sequence)",
		"event_id text NOT NULL UNIQUE", "frame bytea NOT NULL",
		"VALUES (true, gen_random_uuid())", "scope jsonb NOT NULL", "initial_offsets jsonb NOT NULL",
		"BEFORE UPDATE ON api_stream.stream_delivery_outbox",
		"RAISE EXCEPTION 'stream delivery envelopes are immutable';",
		"CREATE INDEX stream_snapshots_expiry ON api_stream.stream_snapshots (expires_at)",
	} {
		if !strings.Contains(up, required) {
			t.Errorf("missing stream storage contract: %s", required)
		}
	}
	if !slices.Contains(sections["Down"], "DROP FUNCTION api_stream.reject_stream_delivery_update();") {
		t.Fatal("Down must remove the immutable delivery trigger function")
	}
}

func migrationSections(t *testing.T, raw []byte) map[string][]string {
	t.Helper()
	sections := map[string][]string{}
	direction := ""
	block := false
	var statement strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "-- +goose ") {
			switch line {
			case "-- +goose Up", "-- +goose Down":
				if block || statement.Len() != 0 {
					t.Fatal("direction changed inside a statement")
				}
				next := strings.TrimPrefix(line, "-- +goose ")
				if _, exists := sections[next]; exists || (next == "Up" && direction != "") || (next == "Down" && direction != "Up") {
					t.Fatalf("invalid goose direction: %s", line)
				}
				direction = next
				sections[direction] = nil
			case "-- +goose StatementBegin":
				if direction == "" || block || statement.Len() != 0 {
					t.Fatal("invalid StatementBegin")
				}
				block = true
			case "-- +goose StatementEnd":
				if !block || !strings.HasSuffix(strings.TrimSpace(statement.String()), ";") {
					t.Fatal("invalid StatementEnd")
				}
				sections[direction] = append(sections[direction], strings.TrimSpace(statement.String()))
				statement.Reset()
				block = false
			default:
				t.Fatalf("unexpected goose annotation: %s", line)
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		if direction == "" {
			t.Fatal("SQL before goose Up")
		}
		statement.WriteString(line + "\n")
		if !block && strings.HasSuffix(line, ";") {
			sections[direction] = append(sections[direction], strings.TrimSpace(statement.String()))
			statement.Reset()
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if block || statement.Len() != 0 || len(sections["Up"]) == 0 || len(sections["Down"]) == 0 {
		t.Fatal("incomplete goose Up/Down migration")
	}
	return sections
}
