package rest

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestDevelopmentSearchNormalizesAndBoundsQueries(t *testing.T) {
	for _, check := range []struct {
		input, normalized, match string
		valid                    bool
	}{
		{"  Alpha  ", "alpha", "text", true},
		{"A\u200eB", "ab", "text", true},
		{"0xABCD", "0xabcd", "partial_address", true},
		{"0x" + strings.Repeat("A", 40), "0x" + strings.Repeat("a", 40), "exact_address", true},
		{"x", "", "", false},
		{"0x", "", "", false},
		{"0xzz", "", "", false},
		{strings.Repeat("a", 1025), "", "", false},
	} {
		got, match, err := normalizeDevelopmentQuery(check.input)
		if got != check.normalized || match != check.match || (err == nil) != check.valid {
			t.Fatalf("query %q: %q %q %v", check.input, got, match, err)
		}
	}
	for _, check := range []struct {
		query  string
		status int
	}{
		{"q=alpha&type=pool&limit=03", 422},
		{"q=alpha&type=pool&limit=bad", 422},
		{"q=alpha&type=pool&unexpected=1", 400},
		{"q=alpha&type=pool&filter=risk", 501},
		{"q=alpha&type=pool&sort=score", 422},
		{"q=alpha&type=pool&cursor=invalid", 400},
		{"q=%ZZ&type=pool", 400},
		{"q=" + strings.Repeat("a", 8200), 413},
	} {
		request := httptest.NewRequest(http.MethodGet, "/v2/search/expanded?"+check.query, nil)
		_, err := parseDevelopmentSearch(request, true)
		var failure devSearchError
		if !errors.As(err, &failure) || failure.status != check.status || failure.Error() == "" {
			t.Fatalf("query status %d for %q: %v", check.status, check.query, err)
		}
	}
	query := url.Values{"q": {"Alpha"}, "type": {"pool"}}.Encode()
	request := httptest.NewRequest(http.MethodGet, "/v2/search?"+query, nil)
	parsed, err := parseDevelopmentSearch(request, false)
	if err != nil || parsed.key != "alpha" || parsed.match != "text" || parsed.kind != "pool" || parsed.binding() == "" {
		t.Fatalf("normalized search request: %+v %v", parsed, err)
	}
}

func TestDevelopmentSearchLabelsUseIndexedIdentity(t *testing.T) {
	const address = "0x1111111111111111111111111111111111111111"
	for _, check := range []struct {
		hit  store.DevelopmentSearchHit
		kind string
		want string
	}{
		{store.DevelopmentSearchHit{Name: "Named pool", Pool: address}, "pool", "Named pool"},
		{store.DevelopmentSearchHit{Symbol: "ABC", Token: address}, "token", "ABC"},
		{store.DevelopmentSearchHit{Token: address}, "token", address},
		{store.DevelopmentSearchHit{Pool: address}, "pool", address},
		{store.DevelopmentSearchHit{Display: "Alice", Wallet: address}, "profile", "Alice"},
		{store.DevelopmentSearchHit{Wallet: address}, "profile", address},
	} {
		if label := devSearchLabel(check.hit, check.kind); label != check.want {
			t.Fatalf("%s label: %q, want %q", check.kind, label, check.want)
		}
	}
	if got := truncateRunes(strings.Repeat("é", 110), 100); len([]rune(got)) != 100 {
		t.Fatalf("long label retained %d runes", len([]rune(got)))
	}
	for _, check := range []struct {
		cause  error
		status int
		code   string
	}{
		{searchFail(422, literalFIELDINVALID), 422, literalFIELDINVALID},
		{store.ErrDataIntegrity, 503, literalINTEGRITYQUARANTINED},
		{errors.New("index unavailable"), 503, literalDEPENDENCYUNAVAILABLE},
	} {
		response := httptest.NewRecorder()
		writeSearchFailure(response, httptest.NewRequest(http.MethodGet, "/v2/search", nil), check.cause)
		if response.Code != check.status || !strings.Contains(response.Body.String(), `"code":"`+check.code+`"`) {
			t.Fatalf("search failure: %d %s", response.Code, response.Body.String())
		}
	}
}
