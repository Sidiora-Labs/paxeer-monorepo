package rehearsal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	EnvGatewayMigrationsDir = "CEREMONY_GATEWAY_MIGRATIONS_DIR"
	migrationLockKey        = "paxeer:migrations"
	ledgerDDL               = `
        create table if not exists _migrations (
          filename   text primary key,
          applied_at timestamptz not null default now()
        )
      `
)

var (
	ErrSchema           = errors.New("rehearsal: target schema does not match the source")
	ErrUnknownMigration = errors.New("rehearsal: source ledger names a migration the directory does not carry")
	ErrMigrations       = errors.New("rehearsal: gateway migrations directory is unusable")
)

type Migrations struct {
	Dir   string
	Files []string
}

func LoadMigrations(dir string) (Migrations, error) {
	if strings.TrimSpace(dir) == "" {
		return Migrations{}, fmt.Errorf("%w: %s is not set", ErrConfig, EnvGatewayMigrationsDir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Migrations{}, fmt.Errorf("%w: %v", ErrMigrations, err)
	}
	m := Migrations{Dir: dir}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			m.Files = append(m.Files, e.Name())
		}
	}
	if len(m.Files) == 0 {
		return Migrations{}, fmt.Errorf("%w: no .sql files in %s", ErrMigrations, dir)
	}
	sort.Strings(m.Files)
	return m, nil
}

func (m Migrations) apply(ctx context.Context, conn *pgx.Conn, file string) error {
	sqlText, err := os.ReadFile(filepath.Join(m.Dir, file))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMigrations, err)
	}
	statements, err := migrationStatements(string(sqlText))
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrMigrations, file, err)
	}
	if conn.PgConn().TxStatus() != 'I' {
		return fmt.Errorf("%w: migration %s requires an idle connection", ErrTarget, file)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%w: begin %s: %v", ErrTarget, file, err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var present int
	if err := tx.QueryRow(ctx, "select count(*) from _migrations where filename = $1", file).Scan(&present); err != nil {
		return fmt.Errorf("%w: ledger lookup %s: %v", ErrTarget, file, err)
	}
	if present > 0 {
		return nil
	}
	for _, statement := range statements {
		// Extended protocol admits exactly one statement. Even a change to SQL
		// string settings cannot smuggle a second command past the lexer.
		if _, err := conn.PgConn().ExecParams(ctx, statement, nil, nil, nil, nil).Close(); err != nil {
			return fmt.Errorf("%w: apply %s: %v", ErrTarget, file, err)
		}
	}
	if _, err := tx.Exec(ctx, "insert into _migrations(filename) values ($1)", file); err != nil {
		return fmt.Errorf("%w: record %s: %v", ErrTarget, file, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%w: commit %s: %v", ErrTarget, file, err)
	}
	return nil
}

func migrationStatements(sql string) ([]string, error) {
	var statements []string
	start, first := 0, ""
	finish := func(end int) error {
		if first == "" {
			start = end + 1
			return nil
		}
		switch first {
		case "BEGIN", "START", "COMMIT", "END", "ROLLBACK", "ABORT", "SAVEPOINT", "RELEASE", "PREPARE", "EXECUTE":
			return fmt.Errorf("transaction-control SQL %s is not supported", first)
		}
		statements = append(statements, sql[start:end])
		start, first = end+1, ""
		return nil
	}
	word := func(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '_' || b >= 128 }
	for i := 0; i < len(sql); {
		switch {
		case strings.ContainsRune(" \t\r\n\f", rune(sql[i])):
			i++
		case strings.HasPrefix(sql[i:], "--"):
			if end := strings.IndexByte(sql[i:], '\n'); end >= 0 {
				i += end + 1
			} else {
				i = len(sql)
			}
		case strings.HasPrefix(sql[i:], "/*"):
			depth := 1
			i += 2
			for i < len(sql) && depth > 0 {
				switch {
				case strings.HasPrefix(sql[i:], "/*"):
					depth++
					i += 2
				case strings.HasPrefix(sql[i:], "*/"):
					depth--
					i += 2
				default:
					i++
				}
			}
			if depth != 0 {
				return nil, errors.New("unterminated SQL comment")
			}
		case sql[i] == ';':
			if err := finish(i); err != nil {
				return nil, err
			}
			i++
		case word(sql[i]):
			end := i + 1
			for end < len(sql) && (word(sql[end]) || sql[end] >= '0' && sql[end] <= '9' || sql[end] == '$') {
				end++
			}
			if first == "" {
				first = strings.ToUpper(sql[i:end])
			}
			i = end
		case sql[i] == '\'' || sql[i] == '"':
			if first == "" {
				return nil, errors.New("expected SQL command before quote")
			}
			quote, closed := sql[i], false
			escaped := quote == '\'' && i > 0 && (sql[i-1] == 'e' || sql[i-1] == 'E') && (i < 2 || !word(sql[i-2]) && !(sql[i-2] >= '0' && sql[i-2] <= '9') && sql[i-2] != '$')
			i++
			for i < len(sql) {
				if escaped && sql[i] == '\\' {
					i += 2
				} else if sql[i] == quote {
					i++
					if i < len(sql) && sql[i] == quote {
						i++
					} else {
						closed = true
						break
					}
				} else {
					i++
				}
			}
			if !closed {
				return nil, errors.New("unterminated SQL quote")
			}
		case sql[i] == '$':
			if first == "" {
				return nil, errors.New("expected SQL command before dollar quote")
			}
			end := i + 1
			for end < len(sql) && (word(sql[end]) || end > i+1 && sql[end] >= '0' && sql[end] <= '9') {
				end++
			}
			if end >= len(sql) || sql[end] != '$' {
				i++
				continue
			}
			tag := sql[i : end+1]
			close := strings.Index(sql[end+1:], tag)
			if close < 0 {
				return nil, errors.New("unterminated SQL dollar quote")
			}
			i = end + 1 + close + len(tag)
		default:
			if first == "" {
				return nil, errors.New("expected SQL command")
			}
			i++
		}
	}
	if err := finish(len(sql)); err != nil {
		return nil, err
	}
	return statements, nil
}

func PrepareSchema(ctx context.Context, conn *pgx.Conn, m Migrations, ledger []string, tables []Table, load func(context.Context) error) ([]string, error) {
	known := make(map[string]bool, len(m.Files))
	for _, f := range m.Files {
		known[f] = true
	}
	listed := make(map[string]bool, len(ledger))
	for _, f := range ledger {
		if !known[f] {
			return nil, fmt.Errorf("%w: %s", ErrUnknownMigration, f)
		}
		listed[f] = true
	}
	if _, err := conn.Exec(ctx, "select pg_advisory_lock(hashtextextended($1, 0))", migrationLockKey); err != nil {
		return nil, fmt.Errorf("%w: migration lock: %v", ErrTarget, err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "select pg_advisory_unlock(hashtextextended($1, 0))", migrationLockKey)
	}()
	if _, err := conn.Exec(ctx, ledgerDDL); err != nil {
		return nil, fmt.Errorf("%w: ledger: %v", ErrTarget, err)
	}
	for _, f := range m.Files {
		if listed[f] {
			if err := m.apply(ctx, conn, f); err != nil {
				return nil, err
			}
		}
	}
	if err := CheckCounterparts(ctx, conn, tables); err != nil {
		return nil, err
	}
	if err := load(ctx); err != nil {
		return nil, err
	}
	var applied []string
	for _, f := range m.Files {
		if listed[f] {
			continue
		}
		if err := m.apply(ctx, conn, f); err != nil {
			return applied, err
		}
		applied = append(applied, f)
	}
	return applied, nil
}
