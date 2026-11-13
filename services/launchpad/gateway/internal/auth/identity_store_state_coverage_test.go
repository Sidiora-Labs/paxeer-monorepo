package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIdentitySessionStoreRejectsChangedLiveState(t *testing.T) {
	f := fixtureIdentity(t)
	credentials, principal := f.login(t)
	validLink := WalletLinkChallengeRequest{Wallet: f.wallet, ChainID: "125", Visibility: privateVisibility}

	unknown := strings.Repeat("a", 64) + "." + strings.Repeat("b", 64)
	if _, err := f.s.Refresh(f.ctx, unknown); err == nil {
		t.Fatal("unknown refresh family was accepted")
	}
	if _, err := f.pool.Exec(f.ctx, `UPDATE identity_refresh SET expires_at=$2 WHERE refresh_hash=$1`, digest(credentials.RefreshSecret), f.s.now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.Refresh(f.ctx, credentials.RefreshSecret); err == nil {
		t.Fatal("expired refresh family was accepted")
	}
	var consumed *time.Time
	if err := f.pool.QueryRow(f.ctx, `SELECT consumed_at FROM identity_refresh WHERE refresh_hash=$1`, digest(credentials.RefreshSecret)).Scan(&consumed); err != nil || consumed != nil {
		t.Fatalf("expired refresh was consumed: at=%v err=%v", consumed, err)
	}

	for _, tc := range []struct {
		name         string
		edit         func(*Principal)
		logoutDenied bool
	}{
		{"other account", func(p *Principal) { p.accountID = uuid.NewString() }, true},
		{"other session", func(p *Principal) { p.sessionID = uuid.NewString() }, true},
		{"other epoch", func(p *Principal) { p.epoch++ }, true},
		{"other wallet", func(p *Principal) { p.wallet = "0x" + strings.Repeat("0", 40) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := principal
			tc.edit(&changed)
			if err := f.s.CheckSocket(f.ctx, changed); err == nil {
				t.Fatal("changed session authority was accepted by socket")
			}
			if tc.logoutDenied {
				if err := f.s.Logout(f.ctx, changed, false); err == nil {
					t.Fatal("changed session authority was accepted by logout")
				}
			}
		})
	}

	if _, err := f.pool.Exec(f.ctx, `UPDATE identity_sessions SET reauthenticated_at=$2 WHERE session_id=$1`, principal.sessionID, f.s.now().Add(-6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.s.IssueLink(f.ctx, principal, validLink, "127.0.0.1"); err == nil {
		t.Fatal("stale session issued a wallet link challenge")
	}
	if err := f.s.CheckSocket(f.ctx, principal); err != nil {
		t.Fatalf("stale step-up did not preserve the live socket: %v", err)
	}
	if err := f.s.Logout(f.ctx, principal, false); err != nil {
		t.Fatalf("live session logout: %v", err)
	}
	if err := f.s.Logout(f.ctx, principal, false); err == nil {
		t.Fatal("revoked session logged out twice")
	}
}
