package control

import (
	"bytes"
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/security/control/enrolment"
	"github.com/Sidiora-Technologies/KindleLaunch/security/control/enrolment/enrolmenttest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOperatorEnrolmentResolvesPrincipal(t *testing.T) {
	database := coordinatortest.Bootstrap(t, enrolmenttest.Start(t), "operator_runtime", "operator_registrar", "control_runtime", "identity_runtime")
	ctx := context.Background()
	connect := func(role string) *pgxpool.Pool {
		t.Helper()
		pool, err := pgxpool.New(ctx, database.Runtimes[role].DSN)
		if err != nil {
			t.Fatal("connect operator role")
		}
		t.Cleanup(pool.Close)
		return pool
	}
	registrar, err := enrolment.New(connect("operator_registrar"), bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	runtimePool := connect("operator_runtime")
	opener, err := enrolment.New(runtimePool, bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	actor := id()
	scopes := []string{"platform", "pool:125"}
	secret, err := registrar.Enrol(ctx, actor, "operator-wallet", "infrastructure", scopes)
	if err != nil {
		t.Fatal(err)
	}
	var now time.Time
	if err := runtimePool.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		t.Fatal(err)
	}
	code, err := enrolment.Code(secret, now)
	if err != nil {
		t.Fatal(err)
	}
	credential, sessionID, err := opener.Open(ctx, actor, code)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := NewPostgresIdentityV2(connect("identity_runtime"), "pnl", "pnl")
	if err != nil {
		t.Fatal(err)
	}
	principal, err := identity.Resolve(ctx, credential)
	if err != nil || principal.ID != actor || principal.Role != "infrastructure" || !reflect.DeepEqual(principal.Scopes, scopes) || principal.SessionID != sessionID || !uuidPattern.MatchString(principal.CredentialID) || !identity.Active(ctx, principal) {
		t.Fatal("operator credential did not resolve its enrolled principal")
	}
	if err := opener.Close(ctx, sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := identity.Resolve(ctx, credential); err != Denied || identity.Active(ctx, principal) {
		t.Fatal("closed operator principal remained active")
	}
}
