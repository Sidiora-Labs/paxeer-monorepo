package engine

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// SQLiteStore persists task results in a SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at dbPath and runs
// any pending schema migrations. The file is opened in WAL mode with
// pragmas tuned for a single-writer sidecar workload.
//
// The database file must reside on a local or block-device-backed
// filesystem (e.g. EBS, GCE PD, local SSD). WAL mode is unsafe on
// NFS-backed volumes (EFS, Azure Files, CephFS over NFS) because they
// do not support the POSIX byte-range locks that SQLite requires for
// the shared-memory (-shm) file.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	return openStore(dbPath)
}

// NewMemoryStore returns a SQLiteStore backed by an in-memory SQLite
// database (the ":memory:" DSN). The database exists only for the
// lifetime of the returned store — nothing is written to disk. Useful
// for tests and non-sidecar CLI commands.
func NewMemoryStore() (*SQLiteStore, error) {
	return openStore(":memory:")
}

func openStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Save(r *TaskResult) error {
	params, err := json.Marshal(r.Params)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	args := []any{
		r.Type, string(r.Status), r.Run, string(params), nullableRawJSON(r.Result),
		r.Error, r.SubmittedAt.UTC().Format(time.RFC3339Nano), formatNullableTime(r.CompletedAt),
	}
	var res sql.Result
	generation := r.Generation
	if generation == "" {
		generation = uuid.NewString()
		res, err = s.db.Exec(`
			INSERT INTO task_results
				(type, status, run, params, result, error, submitted_at, completed_at, id, generation)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				type=excluded.type, status=excluded.status, run=excluded.run,
				params=excluded.params, result=excluded.result, error=excluded.error,
				submitted_at=excluded.submitted_at, completed_at=excluded.completed_at,
				generation=excluded.generation
			WHERE task_results.status = 'failed'`, append(args, r.ID, generation)...)
	} else {
		res, err = s.db.Exec(`
			UPDATE task_results SET type=?, status=?, run=?, params=?, result=?, error=?,
				submitted_at=?, completed_at=?
			WHERE id=? AND generation=? AND NOT EXISTS (
				SELECT 1 FROM task_cancellations WHERE id=? AND generation=?
			)`, append(args, r.ID, generation, r.ID, generation)...)
	}
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrTaskGeneration
	}
	r.Generation = generation
	return nil
}

