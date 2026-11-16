package meshpki_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/meshpki"
)

func run(t *testing.T, args ...string) []byte {
	t.Helper()
	var out, diagnostic bytes.Buffer
	if err := meshpki.Run(args, &out, &diagnostic); err != nil {
		t.Fatalf("%s: %v", args[0], err)
	}
	if diagnostic.Len() != 0 {
		t.Fatal("successful command emitted diagnostics")
	}
	if args[0] != "describe" && out.Len() != 0 {
		t.Fatal("generation action emitted material")
	}
	return out.Bytes()
}

func read(t *testing.T, dir, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func parseCert(t *testing.T, raw []byte) *x509.Certificate {
	t.Helper()
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" || len(rest) != 0 {
		t.Fatal("expected one PEM certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func authority(t *testing.T, dir string) *x509.CertPool {
	t.Helper()
	run(t, "authority", "--dir", dir, "--lifetime", "17520h")
	cert := parseCert(t, read(t, dir, "ca.cert.pem"))
	key, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok || key.Curve != elliptic.P256() || !cert.IsCA || !cert.MaxPathLenZero || cert.KeyUsage&x509.KeyUsageCertSign == 0 || cert.CheckSignatureFrom(cert) != nil {
		t.Fatal("authority must be a self-signed P-256 signing authority without intermediates")
	}
	if cert.NotAfter.Sub(cert.NotBefore) != 17520*time.Hour {
		t.Fatal("authority lifetime differs")
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	return roots
}

func service(t *testing.T, dir, name string, names []string) tls.Certificate {
	t.Helper()
	run(t, "issue", "--dir", dir, "--service", name, "--names", strings.Join(names, ","), "--lifetime", "8760h")
	run(t, "assertion", "--dir", dir, "--service", name)
	pair, err := tls.LoadX509KeyPair(filepath.Join(dir, name+".cert.pem"), filepath.Join(dir, name+".key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	return pair
}

func TestAuthorityVerifiesBothUsesAndEveryName(t *testing.T) {
	dir := t.TempDir()
	roots := authority(t, dir)
	names := []string{"metadata", "metadata-private", net.IPv4(127, 0, 0, 1).String(), net.IPv6loopback.String()}
	pair := service(t, dir, "metadata", names)
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if leaf.Subject.CommonName != "metadata" || leaf.NotAfter.Sub(leaf.NotBefore) != 8760*time.Hour {
		t.Fatal("service identity or lifetime differs")
	}
	if key, ok := leaf.PublicKey.(*ecdsa.PublicKey); !ok || key.Curve != elliptic.P256() {
		t.Fatal("leaf must use P-256")
	}
	for _, usage := range []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth} {
		for _, name := range names {
			if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: name, KeyUsages: []x509.ExtKeyUsage{usage}}); err != nil {
				t.Fatalf("name %s usage %d: %v", name, usage, err)
			}
		}
	}
	if leaf.VerifyHostname("other-service") == nil {
		t.Fatal("unissued name accepted")
	}
	foreign := t.TempDir()
	foreignRoots := authority(t, foreign)
	foreignPair := service(t, foreign, "metadata", names)
	foreignLeaf, err := x509.ParseCertificate(foreignPair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, usage := range []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth} {
		if _, err := foreignLeaf.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{usage}}); err == nil {
			t.Fatal("foreign authority accepted")
		}
		if _, err := leaf.Verify(x509.VerifyOptions{Roots: foreignRoots, KeyUsages: []x509.ExtKeyUsage{usage}}); err == nil {
			t.Fatal("local certificate accepted by foreign authority")
		}
	}
	t.Log("Issued names verify for TLS server and client use; foreign authorities are refused")
}

