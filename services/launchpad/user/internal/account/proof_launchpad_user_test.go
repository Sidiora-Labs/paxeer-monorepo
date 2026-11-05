// Proves: Store and AdaptV2Preferences/AdaptV2DeviceRegister/AdaptV2AccountJob/AdaptV2ProfilePatch/AdaptV2Watchlist -> HTTP.Handler over TCP -> v2.Decode*Response.
//
// The consumer is this test, using the exported generated Go decoders. The
// server starts at Handler's authenticated-context boundary: WithPrincipal and
// the verified identity context are connection setup, not replacement handlers.
// PostgreSQL, route dispatch, mutations, signed device verification, adapters,
// response serialization and consumer decoding all run their real code paths.
// Closing the server and pool before reopening also proves durable observations.
// The proof owns a private PostgreSQL process initialized from repository
// migrations; it requires no database URL or operator-provisioned service.
//
// Repairs exposed here: preferences_http.go ownerAction classified unsupported
// methods and extra path segments as owner actions; v2_owner_wire.go
// v2OwnerEnvelope collapsed duplicate producer keys before validating them;
// v2_wire.go AdaptV2Watchlist accepted positions outside the 500-entry store
// bound; v2_owner_http.go deviceV2Delete ignored mutation bodies; and
// v2_watchlist_http.go watchV2Delete ignored undeclared owner query parameters.
// v2_owner_http.go exportV2Post now snapshots the request timestamp: the
// underlying privacy.go RequestExport reads the clock separately for its
// returned job and its INSERT, so v2 must give both the same instant to preserve
// the consumer's complete job value after reopening.
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
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/migrations"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

func proofLaunchpadUserStore(t *testing.T) Store {
	t.Helper()
	initdb, err := exec.LookPath("initdb")
	if err != nil {
		matches, globErr := filepath.Glob("/usr/lib/postgresql/*/bin/initdb")
		must(t, globErr)
		if len(matches) == 0 {
			t.Fatal("services/launchpad/user/internal/account/proof_launchpad_user_test.go: PostgreSQL initdb producer binary is required")
		}
		initdb = matches[len(matches)-1]
	}
	root, err := os.MkdirTemp("", "launchpad-user-proof-")
	must(t, err)
	t.Cleanup(func() { must(t, os.RemoveAll(root)) })
	var credential *syscall.Credential
	if os.Geteuid() == 0 {
		account, err := user.Lookup("postgres")
		must(t, err)
		uid, err := strconv.ParseUint(account.Uid, 10, 32)
		must(t, err)
		gid, err := strconv.ParseUint(account.Gid, 10, 32)
		must(t, err)
		credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		must(t, os.Chown(root, int(uid), int(gid)))
	}
	run := func(binary string, args ...string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = root
		if credential != nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
		}
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("PostgreSQL producer %s: %w: %s", filepath.Base(binary), err, output)
		}
		return nil
	}
	data := filepath.Join(root, "data")
	must(t, run(initdb, "-D", data, "-U", "postgres", "--auth=trust", "--no-locale", "-E", "UTF8"))
	pgctl := filepath.Join(filepath.Dir(initdb), "pg_ctl")
	must(t, run(pgctl, "-D", data, "-l", filepath.Join(root, "postgres.log"), "-o", "-h '' -p 5432 -k "+root, "-w", "-t", "20", "start"))
	t.Cleanup(func() { must(t, run(pgctl, "-D", data, "-m", "fast", "-w", "-t", "20", "stop")) })

	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig("postgres://postgres@localhost/postgres?sslmode=disable")
	must(t, err)
	cfg.ConnConfig.Host = root
	cfg.ConnConfig.RuntimeParams["search_path"] = "launchpad_user_proof"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	must(t, err)
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, "CREATE SCHEMA launchpad_user_proof")
	must(t, err)
	db := stdlib.OpenDB(*cfg.ConnConfig)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS, goose.WithExcludeVersions([]int64{1}))
	must(t, err)
	_, err = provider.Up(ctx)
	must(t, err)
	versions, err := database.NewStore(database.DialectPostgres, "v2_owner_identity_version")
	must(t, err)
	identity, err := goose.NewProvider("", db, os.DirFS("../../../gateway/internal/devidentity/migrations"), goose.WithStore(versions))
	must(t, err)
	_, err = identity.Up(ctx)
	must(t, err)
	return Store{DB: pool, Clock: func() time.Time { return time.Now().UTC() }}
}

