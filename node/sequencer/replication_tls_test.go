package sequencer

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Repair103TLSFixture issues real CA-signed certificates for both roles.
// It is exported only in the test build so external package tests share it.
func Repair103TLSFixture(t *testing.T) (ReplicationTLSConfig, ReplicationTLSConfig) {
	t.Helper()
	dir := t.TempDir()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "replication-test-root"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, key)
	require.NoError(t, err)
	roots := filepath.Join(dir, "roots.pem")
	require.NoError(t, os.WriteFile(roots, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	issue := func(name string, serial int64) (ReplicationTLSConfig, string) {
		pub, private, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		cert := &x509.Certificate{SerialNumber: big.NewInt(serial), DNSNames: []string{name}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
		der, err := x509.CreateCertificate(rand.Reader, cert, ca, pub, key)
		require.NoError(t, err)
		parsed, err := x509.ParseCertificate(der)
		require.NoError(t, err)
		digest := sha256.Sum256(parsed.RawSubjectPublicKeyInfo)
		cfg := ReplicationTLSConfig{CAFile: roots, CertFile: filepath.Join(dir, name+".pem"), KeyFile: filepath.Join(dir, name+".key")}
		require.NoError(t, os.WriteFile(cfg.CertFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
		pk, err := x509.MarshalPKCS8PrivateKey(private)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(cfg.KeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk}), 0600))
		return cfg, hex.EncodeToString(digest[:])
	}
	primary, primaryPin := issue("primary.test", 2)
	standby, standbyPin := issue("standby.test", 3)
	primary.PeerName, primary.PeerSPKI = "standby.test", standbyPin
	standby.PeerName, standby.PeerSPKI = "primary.test", primaryPin
	return primary, standby
}

func repair103TLSListener(t *testing.T) (net.Listener, ReplicationTLSConfig) {
	t.Helper()
	primary, standby := Repair103TLSFixture(t)
	cfg, err := standby.TLSConfig(true)
	require.NoError(t, err)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	require.NoError(t, err)
	return listener, primary
}
