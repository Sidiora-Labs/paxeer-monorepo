package account

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
)

func TestExtendedProfileInputBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       error
	}{
		{"null profile field", `{"expectedRevision":"0","displayName":null}`, ErrInvalid},
		{"unknown field", `{"expectedRevision":"0","unknown":true}`, ErrUnknown},
		{"invalid website", `{"expectedRevision":"0","website":"http://example.org"}`, ErrInvalid},
		{"invalid banner", `{"expectedRevision":"0","bannerAssetId":"not-an-id"}`, ErrInvalid},
		{"too many socials", `{"expectedRevision":"0","socials":{"website":"https://example.org","twitter":"https://x.com/a","telegram":"https://t.me/a","discord":"https://discord.com/a","extra":"https://example.org"}}`, ErrInvalid},
		{"unknown social", `{"expectedRevision":"0","socials":{"bluesky":"https://example.org"}}`, ErrUnknown},
		{"invalid social link", `{"expectedRevision":"0","socials":{"twitter":"javascript:alert(1)"}}`, ErrInvalid},
		{"trailing document", `{"expectedRevision":"0"}{"expectedRevision":"1"}`, ErrInvalid},
		{"invalid encoding", string([]byte{0xff}), ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var patch ExtendedProfilePatch
			if err := DecodeExtendedProfile(strings.NewReader(tc.body), &patch); !errors.Is(err, tc.want) {
				t.Fatalf("profile input error = %v, want %v", err, tc.want)
			}
		})
	}
	if err := DecodeExtendedProfile(strings.NewReader(`{}`), nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil profile destination = %v", err)
	}
}

func TestWorkerAndMetricsBoundaries(t *testing.T) {
	ctx := context.Background()
	store := fixture(t)
	stats, err := store.Stats(ctx)
	if err != nil || stats.PendingExports != 0 || stats.PendingDeletions != 0 || stats.WatchlistMax != 0 {
		t.Fatalf("empty account statistics = %+v, %v", stats, err)
	}
	for _, tc := range []struct {
		name   string
		worker Worker
		want   error
	}{
		{"missing runtime", Worker{Store: store, Owner: "worker", LeaseDuration: time.Minute}, errors.New("DEPENDENCY_UNAVAILABLE")},
		{"missing owner", Worker{Store: store, Runtime: &Runtime{}, LeaseDuration: time.Minute}, ErrInvalid},
		{"short lease", Worker{Store: store, Runtime: &Runtime{}, Owner: "worker", LeaseDuration: time.Millisecond}, ErrInvalid},
		{"long lease", Worker{Store: store, Runtime: &Runtime{}, Owner: "worker", LeaseDuration: 6 * time.Minute}, ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.worker.RunOnce(ctx)
			if !errors.Is(err, tc.want) && (err == nil || err.Error() != tc.want.Error()) {
				t.Fatalf("worker error = %v, want %v", err, tc.want)
			}
		})
	}
	metrics := &Metrics{}
	metrics.Observe("profile", "success", 2*time.Millisecond)
	metrics.Observe("profile", "success", 5*time.Millisecond)
	metrics.Observe("unknown", "success", time.Second)
	metrics.Observe("profile", "unknown", time.Second)
	got := metrics.Snapshot()
	if len(got) != 1 || got["profile.success"].Count != 2 || got["profile.success"].MaxNanos != uint64(5*time.Millisecond) || got["profile.success"].TotalNanos != uint64(7*time.Millisecond) {
		t.Fatalf("account observations = %+v", got)
	}
	if empty := (*Metrics)(nil).Snapshot(); !reflect.DeepEqual(empty, map[string]Observation{}) {
		t.Fatalf("nil metrics snapshot = %+v", empty)
	}
}

