// Package meshpki issues private TLS identities and Ed25519 assertion keys.
package meshpki

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"regexp"
	"strings"
	"time"
)

const (
	// MinLifetime bounds certificate validity to at least one hour.
	MinLifetime = time.Hour
	// MaxAuthorityLifetime bounds an authority to ten 365-day years.
	MaxAuthorityLifetime = 10 * 365 * 24 * time.Hour
	// MaxServiceLifetime bounds a service certificate to one 365-day year.
	MaxServiceLifetime = 365 * 24 * time.Hour
)

var serviceToken = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

func checkService(service string) error {
	if !serviceToken.MatchString(service) || service == "ca" {
		return errors.New("service must be a lowercase token of 1 to 63 characters other than ca")
	}
	return nil
}

func validity(lifetime, maximum time.Duration) (time.Time, time.Time, error) {
	if lifetime < MinLifetime || lifetime > maximum {
		return time.Time{}, time.Time{}, fmt.Errorf("lifetime must be between %s and %s", MinLifetime, maximum)
	}
	now := time.Now().UTC().Truncate(time.Second)
	return now, now.Add(lifetime), nil
}

func template(start, end time.Time) (*x509.Certificate, error) {
	n, err := rand.Int(rand.Reader, new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)))
	if err != nil {
		return nil, err
	}
	return &x509.Certificate{SerialNumber: n.Add(n, big.NewInt(1)), NotBefore: start, NotAfter: end, BasicConstraintsValid: true}, nil
}

func certificate(raw []byte) (*x509.Certificate, error) {
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("exactly one PEM certificate required")
	}
	return x509.ParseCertificate(block.Bytes)
}

