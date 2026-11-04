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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func repair76Reopen(t *testing.T, s Store) Store {
	t.Helper()
	cfg := s.DB.Config().Copy()
	s.DB.Close()
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	must(t, err)
	must(t, pool.Ping(context.Background()))
	t.Cleanup(pool.Close)
	s.DB = pool
	return s
}

func repair76Publisher(t *testing.T, s Store, kinds ...string) (*delivery.Store, map[string]string) {
	t.Helper()
	schema := s.DB.Config().ConnConfig.RuntimeParams["search_path"]
	if schema == "" {
		schema = "public"
	}
	ddl, err := delivery.SchemaDDL(schema)
	must(t, err)
	_, err = s.DB.Exec(context.Background(), ddl)
	must(t, err)
	subjects, allowed, events := map[string]string{}, map[string]bool{}, map[string]bool{}
	for _, kind := range kinds {
		subjects[kind] = "users." + kind
		allowed[subjects[kind]], events[kind] = true, true
	}
	publisher, err := delivery.NewStore(s.DB, schema, delivery.Policy{Subjects: allowed, EventTypes: events})
	must(t, err)
	return publisher, subjects
}

func TestConnectedRepair76_device_token_rotation(t *testing.T) {
	s := connectedFixture(t)
	s.Clock = func() time.Time { return time.Now().UTC() }
	ctx := context.Background()
	p := owner(t, s, 7601)
	wallet := fmt.Sprintf("0x%040x", 7601)
	session := uuid.New()
	seedConnectedIdentity(t, s, p, session, wallet)
	must(t, s.SetConsent(ctx, p, p.AccountID, "0", Consent{Category: "watchlist", Channel: "push", Enabled: true, Source: "settings", PolicyVersion: "1"}))
	public, private, err := ed25519.GenerateKey(rand.Reader)
	must(t, err)
	publisher, subjects := repair76Publisher(t, s, "endpoint.bound")
	runtime := &Runtime{EndpointKey: bytes.Repeat([]byte{76}, 32), EndpointFingerprintKey: bytes.Repeat([]byte{77}, 32), DeviceVerifier: CompactDeviceProofVerifier(public), Delivery: publisher, Subjects: subjects}
	identity := sharedauth.IdentityPrincipal{AccountID: p.AccountID.String(), SessionID: session.String(), Wallet: wallet, ProofAt: p.ProofAt, AccessExpiresAt: time.Now().Add(time.Hour)}
	core := (HTTP{Store: s, Runtime: runtime}).Handler()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(WithPrincipal(r.Context(), p), identityContextKey{}, identity)
		core.ServeHTTP(w, r.WithContext(ctx))
	}))
	t.Cleanup(server.Close)
	register := func(device uuid.UUID, token string, status int) {
		t.Helper()
		proof := make([]byte, 107)
		proof[0] = 2
		copy(proof[1:17], p.AccountID[:])
		copy(proof[17:33], session[:])
		copy(proof[33:49], device[:])
		digest := sha256.Sum256([]byte(token))
		copy(proof[49:81], digest[:])
		_, err := rand.Read(proof[81:97])
		must(t, err)
		binary.BigEndian.PutUint64(proof[97:105], uint64(s.now().Add(time.Minute).Unix()))
		proof[105], proof[106] = 3, 3
		body, err := json.Marshal(DeviceBody{Platform: "android", Provider: "fcm", Token: token, DeviceProof: base64.RawURLEncoding.EncodeToString(append(proof, ed25519.Sign(private, proof)...))})
		must(t, err)
		req, err := http.NewRequest(http.MethodPut, server.URL+"/v2/me/devices/"+device.String(), bytes.NewReader(body))
		must(t, err)
		response, err := server.Client().Do(req)
		must(t, err)
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		must(t, err)
		if response.StatusCode != status {
			t.Fatalf("registration: %d, want %d: %s", response.StatusCode, status, raw)
		}
		if status == http.StatusOK {
			decoded, err := v2.DecodeDeviceRegisterResponse(raw)
			must(t, err)
			if decoded.Data.DeviceId != device.String() {
				t.Fatal(decoded)
			}
		}
	}
	device := uuid.New()
	for i := 0; i < 40; i++ {
		register(device, fmt.Sprintf("installation-token-%d", i), http.StatusOK)
	}
	for i := 0; i < 31; i++ {
		register(uuid.New(), fmt.Sprintf("other-installation-%d", i), http.StatusOK)
	}
	register(device, "current-token-at-capacity", http.StatusOK)
	register(uuid.New(), "over-capacity", http.StatusConflict)
	server.Close()
	s = repair76Reopen(t, s)
	devices, err := s.Devices(ctx, p)
	must(t, err)
	if len(devices) != 32 {
		t.Fatalf("target count after reopen = %d", len(devices))
	}
	seen := map[uuid.UUID]bool{}
	for _, binding := range devices {
		if seen[binding.DeviceID] {
			t.Fatal("duplicate installation", binding.DeviceID)
		}
		seen[binding.DeviceID] = true
	}
	var count int
	var sealed []byte
	var revision int64
	must(t, s.DB.QueryRow(ctx, `SELECT count(*) FROM account_endpoint WHERE account_id=$1 AND device_id=$2`, p.AccountID, device).Scan(&count))
	if count != 1 {
		t.Fatalf("single-target lookup has %d candidates", count)
	}
	must(t, s.DB.QueryRow(ctx, `SELECT ciphertext,consent_revision FROM account_endpoint WHERE account_id=$1 AND device_id=$2`, p.AccountID, device).Scan(&sealed, &revision))
	plain, err := Open(runtime.EndpointKey, sealed, []byte(p.AccountID.String()))
	must(t, err)
	if string(plain) != "current-token-at-capacity" {
		t.Fatalf("recovered stale endpoint %q", plain)
	}
	if revision < 1 {
		t.Fatal("endpoint revision was not persisted")
	}
	var consentRevision int64
	must(t, s.DB.QueryRow(ctx, `SELECT revision FROM account_consent WHERE account_id=$1 AND category='watchlist' AND channel='push'`, p.AccountID).Scan(&consentRevision))
	eligible, err := s.EndpointEligible(ctx, p.AccountID, device, "watchlist", "push", consentRevision)
	must(t, err)
	if !eligible {
		t.Fatal("recovered endpoint is not deliverable")
	}
}