func TestDescribeDigestsAndAssertionReaderForms(t *testing.T) {
	dir := t.TempDir()
	authority(t, dir)
	service(t, dir, "gateway", []string{"gateway", "gateway-private"})
	raw := run(t, "describe", "--dir", dir, "--service", "gateway")
	var d meshpki.Description
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&d); err != nil {
		t.Fatal(err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		t.Fatal("describe must emit exactly one JSON object")
	}
	certPEM := read(t, dir, d.CertFile)
	caPEM := read(t, dir, d.CAFile)
	// services/launchpad/gateway/internal/app/app.go: gatewayBoundBytes,
	// and services/launchpad/social/internal/app/app.go: socialBoundBytes.
	// Both bind CA/client/server files by hashing the exact file bytes.
	if d.CASHA256 != fmt.Sprintf("%x", sha256.Sum256(caPEM)) {
		t.Fatal("CA file digest differs from bound-file reader")
	}
	if d.CertSHA256 != fmt.Sprintf("%x", sha256.Sum256(certPEM)) {
		t.Fatal("certificate file digest differs from bound-file reader")
	}
	cert := parseCert(t, certPEM)
	// gateway newGatewayConnectedRuntime hashes pair.Certificate[0].
	pair, err := tls.LoadX509KeyPair(filepath.Join(dir, d.CertFile), filepath.Join(dir, "gateway.key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if d.PeerCertSHA256 != fmt.Sprintf("%x", sha256.Sum256(pair.Certificate[0])) {
		t.Fatal("gateway peer pin differs")
	}
	// user pinnedAccountClient, metadata metadataPinnedClient, social
	// platformPrivate.handler/authority and livestream platformPrivate.handler
	// hash the verified peer certificate's Raw DER; shared/auth identity
	// transport uses the same certificate bytes for API identity peers.
	sum := sha256.Sum256(cert.Raw)
	if d.PeerCertSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("peer pin differs from TLS reader")
	}
	if d.PeerCertSHA256 == d.CertSHA256 {
		t.Fatal("file binding and DER pin must be distinct")
	}
	// gateway identityServiceVerifiers decodes unpadded base64url, exactly 32 bytes.
	public, err := base64.RawURLEncoding.Strict().DecodeString(d.PublicKey)
	if err != nil || len(public) != ed25519.PublicKeySize || strings.ContainsAny(d.PublicKey, "+/=") {
		t.Fatal("identity peer public key encoding differs")
	}
	stdPublic, err := base64.StdEncoding.Strict().DecodeString(d.PublicKeyStdBase64)
	if err != nil || !bytes.Equal(public, stdPublic) {
		t.Fatal("ACCOUNT_GATEWAY_PUBLIC_KEY differs")
	}
	hexPublic, err := hex.DecodeString(d.PublicKeyHex)
	if err != nil || !bytes.Equal(public, hexPublic) || strings.ToLower(d.PublicKeyHex) != d.PublicKeyHex {
		t.Fatal("metadata assertion public key differs")
	}
	forms := []struct {
		suffix string
		decode func(string) ([]byte, error)
	}{
		// gateway newGatewayConnectedRuntime and social newSocialRuntime.
		{"rawurl", base64.RawURLEncoding.Strict().DecodeString},
		// livestream config.IdentityAuthority and shared/auth.DecodeIdentitySigningKey.
		{"std", base64.StdEncoding.Strict().DecodeString},
		// api runtimeIdentity.
		{"rawstd", base64.RawStdEncoding.Strict().DecodeString},
		// metadata newProduction reads METADATA_ASSERTION_PRIVATE_KEY_HEX.
		{"hex", hex.DecodeString},
	}
	var canonical []byte
	for _, form := range forms {
		encoded := read(t, dir, "gateway.assertion."+form.suffix)
		private, err := form.decode(string(encoded))
		if err != nil || len(private) != ed25519.PrivateKeySize {
			t.Fatalf("%s private key encoding differs", form.suffix)
		}
		if canonical == nil {
			canonical = private
		} else if !bytes.Equal(canonical, private) {
			t.Fatal("private key encodings refer to different keys")
		}
		regenerated := ed25519.NewKeyFromSeed(private[:ed25519.SeedSize])
		if !bytes.Equal(private, regenerated) || !bytes.Equal(public, regenerated.Public().(ed25519.PublicKey)) {
			t.Fatal("private key does not match public key")
		}
		if bytes.Contains(raw, encoded) {
			t.Fatal("describe exposed a private key")
		}
	}
	// user config.DecodeAccountKey takes standard base64 of a 32-byte seed;
	// account runtime constructs ed25519.NewKeyFromSeed(signingSeed).
	encodedSeed := read(t, dir, "gateway.assertion.seed.std")
	seed, err := base64.StdEncoding.Strict().DecodeString(string(encodedSeed))
	if err != nil || len(seed) != ed25519.SeedSize || !bytes.Equal(ed25519.NewKeyFromSeed(seed), canonical) {
		t.Fatal("account signing seed differs")
	}
	for _, private := range [][]byte{encodedSeed, canonical, read(t, dir, "gateway.key.pem"), read(t, dir, "ca.key.pem")} {
		if bytes.Contains(raw, private) {
			t.Fatal("describe exposed private material")
		}
	}
	message := []byte("service assertion")
	if !ed25519.Verify(public, message, ed25519.Sign(ed25519.PrivateKey(canonical), message)) {
		t.Fatal("assertion signature did not verify")
	}
	if d.Service != "gateway" || !reflect.DeepEqual(d.Names, []string{"gateway", "gateway-private"}) || !d.NotAfter.Equal(cert.NotAfter) || !d.NotBefore.Equal(cert.NotBefore) {
		t.Fatal("public service metadata differs")
	}
	// Describe works with only public material present.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != d.CAFile && entry.Name() != d.CertFile && entry.Name() != "gateway.assertion.public" {
			if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got := run(t, "describe", "--dir", dir, "--service", "gateway"); !bytes.Equal(got, raw) {
		t.Fatal("describe depends on private files")
	}
	t.Log("Public JSON matches PEM bindings, DER pins and each service's assertion-key encoding")
}

