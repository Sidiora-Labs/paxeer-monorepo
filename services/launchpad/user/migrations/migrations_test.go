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

func TestEmbeddedMigrationsIncludeAccountStore(t *testing.T) {
	files, err := fs.Glob(FS, "*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(files, "00002_account_store.sql") {
		t.Fatalf("account store migration missing from embedded set: %v", files)
	}
	var sections map[string][]string
	for _, name := range files {
		raw, err := FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		parsed := migrationSections(t, raw)
		if name == "00002_account_store.sql" {
			sections = parsed
		}
	}
	tables := []string{
		"account_state", "account_watch", "account_consent", "account_outbox",
		"account_idempotency", "account_endpoint_proof", "account_device_installation",
		"account_endpoint", "account_job", "account_backfill", "account_backfill_range",
		"account_read_switch", "account_revision_inbox",
	}
	creates := map[string]int{}
	pattern := regexp.MustCompile(`(?s)^CREATE TABLE ([a-z_]+) \(`)
	for _, statement := range sections["Up"] {
		if match := pattern.FindStringSubmatch(statement); match != nil {
			creates[match[1]]++
		}
	}
	if len(creates) != len(tables) || len(sections["Down"]) != len(tables) {
		t.Fatalf("account schema create/drop coverage: %v / %v", creates, sections["Down"])
	}
	for i, table := range tables {
		if creates[table] != 1 {
			t.Errorf("expected exactly one CREATE TABLE for %s", table)
		}
		if sections["Down"][len(tables)-1-i] != "DROP TABLE "+table+";" {
			t.Errorf("Down must drop %s in reverse dependency order", table)
		}
	}
	up := strings.Join(sections["Up"], "\n")
	for _, required := range []string{
		"wallet varchar(42) NOT NULL UNIQUE",
		"PRIMARY KEY (account_id, chain_id, pool)",
		"PRIMARY KEY (account_id, category, channel)",
		"PRIMARY KEY (account_id, method, path, key)",
		"CREATE INDEX account_watch_page ON account_watch (account_id, added_at, pool)",
		"CREATE INDEX account_job_claim ON account_job (requested_at, job_id)",
		"CREATE INDEX account_endpoint_account_device ON account_endpoint (account_id, device_id)",
		"lease_until timestamptz", "fence bigint NOT NULL DEFAULT 0",
	} {
		if !strings.Contains(up, required) {
			t.Errorf("missing account query contract: %s", required)
		}
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
