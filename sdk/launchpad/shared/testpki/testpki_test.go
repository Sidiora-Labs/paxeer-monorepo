package testpki

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"os"
	"testing"
)

func TestIssuedIdentities(t *testing.T) {
	pki := New(t)
	trusted := pki.Leaf("gateway", "gateway.test")
	foreign := Foreign(t, "outside", "gateway.test")

	caPEM, err := os.ReadFile(pki.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	caBlock, _ := pem.Decode(caPEM)
	if caBlock == nil || caBlock.Type != "CERTIFICATE" {
		t.Fatal("CA file does not contain a certificate")
	}
	if _, err := x509.ParseCertificate(caBlock.Bytes); err != nil {
		t.Fatal(err)
	}
	certPEM, err := os.ReadFile(trusted.CertFile)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM, err := os.ReadFile(trusted.KeyFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(certPEM) != string(trusted.CertPEM) || string(keyPEM) != string(trusted.KeyPEM) {
		t.Fatal("issued files differ from the returned identity")
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil || len(pair.Certificate) != 1 || len(trusted.Pair.Certificate) != 1 {
		t.Fatalf("issued key pair: %v", err)
	}
	cert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if digest := sha256.Sum256(cert.Raw); trusted.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatal("certificate pin does not match issued DER")
	}
	if _, err := cert.Verify(x509.VerifyOptions{Roots: pki.Roots(), DNSName: "gateway.test", KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		t.Fatalf("server identity: %v", err)
	}
	if _, err := cert.Verify(x509.VerifyOptions{Roots: pki.Roots(), DNSName: "gateway.test", KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		t.Fatalf("client identity: %v", err)
	}
	if _, err := cert.Verify(x509.VerifyOptions{Roots: pki.Roots(), DNSName: "other.test"}); err == nil {
		t.Fatal("certificate accepted for another DNS name")
	}
	if _, err := foreign.Pair.Leaf.Verify(x509.VerifyOptions{Roots: pki.Roots(), DNSName: "gateway.test"}); err == nil {
		t.Fatal("foreign certificate trusted by local CA")
	}

	assertion := Ed25519(t)
	private, err := base64.StdEncoding.DecodeString(assertion.StdBase64)
	if err != nil || string(private) != string(assertion.Private) {
		t.Fatalf("assertion private key encoding: %v", err)
	}
	public, err := base64.RawURLEncoding.DecodeString(assertion.PublicRawURL)
	if err != nil || len(public) != ed25519.PublicKeySize {
		t.Fatalf("assertion public key encoding: %v", err)
	}
	message := []byte("gateway assertion")
	signature := ed25519.Sign(assertion.Private, message)
	if !ed25519.Verify(public, message, signature) || ed25519.Verify(public, []byte("another assertion"), signature) {
		t.Fatal("assertion signature verification differs from key encoding")
	}
}
