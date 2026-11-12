package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestHTTPAuthorizationCommitsAndReplaysOneOperation(t *testing.T) {
	pool, _ := phase2AuthDatabase(t)
	ctx := context.Background()
	endpoint := os.Getenv("L34_ANVIL_URL")
	if endpoint == "" {
		t.Fatal("owned chain provider required")
	}
	backend, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	committed, err := NewReportedHashBackend(backend.Client()).ReportedBlockByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO auth_committed_blocks VALUES(125,$1,$2,'healthy')`, committed.Header.Number.String(), committed.Hash.Hex()); err != nil {
		t.Fatal(err)
	}
	walletKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	wallet := strings.ToLower(crypto.PubkeyToAddress(walletKey.PublicKey).Hex())
	accountID, sessionID := uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO identity_accounts VALUES($1,0,clock_timestamp())`, accountID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO identity_wallet_links VALUES($1,$2,125,$3,'private','verified',clock_timestamp())`, uuid.NewString(), accountID, wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO identity_sessions VALUES($1,$2,$3,0,$4,$4,clock_timestamp(),clock_timestamp()+interval '1 hour',clock_timestamp()+interval '1 day',clock_timestamp(),clock_timestamp(),'auth-transaction',NULL)`, sessionID, accountID, wallet, []byte("session-secret-digest")); err != nil {
		t.Fatal(err)
	}
	public, signing, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(247), Subject: pkix.Name{CommonName: "auth-transaction"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, signing)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: signing}
	certHash := sha256.Sum256(der)
	assertions := ServiceAssertionVerifier{Transport: AssertionTransport{"X-Test-Assertion", "X-Test-Service-Signature"}, Issuer: "gateway", Audience: "account", PeerCertSHA256: hex.EncodeToString(certHash[:]), Key: public}
	policies, routes := AccountOwnerPoliciesV2()
	authorizer := OperationAuthorizer{Ledger: Ledger{Pool: pool, Environment: "test", AuditKey: bytes.Repeat([]byte("a"), 32)}, Verifier: CommittedProofVerifier{Backend: NewReportedHashBackend(backend.Client()), Blocks: PostgresCommittedBlockProvider{Pool: pool}}, Policies: policies, Routes: routes, Transport: OperationTransport{"X-Test-Operation", "X-Test-Wallet-Signature"}}
	owner := func(_ context.Context, p IdentityPrincipal, _ *http.Request) (string, error) { return p.Wallet, nil }
	mutation := func(ctx context.Context, tx pgx.Tx, body []byte) ([]byte, error) {
		digest, err := BodyDigest(body)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO auth_owned_effects VALUES($1,$2)`, uuid.NewString(), digest); err != nil {
			return nil, err
		}
		return []byte(`{"updated":true}`), nil
	}
	denied := errors.New("owner lookup unavailable")
	makeHandler := func(resolve OperationOwnerResolver, mutate Mutation) http.Handler {
		return authorizer.Handler("profile.update", assertions, PostgresIdentityResolver{Pool: pool}, resolve, mutate)
	}
	handlers := map[string]http.Handler{
		"default":       makeHandler(owner, mutation),
		"configuration": makeHandler(nil, mutation),
		"owner":         makeHandler(func(context.Context, IdentityPrincipal, *http.Request) (string, error) { return "", denied }, mutation),
		"scope": makeHandler(func(context.Context, IdentityPrincipal, *http.Request) (string, error) {
			return "0x" + strings.Repeat("f", 40), nil
		}, mutation),
		"mutation": makeHandler(owner, func(context.Context, pgx.Tx, []byte) ([]byte, error) { return nil, ErrScopeDenied }),
		"response": makeHandler(owner, func(context.Context, pgx.Tx, []byte) ([]byte, error) { return []byte("invalid"), nil }),
		"dependency": makeHandler(owner, func(context.Context, pgx.Tx, []byte) ([]byte, error) {
			return nil, &VerificationError{Category: valueUpstream, Retryable: true}
		}),
		"session epoch": makeHandler(func(ctx context.Context, p IdentityPrincipal, _ *http.Request) (string, error) {
			_, err := pool.Exec(ctx, "UPDATE identity_accounts SET auth_epoch=auth_epoch+1 WHERE account_id=$1", p.AccountID)
			return p.Wallet, err
		}, mutation),
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		selected := handlers[r.Header.Get("X-Test-Case")]
		if selected == nil {
			selected = handlers["default"]
		}
		selected.ServeHTTP(w, r)
	})
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert})
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{cert}}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	body := []byte(`{"name":"owner"}`)
	bodyHash, err := BodyDigest(body)
	if err != nil {
		t.Fatal(err)
	}
	path := "/v1/profiles/" + wallet
	issued := time.Now().Unix()
	envelope := OperationEnvelope{Version: 1, ChainID: "125", Environment: "test", Actor: wallet, Action: "profile.update", Method: http.MethodPatch, Path: path, BodyHash: bodyHash, Nonce: strings.Repeat("e", 64), IssuedAt: issued, ExpiresAt: issued + 300}
	statement, err := envelope.Statement()
	if err != nil {
		t.Fatal(err)
	}
	signature, err := crypto.Sign(accounts.TextHash([]byte(statement)), walletKey)
	if err != nil {
		t.Fatal(err)
	}
	signature[64] += 27
	rawEnvelope, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	requestID := uuid.NewString()
	caseName := "default"
	var requestEdit func(*http.Request)
	var assertionEdit func(*ServiceAssertion)
	send := func(operation string) (int, []byte) {
		t.Helper()
		request, err := http.NewRequest(http.MethodPatch, "https://"+listener.Addr().String()+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("X-Request-ID", requestID)
		request.Header.Set("X-Test-Operation", operation)
		request.Header.Set("X-Test-Wallet-Signature", hexutil.Encode(signature))
		request.Header.Set("X-Test-Case", caseName)
		if requestEdit != nil {
			requestEdit(request)
		}
		issuedAt := time.Now().Unix()
		assertion := ServiceAssertion{Version: 1, Issuer: "gateway", Audience: "account", Principal: Principal{Wallet: wallet, SessionID: sessionID, Roles: []string{"owner"}, TraceID: requestID}, RequestDigest: RequestDigest(request.Method, request.URL.RequestURI(), body), IssuedAt: issuedAt, ExpiresAt: issuedAt + 30}
		if assertionEdit != nil {
			assertionEdit(&assertion)
		}
		raw, err := json.Marshal(assertion)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("X-Test-Assertion", base64.RawURLEncoding.EncodeToString(raw))
		request.Header.Set("X-Test-Service-Signature", base64.RawURLEncoding.EncodeToString(ed25519.Sign(signing, raw)))
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		wire, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, wire
	}
	operation := base64.RawURLEncoding.EncodeToString(rawEnvelope)
	for attempt := 0; attempt < 2; attempt++ {
		status, response := send(operation)
		if status != http.StatusOK || !bytes.Contains(response, []byte(`"updated":true`)) {
			t.Fatalf("operation %d = %d %s", attempt, status, response)
		}
	}
	var effects, ledger int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM auth_owned_effects),(SELECT count(*) FROM operation_ledger WHERE actor=$1 AND action='profile.update')`, wallet).Scan(&effects, &ledger); err != nil || effects != 1 || ledger != 1 {
		t.Fatalf("committed operation = %d effects, %d ledger rows, %v", effects, ledger, err)
	}
	status, response := send("malformed-envelope")
	if status < 400 || !bytes.Contains(response, []byte(`"code"`)) {
		t.Fatalf("malformed operation = %d %s", status, response)
	}
	for _, tc := range []struct {
		name   string
		status int
	}{
		{"configuration", 503}, {"owner", 401}, {"scope", 403}, {"mutation", 403}, {"response", 401}, {"dependency", 503},
		{"body digest", 403}, {"action", 403}, {"signature", 403}, {"query", 403}, {"method", 403}, {"expired envelope", 403}, {"body limit", 403}, {"session", 403}, {"assertion signature", 401}, {"assertion roles", 403}, {"session epoch", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caseName = tc.name
			requestEdit = nil
			assertionEdit = nil
			requestID = uuid.NewString()
			candidate := envelope
			candidate.Nonce = authBoundaryDigest(tc.name)
			switch tc.name {
			case "body digest":
				candidate.BodyHash = strings.Repeat("0", 64)
			case "action":
				candidate.Action = "wallet.unlink"
			case "expired envelope":
				candidate.IssuedAt = time.Now().Add(-10 * time.Minute).Unix()
				candidate.ExpiresAt = candidate.IssuedAt + 300
			case "signature":
				requestEdit = func(r *http.Request) { r.Header.Del("X-Test-Wallet-Signature") }
			case "query":
				requestEdit = func(r *http.Request) { r.URL.RawQuery = "page=1" }
			case "method":
				requestEdit = func(r *http.Request) { r.Method = http.MethodPost }
			case "body limit":
				requestEdit = func(r *http.Request) {
					r.Body = io.NopCloser(strings.NewReader(strings.Repeat("x", (1<<20)+1)))
					r.ContentLength = (1 << 20) + 1
				}
			case "session":
				assertionEdit = func(a *ServiceAssertion) { a.Principal.SessionID = uuid.NewString() }
			case "assertion signature":
				requestEdit = func(r *http.Request) { r.Header.Add("X-Request-ID", uuid.NewString()) }
			case "assertion roles":
				assertionEdit = func(a *ServiceAssertion) { a.Principal.Roles = nil }
			}
			message, err := candidate.Statement()

			if err != nil {
				t.Fatal(err)
			}
			signature, err = crypto.Sign(accounts.TextHash([]byte(message)), walletKey)
			if err != nil {
				t.Fatal(err)
			}
			signature[64] += 27
			raw, err := json.Marshal(candidate)
			if err != nil {
				t.Fatal(err)
			}
			status, response := send(base64.RawURLEncoding.EncodeToString(raw))
			if status != tc.status || !bytes.Contains(response, []byte(`"code"`)) {
				t.Fatalf("denied request: %d %s", status, response)
			}
			var responseBody struct {
				RequestID string `json:"requestId"`
			}
			if json.Unmarshal(response, &responseBody) != nil || !validIdentityID(responseBody.RequestID) {
				t.Fatalf("public denial identity: %s", response)
			}
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM auth_owned_effects").Scan(&effects); err != nil || effects != 1 {
				t.Fatalf("denied request changed effects: %d %v", effects, err)
			}
		})
	}

}

func authBoundaryDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
