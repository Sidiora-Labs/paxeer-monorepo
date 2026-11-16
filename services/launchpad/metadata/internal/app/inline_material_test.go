package app

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/config"
)

func inlineMaterialCertificates(t *testing.T) (config.Config, *tls.Config) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "metadata client authority"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, public, private)
	if err != nil {
		t.Fatal(err)
	}
	ca, err = x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	issue := func(serial int64, usage x509.ExtKeyUsage) (string, string) {
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		leaf := &x509.Certificate{
			SerialNumber: big.NewInt(serial), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
			KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage},
			IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		}
		der, err := x509.CreateCertificate(rand.Reader, leaf, ca, pub, private)
		if err != nil {
			t.Fatal(err)
		}
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
			string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	}
	cert, key := issue(2, x509.ExtKeyUsageServerAuth)
	clientCert, clientKey := issue(3, x509.ExtKeyUsageClientAuth)
	pair, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	return config.Config{TLSCertPEM: cert, TLSKeyPEM: key,
			TLSClientCAPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}))},
		&tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{pair}}
}

func TestInlineMaterialListener(t *testing.T) {
	inline, clientTLS := inlineMaterialCertificates(t)
	for mask := 0; mask < 8; mask++ {
		t.Run(fmt.Sprintf("file_sources_%d", mask), func(t *testing.T) {
			cfg := inline
			for bit, material := range []struct{ file, inline *string }{
				{&cfg.TLSCertFile, &cfg.TLSCertPEM}, {&cfg.TLSKeyFile, &cfg.TLSKeyPEM}, {&cfg.TLSClientCAFile, &cfg.TLSClientCAPEM},
			} {
				if mask&(1<<bit) == 0 {
					continue
				}
				*material.file = filepath.Join(t.TempDir(), "material.pem")
				if err := os.WriteFile(*material.file, []byte(*material.inline), 0o600); err != nil {
					t.Fatal(err)
				}
				*material.inline = ""
			}
			serverTLS, err := metadataServerTLS(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if serverTLS.MinVersion != tls.VersionTLS13 || serverTLS.ClientAuth != tls.RequireAndVerifyClientCert {
				t.Fatal("listener must require TLS 1.3 and verified client certificates")
			}
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.TLS.Version != tls.VersionTLS13 || len(r.TLS.VerifiedChains) == 0 {
					t.Error("listener did not verify a TLS 1.3 client")
					w.WriteHeader(http.StatusForbidden)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			server.TLS = serverTLS
			server.StartTLS()
			defer server.Close()
			request := func(configuration *tls.Config, admitted bool) {
				t.Helper()
				transport := &http.Transport{TLSClientConfig: configuration}
				defer transport.CloseIdleConnections()
				client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
				response, err := client.Get(server.URL)
				if !admitted {
					if err == nil {
						response.Body.Close()
						t.Fatal("listener accepted an unauthorized TLS client")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				if response.StatusCode != http.StatusNoContent || response.TLS.Version != tls.VersionTLS13 || len(response.TLS.VerifiedChains) == 0 {
					t.Fatal("authorized TLS 1.3 client did not complete the request")
				}
			}
			request(clientTLS.Clone(), true)
			without := clientTLS.Clone()
			without.Certificates = nil
			request(without, false)
			older := clientTLS.Clone()
			older.MinVersion, older.MaxVersion = tls.VersionTLS12, tls.VersionTLS12
			request(older, false)
			_, untrusted := inlineMaterialCertificates(t)
			untrusted.RootCAs = clientTLS.RootCAs
			request(untrusted, false)
			if cfg.TLSKeyFile != "" {
				if err := os.Chmod(cfg.TLSKeyFile, 0o644); err != nil {
					t.Fatal(err)
				}
				if _, err := metadataServerTLS(cfg); err == nil || err.Error() != "metadata_tls_key_authority_unavailable" {
					t.Fatalf("readable key file: %v", err)
				}
			}
		})
	}
}

func TestInlineMaterialAdmission(t *testing.T) {
	base, _ := inlineMaterialCertificates(t)
	for _, material := range []struct {
		name         string
		selectSource func(*config.Config) (*string, *string)
	}{
		{"METADATA_TLS_CERT", func(c *config.Config) (*string, *string) { return &c.TLSCertFile, &c.TLSCertPEM }},
		{"METADATA_TLS_KEY", func(c *config.Config) (*string, *string) { return &c.TLSKeyFile, &c.TLSKeyPEM }},
		{"METADATA_TLS_CLIENT_CA", func(c *config.Config) (*string, *string) { return &c.TLSClientCAFile, &c.TLSClientCAPEM }},
	} {
		for _, form := range []string{"both", "neither", "oversized", "invalid"} {
			t.Run(material.name+"_"+form, func(t *testing.T) {
				cfg := base
				file, inline := material.selectSource(&cfg)
				switch form {
				case "both":
					*file = filepath.Join(t.TempDir(), "material.pem")
				case "neither":
					*inline = ""
				case "oversized":
					*inline = strings.Repeat("x", (1<<20)+1)
				case "invalid":
					*inline = "invalid PEM material"
				}
				_, err := metadataServerTLS(cfg)
				if err == nil {
					t.Fatal("invalid listener material accepted")
				}
				if form != "invalid" && !strings.Contains(err.Error(), material.name) {
					t.Fatal("material admission must name the source")
				}
				if cfg.TLSKeyPEM != "" && strings.Contains(err.Error(), cfg.TLSKeyPEM) {
					t.Fatal("listener error contains private key material")
				}
			})
		}
	}
}
