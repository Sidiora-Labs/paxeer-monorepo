package errors

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/stretchr/testify/assert"
)

func TestRegisterError(t *testing.T) {
	isolateErrorRegistry(t)
	var error *Error
	// this is the number of errors registered by default in errors.go
	registeredErrorsCount := 16
	assert.Equal(t, len(registry.list()), registeredErrorsCount)
	assert.ElementsMatch(t, registry.list(), ListErrors())
	// add a new Error
	error = RegisterError(69, "nice!", false, "nice!")
	assert.NotNil(t, error)
	// now we have a new error
	registeredErrorsCount++
	assert.Equal(t, len(ListErrors()), registeredErrorsCount)
	// re-register an error should not change anything
	beforeDuplicate := snapshotErrorRegistry(t)
	RegisterError(69, "nice!", false, "nice!")
	assert.Equal(t, len(ListErrors()), registeredErrorsCount)
	assert.Equal(t, beforeDuplicate, snapshotErrorRegistry(t))
	conflicting := RegisterError(69, "conflicting message", true, "conflicting description")
	assert.NotNil(t, conflicting)
	assert.Equal(t, beforeDuplicate, snapshotErrorRegistry(t))
	assert.Equal(t, error.rosErr, registry.errors[69])

	// test sealing
	assert.Equal(t, registry.sealed, false)
	errors := SealAndListErrors()
	assert.Equal(t, registry.sealed, true)
	assert.Equal(t, len(errors), registeredErrorsCount)
	beforeLate := snapshotErrorRegistry(t)
	// add a new error on a sealed registry
	error = RegisterError(1024, "bytes", false, "bytes")
	assert.NotNil(t, error)
	assert.Equal(t, beforeLate, snapshotErrorRegistry(t))
	assert.ElementsMatch(t, errors, ListErrors())
	assert.NotContains(t, registry.errors, int32(1024))
	RegisterError(69, "conflict after seal", true, "conflict after seal")
	assert.Equal(t, beforeLate, snapshotErrorRegistry(t))
	assert.ElementsMatch(t, errors, SealAndListErrors())

}

func isolateErrorRegistry(t *testing.T) {
	t.Helper()
	saved := registry
	registry = errorRegistry{mu: new(sync.RWMutex), errors: make(map[int32]*types.Error)}
	for code, err := range saved.errors {
		registry.errors[code] = err
	}
	t.Cleanup(func() { registry = saved })
}

func snapshotErrorRegistry(t *testing.T) string {
	t.Helper()
	errors := make(map[int32]*types.Error)
	for _, err := range ListErrors() {
		if _, exists := errors[err.Code]; exists {
			t.Fatalf("duplicate error code %d", err.Code)
		}
		errors[err.Code] = err
	}
	snapshot, err := json.Marshal(errors)
	if err != nil {
		t.Fatal(err)
	}
	return string(snapshot)
}

func TestError_Error(t *testing.T) {
	var error *Error
	// nil cases
	assert.False(t, ErrOffline.Is(error))
	error = &Error{}
	assert.False(t, ErrOffline.Is(error))
	// wrong type
	assert.False(t, ErrOffline.Is(&MyError{}))
	// test with wrapping an error
	error = WrapError(ErrOffline, "offline")
	assert.True(t, ErrOffline.Is(error))

	// test equality
	assert.False(t, ErrOffline.Is(ErrBadGateway))
	assert.True(t, ErrBadGateway.Is(ErrBadGateway))
}

func TestToRosetta(t *testing.T) {
	var error *Error
	// nil case
	assert.NotNil(t, ToRosetta(error))
	// wrong type
	assert.NotNil(t, ToRosetta(&MyError{}))
}

type MyError struct {
}

func (e *MyError) Error() string {
	return ""
}
func (e *MyError) Is(err error) bool {
	return true
}
