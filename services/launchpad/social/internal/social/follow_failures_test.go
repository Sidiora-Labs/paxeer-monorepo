package social

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/go-chi/chi/v5"
)

func TestFollowRoutesValidateAddressesAndReportStoreAvailability(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	h := New(Deps{Queries: sqlcdb.New(pool)})
	router := chi.NewRouter()
	h.RegisterRoutes(router)
	actor := "0x00000000000000000000000000000000000000aa"
	target := "0x00000000000000000000000000000000000000bb"
	request := func(method, path, wallet string, want int) {
		t.Helper()
		r := httptest.NewRequest(method, path, nil)
		if wallet != "" {
			r.Header.Set("X-Actor-Wallet", wallet)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s = %d, want %d: %s", method, path, w.Code, want, w.Body.String())
		}
	}
	request(http.MethodPut, "/follow/invalid", actor, http.StatusBadRequest)
	request(http.MethodDelete, "/follow/"+target, "", http.StatusUnauthorized)
	request(http.MethodDelete, "/follow/invalid", actor, http.StatusBadRequest)
	for _, route := range []string{"followers", "following", "follow-stats"} {
		request(http.MethodGet, "/users/invalid/"+route, actor, http.StatusBadRequest)
	}
	request(http.MethodPut, "/follow/"+target, actor, http.StatusOK)
	request(http.MethodGet, "/users/"+target+"/follow-stats", actor, http.StatusOK)
	pool.Close()
	request(http.MethodPut, "/follow/"+target, actor, http.StatusInternalServerError)
	request(http.MethodDelete, "/follow/"+target, actor, http.StatusInternalServerError)
	request(http.MethodGet, "/users/"+target+"/followers", actor, http.StatusInternalServerError)
	request(http.MethodGet, "/users/"+target+"/following", actor, http.StatusInternalServerError)
	request(http.MethodGet, "/users/"+target+"/follow-stats", actor, http.StatusInternalServerError)
}
