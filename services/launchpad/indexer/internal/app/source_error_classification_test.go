package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCanonicalSourceReadErrorsKeepBoundedOperationalClasses(t *testing.T) {
	for _, tc := range []struct {
		err   error
		class string
	}{
		{pgx.ErrNoRows, "missing_watermark"},
		{&pgconn.PgError{Code: "42P01", Message: "relation unavailable"}, "database"},
		{errors.New("source read unavailable"), "read_error"},
	} {
		if got := canonicalSourceReadErrorClass(fmt.Errorf("checkpoint: %w", tc.err)); got != tc.class {
			t.Fatalf("source error class %q, want %q", got, tc.class)
		}
	}
}