func TestConnectedRepair76_backfill_profile_fields(t *testing.T) {
	s := connectedLegacyFixture(t)
	ctx := context.Background()
	p := owner(t, s, 7602)
	wallet := fmt.Sprintf("0x%040x", 7602)
	seedConnectedIdentity(t, s, p, uuid.New(), wallet)
	_, err := s.DB.Exec(ctx, `INSERT INTO users.user_profiles(wallet_address,display_name,bio,website,twitter,telegram,discord,created_at,updated_at) VALUES($1,'Legacy profile','Legacy biography','https://example.org/profile','https://x.com/profile','https://t.me/profile','https://discord.com/profile',1,2)`, wallet)
	must(t, err)
	publisher, subjects := repair76Publisher(t, s, "account.backfill.completed")
	runtime := &Runtime{Delivery: publisher, Subjects: subjects}
	counts, err := (HTTP{Store: s, Runtime: runtime}).BackfillLegacyRange(ctx, BackfillRange{ID: "full-profile", Limit: 100})
	must(t, err)
	if counts.Verified != 1 || counts.Linked != 1 || counts.Quarantined != 0 {
		t.Fatal(counts)
	}
	must(t, s.SetStableAccountReads(ctx, false))
	before, err := s.PublicFromReadSwitch(ctx, wallet)
	must(t, err)
	s = repair76Reopen(t, s)
	must(t, s.SetStableAccountReads(ctx, true))
	after, err := s.PublicFromReadSwitch(ctx, wallet)
	must(t, err)
	if !reflect.DeepEqual(before.Profile, after.Profile) || after.Source != "stable" {
		t.Fatal(before, after)
	}
	var stored Profile
	var raw []byte
	must(t, s.DB.QueryRow(ctx, `SELECT profile FROM account_state WHERE account_id=$1`, p.AccountID).Scan(&raw))
	must(t, json.Unmarshal(raw, &stored))
	wantSocials := map[string]string{"twitter": "https://x.com/profile", "telegram": "https://t.me/profile", "discord": "https://discord.com/profile"}
	if !reflect.DeepEqual(stored.Socials, wantSocials) {
		t.Fatal(stored.Socials)
	}
	handler := (HTTP{Store: s, Runtime: &Runtime{UseReadSwitch: true}}).Handler()
	response := httpRequest(handler, Principal{}, http.MethodGet, "/v2/profiles/"+wallet, "")
	decoded, err := v2.DecodeProfileGetResponse(response.Body.Bytes())
	must(t, err)
	if decoded.Data.Profile.Website == nil || *decoded.Data.Profile.Website != "https://example.org/profile" {
		t.Fatal(decoded)
	}
	must(t, s.SetStableAccountReads(ctx, false))
	_, err = s.DB.Exec(ctx, `UPDATE users.user_profiles SET website='https://example.org/changed' WHERE wallet_address=$1`, wallet)
	must(t, err)
	if err = s.SetStableAccountReads(ctx, true); !errors.Is(err, ErrState) {
		t.Fatal("unverified source change enabled cutover", err)
	}
	counts, err = s.BackfillLegacyRange(ctx, BackfillRange{ID: "changed-source", Limit: 100})
	must(t, err)
	if counts.Quarantined != 1 || counts.Verified != 0 {
		t.Fatal(counts)
	}
}

