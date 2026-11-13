package account

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
)

func TestDeletionHandlerRequiresOwnerSessionAndOperation(t *testing.T) {
	s := fixture(t)
	p := owner(t, s, 9681)
	identity := sharedauth.IdentityPrincipal{AccountID: p.AccountID.String(), SessionID: uuid.NewString(), Wallet: fmt.Sprintf("0x%040x", 9681), ProofAt: p.ProofAt, AccessExpiresAt: time.Now().Add(time.Hour)}
	for _, tc := range []struct {
		name, body, key string
		runtime         *Runtime
		principal       bool
		identity        *sharedauth.IdentityPrincipal
		want            int
	}{
		{"anonymous", "", "", nil, false, nil, http.StatusUnauthorized},
		{"unavailable account operation", "", "", nil, true, nil, http.StatusServiceUnavailable},
		{"oversize body", strings.Repeat("x", 16385), "", &Runtime{Authorizer: &sharedauth.OperationAuthorizer{}}, true, nil, http.StatusUnprocessableEntity},
		{"missing identity session", "", "", &Runtime{Authorizer: &sharedauth.OperationAuthorizer{}}, true, nil, http.StatusUnauthorized},
		{"invalid identity session", "", "", &Runtime{Authorizer: &sharedauth.OperationAuthorizer{}}, true, &sharedauth.IdentityPrincipal{SessionID: "invalid"}, http.StatusForbidden},
		{"missing idempotency key", "", "", &Runtime{Authorizer: &sharedauth.OperationAuthorizer{}}, true, &identity, http.StatusUnprocessableEntity},
		{"unsigned operation", "", uuid.NewString(), &Runtime{Authorizer: &sharedauth.OperationAuthorizer{}}, true, &identity, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/v2/me/deletion-requests", strings.NewReader(tc.body))
			if tc.principal {
				r = r.WithContext(WithPrincipal(r.Context(), p))
			}
			if tc.identity != nil {
				r = r.WithContext(context.WithValue(r.Context(), identityContextKey{}, *tc.identity))
			}
			r.Header.Set("Idempotency-Key", tc.key)
			w := httptest.NewRecorder()
			(HTTP{Store: s, Runtime: tc.runtime}).deletionPost(w, r)
			if w.Code != tc.want {
				t.Fatalf("deletion request = %d %s, want %d", w.Code, w.Body.String(), tc.want)
			}
		})
	}
}
