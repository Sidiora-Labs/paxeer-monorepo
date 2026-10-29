package account

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

func v2OwnerFixture(t *testing.T) Store {
	t.Helper()
	s := fixture(t)
	db := stdlib.OpenDB(*s.DB.Config().ConnConfig)
	defer db.Close()
	versions, err := database.NewStore(database.DialectPostgres, "v2_owner_identity_version")
	must(t, err)
	provider, err := goose.NewProvider("", db,
		os.DirFS("../../../gateway/internal/devidentity/migrations"),
		goose.WithStore(versions))
	must(t, err)
	_, err = provider.Up(context.Background())
	must(t, err)
	return s
}

func requireV2OwnerResponse(t *testing.T, response *httptest.ResponseRecorder, status int, schema string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status %d, want %d: %s", response.Code, status, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("owner response headers: %v", response.Header())
	}
	must(t, v2.ValidateJSON(schema, response.Body.Bytes()))
}

func TestPostgresHTTPV2OwnerPreferencesAndDevices(t *testing.T) {
	s := v2OwnerFixture(t)
	p := owner(t, s, 94234)
	other := owner(t, s, 94235)
	handler := (HTTP{Store: s}).Handler()
	read := httpRequest(handler, p, http.MethodGet, "/v2/me/preferences", "")
	requireV2OwnerResponse(t, read, http.StatusOK, "PreferencesReadResponse")
	var initial v2.PreferencesReadResponse
	must(t, json.Unmarshal(read.Body.Bytes(), &initial))
	if initial.Data.Preferences.Revision != "0" || initial.Data.Preferences.Locale != "en" || initial.Data.Preferences.NotificationCategories != (v2.NotificationCategories{}) {
		t.Fatalf("default preferences: %+v", initial.Data.Preferences)
	}
	patch := `{"expectedRevision":"0","locale":"fr-CA","timezone":"America/Toronto","channels":{"email":false,"inapp":true,"push":true},"quietHours":{"timezone":"America/Toronto","start":"21:30","end":"06:45"}}`
	updated := httpRequest(handler, p, http.MethodPatch, "/v2/me/preferences", patch)
	requireV2OwnerResponse(t, updated, http.StatusOK, "PreferencesUpdateResponse")
	var preferences v2.PreferencesUpdateResponse
	must(t, json.Unmarshal(updated.Body.Bytes(), &preferences))
	if preferences.Data.Preferences.Revision != "1" || preferences.Data.Preferences.Locale != "fr-CA" || preferences.Data.Preferences.Timezone != "America/Toronto" || len(preferences.Data.Preferences.Channels) != 2 || preferences.Data.Preferences.QuietHours.Start != "21:30" {
		t.Fatalf("patched preferences: %+v", preferences.Data.Preferences)
	}
	read = httpRequest(handler, p, http.MethodGet, "/v2/me/preferences", "")
	requireV2OwnerResponse(t, read, http.StatusOK, "PreferencesReadResponse")
	var persisted v2.PreferencesReadResponse
	must(t, json.Unmarshal(read.Body.Bytes(), &persisted))
	if !reflect.DeepEqual(persisted.Data.Preferences, preferences.Data.Preferences) {
		t.Fatalf("preferences did not persist: %+v != %+v", persisted.Data.Preferences, preferences.Data.Preferences)
	}
	for _, test := range []struct {
		principal    Principal
		method, body string
		status       int
	}{
		{Principal{}, http.MethodGet, "", http.StatusUnauthorized},
		{Principal{}, http.MethodPatch, patch, http.StatusUnauthorized},
		{p, http.MethodPatch, patch, http.StatusPreconditionFailed},
		{p, http.MethodPatch, `{"expectedRevision":"1","unknown":true}`, http.StatusUnprocessableEntity},
	} {
		requireV2OwnerResponse(t, httpRequest(handler, test.principal, test.method, "/v2/me/preferences", test.body), test.status, "Error")
	}
	untouched := httpRequest(handler, other, http.MethodGet, "/v2/me/preferences", "")
	requireV2OwnerResponse(t, untouched, http.StatusOK, "PreferencesReadResponse")
	var separate v2.PreferencesReadResponse
	must(t, json.Unmarshal(untouched.Body.Bytes(), &separate))
	if separate.Data.Preferences.Revision != "0" || separate.Data.Preferences.Locale != "en" {
		t.Fatal("another owner's preferences changed", separate.Data.Preferences)
	}

	ctx := context.Background()
	session, device := uuid.New(), uuid.New()
	wallet := fmt.Sprintf("0x%040x", 94234)
	seedConnectedIdentity(t, s, p, session, wallet)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	must(t, err)
	token := "v2-owner-device-provider-token"
	proof := make([]byte, 107)
	proof[0] = 2
	copy(proof[1:17], p.AccountID[:])
	copy(proof[17:33], session[:])
	copy(proof[33:49], device[:])
	digest := sha256.Sum256([]byte(token))
	copy(proof[49:81], digest[:])
	_, err = rand.Read(proof[81:97])
	must(t, err)
	binary.BigEndian.PutUint64(proof[97:105], uint64(s.now().Add(time.Minute).Unix()))
	proof[105], proof[106] = 1, 1
	body, err := json.Marshal(DeviceBody{Platform: "web", Provider: "webpush", Token: token, DeviceProof: base64.RawURLEncoding.EncodeToString(append(proof, ed25519.Sign(private, proof)...))})
	must(t, err)
	schema := s.DB.Config().ConnConfig.RuntimeParams["search_path"]
	ddl, err := delivery.SchemaDDL(schema)
	must(t, err)
	_, err = s.DB.Exec(ctx, ddl)
	must(t, err)
	publisher, err := delivery.NewStore(s.DB, schema, delivery.Policy{Subjects: map[string]bool{"users.endpoint.bound": true, "users.endpoint.revoked": true}, EventTypes: map[string]bool{"endpoint.bound": true, "endpoint.revoked": true}})
	must(t, err)
	runtime := &Runtime{EndpointKey: bytes.Repeat([]byte{7}, 32), EndpointFingerprintKey: bytes.Repeat([]byte{8}, 32), DeviceVerifier: CompactDeviceProofVerifier(public), Delivery: publisher, Subjects: map[string]string{"endpoint.bound": "users.endpoint.bound", "endpoint.revoked": "users.endpoint.revoked"}}
	core := (HTTP{Store: s, Runtime: runtime}).Handler()
	identity := sharedauth.IdentityPrincipal{AccountID: p.AccountID.String(), SessionID: session.String(), Wallet: wallet, ProofAt: p.ProofAt, AccessExpiresAt: time.Now().Add(time.Hour)}
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		core.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityContextKey{}, identity)))
	})
	path := "/v2/me/devices/" + device.String()
	registered := httpRequest(handler, p, http.MethodPut, path, string(body))
	requireV2OwnerResponse(t, registered, http.StatusOK, "DeviceRegisterResponse")
	var registration v2.DeviceRegisterResponse
	must(t, json.Unmarshal(registered.Body.Bytes(), &registration))
	if registration.Data.DeviceId != device.String() || registration.Data.State != "active" || registration.Data.Revision != "2" || bytes.Contains(registered.Body.Bytes(), []byte(token)) {
		t.Fatal("device registration", registered.Body.String())
	}
	requireV2OwnerResponse(t, httpRequest(handler, p, http.MethodPut, path, string(body)), http.StatusForbidden, "Error")
	requireV2OwnerResponse(t, httpRequest(handler, Principal{}, http.MethodPut, path, string(body)), http.StatusUnauthorized, "Error")
	requireV2OwnerResponse(t, httpRequest(handler, Principal{}, http.MethodDelete, path, ""), http.StatusUnauthorized, "Error")
	for range 2 {
		revoked := httpRequest(handler, p, http.MethodDelete, path, "")
		if revoked.Code != http.StatusNoContent || revoked.Body.Len() != 0 {
			t.Fatalf("device revoke: %d %s", revoked.Code, revoked.Body.String())
		}
	}
	devices, err := s.Devices(ctx, p)
	must(t, err)
	if len(devices) != 1 || devices[0].DeviceID != device || devices[0].State != "revoked" {
		t.Fatal("device revocation did not persist", devices)
	}
}