func TestConnectedRepair76_backfill_unresolved_fields_and_assets(t *testing.T) {
	for _, asset := range []bool{false, true} {
		t.Run(fmt.Sprintf("asset_%t", asset), func(t *testing.T) {
			s := connectedLegacyFixture(t)
			ctx := context.Background()
			p := owner(t, s, 7603)
			wallet := fmt.Sprintf("0x%040x", 7603)
			seedConnectedIdentity(t, s, p, uuid.New(), wallet)
			_, err := s.DB.Exec(ctx, `INSERT INTO users.user_profiles(wallet_address,display_name,bio,twitter,created_at,updated_at) VALUES($1,'Legacy','bio',$2,1,1)`, wallet, map[bool]string{false: "javascript:alert(1)", true: "https://x.com/profile"}[asset])
			must(t, err)
			if asset {
				_, err = s.DB.Exec(ctx, `INSERT INTO users.user_images VALUES($1,$2,'avatar','legacy/avatar.png','image/png',100,1)`, uuid.NewString(), wallet)
				must(t, err)
			}
			counts, err := s.BackfillLegacyRange(ctx, BackfillRange{ID: "unresolved", Limit: 100})
			must(t, err)
			if counts.Quarantined != 1 || counts.Verified != 0 || counts.Linked != 0 {
				t.Fatal(counts)
			}
			s = repair76Reopen(t, s)
			var state, reason string
			must(t, s.DB.QueryRow(ctx, `SELECT state,reason FROM account_backfill WHERE source_key=$1`, "legacy:"+wallet).Scan(&state, &reason))
			if state != "quarantined" || reason == "" {
				t.Fatal(state, reason)
			}
			var events int
			must(t, s.DB.QueryRow(ctx, `SELECT count(*) FROM account_outbox WHERE kind='account.backfill.completed'`).Scan(&events))
			if events != 0 {
				t.Fatal("unresolved migration published completion")
			}
			if err = s.SetStableAccountReads(ctx, true); !errors.Is(err, ErrState) {
				t.Fatal("unresolved migration enabled cutover", err)
			}
		})
	}
}

