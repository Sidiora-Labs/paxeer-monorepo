package l16

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5/pgxpool"
)

type liveOwnerSessionFixture struct {
	localCanonicalFixture
	worker   *LiveTransferWorker
	identity auth.HTTPIdentityAuthority
	access   string
	admin    *pgxpool.Pool
}

func newLiveOwnerSessionFixture(t *testing.T) liveOwnerSessionFixture {
	t.Helper()
	f := newLocalCanonicalFixture(t, "gift")
	second := localCanonicalSecondNode(t, f)
	identity, access := localCanonicalIdentity(t, f)
	policy := delivery.Policy{Subjects: map[string]bool{RebuildSubject: true, "portfolio.snapshot.v2": true}, EventTypes: map[string]bool{RebuildSubject: true, "portfolio.snapshot.v2": true}}
	outbox, err := delivery.NewStoreWithEpochGuard(f.authority.pool, "pnl", policy, f.authority.epochGuard)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(f.authority, second, [2]string{"local-primary", "local-fork"}, strings.Repeat("c", 64), outbox)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := NewLiveTransferWorker(service, l17.Repository{Pool: f.authority.pool}, identity, 1)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgxpool.New(f.ctx, f.db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	return liveOwnerSessionFixture{f, worker, identity, access, admin}
}

func (f liveOwnerSessionFixture) principalWithProofAge(t *testing.T, age time.Duration) auth.IdentityPrincipal {
	t.Helper()
	proofAt := time.Now().UTC().Add(-age).Truncate(time.Microsecond)
	tag, err := f.admin.Exec(f.ctx, `UPDATE gateway_identity.identity_sessions SET reauthenticated_at=$2 WHERE wallet=$1`, f.chain.Wallet, proofAt)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatal(tag, err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v3/me/portfolio", nil).WithContext(f.ctx)
	request.Header.Set("Authorization", "Bearer "+f.access)
	p, err := f.identity.AuthenticateRequest(f.ctx, request)
	if err != nil || !p.ProofAt.Equal(proofAt) || !p.AccessExpiresAt.After(time.Now()) {
		t.Fatal(p, err)
	}
	if err = f.identity.CheckStreamSession(f.ctx, p.StreamPrincipal()); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLiveOwnerSessionEnrollmentPreservesProofTime(t *testing.T) {
	parallelDatabaseTest(t)
	f := newLiveOwnerSessionFixture(t)
	for _, age := range []time.Duration{16 * time.Minute, 48 * time.Hour} {
		t.Run(age.String(), func(t *testing.T) {
			p := f.principalWithProofAge(t, age)
			if err := f.worker.EnqueueOwned(f.ctx, p); err != nil {
				t.Fatal(err)
			}
			var proofAt, expiresAt time.Time
			var account, session string
			if err := f.admin.QueryRow(f.ctx, `SELECT proof_at,expires_at,account_id,session_id FROM pnl.inventory_l16_live_owners_v1 WHERE wallet=$1`, p.Wallet).Scan(&proofAt, &expiresAt, &account, &session); err != nil || !proofAt.Equal(p.ProofAt) || !expiresAt.Equal(p.AccessExpiresAt) || account != p.AccountID || session != p.SessionID {
				t.Fatal(proofAt, expiresAt, account, session, err)
			}
		})
	}
}

func TestLiveOwnerSessionCommitsAgedProof(t *testing.T) {
	parallelDatabaseTest(t)
	f := newLiveOwnerSessionFixture(t)
	p := f.principalWithProofAge(t, 48*time.Hour)
	if err := f.worker.EnqueueOwned(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	if count, err := f.worker.RunOnce(f.ctx); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	var accepted, committed int64
	var snapshot string
	if err := f.admin.QueryRow(f.ctx, `SELECT o.accepted_height,c.height,o.accepted_snapshot::text FROM pnl.inventory_l16_live_owners_v1 o JOIN pnl.inventory_l16_live_commits_v1 c ON c.snapshot_id=o.accepted_snapshot WHERE o.wallet=$1`, p.Wallet).Scan(&accepted, &committed, &snapshot); err != nil || accepted != int64(f.chain.Height) || committed != accepted || snapshot == "" {
		t.Fatal(accepted, committed, snapshot, err)
	}
	active, err := f.worker.service.Active(f.ctx, p.Wallet, f.chain.Token)
	if err != nil || active.Height != accepted || active.State.Unknown != "5" {
		t.Fatal(active, err)
	}
}

func TestLiveOwnerSessionExpiryAndRevocation(t *testing.T) {
	parallelDatabaseTest(t)
	f := newLiveOwnerSessionFixture(t)
	p := f.principalWithProofAge(t, time.Minute)
	if err := f.worker.EnqueueOwned(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.admin.Exec(f.ctx, `UPDATE pnl.inventory_l16_live_owners_v1 SET expires_at=clock_timestamp()-interval '1 second' WHERE wallet=$1`, p.Wallet); err != nil {
		t.Fatal(err)
	}
	if count, err := f.worker.RunOnce(f.ctx); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	if err := f.worker.EnqueueOwned(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.admin.Exec(f.ctx, `UPDATE gateway_identity.identity_sessions SET revoked_at=clock_timestamp() WHERE session_id=$1`, p.SessionID); err != nil {
		t.Fatal(err)
	}
	if count, err := f.worker.RunOnce(f.ctx); count != 0 || !errors.Is(err, auth.ErrScopeDenied) {
		t.Fatal(count, err)
	}
	if err := f.worker.EnqueueOwned(f.ctx, p); !errors.Is(err, auth.ErrScopeDenied) {
		t.Fatal(err)
	}
	var height int64
	var commits, snapshots int
	var attempted time.Time
	if err := f.admin.QueryRow(f.ctx, `SELECT accepted_height,last_attempt_at,(SELECT count(*) FROM pnl.inventory_l16_live_commits_v1),(SELECT count(*) FROM pnl.portfolio_snapshots) FROM pnl.inventory_l16_live_owners_v1 WHERE wallet=$1`, p.Wallet).Scan(&height, &attempted, &commits, &snapshots); err != nil || height != 0 || !attempted.Equal(time.Unix(0, 0)) || commits != 0 || snapshots != 0 {
		t.Fatal(height, attempted, commits, snapshots, err)
	}
}

func TestLiveOwnerSessionMiddlewareRequiresRecentProofForWrites(t *testing.T) {
	parallelDatabaseTest(t)
	f := newLiveOwnerSessionFixture(t)
	p := f.principalWithProofAge(t, 16*time.Minute)
	calls := 0
	handler := f.worker.EnrollmentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			request := httptest.NewRequest(method, "/v3/me/portfolio", nil).WithContext(f.ctx)
			request.Header.Set("Authorization", "Bearer "+f.access)
			out := httptest.NewRecorder()
			before := calls
			handler.ServeHTTP(out, request)
			read := method == http.MethodGet || method == http.MethodHead
			if read {
				if out.Code != http.StatusNoContent || calls != before+1 {
					t.Fatal(out.Code, out.Body.String(), calls)
				}
			} else {
				if out.Code != http.StatusForbidden || out.Body.String() != "portfolio enrollment unavailable\n" || calls != before {
					t.Fatal(out.Code, out.Body.String(), calls)
				}
				var enrolled int
				if err := f.admin.QueryRow(f.ctx, `SELECT count(*) FROM pnl.inventory_l16_live_owners_v1 WHERE wallet=$1`, p.Wallet).Scan(&enrolled); err != nil || enrolled != 0 {
					t.Fatal(enrolled, err)
				}
			}
		})
	}
	f.principalWithProofAge(t, time.Minute)
	request := httptest.NewRequest(http.MethodPost, "/v3/me/portfolio", nil).WithContext(f.ctx)
	request.Header.Set("Authorization", "Bearer "+f.access)
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, request)
	if out.Code != http.StatusNoContent || calls != 3 {
		t.Fatal(out.Code, out.Body.String(), calls)
	}
}
