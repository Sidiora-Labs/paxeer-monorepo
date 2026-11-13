package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestIdentityRefreshEnforcesReceiptAndSessionState(t *testing.T) {
	f := fixtureIdentity(t)
	for _, secret := range []string{"short", strings.Repeat("z", 64) + "." + strings.Repeat("z", 64), strings.Repeat("a", 64) + "." + strings.Repeat("b", 64)} {
		if _, err := f.s.Refresh(f.ctx, secret); err == nil {
			t.Fatalf("unissued refresh receipt %q accepted", secret[:5])
		}
	}
	for _, tc := range []struct {
		name   string
		change func(string)
	}{
		{"expired receipt", func(secret string) {
			if _, err := f.pool.Exec(f.ctx, `UPDATE identity_refresh SET expires_at=clock_timestamp()-interval '1 second' WHERE refresh_hash=$1`, digest(secret)); err != nil {
				t.Fatal(err)
			}
		}},
		{"revoked session", func(secret string) {
			if _, err := f.pool.Exec(f.ctx, `UPDATE identity_sessions SET revoked_at=clock_timestamp() WHERE session_id=(SELECT session_id FROM identity_refresh WHERE refresh_hash=$1)`, digest(secret)); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			credentials, _ := f.login(t)
			tc.change(credentials.RefreshSecret)
			if _, err := f.s.Refresh(f.ctx, credentials.RefreshSecret); err == nil {
				t.Fatal("refresh accepted after receipt or session authority ended")
			}
		})
	}
	credentials, principal := f.login(t)
	canceled, stop := context.WithCancel(f.ctx)
	stop()
	if _, err := f.s.Refresh(canceled, credentials.RefreshSecret); err == nil {
		t.Fatal("refresh accepted after request cancellation")
	}
	if err := f.s.Logout(f.ctx, Principal{}, false); err == nil {
		t.Fatal("logout accepted without a principal")
	}
	principal.sessionID = uuid.NewString()
	if err := f.s.Logout(f.ctx, principal, false); err == nil {
		t.Fatal("logout accepted an unowned session")
	}
}
