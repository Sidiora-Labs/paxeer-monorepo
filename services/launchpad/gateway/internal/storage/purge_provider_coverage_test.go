package storage

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestConnectedPurgeProviderBindsSubmissionAndCompletion(t *testing.T) {
	type reply struct {
		status int
		body   string
	}
	type request struct {
		path, token, idempotency string
		body                     []byte
	}
	replies := make(chan reply, 8)
	requests := make(chan request, 8)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- request{r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Idempotency-Key"), body}
		response := <-replies
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.status)
		_, _ = io.WriteString(w, response.body)
	}))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	token := strings.Repeat("s", 32)
	provider, err := NewHTTPPurgeProvider(server.URL, token, transport)
	if err != nil {
		t.Fatal(err)
	}
	id, hash, providerID := uuid.NewString(), strings.Repeat("a", 64), "cdn-purge-1"
	replies <- reply{http.StatusAccepted, `{"operationId":"` + id + `","providerId":"` + providerID + `","state":"accepted","completionProof":""}`}
	submitted, err := provider.Submit(context.Background(), id, hash)
	if err != nil || submitted.OperationID != id || submitted.ProviderID != providerID || submitted.State != acceptedState {
		t.Fatalf("purge submission: %+v err=%v", submitted, err)
	}
	first := <-requests
	var submittedBody purgeSubmission
	if err := json.Unmarshal(first.body, &submittedBody); err != nil || first.path != "/v1/purges" || first.token != "Bearer "+token || first.idempotency != id || submittedBody.OperationID != id || submittedBody.Hash != hash {
		t.Fatalf("bound purge request: %+v body=%+v err=%v", first, submittedBody, err)
	}
	replies <- reply{http.StatusOK, `{"operationId":"` + id + `","providerId":"` + providerID + `","state":"complete","completionProof":"cdn-confirmed"}`}
	completed, err := provider.Status(context.Background(), id, providerID)
	if err != nil || completed.State != completeState || completed.CompletionProof != "cdn-confirmed" {
		t.Fatalf("purge completion: %+v err=%v", completed, err)
	}
	second := <-requests
	if second.path != "/v1/purges/status" || second.token != "Bearer "+token || second.idempotency != "" || !strings.Contains(string(second.body), providerID) {
		t.Fatalf("bound status request: %+v", second)
	}
	for _, tc := range []reply{
		{http.StatusUnauthorized, `{"code":"denied"}`},
		{http.StatusOK, `{"operationId":"` + id + `","providerId":"` + providerID + `","state":"complete"}`},
		{http.StatusOK, `{"operationId":"` + id + `","providerId":"` + providerID + `","state":"unknown"}`},
		{http.StatusOK, `{"operationId":"` + id + `","providerId":"` + providerID + `","state":"accepted","unknown":true}`},
		{http.StatusOK, strings.Repeat("x", 16385)},
	} {
		replies <- tc
		if receipt, err := provider.Status(context.Background(), id, providerID); !errors.Is(err, ErrUnavailable) || receipt.OperationID != "" {
			t.Fatalf("invalid provider receipt admitted: %+v err=%v", receipt, err)
		}
		<-requests
	}
}
