package engine

import "errors"

var ErrTaskGeneration = errors.New("task generation is no longer current")

// ResultStore persists task results across all lifecycle states.
// Implementations must be safe for concurrent use.
type ResultStore interface {
	// Save assigns a new persisted Generation to a new submission (or retry of
	// a failed task). Updates require the same live Generation; obsolete writes
	// return ErrTaskGeneration and never create a row.
	Save(r *TaskResult) error

	// ClearCancellation retires only this generation's deletion tombstone after
	// its handler has stopped.
	ClearCancellation(id, generation string) error

	// Get returns a result by ID, or (nil, nil) when not found.
	Get(id string) (*TaskResult, error)

	// List returns the most recent results, newest first, up to limit.
	List(limit int) ([]TaskResult, error)

	// ListStaleTasks returns tasks left in "running" state from a
	// previous process that exited without completing them.
	ListStaleTasks() ([]TaskResult, error)

	// Delete removes a result by ID. Returns true if it existed.
	Delete(id string) (bool, error)

	// DeleteByType removes all results of the given task type and returns
	// how many rows were removed. Used by mark-not-ready to purge recorded
	// mark-ready results so a stranded running one cannot rehydrate and
	// release a node hold after a data wipe.
	DeleteByType(taskType string) (int, error)

	// LatestByType returns the most recently submitted result of the given
	// task type (any status), or (nil, nil) when none exists. Rehydration
	// uses it to detect a hold that supersedes a stranded mark-ready even
	// after the hold's own record has gone terminal (e.g. a failed purge
	// persisted it Failed), which a stale-only scan would miss.
	LatestByType(taskType string) (*TaskResult, error)

	// Readiness restores the persisted release/hold lifecycle. Only a completed
	// mark-ready newer than every hold attempt releases the gate; ties are held.
	Readiness() (bool, error)

	// Ping verifies the store is responsive. Used by liveness checks.
	Ping() error

	// Close releases underlying resources.
	Close() error
}
