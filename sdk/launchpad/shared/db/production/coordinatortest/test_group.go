package coordinatortest

import "testing"

// TestGroup bounds the number of independent top-level tests using fixture
// services concurrently, including their resource cleanup.
type TestGroup struct {
	slots chan struct{}
}

// NewTestGroup creates a group with capacity for limit concurrent tests.
func NewTestGroup(limit int) *TestGroup {
	if limit < 1 {
		panic("test group limit must be positive")
	}
	return &TestGroup{slots: make(chan struct{}, limit)}
}

// Parallel schedules an independent top-level test with Go's parallel tests
// and waits for a fixture slot. Call it before creating resources or starting
// timeouts; the slot remains occupied until subsequent cleanups finish.
// Tests that change process state or share mutable fixtures must stay sequential.
func (g *TestGroup) Parallel(t *testing.T) {
	t.Helper()
	t.Parallel()
	g.slots <- struct{}{}
	t.Cleanup(func() { <-g.slots })
}
