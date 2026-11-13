package upload

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/internaltest"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestUploadRouteEnforcesActualSharedAdmission(t *testing.T) {
	db, _ := internaltest.NewRedis(t)
	namespace := "upload-route-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	admission, err := NewAdmission(db, namespace, 2)
	if err != nil {
		t.Fatal(err)
	}
	metadata := httptest.NewServer(http.NotFoundHandler())
	defer metadata.Close()
	handler := New(Deps{Admission: admission, MetadataBaseURL: metadata.URL, MaxBytes: 6 << 20})
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	request := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/upload/token/"+testAddr, strings.NewReader("not multipart"))
		r = r.WithContext(auth.WithActor(r.Context(), testAddr))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	if response := request(); response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), "upload exceeds size limit or is malformed") {
		t.Fatalf("admitted malformed upload: status=%d body=%s", response.Code, response.Body.String())
	}
	first, err := admission.Acquire(context.Background(), testAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer first()
	second, err := admission.Acquire(context.Background(), testAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer second()
	if response := request(); response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" {
		t.Fatalf("shared admission limit: status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
}