type v2OwnerEOAVerifier struct{}

func (v2OwnerEOAVerifier) Verify(ctx context.Context, wallet, statement, signature string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := sharedauth.VerifyEOA(wallet, statement, signature, nil)
	return err
}

func TestPostgresHTTPV2OwnerExportsAndDeletion(t *testing.T) {
	s := v2OwnerFixture(t)
	p := owner(t, s, 94334)
	other := owner(t, s, 94335)
	handler := (HTTP{Store: s}).Handler()
	exported := httpRequest(handler, p, http.MethodPost, "/v2/me/exports", `{"format":"json","includeMedia":false}`)
	requireV2OwnerResponse(t, exported, http.StatusCreated, "AccountExportResponse")
	var export v2.AccountExportResponse
	must(t, json.Unmarshal(exported.Body.Bytes(), &export))
	if export.Data.Job.AccountId != p.AccountID.String() || export.Data.Job.Kind != "export" || export.Data.Job.State != "queued" {
		t.Fatal(export.Data.Job)
	}
	path := "/v2/me/exports/" + export.Data.Job.JobId
	status := httpRequest(handler, p, http.MethodGet, path, "")
	requireV2OwnerResponse(t, status, http.StatusOK, "AccountExportStatusResponse")
	var exportStatus v2.AccountExportStatusResponse
	must(t, json.Unmarshal(status.Body.Bytes(), &exportStatus))
	if exportStatus.Data.Job.JobId != export.Data.Job.JobId || exportStatus.Data.DownloadUrl != nil {
		t.Fatal(exportStatus.Data)
	}
	requireV2OwnerResponse(t, httpRequest(handler, other, http.MethodGet, path, ""), http.StatusNotFound, "Error")
	requireV2OwnerResponse(t, httpRequest(handler, Principal{}, http.MethodPost, "/v2/me/exports", `{"format":"json","includeMedia":false}`), http.StatusUnauthorized, "Error")
	requireV2OwnerResponse(t, httpRequest(handler, Principal{}, http.MethodGet, path, ""), http.StatusUnauthorized, "Error")
	requireV2OwnerResponse(t, httpRequest(handler, p, http.MethodPost, "/v2/me/exports", `{"format":"json","includeMedia":true}`), http.StatusNotImplemented, "Error")

	t.Run("deletion creation", func(t *testing.T) {
		key, err := crypto.GenerateKey()
		must(t, err)
		wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
		accountID, session := uuid.New(), uuid.New()
		must(t, s.Create(context.Background(), accountID, wallet))
		principal := Principal{AccountID: accountID, ProofAt: s.now()}
		seedConnectedIdentity(t, s, principal, session, wallet)
		reauthProof := strings.Repeat("ab", 32)
		reauthHash := sha256.Sum256([]byte(reauthProof))
		_, err = s.DB.Exec(context.Background(), `INSERT INTO identity_reauth(proof_hash,account_id,session_id,action,issued_at,expires_at) VALUES($1,$2,$3,'account.delete',clock_timestamp(),clock_timestamp()+interval '5 minutes')`, reauthHash[:], accountID, session)
		must(t, err)
		_, err = s.DB.Exec(context.Background(), sharedauth.LedgerDDL)
		must(t, err)
		policies, routes := sharedauth.AccountOwnerPoliciesV2()
		route := routes["account.delete"]
		route.Template = "/v2/me/deletion-requests"
		routes["account.delete"] = route
		policy := policies.Actions["account.delete"]
		policy.RouteTemplate = route.Template
		policies.Actions["account.delete"] = policy
		authorizer := &sharedauth.OperationAuthorizer{Ledger: sharedauth.Ledger{Pool: s.DB, Environment: "test", AuditKey: bytes.Repeat([]byte{9}, 32)}, Verifier: v2OwnerEOAVerifier{}, Policies: policies, Routes: routes, Transport: sharedauth.OperationTransport{EnvelopeHeader: "X-Test-Operation", SignatureHeader: "X-Test-Signature"}}
		body := fmt.Sprintf(`{"confirmAccountId":%q,"expectedRevision":"0","reauthProof":%q}`, accountID.String(), reauthProof)
		digest, err := sharedauth.BodyDigestV2([]byte(body))
		must(t, err)
		now := time.Now()
		envelope := sharedauth.OperationEnvelope{Version: 2, ChainID: "125", Environment: "test", Actor: wallet, Action: "account.delete", Method: http.MethodPost, Path: route.Template, BodyHash: digest, Nonce: strings.Repeat("cd", 32), IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()}
		statement, err := envelope.Statement()
		must(t, err)
		signature, err := crypto.Sign(accounts.TextHash([]byte(statement)), key)
		must(t, err)
		raw, err := json.Marshal(envelope)
		must(t, err)
		identity := sharedauth.IdentityPrincipal{AccountID: accountID.String(), SessionID: session.String(), Wallet: wallet, ProofAt: now, AccessExpiresAt: now.Add(time.Hour)}
		core := (HTTP{Store: s, Runtime: &Runtime{Authorizer: authorizer, EndpointKey: bytes.Repeat([]byte{7}, 32)}}).Handler()
		authorized := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Request-ID", uuid.NewString())
			r.Header.Set("Idempotency-Key", uuid.NewString())
			r.Header.Set(authorizer.Transport.EnvelopeHeader, base64.RawURLEncoding.EncodeToString(raw))
			r.Header.Set(authorizer.Transport.SignatureHeader, hexutil.Encode(signature))
			core.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityContextKey{}, identity)))
		})
		created := httpRequest(authorized, principal, http.MethodPost, route.Template, body)
		requireV2OwnerResponse(t, created, http.StatusAccepted, "AccountDeleteResponse")
		var deletion v2.AccountDeleteResponse
		must(t, json.Unmarshal(created.Body.Bytes(), &deletion))
		if deletion.Data.Job.AccountId != accountID.String() || deletion.Data.Job.Kind != "deletion" || deletion.Data.Job.JobId == "" || len(deletion.Data.RetentionSummary) != 2 || created.Header().Get("X-Recovery-Receipt") == "" {
			t.Fatal("deletion response", created.Body.String(), created.Header())
		}
		status := httpRequest(authorized, principal, http.MethodGet, route.Template+"/"+deletion.Data.Job.JobId, "")
		requireV2OwnerResponse(t, status, http.StatusOK, "AccountDeleteStatusResponse")
		var progress v2.AccountDeleteStatusResponse
		must(t, json.Unmarshal(status.Body.Bytes(), &progress))
		if progress.Data.Job.JobId != deletion.Data.Job.JobId {
			t.Fatal("deletion job id did not round-trip", progress.Data.Job)
		}
	})

	t.Run("deletion status and recovery", func(t *testing.T) {
		session := uuid.New()
		seedConnectedIdentity(t, s, p, session, fmt.Sprintf("0x%040x", 94334))
		job, err := s.RequestDeletion(context.Background(), p, "1", func(ctx context.Context, tx pgx.Tx, account uuid.UUID) error {
			return sharedauth.RevokeIdentityAccountTx(ctx, tx, account.String(), "account_deleted")
		})
		must(t, err)
		h := HTTP{Store: s, Runtime: &Runtime{EndpointKey: bytes.Repeat([]byte{7}, 32)}}
		handler := h.Handler()
		path := "/v2/me/deletion-requests/" + job.ID.String()
		response := httpRequest(handler, p, http.MethodGet, path, "")
		requireV2OwnerResponse(t, response, http.StatusOK, "AccountDeleteStatusResponse")
		var progress v2.AccountDeleteStatusResponse
		must(t, json.Unmarshal(response.Body.Bytes(), &progress))
		if progress.Data.Job.JobId != job.ID.String() || progress.Data.Job.Kind != "deletion" || len(progress.Data.Job.DomainResults) != 4 || len(progress.Data.Job.RetainedReasons) != 2 {
			t.Fatal(progress.Data.Job)
		}
		receipt, err := h.recoveryReceipt(deletionProjection(job).Job)
		must(t, err)
		recovery := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Recovery-Receipt", receipt)
			h.ServiceHandler(nil).ServeHTTP(w, r)
		})
		requireV2OwnerResponse(t, httpRequest(recovery, Principal{}, http.MethodGet, path, ""), http.StatusOK, "AccountDeleteStatusResponse")
		requireV2OwnerResponse(t, httpRequest(handler, other, http.MethodGet, path, ""), http.StatusNotFound, "Error")
		requireV2OwnerResponse(t, httpRequest(handler, Principal{}, http.MethodPost, "/v2/me/deletion-requests", `{}`), http.StatusUnauthorized, "Error")
	})
}

