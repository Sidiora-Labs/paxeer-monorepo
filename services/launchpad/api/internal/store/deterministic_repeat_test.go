package store

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func repeatDeterministicRead[T any](read func() (T, error)) (T, error) {
	for run := 1; ; run++ {
		result, err := read()
		var databaseError *pgconn.PgError
		if run == 5 || !errors.As(err, &databaseError) || databaseError.Code != "57014" {
			return result, err
		}
	}
}

func TestDeterministicReadRepeatsAfterTheStatementLimit(t *testing.T) {
	t.Run("statement limit", func(t *testing.T) {
		runs := 0
		result, err := repeatDeterministicRead(func() (string, error) {
			runs++
			if runs <= 2 {
				return "", fmt.Errorf("read: %w", &pgconn.PgError{Code: "57014"})
			}
			return "snapshot", nil
		})
		if runs != 3 || result != "snapshot" || err != nil {
			t.Fatalf("runs=%d result=%q error=%v", runs, result, err)
		}
	})
	for _, readError := range []error{
		&pgconn.PgError{Code: "42P01"},
		ErrDataIntegrity,
		context.Canceled,
		context.DeadlineExceeded,
		errors.New("canceling statement due to statement timeout (SQLSTATE 57014)"),
	} {
		t.Run(readError.Error(), func(t *testing.T) {
			runs := 0
			result, err := repeatDeterministicRead(func() (string, error) {
				runs++
				return "result", readError
			})
			if runs != 1 || result != "result" || err != readError {
				t.Fatalf("runs=%d result=%q error=%v; want original error %v", runs, result, err, readError)
			}
		})
	}
}

func TestDeterministicReadGivesUpAfterFiveRuns(t *testing.T) {
	runs := 0
	var lastError error
	result, err := repeatDeterministicRead(func() (int, error) {
		runs++
		lastError = &pgconn.PgError{Code: "57014", Detail: fmt.Sprintf("read %d", runs)}
		return runs, lastError
	})
	if runs != 5 || result != 5 || err != lastError {
		t.Fatalf("runs=%d result=%d error=%v; want last error %v", runs, result, err, lastError)
	}
}