type proofLaunchpadUserClient struct {
	t      *testing.T
	server *httptest.Server
}

func (c proofLaunchpadUserClient) request(method, path, body string, status int) []byte {
	c.t.Helper()
	req, err := http.NewRequest(method, c.server.URL+path, strings.NewReader(body))
	must(c.t, err)
	id := uuid.NewString()
	req.Header.Set("X-Request-ID", id)
	req.Header.Set("Content-Type", "application/json")
	response, err := c.server.Client().Do(req)
	must(c.t, err)
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	must(c.t, err)
	if response.StatusCode != status {
		c.t.Fatalf("%s %s: status %d, want %d: %s", method, path, response.StatusCode, status, raw)
	}
	if response.Header.Get("Cache-Control") != "private, no-store" || response.Header.Get("X-Content-Type-Options") != "nosniff" {
		c.t.Fatalf("%s headers: %v", path, response.Header)
	}
	if status == http.StatusNoContent {
		if len(raw) != 0 {
			c.t.Fatalf("DELETE returned a body: %s", raw)
		}
		return raw
	}
	if response.Header.Get("Content-Type") != "application/json" {
		c.t.Fatal(response.Header)
	}
	var envelope struct {
		RequestID string `json:"requestId"`
	}
	must(c.t, json.Unmarshal(raw, &envelope))
	if envelope.RequestID != id {
		c.t.Fatalf("cached or uncorrelated response: %s != %s", envelope.RequestID, id)
	}
	if status >= 400 {
		must(c.t, v2.ValidateJSON("Error", raw))
	}
	return raw
}

