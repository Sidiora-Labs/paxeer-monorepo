package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/sei-protocol/seictl/sidecar/engine"
)

const (
	MaxTaskRequestBytes                   = 64 * 1024
	MaxQueuedTasks                        = 32
	MaxRetainedTasks                      = 256
	TaskStatusQueued    engine.TaskStatus = "queued"
)

var (
	ErrTaskOverload             = errors.New("task capacity exhausted; delete unneeded terminal results or retry after work completes")
	ErrTaskAdmissionUnavailable = errors.New("durable task admission is unavailable")
)

// admissionState has one worker for the node, rather than one waiter per task.
// The active row survives cancellation until the engine reports handler exit.
type admissionState struct {
	db     *sql.DB
	mu     sync.Mutex
	wake   chan struct{}
	stop   chan struct{}
	wg     sync.WaitGroup
	eng    *engine.Engine
	closed bool
	lock   *os.File
}

func lockAdmission(path string) (*os.File, error) {
	if path == ":memory:" {
		return nil, nil
	}
	f, err := os.OpenFile(path+".admission.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("node task store is already open: %w", err)
	}
	return f, nil
}

func openAdmission(path string) (*admissionState, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=FULL",
		`CREATE TABLE IF NOT EXISTS task_admission (
   sequence INTEGER PRIMARY KEY AUTOINCREMENT,
   id TEXT NOT NULL UNIQUE, type TEXT NOT NULL, params TEXT NOT NULL,
   submitted_at TEXT NOT NULL, generation TEXT NOT NULL DEFAULT '',
   state TEXT NOT NULL CHECK(state IN ('pending','active','cancelled'))
  )`,
		`CREATE UNIQUE INDEX IF NOT EXISTS task_admission_one_active ON task_admission((1)) WHERE state IN ('active','cancelled')`,
	} {
		if _, err = db.Exec(statement); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &admissionState{db: db, wake: make(chan struct{}, 1), stop: make(chan struct{})}, nil
}

func (s *TaskStore) signalAdmission() {
	select {
	case s.admission.wake <- struct{}{}:
	default:
	}
}

func (s *TaskStore) bindEngine(eng *engine.Engine) error {
	a := s.admission
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return ErrTaskAdmissionUnavailable
	}
	if a.eng != nil {
		if a.eng != eng {
			return fmt.Errorf("%w: store already belongs to another engine", ErrTaskAdmissionUnavailable)
		}
		return nil
	}
	var running int
	if err := a.db.QueryRow(`SELECT count(*) FROM task_results WHERE status='running'`).Scan(&running); err != nil {
		return err
	}
	if running > 1 {
		return fmt.Errorf("%w: multiple legacy running tasks require bounded engine recovery", ErrTaskAdmissionUnavailable)
	}
	// Only startup may reclaim an active slot from a previous process. No
	// live handler can still own it after this store has been closed/reopened.
	_, err := a.db.Exec(`DELETE FROM task_admission WHERE state = 'cancelled' OR
  (state = 'active' AND EXISTS (SELECT 1 FROM task_results r WHERE r.id=task_admission.id AND r.status IN ('completed','failed')))`)
	if err != nil {
		return err
	}
	_, err = a.db.Exec(`UPDATE task_admission SET state='pending' WHERE state='active'
  AND NOT EXISTS (SELECT 1 FROM task_results r WHERE r.id=task_admission.id)`)
	if err != nil {
		return err
	}
	_, err = a.db.Exec(`UPDATE task_admission SET generation=COALESCE((SELECT generation FROM task_results r WHERE r.id=task_admission.id),generation) WHERE state='active'`)
	if err != nil {
		return err
	}
	a.eng = eng
	a.wg.Add(1)
	go s.runAdmission()
	s.signalAdmission()
	return nil
}

func knownTaskType(t engine.TaskType) bool {
	switch t {
	case engine.TaskSnapshotRestore, engine.TaskConfigPatch, engine.TaskConfigApply,
		engine.TaskConfigValidate, engine.TaskConfigReload, engine.TaskMarkReady,
		engine.TaskRestartSeid, engine.TaskConfigureGenesis, engine.TaskConfigureStateSync,
		engine.TaskSnapshotUpload, engine.TaskSnapshotUploadOnce, engine.TaskResultExport,
		engine.TaskAwaitCondition, engine.TaskGenerateIdentity, engine.TaskGenerateGentx,
		engine.TaskUploadGenesisArtifacts, engine.TaskAssembleAndUploadGenesis,
		engine.TaskSetGenesisPeers, engine.TaskGovVote, engine.TaskGovSoftwareUpgrade,
		engine.TaskGovParamChange, engine.TaskEvmLogicalDigest, engine.TaskMarkNotReady,
		engine.TaskStopSeid, engine.TaskResetData:
		return true
	}
	return false
}

const retainedTaskCount = `SELECT count(*) FROM (
 SELECT id FROM task_results UNION SELECT id FROM task_admission
 UNION SELECT task_id FROM tx_markers UNION SELECT id FROM task_cancellations
)`