func TestConnectedRepair76_backfill_provisioning_order(t *testing.T) {
	for _, identityFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("identity_first_%t", identityFirst), func(t *testing.T) {
			s := connectedLegacyFixture(t)
			ctx := context.Background()
			p := Principal{AccountID: uuid.New(), ProofAt: s.now()}
			wallet := fmt.Sprintf("0x%040x", 7604)
			session := uuid.New()
			seedConnectedIdentity(t, s, p, session, wallet)
			gateway := sharedauth.Principal{Wallet: wallet, SessionID: session.String(), Roles: []string{"owner"}}
			provision := func() {
				t.Helper()
				must(t, s.ensureIdentityAccount(ctx, sharedauth.PostgresIdentityResolver{Pool: s.DB}, gateway))
			}
			if identityFirst {
				provision()
			}
			_, err := s.DB.Exec(ctx, `INSERT INTO users.user_profiles(wallet_address,display_name,bio,created_at,updated_at) VALUES($1,'Legacy name','Legacy bio',1,1)`, wallet)
			must(t, err)
			r := BackfillRange{ID: "provisioning-order", Limit: 100}
			counts, err := s.BackfillLegacyRange(ctx, r)
			must(t, err)
			if counts.Verified != 1 || counts.Quarantined != 0 {
				t.Fatal(counts)
			}
			if !identityFirst {
				provision()
			}
			s = repair76Reopen(t, s)
			profile, err := s.Public(ctx, wallet)
			must(t, err)
			if profile.DisplayName != "Legacy name" || profile.Bio != "Legacy bio" {
				t.Fatal(profile)
			}
			replay, err := s.BackfillLegacyRange(ctx, r)
			must(t, err)
			if replay != counts {
				t.Fatal(counts, replay)
			}
			must(t, s.PatchProfile(ctx, p, p.AccountID, ProfilePatch{ExpectedRevision: profile.Revision, DisplayName: str("Later user edit")}))
			counts, err = s.BackfillLegacyRange(ctx, BackfillRange{ID: "preserve-edit", Limit: 100})
			must(t, err)
			if counts.Quarantined != 1 || counts.Verified != 0 {
				t.Fatal(counts)
			}
			s = repair76Reopen(t, s)
			profile, err = s.Public(ctx, wallet)
			must(t, err)
			if profile.DisplayName != "Later user edit" {
				t.Fatal("backfill overwrote edit", profile)
			}
			var events int
			must(t, s.DB.QueryRow(ctx, `SELECT count(*) FROM account_outbox WHERE account_id=$1 AND kind='account.backfill.completed'`, p.AccountID).Scan(&events))
			if events != 1 {
				t.Fatal("completion was duplicated or emitted for conflicting content", events)
			}
		})
	}
}

func TestConnectedRepair76_export_sdk_authorization(t *testing.T) {
	s := connectedFixture(t)
	s.Clock = func() time.Time { return time.Now().UTC() }
	ctx := context.Background()
	p := owner(t, s, 7605)
	other := owner(t, s, 7606)
	objects := internaltest.NewStore(t, "repair-76-exports")
	publisher, subjects := repair76Publisher(t, s, "account.export.requested")
	runtime := &Runtime{ExportKey: bytes.Repeat([]byte{76}, 32), Objects: objects, Metrics: &Metrics{}, Delivery: publisher, Subjects: subjects}
	core := (HTTP{Store: s, Runtime: runtime}).Handler()
	start := func() *httptest.Server {
		t.Helper()
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			core.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
		}))
		t.Cleanup(server.Close)
		return server
	}
	server := start()
	runtime.DownloadBaseURL = server.URL
	request := func(path string, headers map[string]string, status int) []byte {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, path, nil)
		must(t, err)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		response, err := server.Client().Do(req)
		must(t, err)
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		must(t, err)
		if response.StatusCode != status {
			t.Fatalf("export status %d, want %d: %s", response.StatusCode, status, raw)
		}
		if response.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatal("export response is cacheable")
		}
		return raw
	}
	job, err := s.RequestExport(ctx, p)
	must(t, err)
	must(t, (Worker{Store: s, Runtime: runtime, Owner: "repair-76-worker", LeaseDuration: time.Minute}).RunOnce(ctx))
	statusURL := server.URL + "/v2/me/exports/" + job.ID.String()
	raw := request(statusURL, nil, http.StatusOK)
	typed, err := v2.DecodeAccountExportStatusResponse(raw)
	must(t, err)
	if typed.Data.DownloadUrl == nil || typed.Data.Job.DownloadUrl == nil || *typed.Data.DownloadUrl != *typed.Data.Job.DownloadUrl {
		t.Fatal("typed SDK lost download URL", typed)
	}
	// Only the generated type supplies the URL; no undocumented JSON field is recovered.
	downloadURL, err := url.Parse(*typed.Data.DownloadUrl)
	must(t, err)
	server.Close()
	s = repair76Reopen(t, s)
	core = (HTTP{Store: s, Runtime: runtime}).Handler()
	server = start()
	runtime.DownloadBaseURL = server.URL
	requestURL := server.URL + downloadURL.RequestURI()
	request(server.URL+downloadURL.Path, nil, http.StatusForbidden)
	request(requestURL, map[string]string{"X-Export-Authorization": strings.Repeat("0", 64)}, http.StatusForbidden)
	request(requestURL+"&authorization="+strings.Repeat("1", 64), nil, http.StatusForbidden)
	stranger := httpRequest(core, other, http.MethodGet, downloadURL.RequestURI(), "")
	if stranger.Code != http.StatusForbidden {
		t.Fatalf("another account downloaded export: %d", stranger.Code)
	}
	downloaded := request(requestURL, nil, http.StatusOK)
	if !bytes.Contains(downloaded, []byte(`"preferences"`)) || bytes.Contains(downloaded, []byte(other.AccountID.String())) {
		t.Fatal("downloaded snapshot content invalid", string(downloaded))
	}
	request(requestURL, nil, http.StatusForbidden)
}