func TestProofLaunchpadUserEndToEnd(t *testing.T) {
	s := proofLaunchpadUserStore(t)
	ctx := context.Background()
	p := owner(t, s, 213101)
	other := owner(t, s, 213102)
	wallet := fmt.Sprintf("0x%040x", 213101)
	session, device := uuid.New(), uuid.New()
	seedConnectedIdentity(t, s, p, session, wallet)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	must(t, err)
	schema := s.DB.Config().ConnConfig.RuntimeParams["search_path"]
	ddl, err := delivery.SchemaDDL(schema)
	must(t, err)
	_, err = s.DB.Exec(ctx, ddl)
	must(t, err)
	subjects, allowedSubjects, eventTypes := map[string]string{}, map[string]bool{}, map[string]bool{}
	for _, kind := range []string{"profile.updated", "preferences.updated", "watchlist.changed", "endpoint.bound", "endpoint.revoked", "account.export.requested"} {
		subjects[kind] = "users." + kind
		allowedSubjects[subjects[kind]], eventTypes[kind] = true, true
	}
	publisher, err := delivery.NewStore(s.DB, schema, delivery.Policy{Subjects: allowedSubjects, EventTypes: eventTypes})
	must(t, err)
	runtime := &Runtime{EndpointKey: bytes.Repeat([]byte{7}, 32), EndpointFingerprintKey: bytes.Repeat([]byte{8}, 32), DeviceVerifier: CompactDeviceProofVerifier(public), Delivery: publisher, Subjects: subjects}
	// The canonical registry is a real PostgreSQL table; lookup queries it rather
	// than substituting an assumed pool-existence result.
	_, err = s.DB.Exec(ctx, `CREATE TABLE indexer_canonical_creations_v1(pool text PRIMARY KEY)`)
	must(t, err)
	pool := fmt.Sprintf("0x%040x", 213103)
	_, err = s.DB.Exec(ctx, `INSERT INTO indexer_canonical_creations_v1 VALUES($1)`, pool)
	must(t, err)
	lookup := func(ctx context.Context, chain int, address string) (bool, error) {
		if chain != 125 {
			return false, ErrInvalid
		}
		var exists bool
		err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM indexer_canonical_creations_v1 WHERE pool=$1)`, address).Scan(&exists)
		return exists, err
	}
	start := func(principal Principal, identity sharedauth.IdentityPrincipal) proofLaunchpadUserClient {
		server := httptest.NewUnstartedServer((HTTP{Store: s, Runtime: runtime, PoolLookup: lookup, CursorKey: bytes.Repeat([]byte{9}, 32)}).Handler())
		server.Config.ConnContext = func(ctx context.Context, _ net.Conn) context.Context {
			return context.WithValue(WithPrincipal(ctx, principal), identityContextKey{}, identity)
		}
		server.Start()
		t.Cleanup(server.Close)
		return proofLaunchpadUserClient{t, server}
	}
	identity := sharedauth.IdentityPrincipal{AccountID: p.AccountID.String(), SessionID: session.String(), Wallet: wallet, ProofAt: p.ProofAt, AccessExpiresAt: time.Now().Add(time.Hour)}
	client := start(p, identity)
	stranger := start(other, sharedauth.IdentityPrincipal{})

	// The adapter producers are reached through the registered v2 routes.
	initial, err := v2.DecodePreferencesReadResponse(client.request("GET", "/v2/me/preferences", "", 200))
	must(t, err)
	if initial.Data.Preferences.Locale != "en" || initial.Data.Preferences.Revision != "0" {
		t.Fatal(initial.Data)
	}
	preferences, err := v2.DecodePreferencesUpdateResponse(client.request("PATCH", "/v2/me/preferences", `{"expectedRevision":"0","locale":"fr-CA","timezone":"America/Toronto"}`, 200))
	must(t, err)
	if preferences.Data.Preferences.Locale != "fr-CA" || preferences.Data.Preferences.Revision != "1" {
		t.Fatal(preferences.Data)
	}
	client.request("PATCH", "/v2/me/preferences", `{"expectedRevision":"0","locale":"de"}`, 412)
	profileRaw := client.request("PATCH", "/v2/profiles/"+wallet, `{"expectedRevision":"1","displayName":"Launchpad producer","bio":"real consumer"}`, 200)
	profile, err := v2.DecodeProfilePatchResponse(profileRaw)
	must(t, err)
	if profile.Data.Profile.DisplayName != "Launchpad producer" || profile.Data.Profile.Revision != "2" {
		t.Fatal(profile.Data)
	}
	stranger.request("PATCH", "/v2/profiles/"+wallet, `{"expectedRevision":"2","bio":"stolen"}`, 403)
	for range 2 {
		current, err := s.Public(ctx, wallet)
		must(t, err)
		body := fmt.Sprintf(`{"expectedRevision":%q,"displayName":"Launchpad producer","bio":"real consumer"}`, current.Revision)
		observed, err := v2.DecodeProfilePatchResponse(client.request("PATCH", "/v2/profiles/"+wallet, body, 200))
		must(t, err)
		if observed.Data.Profile.DisplayName != current.DisplayName || observed.Data.Profile.Bio != current.Bio || string(observed.Data.Profile.Revision) == current.Revision || observed.Revision == nil || string(*observed.Revision) != string(observed.Data.Profile.Revision) {
			t.Fatal(observed.Data)
		}
	}
	watch, err := v2.DecodeWatchlistPutResponse(client.request("PUT", "/v2/me/watchlist/"+pool, `{"alertEnabled":true}`, 200))
	must(t, err)
	if watch.Data.Entry.PoolAddress != pool || !watch.Data.Entry.AlertEnabled || watch.Data.Entry.Wallet != wallet {
		t.Fatal(watch.Data)
	}
	client.request("PUT", "/v2/me/watchlist/"+fmt.Sprintf("0x%040x", 213104), `{"alertEnabled":true}`, 404)
	token := "proof-launchpad-provider-token"
	devicePath := "/v2/me/devices/" + device.String()
	var deviceRaw []byte
	for range 2 {
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
		deviceRaw = client.request("PUT", devicePath, string(body), 200)
		registered, err := v2.DecodeDeviceRegisterResponse(deviceRaw)
		must(t, err)
		if registered.Data.DeviceId != device.String() || registered.Data.State != "active" {
			t.Fatal(registered.Data)
		}
		client.request("PUT", devicePath, string(body), 403) // replayed provider proof
	}
	exported, err := v2.DecodeAccountExportResponse(client.request("POST", "/v2/me/exports", `{"format":"json","includeMedia":false}`, 201))
	must(t, err)
	if exported.Data.Job.AccountId != p.AccountID.String() || exported.Data.Job.Kind != "export" || exported.Data.Job.State != "queued" {
		t.Fatal(exported.Data)
	}
	exportPath := "/v2/me/exports/" + exported.Data.Job.JobId
	stranger.request("GET", exportPath, "", 404)
	client.request("GET", "/v2/me/exports/"+uuid.NewString(), "", 404)
	client.request("GET", "/v2/me/exports/not-a-uuid", "", 422)

	// A new service and pool must reconstruct every read from PostgreSQL.
	client.server.Close()
	stranger.server.Close()
	cfg := s.DB.Config()
	s.DB.Close()
	reopened, err := pgxpool.NewWithConfig(ctx, cfg)
	must(t, err)
	s.DB = reopened
	t.Cleanup(reopened.Close)
	publisher, err = delivery.NewStore(reopened, schema, delivery.Policy{Subjects: allowedSubjects, EventTypes: eventTypes})
	must(t, err)
	runtime.Delivery = publisher
	client = start(p, identity)
	var watchRaw, preferencesRaw, exportRaw []byte
	for range 2 {
		preferencesRaw = client.request("GET", "/v2/me/preferences", "", 200)
		observed, err := v2.DecodePreferencesReadResponse(preferencesRaw)
		must(t, err)
		if observed.Data.Preferences.Locale != "fr-CA" || observed.Data.Preferences.Timezone != "America/Toronto" {
			t.Fatal(observed.Data)
		}
		watchRaw = client.request("GET", "/v2/me/watchlist", "", 200)
		page, err := v2.DecodeWatchlistGetResponse(watchRaw)
		must(t, err)
		if len(page.Data.Items) != 1 || !reflect.DeepEqual(page.Data.Items[0], watch.Data.Entry) {
			t.Fatal(page.Data)
		}
		exportRaw = client.request("GET", exportPath, "", 200)
		progress, err := v2.DecodeAccountExportStatusResponse(exportRaw)
		must(t, err)
		if !reflect.DeepEqual(progress.Data.Job, exported.Data.Job) || progress.Data.DownloadUrl != nil {
			t.Fatal(progress.Data)
		}
	}

	// Corrupt actual producer output at the consumer boundary; the generated
	// decoders must refuse it, even when ordinary json.Unmarshal would succeed.
	for _, test := range []struct {
		raw    []byte
		path   []string
		value  json.RawMessage
		decode func([]byte) error
	}{
		{deviceRaw, []string{"data", "deviceId"}, json.RawMessage(`"unknown"`), func(b []byte) error { _, e := v2.DecodeDeviceRegisterResponse(b); return e }},
		{profileRaw, []string{"data", "profile", "revision"}, json.RawMessage(`17`), func(b []byte) error { _, e := v2.DecodeProfilePatchResponse(b); return e }},
		{preferencesRaw, []string{"data", "preferences", "locale"}, json.RawMessage(`17`), func(b []byte) error { _, e := v2.DecodePreferencesReadResponse(b); return e }},
		{watchRaw, []string{"data", "items"}, json.RawMessage(`null`), func(b []byte) error { _, e := v2.DecodeWatchlistGetResponse(b); return e }},
		{exportRaw, []string{"data", "job", "jobId"}, json.RawMessage(`"unknown"`), func(b []byte) error { _, e := v2.DecodeAccountExportStatusResponse(b); return e }},
	} {
		bad := v2OwnerTestMutation(t, test.raw, test.path, test.value)
		if test.decode(bad) == nil {
			t.Fatalf("consumer accepted malformed producer result at %v", test.path)
		}
	}
	legacy := client.request("GET", "/v1/me/preferences", "", 200)
	duplicate := append([]byte(`{"requestId":"ambiguous",`), legacy[1:]...)
	if raw, err := AdaptV2Preferences(duplicate, false); err == nil || raw != nil {
		t.Fatal("duplicate producer keys accepted")
	}
	legacyWatch := client.request("GET", "/v1/me/watchlist", "", 200)
	var legacyPage Envelope[struct {
		Items []WireWatchEntry `json:"items"`
	}]
	must(t, json.Unmarshal(legacyWatch, &legacyPage))
	envelope, err := Response(uuid.New(), legacyPage.Data.Items, *legacyPage.Revision, s.now())
	must(t, err)
	envelope.Data[0].Order = 500
	if raw, err := AdaptV2Watchlist(envelope); err == nil || raw != nil {
		t.Fatal("out-of-store-range order accepted")
	}
	putEnvelope, err := Response(uuid.New(), envelope.Data[0], *legacyPage.Revision, s.now())
	must(t, err)
	if raw, err := AdaptV2WatchPut(putEnvelope); err == nil || raw != nil {
		t.Fatal("out-of-store-range put order accepted")
	}
	for _, adapt := range []struct {
		raw    []byte
		decode func([]byte) ([]byte, error)
	}{
		{deviceRaw, AdaptV2DeviceRegister},
		{exportRaw, func(b []byte) ([]byte, error) { return AdaptV2AccountJob(b, "export", true) }},
	} {
		duplicate := append([]byte(`{"requestId":"ambiguous",`), adapt.raw[1:]...)
		if raw, err := adapt.decode(duplicate); err == nil || raw != nil {
			t.Fatal("duplicate producer keys accepted")
		}
	}
	if raw, err := AdaptV2AccountJob(exportRaw, "deletion", true); err == nil || raw != nil {
		t.Fatal("export decoded as deletion")
	}

	anonymous := start(Principal{}, sharedauth.IdentityPrincipal{})
	for _, target := range []struct{ method, path string }{
		{"GET", "/v2/me/preferences"},
		{"GET", "/v2/me/watchlist"},
		{"DELETE", devicePath},
		{"DELETE", "/v2/me/watchlist/" + pool},
		{"GET", exportPath},
		{"GET", "/v2/me/deletion-requests/" + uuid.NewString()},
	} {
		anonymous.request(target.method, target.path, "", 401)
	}
	client.request("DELETE", devicePath, `{}`, 422)
	devices, err := s.Devices(ctx, p)
	must(t, err)
	if len(devices) != 1 || devices[0].State != "active" {
		t.Fatal("malformed deletion mutated device", devices)
	}
	client.request("DELETE", "/v2/me/watchlist/"+pool+"?accountId="+other.AccountID.String(), "", 422)
	client.request("DELETE", "/v2/me/watchlist/"+pool, `{}`, 422)
	for range 2 {
		client.request("DELETE", devicePath, "", 204)
		client.request("DELETE", "/v2/me/watchlist/"+pool, "", 204)
		devices, err = s.Devices(ctx, p)
		must(t, err)
		if len(devices) != 1 || devices[0].State != "revoked" {
			t.Fatal(devices)
		}
		empty, err := v2.DecodeWatchlistGetResponse(client.request("GET", "/v2/me/watchlist", "", 200))
		must(t, err)
		if len(empty.Data.Items) != 0 {
			t.Fatal(empty.Data)
		}
	}

	// The real store creates deletion state and revokes the real identity session.
	// Recovery is the production capability for reading after that revocation.
	deletionOwner := owner(t, s, 213105)
	deletionSession := uuid.New()
	seedConnectedIdentity(t, s, deletionOwner, deletionSession, fmt.Sprintf("0x%040x", 213105))
	job, err := s.RequestDeletion(ctx, deletionOwner, "0", func(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
		return sharedauth.RevokeIdentityAccountTx(ctx, tx, id.String(), "account_deleted")
	})
	must(t, err)
	client.server.Close()
	anonymous.server.Close()
	cfg = s.DB.Config()
	s.DB.Close()
	reopened, err = pgxpool.NewWithConfig(ctx, cfg)
	must(t, err)
	s.DB = reopened
	t.Cleanup(reopened.Close)
	var revoked bool
	must(t, s.DB.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM identity_sessions WHERE session_id=$1`, deletionSession).Scan(&revoked))
	if !revoked {
		t.Fatal("deletion did not durably revoke the identity session")
	}
	recoveryHTTP := HTTP{Store: s, Runtime: runtime}
	receipt, err := recoveryHTTP.recoveryReceipt(deletionProjection(job).Job)
	must(t, err)
	recoveryServer := httptest.NewServer(recoveryHTTP.ServiceHandler(nil))
	t.Cleanup(recoveryServer.Close)
	deletionPath := "/v2/me/deletion-requests/" + job.ID.String()
	for range 2 {
		req, err := http.NewRequest("GET", recoveryServer.URL+deletionPath, nil)
		must(t, err)
		req.Header.Set("X-Recovery-Receipt", receipt)
		response, err := recoveryServer.Client().Do(req)
		must(t, err)
		raw, err := io.ReadAll(response.Body)
		response.Body.Close()
		must(t, err)
		if response.StatusCode != 200 {
			t.Fatalf("recovery: %d %s", response.StatusCode, raw)
		}
		progress, err := v2.DecodeAccountDeleteStatusResponse(raw)
		must(t, err)
		if progress.Data.Job.JobId != job.ID.String() || progress.Data.Job.AccountId != deletionOwner.AccountID.String() || progress.Data.Job.Kind != "deletion" || len(progress.Data.Job.DomainResults) != 4 || len(progress.Data.Job.RetainedReasons) != 2 {
			t.Fatal(progress.Data)
		}
		malformed := v2OwnerTestMutation(t, raw, []string{"data", "job", "state"}, json.RawMessage(`"unknown"`))
		if _, err := v2.DecodeAccountDeleteStatusResponse(malformed); err == nil {
			t.Fatal("consumer accepted malformed deletion state")
		}
		req.Header.Set("X-Recovery-Receipt", "malformed")
		response, err = recoveryServer.Client().Do(req)
		must(t, err)
		raw, err = io.ReadAll(response.Body)
		response.Body.Close()
		must(t, err)
		if response.StatusCode != 403 {
			t.Fatalf("bad recovery accepted: %d %s", response.StatusCode, raw)
		}
		must(t, v2.ValidateJSON("Error", raw))
	}
}

