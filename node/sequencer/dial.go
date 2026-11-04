package sequencer

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"time"
)

// ReplicationTLSConfig authenticates both replication roles using a dedicated
// certificate, configured trust roots and the peer's pinned certificate key.
// PeerName is the peer's certificate DNS identity; PeerSPKI is its hex SHA-256
// SubjectPublicKeyInfo digest. Trust and pin checks are both required.
type ReplicationTLSConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
	PeerName string
	PeerSPKI string
}

func (c ReplicationTLSConfig) TLSConfig(server bool) (*tls.Config, error) {
	if c.CertFile == "" || c.KeyFile == "" || c.CAFile == "" || c.PeerName == "" {
		return nil, fmt.Errorf("%w: replication TLS certificate, key, roots and peer identity are required", ErrConfigValue)
	}
	pin, err := hex.DecodeString(c.PeerSPKI)
	if err != nil || len(pin) != sha256.Size {
		return nil, fmt.Errorf("%w: replication peer SPKI must be a SHA-256 digest", ErrConfigValue)
	}
	cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("sequencer: replication TLS certificate: %w", err)
	}
	pem, err := os.ReadFile(c.CAFile)
	if err != nil {
		return nil, fmt.Errorf("sequencer: replication TLS roots: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("%w: no replication trust roots", ErrConfigValue)
	}
	cfg := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
		RootCAs:      roots, ClientCAs: roots, ServerName: c.PeerName,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.VerifiedChains) == 0 || len(state.PeerCertificates) == 0 {
				return fmt.Errorf("sequencer: unverified replication peer")
			}
			peer := state.PeerCertificates[0]
			if err := peer.VerifyHostname(c.PeerName); err != nil {
				return err
			}
			digest := sha256.Sum256(peer.RawSubjectPublicKeyInfo)
			if subtle.ConstantTimeCompare(digest[:], pin) != 1 {
				return fmt.Errorf("sequencer: replication peer key differs from pin")
			}
			return nil
		},
	}
	if server {
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg, nil
}

// DialStandby completes mutual TLS authentication before exposing the WAL/ACK
// stream. Missing trust configuration fails closed, including legacy callers.
func DialStandby(address string, timeout time.Duration, auth ...ReplicationTLSConfig) func(ctx context.Context) (net.Conn, error) {
	return func(ctx context.Context) (net.Conn, error) {
		if _, _, err := net.SplitHostPort(address); err != nil {
			return nil, fmt.Errorf("%w %q: %v", ErrConfigStandbyAddress, address, err)
		}
		if timeout <= 0 {
			return nil, fmt.Errorf("%w: must be positive, got %s", ErrConfigReplicationTimeout, timeout)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(auth) != 1 {
			return nil, fmt.Errorf("%w: replication TLS configuration is required", ErrConfigValue)
		}
		cfg, err := auth[0].TLSConfig(false)
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		d := tls.Dialer{NetDialer: &net.Dialer{Timeout: timeout}, Config: cfg}
		return d.DialContext(ctx, "tcp", address)
	}
}
