package account

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestExtendedPreferencesPersistConsentAndPresentation(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 9651)
	ctx := context.Background()
	currency, interval := "EUR", "1h"
	reduced, private := true, true
	first := ExtendedPreferencesPatch{
		PreferencesPatch: PreferencesPatch{ExpectedRevision: "0"},
		Currency:         &currency, ChartInterval: &interval, ReducedMotion: &reduced, PrivateActivity: &private,
		Consents: []ConsentPatch{{Category: "trade", Channel: "push", Enabled: true}},
	}
	if err := s.PatchExtendedPreferences(ctx, p, first); err != nil {
		t.Fatal(err)
	}
	preferences, err := s.ReadPreferences(ctx, p, p.AccountID)
	if err != nil || preferences.Revision != "1" || preferences.Currency != currency || preferences.ChartInterval != interval || !preferences.ReducedMotion || !preferences.PrivateActivity {
		t.Fatalf("extended preferences = %+v, %v", preferences, err)
	}
	var enabled bool
	var revoked int64
	if err := s.DB.QueryRow(ctx, `SELECT enabled,revoke_revision FROM account_consent WHERE account_id=$1 AND category='trade' AND channel='push'`, p.AccountID).Scan(&enabled, &revoked); err != nil || !enabled || revoked != 0 {
		t.Fatalf("enabled consent = %t, %d, %v", enabled, revoked, err)
	}
	if err := s.PatchExtendedPreferences(ctx, p, ExtendedPreferencesPatch{PreferencesPatch: PreferencesPatch{ExpectedRevision: "1"}, Consents: []ConsentPatch{{Category: "trade", Channel: "push", Enabled: false}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRow(ctx, `SELECT enabled,revoke_revision FROM account_consent WHERE account_id=$1 AND category='trade' AND channel='push'`, p.AccountID).Scan(&enabled, &revoked); err != nil || enabled || revoked != 2 {
		t.Fatalf("revoked consent = %t, %d, %v", enabled, revoked, err)
	}
	var eventCount int
	if err := s.DB.QueryRow(ctx, `SELECT count(*) FROM account_outbox WHERE account_id=$1 AND kind='consent.updated'`, p.AccountID).Scan(&eventCount); err != nil || eventCount != 2 {
		t.Fatalf("consent events = %d, %v", eventCount, err)
	}
	const pool = "0x3333333333333333333333333333333333333333"
	if _, err := s.DB.Exec(ctx, `INSERT INTO account_watch(account_id,chain_id,pool,added_at,revision,alert_enabled) VALUES($1,125,$2,clock_timestamp(),2,true)`, p.AccountID, pool); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.ExportSnapshot(ctx, p)
	if err != nil {
		t.Fatalf("account export snapshot = %v", err)
	}
	var document struct {
		Watchlist []WatchEntry `json:"watchlist"`
		Consents  []struct {
			Category       string `json:"category"`
			Channel        string `json:"channel"`
			Enabled        bool   `json:"enabled"`
			RevokeRevision int64  `json:"revokeRevision"`
		} `json:"consents"`
	}
	if err := json.Unmarshal(snapshot, &document); err != nil || len(document.Consents) != 1 || document.Consents[0].Category != "trade" || document.Consents[0].Channel != "push" || document.Consents[0].Enabled || document.Consents[0].RevokeRevision != 2 {
		t.Fatalf("exported consent = %+v, %v", document, err)
	}
	if len(document.Watchlist) != 1 || document.Watchlist[0].Pool != pool || !document.Watchlist[0].AlertEnabled {
		t.Fatalf("exported watchlist = %+v", document.Watchlist)
	}
}

func TestExtendedPreferencesGetIncludesStoredConsent(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 9653)
	ctx := context.Background()
	if err := s.PatchExtendedPreferences(ctx, p, ExtendedPreferencesPatch{
		PreferencesPatch: PreferencesPatch{ExpectedRevision: "0"},
		Consents:         []ConsentPatch{{Category: "trade", Channel: "push", Enabled: true}},
	}); err != nil {
		t.Fatal(err)
	}
	h := HTTP{Store: s}
	for _, tc := range []struct {
		name      string
		principal Principal
		want      int
		consent   bool
	}{
		{"owner", p, http.StatusOK, true},
		{"unknown account", Principal{AccountID: uuid.New()}, http.StatusNotFound, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/v3/me/preferences", nil)
			r = r.WithContext(WithPrincipal(r.Context(), tc.principal))
			w := httptest.NewRecorder()
			h.extendedPreferencesGet(w, r)
			if w.Code != tc.want || tc.consent && !strings.Contains(w.Body.String(), `"category":"trade"`) {
				t.Fatalf("preferences read = %d %s", w.Code, w.Body.String())
			}
		})
	}
	w := httptest.NewRecorder()
	h.extendedPreferencesGet(w, httptest.NewRequest(http.MethodGet, "/v3/me/preferences", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous preferences read = %d %s", w.Code, w.Body.String())
	}
}

func TestExtendedPreferencesRejectInvalidOptions(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 9652)
	invalidCurrency, invalidInterval := "CAD", "2h"
	for _, tc := range []struct {
		name  string
		patch ExtendedPreferencesPatch
	}{
		{"currency", ExtendedPreferencesPatch{PreferencesPatch: PreferencesPatch{ExpectedRevision: "0"}, Currency: &invalidCurrency}},
		{"chart interval", ExtendedPreferencesPatch{PreferencesPatch: PreferencesPatch{ExpectedRevision: "0"}, ChartInterval: &invalidInterval}},
		{"consent category", ExtendedPreferencesPatch{PreferencesPatch: PreferencesPatch{ExpectedRevision: "0"}, Consents: []ConsentPatch{{Category: "unknown", Channel: "push", Enabled: true}}}},
		{"consent channel", ExtendedPreferencesPatch{PreferencesPatch: PreferencesPatch{ExpectedRevision: "0"}, Consents: []ConsentPatch{{Category: "trade", Channel: "sms", Enabled: true}}}},
		{"duplicate consent", ExtendedPreferencesPatch{PreferencesPatch: PreferencesPatch{ExpectedRevision: "0"}, Consents: []ConsentPatch{{Category: "trade", Channel: "push", Enabled: true}, {Category: "trade", Channel: "push", Enabled: false}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.PatchExtendedPreferences(context.Background(), p, tc.patch); !errors.Is(err, ErrInvalid) {
				t.Fatalf("invalid preference %s = %v", tc.name, err)
			}
		})
	}
	var revision int64
	if err := s.DB.QueryRow(context.Background(), `SELECT revision FROM account_state WHERE account_id=$1`, p.AccountID).Scan(&revision); err != nil || revision != 0 {
		t.Fatalf("rejected options changed revision to %d: %v", revision, err)
	}
}
