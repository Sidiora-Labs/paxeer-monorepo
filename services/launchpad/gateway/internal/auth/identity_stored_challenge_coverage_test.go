package auth

import (
	"strings"
	"testing"
	"time"
)

func TestIdentityStoredChallengeBindingsRejectChangedAuthority(t *testing.T) {
	f := fixtureIdentity(t)
	for _, tc := range []struct {
		name, column, reason string
		value                any
	}{
		{"domain", "domain", "challenge_domain", "other.test"},
		{"uri", "uri", "challenge_uri", "https://other.test/auth"},
		{"wallet", "wallet", "challenge_wallet", "0x" + strings.Repeat("0", 40)},
		{"purpose", "purpose", "challenge_purpose", "link_wallet"},
		{"message digest", "message_hash", "challenge_digest", []byte("changed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			challenge := f.challenge(t, f.wallet)
			proof := proofIdentity(t, f.key, challenge)
			if _, err := f.pool.Exec(f.ctx, "UPDATE identity_challenges SET "+tc.column+"=$2 WHERE challenge_id=$1", challenge.ChallengeID, tc.value); err != nil {
				t.Fatal(err)
			}
			observed := ""
			f.s.observe = func(reason string) { observed = reason }
			_, err := f.s.Login(f.ctx, LoginRequest{challenge.ChallengeID, proof, "bound device"})
			requireCode(t, err, "AUTH_EXPIRED")
			if observed != tc.reason {
				t.Fatalf("challenge observation=%q, want %q", observed, tc.reason)
			}
			var consumed *time.Time
			if err := f.pool.QueryRow(f.ctx, "SELECT consumed_at FROM identity_challenges WHERE challenge_id=$1", challenge.ChallengeID).Scan(&consumed); err != nil || consumed != nil {
				t.Fatalf("changed challenge consumed: at=%v err=%v", consumed, err)
			}
		})
	}
	challenge := f.challenge(t, f.wallet)
	proof := proofIdentity(t, f.key, challenge)
	for _, tc := range []struct {
		name string
		edit func(*LoginRequest)
	}{
		{"invalid challenge identity", func(r *LoginRequest) { r.ChallengeID = "invalid" }},
		{"invalid proof wallet", func(r *LoginRequest) { r.Proof.Wallet = "invalid" }},
		{"invalid proof kind", func(r *LoginRequest) { r.Proof.Kind = "unknown" }},
		{"invalid proof bytes", func(r *LoginRequest) { r.Proof.Signature = "0x12" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := LoginRequest{challenge.ChallengeID, proof, "bound device"}
			tc.edit(&request)
			_, err := f.s.Login(f.ctx, request)
			requireCode(t, err, "AUTH_EXPIRED")
		})
	}
	if _, err := f.s.Login(f.ctx, LoginRequest{challenge.ChallengeID, proof, "bound device"}); err != nil {
		t.Fatalf("unchanged challenge: %v", err)
	}
}
