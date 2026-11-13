package proxy

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
)

func authorityWorkflowMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// The helper executables link the owning modules' actual internal handlers.
// Temporary module paths respect Go's internal-package boundary; no source in
// an owning module is patched or replaced for the proof.
func authorityWorkflowBuildAuthority(t *testing.T, owner string) string {
	t.Helper()
	root := proofRepositoryRoot(t)
	dir := t.TempDir()
	original := filepath.Join(root, "services/launchpad", owner)
	manifest, err := os.ReadFile(filepath.Join(original, "go.mod"))
	authorityWorkflowMust(t, err)
	module := "github.com/Sidiora-Technologies/KindleLaunch/media/" + owner
	manifest = bytes.Replace(manifest, []byte("module "+module+"\n"), []byte("module "+module+"/authorityfixture\n"), 1)
	relative := regexp.MustCompile(`=> (\.[^\s]+)`)
	manifest = relative.ReplaceAllFunc(manifest, func(match []byte) []byte {
		return []byte("=> " + filepath.Clean(filepath.Join(original, string(match[3:]))))
	})
	manifest = append(manifest, []byte("\nrequire "+module+" v0.0.0\nreplace "+module+" => "+original+"\n")...)
	authorityWorkflowMust(t, os.WriteFile(filepath.Join(dir, "go.mod"), manifest, 0600))
	sum, err := os.ReadFile(filepath.Join(original, "go.sum"))
	authorityWorkflowMust(t, err)
	authorityWorkflowMust(t, os.WriteFile(filepath.Join(dir, "go.sum"), sum, 0600))
	source, err := os.ReadFile(filepath.Join(root, "services/launchpad/gateway/internal/proxy/testdata/"+owner+"_authority.go"))
	authorityWorkflowMust(t, err)
	authorityWorkflowMust(t, os.WriteFile(filepath.Join(dir, "main.go"), source, 0600))
	binary := filepath.Join(dir, "authority")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-mod=mod", "-o", binary, ".")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build real %s authority: %v\n%s", owner, err, output)
	}
	return binary
}

func authorityWorkflowProcess(t *testing.T, cmd *exec.Cmd) func() {
	t.Helper()
	output := &lockedBuffer{}
	cmd.Stdout = output
	cmd.Stderr = output
	authorityWorkflowMust(t, cmd.Start())
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			_ = cmd.Process.Signal(os.Interrupt)
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				_ = cmd.Process.Kill()
				<-done
				t.Error("authority did not stop")
			}
		})
	}
	t.Cleanup(func() {
		stop()
		if t.Failed() {
			t.Logf("%s output: %s", filepath.Base(cmd.Path), output.String())
		}
	})
	return stop
}

