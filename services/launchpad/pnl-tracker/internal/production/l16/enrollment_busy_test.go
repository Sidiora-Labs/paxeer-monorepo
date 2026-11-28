package l16

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const enrollmentLockLimit = 250 * time.Millisecond

func enrollmentBusyFixture(t *testing.T) (liveOwnerSessionFixture, auth.IdentityPrincipal) {
	t.Helper()
	f := newLiveOwnerSessionFixture(t)
	config := f.worker.service.authority.pool.Config()
	config.ConnConfig.RuntimeParams["lock_timeout"] = strconv.FormatInt(enrollmentLockLimit.Milliseconds(), 10) + "ms"
	config.ConnConfig.RuntimeParams["search_path"] = "pnl,pg_catalog"
	pool, err := pgxpool.NewWithConfig(f.ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	authority := *f.worker.service.authority
	authority.pool = pool
	service := *f.worker.service
	service.authority = &authority
	worker, err := NewLiveTransferWorker(&service, l17.Repository{Pool: pool}, f.identity, 1)
	if err != nil {
		t.Fatal(err)
	}
	f.worker = worker
	p := f.principalWithProofAge(t, time.Minute)
	if err := f.worker.EnqueueOwned(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	return f, p
}

func holdEnrollmentRow(t *testing.T, f liveOwnerSessionFixture, p auth.IdentityPrincipal) pgx.Tx {
	t.Helper()
	tx, err := f.admin.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { inventoryRollback(tx) })
	var wallet string
	if err := tx.QueryRow(f.ctx, `SELECT wallet FROM pnl.inventory_l16_live_owners_v1 WHERE wallet=$1 FOR UPDATE`, p.Wallet).Scan(&wallet); err != nil || wallet != p.Wallet {
		t.Fatal(wallet, err)
	}
	return tx
}

func enrollmentRequest(f liveOwnerSessionFixture, method string) *http.Request {
	request := httptest.NewRequest(method, "/v3/me/portfolio", nil).WithContext(f.ctx)
	request.Header.Set("Authorization", "Bearer "+f.access)
	return request
}

func captureEnrollmentLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	prior := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(prior) })
	return &output
}

func assertEnrollmentLog(t *testing.T, output *bytes.Buffer, p auth.IdentityPrincipal, method, level string) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("enrollment log count=%d", len(lines))
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["method"] != method || entry["level"] != level || entry["err"] != "ERROR: canceling statement due to lock timeout (SQLSTATE 55P03)" {
		t.Fatalf("enrollment log fields=%v", entry)
	}
	if len(entry) != 5 {
		t.Fatalf("enrollment log fields=%v", entry)
	}
	for _, identifier := range []string{p.Wallet, p.AccountID, p.SessionID} {
		if strings.Contains(output.String(), identifier) {
			t.Fatal("enrollment log contains an owner identifier")
		}
	}
}

