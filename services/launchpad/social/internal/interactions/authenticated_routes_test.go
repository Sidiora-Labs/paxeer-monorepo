package interactions

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
)

func TestAuthenticatedSocialRoutesPreservePublicReadsAndPrivateAuthority(t *testing.T) {
	f := connected(t)
	owner := f.wallets[0]
	routes := f.http.AuthenticatedRoutes(sharedauth.HTTPIdentityAuthority{})
	public := httptest.NewRecorder()
	routes.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/users/"+owner+"/followers", nil))
	if public.Code != http.StatusOK || !strings.Contains(public.Body.String(), `"items"`) {
		t.Fatalf("public follow page = %d %s", public.Code, public.Body.String())
	}
	private := httptest.NewRecorder()
	routes.ServeHTTP(private, httptest.NewRequest(http.MethodGet, "/me/social/privacy", nil))
	if private.Code != http.StatusServiceUnavailable || !strings.Contains(private.Body.String(), "DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("private route without identity authority = %d %s", private.Code, private.Body.String())
	}
	credential := httptest.NewRequest(http.MethodGet, "/users/"+owner+"/followers", nil)
	credential.Header.Set("Authorization", "Bearer invalid")
	bound := httptest.NewRecorder()
	routes.ServeHTTP(bound, credential)
	if bound.Code != http.StatusServiceUnavailable || !strings.Contains(bound.Body.String(), "DEPENDENCY_UNAVAILABLE") {
		t.Fatalf("public page with invalid credential = %d %s", bound.Code, bound.Body.String())
	}
}

func TestCommittedFollowPagesRejectMismatchedCursorsAndBounds(t *testing.T) {
	f := connected(t)
	owner := f.wallets[0]
	for _, tc := range []struct {
		limit  int
		cursor string
	}{
		{-1, ""},
		{1, "invalid"},
		{1, strings.Repeat("x", 1025)},
	} {
		if _, err := f.store.Follows(f.ctx[owner], owner, false, tc.limit, tc.cursor); !errors.Is(err, ErrInvalid) {
			t.Fatalf("follow page bounds %+v = %v", tc, err)
		}
	}
	page, err := f.store.Follows(f.ctx[owner], owner, false, 0, "")
	if err != nil || page.Next != "" {
		t.Fatalf("default follow page = %+v: %v", page, err)
	}
	if _, err := (Store{}).Follows(context.Background(), owner, false, 1, ""); !errors.Is(err, ErrDependency) {
		t.Fatalf("unbound follow page = %v", err)
	}
}