func authorityWorkflowReady(t *testing.T, client *http.Client, endpoint string) []byte {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for {
		resp, err := client.Get(endpoint)
		if err == nil {
			raw, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr == nil && resp.StatusCode == 200 {
				return raw
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("real authority not ready at %s: %v", endpoint, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func authorityWorkflowIdentityConfig(e *identityE2E, issuer string) sharedauth.MTLSIdentityClientConfig {
	peer := e.peers[issuer]
	return sharedauth.MTLSIdentityClientConfig{Endpoint: e.identityURL, PeerCertSHA256: e.gatewayServer.SHA256, CAFile: e.pki.CAFile, ClientCertFile: peer.leaf.CertFile, ClientKeyFile: peer.leaf.KeyFile, Issuer: issuer, SigningKey: peer.key.StdBase64}
}

func authorityWorkflowPrivateClient(t *testing.T, e *identityE2E) *http.Client {
	t.Helper()
	transport := &http.Transport{TLSClientConfig: e.gatewayClientTLS(), Proxy: nil}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 3 * time.Second}
}

func authorityWorkflowDSN(t *testing.T, dsn, schema string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	authorityWorkflowMust(t, err)
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	return u.String()
}

func authorityWorkflowMinio(t *testing.T) storage.Config {
	t.Helper()
	port := internaltest.FreePorts(t, 1)[0]
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	key := strings.ReplaceAll(uuid.NewString(), "-", "")
	secret := strings.ReplaceAll(uuid.NewString(), "-", "")
	cmd := exec.Command("minio", "server", t.TempDir(), "--address", fmt.Sprintf("127.0.0.1:%d", port), "--console-address", "127.0.0.1:0")
	cmd.Env = append(os.Environ(), "MINIO_ROOT_USER="+key, "MINIO_ROOT_PASSWORD="+secret)
	authorityWorkflowProcess(t, cmd)
	authorityWorkflowReady(t, &http.Client{Timeout: time.Second}, endpoint+"/minio/health/ready")
	client := s3.New(s3.Options{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider(key, secret, ""), BaseEndpoint: aws.String(endpoint), UsePathStyle: true})
	_, err := client.CreateBucket(context.Background(), &s3.CreateBucketInput{Bucket: aws.String("repair48-exports")})
	authorityWorkflowMust(t, err)
	return storage.Config{Endpoint: endpoint, Region: "us-east-1", AccessKeyID: key, SecretAccessKey: secret, Bucket: "repair48-exports"}
}

type authorityWorkflowUser struct {
	e               *identityE2E
	edge            *httptest.Server
	restart         func()
	direct          *http.Client
	endpoint, audit string
}

func authorityWorkflowUserAuthority(t *testing.T, version string, objects storage.Config) authorityWorkflowUser {
	t.Helper()
	e := newIdentityE2E(t)
	ctx := context.Background()
	root := proofRepositoryRoot(t)
	migration, err := os.ReadFile(filepath.Join(root, "services/launchpad/user/migrations/00002_account_store.sql"))
	authorityWorkflowMust(t, err)
	up, _, found := strings.Cut(string(migration), "-- +goose Down")
	if !found {
		t.Fatal("account migration boundary absent")
	}
	_, err = e.identity.Pool.Exec(ctx, up)
	authorityWorkflowMust(t, err)
	_, err = e.identity.Pool.Exec(ctx, sharedauth.LedgerDDL)
	authorityWorkflowMust(t, err)
	deliveryDDL, err := delivery.SchemaDDL("gateway_identity")
	authorityWorkflowMust(t, err)
	_, err = e.identity.Pool.Exec(ctx, deliveryDDL)
	authorityWorkflowMust(t, err)
	// The identity authority owns these real tables. Writable SQL views expose
	// those exact rows to the existing users-owned deletion functions.
	_, err = e.identity.Pool.Exec(ctx, `CREATE SCHEMA users;
CREATE VIEW users.identity_accounts AS SELECT * FROM gateway_identity.identity_accounts;
CREATE VIEW users.identity_sessions AS SELECT * FROM gateway_identity.identity_sessions;
CREATE VIEW users.identity_reauth AS SELECT * FROM gateway_identity.identity_reauth;
CREATE VIEW users.identity_outbox AS SELECT * FROM gateway_identity.identity_outbox;`)
	authorityWorkflowMust(t, err)
	functions, err := os.ReadFile(filepath.Join(root, "migrations/330001.sql"))
	authorityWorkflowMust(t, err)
	_, ddl, found := strings.Cut(string(functions), "CREATE FUNCTION users.consume_account_deletion_reauth")
	if !found {
		t.Fatal("deletion authority functions absent")
	}
	_, err = e.identity.Pool.Exec(ctx, "CREATE FUNCTION users.consume_account_deletion_reauth"+ddl)
	authorityWorkflowMust(t, err)
	leaf := e.pki.Leaf("user-authority", "user.railway.internal")
	port := internaltest.FreePorts(t, 1)[0]
	endpoint := fmt.Sprintf("https://127.0.0.1:%d", port)
	audit := filepath.Join(t.TempDir(), "request-headers.jsonl")
	cfg := map[string]any{"DSN": authorityWorkflowDSN(t, e.identity.Pool.Config().ConnString(), "gateway_identity"), "RPC": e2eChain(t), "Listen": fmt.Sprintf("127.0.0.1:%d", port), "Cert": leaf.CertFile, "Key": leaf.KeyFile, "CA": e.pki.CAFile, "GatewayPin": e.gatewayClient.SHA256, "GatewayPublicKey": base64.StdEncoding.EncodeToString(e.gatewayKey.Private.Public().(ed25519.PublicKey)), "Identity": authorityWorkflowIdentityConfig(e, "social"), "Objects": objects, "Version": version, "HeaderAudit": audit}
	configFile := filepath.Join(t.TempDir(), "configuration.json")
	raw, err := json.Marshal(cfg)
	authorityWorkflowMust(t, err)
	authorityWorkflowMust(t, os.WriteFile(configFile, raw, 0600))
	binary := authorityWorkflowBuildAuthority(t, "user")
	client := authorityWorkflowPrivateClient(t, e)
	start := func() func() {
		stop := authorityWorkflowProcess(t, exec.Command(binary, configFile))
		authorityWorkflowReady(t, client, endpoint+"/health/ready")
		return stop
	}
	stop := start()
	proxy, err := NewConnected(ConnectedDeps{Name: "user", URL: endpoint, Routes: UserConnectedRoutes(), Identity: e.authorityForGateway, SigningKey: e.gatewayKey.Private, TLS: e.gatewayClientTLS()})
	authorityWorkflowMust(t, err)
	t.Cleanup(proxy.client.CloseIdleConnections)
	edge := httptest.NewServer(proxy)
	t.Cleanup(edge.Close)
	return authorityWorkflowUser{e: e, edge: edge, direct: client, endpoint: endpoint, audit: audit, restart: func() { stop(); stop = start() }}
}

func authorityWorkflowSigned(t *testing.T, session e2eSession, action, path, body string) map[string]string {
	t.Helper()
	digest, err := sharedauth.BodyDigest([]byte(body))
	authorityWorkflowMust(t, err)
	now := time.Now()
	nonce := strings.ReplaceAll(uuid.NewString(), "-", "") + strings.ReplaceAll(uuid.NewString(), "-", "")
	envelope := sharedauth.OperationEnvelope{Version: 1, ChainID: "125", Environment: "test", Actor: session.wallet, Action: action, Method: http.MethodPost, Path: path, BodyHash: digest, Nonce: nonce, IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()}
	statement, err := envelope.Statement()
	authorityWorkflowMust(t, err)
	signature, err := crypto.Sign(accounts.TextHash([]byte(statement)), session.key)
	authorityWorkflowMust(t, err)
	raw, err := json.Marshal(envelope)
	authorityWorkflowMust(t, err)
	return map[string]string{"Authorization": "Bearer " + session.token, "X-Kindle-Operation": base64.RawURLEncoding.EncodeToString(raw), "X-Kindle-Operation-Signature": "0x" + hex.EncodeToString(signature), "X-Kindle-Signature": "0x" + hex.EncodeToString(signature), "Idempotency-Key": uuid.NewString(), "Content-Type": "application/json"}
}

func TestExportCapability(t *testing.T) {
	fixture := authorityWorkflowUserAuthority(t, "v1", authorityWorkflowMinio(t))
	owner := fixture.e.signIn()
	other := fixture.e.signIn()
	for _, prefix := range []string{"", "Bearer "} {
		body := `{"format":"json","includeMedia":false}`
		resp, raw := e2eSend(t, nil, "POST", fixture.edge.URL+"/v1/me/exports", json.RawMessage(body), authorityWorkflowSigned(t, owner, "account.export", "/v1/me/exports", body))
		if resp.StatusCode != 201 {
			t.Fatalf("real export creation: %d %s", resp.StatusCode, raw)
		}
		var created struct {
			Data struct{ Job struct{ JobID string } }
		}
		authorityWorkflowMust(t, json.Unmarshal(raw, &created))
		if _, err := uuid.Parse(created.Data.Job.JobID); err != nil {
			t.Fatal("producer job identity absent")
		}
		statusPath := "/v1/me/exports/" + created.Data.Job.JobID
		var ready struct {
			Data struct {
				Job struct {
					State                 string
					DownloadAuthorization *string
					DownloadURL           *string
				}
			}
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			resp, raw = e2eSend(t, nil, "GET", fixture.edge.URL+statusPath, nil, map[string]string{"Authorization": "Bearer " + owner.token})
			if resp.StatusCode != 200 {
				t.Fatalf("real export status: %d %s", resp.StatusCode, raw)
			}
			authorityWorkflowMust(t, json.Unmarshal(raw, &ready))
			if ready.Data.Job.State == "completed" && ready.Data.Job.DownloadAuthorization != nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("real worker did not prepare export: %s", raw)
			}
			time.Sleep(30 * time.Millisecond)
		}
		capability := prefix + *ready.Data.Job.DownloadAuthorization
		if !validExportAuthorization(capability) || ready.Data.Job.DownloadURL == nil {
			t.Fatal("real producer capability invalid")
		}
		resp, raw = e2eSend(t, nil, "GET", fixture.edge.URL+"/v1/me/preferences", nil, map[string]string{"Authorization": "Bearer " + owner.token, "X-Export-Authorization": capability})
		if resp.StatusCode != 200 {
			t.Fatalf("ordinary owner read: %d %s", resp.StatusCode, raw)
		}
		path := "/v2/me/exports/" + created.Data.Job.JobID + "/download"
		for _, values := range [][]string{{capability, capability}, {capability + ", " + capability}, {strings.Repeat("a", 65)}, {"Bearer " + strings.Repeat("z", 64)}, {""}} {
			req, err := http.NewRequest("GET", fixture.edge.URL+path, nil)
			authorityWorkflowMust(t, err)
			req.Header.Set("Authorization", "Bearer "+owner.token)
			req.Header["X-Export-Authorization"] = values
			res, err := http.DefaultClient.Do(req)
			authorityWorkflowMust(t, err)
			_ = res.Body.Close()
			if res.StatusCode != 400 {
				t.Fatalf("ambiguous/malformed capability accepted: %d", res.StatusCode)
			}
		}
		resp, raw = e2eSend(t, nil, "GET", fixture.edge.URL+path, nil, map[string]string{"Authorization": "Bearer " + other.token, "X-Export-Authorization": capability})
		if resp.StatusCode != 403 {
			t.Fatalf("other owner redeemed capability: %d %s", resp.StatusCode, raw)
		}
		headers := map[string]string{"Authorization": "Bearer " + owner.token, "X-Export-Authorization": capability}
		if prefix == "Bearer " {
			delete(headers, "Authorization")
			headers["Cookie"] = "__Host-kindle_access=" + owner.cookie
		}
		resp, raw = e2eSend(t, nil, "GET", fixture.edge.URL+path, nil, headers)
		if resp.StatusCode != 200 || !bytes.Contains(raw, []byte(owner.wallet)) || bytes.Contains(raw, []byte(other.wallet)) || resp.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("real owner download: %d %s", resp.StatusCode, raw)
		}
		observations, err := os.ReadFile(fixture.audit)
		authorityWorkflowMust(t, err)
		var last map[string]string
		for _, line := range bytes.Split(bytes.TrimSpace(observations), []byte("\n")) {
			var observed map[string]string
			authorityWorkflowMust(t, json.Unmarshal(line, &observed))
			if observed["path"] == "/v1/me/preferences" && observed["export"] != "" {
				t.Fatal("export capability escaped download route")
			}
			last = observed
		}
		if last["path"] != path || last["export"] != capability || last["authorization"] != headers["Authorization"] || last["cookie"] != headers["Cookie"] {
			t.Fatal("gateway changed an owner download credential")
		}
		resp, raw = e2eSend(t, nil, "GET", fixture.edge.URL+path, nil, map[string]string{"Authorization": "Bearer " + owner.token, "X-Export-Authorization": capability})
		if resp.StatusCode != 403 {
			t.Fatalf("capability replay accepted: %d %s", resp.StatusCode, raw)
		}
	}
}

func authorityWorkflowReauth(t *testing.T, e *identityE2E, owner e2eSession) string {
	t.Helper()
	ctx := context.Background()
	request := httptest.NewRequest("POST", "https://kindle.test/v2/auth/reauth/challenges", nil)
	request.Header.Set("Authorization", "Bearer "+owner.token)
	principal, err := e.identity.Identity.AuthenticateRequest(ctx, request)
	authorityWorkflowMust(t, err)
	challenge, err := e.identity.Identity.IssueActionStepUp(ctx, principal, "127.0.0.1", "account.delete")
	authorityWorkflowMust(t, err)
	if challenge.ChallengeID == "" || challenge.Message == "" {
		t.Fatal("real deletion reauthentication challenge absent")
	}
	sig, err := crypto.Sign(accounts.TextHash([]byte(challenge.Message)), owner.key)
	authorityWorkflowMust(t, err)
	sig[64] += 27
	walletProof := auth.WalletProof{Kind: "eoa_eip191", Wallet: owner.wallet, Signature: "0x" + hex.EncodeToString(sig)}
	proof, err := e.identity.Identity.VerifyActionStepUp(ctx, principal, challenge.ChallengeID, walletProof, "account.delete")
	authorityWorkflowMust(t, err)
	if len(proof) != 64 {
		t.Fatal("real reauthentication proof absent")
	}
	if _, err := e.identity.Identity.VerifyActionStepUp(ctx, principal, challenge.ChallengeID, walletProof, "account.delete"); err == nil {
		t.Fatal("real reauthentication challenge replay accepted")
	}
	return proof
}

func authorityWorkflowDeletion(t *testing.T, version string) {
	t.Helper()
	fixture := authorityWorkflowUserAuthority(t, version, storage.Config{})
	owner := fixture.e.signIn()
	req := httptest.NewRequest("GET", "https://kindle.test/v2/me/preferences", nil)
	req.Header.Set("Authorization", "Bearer "+owner.token)
	principal, err := fixture.e.authorityForGateway.AuthenticateRequest(context.Background(), req)
	authorityWorkflowMust(t, err)
	path := "/" + version + "/me/deletion-requests"
	body := fmt.Sprintf(`{"confirmAccountId":%q,"expectedRevision":"0","reauthProof":%q}`, principal.AccountID, authorityWorkflowReauth(t, fixture.e, owner))
	resp, raw := e2eSend(t, nil, "POST", fixture.edge.URL+path, json.RawMessage(body), authorityWorkflowSigned(t, owner, "account.delete", path, body))
	if resp.StatusCode != 202 {
		t.Fatalf("real accepted deletion: %d %s", resp.StatusCode, raw)
	}
	var accepted struct {
		Data struct {
			Job struct{ JobID, AccountID string }
		}
	}
	authorityWorkflowMust(t, json.Unmarshal(raw, &accepted))
	receipt := resp.Header.Get("X-Recovery-Receipt")
	if !validRecoveryHeader(resp.Header, accepted.Data.Job.JobID) || recoveryAccount(receipt) != principal.AccountID {
		t.Fatal("authority-issued recovery receipt lost or changed")
	}
	for _, values := range [][]string{{receipt, receipt}, {""}, {receipt + "x"}, {receipt + ", " + receipt}, {strings.Repeat("a", 4097)}} {
		header := resp.Header.Clone()
		header["X-Recovery-Receipt"] = values
		if validRecoveryHeader(header, accepted.Data.Job.JobID) {
			t.Fatal("malformed or ambiguous recovery receipt admitted")
		}
	}
	if version == "v2" {
		_, err = v2.DecodeAccountDeleteResponse(raw)
		authorityWorkflowMust(t, err)
	}
	resp, raw = e2eSend(t, nil, "GET", fixture.edge.URL+"/v2/me/preferences", nil, map[string]string{"Authorization": "Bearer " + owner.token})
	if resp.StatusCode != 401 {
		t.Fatalf("deletion did not revoke session: %d %s", resp.StatusCode, raw)
	}
	fixture.restart()
	for _, statusVersion := range []string{"v1", "v2"} {
		statusPath := "/" + statusVersion + "/me/deletion-requests/" + accepted.Data.Job.JobID
		resp, raw = e2eSend(t, nil, "GET", fixture.edge.URL+statusPath, nil, map[string]string{"X-Recovery-Receipt": receipt})
		if resp.StatusCode != 200 || !bytes.Contains(raw, []byte(accepted.Data.Job.JobID)) || !bytes.Contains(raw, []byte(principal.AccountID)) {
			t.Fatalf("receipt-only recovery after restart: %d %s", resp.StatusCode, raw)
		}
		if statusVersion == "v2" {
			_, err = v2.DecodeAccountDeleteStatusResponse(raw)
			authorityWorkflowMust(t, err)
		}
		// Even a revoked normal session must not prevent receipt recovery.
		resp, raw = e2eSend(t, nil, "GET", fixture.edge.URL+statusPath, nil, map[string]string{"X-Recovery-Receipt": receipt, "Authorization": "Bearer " + owner.token})
		if resp.StatusCode != 200 {
			t.Fatalf("revoked session blocked receipt: %d %s", resp.StatusCode, raw)
		}
		resp, raw = e2eSend(t, nil, "GET", fixture.edge.URL+"/"+statusVersion+"/me/deletion-requests/"+uuid.NewString(), nil, map[string]string{"X-Recovery-Receipt": receipt})
		if resp.StatusCode != 400 {
			t.Fatalf("receipt escaped job binding: %d %s", resp.StatusCode, raw)
		}
		decoded, err := base64.RawURLEncoding.DecodeString(receipt)
		authorityWorkflowMust(t, err)
		decoded[len(decoded)-1] ^= 1
		resp, raw = e2eSend(t, nil, "GET", fixture.edge.URL+statusPath, nil, map[string]string{"X-Recovery-Receipt": base64.RawURLEncoding.EncodeToString(decoded)})
		if resp.StatusCode != 403 {
			t.Fatalf("authority accepted tampered receipt: %d %s", resp.StatusCode, raw)
		}
		request, err := http.NewRequest("GET", fixture.edge.URL+statusPath, nil)
		authorityWorkflowMust(t, err)
		request.Header["X-Recovery-Receipt"] = []string{receipt, receipt}
		response, err := http.DefaultClient.Do(request)
		authorityWorkflowMust(t, err)
		_ = response.Body.Close()
		if response.StatusCode != 400 {
			t.Fatal("duplicate recovery receipts accepted")
		}
		// A receipt alone on the private socket is insufficient: the real user
		// authority still requires the gateway's signed service assertion.
		response, raw = e2eSend(t, fixture.direct, "GET", fixture.endpoint+statusPath, nil, map[string]string{"X-Recovery-Receipt": receipt})
		if response.StatusCode != 403 {
			t.Fatalf("unsigned private recovery admitted: %d %s", response.StatusCode, raw)
		}
	}
}

func TestDeletionReceipt(t *testing.T) { authorityWorkflowDeletion(t, "v1") }
func TestV2Recovery(t *testing.T)      { authorityWorkflowDeletion(t, "v2") }

func authorityWorkflowVault(t *testing.T) (string, string, string) {
	t.Helper()
	port := internaltest.FreePorts(t, 1)[0]
	dir := t.TempDir()
	token := strings.ReplaceAll(uuid.NewString(), "-", "")
	endpoint := fmt.Sprintf("https://127.0.0.1:%d", port)
	cmd := exec.Command("vault", "server", "-dev", "-dev-tls", "-dev-tls-cert-dir="+dir, "-dev-root-token-id="+token, fmt.Sprintf("-dev-listen-address=127.0.0.1:%d", port))
	cmd.Env = append(os.Environ(), "VAULT_LOG_LEVEL=error")
	authorityWorkflowProcess(t, cmd)
	caFile := filepath.Join(dir, "vault-ca.pem")
	deadline := time.Now().Add(20 * time.Second)
	var ca []byte
	for {
		var err error
		ca, err = os.ReadFile(caFile)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Vault dev CA absent")
		}
		time.Sleep(50 * time.Millisecond)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		t.Fatal("invalid real Vault CA")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots}}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: time.Second}
	authorityWorkflowReady(t, client, endpoint+"/v1/sys/health")
	for _, setup := range []struct{ path, body string }{{"/v1/sys/mounts/transit", `{"type":"transit"}`}, {"/v1/transit/keys/repair48", `{"type":"aes256-gcm96"}`}} {
		req, err := http.NewRequest("POST", endpoint+setup.path, strings.NewReader(setup.body))
		authorityWorkflowMust(t, err)
		req.Header.Set("X-Vault-Token", token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		authorityWorkflowMust(t, err)
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != 204 && resp.StatusCode != 200 {
			t.Fatalf("real Vault transit setup: %d %s", resp.StatusCode, raw)
		}
	}
	return endpoint, caFile, token
}