func TestProofLaunchpadUserOwnerActions(t *testing.T) {
	for _, test := range []struct{ method, path, action string }{
		{"PATCH", "/v2/profiles/wallet", "profile.update"},
		{"PATCH", "/v2/me/preferences", "preferences.update"},
		{"PUT", "/v2/me/devices/device", "device.register"},
		{"DELETE", "/v2/me/devices/device", "device.revoke"},
		{"PUT", "/v2/me/watchlist/pool", "watchlist.update"},
		{"DELETE", "/v2/me/watchlist/pool", "watchlist.remove"},
		{"POST", "/v2/me/exports", "account.export"},
		{"POST", "/v2/profiles/wallet", ""},
		{"DELETE", "/v2/me/preferences", ""},
		{"PATCH", "/v2/me/devices/device", ""},
		{"POST", "/v2/me/watchlist/pool", ""},
		{"POST", "/v2/me/exports/job", ""},
		{"GET", "/v2/me/exports/job", ""},
		{"GET", "/v2/me/deletion-requests/job", ""},
		{"PUT", "/v2/me/devices/device/extra", ""},
		{"PATCH", "/v2/profiles/", ""},
		{"PUT", "/v3/me/devices/device", ""},
	} {
		if got := ownerAction(test.method, test.path); got != test.action {
			t.Fatalf("%s %s: %q != %q", test.method, test.path, got, test.action)
		}
	}
}
