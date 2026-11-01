package discovery

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

func TestPostgresV2AlertCreateAndDelete(t *testing.T) {
	s, id := realStore(t)
	owner, other := id+"owner", id+"other"
	const wallet = "0x1234567890abcdef1234567890abcdef12345678"
	h := HTTP{
		Store: s,
		Authenticate: func(r *http.Request) (string, error) {
			return r.Header.Get("X-Test-Account"), nil
		},
		Wallet: func(*http.Request) (string, error) { return wallet, nil },
	}
	t.Cleanup(func() {
		if _, err := s.DB.Exec(`DELETE FROM narrative_discovery.rules WHERE account_id IN ($1,$2)`, owner, other); err != nil {
			t.Error(err)
		}
	})
	request := func(server HTTP, actor, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("X-Test-Account", actor)
		r.Header.Set("X-Request-Id", discoveryErrorRequestID)
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, r)
		return w
	}
	marshal := func(value any) string {
		t.Helper()
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	assertCount := func(active bool, want int) {
		t.Helper()
		query := `SELECT count(*) FROM narrative_discovery.rules WHERE account_id IN ($1,$2)`
		if active {
			query += ` AND document->>'Disabled'='false'`
		}
		var count int
		if err := s.DB.QueryRow(query, owner, other).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != want {
			t.Fatalf("rule count (active=%t) = %d, want %d", active, count, want)
		}
	}
	assertStored := func(want AlertRule) {
		t.Helper()
		var body []byte
		var account string
		if err := s.DB.QueryRow(`SELECT account_id,document FROM narrative_discovery.rules WHERE id=$1`, want.ID).Scan(&account, &body); err != nil {
			t.Fatal(err)
		}
		var got AlertRule
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if account != owner || !reflect.DeepEqual(got, want) {
			t.Fatalf("stored account = %q, rule = %+v, want %+v", account, got, want)
		}
	}
	chain, asset, currency, provider := "125", pool, "USD", "market-provider"
	decimals := v2.SafeInteger(18)
	input := v2.AlertCreateBody{
		PoolAddress: pool, Direction: "crosses_above", CooldownSeconds: 300,
		Threshold: v2.InputPrice{
			Numerator: "900719925474099312345678901234567890", Denominator: "1000000000000000001",
			BaseAsset:  v2.InputAssetRef{Kind: "erc20", ChainId: &chain, Address: &asset, Decimals: &decimals},
			QuoteAsset: v2.InputAssetRef{Kind: "fiat_display", CurrencyCode: &currency},
			Source:     v2.InputPriceSource{Kind: "external_display", AmountBasis: "provider_display", ProviderId: &provider, ObservedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000Z")},
		},
	}
	valid := marshal(input)
	if err := v2.ValidateJSON("AlertCreateBody", []byte(valid)); err != nil {
		t.Fatalf("invalid frozen input: %v", err)
	}
	routes := []struct{ method, path string }{
		{http.MethodPost, "/v2/me/alerts"},
		{http.MethodDelete, "/v2/me/alerts/" + uuid.NewString()},
	}
	for _, route := range routes {
		for _, server := range []HTTP{h, {Store: s}, {Store: s, Authenticate: h.Authenticate}} {
			assertDiscoveryFrozenError(t, request(server, "", route.method, route.path, valid), http.StatusUnauthorized, "AUTH_REQUIRED", discoveryErrorRequestID)
		}
		noWallet := h
		noWallet.Wallet = nil
		assertDiscoveryFrozenError(t, request(noWallet, owner, route.method, route.path, valid), http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", discoveryErrorRequestID)
		noStore := h
		noStore.Store = Store{}
		assertDiscoveryFrozenError(t, request(noStore, owner, route.method, route.path, valid), http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", discoveryErrorRequestID)
		badAuth := h
		badAuth.Authenticate = func(*http.Request) (string, error) { return owner, ErrForbidden }
		assertDiscoveryFrozenError(t, request(badAuth, owner, route.method, route.path, valid), http.StatusUnauthorized, "AUTH_REQUIRED", discoveryErrorRequestID)
	}
	for _, resolver := range []func(*http.Request) (string, error){
		func(*http.Request) (string, error) { return "", nil },
		func(*http.Request) (string, error) { return "not-a-wallet", nil },
		func(*http.Request) (string, error) { return wallet, errors.New("wallet authority unavailable") },
	} {
		server := h
		server.Wallet = resolver
		assertDiscoveryFrozenError(t, request(server, owner, http.MethodPost, "/v2/me/alerts", valid), http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", discoveryErrorRequestID)
	}
	for _, body := range []string{
		"", "null", "[]", "{}", `{"poolAddress":`, valid + " {}",
		strings.TrimSuffix(valid, "}") + `,"wallet":"` + wallet + `"}`,
		strings.TrimSuffix(valid, "}") + `,"ownerAccountId":"` + other + `"}`,
		strings.Replace(valid, `"numerator":`, `"extra":true,"numerator":`, 1),
		strings.Replace(valid, `"denominator":"1000000000000000001"`, `"denominator":"0"`, 1),
		strings.Replace(valid, `"cooldownSeconds":300`, `"cooldownSeconds":59`, 1),
		strings.Replace(valid, `"cooldownSeconds":300`, `"cooldownSeconds":null`, 1),
		strings.Replace(valid, `"poolAddress"`, `"PoolAddress"`, 1),
		strings.Replace(valid, "crosses_above", "above", 1),
		`{"padding":"` + strings.Repeat("x", 16384) + `"}`,
	} {
		assertDiscoveryFrozenError(t, request(h, owner, http.MethodPost, "/v2/me/alerts", body), http.StatusBadRequest, "INVALID_REQUEST", discoveryErrorRequestID)
	}
	assertDiscoveryFrozenError(t, request(h, owner, http.MethodDelete, "/v2/me/alerts/not-a-uuid", ""), http.StatusBadRequest, "INVALID_REQUEST", discoveryErrorRequestID)
	assertDiscoveryFrozenError(t, request(h, owner, routes[1].method, routes[1].path, ""), http.StatusNotFound, "NOT_FOUND", discoveryErrorRequestID)
	assertCount(false, 0)
	ids := make(map[string]bool)
	for i, direction := range []string{"crosses_above", "crosses_below"} {
		input.Direction = direction
		before := time.Now().UTC().Truncate(time.Millisecond)
		w := request(h, owner, http.MethodPost, "/v2/me/alerts", marshal(input))
		if w.Code != http.StatusCreated {
			t.Fatalf("create status = %d: %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "application/json" || w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("create headers: %v", w.Header())
		}
		if err := v2.ValidateJSON("AlertCreateResponse", w.Body.Bytes()); err != nil {
			t.Fatalf("invalid frozen create response: %v: %s", err, w.Body.String())
		}
		var response v2.AlertCreateResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		alert := response.Data.Alert
		alertID, err := uuid.Parse(alert.AlertId)
		if err != nil || alertID.Version() != 4 || alertID.Variant() != uuid.RFC4122 || ids[alert.AlertId] {
			t.Fatalf("expected fresh RFC 4122 UUID v4: %q (%v)", alert.AlertId, err)
		}
		ids[alert.AlertId] = true
		if alert.Wallet != wallet || alert.PoolAddress != pool || alert.Direction != direction || alert.CooldownSeconds != input.CooldownSeconds || alert.State != "enabled" || marshal(alert.Threshold) != marshal(input.Threshold) {
			t.Fatalf("create lost owner wallet or input: %+v", alert)
		}
		generatedAt, err := time.Parse("2006-01-02T15:04:05.000Z", response.GeneratedAt)
		if err != nil || generatedAt.Before(before) || generatedAt.After(time.Now()) || response.RequestId != discoveryErrorRequestID || response.Quality != "complete" || response.Revision == nil || *response.Revision != "1" || response.Watermark != nil || response.NextCursor != nil {
			t.Fatalf("invalid create envelope: %+v (%v)", response, err)
		}
		want := AlertRule{
			ID: alert.AlertId, OwnerAccountID: owner, PoolAddress: pool, Metric: "price", Direction: direction,
			Threshold:  string(input.Threshold.Numerator) + "/" + string(input.Threshold.Denominator),
			Hysteresis: "0", Cooldown: 300 * time.Second, Channels: []string{"inapp"}, Revision: 1,
		}
		assertStored(want)
		assertCount(false, i+1)
		assertCount(true, 1)
		path := "/v2/me/alerts/" + alert.AlertId
		assertDiscoveryFrozenError(t, request(h, other, http.MethodDelete, path, ""), http.StatusNotFound, "NOT_FOUND", discoveryErrorRequestID)
		assertDiscoveryFrozenError(t, request(h, "", http.MethodDelete, path, ""), http.StatusUnauthorized, "AUTH_REQUIRED", discoveryErrorRequestID)
		noWallet := h
		noWallet.Wallet = nil
		assertDiscoveryFrozenError(t, request(noWallet, owner, http.MethodDelete, path, ""), http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", discoveryErrorRequestID)
		assertStored(want)
		w = request(h, owner, http.MethodDelete, path, "")
		if w.Code != http.StatusNoContent || w.Body.Len() != 0 || w.Header().Get("Content-Type") != "" || w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("delete must return an empty 204: %d %v %q", w.Code, w.Header(), w.Body.String())
		}
		want.Disabled, want.Revision = true, 2
		assertStored(want)
		assertCount(true, 0)
		assertDiscoveryFrozenError(t, request(h, owner, http.MethodDelete, path, ""), http.StatusNotFound, "NOT_FOUND", discoveryErrorRequestID)
		assertStored(want)
	}
}