func (s *TaskStore) SubmitQueued(task engine.Task) (string, error) {
	if !knownTaskType(task.Type) {
		return "", fmt.Errorf("unknown task type: %s", task.Type)
	}
	if task.ID == "" {
		task.ID = uuid.NewString()
	} else if _, err := uuid.Parse(task.ID); err != nil {
		return "", engine.ErrInvalidTaskID
	}
	params, err := json.Marshal(task.Params)
	if err != nil {
		return "", err
	}
	if len(params) > MaxTaskRequestBytes {
		return "", ErrTaskOverload
	}
	a := s.admission
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.eng == nil {
		return "", ErrTaskAdmissionUnavailable
	}
	tx, err := a.db.Begin()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTaskAdmissionUnavailable, err)
	}
	defer tx.Rollback()
	var cancelled int
	if err = tx.QueryRow(`SELECT count(*) FROM task_admission WHERE id=? AND state='cancelled'`, task.ID).Scan(&cancelled); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTaskAdmissionUnavailable, err)
	}
	if cancelled != 0 {
		return "", ErrTaskOverload
	}
	// A failed result is written after its consumer returned. It can be
	// retried even if the old engine cleanup has not yet retired its claim.
	// Clearing the claim generation prevents that cleanup deleting the retry.
	var retried int
	if err = tx.QueryRow(`SELECT count(*) FROM task_admission q JOIN task_results r ON r.id=q.id WHERE q.id=? AND q.state='active' AND q.generation=r.generation AND r.status='failed'`, task.ID).Scan(&retried); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTaskAdmissionUnavailable, err)
	}
	if retried != 0 {
		if _, err = tx.Exec(`UPDATE task_admission SET type=?,params=?,submitted_at=?,state='pending',generation='' WHERE id=?`, string(task.Type), string(params), time.Now().UTC().Format(time.RFC3339Nano), task.ID); err != nil {
			return "", fmt.Errorf("%w: %v", ErrTaskAdmissionUnavailable, err)
		}
		if err = tx.Commit(); err != nil {
			return "", fmt.Errorf("%w: %v", ErrTaskAdmissionUnavailable, err)
		}
		s.signalAdmission()
		return task.ID, nil
	}
	var found int
	err = tx.QueryRow(`SELECT 1 FROM task_admission WHERE id=? UNION SELECT 1 FROM task_results WHERE id=? AND status IN ('running','completed') LIMIT 1`, task.ID, task.ID).Scan(&found)
	if err == nil {
		return task.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%w: %v", ErrTaskAdmissionUnavailable, err)
	}
	var queued, retained, existing int
	if err = tx.QueryRow("SELECT count(*) FROM task_admission").Scan(&queued); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTaskAdmissionUnavailable, err)
	}
	if err = tx.QueryRow(retainedTaskCount).Scan(&retained); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTaskAdmissionUnavailable, err)
	}
	if err = tx.QueryRow(`SELECT count(*) FROM (SELECT id FROM task_results UNION SELECT task_id AS id FROM tx_markers UNION SELECT id FROM task_cancellations) WHERE id=?`, task.ID).Scan(&existing); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTaskAdmissionUnavailable, err)
	}
	if queued >= MaxQueuedTasks || (existing == 0 && retained >= MaxRetainedTasks) {
		return "", ErrTaskOverload
	}
	_, err = tx.Exec(`INSERT INTO task_admission(id,type,params,submitted_at,state) VALUES(?,?,?,?,'pending')`, task.ID, string(task.Type), string(params), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTaskAdmissionUnavailable, err)
	}
	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTaskAdmissionUnavailable, err)
	}
	s.signalAdmission()
	return task.ID, nil
}

func (s *TaskStore) runAdmission() {
	a := s.admission
	defer a.wg.Done()
	retry := time.NewTicker(time.Second)
	defer retry.Stop()
	for {
		select {
		case <-a.stop:
			return
		case <-a.wake:
		case <-retry.C:
		}
		task, err := s.claimQueued()
		if err != nil {
			serverLog.Error("task queue dispatch unavailable", "err", err)
			continue
		}
		if task == nil {
			continue
		}
		if _, err = a.eng.Submit(*task); err != nil {
			a.mu.Lock()
			released, releaseErr := a.db.Exec(`DELETE FROM task_admission WHERE id=? AND state='cancelled' AND NOT EXISTS (SELECT 1 FROM task_results WHERE id=?)`, task.ID, task.ID)
			a.mu.Unlock()
			if releaseErr != nil {
				serverLog.Error("cancelled dispatch release failed", "err", releaseErr)
				continue
			}
			if n, _ := released.RowsAffected(); n > 0 {
				s.signalAdmission()
				continue
			}
			// Expose an execution refusal as a durable failed result. If persistence
			// itself failed, keep the active claim for recovery instead of dropping it.
			now := time.Now().UTC()
			r := &engine.TaskResult{ID: task.ID, Type: string(task.Type), Params: task.Params,
				Status: engine.TaskStatusFailed, Run: 1, SubmittedAt: now, CompletedAt: &now, Error: err.Error()}
			if saveErr := s.Save(r); saveErr != nil {
				serverLog.Error("retaining task queue recovery claim", "id", task.ID, "err", saveErr)
				continue
			}
			if clearErr := s.ClearCancellation(task.ID, r.Generation); clearErr != nil {
				serverLog.Error("task queue release failed", "err", clearErr)
			}
		}
	}
}

