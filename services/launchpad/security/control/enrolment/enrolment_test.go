package enrolment

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/security/control/enrolment/enrolmenttest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOperatorRFC6238(t *testing.T) {
	secret := []byte("12345678901234567890")
	for _, vector := range []struct {
		seconds int64
		want    string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	} {
		got, err := Code(secret, time.Unix(vector.seconds, 0))
		if err != nil || got != vector.want {
			t.Fatalf("RFC 6238 SHA-1 vector at %d", vector.seconds)
		}
	}
	for _, secret := range [][]byte{nil, make([]byte, 19), make([]byte, 21)} {
		if _, err := Code(secret, time.Unix(59, 0)); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid factor length accepted")
		}
	}
	if _, err := Code(secret, time.Unix(-1, 0)); !errors.Is(err, ErrInvalid) {
		t.Fatal("negative clock accepted")
	}
}

func TestOperatorSealing(t *testing.T) {
	secret := []byte("12345678901234567890")
	key := bytes.Repeat([]byte{42}, 32)
	actor := "11111111-1111-4111-8111-111111111111"
	sealed, err := seal(secret, key, actor)
	if err != nil || len(sealed) != 12+20+16 || bytes.Contains(sealed, secret) {
		t.Fatal("factor must be sealed with a twelve-byte nonce")
	}
	opened, err := unseal(sealed, key, actor)
	if err != nil || !bytes.Equal(secret, opened) {
		t.Fatal("factor opening did not recover the enrolment")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	opened, err = aead.Open(nil, sealed[:12], sealed[12:], []byte(actor))
	if err != nil || !bytes.Equal(opened, secret) {
		t.Fatal("AES-256-GCM encoding or actor binding differs")
	}
	other, err := seal(secret, key, actor)
	if err != nil || bytes.Equal(other[:12], sealed[:12]) {
		t.Fatal("factor sealing reused a nonce")
	}
	tampered := append([]byte(nil), sealed...)
	tampered[len(tampered)-1] ^= 1
	for _, input := range []struct {
		sealed, key []byte
		actor       string
	}{
		{sealed, key, "22222222-2222-4222-8222-222222222222"},
		{sealed, bytes.Repeat([]byte{43}, 32), actor},
		{nil, key, actor},
		{sealed[:12], key, actor},
		{sealed[:len(sealed)-1], key, actor},
		{append(append([]byte(nil), sealed...), 0), key, actor},
		{tampered, key, actor},
	} {
		if opened, err := unseal(input.sealed, input.key, input.actor); !errors.Is(err, ErrDenied) || opened != nil {
			t.Fatal("unauthenticated factor opening accepted")
		}
	}
	if _, err := seal(secret, key[:31], actor); !errors.Is(err, ErrInvalid) {
		t.Fatal("short sealing key accepted")
	}
	if _, err := unseal(sealed, key[:31], actor); !errors.Is(err, ErrInvalid) {
		t.Fatal("short opening key accepted")
	}
	if _, err := New(nil, key); !errors.Is(err, ErrInvalid) {
		t.Fatal("missing pool accepted")
	}
}

type operatorFixture struct {
	ctx                                 context.Context
	db                                  coordinatortest.Database
	admin, runtime, registrar, identity *pgxpool.Pool
	opener, enroler                     *Store
	key                                 []byte
}

func operatorDatabase(t *testing.T) operatorFixture {
	t.Helper()
	db := coordinatortest.Bootstrap(t, enrolmenttest.Start(t), "operator_runtime", "operator_registrar", "control_runtime", "identity_runtime")
	ctx := context.Background()
	connect := func(dsn string) *pgxpool.Pool {
		t.Helper()
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			t.Fatal("create operator connection")
		}
		t.Cleanup(pool.Close)
		return pool
	}
	f := operatorFixture{ctx: ctx, db: db, admin: connect(db.AdminDSN), runtime: connect(db.Runtimes["operator_runtime"].DSN), registrar: connect(db.Runtimes["operator_registrar"].DSN), identity: connect(db.Runtimes["identity_runtime"].DSN), key: bytes.Repeat([]byte{17}, 32)}
	var err error
	f.opener, err = New(f.runtime, f.key)
	if err != nil {
		t.Fatal(err)
	}
	f.enroler, err = New(f.registrar, f.key)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f operatorFixture) enrol(t *testing.T, role string) (string, []byte) {
	t.Helper()
	actor, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	secret, err := f.enroler.Enrol(f.ctx, actor, "operator-wallet", role, []string{"platform", "pool:125"})
	if err != nil || len(secret) != factorBytes {
		t.Fatal("enrol operator:", err)
	}
	return actor, secret
}

