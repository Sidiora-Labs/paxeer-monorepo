package bringup

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var identityFiles = []struct{ File, Suffix string }{
	{"cert.pem", "CERT"}, {"key.pem", "KEY"}, {"ca.pem", "CA"}, {"cert.der", "CERT_DER"},
	{"key.der", "KEY_DER"}, {"ca.der", "CA_DER"}, {"identity.p12", "P12"}, {"password", "PASSWORD"},
}

type IdentityBundle map[string][]byte

type CertificateState struct {
	State       string
	Fingerprint string
	Days        int64
}

func (s CertificateState) String() string {
	if s.Fingerprint == "" {
		return s.State + " - -"
	}
	return fmt.Sprintf("%s %s %d", s.State, s.Fingerprint, s.Days)
}

func parsePEMCertificate(data []byte) (*x509.Certificate, error) {
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("invalid certificate PEM")
	}
	return x509.ParseCertificate(block.Bytes)
}

// CertificateInventory first proves the issuer independently of wall time.
// An expired issuer is never accepted as an operational renewal authority.
func CertificateInventory(certPEM, caPEM []byte, now time.Time, missing int, cn, eku, sans string) CertificateState {
	result := CertificateState{State: "unreadable"}
	cert, err := parsePEMCertificate(certPEM)
	if err != nil {
		return result
	}
	ca, err := parsePEMCertificate(caPEM)
	if err != nil {
		return result
	}
	fingerprint := sha256.Sum256(cert.Raw)
	encoded := strings.ToUpper(hex.EncodeToString(fingerprint[:]))
	pairs := make([]string, 0, 32)
	for n := 0; n < len(encoded); n += 2 {
		pairs = append(pairs, encoded[n:n+2])
	}
	result.Fingerprint = strings.Join(pairs, ":")
	result.Days = int64(cert.NotAfter.Sub(now).Hours() / 24)
	if !ca.IsCA || cert.CheckSignatureFrom(ca) != nil || !bytes.Equal(cert.RawIssuer, ca.RawSubject) {
		result.State = "foreign"
		return result
	}
	if now.Before(ca.NotBefore) || !now.Before(ca.NotAfter) {
		result.State = "unreadable"
		return result
	}
	if now.Before(cert.NotBefore) || len(cert.UnhandledCriticalExtensions) > 0 {
		result.State = "incompatible"
		return result
	}
	if cn != "" && !certificateRoleMatches(cert, cn, eku, sans) {
		result.State = "incompatible"
		return result
	}
	if missing > 0 {
		result.State = "partial"
		return result
	}
	if !now.Before(cert.NotAfter) {
		result.State = "expired"
		return result
	}
	result.State = "present"
	return result
}

func certificateRoleMatches(cert *x509.Certificate, cn, eku, sans string) bool {
	if cert.IsCA || cert.Subject.CommonName != cn || len(cert.ExtKeyUsage) != 1 || len(cert.UnknownExtKeyUsage) != 0 {
		return false
	}
	wanted := x509.ExtKeyUsageClientAuth
	if eku == "serverAuth" {
		wanted = x509.ExtKeyUsageServerAuth
	} else if eku != "clientAuth" {
		return false
	}
	if cert.ExtKeyUsage[0] != wanted {
		return false
	}
	actual := map[string]bool{}
	for _, value := range cert.DNSNames {
		actual["DNS:"+value] = true
	}
	for _, value := range cert.IPAddresses {
		actual["IP:"+value.String()] = true
	}
	for _, value := range cert.URIs {
		actual["URI:"+value.String()] = true
	}
	if len(cert.EmailAddresses) > 0 {
		return false
	}
	expected := map[string]bool{}
	if sans != "-" && sans != "" {
		for _, value := range strings.Split(sans, ",") {
			expected[value] = true
		}
	}
	if len(expected) != len(actual) {
		return false
	}
	for value := range expected {
		if !actual[value] {
			return false
		}
	}
	return true
}

func BundleConsistent(bundle IdentityBundle) error {
	for _, file := range identityFiles {
		if len(bundle[file.File]) == 0 {
			return errors.New("partial identity bundle")
		}
	}
	cert, err := parsePEMCertificate(bundle["cert.pem"])
	if err != nil {
		return errors.New("inconsistent identity bundle")
	}
	ca, err := parsePEMCertificate(bundle["ca.pem"])
	if err != nil || !bytes.Equal(bundle["cert.der"], cert.Raw) || !bytes.Equal(bundle["ca.der"], ca.Raw) || cert.CheckSignatureFrom(ca) != nil {
		return errors.New("inconsistent identity bundle")
	}
	block, rest := pem.Decode(bundle["key.pem"])
	if block == nil || len(bytes.TrimSpace(rest)) != 0 {
		return errors.New("inconsistent identity bundle")
	}
	var key any
	switch block.Type {
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	default:
		return errors.New("inconsistent identity bundle")
	}
	if err != nil {
		return errors.New("inconsistent identity bundle")
	}
	derKey, err := x509.ParsePKCS8PrivateKey(bundle["key.der"])
	if err != nil || !samePublicKey(key, derKey) || !samePublicKey(key, cert.PublicKey) {
		return errors.New("inconsistent identity bundle")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(bundle["password"]), "\n"), "\r")
	p12Key, certs, err := decodeIdentityP12(bundle["identity.p12"], password)
	if err != nil || !samePublicKey(key, p12Key) || len(certs) != 2 {
		return errors.New("inconsistent identity bundle")
	}
	leaf, issuer := false, false
	for _, c := range certs {
		leaf = leaf || bytes.Equal(c.Raw, cert.Raw)
		issuer = issuer || bytes.Equal(c.Raw, ca.Raw)
	}
	if !leaf || !issuer {
		return errors.New("inconsistent identity bundle")
	}
	return nil
}