func (s *SQLiteStore) Get(id string) (*TaskResult, error) {
	row := s.db.QueryRow(selectColumns+` WHERE id = ?`, id)
	r, err := scanTaskResult(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *SQLiteStore) List(limit int) ([]TaskResult, error) {
	return s.queryMany(selectColumns+` ORDER BY submitted_at DESC LIMIT ?`, limit)
}

func (s *SQLiteStore) ListStaleTasks() ([]TaskResult, error) {
	return s.queryMany(selectColumns+` WHERE status = ?`, string(TaskStatusRunning))
}

func (s *SQLiteStore) Delete(id string) (bool, error) {
	n, err := s.deleteTasks("id = ?", id)
	return n > 0, err
}

func (s *SQLiteStore) DeleteByType(taskType string) (int, error) {
	return s.deleteTasks("type = ?", taskType)
}

func (s *SQLiteStore) deleteTasks(predicate string, value string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR IGNORE INTO task_cancellations (id, generation)
		SELECT id, generation FROM task_results WHERE status = 'running' AND `+predicate, value); err != nil {
		return 0, err
	}
	res, err := tx.Exec("DELETE FROM task_results WHERE "+predicate, value)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(n), nil
}

func (s *SQLiteStore) ClearCancellation(id, generation string) error {
	_, err := s.db.Exec("DELETE FROM task_cancellations WHERE id = ? AND generation = ?", id, generation)
	return err
}

func (s *SQLiteStore) LatestByType(taskType string) (*TaskResult, error) {
	row := s.db.QueryRow(selectColumns+` WHERE type = ? ORDER BY submitted_at DESC LIMIT 1`, taskType)
	r, err := scanTaskResult(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *SQLiteStore) Readiness() (bool, error) {
	rows, err := s.db.Query(`SELECT type, submitted_at FROM task_results
		WHERE type = ? OR (type = ? AND status = ?)`,
		string(TaskMarkNotReady), string(TaskMarkReady), string(TaskStatusCompleted))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var release, hold time.Time
	for rows.Next() {
		var taskType, submittedAt string
		if err := rows.Scan(&taskType, &submittedAt); err != nil {
			return false, err
		}
		submitted, err := time.Parse(time.RFC3339Nano, submittedAt)
		if err != nil {
			return false, fmt.Errorf("parse readiness submitted_at: %w", err)
		}
		if taskType == string(TaskMarkNotReady) {
			if submitted.After(hold) {
				hold = submitted
			}
		} else if submitted.After(release) {
			release = submitted
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return !release.IsZero() && release.After(hold), nil
}

// SaveTxMarker persists a pre-broadcast marker and fsyncs it (via checkpoint,
// since the store runs synchronous=NORMAL) before returning, so it survives a
// crash. Callers MUST let it return before broadcasting.
func (s *SQLiteStore) SaveTxMarker(m *TxMarker) error {
	if _, err := s.db.Exec(`
		INSERT OR REPLACE INTO tx_markers
			(task_id, tx_hash, tx_bytes, account_number, sequence, chain_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.TaskID, m.TxHash, m.TxBytes, m.AccountNumber, m.Sequence, m.ChainID,
		time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return err
	}
	// Ignoring FULL's busy row is safe only under SetMaxOpenConns(1) (no
	// concurrent readers); if that's raised, use TRUNCATE or assert busy==0.
	if _, err := s.db.Exec("PRAGMA wal_checkpoint(FULL)"); err != nil {
		return fmt.Errorf("checkpoint tx marker: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetTxMarker(taskID string) (*TxMarker, error) {
	row := s.db.QueryRow(
		`SELECT task_id, tx_hash, tx_bytes, account_number, sequence, chain_id
		 FROM tx_markers WHERE task_id = ?`, taskID)
	var m TxMarker
	err := row.Scan(&m.TaskID, &m.TxHash, &m.TxBytes, &m.AccountNumber, &m.Sequence, &m.ChainID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *SQLiteStore) Ping() error {
	var n int
	return s.db.QueryRow("SELECT 1").Scan(&n)
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// --- query helpers ---

const selectColumns = `
	SELECT id, type, status, run, params, result, error, submitted_at, completed_at, generation
	FROM task_results`

// queryMany executes a query and scans all rows into TaskResults.
func (s *SQLiteStore) queryMany(query string, args ...any) ([]TaskResult, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TaskResult
	for rows.Next() {
		r, err := scanTaskResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *r)
	}
	return results, rows.Err()
}

// rowScanner abstracts *sql.Row and *sql.Rows for shared scan logic.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanTaskResult(s rowScanner) (*TaskResult, error) {
	var (
		r           TaskResult
		status      string
		paramsJSON  string
		resultJSON  sql.NullString
		submittedAt string
		completedAt sql.NullString
	)

	if err := s.Scan(
		&r.ID, &r.Type, &status, &r.Run, &paramsJSON, &resultJSON,
		&r.Error, &submittedAt, &completedAt, &r.Generation,
	); err != nil {
		return nil, err
	}

	r.Status = TaskStatus(status)

	if paramsJSON != "" {
		if err := json.Unmarshal([]byte(paramsJSON), &r.Params); err != nil {
			return nil, fmt.Errorf("unmarshal params: %w", err)
		}
	}

	if resultJSON.Valid && resultJSON.String != "" {
		r.Result = json.RawMessage(resultJSON.String)
	}

	t, err := time.Parse(time.RFC3339Nano, submittedAt)
	if err != nil {
		return nil, fmt.Errorf("parse submitted_at: %w", err)
	}
	r.SubmittedAt = t

	if completedAt.Valid {
		t, err := time.Parse(time.RFC3339Nano, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse completed_at: %w", err)
		}
		r.CompletedAt = &t
	}

	return &r, nil
}

func formatNullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// nullableRawJSON binds a result payload as SQL NULL when empty so the
// common no-result case stores NULL rather than an empty string.
func nullableRawJSON(r json.RawMessage) any {
	if len(r) == 0 {
		return nil
	}
	return string(r)
}