func (f operatorFixture) now(t *testing.T) time.Time {
	t.Helper()
	var now time.Time
	if err := f.admin.QueryRow(f.ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		t.Fatal(err)
	}
	return now
}

func (f operatorFixture) visible(t *testing.T, credential string, want int) {
	t.Helper()
	hash := sha256.Sum256([]byte(credential))
	var count int
	if err := f.identity.QueryRow(f.ctx, "SELECT count(*) FROM pnl.authority_credentials_v2 WHERE token_hash=$1", hash[:]).Scan(&count); err != nil || count != want {
		t.Fatalf("operator authority view rows=%d want=%d: %v", count, want, err)
	}
}

func (f operatorFixture) lastStep(t *testing.T, actor string) int64 {
	t.Helper()
	var step int64
	if err := f.admin.QueryRow(f.ctx, "SELECT last_step FROM security.operator_enrolments WHERE actor=$1", actor).Scan(&step); err != nil {
		t.Fatal(err)
	}
	return step
}

func TestOperatorSessions(t *testing.T) {
	f := operatorDatabase(t)
	t.Run("registered enrolment constraints", func(t *testing.T) {
		var required int
		var public bool
		if err := f.admin.QueryRow(f.ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema='security' AND table_name='operator_enrolments' AND is_nullable='NO'").Scan(&required); err != nil || required != 7 {
			t.Fatal("enrolment columns must be nonnullable except revocation")
		}
		if err := f.admin.QueryRow(f.ctx, "SELECT EXISTS(SELECT 1 FROM pg_class c CROSS JOIN LATERAL aclexplode(coalesce(c.relacl,acldefault('r',c.relowner))) a WHERE c.oid='security.operator_enrolments'::regclass AND a.grantee=0)").Scan(&public); err != nil || public {
			t.Fatal("enrolment table permits public access")
		}
		for _, input := range []struct {
			role   string
			scopes []string
		}{
			{"user", []string{"platform"}}, {"unknown", []string{"platform"}}, {"support", []string{}}, {"support", make([]string, 33)},
		} {
			_, err := f.admin.Exec(f.ctx, "INSERT INTO security.operator_enrolments(actor,wallet,role,scopes,factor) VALUES(gen_random_uuid(),'wallet',$1,$2,'factor')", input.role, input.scopes)
			var constraint *pgconn.PgError
			if !errors.As(err, &constraint) || constraint.Code != "23514" {
				t.Fatal("enrolment role or scope constraint was not enforced")
			}
		}
	})
	t.Run("reviewed roles pass readiness", func(t *testing.T) {
		for _, name := range []string{"operator_runtime", "operator_registrar"} {
			rt := f.db.Runtimes[name]
			expected, err := production.LoadExpectedSchema(rt.ExpectedSchema, rt.ExpectedSchemaSHA256)
			if err != nil {
				t.Fatal(err)
			}
			pool := f.runtime
			if name == "operator_registrar" {
				pool = f.registrar
			}
			if err := production.CheckReadiness(f.ctx, pool, expected); err != nil {
				t.Fatal("operator readiness:", err)
			}
		}
	})
	t.Run("enrolment and opening bind authority", func(t *testing.T) {
		before := f.now(t)
		actor, secret := f.enrol(t, "infrastructure")
		var wallet, role string
		var scopes []string
		var sealed []byte
		var step int64
		var enrolledAt time.Time
		var revokedAt *time.Time
		if err := f.admin.QueryRow(f.ctx, "SELECT wallet,role,scopes,factor,last_step,enrolled_at,revoked_at FROM security.operator_enrolments WHERE actor=$1", actor).Scan(&wallet, &role, &scopes, &sealed, &step, &enrolledAt, &revokedAt); err != nil {
			t.Fatal(err)
		}
		opened, err := unseal(sealed, f.key, actor)
		if err != nil || !bytes.Equal(opened, secret) || bytes.Equal(sealed, secret) || wallet != "operator-wallet" || role != "infrastructure" || !reflect.DeepEqual(scopes, []string{"platform", "pool:125"}) || step != 0 || enrolledAt.Before(before) || enrolledAt.After(f.now(t)) || revokedAt != nil {
			t.Fatal("stored enrolment does not bind its factor and operator")
		}
		if secret, err := f.enroler.Enrol(f.ctx, actor, "other-wallet", "finance", []string{"other"}); !errors.Is(err, ErrEnrolled) || secret != nil {
			t.Fatal("active enrolment replaced")
		}
		code, err := Code(secret, f.now(t))
		if err != nil {
			t.Fatal(err)
		}
		credential, sessionID, err := f.opener.Open(f.ctx, strings.ToUpper(actor), code)
		if err != nil {
			t.Fatal("open operator:", err)
		}
		if raw, err := hex.DecodeString(credential); err != nil || len(raw) != 32 || credential != strings.ToLower(credential) || !uuidPattern.MatchString(sessionID) || sessionID != strings.ToLower(sessionID) || sessionID[14] != '4' {
			t.Fatal("operator authority encoding differs")
		}
		var sessionActor, audience, credentialID, credentialActor, credentialSession string
		var issued, expires, mfaExpires, credentialExpires time.Time
		var revoked bool
		if err := f.admin.QueryRow(f.ctx, "SELECT wallet,actor,audience,issued_at,expires_at,mfa_expires_at,scopes,revoked FROM security.operator_sessions WHERE session_id=$1", sessionID).Scan(&wallet, &sessionActor, &audience, &issued, &expires, &mfaExpires, &scopes, &revoked); err != nil {
			t.Fatal(err)
		}
		if wallet != "operator-wallet" || sessionActor != actor || audience != "control" || issued.Before(before) || issued.After(f.now(t)) || expires.Sub(issued) != sessionLifetime || !mfaExpires.Equal(expires) || revoked || !reflect.DeepEqual(scopes, []string{"platform", "pool:125"}) {
			t.Fatal("operator session binding or lifetime differs")
		}
		var storedHash []byte
		if err := f.admin.QueryRow(f.ctx, "SELECT credential_id::text,actor_id::text,role,scopes,token_hash,session_id::text,expires_at,revoked_at FROM pnl.authority_credentials WHERE session_id=$1", sessionID).Scan(&credentialID, &credentialActor, &role, &scopes, &storedHash, &credentialSession, &credentialExpires, &revokedAt); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte(credential))
		if !uuidPattern.MatchString(credentialID) || credentialID == sessionID || credentialActor != actor || role != "infrastructure" || !reflect.DeepEqual(scopes, []string{"platform", "pool:125"}) || !bytes.Equal(storedHash, hash[:]) || credentialSession != sessionID || !credentialExpires.Equal(expires) || revokedAt != nil || f.lastStep(t, actor) != issued.Unix()/stepSeconds {
			t.Fatal("operator credential does not bind its session and enrolment")
		}
		f.visible(t, credential, 1)
		if token, session, err := f.opener.Open(f.ctx, actor, code); !errors.Is(err, ErrDenied) || token != "" || session != "" {
			t.Fatal("one-time step replay accepted")
		}
		if err := f.opener.Close(f.ctx, strings.ToUpper(sessionID)); err != nil {
			t.Fatal(err)
		}
		f.visible(t, credential, 0)
		if err := f.admin.QueryRow(f.ctx, "SELECT revoked FROM security.operator_sessions WHERE session_id=$1", sessionID).Scan(&revoked); err != nil || !revoked {
			t.Fatal("closed session still active")
		}
		if err := f.admin.QueryRow(f.ctx, "SELECT revoked_at FROM pnl.authority_credentials WHERE session_id=$1", sessionID).Scan(&revokedAt); err != nil || revokedAt == nil {
			t.Fatal("closed credential still active")
		}
		if err := f.opener.Close(f.ctx, sessionID); err != nil {
			t.Fatal("close is not idempotent")
		}
	})
	t.Run("previous step and revocation close every authority", func(t *testing.T) {
		actor, secret := f.enrol(t, "finance")
		now := f.now(t)
		if remaining := time.Duration(stepSeconds-now.Unix()%stepSeconds)*time.Second - time.Duration(now.Nanosecond()); remaining < 5*time.Second {
			time.Sleep(remaining + 100*time.Millisecond)
			now = f.now(t)
		}
		previous, err := Code(secret, now.Add(-30*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		first, firstSession, err := f.opener.Open(f.ctx, actor, previous)
		if err != nil {
			t.Fatal("previous operator step refused:", err)
		}
		if step := f.lastStep(t, actor); step != now.Unix()/stepSeconds-1 {
			t.Fatal("previous step was not consumed")
		}
		current, err := Code(secret, now)
		if err != nil {
			t.Fatal(err)
		}
		second, secondSession, err := f.opener.Open(f.ctx, actor, current)
		if err != nil || firstSession == secondSession || first == second {
			t.Fatal("fresh operator step did not create fresh authority")
		}
		f.visible(t, first, 1)
		f.visible(t, second, 1)
		if err := f.enroler.Revoke(f.ctx, actor); err != nil {
			t.Fatal(err)
		}
		f.visible(t, first, 0)
		f.visible(t, second, 0)
		var openSessions, openCredentials int
		var revokedAt *time.Time
		if err := f.admin.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM security.operator_sessions WHERE actor=$1 AND NOT revoked),(SELECT count(*) FROM pnl.authority_credentials WHERE actor_id=$1::uuid AND revoked_at IS NULL),revoked_at FROM security.operator_enrolments WHERE actor=$1::uuid`, actor).Scan(&openSessions, &openCredentials, &revokedAt); err != nil || openSessions != 0 || openCredentials != 0 || revokedAt == nil {
			t.Fatal("operator revocation left open authority")
		}
		if token, session, err := f.opener.Open(f.ctx, actor, current); !errors.Is(err, ErrDenied) || token != "" || session != "" {
			t.Fatal("revoked operator authenticated")
		}
		if err := f.enroler.Revoke(f.ctx, actor); err != nil {
			t.Fatal("revoke is not idempotent")
		}
		replacement, err := f.enroler.Enrol(f.ctx, strings.ToUpper(actor), "replacement-wallet", "support", []string{"support:125"})
		if err != nil || len(replacement) != factorBytes || bytes.Equal(replacement, secret) || f.lastStep(t, actor) != 0 {
			t.Fatal("revoked operator could not receive a new factor")
		}
		next, err := Code(replacement, f.now(t))
		if err != nil {
			t.Fatal(err)
		}
		token, _, err := f.opener.Open(f.ctx, actor, next)
		if err != nil {
			t.Fatal("replacement enrolment did not authenticate")
		}
		f.visible(t, token, 1)
		f.visible(t, first, 0)
		f.visible(t, second, 0)
	})
	t.Run("wrong and stale codes leave enrolment unchanged", func(t *testing.T) {
		actor, secret := f.enrol(t, "moderator")
		now := f.now(t)
		wrong := "000000"
		for n := 1; wrong == codeForStep(secret, uint64(now.Unix()/stepSeconds-1)) || wrong == codeForStep(secret, uint64(now.Unix()/stepSeconds)) || wrong == codeForStep(secret, uint64(now.Unix()/stepSeconds+1)); n++ {
			wrong = fmt.Sprintf("%06d", n)
		}
		for _, code := range []string{wrong, "", "12345", "1234567", "abcdef", "00000\n", codeForStep(secret, uint64(now.Unix()/stepSeconds-2)), codeForStep(secret, uint64(now.Unix()/stepSeconds+2))} {
			if token, session, err := f.opener.Open(f.ctx, actor, code); !errors.Is(err, ErrDenied) || token != "" || session != "" || f.lastStep(t, actor) != 0 {
				t.Fatal("invalid operator proof consumed a step")
			}
		}
		var sessions, credentials int
		if err := f.admin.QueryRow(f.ctx, "SELECT (SELECT count(*) FROM security.operator_sessions WHERE actor=$1),(SELECT count(*) FROM pnl.authority_credentials WHERE actor_id=$1::uuid)", actor).Scan(&sessions, &credentials); err != nil || sessions != 0 || credentials != 0 {
			t.Fatal("invalid operator proof created authority")
		}
		current, err := Code(secret, f.now(t))
		if err != nil {
			t.Fatal(err)
		}
		otherKey, err := New(f.runtime, bytes.Repeat([]byte{18}, 32))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := otherKey.Open(f.ctx, actor, current); !errors.Is(err, ErrDenied) || f.lastStep(t, actor) != 0 {
			t.Fatal("another sealing key authenticated an operator")
		}
		if _, err := f.admin.Exec(f.ctx, "UPDATE security.operator_enrolments SET last_step=$2 WHERE actor=$1", actor, now.Unix()/stepSeconds+3); err != nil {
			t.Fatal(err)
		}
		if _, _, err := f.opener.Open(f.ctx, actor, current); !errors.Is(err, ErrDenied) {
			t.Fatal("non-increasing step accepted")
		}
	})
	t.Run("concurrent replay consumes one step", func(t *testing.T) {
		actor, secret := f.enrol(t, "support")
		code, err := Code(secret, f.now(t))
		if err != nil {
			t.Fatal(err)
		}
		results := make(chan error, 2)
		var start sync.WaitGroup
		start.Add(1)
		for range 2 {
			go func() {
				start.Wait()
				_, _, err := f.opener.Open(f.ctx, actor, code)
				results <- err
			}()
		}
		start.Done()
		var accepted, denied int
		for range 2 {
			switch err := <-results; {
			case err == nil:
				accepted++
			case errors.Is(err, ErrDenied):
				denied++
			default:
				t.Fatal("concurrent authentication:", err)
			}
		}
		if accepted != 1 || denied != 1 {
			t.Fatal("concurrent replay created duplicate authority")
		}
	})
	t.Run("runtime cannot register or revoke operators", func(t *testing.T) {
		actor, secret := f.enrol(t, "infrastructure")
		for _, statement := range []string{
			"INSERT INTO security.operator_enrolments(actor,wallet,role,scopes,factor) VALUES(gen_random_uuid(),'wallet','support',ARRAY['platform'],'factor')",
			"UPDATE security.operator_enrolments SET revoked_at=clock_timestamp() WHERE actor=$1",
		} {
			args := []any{}
			if strings.Contains(statement, "$1") {
				args = append(args, actor)
			}
			_, err := f.runtime.Exec(f.ctx, statement, args...)
			var permission *pgconn.PgError
			if !errors.As(err, &permission) || permission.Code != "42501" {
				t.Fatal("operator runtime exceeded its column privileges")
			}
		}
		code, err := Code(secret, f.now(t))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := f.opener.Open(f.ctx, actor, code); err != nil {
			t.Fatal("refused runtime writes altered the enrolment")
		}
	})
	t.Run("invalid enrolments and missing actors are refused", func(t *testing.T) {
		actor, err := newID()
		if err != nil {
			t.Fatal(err)
		}
		for _, input := range []struct {
			actor, wallet, role string
			scopes              []string
		}{
			{"invalid", "wallet", "finance", []string{"platform"}},
			{actor, "", "finance", []string{"platform"}},
			{actor, " wallet", "finance", []string{"platform"}},
			{actor, "wallet\n", "finance", []string{"platform"}},
			{actor, "wallet", "user", []string{"platform"}},
			{actor, "wallet", "finance", nil},
			{actor, "wallet", "finance", make([]string, 33)},
			{actor, "wallet", "finance", []string{"bad scope"}},
		} {
			if secret, err := f.enroler.Enrol(f.ctx, input.actor, input.wallet, input.role, input.scopes); !errors.Is(err, ErrInvalid) || secret != nil {
				t.Fatal("invalid enrolment accepted")
			}
		}
		for _, invalid := range []string{actor, "invalid"} {
			if _, _, err := f.opener.Open(f.ctx, invalid, "000000"); !errors.Is(err, ErrDenied) {
				t.Fatal("missing operator authenticated")
			}
		}
		if err := f.opener.Close(f.ctx, "invalid"); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid session identifier accepted")
		}
		if err := f.enroler.Revoke(f.ctx, "invalid"); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid actor revocation accepted")
		}
		if _, err := New(f.runtime, f.key[:31]); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid store key accepted")
		}
	})
}