func TestIngestResponse(t *testing.T) {
	e := newIdentityE2E(t)
	owner := e.signIn()
	other := e.signIn()
	vaultURL, vaultCA, vaultToken := authorityWorkflowVault(t)
	ctx := context.Background()
	db, err := pgx.Connect(ctx, e.database.AdminDSN)
	authorityWorkflowMust(t, err)
	defer db.Close(ctx)
	poolAddress := "0x" + strings.Repeat("a9", 20)
	_, err = db.Exec(ctx, `INSERT INTO indexer.pools VALUES($1,$2,$3,$4,$5,1,$6,100,$7)`, poolAddress, "0x"+strings.Repeat("70", 20), owner.wallet, "0x"+strings.Repeat("0b", 20), "0x"+strings.Repeat("ab", 32), time.Now().Unix(), "0x"+strings.Repeat("cd", 32))
	authorityWorkflowMust(t, err)
	_, err = db.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) SELECT token_address,pool_address,creator,$1,optical,nft_id,created_block,$2,tx_hash,0,0,created_at,'{}','{}' FROM indexer.pools WHERE pool_address=$3`, "0x"+strings.Repeat("f0", 20), "0x"+strings.Repeat("de", 32), poolAddress)
	authorityWorkflowMust(t, err)
	leaf := e.pki.Leaf("livestream-authority", "livestream.railway.internal")
	port := internaltest.FreePorts(t, 1)[0]
	endpoint := fmt.Sprintf("https://127.0.0.1:%d", port)
	streamKey := strings.ReplaceAll(uuid.NewString(), "-", "")
	cfg := map[string]any{"DSN": authorityWorkflowDSN(t, e.database.AdminDSN, "livestream"), "RPC": e2eChain(t), "Listen": fmt.Sprintf("127.0.0.1:%d", port), "Cert": leaf.CertFile, "Key": leaf.KeyFile, "CA": e.pki.CAFile, "GatewayPin": e.gatewayClient.SHA256, "GatewayPublicKey": base64.StdEncoding.EncodeToString(e.gatewayKey.Private.Public().(ed25519.PublicKey)), "Identity": authorityWorkflowIdentityConfig(e, "livestream"), "Wallet": owner.wallet, "Pool": poolAddress, "VaultURL": vaultURL, "VaultCA": vaultCA, "VaultToken": vaultToken, "StreamKey": streamKey}
	file := filepath.Join(t.TempDir(), "configuration.json")
	raw, err := json.Marshal(cfg)
	authorityWorkflowMust(t, err)
	authorityWorkflowMust(t, os.WriteFile(file, raw, 0600))
	binary := authorityWorkflowBuildAuthority(t, "livestream")
	authorityWorkflowProcess(t, exec.Command(binary, file))
	ready := authorityWorkflowReady(t, authorityWorkflowPrivateClient(t, e), endpoint+"/health/ready")
	var operation struct{ StreamID string }
	authorityWorkflowMust(t, json.Unmarshal(ready, &operation))
	proxy, err := NewConnected(ConnectedDeps{Name: "livestream", URL: endpoint, Routes: LivestreamConnectedRoutes(), Identity: e.authorityForGateway, SigningKey: e.gatewayKey.Private, TLS: e.gatewayClientTLS()})
	authorityWorkflowMust(t, err)
	t.Cleanup(proxy.client.CloseIdleConnections)
	edge := httptest.NewServer(proxy)
	t.Cleanup(edge.Close)
	path := "/v2/streams/" + operation.StreamID + "/ingest-key"
	for _, session := range []e2eSession{other, owner} {
		resp, body := e2eSend(t, nil, "POST", edge.URL+path, json.RawMessage(`{}`), authorityWorkflowSigned(t, session, "stream.credential", path, `{}`))
		if session.wallet == other.wallet {
			if resp.StatusCode != 403 || bytes.Contains(body, []byte(streamKey)) {
				t.Fatalf("non-owner credential disclosure: %d %s", resp.StatusCode, body)
			}
			continue
		}
		if resp.StatusCode != 200 || resp.Header.Get("Cache-Control") != "no-store" || !validIngestResponse(body, operation.StreamID, time.Now()) {
			t.Fatalf("real private ingest response: %d %s", resp.StatusCode, body)
		}
		var observed struct {
			StreamID  string
			IngestKey struct{ StreamKey, IngestEndpoint, VisibilityUntil string }
		}
		authorityWorkflowMust(t, json.Unmarshal(body, &observed))
		if observed.StreamID != operation.StreamID || observed.IngestKey.StreamKey != streamKey || observed.IngestKey.IngestEndpoint != "rtmps://ingest.example.test/live" {
			t.Fatal("real credential changed at gateway")
		}
		if !containsSecret(body) {
			t.Fatal("private producer response would pass the generic route secret filter")
		}
		// Mutate actual handler output to prove the exception stays a closed,
		// stream-bound response instead of exempting arbitrary secret material.
		for _, damaged := range [][]byte{
			bytes.Replace(body, []byte(streamKey), []byte(strings.Repeat("a", 129)), 1),
			bytes.Replace(body, []byte(operation.StreamID), []byte(uuid.NewString()), 1),
			bytes.Replace(body, []byte(`"streamKey":`), []byte(`"providerToken":"leak","streamKey":`), 1),
			bytes.Replace(body, []byte(`"streamKey":`), []byte(`"streamKey":"duplicate","streamKey":`), 1),
			bytes.Replace(body, []byte("rtmps://ingest.example.test/live"), []byte("https://user:password@ingest.example.test/live"), 1),
			bytes.Replace(body, []byte(observed.IngestKey.VisibilityUntil), []byte(time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)), 1),
		} {
			if validIngestResponse(damaged, operation.StreamID, time.Now()) {
				t.Fatal("invalid private producer response admitted")
			}
		}
		resp, raw = e2eSend(t, nil, "GET", edge.URL+"/v2/streams/"+operation.StreamID, nil, nil)
		if resp.StatusCode != 200 || bytes.Contains(raw, []byte(streamKey)) || containsSecret(raw) {
			t.Fatalf("public stream disclosed private credential: %d %s", resp.StatusCode, raw)
		}
	}
	resp, raw := e2eSend(t, nil, "POST", edge.URL+path, json.RawMessage(`{}`), nil)
	if resp.StatusCode != 401 || bytes.Contains(raw, []byte(streamKey)) {
		t.Fatalf("anonymous credential disclosure: %d %s", resp.StatusCode, raw)
	}
}

// The production platform runtime already uses the shared ticket authenticator.
// The connected runtime's separate method requires the social-owner wiring
// request in spec/layerx-merge/programme/records/services/launchpad/gateway/internal/proxy/repair_48.kvx; this test does not substitute an authenticator.
func TestTicketOnlySocket(t *testing.T) {
	e := newIdentityE2E(t)
	owner := e.signIn()
	e.prepareSocialAccount(owner.wallet)
	social := e.startSocial()
	proxy, err := NewConnectedWS(ConnectedWSDeps{URL: social.privateURL, Identity: e.authorityForGateway, SigningKey: e.gatewayKey.Private, TLS: e.gatewayClientTLS(), Origins: []string{e2eOrigin}})
	authorityWorkflowMust(t, err)
	edge := httptest.NewServer(proxy)
	t.Cleanup(edge.Close)
	topic := "private.chat." + owner.wallet
	issue := func() string {
		request := httptest.NewRequest("POST", "https://kindle.test/v2/stream/tickets", nil)
		request.Header.Set("Authorization", "Bearer "+owner.token)
		principal, err := e.identity.Identity.AuthenticateRequest(context.Background(), request)
		authorityWorkflowMust(t, err)
		topics := []string{topic}
		ticket, err := e.identity.Identity.IssueChatStreamTicket(context.Background(), principal, sharedauth.StreamTicketBinding{SchemaMajor: 2, Topics: topics, FilterDigest: sharedauth.StreamFilterDigest(topics), Origin: e2eOrigin, Endpoint: "/v2/stream"})
		authorityWorkflowMust(t, err)
		return ticket
	}
	dial := func(ticket, selectedTopic string) (*websocket.Conn, *http.Response, error) {
		dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second, Subprotocols: []string{"sidiora.v2", "ticket." + ticket}}
		return dialer.Dial("ws"+strings.TrimPrefix(edge.URL, "http")+"/v2/stream?topics="+url.QueryEscape(selectedTopic), http.Header{"Origin": []string{e2eOrigin}})
	}
	ticket := issue()
	socket, resp, err := dial(ticket, topic)
	authorityWorkflowMust(t, err)
	defer socket.Close()
	if resp.StatusCode != 101 || socket.Subprotocol() != "sidiora.v2" {
		t.Fatal("ticket-only production socket not negotiated")
	}
	for _, attempt := range []struct{ ticket, topic string }{{ticket, topic}, {issue(), "private.chat.0x" + strings.Repeat("11", 20)}} {
		connection, response, err := dial(attempt.ticket, attempt.topic)
		if connection != nil {
			_ = connection.Close()
		}
		if response != nil {
			_ = response.Body.Close()
		}
		if err == nil || response == nil || (response.StatusCode != 401 && response.StatusCode != 403) {
			t.Fatalf("ticket replay/binding mismatch admitted: %v %v", response, err)
		}
	}
	authorityWorkflowMust(t, socket.SetReadDeadline(time.Now().Add(7*time.Second)))
	authorityWorkflowMust(t, socket.WriteJSON(map[string]string{"type": "subscribe_dms"}))
	for {
		var message struct{ Type string }
		authorityWorkflowMust(t, socket.ReadJSON(&message))
		if message.Type == "subscribed_dms" {
			break
		}
		if message.Type != "session_accepted" {
			t.Fatalf("private subscription failed: %s", message.Type)
		}
	}
	// A real owner logout revokes the session behind the redeemed ticket.
	request := httptest.NewRequest("POST", "https://kindle.test/v2/auth/sessions:logout-all", nil)
	request.Header.Set("Authorization", "Bearer "+owner.token)
	principal, err := e.identity.Identity.AuthenticateRequest(context.Background(), request)
	authorityWorkflowMust(t, err)
	authorityWorkflowMust(t, e.identity.Identity.Logout(context.Background(), principal, true))
	if err := e.identity.Identity.CheckSocket(context.Background(), principal); err == nil {
		t.Fatal("real identity authority retained revoked session")
	}
	authorityWorkflowMust(t, socket.SetReadDeadline(time.Now().Add(7*time.Second)))
	// Exercise the runtime's ongoing validation on private operations while
	// its bounded session-success cache expires; the closure bound stays seven seconds.
	stopRequests := make(chan struct{})
	requestsDone := make(chan struct{})
	go func() {
		defer close(requestsDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopRequests:
				return
			case <-ticker.C:
				if socket.SetWriteDeadline(time.Now().Add(time.Second)) != nil || socket.WriteJSON(map[string]string{"type": "subscribe_dms"}) != nil {
					return
				}
			}
		}
	}()
	defer func() { close(stopRequests); <-requestsDone }()
	for {
		_, _, err = socket.ReadMessage()
		if err != nil {
			break
		}
	}
	if timeout, ok := err.(interface{ Timeout() bool }); ok && timeout.Timeout() {
		t.Fatal("ticket-only socket remained open after session revocation")
	}
}
