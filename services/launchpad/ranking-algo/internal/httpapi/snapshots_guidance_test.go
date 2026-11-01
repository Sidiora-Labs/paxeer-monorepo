package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	v1 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
	"github.com/go-chi/chi/v5"
)

func TestSnapshotInvalidCategoryGuidanceListsFrozenCategories(t *testing.T) {
	frozen := []string{"trending", "breakout", "volume", "gainers", "losers", "movers", "unusual", "new"}
	if !slices.Equal(ranker.Categories[:], frozen) {
		t.Fatalf("ranker categories = %v, want frozen IF_RANKINGS enum %v", ranker.Categories, frozen)
	}
	for _, category := range frozen {
		if got, err := snapshotCategory(category); err != nil || got != category {
			t.Fatalf("snapshotCategory(%q) = %q, %v", category, got, err)
		}
	}
	if got, err := snapshotCategory("top_volume"); err != nil || got != "volume" {
		t.Fatalf("snapshotCategory(top_volume) = %q, %v; want volume", got, err)
	}

	router := chi.NewRouter()
	RegisterSnapshots(router, &Snapshots{})
	for _, path := range []string{
		"/rankings/bogus",
		"/rankings/runs/00000000-0000-4000-8000-000000000001/bogus",
	} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", response.Code, response.Body.String())
			}
			var body struct {
				v1.Error
				Supported []string `json:"supported"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode invalid-category response: %v", err)
			}
			if body.Code != "INVALID_CATEGORY" || body.RequestId == "" || body.Retryable {
				t.Fatalf("invalid error envelope: %+v", body.Error)
			}
			if !slices.Equal(body.Supported, frozen) {
				t.Fatalf("supported = %v, want frozen IF_RANKINGS enum %v", body.Supported, frozen)
			}
		})
	}
}