func samePublicKey(a, b any) bool {
	if signer, ok := a.(crypto.Signer); ok {
		a = signer.Public()
	}
	if signer, ok := b.(crypto.Signer); ok {
		b = signer.Public()
	}
	left, err := x509.MarshalPKIXPublicKey(a)
	if err != nil {
		return false
	}
	right, err := x509.MarshalPKIXPublicKey(b)
	return err == nil && bytes.Equal(left, right)
}

// ReadProtectedBundle never follows symlinks or prints private material.
type IdentityCustody struct{ root *os.Root }

func OpenIdentityCustody(directory string) (*IdentityCustody, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	for path := absolute; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("unreadable protected identity directory")
		}
		if path == absolute && (!info.IsDir() || info.Mode().Perm() != 0700 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid())) {
			return nil, errors.New("unprotected identity directory")
		}
		if filepath.Dir(path) == path {
			break
		}
	}
	root, err := os.OpenRoot(absolute)
	if err != nil {
		return nil, errors.New("identity directory unreadable")
	}
	return &IdentityCustody{root: root}, nil
}

func (c *IdentityCustody) Close() error { return c.root.Close() }

func (c *IdentityCustody) Read() (IdentityBundle, error) {
	bundle := IdentityBundle{}
	for _, file := range identityFiles {
		handle, err := c.root.OpenFile(file.File, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, errors.New("unreadable identity file")
		}
		info, err := handle.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Sys().(*syscall.Stat_t).Nlink != 1 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
			handle.Close()
			return nil, errors.New("unprotected identity file")
		}
		data, err := io.ReadAll(io.LimitReader(handle, (1<<20)+1))
		handle.Close()
		if err != nil || len(data) > 1<<20 {
			return nil, errors.New("unreadable identity file")
		}
		bundle[file.File] = data
	}
	return bundle, nil
}

func ReadProtectedBundle(directory string) (IdentityBundle, error) {
	custody, err := OpenIdentityCustody(directory)
	if err != nil {
		return nil, err
	}
	defer custody.Close()
	return custody.Read()
}

func InspectBundleReport(report io.Reader, ca []byte, now time.Time, cn, eku, sans string) CertificateState {
	data, err := io.ReadAll(io.LimitReader(report, 1<<20))
	if err != nil {
		return CertificateState{State: "unreadable"}
	}
	status, cert, _ := strings.Cut(string(data), "\n")
	switch status {
	case "bundle=absent":
		return CertificateState{State: "absent"}
	case "bundle=partial":
		return CertificateState{State: "partial"}
	case "bundle=inconsistent":
		return CertificateState{State: "inconsistent"}
	case "bundle=consistent":
		return CertificateInventory([]byte(cert), ca, now, 0, cn, eku, sans)
	default:
		return CertificateState{State: "unreadable"}
	}
}

func ReadRailwayBundle(r io.Reader, prefix string) (IdentityBundle, error) {
	raw, err := io.ReadAll(io.LimitReader(r, 16<<20))
	if err != nil {
		return nil, errors.New("unreadable custody variables")
	}
	values := map[string]string{}
	if len(bytes.TrimSpace(raw)) > 0 && bytes.TrimSpace(raw)[0] == '[' {
		var entries []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		if json.Unmarshal(raw, &entries) != nil {
			return nil, errors.New("unreadable custody variables")
		}
		for _, entry := range entries {
			if _, exists := values[entry.Name]; exists {
				return nil, errors.New("duplicate custody variable")
			}
			values[entry.Name] = entry.Value
		}
	} else if json.Unmarshal(raw, &values) != nil {
		return nil, errors.New("unreadable custody variables")
	}
	bundle := IdentityBundle{}
	for _, file := range identityFiles {
		encoded, ok := values[prefix+"_"+file.Suffix]
		if !ok || encoded == "" {
			continue
		}
		data, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil {
			return nil, errors.New("inconsistent custody encoding")
		}
		bundle[file.File] = data
	}
	return bundle, nil
}

// BundleReport is the only result transported out of remote custody.
func BundleReport(bundle IdentityBundle, w io.Writer) error {
	if len(bundle) == 0 {
		_, err := fmt.Fprintln(w, "bundle=absent")
		return err
	}
	if len(bundle) != len(identityFiles) {
		_, err := fmt.Fprintln(w, "bundle=partial")
		return err
	}
	if err := BundleConsistent(bundle); err != nil {
		_, writeErr := fmt.Fprintln(w, "bundle=inconsistent")
		return writeErr
	}
	_, err := fmt.Fprintln(w, "bundle=consistent")
	if err == nil {
		_, err = w.Write(bundle["cert.pem"])
	}
	return err
}
