package account

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
)

func TestGatewayOwnerWriteCommitsSignedPreferenceAndRetainsResponse(t *testing.T) {
	s := v2OwnerDeletionFixture(t)
	ctx := context.Background()
	walletKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	wallet := strings.ToLower(crypto.PubkeyToAddress(walletKey.PublicKey).Hex())
	id, session := uuid.New(), uuid.New()
	if _, err := s.DB.Exec(ctx, `INSERT INTO identity_accounts VALUES($1,1,clock_timestamp())`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO identity_wallet_links VALUES($1,$2,125,$3,'private','verified',clock_timestamp())`, uuid.New(), id, wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO identity_sessions(session_id,account_id,wallet,auth_epoch,access_hash,csrf_hash,issued_at,access_expires_at,refresh_expires_at,reauthenticated_at,last_seen_at,device_label)
		VALUES($1,$2,$3,1,$4,$5,clock_timestamp(),clock_timestamp()+interval '1 hour',clock_timestamp()+interval '1 day',clock_timestamp(),clock_timestamp(),'gateway')`, session, id, wallet, []byte(uuid.NewString()), []byte(uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	resolver := sharedauth.PostgresIdentityResolver{Pool: s.DB}
	identity, err := resolver.Resolve(ctx, sharedauth.Principal{Wallet: wallet, SessionID: session.String()})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ensureIdentityAccount(ctx, resolver, sharedauth.Principal{Wallet: wallet, SessionID: session.String(), Roles: []string{"owner"}}); err != nil {
		t.Fatalf("provision owner: %v", err)
	}
	if _, err := s.DB.Exec(ctx, `SELECT actor,action,nonce,envelope_digest,body_digest,request_id,response,response_digest FROM operation_ledger LIMIT 0`); err != nil {
		t.Fatalf("operation ledger schema: %v", err)
	}
	if _, err := s.DB.Exec(ctx, `SELECT request_id,actor_pseudonym,action,result,policy_version,request_digest FROM security_audit_outbox LIMIT 0`); err != nil {
		t.Fatalf("operation audit schema: %v", err)
	}
	const token = "identity-service-token-for-account-gateway"
	var differentSession atomic.Bool
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v2/identity/request:authenticate" || r.Header.Get("Authorization") != "Bearer "+token {
			http.NotFound(w, r)
			return
		}
		resolved := identity
		if differentSession.Load() {
			resolved.SessionID = uuid.NewString()
		}
		if err := json.NewEncoder(w).Encode(resolved); err != nil {
			t.Error(err)
		}
	}))
	defer identityServer.Close()
	pki := testpki.New(t)
	serverLeaf := pki.Leaf("user", "user.test")
	clientLeaf := pki.Leaf("gateway", "gateway.test")
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	transport := sharedauth.AssertionTransport{AssertionHeader: "X-Test-Assertion", SignatureHeader: "X-Test-Signature"}
	issuer := sharedauth.ServiceAssertionIssuer{Issuer: "gateway", Audience: "user", Key: privateKey, Transport: transport}
	verifier := sharedauth.ServiceAssertionVerifier{Issuer: "gateway", Audience: "user", PeerCertSHA256: clientLeaf.SHA256, Key: publicKey, Transport: transport}
	policies, routes := sharedauth.AccountOwnerPoliciesV2()
	operations := sharedauth.OperationAuthorizer{
		Ledger:   sharedauth.Ledger{Pool: s.DB, Environment: "test", AuditKey: bytes.Repeat([]byte{9}, 32)},
		Verifier: v2OwnerEOAVerifier{}, Policies: policies, Routes: routes,
		Transport: sharedauth.OperationTransport{EnvelopeHeader: "X-Test-Operation", SignatureHeader: "X-Test-Operation-Signature"},
	}
	var operationCategory string
	operations.Observe = func(category string) { operationCategory = category }
	subjects := map[string]string{"preferences.updated": "users.preferences.updated", "profile.updated": "users.profile.updated"}
	publisher, err := delivery.NewStore(s.DB, "users", delivery.Policy{Subjects: map[string]bool{"users.preferences.updated": true, "users.profile.updated": true}, EventTypes: map[string]bool{"preferences.updated": true, "profile.updated": true}})
	if err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{ExtendedV3: true, IdentityAuthority: sharedauth.HTTPIdentityAuthority{Endpoint: identityServer.URL, ServiceToken: token, AllowPlainHTTP: true}, Delivery: publisher, Subjects: subjects}
	identityProbe := httptest.NewRequest(http.MethodPatch, "/v1/me/preferences", strings.NewReader(`{"expectedRevision":"0","locale":"fr"}`))
	identityProbe.Header.Set("Authorization", "Bearer browser-session")
	if authenticated, probeErr := runtime.IdentityAuthority.AuthenticateRequest(ctx, identityProbe); probeErr != nil || authenticated.AccountID != identity.AccountID {
		t.Fatalf("identity authority for owner write = %+v, %v", authenticated, probeErr)
	}
	server := httptest.NewUnstartedServer((HTTP{Store: s, Runtime: runtime}).GatewayHandler(verifier, resolver, operations))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{serverLeaf.Pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.Roots()}
	server.StartTLS()
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{clientLeaf.Pair}}}}
	defer client.CloseIdleConnections()
	probe := func(tb *testing.T, method, path, body string, roles []string, assertion bool) (int, []byte) {
		tb.Helper()
		r, err := http.NewRequestWithContext(ctx, method, server.URL+path, strings.NewReader(body))
		if err != nil {
			tb.Fatal(err)
		}
		if assertion {
			r.Header.Set("X-Request-ID", uuid.NewString())
			r.Header.Set("Authorization", "Bearer browser-session")
			if err := issuer.SignRequest(r, identity, roles, []byte(body)); err != nil {
				tb.Fatal(err)
			}
		}
		response, err := client.Do(r)
		if err != nil {
			tb.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			tb.Fatal(err)
		}
		return response.StatusCode, raw
	}
	for _, tc := range []struct {
		name, method, path, body string
		roles                    []string
		assertion                bool
		want                     int
	}{
		{"public profile", http.MethodGet, "/v1/profiles/" + wallet, "", nil, false, http.StatusOK},
		{"owner preference", http.MethodGet, "/v3/me/preferences", "", []string{"owner"}, true, http.StatusOK},
		{"non-owner preference", http.MethodGet, "/v3/me/preferences", "", []string{"reader"}, true, http.StatusForbidden},
		{"unknown owner action", http.MethodPost, "/v1/me/unknown", "", []string{"owner"}, true, http.StatusForbidden},
		{"oversize body", http.MethodPost, "/v1/me/unknown", strings.Repeat("x", 16385), nil, false, http.StatusUnprocessableEntity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, raw := probe(t, tc.method, tc.path, tc.body, tc.roles, tc.assertion)
			if status != tc.want {
				t.Fatalf("gateway route = %d %s, want %d", status, raw, tc.want)
			}
		})
	}
	differentSession.Store(true)
	if status, raw := probe(t, http.MethodGet, "/v3/me/preferences", "", []string{"owner"}, true); status != http.StatusUnauthorized {
		t.Fatalf("different identity session = %d %s", status, raw)
	}
	differentSession.Store(false)
	runtime.ExtendedV3 = false
	if status, raw := probe(t, http.MethodPatch, "/v3/me/preferences", `{"expectedRevision":"0"}`, []string{"owner"}, true); status != http.StatusForbidden {
		t.Fatalf("disabled extended write = %d %s", status, raw)
	}
	runtime.ExtendedV3 = true
	authority := runtime.IdentityAuthority
	runtime.IdentityAuthority = nil
	if status, raw := probe(t, http.MethodGet, "/v3/me/preferences", "", []string{"owner"}, true); status != http.StatusServiceUnavailable {
		t.Fatalf("unavailable identity authority = %d %s", status, raw)
	}
	runtime.IdentityAuthority = authority
	runtime.EndpointKey = bytes.Repeat([]byte{7}, 32)
	recoveryJob := uuid.New()
	receipt, err := (HTTP{Store: s, Runtime: runtime}).recoveryReceipt(WireAccountJob{JobID: recoveryJob.String(), AccountID: id.String()})
	if err != nil {
		t.Fatal(err)
	}
	recoveryIssuer := sharedauth.ServiceRequestIssuer{Issuer: "gateway", Audience: "user", Key: privateKey, Transport: transport}
	recoveryRead := func() (int, []byte) {
		t.Helper()
		r, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/v1/me/deletion-requests/"+recoveryJob.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("X-Request-ID", uuid.NewString())
		r.Header.Set("X-Recovery-Receipt", receipt)
		if err := recoveryIssuer.SignRequest(r, nil); err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, raw
	}
	if status, raw := recoveryRead(); status != http.StatusForbidden {
		t.Fatalf("unverified recovery read = %d %s", status, raw)
	}
	runtime.RecoveryGatewayVerifier = &sharedauth.ServiceRequestVerifier{Issuer: "gateway", Audience: "user", PeerCertSHA256: clientLeaf.SHA256, Key: publicKey, Transport: transport}
	if status, raw := recoveryRead(); status != http.StatusNotFound {
		t.Fatalf("unknown recovery job = %d %s", status, raw)
	}
	const path = "/v1/me/preferences"
	const body = `{"expectedRevision":"0","locale":"fr"}`
	digest, err := sharedauth.BodyDigest([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	envelope := sharedauth.OperationEnvelope{Version: 1, ChainID: "125", Environment: "test", Actor: wallet, Action: "preferences.update", Method: http.MethodPatch, Path: path, BodyHash: digest, Nonce: strings.Repeat("ab", 32), IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()}
	statement, err := envelope.Statement()
	if err != nil {
		t.Fatal(err)
	}
	signature, err := crypto.Sign(accounts.TextHash([]byte(statement)), walletKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := (v2OwnerEOAVerifier{}).Verify(ctx, wallet, statement, hexutil.Encode(signature)); err != nil {
		t.Fatalf("wallet operation proof: %v", err)
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	requestID := uuid.NewString()
	request := func(body, signature string) (int, []byte) {
		t.Helper()
		r, err := http.NewRequestWithContext(ctx, http.MethodPatch, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("X-Request-ID", requestID)
		r.Header.Set("Authorization", "Bearer browser-session")
		if err := issuer.SignRequest(r, identity, []string{"owner"}, []byte(body)); err != nil {
			t.Fatal(err)
		}
		r.Header.Set(operations.Transport.EnvelopeHeader, base64.RawURLEncoding.EncodeToString(encoded))
		r.Header.Set(operations.Transport.SignatureHeader, signature)
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, raw
	}
	status, raw := request(body, hexutil.Encode(signature))
	if status != http.StatusOK || !bytes.Contains(raw, []byte(`"locale":"fr"`)) {
		t.Fatalf("signed preference write = %d %s; authorization = %s", status, raw, operationCategory)
	}
	status, replay := request(body, hexutil.Encode(signature))
	if status != http.StatusOK || !bytes.Equal(replay, raw) {
		t.Fatalf("retained preference response = %d %s, first %s", status, replay, raw)
	}
	status, _ = request(`{"expectedRevision":"0","locale":"de"}`, hexutil.Encode(signature))
	if status != http.StatusForbidden {
		t.Fatalf("changed signed preference = %d", status)
	}
	var revision int64
	if err := s.DB.QueryRow(ctx, `SELECT revision FROM account_state WHERE account_id=$1`, id).Scan(&revision); err != nil || revision != 1 {
		t.Fatalf("preference revision = %d, %v", revision, err)
	}
	withKey := func(method, targetPath, action, body, key string) (int, []byte) {
		t.Helper()
		digest, err := sharedauth.BodyDigest([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		fresh := envelope
		fresh.Method = method
		fresh.Path = targetPath
		fresh.Action = action
		fresh.BodyHash = digest
		fresh.Nonce = strings.Repeat(strings.ReplaceAll(uuid.NewString(), "-", ""), 2)
		fresh.IssuedAt = time.Now().Unix()
		fresh.ExpiresAt = time.Now().Add(time.Minute).Unix()
		statement, err := fresh.Statement()
		if err != nil {
			t.Fatal(err)
		}
		proof, err := crypto.Sign(accounts.TextHash([]byte(statement)), walletKey)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(fresh)
		if err != nil {
			t.Fatal(err)
		}
		r, err := http.NewRequestWithContext(ctx, method, server.URL+targetPath, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("X-Request-ID", uuid.NewString())
		r.Header.Set("Authorization", "Bearer browser-session")
		r.Header.Set("Idempotency-Key", key)
		if err := issuer.SignRequest(r, identity, []string{"owner"}, []byte(body)); err != nil {
			t.Fatal(err)
		}
		r.Header.Set(operations.Transport.EnvelopeHeader, base64.RawURLEncoding.EncodeToString(encoded))
		r.Header.Set(operations.Transport.SignatureHeader, hexutil.Encode(proof))
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, raw
	}
	key := uuid.NewString()
	nextBody := `{"expectedRevision":"1","locale":"de"}`
	status, raw = withKey(http.MethodPatch, path, "preferences.update", nextBody, key)
	if status != http.StatusOK || !bytes.Contains(raw, []byte(`"locale":"de"`)) {
		t.Fatalf("idempotent preference write = %d %s", status, raw)
	}
	status, replay = withKey(http.MethodPatch, path, "preferences.update", nextBody, key)
	if status != http.StatusOK || !bytes.Equal(replay, raw) {
		t.Fatalf("idempotent preference replay = %d %s, first %s", status, replay, raw)
	}
	status, _ = withKey(http.MethodPatch, path, "preferences.update", `{"expectedRevision":"2","locale":"es"}`, key)
	if status != http.StatusConflict {
		t.Fatalf("changed idempotent preference = %d", status)
	}
	if err := s.DB.QueryRow(ctx, `SELECT revision FROM account_state WHERE account_id=$1`, id).Scan(&revision); err != nil || revision != 2 {
		t.Fatalf("idempotent preference revision = %d, %v", revision, err)
	}
	status, raw = withKey(http.MethodPatch, path, "preferences.update", `{"expectedRevision":"2","timezone":"Mars/Olympus"}`, "")
	if status != http.StatusUnprocessableEntity || !bytes.Contains(raw, []byte("timezone")) {
		t.Fatalf("invalid signed preference = %d %s", status, raw)
	}
	status, raw = withKey(http.MethodPatch, path, "preferences.update", `{"expectedRevision":"2","locale":"es"}`, "invalid")
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("invalid operation idempotency key = %d %s", status, raw)
	}
	profilePath := "/v3/profiles/" + wallet
	profileBody := `{"expectedRevision":"2","displayName":"Ada"}`
	status, raw = withKey(http.MethodPatch, profilePath, "profile.update", profileBody, "")
	if status != http.StatusOK || !bytes.Contains(raw, []byte(`"displayName":"Ada"`)) {
		t.Fatalf("signed extended profile = %d %s", status, raw)
	}
	runtime.ExtendedV3 = false
	status, _ = withKey(http.MethodPatch, profilePath, "profile.update", `{"expectedRevision":"3","displayName":"Other"}`, "")
	if status != http.StatusForbidden {
		t.Fatalf("disabled extended profile write = %d", status)
	}
}