func TestOwnerActionCoversV2Paths(t *testing.T) {
	for _, test := range []struct {
		method, path, action string
	}{
		{http.MethodPatch, "/profiles/0x0000000000000000000000000000000000000001", "profile.update"},
		{http.MethodPatch, "/me/preferences", "preferences.update"},
		{http.MethodPut, "/me/watchlist/0x0000000000000000000000000000000000000001", "watchlist.update"},
		{http.MethodDelete, "/me/watchlist/0x0000000000000000000000000000000000000001", "watchlist.remove"},
		{http.MethodPut, "/me/devices/" + uuid.NewString(), "device.register"},
		{http.MethodDelete, "/me/devices/" + uuid.NewString(), "device.revoke"},
		{http.MethodPost, "/me/exports", "account.export"},
	} {
		for _, version := range []string{"/v1", "/v2"} {
			if got := ownerAction(test.method, version+test.path); got != test.action {
				t.Errorf("%s %s%s: %q, want %q", test.method, version, test.path, got, test.action)
			}
		}
	}
	for path, action := range map[string]string{
		"/v3/profiles/wallet": "profile.update",
		"/v3/me/preferences":  "preferences.update",
	} {
		if got := ownerAction(http.MethodPatch, path); got != action {
			t.Errorf("existing v3 action %s: %q, want %q", path, got, action)
		}
	}
	for _, path := range []string{"/v2/unknown", "/v2/me/preferences/unknown", "/v2/me/exports/job", "/v2/me/deletion-requests", "/v2/me/devices", "/v2/me/watchlist", "/v4/me/preferences", ""} {
		if got := ownerAction(http.MethodPost, path); got != "" {
			t.Errorf("unknown path %s mapped to %q", path, got)
		}
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, path := range []string{"/v2/profiles/wallet", "/v2/me/preferences", "/v2/me/watchlist/pool", "/v2/me/devices/device", "/v2/me/exports"} {
			if got := ownerAction(method, path); got != "" {
				t.Errorf("read method %s %s mapped to %q", method, path, got)
			}
		}
	}
}
