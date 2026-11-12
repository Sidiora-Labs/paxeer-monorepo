package account

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/google/uuid"
)

func TestExtendedOwnerHandlersPersistProfileAndPreferences(t *testing.T) {
	s := connectedLegacyFixture(t)
	ctx := context.Background()
	_, err := s.DB.Exec(ctx, `CREATE TABLE indexer.indexer_canonical_creations_v1(token text NOT NULL,pool text NOT NULL,creator text NOT NULL,height bigint NOT NULL,transaction_index integer NOT NULL,log_index integer NOT NULL,block_timestamp bigint NOT NULL)`)
	must(t, err)
	p := owner(t, s, 8221)
	wallet := "0x000000000000000000000000000000000000201d"
	if _, err := s.PublicPresentation(ctx, wallet); err != nil {
		t.Fatalf("owner presentation before edit: %v", err)
	}
	if err := s.PatchExtendedProfile(ctx, p, ExtendedProfilePatch{ProfilePatch: ProfilePatch{ExpectedRevision: "0", DisplayName: str("First")}}); err != nil {
		t.Fatalf("owner profile update: %v", err)
	}
	if got, err := s.PublicPresentation(ctx, wallet); err != nil || got.DisplayName != "First" {
		t.Fatalf("owner presentation after update = %+v, %v", got, err)
	}
	ddl, err := delivery.SchemaDDL("public")
	must(t, err)
	_, err = s.DB.Exec(ctx, ddl)
	must(t, err)
	subjects := map[string]string{"profile.updated": "account.profile.updated", "preferences.updated": "account.preferences.updated"}
	policy := delivery.Policy{Subjects: map[string]bool{"account.profile.updated": true, "account.preferences.updated": true}, EventTypes: map[string]bool{"profile.updated": true, "preferences.updated": true}}
	durable, err := delivery.NewStore(s.DB, "public", policy)
	must(t, err)
	h := HTTP{Store: s, Runtime: &Runtime{ExtendedV3: true, Delivery: durable, Subjects: subjects}}
	profileHandler := h.ExtendedProfileHandler()
	request := func(handler http.Handler, method, path, body string, principal bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.SetPathValue("wallet", strings.TrimPrefix(path, "/v3/profiles/"))
		if principal {
			r = r.WithContext(WithPrincipal(r.Context(), p))
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	path := "/v3/profiles/" + wallet
	if got := request(profileHandler, http.MethodGet, path, "", true); got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("profile method = %d", got.Code)
	}
	if got := request(profileHandler, http.MethodPatch, path, `{}`, false); got.Code != http.StatusUnauthorized {
		t.Fatalf("profile without owner = %d %s", got.Code, got.Body.String())
	}
	first := request(profileHandler, http.MethodPatch, path, `{"expectedRevision":"1","displayName":"Ada","website":"https://example.org","socials":{"twitter":"https://x.com/ada"}}`, true)
	var profile struct {
		Data struct {
			Profile ProfilePresentation `json:"profile"`
		} `json:"data"`
	}
	if first.Code != http.StatusOK || json.Unmarshal(first.Body.Bytes(), &profile) != nil || profile.Data.Profile.DisplayName != "Ada" || profile.Data.Profile.Socials["twitter"] != "https://x.com/ada" || profile.Data.Profile.Revision != "2" {
		t.Fatalf("extended profile write = %d %s", first.Code, first.Body.String())
	}
	if stale := request(profileHandler, http.MethodPatch, path, `{"expectedRevision":"0","displayName":"Stale"}`, true); stale.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale profile write = %d %s", stale.Code, stale.Body.String())
	}
	if foreign := request(profileHandler, http.MethodPatch, "/v3/profiles/0x2222222222222222222222222222222222222222", `{"expectedRevision":"1","displayName":"Foreign"}`, true); foreign.Code != http.StatusForbidden {
		t.Fatalf("foreign profile write = %d %s", foreign.Code, foreign.Body.String())
	}

	preferencesHandler := h.ExtendedPreferencesHandler()
	if got := request(preferencesHandler, http.MethodGet, "/v3/me/preferences", "", true); got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("preferences method = %d", got.Code)
	}
	response := request(preferencesHandler, http.MethodPatch, "/v3/me/preferences", `{"expectedRevision":"2","timezone":"Europe/Berlin","channels":{"push":true,"inapp":false,"email":false}}`, true)
	var preferences struct {
		Data struct {
			Preferences Preferences `json:"preferences"`
		} `json:"data"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &preferences) != nil || preferences.Data.Preferences.Timezone != "Europe/Berlin" || preferences.Data.Preferences.Revision != "3" {
		t.Fatalf("extended preferences write = %d %s", response.Code, response.Body.String())
	}
}

func TestExtendedProfileRejectsUnapprovedLinksAndAssets(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 9692)
	badLink := "javascript:alert(1)"
	asset := uuid.NewString()
	for _, tc := range []struct {
		name  string
		patch ExtendedProfilePatch
		want  error
	}{
		{"website", ExtendedProfilePatch{ProfilePatch: ProfilePatch{ExpectedRevision: "0"}, Website: Optional[string]{Set: true, Value: &badLink}}, ErrInvalid},
		{"unknown social", ExtendedProfilePatch{ProfilePatch: ProfilePatch{ExpectedRevision: "0"}, Socials: map[string]Optional[string]{"other": {Set: true, Value: &badLink}}}, ErrUnknown},
		{"unsafe social", ExtendedProfilePatch{ProfilePatch: ProfilePatch{ExpectedRevision: "0"}, Socials: map[string]Optional[string]{"twitter": {Set: true, Value: &badLink}}}, ErrInvalid},
		{"missing asset authority", ExtendedProfilePatch{ProfilePatch: ProfilePatch{ExpectedRevision: "0", AvatarAssetID: Optional[string]{Set: true, Value: &asset}}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := s.PatchExtendedProfile(context.Background(), p, tc.patch)
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("profile boundary = %v, want %v", err, tc.want)
			}
		})
	}
	if err := s.PatchProfile(context.Background(), p, p.AccountID, ProfilePatch{ExpectedRevision: "0", AvatarAssetID: Optional[string]{Set: true, Value: &asset}}); err == nil || err.Error() != "DEPENDENCY_UNAVAILABLE" {
		t.Fatalf("profile avatar without asset authority = %v", err)
	}
	profile, err := s.Public(context.Background(), "0x00000000000000000000000000000000000025dc")
	if err != nil || profile.Revision != "0" || profile.AvatarAssetID != nil {
		t.Fatalf("rejected profile mutations = %+v, %v", profile, err)
	}
}

func TestDevicesReadShowsRevocationState(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 8222)
	device := uuid.New()
	fingerprint := sha256.Sum256([]byte(device.String()))
	_, err := s.DB.Exec(context.Background(), `INSERT INTO account_endpoint(fingerprint,account_id,ciphertext,platform,consent_revision,validated_at,revoked,device_id,provider)
		VALUES($1,$2,$3,'android',0,clock_timestamp(),false,$4,'fcm')`, fingerprint[:], p.AccountID, []byte("sealed-endpoint"), device)
	must(t, err)
	handler := (HTTP{Store: s}).Handler()
	get := func() DeviceDescription {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, "/v1/me/devices", nil)
		r = r.WithContext(WithPrincipal(r.Context(), p))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		var body struct {
			Data struct {
				Devices []DeviceDescription `json:"devices"`
			} `json:"data"`
		}
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &body) != nil || len(body.Data.Devices) != 1 {
			t.Fatalf("devices read = %d %s", w.Code, w.Body.String())
		}
		return body.Data.Devices[0]
	}
	if got := get(); got.DeviceID != device || got.Provider != "fcm" || got.State != "active" {
		t.Fatalf("active device = %+v", got)
	}
	_, err = s.DB.Exec(context.Background(), `UPDATE account_endpoint SET revoked=true WHERE device_id=$1`, device)
	must(t, err)
	if got := get(); got.DeviceID != device || got.State != "revoked" {
		t.Fatalf("revoked device = %+v", got)
	}
}
