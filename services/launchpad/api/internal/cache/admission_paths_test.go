package cache

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
)

func TestWriteAdmissionRejectsMissingScopeAndUnavailableBudget(t *testing.T) {
	runtime := repairM992Runtime(t, repairM992Config(t, 0))
	adapter := runtime.adapter
	const route = "account_update"
	adapter.policies[route] = RoutePolicy{ID: route, Version: "v2", Origin: "postgres", Generation: "g1", Class: AccountWrite, RequestLimit: 10, RequestWindow: time.Hour}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	for _, tc := range []struct {
		name    string
		resolve ScopeResolver
		want    int
	}{
		{"missing resolver", nil, 503},
		{"unregistered route", func(*http.Request) (VerifiedScope, error) { return VerifiedScope{}, nil }, 503},
		{"denied identity", func(*http.Request) (VerifiedScope, error) { return VerifiedScope{}, sharedauth.ErrScopeDenied }, 403},
		{"missing revision", func(*http.Request) (VerifiedScope, error) { return VerifiedScope{Principal: "account"}, nil }, 403},
		{"closed budget", func(*http.Request) (VerifiedScope, error) {
			return VerifiedScope{Principal: "account", AuthorizationRevision: "1", NetworkID: "127.0.0.1"}, nil
		}, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := route
			if tc.name == "unregistered route" {
				id = "unknown"
			}
			response := httptest.NewRecorder()
			adapter.AdmissionMiddleware(id, tc.resolve, next).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/account", nil))
			if response.Code != tc.want || response.Header().Get("Cache-Control") != "private, no-store" || called {
				t.Fatalf("%s admission: status=%d header=%v called=%t", tc.name, response.Code, response.Header(), called)
			}
		})
	}
}

func TestAccountingMarketRequiresDurablePosition(t *testing.T) {
	runtime := repairM992Runtime(t, repairM992Config(t, 0))
	if err := runtime.bindings.ConsumeMarket(t.Context(), nil); err == nil {
		t.Fatal("missing durable message accepted")
	}
	if err := runtime.bindings.ConsumeMarket(t.Context(), &natsdriver.Message{}); !errors.Is(err, delivery.ErrIntegrity) {
		t.Fatalf("message without broker position: %v", err)
	}
}
