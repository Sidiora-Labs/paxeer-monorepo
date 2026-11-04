package server

import (
	"encoding/json"
	"fmt"

	"github.com/sei-protocol/seictl/sidecar/engine"
)

const exactParamsKey = "__seictl_task_params_json_v1"

// TaskStore keeps parameter JSON inside a string envelope at the SQLite
// boundary, where the underlying store otherwise decodes numbers as float64.
// Every read removes the envelope before handing parameters to the engine.
type TaskStore struct {
	*engine.SQLiteStore
}

func NewTaskStore(path string) (*TaskStore, error) {
	store, err := engine.NewSQLiteStore(path)
	if err != nil {
		return nil, err
	}
	return &TaskStore{SQLiteStore: store}, nil
}

func (s *TaskStore) Save(r *engine.TaskResult) error {
	params, err := json.Marshal(r.Params)
	if err != nil {
		return fmt.Errorf("encode exact task params: %w", err)
	}
	copy := *r
	copy.Params = map[string]any{exactParamsKey: string(params)}
	return s.SQLiteStore.Save(&copy)
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
	return restoreTaskResults(s.SQLiteStore.List(limit))
}

func (s *TaskStore) ListStaleTasks() ([]engine.TaskResult, error) {
	return restoreTaskResults(s.SQLiteStore.ListStaleTasks())
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
