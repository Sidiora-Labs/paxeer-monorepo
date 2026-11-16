package app

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/config"
)

func TestHealthListener(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/private", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	health := httptest.NewServer(healthOnlyHandler(mux))
	defer health.Close()
	for _, path := range []string{"/health/live", "/health/ready"} {
		response, err := health.Client().Get(health.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("plain %s = %d", path, response.StatusCode)
		}
	}
	response, err := health.Client().Get(health.URL + "/private")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("plain private = %d", response.StatusCode)
	}

	clientCertificate, roots := testClientCertificate(t)
	tlsServer := httptest.NewTLSServer(mux)
	defer tlsServer.Close()
	tlsServer.TLS.ClientAuth = tls.RequireAndVerifyClientCert
	tlsServer.TLS.ClientCAs = roots
	withoutCertificate := tlsServer.Client()
	if _, err := withoutCertificate.Get(tlsServer.URL + "/health/ready"); err == nil {
		t.Fatal("TLS listener accepted a client without a certificate")
	}
	withCertificate := tlsServer.Client()
	transport := withCertificate.Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.Certificates = []tls.Certificate{clientCertificate}
	withCertificate.Transport = transport
	for _, path := range []string{"/health/live", "/health/ready", "/private"} {
		response, err := withCertificate.Get(tlsServer.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		want := http.StatusOK
		if path == "/private" {
			want = http.StatusNoContent
		}
		if response.StatusCode != want {
			t.Fatalf("TLS %s = %d, want %d", path, response.StatusCode, want)
		}
	}

	cfg := config.Config{NodeEnv: "production", RuntimeProfile: "container", Ingress: config.IngressMTLS, Port: 8080, PrivateListenAddress: "127.0.0.1:8080"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "METADATA_PRIVATE_LISTEN_ADDRESS") {
		t.Fatalf("equal listener ports were accepted: %v", err)
	}
}

func testClientCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "health test CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	clientTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "health test client"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, KeyUsage: x509.KeyUsageDigitalSignature}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	clientPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)})
	certificate, err := tls.X509KeyPair(clientPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	return certificate, roots
}
