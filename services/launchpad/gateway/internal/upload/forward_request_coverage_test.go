package upload

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestUploadReportsInvalidConfiguredForwardAuthority(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("wallet", "0x1234567890abcdef1234567890abcdef12345678"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	handler := New(Deps{MetadataBaseURL: "://invalid", Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	request := httptest.NewRequest(http.MethodPost, "/upload/token/0x1234567890abcdef1234567890abcdef12345678", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "Internal Server Error") || !strings.Contains(logs.String(), "upload error") {
		t.Fatalf("forward authority response: status=%d body=%s logs=%s", response.Code, response.Body.String(), logs.String())
	}
}

func TestUploadBoundsActualMetadataResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.Repeat("x", (1<<20)+1)))
	}))
	defer upstream.Close()
	handler := New(Deps{MetadataBaseURL: upstream.URL, MaxBytes: 6 << 20})
	response := httptest.NewRecorder()
	handler.forward(response, httptest.NewRequest(http.MethodPost, "/upload/token/"+testAddr, nil), testAddr, bytes.NewBufferString("owned upload form"), "multipart/form-data")
	var body struct {
		StatusCode int    `json:"statusCode"`
		Error      string `json:"error"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != http.StatusBadGateway || body.StatusCode != http.StatusBadGateway || body.Error != "Internal Server Error" || body.Message != "An unexpected error occurred" {
		t.Fatalf("bounded metadata response: status=%d body=%s", response.Code, response.Body.String())
	}
}
