package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	v1 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
	"github.com/go-chi/chi/v5"
)

func TestRepairM_933_SnapshotQueryContract(t *testing.T) {
	t.Run("bounds_and_canonical_categories", TestSnapshotQueryBoundsAndCanonicalCategories)
	t.Run("actual_dependencies_required", TestSnapshotRoutesRequireActualDependencies)
	t.Run("frozen_contract_categories", TestSnapshotRoutesAcceptEveryFrozenRankingCategory)
}

func TestRepairM_933_FrozenCategoryRouting(t *testing.T) {
	router := chi.NewRouter()
	RegisterSnapshots(router, &Snapshots{})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	for _, category := range []string{"trending", "breakout", "volume", "gainers", "losers", "movers", "unusual", "new", "top_volume"} {
		for _, route := range []struct {
			path string
			code string
		}{
			{"/rankings/" + category + "?limit=0", "INVALID_PAGINATION"},
			{"/rankings/runs/not-a-uuid/" + category, "INVALID_SNAPSHOT"},
		} {
			t.Run(category+"/"+route.code, func(t *testing.T) {
				response, err := server.Client().Get(server.URL + route.path)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				var body v1.Error
				if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if response.StatusCode != http.StatusBadRequest || body.Code != route.code || body.RequestId == "" || body.Retryable || body.FieldErrors == nil {
					t.Fatalf("category must reach subsequent route validation: status=%d body=%+v; want 400 %s", response.StatusCode, body, route.code)
				}
			})
		}
	}
}

func TestRepairM_933_UnknownCategoryGuidance(t *testing.T) {
	frozen := []string{"trending", "breakout", "volume", "gainers", "losers", "movers", "unusual", "new"}
	router := chi.NewRouter()
	RegisterSnapshots(router, &Snapshots{})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	for _, category := range []string{"arbitrary", "unsupported", "Gainers"} {
		for _, path := range []string{
			"/rankings/" + category + "?limit=0",
			"/rankings/runs/not-a-uuid/" + category,
		} {
			t.Run(path, func(t *testing.T) {
				response, err := server.Client().Get(server.URL + path)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				var body struct {
					v1.Error
					Supported []string `json:"supported"`
				}
				if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if response.StatusCode != http.StatusBadRequest || body.Code != "INVALID_CATEGORY" || body.RequestId == "" || body.Retryable || body.FieldErrors == nil {
					t.Fatalf("invalid category envelope: status=%d body=%+v", response.StatusCode, body)
				}
				if !slices.Equal(body.Supported, frozen) {
					t.Fatalf("supported=%v; want every frozen category %v", body.Supported, frozen)
				}
			})
		}
	}
}