func TestPrivateModesAndNoReplacement(t *testing.T) {
	dir := t.TempDir()
	authority(t, dir)
	service(t, dir, "social", []string{"social"})
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	original := map[string][]byte{}
	for _, entry := range entries {
		original[entry.Name()] = read(t, dir, entry.Name())
		if strings.HasSuffix(entry.Name(), ".key.pem") || strings.Contains(entry.Name(), ".assertion.") && !strings.HasSuffix(entry.Name(), ".public") {
			info, err := entry.Info()
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Fatalf("private mode for %s = %o", entry.Name(), info.Mode().Perm())
			}
		}
	}
	actions := [][]string{
		{"authority", "--dir", dir, "--lifetime", "17520h"},
		{"issue", "--dir", dir, "--service", "social", "--names", "social", "--lifetime", "8760h"},
		{"assertion", "--dir", dir, "--service", "social"},
	}
	for _, args := range actions {
		var out, diagnostic bytes.Buffer
		if err := meshpki.Run(args, &out, &diagnostic); err == nil {
			t.Fatalf("second %s accepted", args[0])
		}
		if out.Len() != 0 || diagnostic.Len() != 0 {
			t.Fatal("failed action emitted material")
		}
		for name, want := range original {
			if !bytes.Equal(read(t, dir, name), want) {
				t.Fatalf("second %s replaced %s", args[0], name)
			}
		}
	}
	t.Log("Every private file is mode 0600 and repeated actions preserve all files")
}

func TestFullGatewayUpstreamSet(t *testing.T) {
	dir := t.TempDir()
	roots := authority(t, dir)
	pairs := map[string]tls.Certificate{}
	for _, name := range []string{"gateway", "user", "metadata", "social", "livestream", "api"} {
		pairs[name] = service(t, dir, name, []string{name, name + "-private"})
		run(t, "describe", "--dir", dir, "--service", name)
	}
	// Exchange real TLS 1.3 handshakes in both directions over connected sockets.
	// The gateway is a client to each upstream and the identity server for each.
	for _, upstream := range []string{"user", "metadata", "social", "livestream", "api"} {
		for _, endpoints := range [][2]string{{"gateway", upstream}, {upstream, "gateway"}} {
			clientName, serverName := endpoints[0], endpoints[1]
			t.Run(clientName+"_to_"+serverName, func(t *testing.T) {
				left, right := net.Pipe()
				defer left.Close()
				defer right.Close()
				client := tls.Client(left, &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, ServerName: serverName + "-private", Certificates: []tls.Certificate{pairs[clientName]}})
				server := tls.Server(right, &tls.Config{MinVersion: tls.VersionTLS13, SessionTicketsDisabled: true, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots, Certificates: []tls.Certificate{pairs[serverName]}})
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				completed := make(chan error, 1)
				go func() { completed <- server.HandshakeContext(ctx) }()
				if err := client.HandshakeContext(ctx); err != nil {
					t.Fatal(err)
				}
				if err := <-completed; err != nil {
					t.Fatal(err)
				}
				for _, state := range []tls.ConnectionState{client.ConnectionState(), server.ConnectionState()} {
					if state.Version != tls.VersionTLS13 || len(state.VerifiedChains) == 0 {
						t.Fatal("mutually verified TLS 1.3 required")
					}
				}
				peer := server.ConnectionState().PeerCertificates[0]
				if !bytes.Equal(peer.Raw, pairs[clientName].Certificate[0]) || peer.VerifyHostname(clientName+"-private") != nil {
					t.Fatal("incorrect client identity")
				}
				description, err := meshpki.DescribeService(dir, clientName)
				if err != nil || description.PeerCertSHA256 != fmt.Sprintf("%x", sha256.Sum256(peer.Raw)) {
					t.Fatal("client pin differs on actual TLS channel")
				}
			})
		}
	}
	t.Log("Gateway, four upstream services and API load as standard TLS pairs and mutually authenticate")
}