func (s *TaskStore) claimQueued() (*engine.Task, error) {
	a := s.admission
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil, nil
	}
	var busy int
	if err := a.db.QueryRow(`SELECT (SELECT count(*) FROM task_admission WHERE state IN ('active','cancelled')) + (SELECT count(*) FROM task_results WHERE status='running')`).Scan(&busy); err != nil {
		return nil, err
	}
	if busy != 0 {
		return nil, nil
	}
	var task engine.Task
	var params string
	err := a.db.QueryRow(`SELECT id,type,params FROM task_admission WHERE state='pending' ORDER BY sequence LIMIT 1`).Scan(&task.ID, &task.Type, &params)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	task.Params, err = decodeTaskParams([]byte(params))
	if err != nil {
		return nil, err
	}
	if _, err = a.db.Exec(`UPDATE task_admission SET state='active',generation='' WHERE id=? AND state='pending'`, task.ID); err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *TaskStore) queuedResult(id string) (*engine.TaskResult, error) {
	var r engine.TaskResult
	var params, submitted string
	err := s.admission.db.QueryRow(`SELECT id,type,params,submitted_at FROM task_admission WHERE id=? AND state!='cancelled'`, id).Scan(&r.ID, &r.Type, &params, &submitted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Params, err = decodeTaskParams([]byte(params))
	if err != nil {
		return nil, err
	}
	r.SubmittedAt, err = time.Parse(time.RFC3339Nano, submitted)
	if err != nil {
		return nil, err
	}
	r.Status, r.Run = TaskStatusQueued, 1
	return &r, nil
}

func (s *TaskStore) ClearCancellation(id, generation string) error {
	a := s.admission
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := s.SQLiteStore.ClearCancellation(id, generation); err != nil {
		return err
	}
	// Engine calls this only after the handler returns, including after DELETE.
	// A shutdown leaves its running row and active claim intact for restart.
	_, err := a.db.Exec(`DELETE FROM task_admission WHERE id=? AND generation=? AND (state='cancelled' OR (state='active' AND EXISTS (SELECT 1 FROM task_results r WHERE r.id=task_admission.id AND r.generation=? AND r.status IN ('completed','failed'))))`, id, generation, generation)
	if err == nil {
		s.signalAdmission()
	}
	return err
}

func (s *TaskStore) Delete(id string) (bool, error) { return s.deleteAdmitted("id", id) }
func (s *TaskStore) DeleteByType(taskType string) (int, error) {
	n, err := s.deleteAdmittedCount("type", taskType)
	return n, err
}
func (s *TaskStore) deleteAdmitted(column, value string) (bool, error) {
	n, err := s.deleteAdmittedCount(column, value)
	return n > 0, err
}

func (s *TaskStore) deleteAdmittedCount(column, value string) (int, error) {
	a := s.admission
	a.mu.Lock()
	defer a.mu.Unlock()
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	// column is one of the two fixed literals above, never request input.
	var n int
	if err = tx.QueryRow(`SELECT count(*) FROM (SELECT id FROM task_results WHERE `+column+`=? UNION SELECT id FROM task_admission WHERE state!='cancelled' AND `+column+`=?)`, value, value).Scan(&n); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(`INSERT OR IGNORE INTO task_cancellations(id,generation) SELECT id,generation FROM task_results WHERE status='running' AND `+column+`=?`, value); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(`DELETE FROM task_results WHERE `+column+`=?`, value); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(`DELETE FROM task_admission WHERE state='pending' AND `+column+`=?`, value); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(`UPDATE task_admission SET state='cancelled' WHERE state='active' AND `+column+`=?`, value); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	s.signalAdmission()
	return n, nil
}

func (s *TaskStore) Close() error {
	a := s.admission
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	close(a.stop)
	a.mu.Unlock()
	a.wg.Wait()
	err := errors.Join(a.db.Close(), s.SQLiteStore.Close())
	if a.lock != nil {
		err = errors.Join(err, a.lock.Close())
	}
	return err
}

func (s *TaskStore) listWithQueued(limit int) ([]engine.TaskResult, error) {
	results, err := restoreTaskResults(s.SQLiteStore.List(limit))
	if err != nil {
		return nil, err
	}
	rows, err := s.admission.db.Query(`SELECT id FROM task_admission WHERE state!='cancelled' ORDER BY sequence DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(results))
	for _, r := range results {
		seen[r.ID] = true
	}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		r, err := s.queuedResult(id)
		if err != nil {
			return nil, err
		}
		if r != nil {
			results = append(results, *r)
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].SubmittedAt.After(results[j].SubmittedAt) })
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}
