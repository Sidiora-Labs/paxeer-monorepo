package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"time"

	"github.com/sei-protocol/seictl/sidecar/engine"
)

const exactParamsKey = "__seictl_task_params_json_v1"

// TaskStore keeps parameter JSON inside a string envelope at the SQLite
// boundary, where the underlying store otherwise decodes numbers as float64.
// Every read removes the envelope before handing parameters to the engine.
type TaskStore struct {
	*engine.SQLiteStore
	admission *admissionState
}

func NewTaskStore(path string) (*TaskStore, error) {
	lock, err := lockAdmission(path)
	if err != nil {
		return nil, err
	}
	if path == ":memory:" {
		path = "file:admission-" + uuid.NewString() + "?mode=memory&cache=shared"
	}
	store, err := engine.NewSQLiteStore(path)
	if err != nil {
		if lock != nil {
			lock.Close()
		}
		return nil, err
	}
	admission, err := openAdmission(path)
	if err != nil {
		store.Close()
		if lock != nil {
			lock.Close()
		}
		return nil, fmt.Errorf("open task admission: %w", err)
	}
	admission.lock = lock
	return &TaskStore{SQLiteStore: store, admission: admission}, nil
}

func (s *TaskStore) Save(r *engine.TaskResult) error {
	s.admission.mu.Lock()
	defer s.admission.mu.Unlock()
	if r.Generation == "" {
		var cancelled int
		if err := s.admission.db.QueryRow(`SELECT count(*) FROM task_admission WHERE id=? AND state='cancelled'`, r.ID).Scan(&cancelled); err != nil {
			return err
		}
		if cancelled != 0 {
			return engine.ErrTaskGeneration
		}
		var count, exists int
		if err := s.admission.db.QueryRow(retainedTaskCount).Scan(&count); err != nil {
			return err
		}
		if err := s.admission.db.QueryRow(`SELECT count(*) FROM (SELECT id FROM task_results UNION SELECT id FROM task_admission UNION SELECT task_id AS id FROM tx_markers UNION SELECT id FROM task_cancellations) WHERE id=?`, r.ID).Scan(&exists); err != nil {
			return err
		}
		if count >= MaxRetainedTasks && exists == 0 {
			return ErrTaskOverload
		}
	}
	params, err := json.Marshal(r.Params)
	if err != nil {
		return fmt.Errorf("encode exact task params: %w", err)
	}
	copy := *r
	copy.Params = map[string]any{exactParamsKey: string(params)}
	if err := s.SQLiteStore.Save(&copy); err != nil {
		return err
	}
	if r.Generation == "" {
		if _, err := s.admission.db.Exec(`UPDATE task_admission SET generation=? WHERE id=? AND state='active'`, copy.Generation, r.ID); err != nil {
			return err
		}
	}
	r.Generation = copy.Generation
	return nil
}

func restoreTaskParams(r *engine.TaskResult) error {
	if r == nil {
		return nil
	}
	encoded, ok := r.Params[exactParamsKey]
	if !ok {
		// Old nonnumeric rows remain readable. Numeric rows need decoding
		// from the original SQLite JSON by the engine's migration owner;
		// reconstructing them from float64 could sign a different request.
		if hasDecodedNumber(r.Params) {
			return fmt.Errorf("task %s has legacy numeric params; exact-JSON migration required", r.ID)
		}
		return nil
	}
	raw, ok := encoded.(string)
	if !ok || len(r.Params) != 1 {
		return fmt.Errorf("task %s has an invalid exact-JSON params envelope", r.ID)
	}
	params, err := decodeTaskParams([]byte(raw))
	if err != nil {
		return fmt.Errorf("decode exact task params for %s: %w", r.ID, err)
	}
	r.Params = params
	return nil
}

func hasDecodedNumber(v any) bool {
	switch v := v.(type) {
	case float64:
		return true
	case map[string]any:
		for _, value := range v {
			if hasDecodedNumber(value) {
				return true
			}
		}
	case []any:
		for _, value := range v {
			if hasDecodedNumber(value) {
				return true
			}
		}
	}
	return false
}

func (s *TaskStore) Get(id string) (*engine.TaskResult, error) {
	r, err := s.SQLiteStore.Get(id)
	if err != nil {
		return nil, err
	}
	if err := restoreTaskParams(r); err != nil {
		return nil, err
	}
	if r == nil {
		return s.queuedResult(id)
	}
	return r, nil
}

func restoreTaskResults(results []engine.TaskResult, err error) ([]engine.TaskResult, error) {
	if err != nil {
		return nil, err
	}
	for i := range results {
		if err := restoreTaskParams(&results[i]); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (s *TaskStore) List(limit int) ([]engine.TaskResult, error) {
	return s.listWithQueued(limit)
}

func (s *TaskStore) ListStaleTasks() ([]engine.TaskResult, error) {
	results, err := restoreTaskResults(s.SQLiteStore.ListStaleTasks())
	if err != nil {
		return nil, err
	}
	if len(results) > 1 {
		return nil, errors.New("multiple legacy running tasks require bounded engine recovery")
	}
	// Adopt the single legacy running row before Engine rehydrates it. Its
	// DELETE must hold the same node slot as a task admitted through HTTP.
	if len(results) == 1 {
		s.admission.mu.Lock()
		defer s.admission.mu.Unlock()
		r := results[0]
		params, err := json.Marshal(r.Params)
		if err != nil {
			return nil, err
		}
		if _, err = s.admission.db.Exec(`INSERT OR IGNORE INTO task_admission(id,type,params,submitted_at,generation,state) VALUES(?,?,?,?,?,'active')`, r.ID, r.Type, string(params), r.SubmittedAt.UTC().Format(time.RFC3339Nano), r.Generation); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (s *TaskStore) LatestByType(taskType string) (*engine.TaskResult, error) {
	r, err := s.SQLiteStore.LatestByType(taskType)
	if err != nil {
		return nil, err
	}
	if err := restoreTaskParams(r); err != nil {
		return nil, err
	}
	return r, nil
}

var _ engine.ResultStore = (*TaskStore)(nil)
var _ engine.Checkpointer = (*TaskStore)(nil)