func TestBackfillRangeRejectsInvalidCursorAndPreservesEmptyResult(t *testing.T) {
	store := connectedLegacyFixture(t)
	ctx := context.Background()
	for _, input := range []BackfillRange{
		{ID: "", Limit: 1},
		{ID: "range", Limit: 0},
		{ID: "range", Limit: 501},
		{ID: strings.Repeat("a", 129), Limit: 1},
		{ID: "range", After: "invalid-wallet", Limit: 1},
	} {
		out, err := store.BackfillLegacyRange(ctx, input)
		if !errors.Is(err, ErrInvalid) || out.Scanned != 0 {
			t.Fatalf("invalid range %+v = %+v, %v", input, out, err)
		}
	}
	input := BackfillRange{ID: uuid.NewString(), Limit: 10}
	first, err := store.BackfillLegacyRange(ctx, input)
	if err != nil || first.Scanned != 0 || first.Checksum == "" {
		t.Fatalf("empty range = %+v, %v", first, err)
	}
	replay, err := store.BackfillLegacyRange(ctx, input)
	if err != nil || replay != first {
		t.Fatalf("range replay = %+v, %v; first = %+v", replay, err, first)
	}
}

func TestBackfillRangeQuarantinesInvalidLegacyWatchlist(t *testing.T) {
	store := connectedLegacyFixture(t)
	ctx := context.Background()
	p := owner(t, store, 9487)
	wallet := "0x000000000000000000000000000000000000250f"
	seedConnectedIdentity(t, store, p, uuid.New(), wallet)
	if _, err := store.DB.Exec(ctx, `INSERT INTO users.user_profiles(wallet_address,display_name,bio,created_at,updated_at) VALUES($1,'Legacy user','Bio',1,1)`, wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec(ctx, `INSERT INTO users.watchlists(wallet_address,pool_address,added_at) VALUES($1,'invalid-pool',1)`, wallet); err != nil {
		t.Fatal(err)
	}
	counts, err := store.BackfillLegacyRange(ctx, BackfillRange{ID: uuid.NewString(), Limit: 10})
	if err != nil || counts.Scanned != 1 || counts.Quarantined != 1 || counts.Verified != 0 || counts.WatchlistCount != 0 {
		t.Fatalf("invalid legacy watchlist = %+v, %v", counts, err)
	}
	var state, reason string
	if err := store.DB.QueryRow(ctx, `SELECT state,reason FROM account_backfill WHERE source_key=$1`, "legacy:"+wallet).Scan(&state, &reason); err != nil || state != "quarantined" || !strings.Contains(reason, "watchlist") {
		t.Fatalf("legacy watchlist decision = %q, %q, %v", state, reason, err)
	}
}

func TestGatewayRejectsUnboundRecoveryAndOversizedOwnerBody(t *testing.T) {
	handler := (HTTP{}).GatewayHandler(sharedauth.ServiceAssertionVerifier{}, sharedauth.PostgresIdentityResolver{}, sharedauth.OperationAuthorizer{})
	for _, tc := range []struct {
		path, body string
		header     string
		want       int
	}{
		{"/v1/me/deletion-requests/" + uuid.NewString(), "", "receipt", http.StatusForbidden},
		{"/v1/me/preferences", strings.Repeat("x", 16385), "", http.StatusUnprocessableEntity},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, strings.NewReader(tc.body))
		if tc.header != "" {
			req.Header.Set("X-Recovery-Receipt", tc.header)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != tc.want {
			t.Fatalf("gateway %s = %d %s, want %d", tc.path, response.Code, response.Body.String(), tc.want)
		}
	}
}

func TestOwnerHTTPRejectsMalformedInputs(t *testing.T) {
	store := fixture(t)
	p := owner(t, store, 9501)
	wallet := fmt.Sprintf("0x%040x", 9501)
	other := fmt.Sprintf("0x%040x", 9502)
	handler := (HTTP{Store: store, CursorKey: bytes.Repeat([]byte{7}, 32)}).Handler()
	for _, tc := range []struct {
		name, method, path, body, match string
		principal                       Principal
		want                            int
	}{
		{"invalid profile address", http.MethodGet, "/v1/profiles/invalid", "", "", Principal{}, http.StatusUnprocessableEntity},
		{"unauthenticated profile edit", http.MethodPatch, "/v1/profiles/" + wallet, `{"expectedRevision":"0"}`, "", Principal{}, http.StatusUnauthorized},
		{"invalid profile target", http.MethodPatch, "/v1/profiles/invalid", `{"expectedRevision":"0"}`, "", p, http.StatusUnprocessableEntity},
		{"foreign profile target", http.MethodPatch, "/v1/profiles/" + other, `{"expectedRevision":"0"}`, "", p, http.StatusForbidden},
		{"invalid profile document", http.MethodPatch, "/v1/profiles/" + wallet, `{"expectedRevision":null}`, "", p, http.StatusUnprocessableEntity},
		{"stale profile validator", http.MethodPatch, "/v1/profiles/" + wallet, `{"expectedRevision":"0"}`, `"other"`, p, http.StatusPreconditionFailed},
		{"unauthenticated watchlist", http.MethodGet, "/v1/me/watchlist", "", "", Principal{}, http.StatusUnauthorized},
		{"unknown watchlist parameter", http.MethodGet, "/v1/me/watchlist?other=1", "", "", p, http.StatusUnprocessableEntity},
		{"duplicate watchlist limit", http.MethodGet, "/v1/me/watchlist?limit=1&limit=2", "", "", p, http.StatusUnprocessableEntity},
		{"zero watchlist limit", http.MethodGet, "/v1/me/watchlist?limit=0", "", "", p, http.StatusUnprocessableEntity},
		{"large watchlist limit", http.MethodGet, "/v1/me/watchlist?limit=101", "", "", p, http.StatusUnprocessableEntity},
		{"malformed watchlist cursor", http.MethodGet, "/v1/me/watchlist?cursor=bad", "", "", p, http.StatusBadRequest},
		{"invalid watchlist body", http.MethodPut, "/v1/me/watchlist/" + wallet, `{}`, "", p, http.StatusUnprocessableEntity},
		{"invalid watchlist address", http.MethodPut, "/v1/me/watchlist/invalid", `{"alertEnabled":true}`, "", p, http.StatusUnprocessableEntity},
		{"delete with body", http.MethodDelete, "/v1/me/watchlist/" + wallet, `{}`, "", p, http.StatusUnprocessableEntity},
		{"unauthenticated deletion", http.MethodDelete, "/v1/me/watchlist/" + wallet, "", "", Principal{}, http.StatusUnauthorized},
		{"invalid export", http.MethodPost, "/v1/me/exports", `{"format":"xml","includeMedia":false}`, "", p, http.StatusUnprocessableEntity},
		{"media export unavailable", http.MethodPost, "/v1/me/exports", `{"format":"json","includeMedia":true}`, "", p, http.StatusNotImplemented},
		{"unknown export", http.MethodGet, "/v1/me/exports/" + uuid.NewString(), "", "", p, http.StatusNotFound},
		{"invalid export id", http.MethodGet, "/v1/me/exports/invalid", "", "", p, http.StatusUnprocessableEntity},
		{"invalid deletion id", http.MethodGet, "/v1/me/deletion-requests/invalid", "", "", p, http.StatusUnprocessableEntity},
		{"deletion requires operation authorization", http.MethodPost, "/v1/me/deletion-requests", `{}`, "", p, http.StatusServiceUnavailable},
		{"export download requires authorization", http.MethodGet, "/v2/me/exports/" + uuid.NewString() + "/download", "", "", p, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.match != "" {
				req.Header.Set("If-Match", tc.match)
			}
			if tc.principal.AccountID != uuid.Nil {
				req = req.WithContext(WithPrincipal(req.Context(), tc.principal))
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != tc.want {
				t.Fatalf("%s %s = %d %s, want %d", tc.method, tc.path, response.Code, response.Body.String(), tc.want)
			}
		})
	}
	var revision int64
	if err := store.DB.QueryRow(context.Background(), `SELECT revision FROM account_state WHERE account_id=$1`, p.AccountID).Scan(&revision); err != nil || revision != 0 {
		t.Fatalf("rejected writes changed revision to %d: %v", revision, err)
	}
}

func TestExportAuthorizationAndRecoveryReceiptBoundaries(t *testing.T) {
	token := strings.Repeat("ab", 32)
	for _, tc := range []struct {
		name, query, header string
		valid               bool
	}{
		{"query token", "authorization=" + token, "", true},
		{"matching header", "authorization=" + token, "Bearer " + token, true},
		{"header token", "", "Bearer " + token, true},
		{"conflicting credentials", "authorization=" + token, "Bearer " + strings.Repeat("cd", 32), false},
		{"unknown parameter", "other=1", "", false},
		{"invalid token", "authorization=not-hex", "", false},
		{"empty token", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/v2/me/exports?"+tc.query, nil)
			if tc.header != "" {
				r.Header.Set("X-Export-Authorization", tc.header)
			}
			got, err := exportDownloadAuthorization(r)
			if tc.valid && (err != nil || got != token) {
				t.Fatalf("export authorization = %q, %v", got, err)
			}
			if !tc.valid && !errors.Is(err, ErrDenied) {
				t.Fatalf("invalid export authorization = %q, %v", got, err)
			}
		})
	}
	store := Store{Clock: func() time.Time { return time.Unix(1760000000, 0) }}
	h := HTTP{Store: store, Runtime: &Runtime{EndpointKey: bytes.Repeat([]byte{4}, 32)}}
	job, accountID := uuid.New(), uuid.New()
	receipt, err := h.recoveryReceipt(WireAccountJob{JobID: job.String(), AccountID: accountID.String()})
	if err != nil || receipt == "" {
		t.Fatalf("recovery receipt = %q, %v", receipt, err)
	}
	r := httptest.NewRequest(http.MethodGet, "/v1/me/deletion-requests/"+job.String(), nil)
	r.SetPathValue("jobId", job.String())
	r.Header.Set("X-Recovery-Receipt", receipt)
	p, err := h.recoveryPrincipal(r)
	if err != nil || p.AccountID != accountID {
		t.Fatalf("recovery principal = %+v, %v", p, err)
	}
	r.SetPathValue("jobId", uuid.NewString())
	if _, err := h.recoveryPrincipal(r); !errors.Is(err, ErrDenied) {
		t.Fatalf("recovery receipt used for another job: %v", err)
	}
	if _, err := h.recoveryReceipt(WireAccountJob{JobID: "invalid", AccountID: accountID.String()}); err == nil {
		t.Fatal("invalid recovery job accepted")
	}
	if _, err := h.recoveryReceipt(WireAccountJob{JobID: job.String(), AccountID: "invalid"}); err == nil {
		t.Fatal("invalid recovery account accepted")
	}
}

