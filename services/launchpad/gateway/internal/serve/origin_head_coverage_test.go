package serve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/cache"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/internaltest"
	"github.com/go-chi/chi/v5"
)

func TestServeOriginMetadataAndClosedCache(t *testing.T) {
	store := internaltest.NewStore(t, "token-bucket")
	db, _ := internaltest.NewRedis(t)
	handler := New(Deps{Buckets: map[string]Reader{"token": store}, Cache: cache.New(db, time.Minute), CacheMaxSize: 1 << 20})
	if err := store.Put(context.Background(), objKey, strings.NewReader("PNGDATA-1234"), 12, "image/png"); err != nil {
		t.Fatal(err)
	}
	metadata := do(t, handler, http.MethodHead, "/media/token/"+objKey, nil)
	if metadata.Code != http.StatusOK || metadata.Body.Len() != 0 || metadata.Header().Get("Content-Length") != "12" || metadata.Header().Get("ETag") == "" {
		t.Fatalf("uncached origin metadata: status=%d headers=%v body=%s", metadata.Code, metadata.Header(), metadata.Body.String())
	}
	missing := do(t, handler, http.MethodHead, "/media/token/absent.png", nil)
	if missing.Code != http.StatusNotFound || missing.Header().Get("Cache-Control") != "public, max-age=30" {
		t.Fatalf("missing origin metadata: status=%d headers=%v body=%s", missing.Code, missing.Header(), missing.Body.String())
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	response := do(t, handler, http.MethodGet, "/media/token/"+objKey, nil)
	if response.Code != http.StatusOK || response.Body.String() != "PNGDATA-1234" || response.Header().Get("ETag") != metadata.Header().Get("ETag") {
		t.Fatalf("origin after cache shutdown: status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
}

func TestServeOriginMetadataHonorsCanceledRead(t *testing.T) {
	store := internaltest.NewStore(t, "token-bucket")
	handler := New(Deps{Buckets: map[string]Reader{"token": store}})
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodHead, "/media/token/"+objKey, nil).WithContext(ctx)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway || response.Header().Get("ETag") != "" || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("canceled origin metadata: status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
}
