package account

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
)

func TestDevelopmentProfileProjectionFromLegacyRows(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	ctx := context.Background()
	wallet := "0x1111111111111111111111111111111111111111"
	missing := "0x2222222222222222222222222222222222222222"
	_, err := pool.Exec(ctx, `INSERT INTO users.user_profiles(wallet_address,display_name,bio,website,twitter,telegram,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,1760000000,1760000001)`, wallet, "  Ada  ", "builder", "example.com", "@ada", "https://t.me/ada")
	if err != nil {
		t.Fatal(err)
	}
	internaltest.SeedPool(t, pool, "0x3333333333333333333333333333333333333333", "0x4444444444444444444444444444444444444444", wallet, 1760000000)
	reads := DevelopmentProfileReads{Queries: sqlcdb.New(pool)}
	if len(reads.Routes()) != 4 {
		t.Fatalf("profile routes = %v", reads.Routes())
	}
	handler := reads.Handler()
	get := func(path, etag string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, path, nil)
		if etag != "" {
			r.Header.Set("If-None-Match", etag)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	w := get("/v2/profiles/"+wallet, "")
	var body struct {
		Data struct {
			Profile PublicProfile `json:"profile"`
		} `json:"data"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Data.Profile.DisplayName != "Ada" || body.Data.Profile.Bio != "builder" || body.Data.Profile.Revision != "1760000001" || body.Data.Profile.Website == nil || *body.Data.Profile.Website != "https://example.com" {
		t.Fatalf("legacy profile = %d %s", w.Code, w.Body.String())
	}
	if tag := w.Header().Get("ETag"); tag == "" || get("/v2/profiles/"+wallet, tag).Code != http.StatusNotModified {
		t.Fatalf("profile conditional read: %q", tag)
	}
	for path, code := range map[string]int{
		"/v2/profiles/" + missing:              http.StatusNotFound,
		"/v2/profiles/not-a-wallet":            http.StatusUnprocessableEntity,
		"/v2/profiles/" + wallet + "?expand=1": http.StatusUnprocessableEntity,
	} {
		if got := get(path, ""); got.Code != code {
			t.Fatalf("%s = %d %s, want %d", path, got.Code, got.Body.String(), code)
		}
	}
	w = get("/v2/profiles:batch?wallets="+wallet+","+missing, "")
	var batch struct {
		Data struct {
			Items []PublicProfile `json:"items"`
		} `json:"data"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &batch) != nil || len(batch.Data.Items) != 1 || batch.Data.Items[0].Wallet != wallet {
		t.Fatalf("legacy batch = %d %s", w.Code, w.Body.String())
	}
	w = get("/v3/profiles/"+wallet, "")
	var extended struct {
		Data struct {
			Profile ProfilePresentation `json:"profile"`
		} `json:"data"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &extended) != nil {
		t.Fatalf("extended profile = %d %s", w.Code, w.Body.String())
	}
	p := extended.Data.Profile
	if p.Socials["website"] != "https://example.com" || p.Socials["telegram"] != "https://t.me/ada" || p.Socials["twitter"] != "" || len(p.CreatorPools) != 1 || p.CreatorPools[0].PoolAddress != "0x3333333333333333333333333333333333333333" {
		t.Fatalf("legacy presentation = %+v", p)
	}
	if w = get("/v2/profiles:batch?wallets="+wallet+","+wallet, ""); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate batch wallet = %d %s", w.Code, w.Body.String())
	}
	if !strings.HasPrefix(get("/v1/profiles/"+wallet, "").Header().Get("Content-Type"), "application/json") {
		t.Fatal("v1 legacy profile lost JSON response")
	}
}

func TestDevelopmentProfileWritesAdvanceLegacyRevision(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	wallet := "0x5555555555555555555555555555555555555555"
	reads := DevelopmentProfileReads{Queries: sqlcdb.New(pool)}
	writes := DevelopmentProfileWrites{DB: pool, Reads: reads, Clock: func() time.Time { return time.Unix(1760000000, 0) }}
	if len(writes.Routes()) != 1 || writes.Routes()[0] != "/v3/profiles/{wallet}" {
		t.Fatalf("development write routes = %v", writes.Routes())
	}
	request := httptest.NewRequest(http.MethodPatch, "/v3/profiles/"+wallet, strings.NewReader(`{"expectedRevision":"0","displayName":"Ada"}`))
	response := httptest.NewRecorder()
	writes.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("write without identity = %d %s", response.Code, response.Body.String())
	}
	request.Header.Set("Authorization", "Bearer browser-session")
	response = httptest.NewRecorder()
	writes.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("write without identity authority = %d %s", response.Code, response.Body.String())
	}
	name, bio := "Ada", "builds markets"
	website := "https://example.org"
	first := ExtendedProfilePatch{ProfilePatch: ProfilePatch{ExpectedRevision: "0", DisplayName: &name, Bio: &bio}, Website: Optional[string]{Set: true, Value: &website}, Socials: map[string]Optional[string]{"twitter": {Set: true, Value: &website}}}
	if err := developmentLinks(first); err != nil {
		t.Fatal(err)
	}
	if err := writes.apply(context.Background(), wallet, first); err != nil {
		t.Fatal(err)
	}
	profile, err := reads.presentation(context.Background(), wallet)
	if err != nil || profile.DisplayName != name || profile.Bio != bio || profile.Socials["twitter"] != website || profile.Revision != "1760000000" {
		t.Fatalf("new legacy profile = %+v, %v", profile, err)
	}
	if err := writes.apply(context.Background(), wallet, first); err != ErrConflict {
		t.Fatalf("stale legacy profile write = %v", err)
	}
	second := ExtendedProfilePatch{ProfilePatch: ProfilePatch{ExpectedRevision: profile.Revision, DisplayName: str("Ada Lovelace")}, Website: Optional[string]{Set: true}, Socials: map[string]Optional[string]{"twitter": {Set: true}}}
	if err := writes.apply(context.Background(), wallet, second); err != nil {
		t.Fatal(err)
	}
	updated, err := reads.presentation(context.Background(), wallet)
	if err != nil || updated.DisplayName != "Ada Lovelace" || updated.Website != nil || updated.Socials["twitter"] != "" || updated.Revision != "1760000001" {
		t.Fatalf("edited legacy profile = %+v, %v", updated, err)
	}
	bad := first
	bad.Website = Optional[string]{Set: true, Value: str("https://bad_example.org")}
	if err := developmentLinks(bad); err == nil {
		t.Fatal("link that cannot be served back was accepted")
	}
}

func TestDevelopmentProfileWriteAuthenticatesAndPersists(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	wallet := "0x5555555555555555555555555555555555555555"
	const token = "development-identity-service-token-32-characters"
	identity := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v2/identity/request:authenticate" || r.Header.Get("Authorization") != "Bearer "+token {
			http.NotFound(w, r)
			return
		}
		var input sharedauth.IdentityAuthenticationRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if input.Authorization != "Bearer owner-session" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		now := time.Now().UTC()
		_ = json.NewEncoder(w).Encode(sharedauth.IdentityPrincipal{AccountID: uuid.NewString(), SessionID: uuid.NewString(), Wallet: wallet, AuthEpoch: 1, ProofAt: now.Add(-time.Minute), AccessExpiresAt: now.Add(15 * time.Minute)})
	}))
	defer identity.Close()
	reads := DevelopmentProfileReads{Queries: sqlcdb.New(pool)}
	writes := DevelopmentProfileWrites{DB: pool, Reads: reads, Identity: sharedauth.HTTPIdentityAuthority{Endpoint: identity.URL, ServiceToken: token, AllowPlainHTTP: true}, Clock: func() time.Time { return time.Unix(1760000000, 0) }}
	handler := writes.Handler()
	request := func(path, body, authorization, match string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body))
		if authorization != "" {
			r.Header.Set("Authorization", authorization)
		}
		if match != "" {
			r.Header.Set("If-Match", match)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	path := "/v3/profiles/" + wallet
	body := `{"expectedRevision":"0","displayName":"Ada","website":"https://example.org","socials":{"twitter":"https://x.com/ada"}}`
	if response := request(path, body, "Bearer invalid", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("rejected identity = %d %s", response.Code, response.Body.String())
	}
	if response := request("/v3/profiles/0x2222222222222222222222222222222222222222", body, "Bearer owner-session", ""); response.Code != http.StatusForbidden {
		t.Fatalf("foreign profile = %d %s", response.Code, response.Body.String())
	}
	response := request(path, body, "Bearer owner-session", `"0"`)
	if response.Code != http.StatusOK || response.Header().Get("ETag") == "" {
		t.Fatalf("profile write = %d %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Revision string `json:"revision"`
		Data     struct {
			Profile ProfilePresentation `json:"profile"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil || envelope.Data.Profile.DisplayName != "Ada" || envelope.Data.Profile.Socials["twitter"] != "https://x.com/ada" || envelope.Revision == "0" {
		t.Fatalf("written profile = %+v, %v", envelope, err)
	}
	if response := request(path, body, "Bearer owner-session", `"0"`); response.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale profile write = %d %s", response.Code, response.Body.String())
	}
	if response := request(path, `{"expectedRevision":"`+envelope.Revision+`","displayName":"Updated"}`, "Bearer owner-session", `"0"`); response.Code != http.StatusPreconditionFailed {
		t.Fatalf("mismatched If-Match = %d %s", response.Code, response.Body.String())
	}
	if response := request(path, `{"expectedRevision":"`+envelope.Revision+`","website":"http://example.org"}`, "Bearer owner-session", ""); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("insecure profile link = %d %s", response.Code, response.Body.String())
	}
	if response := request(path, `{"expectedRevision":"`+envelope.Revision+`","avatarAssetId":"`+uuid.NewString()+`"}`, "Bearer owner-session", ""); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unapproved development avatar = %d %s", response.Code, response.Body.String())
	}
	for _, tc := range []struct {
		name, body string
	}{
		{"oversized body", strings.Repeat("x", 16385)},
		{"invalid JSON", `{"expectedRevision":`},
		{"unknown field", `{"expectedRevision":"` + envelope.Revision + `","unknown":true}`},
		{"invalid social", `{"expectedRevision":"` + envelope.Revision + `","socials":{"twitter":"http://example.org"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if response := request(path, tc.body, "Bearer owner-session", ""); response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("invalid profile document = %d %s", response.Code, response.Body.String())
			}
		})
	}
	updated, err := reads.presentation(context.Background(), wallet)
	if err != nil || updated.DisplayName != "Ada" || updated.Revision != envelope.Revision {
		t.Fatalf("persisted profile = %+v, %v", updated, err)
	}
	identity.Close()
	if response := request(path, `{"expectedRevision":"`+envelope.Revision+`","displayName":"Later"}`, "Bearer owner-session", ""); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable identity authority = %d %s", response.Code, response.Body.String())
	}
}