func TestEndpointBindingRejectsInvalidProofAndKeepsState(t *testing.T) {
	store := fixture(t)
	p := owner(t, store, 9561)
	key := bytes.Repeat([]byte{6}, 32)
	for _, tc := range []struct {
		name, token, platform, proof string
		key                          []byte
		verify                       EndpointProofVerifier
		want                         error
	}{
		{"empty token", "", "web", "proof", key, func(context.Context, uuid.UUID, string, string, time.Time) error { return nil }, ErrInvalid},
		{"unknown platform", "token", "other", "proof", key, func(context.Context, uuid.UUID, string, string, time.Time) error { return nil }, ErrInvalid},
		{"short key", "token", "web", "proof", []byte("short"), func(context.Context, uuid.UUID, string, string, time.Time) error { return nil }, ErrInvalid},
		{"missing verifier", "token", "web", "proof", key, nil, errors.New("DEPENDENCY_UNAVAILABLE")},
		{"rejected proof", "token", "web", "proof", key, func(context.Context, uuid.UUID, string, string, time.Time) error { return ErrDenied }, ErrDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := store.BindEndpoint(context.Background(), p, tc.token, tc.platform, tc.proof, tc.key, tc.verify)
			if !errors.Is(err, tc.want) && (err == nil || err.Error() != tc.want.Error()) {
				t.Fatalf("endpoint binding error = %v, want %v", err, tc.want)
			}
		})
	}
	if err := store.RevokeEndpoint(context.Background(), p, "token", []byte("short")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short revocation key = %v", err)
	}
	var count int
	if err := store.DB.QueryRow(context.Background(), `SELECT count(*) FROM account_endpoint`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rejected endpoint bindings persisted %d rows: %v", count, err)
	}
	first := fingerprint(key, "token")
	second := fingerprint(key, "other")
	if !bytes.Equal(first, fingerprint(key, "token")) || bytes.Equal(first, second) || bytes.Equal(first, sha256.New().Sum(nil)) {
		t.Fatal("endpoint fingerprint is not token bound")
	}
}