// Keep the service-bound integration proofs above intact. The repair gate below
// exercises the production decisions and wire codecs without external services.
func repair76ReopenJSON(t *testing.T, value any, recovered any) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "record.json")
	writer, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(t, err)
	must(t, json.NewEncoder(writer).Encode(value))
	must(t, writer.Sync())
	must(t, writer.Close())
	reader, err := os.Open(path)
	must(t, err)
	must(t, json.NewDecoder(reader).Decode(recovered))
	must(t, reader.Close())
}

func TestRepair_76_device_token_rotation(t *testing.T) {
	// The cap counts the exact rows the delivery authority enumerates, including
	// revoked rows and historical duplicates replaced by this installation.
	for _, tc := range []struct {
		total, replaced int
		allowed         bool
	}{
		{0, 0, true}, {31, 0, true}, {32, 0, false}, {32, 1, true},
		{40, 40, true}, {63, 32, true}, {64, 32, false},
		{-1, 0, false}, {1, -1, false}, {1, 2, false},
	} {
		err := deviceEndpointCapacity(tc.total, tc.replaced)
		if (err == nil) != tc.allowed {
			t.Fatalf("capacity total=%d replaced=%d: %v", tc.total, tc.replaced, err)
		}
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	must(t, err)
	binding := DeviceBinding{AccountID: uuid.New(), SessionID: uuid.New(), InstallationID: uuid.New(), Platform: "android", Provider: "fcm"}
	now := time.Now().UTC()
	key := bytes.Repeat([]byte{76}, 32)
	verify := CompactDeviceProofVerifier(public)
	var lastCiphertext []byte
	for i := 0; i < 40; i++ {
		binding.Token = fmt.Sprintf("installation-token-%d", i)
		proof := make([]byte, 107)
		proof[0] = 2
		copy(proof[1:17], binding.AccountID[:])
		copy(proof[17:33], binding.SessionID[:])
		copy(proof[33:49], binding.InstallationID[:])
		digest := sha256.Sum256([]byte(binding.Token))
		copy(proof[49:81], digest[:])
		_, err = rand.Read(proof[81:97])
		must(t, err)
		binary.BigEndian.PutUint64(proof[97:105], uint64(now.Add(time.Minute).Unix()))
		proof[105], proof[106] = 3, 3
		binding.Proof = base64.RawURLEncoding.EncodeToString(append(proof, ed25519.Sign(private, proof)...))
		must(t, (DeviceBody{Platform: binding.Platform, Provider: binding.Provider, Token: binding.Token, DeviceProof: binding.Proof}).Validate())
		must(t, verify(context.Background(), binding, now))
		lastCiphertext, err = Seal(key, []byte(binding.Token), []byte(binding.AccountID.String()))
		must(t, err)
		oldToken := binding.Token
		binding.Token = "stale-token"
		if err := verify(context.Background(), binding, now); !errors.Is(err, ErrDenied) {
			t.Fatal("rotation proof authorized a different token", err)
		}
		binding.Token = oldToken
	}
	var recovered []byte
	repair76ReopenJSON(t, lastCiphertext, &recovered)
	plain, err := Open(key, recovered, []byte(binding.AccountID.String()))
	must(t, err)
	if string(plain) != "installation-token-39" {
		t.Fatalf("recovered stale endpoint %q", plain)
	}
	if _, err = Open(key, recovered, []byte(uuid.NewString())); err == nil {
		t.Fatal("another account decrypted the endpoint")
	}
}

func repair76LegacyProfile(t *testing.T) (string, json.RawMessage, Profile) {
	t.Helper()
	wallet := fmt.Sprintf("0x%040x", 7602)
	raw, err := json.Marshal(map[string]any{
		"wallet_address": wallet, "display_name": "Legacy profile", "bio": "Legacy biography",
		"website": "https://example.org/profile", "twitter": "https://x.com/profile",
		"telegram": "https://t.me/profile", "discord": "https://discord.com/profile",
		"created_at": 1, "updated_at": 2,
	})
	must(t, err)
	profile, err := legacyProfileProjection(context.Background(), wallet, raw, nil)
	must(t, err)
	profile.Revision, profile.SchemaVersion = "0", 1
	return wallet, raw, profile
}

func TestRepair_76_backfill_profile_fields(t *testing.T) {
	wallet, raw, source := repair76LegacyProfile(t)
	var recovered Profile
	repair76ReopenJSON(t, source, &recovered)
	if !sameLegacyProfile(source, recovered) || !reflect.DeepEqual(source, recovered) {
		t.Fatal("profile content changed across codec reopen", source, recovered)
	}
	wantSocials := map[string]string{"twitter": "https://x.com/profile", "telegram": "https://t.me/profile", "discord": "https://discord.com/profile"}
	if !reflect.DeepEqual(recovered.Socials, wantSocials) {
		t.Fatal("social values were lost", recovered.Socials)
	}
	if recovered.Website == nil || *recovered.Website != "https://example.org/profile" || recovered.DisplayName != "Legacy profile" || recovered.Bio != "Legacy biography" {
		t.Fatal("legacy public projection was not preserved", recovered)
	}
	envelope, err := Response(uuid.New(), recovered.PublicProfile, recovered.Revision, time.Now())
	must(t, err)
	encoded, err := AdaptV2ProfilePatch(envelope)
	must(t, err)
	decoded, err := v2.DecodeProfilePatchResponse(encoded)
	must(t, err)
	if decoded.Data.Profile.Website == nil || *decoded.Data.Profile.Website != *recovered.Website || decoded.Data.Profile.DisplayName != recovered.DisplayName || decoded.Data.Profile.Bio != recovered.Bio {
		t.Fatal("generated consumer lost public content", decoded)
	}
	inventory, err := legacySourceBytes(wallet, []json.RawMessage{raw}, nil, []legacyWatch{})
	must(t, err)
	for _, field := range []string{"website", "twitter", "telegram", "discord", "bio", "display_name", "updated_at"} {
		var fields map[string]any
		must(t, json.Unmarshal(raw, &fields))
		fields[field] = "changed"
		changed, err := json.Marshal(fields)
		must(t, err)
		digestInput, err := legacySourceBytes(wallet, []json.RawMessage{changed}, nil, []legacyWatch{})
		must(t, err)
		if sha256.Sum256(inventory) == sha256.Sum256(digestInput) {
			t.Fatal("unaccounted legacy field", field)
		}
	}
	image, err := json.Marshal(map[string]any{"id": uuid.NewString(), "image_type": "avatar", "storage_key": "legacy/avatar.png"})
	must(t, err)
	withImage, err := legacySourceBytes(wallet, []json.RawMessage{raw}, []json.RawMessage{image}, []legacyWatch{})
	must(t, err)
	if sha256.Sum256(inventory) == sha256.Sum256(withImage) {
		t.Fatal("image inventory did not affect source digest")
	}
}

func TestRepair_76_backfill_unresolved_fields_and_assets(t *testing.T) {
	wallet, raw, _ := repair76LegacyProfile(t)
	for _, field := range []string{"website", "twitter", "telegram", "discord"} {
		var fields map[string]any
		must(t, json.Unmarshal(raw, &fields))
		fields[field] = "javascript:alert(1)"
		invalid, err := json.Marshal(fields)
		must(t, err)
		if _, err = legacyProfileProjection(context.Background(), wallet, invalid, nil); !errors.Is(err, ErrInvalid) {
			t.Fatal("unresolved field accepted", field, err)
		}
	}
	image := json.RawMessage(fmt.Sprintf(`{"id":%q,"image_type":"avatar","storage_key":"legacy/avatar.png"}`, uuid.NewString()))
	if _, err := legacyProfileProjection(context.Background(), wallet, raw, []json.RawMessage{image}); !errors.Is(err, ErrState) {
		t.Fatal("asset without ownership approval accepted", err)
	}
	invalidImage := json.RawMessage(`{"id":"legacy-image","image_type":"avatar"}`)
	if _, err := legacyProfileProjection(context.Background(), wallet, raw, []json.RawMessage{invalidImage}); !errors.Is(err, ErrInvalid) {
		t.Fatal("unmigrated asset identifier accepted", err)
	}
}

func TestRepair_76_backfill_provisioning_order(t *testing.T) {
	wallet, _, source := repair76LegacyProfile(t)
	for _, identityFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("identity_first_%t", identityFirst), func(t *testing.T) {
			current := source
			if identityFirst {
				current = Profile{PublicProfile: PublicProfile{Wallet: wallet, Revision: "7"}, SchemaVersion: 1}
			}
			profile, err := reconcileLegacyProfile(current, source, 7, false)
			must(t, err)
			var recovered Profile
			repair76ReopenJSON(t, profile, &recovered)
			if !sameLegacyProfile(source, recovered) {
				t.Fatal("provisioning discarded legacy content", recovered)
			}
			if identityFirst && recovered.Revision != "7" {
				t.Fatal("backfill reset stable revision", recovered.Revision)
			}
			replay, err := reconcileLegacyProfile(recovered, source, 7, false)
			must(t, err)
			if !reflect.DeepEqual(replay, recovered) {
				t.Fatal("replay changed a reconciled profile")
			}
			for _, field := range []string{"displayName", "bio", "website", "avatarAssetId", "bannerAssetId", "socials", "wallet"} {
				encoded, err := json.Marshal(recovered)
				must(t, err)
				var fields map[string]any
				must(t, json.Unmarshal(encoded, &fields))
				if field == "socials" {
					fields[field] = map[string]string{"twitter": "https://x.com/later-edit"}
				} else {
					fields[field] = "Later user edit"
				}
				encoded, err = json.Marshal(fields)
				must(t, err)
				var edited Profile
				must(t, json.Unmarshal(encoded, &edited))
				result, err := reconcileLegacyProfile(edited, source, 8, true)
				if !errors.Is(err, ErrConflict) || !reflect.DeepEqual(result, edited) {
					t.Fatal("backfill overwrote edit or failed to quarantine", field, err)
				}
			}
			blank := Profile{PublicProfile: PublicProfile{Wallet: wallet, Revision: "9"}}
			if _, err = reconcileLegacyProfile(blank, source, 9, true); !errors.Is(err, ErrConflict) {
				t.Fatal("explicitly cleared user content was overwritten", err)
			}
		})
	}
}

