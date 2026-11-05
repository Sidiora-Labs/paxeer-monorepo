package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHPXIngressProcess(t *testing.T) {
	directory := os.Getenv("PAXEER_X_HPX_GATE_DIRECTORY")
	if directory == "" {
		t.Fatal("actual protected gate directory required")
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		t.Fatal("protected gate directory required")
	}
	state := filepath.Join(directory, "registry")
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	registry, err := openRegistry(state)
	if err != nil {
		t.Fatal(err)
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		t.Fatal(err)
	}
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(public)
	owner := &server{cfg: config{ChainID: "paxeer_125-1", RegisterTok: hex.EncodeToString(token)}, reg: registry}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/register", owner.handleRegister)
	mux.HandleFunc("/api/myip", owner.handleMyIP)
	mux.HandleFunc("/api/nodes", owner.handleNodes)
	mux.HandleFunc("/healthz", owner.handleHealth)
	caPublic, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authority := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "HPX isolated ingress authority"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(10 * time.Minute),
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, IsCA: true, BasicConstraintsValid: true}
	caDER, err := x509.CreateCertificate(rand.Reader, authority, authority, caPublic, caPrivate)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	leafPublic, leafPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), DNSNames: []string{"localhost"}, Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: authority.NotBefore, NotAfter: authority.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, parent, leafPublic, caPrivate)
	if err != nil {
		t.Fatal(err)
	}
	serving := httptest.NewUnstartedServer(mux)
	serving.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{{Certificate: [][]byte{leafDER, caDER}, PrivateKey: leafPrivate}}}
	serving.StartTLS()
	defer serving.Close()
	ca := filepath.Join(directory, "ca.der")
	if err := os.WriteFile(ca, caDER, 0600); err != nil {
		t.Fatal(err)
	}
	document := map[string]string{"url": strings.Replace(serving.URL, "127.0.0.1", "localhost", 1), "ca": ca, "token": owner.cfg.RegisterTok, "node_id": hex.EncodeToString(digest[:20])}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	pending := filepath.Join(directory, "ready.pending")
	if err := os.WriteFile(pending, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(pending, filepath.Join(directory, "ready.json")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(directory, "stop")); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("bounded actual HPX fixture exceeded its lifetime")
}
