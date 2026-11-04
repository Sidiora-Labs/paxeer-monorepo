package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
)

func fixtureReplicationTLS(dir string) (sequencer.ReplicationTLSConfig, sequencer.ReplicationTLSConfig, error) {
	var empty sequencer.ReplicationTLSConfig
	caPublic, caKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return empty, empty, err
	}
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "private recorder replication CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, caPublic, caKey)
	if err != nil {
		return empty, empty, err
	}
	roots := filepath.Join(dir, "replication-ca.pem")
	if err := os.WriteFile(roots, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600); err != nil {
		return empty, empty, err
	}
	issue := func(name string, serial int64) (sequencer.ReplicationTLSConfig, string, error) {
		public, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return empty, "", err
		}
		certificate := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, KeyUsage: x509.KeyUsageDigitalSignature}
		der, err := x509.CreateCertificate(rand.Reader, certificate, ca, public, caKey)
		if err != nil {
			return empty, "", err
		}
		secret, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return empty, "", err
		}
		cfg := sequencer.ReplicationTLSConfig{CAFile: roots, CertFile: filepath.Join(dir, name+".pem"), KeyFile: filepath.Join(dir, name+".key")}
		if err := os.WriteFile(cfg.CertFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
			return empty, "", err
		}
		if err := os.WriteFile(cfg.KeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: secret}), 0600); err != nil {
			return empty, "", err
		}
		parsed, err := x509.ParseCertificate(der)
		if err != nil {
			return empty, "", err
		}
		digest := sha256.Sum256(parsed.RawSubjectPublicKeyInfo)
		return cfg, hex.EncodeToString(digest[:]), nil
	}
	primary, primaryPin, err := issue("recorder-primary", 2)
	if err != nil {
		return empty, empty, err
	}
	standby, standbyPin, err := issue("recorder-standby", 3)
	if err != nil {
		return empty, empty, err
	}
	primary.PeerName, primary.PeerSPKI = "recorder-standby", standbyPin
	standby.PeerName, standby.PeerSPKI = "recorder-primary", primaryPin
	return primary, standby, nil
}

func (n *node) configureReplicationTLS() error {
	primary, standby, err := fixtureReplicationTLS(filepath.Join(n.home, "config"))
	if err != nil {
		return err
	}
	for _, peer := range []struct {
		node *node
		tls  sequencer.ReplicationTLSConfig
	}{{n, primary}, {n.standby, standby}} {
		if err := editTOML(filepath.Join(peer.node.home, "config", "app.toml"), func(cfg map[string]any) {
			section := cfg["sequencer"].(map[string]any)
			section["replication-tls-cert"], section["replication-tls-key"], section["replication-tls-ca"] = peer.tls.CertFile, peer.tls.KeyFile, peer.tls.CAFile
			section["replication-peer-name"], section["replication-peer-spki"] = peer.tls.PeerName, peer.tls.PeerSPKI
		}); err != nil {
			return err
		}
	}
	return nil
}
