package notifications

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderWireRejectsMalformedKeysAndOversizedResponses(t *testing.T) {
	if _, err := ecKey("invalid-pem"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing key envelope accepted: %v", err)
	}
	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(p384)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ecKey(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}))); !errors.Is(err, ErrInvalid) {
		t.Fatalf("wrong signing curve accepted: %v", err)
	}
	if _, err := ecKey(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("not-pkcs8")}))); err == nil {
		t.Fatal("malformed signing key accepted")
	}
	key, err := ecKey(repair958PEM(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ecJWT(key, math.NaN(), map[string]string{"sub": "device"}); err == nil {
		t.Fatal("non-JSON JWT header accepted")
	}
	if _, err := ecJWT(key, map[string]string{"typ": "JWT"}, math.NaN()); err == nil {
		t.Fatal("non-JSON JWT claim accepted")
	}
	if _, err := jsonRequest(context.Background(), http.MethodPost, "https://example.test", math.NaN()); err == nil {
		t.Fatal("non-JSON provider body accepted")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(w, string(make([]byte, 65537))); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := sendRequest(server.Client(), request); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized provider response accepted: %v", err)
	}
}
