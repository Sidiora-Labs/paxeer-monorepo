package upload

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestConnectedUploadResponsePreservesAndNormalizesErrors(t *testing.T) {
	original := httptest.NewRecorder()
	sharedhttp.WriteErrorEnvelope(original, http.StatusForbidden, "SCOPE_DENIED", "asset blocked", false, 0, nil, nil)
	structured := original.Body.Bytes()
	parsed, err := v2.DecodeError(structured)
	if err != nil || parsed.Code != "SCOPE_DENIED" {
		t.Fatalf("source refusal envelope: %+v err=%v", parsed, err)
	}
	for _, tc := range []struct {
		name, body string
		status     int
		wantCode   string
	}{
		{"upstream envelope", string(structured), http.StatusForbidden, "SCOPE_DENIED"},
		{"unstructured field error", "invalid asset", http.StatusUnprocessableEntity, sharedhttp.CodeFieldInvalid},
		{"oversized body", "upload exceeds limit", http.StatusRequestEntityTooLarge, sharedhttp.CodeFieldInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			wrapped := &connectedUploadResponse{ResponseWriter: response}
			wrapped.WriteHeader(tc.status)
			if _, err := wrapped.Write([]byte(tc.body)); err != nil {
				t.Fatal(err)
			}
			wrapped.finish()
			if wrapped.Unwrap() != response || response.Code != tc.status {
				t.Fatalf("connected response: status=%d unwrap=%T", response.Code, wrapped.Unwrap())
			}
			result, err := v2.DecodeError(response.Body.Bytes())
			if err != nil || result.Code != tc.wantCode {
				t.Fatalf("normalized refusal: %+v err=%v body=%s", result, err, response.Body.String())
			}
			if tc.name == "upstream envelope" && !bytes.Equal(response.Body.Bytes(), structured) {
				t.Fatalf("structured refusal changed: got=%s want=%s", response.Body.Bytes(), structured)
			}
		})
	}
	approved := httptest.NewRecorder()
	wrapped := &connectedUploadResponse{ResponseWriter: approved}
	if _, err := wrapped.Write([]byte("approved")); err != nil {
		t.Fatal(err)
	}
	wrapped.finish()
	if approved.Code != http.StatusOK || approved.Body.String() != "approved" {
		t.Fatalf("approved upstream response: status=%d body=%s", approved.Code, approved.Body.String())
	}
}

func TestUploadObserverWriterTracksImplicitStatus(t *testing.T) {
	response := httptest.NewRecorder()
	observed := &uploadResponse{ResponseWriter: response}
	if _, err := observed.Write([]byte("accepted")); err != nil {
		t.Fatal(err)
	}
	if observed.status != http.StatusOK || response.Body.String() != "accepted" || observed.Unwrap() != response {
		t.Fatalf("implicit upload status: status=%d body=%s", observed.status, response.Body.String())
	}
	observed.WriteHeader(http.StatusCreated)
	if observed.status != http.StatusOK {
		t.Fatalf("first upload status changed to %d", observed.status)
	}
	w := httptest.NewRecorder()
	writeConnectedRefusal(w, http.StatusTooManyRequests, "rate limited", 0)
	if w.Code != http.StatusTooManyRequests || !strings.Contains(w.Body.String(), sharedhttp.CodeRateLimited) {
		t.Fatalf("rate-limit refusal: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestConnectedUploadRefusalCodesMatchStatus(t *testing.T) {
	for _, tc := range []struct {
		status int
		code   string
	}{
		{http.StatusUnauthorized, "AUTH_EXPIRED"},
		{http.StatusForbidden, "SCOPE_DENIED"},
		{http.StatusNotFound, sharedhttp.CodeNotFound},
		{http.StatusBadRequest, sharedhttp.CodeFieldInvalid},
		{http.StatusRequestEntityTooLarge, sharedhttp.CodeFieldInvalid},
		{http.StatusUnsupportedMediaType, sharedhttp.CodeFieldInvalid},
		{http.StatusUnprocessableEntity, sharedhttp.CodeFieldInvalid},
		{http.StatusTooManyRequests, sharedhttp.CodeRateLimited},
		{http.StatusServiceUnavailable, sharedhttp.CodeDependencyUnavailable},
	} {
		response := httptest.NewRecorder()
		writeConnectedRefusal(response, tc.status, "upload unavailable", 0)
		decoded, err := v2.DecodeError(response.Body.Bytes())
		if err != nil || response.Code != tc.status || decoded.Code != tc.code || decoded.Retryable != (tc.status >= 500 || tc.status == http.StatusTooManyRequests) {
			t.Fatalf("status %d: response=%d envelope=%+v err=%v", tc.status, response.Code, decoded, err)
		}
	}
}
