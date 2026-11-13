package serve

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/internaltest"
	mediastore "github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConnectedApprovedRoutesFailClosedOnMissingAuthority(t *testing.T) {
	if _, err := NewConnectedApproved(Deps{}, nil); !errors.Is(err, mediastore.ErrUnavailable) {
		t.Fatalf("missing authority: %v", err)
	}
	dsn := os.Getenv("L41_POSTGRES_URL")
	if dsn == "" {
		t.Fatal("L41_POSTGRES_URL is required for the gateway authority fixture")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	authority, err := mediastore.NewAuthority(pool, "gateway", "metadata", "chat")
	if err != nil {
		t.Fatal(err)
	}
	bucket := "gateway-connected-route"
	reader := internaltest.NewStore(t, bucket)
	if _, err := NewConnectedApproved(Deps{Buckets: map[string]Reader{bucket: nil}}, authority); !errors.Is(err, mediastore.ErrUnavailable) {
		t.Fatalf("missing object reader: %v", err)
	}
	approved, err := NewConnectedApproved(Deps{Buckets: map[string]Reader{bucket: reader}}, authority)
	if err != nil || !approved.connected {
		t.Fatalf("connected approved handler: handler=%v err=%v", approved, err)
	}
	router := chi.NewRouter()
	if err := approved.RegisterConnectedRoutes(router, "unknown"); !errors.Is(err, mediastore.ErrUnavailable) {
		t.Fatalf("unbound bucket route: %v", err)
	}
	if err := approved.RegisterConnectedRoutes(router, bucket); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, path := range []string{"/approved/invalid/256/file.png", "/approved/" + strings.Repeat("a", 64) + "/256/" + strings.Repeat("b", 64) + ".png"} {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(method, path, nil))
			if response.Code != http.StatusNotFound || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("CDN-Cache-Control") != "no-store" {
				t.Fatalf("connected media denial: method=%s path=%s status=%d headers=%v", method, path, response.Code, response.Header())
			}
		}
	}
}
