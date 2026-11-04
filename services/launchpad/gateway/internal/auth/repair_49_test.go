package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func repair49ChallengeRequest(t *testing.T, f *identityHTTPFixture, key string, body []byte, wantStatus int) []byte {
	t.Helper()
	req, err := http.NewRequestWithContext(f.ctx, http.MethodPost, f.server.URL+"/v2/auth/challenges", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "https://kindle.test")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	response, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != wantStatus {
		t.Fatalf("challenge status = %d, want %d: %s", response.StatusCode, wantStatus, raw)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("challenge response must not be cached")
	}
	return raw
}

func repair49DecodeChallenge(t *testing.T, raw []byte) Challenge {
	t.Helper()
	var envelope Envelope[struct {
		Challenge Challenge `json:"challenge"`
	}]
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if !validUUID(envelope.Data.Challenge.ChallengeID) {
		t.Fatal("gateway did not issue a challenge UUID")
	}
	return envelope.Data.Challenge
}

func TestRepair_49_ChallengeRetryAndVerification(t *testing.T) {
	f := ownerHTTPFixture(t)
	body, err := json.Marshal(ChallengeRequest{Wallet: f.wallet, Origin: "https://kindle.test"})
	if err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	first := repair49DecodeChallenge(t, repair49ChallengeRequest(t, f, key, body, http.StatusCreated))
	if first.Wallet != f.wallet || first.Purpose != "login" || first.ChainID != "125" || first.Domain != "kindle.test" || first.URI != "https://kindle.test/auth" || !strings.HasSuffix(first.Message, "\nRequest ID: "+first.ChallengeID) {
		t.Fatalf("challenge binding changed: %+v", first)
	}
	// More retries than the new-challenge wallet allowance must still replay
	// the one issued challenge without consuming additional issuance capacity.
	for i := 0; i < 12; i++ {
		retry := repair49DecodeChallenge(t, repair49ChallengeRequest(t, f, key, body, http.StatusCreated))
		if !reflect.DeepEqual(retry, first) {
			t.Fatalf("retry %d changed signing authority: first=%+v retry=%+v", i, first, retry)
		}
	}
	var count int
	var storedHash []byte
	if err = f.pool.QueryRow(f.ctx, `SELECT body_hash FROM identity_challenge_requests WHERE wallet=$1 AND request_id=$2`, f.wallet, key).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(body)
	if !bytes.Equal(storedHash, wantHash[:]) {
		t.Fatal("transport did not preserve the challenge attempt's exact body")
	}
	if err = f.pool.QueryRow(f.ctx, `SELECT count(*) FROM identity_challenges`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("retry issued %d challenges, want one: %v", count, err)
	}
	login := LoginRequest{ChallengeID: first.ChallengeID, Proof: proofIdentity(t, f.key, first), DeviceLabel: "Browser wallet"}
	status, raw, headers := f.request(t, http.MethodPost, "/v2/auth/sessions", "", "", login)
	if status != http.StatusOK {
		t.Fatalf("verify issued challenge: %d %s", status, raw)
	}
	var credentials Envelope[LoginData]
	if err = json.Unmarshal(raw, &credentials); err != nil {
		t.Fatal(err)
	}
	principal, err := f.s.AuthenticateJWT(f.ctx, f.s.tokens, credentials.Data.AccessToken)
	if err != nil || principal.Wallet() != f.wallet || principal.SessionID() != credentials.Data.Session.SessionID {
		t.Fatalf("verified challenge did not authenticate its session: %+v %v", principal, err)
	}
	var accessCookie *http.Cookie
	for _, cookie := range (&http.Response{Header: headers}).Cookies() {
		if cookie.Name == "__Host-kindle_access" {
			accessCookie = cookie
		}
	}
	if accessCookie == nil || accessCookie.Value != credentials.Data.AccessToken || !accessCookie.Secure || !accessCookie.HttpOnly || accessCookie.Path != "/" || accessCookie.Domain != "" || accessCookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("login did not deliver a secure host-only browser access cookie")
	}
	replayed := repair49DecodeChallenge(t, repair49ChallengeRequest(t, f, key, body, http.StatusCreated))
	if !reflect.DeepEqual(replayed, first) {
		t.Fatal("retry after verification issued another challenge")
	}
	status, raw, _ = f.request(t, http.MethodPost, "/v2/auth/sessions", "", "", login)
	if status != http.StatusUnauthorized {
		t.Fatalf("challenge retry bypassed single-use proof validation: %d %s", status, raw)
	}
	if err = f.pool.QueryRow(f.ctx, `SELECT count(*) FROM identity_sessions`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("challenge proof created %d sessions, want one: %v", count, err)
	}
	next := repair49DecodeChallenge(t, repair49ChallengeRequest(t, f, uuid.NewString(), body, http.StatusCreated))
	if next.ChallengeID == first.ChallengeID || next.Nonce == first.Nonce || next.Message == first.Message {
		t.Fatal("a new challenge attempt reused prior signing authority")
	}
}

func TestRepair_49_ChallengeRequiresUUID(t *testing.T) {
	f := ownerHTTPFixture(t)
	body, err := json.Marshal(ChallengeRequest{Wallet: f.wallet, Origin: "https://kindle.test"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, key string }{
		{"missing", ""},
		{"malformed", "browser-login-attempt"},
		{"nil", uuid.Nil.String()},
		{"noncanonical", strings.ReplaceAll(uuid.NewString(), "-", "")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := repair49ChallengeRequest(t, f, tc.key, body, http.StatusUnprocessableEntity)
			var failure PublicError
			if err := json.Unmarshal(raw, &failure); err != nil || failure.Code != "FIELD_INVALID" || failure.Retryable {
				t.Fatalf("invalid UUID error = %s: %v", raw, err)
			}
		})
	}
	for _, table := range []string{"identity_challenges", "identity_challenge_requests", "identity_limits", "identity_sessions"} {
		var count int
		if err = f.pool.QueryRow(f.ctx, `SELECT count(*) FROM `+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("invalid UUID mutated %s: count=%d err=%v", table, count, err)
		}
	}
}

func TestRepair_49_ChallengeRetryRequiresExactBody(t *testing.T) {
	f := ownerHTTPFixture(t)
	body, err := json.Marshal(ChallengeRequest{Wallet: f.wallet, Origin: "https://kindle.test"})
	if err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	first := repair49DecodeChallenge(t, repair49ChallengeRequest(t, f, key, body, http.StatusCreated))
	changed := append(append([]byte(nil), body...), '\n')
	raw := repair49ChallengeRequest(t, f, key, changed, http.StatusConflict)
	var failure PublicError
	if err = json.Unmarshal(raw, &failure); err != nil || failure.Code != "IDEMPOTENCY_CONFLICT" || failure.Retryable {
		t.Fatalf("changed body did not conflict: %s %v", raw, err)
	}
	retry := repair49DecodeChallenge(t, repair49ChallengeRequest(t, f, key, body, http.StatusCreated))
	if !reflect.DeepEqual(retry, first) {
		t.Fatal("conflicting request replaced the original challenge")
	}
	var challenges, requests int
	if err = f.pool.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM identity_challenges), (SELECT count(*) FROM identity_challenge_requests)`).Scan(&challenges, &requests); err != nil || challenges != 1 || requests != 1 {
		t.Fatalf("conflict changed durable attempt: challenges=%d requests=%d err=%v", challenges, requests, err)
	}
}