func resetEnrollmentBusy(t *testing.T, f liveOwnerSessionFixture, p auth.IdentityPrincipal) auth.IdentityPrincipal {
	t.Helper()
	if _, err := f.admin.Exec(f.ctx, `UPDATE gateway_identity.identity_sessions SET access_expires_at=$2 WHERE session_id=$1`, p.SessionID, p.AccessExpiresAt); err != nil {
		t.Fatal(err)
	}
	if _, err := f.admin.Exec(f.ctx, `DELETE FROM pnl.inventory_l16_live_owners_v1 WHERE wallet=$1`, p.Wallet); err != nil {
		t.Fatal(err)
	}
	if err := f.worker.EnqueueOwned(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEnrollmentBusy(t *testing.T) {
	f, principal := enrollmentBusyFixture(t)
	t.Run("UnchangedRowWritesNothing", func(t *testing.T) {
		p := resetEnrollmentBusy(t, f, principal)
		again, err := f.identity.AuthenticateRequest(f.ctx, enrollmentRequest(f, http.MethodGet))
		if err != nil || !again.AccessExpiresAt.Equal(p.AccessExpiresAt) {
			t.Fatal("session expiry changed between requests", err)
		}
		var before, after time.Time
		var beforeVersion, afterVersion string
		if err := f.admin.QueryRow(f.ctx, `SELECT updated_at,xmin::text FROM pnl.inventory_l16_live_owners_v1 WHERE wallet=$1`, p.Wallet).Scan(&before, &beforeVersion); err != nil {
			t.Fatal(err)
		}
		holdEnrollmentRow(t, f, p)
		started := time.Now()
		if err := f.worker.EnqueueOwned(f.ctx, p); err != nil {
			t.Fatal(err)
		}
		if elapsed := time.Since(started); elapsed >= enrollmentLockLimit {
			t.Fatalf("unchanged enrollment took %s, lock limit %s", elapsed, enrollmentLockLimit)
		}
		if err := f.admin.QueryRow(f.ctx, `SELECT updated_at,xmin::text FROM pnl.inventory_l16_live_owners_v1 WHERE wallet=$1`, p.Wallet).Scan(&after, &afterVersion); err != nil || !before.Equal(after) || beforeVersion != afterVersion {
			t.Fatal("unchanged enrollment wrote its row", err)
		}
	})
	t.Run("NewExpiryWritesWhenFree", func(t *testing.T) {
		p := resetEnrollmentBusy(t, f, principal)
		held := holdEnrollmentRow(t, f, p)
		var before time.Time
		if err := f.admin.QueryRow(f.ctx, `SELECT updated_at FROM pnl.inventory_l16_live_owners_v1 WHERE wallet=$1`, p.Wallet).Scan(&before); err != nil {
			t.Fatal(err)
		}
		if err := held.Rollback(f.ctx); err != nil {
			t.Fatal(err)
		}
		p.AccessExpiresAt = p.AccessExpiresAt.Add(time.Minute)
		if _, err := f.admin.Exec(f.ctx, `UPDATE gateway_identity.identity_sessions SET access_expires_at=$2 WHERE session_id=$1`, p.SessionID, p.AccessExpiresAt); err != nil {
			t.Fatal(err)
		}
		if err := f.worker.EnqueueOwned(f.ctx, p); err != nil {
			t.Fatal(err)
		}
		var updated, expires time.Time
		if err := f.admin.QueryRow(f.ctx, `SELECT updated_at,expires_at FROM pnl.inventory_l16_live_owners_v1 WHERE wallet=$1`, p.Wallet).Scan(&updated, &expires); err != nil || !updated.After(before) || !expires.Equal(p.AccessExpiresAt) {
			t.Fatal("renewed expiry was not stored", err)
		}
	})
	t.Run("ReadPassesWithLiveAccount", func(t *testing.T) {
		p := resetEnrollmentBusy(t, f, principal)
		summary, err := f.worker.RebuildOwned(f.ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.admin.Exec(f.ctx, `UPDATE pnl.inventory_l16_live_owners_v1 SET expires_at=expires_at-interval '1 minute' WHERE wallet=$1`, p.Wallet); err != nil {
			t.Fatal(err)
		}
		holdEnrollmentRow(t, f, p)
		api := l17.Service{Repository: f.worker.repository, Authority: f.identity, CursorKey: []byte(strings.Repeat("k", 32))}
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(method, func(t *testing.T) {
				output := captureEnrollmentLog(t)
				called := false
				handler := f.worker.EnrollmentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					called = true
					api.PortfolioV3HTTP(w, r)
				}))
				out := httptest.NewRecorder()
				started := time.Now()
				handler.ServeHTTP(out, enrollmentRequest(f, method))
				if out.Code != http.StatusOK || !called || !strings.Contains(out.Body.String(), summary.SnapshotID) {
					t.Fatal(out.Code, out.Body.String(), called)
				}
				if time.Since(started) < enrollmentLockLimit {
					t.Fatal("changed enrollment did not wait for its lock limit")
				}
				assertEnrollmentLog(t, output, p, method, "WARN")
			})
		}
	})
	t.Run("WriteRefusedAndLogged", func(t *testing.T) {
		p := resetEnrollmentBusy(t, f, principal)
		if _, err := f.admin.Exec(f.ctx, `UPDATE pnl.inventory_l16_live_owners_v1 SET expires_at=expires_at-interval '1 minute' WHERE wallet=$1`, p.Wallet); err != nil {
			t.Fatal(err)
		}
		holdEnrollmentRow(t, f, p)
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions} {
			t.Run(method, func(t *testing.T) {
				output := captureEnrollmentLog(t)
				called := false
				handler := f.worker.EnrollmentMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
				out := httptest.NewRecorder()
				started := time.Now()
				handler.ServeHTTP(out, enrollmentRequest(f, method))
				if out.Code != http.StatusServiceUnavailable || called || out.Body.String() != "portfolio enrollment unavailable\n" {
					t.Fatal(out.Code, out.Body.String(), called)
				}
				if time.Since(started) < enrollmentLockLimit {
					t.Fatal("changed enrollment did not wait for its lock limit")
				}
				assertEnrollmentLog(t, output, p, method, "ERROR")
			})
		}
	})
	t.Run("ReadRequiresLiveAccount", func(t *testing.T) {
		p := resetEnrollmentBusy(t, f, principal)
		for _, state := range []string{"cancelled", "expired", "other_account"} {
			t.Run(state, func(t *testing.T) {
				if err := f.worker.EnqueueOwned(f.ctx, p); err != nil {
					t.Fatal(err)
				}
				switch state {
				case "cancelled":
					if err := f.worker.CancelOwned(f.ctx, p); err != nil {
						t.Fatal(err)
					}
				case "expired":
					if _, err := f.admin.Exec(f.ctx, `UPDATE pnl.inventory_l16_live_owners_v1 SET expires_at=clock_timestamp()-interval '1 second' WHERE wallet=$1`, p.Wallet); err != nil {
						t.Fatal(err)
					}
				case "other_account":
					if _, err := f.admin.Exec(f.ctx, `UPDATE pnl.inventory_l16_live_owners_v1 SET account_id='33333333-3333-4333-8333-333333333333' WHERE wallet=$1`, p.Wallet); err != nil {
						t.Fatal(err)
					}
				}
				holdEnrollmentRow(t, f, p)
				output := captureEnrollmentLog(t)
				called := false
				handler := f.worker.EnrollmentMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
				out := httptest.NewRecorder()
				started := time.Now()
				handler.ServeHTTP(out, enrollmentRequest(f, http.MethodGet))
				if out.Code != http.StatusServiceUnavailable || called || out.Body.String() != "portfolio enrollment unavailable\n" {
					t.Fatal(out.Code, out.Body.String(), called)
				}
				if time.Since(started) < enrollmentLockLimit {
					t.Fatal("changed enrollment did not wait for its lock limit")
				}
				assertEnrollmentLog(t, output, p, http.MethodGet, "ERROR")
			})
		}
	})
	t.Run("ErrorWrapsLockTimeout", func(t *testing.T) {
		p := resetEnrollmentBusy(t, f, principal)
		p.AccessExpiresAt = p.AccessExpiresAt.Add(-time.Minute)
		holdEnrollmentRow(t, f, p)
		started := time.Now()
		err := f.worker.EnqueueOwned(f.ctx, p)
		var busy *enrollmentBusyError
		var postgres *pgconn.PgError
		if !errors.As(err, &busy) || !errors.As(err, &postgres) || postgres.Code != "55P03" || errors.Unwrap(busy) != postgres {
			t.Fatal("enrollment lock timeout must preserve its database cause", err)
		}
		if time.Since(started) < enrollmentLockLimit {
			t.Fatal("changed enrollment did not wait for its lock limit")
		}
		ctx, cancel := context.WithCancel(f.ctx)
		cancel()
		err = f.worker.EnqueueOwned(ctx, p)
		if errors.As(err, &busy) {
			t.Fatal("cancellation was classified as a busy enrollment")
		}
		if err == nil {
			t.Fatal("cancelled enrollment succeeded")
		}
	})
	t.Run("StatementTimeoutRefused", func(t *testing.T) {
		p := resetEnrollmentBusy(t, f, principal)
		if _, err := f.admin.Exec(f.ctx, `UPDATE pnl.inventory_l16_live_owners_v1 SET expires_at=expires_at-interval '1 minute' WHERE wallet=$1`, p.Wallet); err != nil {
			t.Fatal(err)
		}
		holdEnrollmentRow(t, f, p)
		config := f.worker.service.authority.pool.Config()
		statementLimit := enrollmentLockLimit / 2
		config.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(statementLimit.Milliseconds(), 10) + "ms"
		pool, err := pgxpool.NewWithConfig(f.ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		authority := *f.worker.service.authority
		authority.pool = pool
		service := *f.worker.service
		service.authority = &authority
		worker, err := NewLiveTransferWorker(&service, l17.Repository{Pool: pool}, f.identity, 1)
		if err != nil {
			t.Fatal(err)
		}
		output := captureEnrollmentLog(t)
		called := false
		handler := worker.EnrollmentMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
		out := httptest.NewRecorder()
		started := time.Now()
		handler.ServeHTTP(out, enrollmentRequest(f, http.MethodGet))
		if out.Code != http.StatusServiceUnavailable || called || out.Body.String() != "portfolio enrollment unavailable\n" {
			t.Fatal(out.Code, out.Body.String(), called)
		}
		if time.Since(started) < statementLimit {
			t.Fatal("enrollment did not wait for its statement limit")
		}
		lines := strings.Split(strings.TrimSpace(output.String()), "\n")
		if len(lines) != 1 {
			t.Fatalf("enrollment log count=%d", len(lines))
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
			t.Fatal(err)
		}
		if entry["method"] != http.MethodGet || entry["level"] != "ERROR" || entry["err"] != "ERROR: canceling statement due to statement timeout (SQLSTATE 57014)" || len(entry) != 5 {
			t.Fatalf("enrollment log fields=%v", entry)
		}
		for _, identifier := range []string{p.Wallet, p.AccountID, p.SessionID} {
			if strings.Contains(output.String(), identifier) {
				t.Fatal("enrollment log contains an owner identifier")
			}
		}
	})
}