func keyPEM(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// CreateAuthority writes a self-signed ECDSA P-256 authority and PKCS#8 key.
// Its lifetime is between MinLifetime and MaxAuthorityLifetime, inclusive.
func CreateAuthority(dir string, lifetime time.Duration) error {
	start, end, err := validity(lifetime, MaxAuthorityLifetime)
	if err != nil {
		return err
	}
	store, err := openStore(dir, true)
	if err != nil {
		return err
	}
	defer store.Close()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	cert, err := template(start, end)
	if err != nil {
		return err
	}
	cert.Subject = pkix.Name{CommonName: "Launchpad private authority"}
	cert.IsCA, cert.MaxPathLenZero = true, true
	cert.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		return err
	}
	private, err := keyPEM(key)
	if err != nil {
		return err
	}
	return writeFiles(store, []outputFile{{"ca.cert.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), false}, {"ca.key.pem", private, true}})
}

// IssueService issues an ECDSA P-256 certificate for all supplied DNS or IP
// names, usable for TLS server and client authentication. Its lifetime is
// between MinLifetime and MaxServiceLifetime and must fit inside the authority.
func IssueService(dir, service string, names []string, lifetime time.Duration) error {
	if err := checkService(service); err != nil {
		return err
	}
	start, end, err := validity(lifetime, MaxServiceLifetime)
	if err != nil {
		return err
	}
	cert, err := template(start, end)
	if err != nil {
		return err
	}
	cert.Subject = pkix.Name{CommonName: service}
	cert.KeyUsage = x509.KeyUsageDigitalSignature
	cert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	if len(names) == 0 || len(names) > 64 {
		return errors.New("between 1 and 64 names required")
	}
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			return errors.New("duplicate certificate name")
		}
		seen[name] = true
		if ip := net.ParseIP(name); ip != nil {
			cert.IPAddresses = append(cert.IPAddresses, ip)
			continue
		}
		if !validDNS(name) {
			return errors.New("certificate name must be a DNS name or IP address")
		}
		cert.DNSNames = append(cert.DNSNames, name)
	}
	store, err := openStore(dir, false)
	if err != nil {
		return err
	}
	defer store.Close()
	caPEM, err := readFile(store, "ca.cert.pem")
	if err != nil {
		return err
	}
	ca, err := certificate(caPEM)
	if err != nil {
		return err
	}
	rawKey, err := readFile(store, "ca.key.pem")
	if err != nil {
		return err
	}
	pair, err := tls.X509KeyPair(caPEM, rawKey)
	if err != nil {
		return errors.New("authority certificate and key do not match")
	}
	caKey, ok := pair.PrivateKey.(*ecdsa.PrivateKey)
	if !ok || caKey.Curve != elliptic.P256() || !ca.IsCA || !ca.BasicConstraintsValid || ca.KeyUsage&x509.KeyUsageCertSign == 0 || ca.CheckSignatureFrom(ca) != nil {
		return errors.New("self-signed P-256 private authority required")
	}
	if start.Before(ca.NotBefore) || end.After(ca.NotAfter) {
		return errors.New("service validity must fit inside authority validity")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, ca, &key.PublicKey, caKey)
	if err != nil {
		return err
	}
	private, err := keyPEM(key)
	if err != nil {
		return err
	}
	return writeFiles(store, []outputFile{{service + ".cert.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), false}, {service + ".key.pem", private, true}})
}

func validDNS(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

// CreateAssertionKey writes one Ed25519 key in the encodings consumed by the
// services. Only the public key file is needed by DescribeService.
func CreateAssertionKey(dir, service string) error {
	if err := checkService(service); err != nil {
		return err
	}
	store, err := openStore(dir, true)
	if err != nil {
		return err
	}
	defer store.Close()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	prefix := service + ".assertion."
	return writeFiles(store, []outputFile{
		{prefix + "public", []byte(base64.RawURLEncoding.EncodeToString(public)), false},
		{prefix + "rawurl", []byte(base64.RawURLEncoding.EncodeToString(private)), true},
		{prefix + "std", []byte(base64.StdEncoding.EncodeToString(private)), true},
		{prefix + "rawstd", []byte(base64.RawStdEncoding.EncodeToString(private)), true},
		{prefix + "hex", []byte(hex.EncodeToString(private)), true},
		{prefix + "seed.std", []byte(base64.StdEncoding.EncodeToString(private.Seed())), true},
	})
}

// Description contains public values only. File digests cover exact PEM bytes;
// PeerCertSHA256 covers the leaf DER. PublicKey is unpadded base64url.
type Description struct {
	Service            string    `json:"service"`
	CAFile             string    `json:"caFile"`
	CASHA256           string    `json:"caSha256"`
	CertFile           string    `json:"certFile"`
	CertSHA256         string    `json:"certSha256"`
	PeerCertSHA256     string    `json:"peerCertSha256"`
	Names              []string  `json:"names"`
	NotBefore          time.Time `json:"notBefore"`
	NotAfter           time.Time `json:"notAfter"`
	PublicKey          string    `json:"publicKey"`
	PublicKeyStdBase64 string    `json:"publicKeyStdBase64"`
	PublicKeyHex       string    `json:"publicKeyHex"`
}

// DescribeService reads only public files and describes a complete service set.
func DescribeService(dir, service string) (Description, error) {
	if err := checkService(service); err != nil {
		return Description{}, err
	}
	store, err := openStore(dir, false)
	if err != nil {
		return Description{}, err
	}
	defer store.Close()
	caRaw, err := readFile(store, "ca.cert.pem")
	if err != nil {
		return Description{}, err
	}
	ca, err := certificate(caRaw)
	if err != nil {
		return Description{}, err
	}
	raw, err := readFile(store, service+".cert.pem")
	if err != nil {
		return Description{}, err
	}
	cert, err := certificate(raw)
	if err != nil {
		return Description{}, err
	}
	if cert.Subject.CommonName != service {
		return Description{}, errors.New("certificate service does not match")
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	for _, usage := range []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth} {
		if _, err = cert.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{usage}}); err != nil {
			return Description{}, errors.New("service certificate is not valid for this authority and both TLS uses")
		}
	}
	encoded, err := readFile(store, service+".assertion.public")
	if err != nil {
		return Description{}, err
	}
	public, err := base64.RawURLEncoding.Strict().DecodeString(string(encoded))
	if err != nil || len(public) != ed25519.PublicKeySize {
		return Description{}, errors.New("invalid assertion public key")
	}
	names := append([]string{}, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		names = append(names, ip.String())
	}
	return Description{Service: service, CAFile: "ca.cert.pem", CASHA256: digest(caRaw), CertFile: service + ".cert.pem", CertSHA256: digest(raw), PeerCertSHA256: digest(cert.Raw), Names: names, NotBefore: cert.NotBefore, NotAfter: cert.NotAfter, PublicKey: string(encoded), PublicKeyStdBase64: base64.StdEncoding.EncodeToString(public), PublicKeyHex: hex.EncodeToString(public)}, nil
}

func digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