func TestRepair_76_export_sdk_authorization(t *testing.T) {
	at := time.Now().UTC()
	job := Job{ID: uuid.New(), AccountID: uuid.New(), Kind: "export", State: "ready", RequestedAt: at}
	key := bytes.Repeat([]byte{76}, 32)
	payload, digest, err := newExportMetadata(key, job.ID, "account-exports/"+job.ID.String()+"/1", at)
	must(t, err)
	var recovered json.RawMessage
	repair76ReopenJSON(t, json.RawMessage(payload), &recovered)
	h := HTTP{Store: Store{Clock: func() time.Time { return at }}, Runtime: &Runtime{ExportKey: key, DownloadBaseURL: "https://exports.example.org"}}
	expires := at.Add(15 * time.Minute)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveV2Owner(w, r, func(w http.ResponseWriter, r *http.Request) {
			h.respondExportStatus(w, r, job, &expires, recovered)
		}, func(raw []byte) ([]byte, error) { return AdaptV2AccountJob(raw, "export", true) })
	}))
	defer server.Close()
	response, err := server.Client().Get(server.URL + "/v2/me/exports/" + job.ID.String())
	must(t, err)
	encoded, err := io.ReadAll(response.Body)
	must(t, err)
	must(t, response.Body.Close())
	if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "private, no-store" || response.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("status transport failed or exposed cacheable credentials", response.Status, response.Header, string(encoded))
	}
	typed, err := v2.DecodeAccountExportStatusResponse(encoded)
	must(t, err)
	if typed.Data.DownloadUrl == nil || typed.Data.Job.DownloadUrl == nil || *typed.Data.DownloadUrl != *typed.Data.Job.DownloadUrl {
		t.Fatal("generated decoder lost authorized URL", typed)
	}
	request, err := http.NewRequest(http.MethodGet, *typed.Data.DownloadUrl, nil)
	must(t, err)
	token, err := exportDownloadAuthorization(request)
	must(t, err)
	if sha256.Sum256([]byte(token)) != digest || request.URL.Path != "/v2/me/exports/"+job.ID.String()+"/download" {
		t.Fatal("typed consumer could not recover the producer credential")
	}
	for _, tc := range []struct{ query, header string }{
		{"", ""}, {"authorization=" + token + "&authorization=" + token, ""},
		{"authorization=", ""}, {"authorization=%zz", ""},
		{"authorization=" + strings.Repeat("A", 64), ""},
		{"authorization=" + token, strings.Repeat("0", 64)},
		{"authorization=" + token + "&extra=1", ""},
	} {
		request.URL.RawQuery = tc.query
		request.Header.Set("X-Export-Authorization", tc.header)
		if _, err := exportDownloadAuthorization(request); !errors.Is(err, ErrDenied) {
			t.Fatal("ambiguous or missing credential accepted", tc, err)
		}
	}
	request.URL.RawQuery = ""
	request.Header.Set("X-Export-Authorization", "Bearer "+token)
	if got, err := exportDownloadAuthorization(request); err != nil || got != token {
		t.Fatal("existing header contract broke", err)
	}
	request.Header.Add("X-Export-Authorization", token)
	if _, err := exportDownloadAuthorization(request); !errors.Is(err, ErrDenied) {
		t.Fatal("duplicate header accepted", err)
	}
	wrongJob := job
	wrongJob.ID = uuid.New()
	if _, err := h.exportStatusProjection(wrongJob, &expires, recovered); err == nil {
		t.Fatal("metadata decrypted under another job")
	}
	for _, state := range []string{"requested", "failed", "expired"} {
		other := job
		other.State = state
		projection, err := h.exportStatusProjection(other, &expires, recovered)
		must(t, err)
		if projection.DownloadURL != nil || projection.DownloadAuthorization != nil {
			t.Fatal("unfinished export exposed authorization", state)
		}
	}
	projection, err := h.exportStatusProjection(job, &at, recovered)
	must(t, err)
	if projection.DownloadURL != nil || projection.DownloadAuthorization != nil {
		t.Fatal("expired export exposed authorization")
	}
	for _, base := range []string{"http://exports.example.org", "https://exports.example.org?authorization=injected", "https://exports.example.org#fragment"} {
		h.Runtime.DownloadBaseURL = base
		if _, err := h.exportStatusProjection(job, &expires, recovered); !errors.Is(err, ErrInvalid) {
			t.Fatal("unsafe download base accepted", base, err)
		}
	}
	// The actual download handler must reject missing auth before any store access.
	request = httptest.NewRequest(http.MethodGet, "/v2/me/exports/"+job.ID.String()+"/download", nil)
	request.SetPathValue("jobId", job.ID.String())
	request = request.WithContext(WithPrincipal(request.Context(), Principal{AccountID: job.AccountID}))
	w := httptest.NewRecorder()
	h.DownloadHandler().ServeHTTP(w, request)
	if w.Code != http.StatusForbidden || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("download handler accepted missing authorization or cached the response", w.Code)
	}
}
