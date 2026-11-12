package rest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestDevelopmentReadErrorClassesStayFinite(t *testing.T) {
	for _, check := range []struct {
		cause error
		class string
	}{
		{&pgconn.PgError{Code: "42P01", Message: "private details"}, "postgres_42P01"},
		{context.DeadlineExceeded, "deadline_exceeded"},
		{context.Canceled, "canceled"},
		{store.ErrDataIntegrity, "data_integrity"},
		{store.ErrAccountingNotReady, "projection_unavailable"},
		{errChainReferenceUnavailable, "chain_reference_unavailable"},
		{ErrDependencyNotConfigured, "dependency_not_configured"},
		{pgx.ErrNoRows, "projection_row_missing"},
		{pgx.ErrTxClosed, "transaction_closed"},
		{pgx.ErrTxCommitRollback, "transaction_rolled_back"},
		{errors.New("private details"), "dependency_error"},
	} {
		got := developmentErrorClass(check.cause)
		if got != check.class || strings.Contains(got, "private details") {
			t.Fatalf("error class for %T: %q, want %q", check.cause, got, check.class)
		}
	}
	if got := PublicETag([]byte("hello")); got != `"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"` {
		t.Fatalf("public validator: %q", got)
	}
}
