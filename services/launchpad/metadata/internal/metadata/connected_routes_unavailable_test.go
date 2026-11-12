package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	raster "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestConnectedRoutesReportUnavailableDependencies(t *testing.T) {
	ctx := context.Background()
	wallet := "0x" + strings.Repeat("a", 40)
	assetID := uuid.NewString()
	bare := ConnectedMedia{}
	if err := bare.Ready(ctx); err != raster.ErrUnavailable {
		t.Fatalf("unconfigured media readiness = %v", err)
	}
	if err := bare.syncModeration(ctx, wallet); err != raster.ErrUnavailable {
		t.Fatalf("unconfigured token visibility = %v", err)
	}
	if err := bare.syncAccountModeration(ctx, assetID); err != raster.ErrUnavailable {
		t.Fatalf("unconfigured account visibility = %v", err)
	}
	if err := bare.ProcessAccountAsset(ctx, assetID); err != raster.ErrUnavailable {
		t.Fatalf("unconfigured account processing = %v", err)
	}
	if err := bare.MaintainAccountAssets(ctx, false); err != raster.ErrUnavailable {
		t.Fatalf("unconfigured account maintenance = %v", err)
	}
	if err := bare.ExpireRejected(ctx); err != raster.ErrUnavailable {
		t.Fatalf("unconfigured asset retention = %v", err)
	}
	bareRouter := chi.NewRouter()
	bare.RegisterRoutes(bareRouter)
	bare.RegisterAccountAuthority(bareRouter)
	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodGet, "/v2/tokens/" + wallet + "/metadata"},
		{http.MethodPatch, "/v2/tokens/" + wallet + "/metadata"},
		{http.MethodGet, "/v2/tokens/" + wallet + "/media"},
		{http.MethodPut, "/v2/tokens/" + wallet + "/assets/" + assetID},
		{http.MethodGet, "/v2/tokens/" + wallet + "/assets/" + assetID},
		{http.MethodPut, "/v2/accounts/" + wallet + "/assets/" + assetID},
		{http.MethodGet, "/v2/accounts/" + wallet + "/assets/" + assetID},
		{http.MethodGet, "/internal/v2/accounts/" + wallet + "/assets/" + assetID},
		{http.MethodDelete, "/internal/v2/accounts/" + wallet + "/assets?deletionId=" + uuid.NewString()},
	} {
		recorder := httptest.NewRecorder()
		bareRouter.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s = %d %s", tc.method, tc.path, recorder.Code, recorder.Body.String())
		}
	}
	pool := edgePool(t, "")
	schema := edgeSchema(t, pool, "route_unavailable")
	store, err := NewProductionStore(pool, schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, store.SchemaDDL()+store.V2SchemaDDL()+store.ConnectedSchemaDDL()+store.AccountAssetSchemaDDL()); err != nil {
		t.Fatal(err)
	}
	configured := ConnectedMedia{Metadata: V2ContractHandlers{Store: store}, Moderation: DevelopmentVisibility{}, Worker: raster.InProcessWorker{}}
	if err := configured.ProcessAccountAsset(ctx, "invalid"); err != raster.ErrUnavailable {
		t.Fatalf("invalid asset processing = %v", err)
	}
	configuredRouter := chi.NewRouter()
	configured.RegisterRoutes(configuredRouter)
	configured.RegisterAccountAuthority(configuredRouter)
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/internal/v2/accounts/" + wallet + "/assets/invalid", http.StatusUnprocessableEntity},
		{"/internal/v2/accounts/" + wallet + "/assets/" + assetID, http.StatusNotFound},
		{"/v2/tokens/" + wallet + "/media", http.StatusServiceUnavailable},
	} {
		recorder := httptest.NewRecorder()
		configuredRouter.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if recorder.Code != tc.want {
			t.Fatalf("GET %s = %d %s, want %d", tc.path, recorder.Code, recorder.Body.String(), tc.want)
		}
	}
	for _, tc := range []struct {
		name, method, path, body, contentType string
		handler                               V2Handlers
		want                                  int
	}{
		{"unconfigured read", http.MethodGet, "/v2/tokens/" + wallet + "/metadata", "", "", V2Handlers{}, http.StatusServiceUnavailable},
		{"unconfigured patch", http.MethodPatch, "/v2/tokens/" + wallet + "/metadata", "", "", V2Handlers{}, http.StatusServiceUnavailable},
		{"invalid token read", http.MethodGet, "/v2/tokens/invalid/metadata", "", "", V2Handlers{Store: store}, http.StatusUnprocessableEntity},
		{"unsupported patch type", http.MethodPatch, "/v2/tokens/" + wallet + "/metadata", `{}`, "text/plain", V2Handlers{Store: store}, http.StatusUnsupportedMediaType},
		{"invalid patch body", http.MethodPatch, "/v2/tokens/" + wallet + "/metadata", `{}`, "application/json", V2Handlers{Store: store}, http.StatusUnprocessableEntity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := chi.NewRouter()
			tc.handler.RegisterRoutes(router)
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.contentType != "" {
				request.Header.Set("Content-Type", tc.contentType)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != tc.want {
				t.Fatalf("%s %s = %d %s, want %d", tc.method, tc.path, recorder.Code, recorder.Body.String(), tc.want)
			}
		})
	}
	configured.Metadata.Canonical = PostgresCanonicalSource{Schema: "unavailable_canonical", CanonicalStreamID: "paxeer125-canonical"}
	client, objects := edgeObjects(t, "metadata-unavailable-object-store")
	configured.Objects = S3AssetObjects{Client: client, QuarantineBucket: objects.Bucket, PublicBucket: objects.Bucket}
	closedRouter := chi.NewRouter()
	configured.RegisterRoutes(closedRouter)
	configured.RegisterAccountAuthority(closedRouter)
	pool.Close()
	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodGet, "/v2/tokens/" + wallet + "/metadata"},
		{http.MethodGet, "/v2/tokens/" + wallet + "/media"},
		{http.MethodPut, "/v2/tokens/" + wallet + "/assets/" + assetID},
		{http.MethodGet, "/v2/tokens/" + wallet + "/assets/" + assetID},
		{http.MethodGet, "/internal/v2/accounts/" + wallet + "/assets/" + assetID},
		{http.MethodDelete, "/internal/v2/accounts/" + wallet + "/assets?deletionId=" + uuid.NewString()},
	} {
		recorder := httptest.NewRecorder()
		closedRouter.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s with unavailable store = %d %s", tc.method, tc.path, recorder.Code, recorder.Body.String())
		}
	}
	if err := configured.ProcessAccountAsset(ctx, assetID); err == nil {
		t.Fatal("account processing accepted unavailable store")
	}
	if err := configured.MaintainAccountAssets(ctx, false); err == nil {
		t.Fatal("account maintenance accepted unavailable store")
	}
	if err := configured.ExpireRejected(ctx); err == nil {
		t.Fatal("asset retention accepted unavailable store")
	}
}
