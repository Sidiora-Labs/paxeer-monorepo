package config

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"go/ast"
	"go/parser"
	"go/token"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func mutualIngressMaterials(t *testing.T) map[string]string {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "metadata test authority"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true,
		KeyUsage:    x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:    []string{"metadata.example"},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	cert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	pin := sha256.Sum256(der)
	return map[string]string{
		"METADATA_TLS_CERT_PEM": cert, "METADATA_TLS_CLIENT_CA_PEM": cert,
		"METADATA_TLS_KEY_PEM":               string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})),
		"METADATA_ASSERTION_PRIVATE_KEY_HEX": hex.EncodeToString(private),
		"GATEWAY_ASSERTION_PUBLIC_KEY_HEX":   hex.EncodeToString(public),
		"USER_ASSERTION_PUBLIC_KEY_HEX":      hex.EncodeToString(public),
		"GATEWAY_PEER_CERT_SHA256":           hex.EncodeToString(pin[:]),
		"IDENTITY_PEER_CERT_SHA256":          hex.EncodeToString(pin[:]),
		"USER_PEER_CERT_SHA256":              hex.EncodeToString(pin[:]),
	}
}

func TestMutualIngressContract(t *testing.T) {
	root, values := platformDocumentValues(t)
	composed := composePlatformDocuments(t, root, values)
	setPlatformDocumentEnvironment(t, root, composed)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NodeEnv != productionEnv || cfg.Ingress != IngressMTLS || cfg.RasterIsolation != IsolationRlimit || cfg.ModerationSource != ModerationDatabase {
		t.Fatal("metadata contract must select production mTLS, rlimit and database moderation")
	}
	if err := cfg.ValidateMediaRuntime(); err != nil {
		t.Fatal(err)
	}
	for _, material := range []struct{ file, inline string }{
		{"TLSCertFile", "TLSCertPEM"}, {"TLSKeyFile", "TLSKeyPEM"}, {"TLSClientCAFile", "TLSClientCAPEM"},
	} {
		for _, form := range []string{"both", "neither", "file", "relative"} {
			t.Run(material.inline+"_"+form, func(t *testing.T) {
				copy := cfg
				fields := reflect.ValueOf(&copy).Elem()
				file, inline := fields.FieldByName(material.file), fields.FieldByName(material.inline)
				file.SetString(filepath.Join(t.TempDir(), "material.pem"))
				if err := os.WriteFile(file.String(), []byte(inline.String()), 0o600); err != nil {
					t.Fatal(err)
				}
				switch form {
				case "neither":
					file.SetString("")
					inline.SetString("")
				case "file":
					inline.SetString("")
				case "relative":
					file.SetString("material.pem")
					inline.SetString("")
				}
				err := copy.ValidateMediaRuntime()
				if form == "file" {
					if err != nil {
						t.Fatal(err)
					}
					return
				}
				field, _ := reflect.TypeOf(copy).FieldByName(material.file)
				if err == nil || !strings.Contains(err.Error(), field.Tag.Get("env")) {
					t.Fatalf("%s material must name its source: %v", form, err)
				}
				if strings.Contains(err.Error(), cfg.TLSKeyPEM) {
					t.Fatal("material error contains a private key")
				}
			})
		}
	}

	var contract map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Required []string          `json:"required"`
		Secrets  []string          `json:"secrets"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "deploy/launchpad/railway/variables.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatal(err)
	}
	entry := contract["metadata"]
	names := map[string]bool{}
	for name := range entry.Fixed {
		names[name] = true
	}
	for _, name := range entry.Required {
		names[name] = true
	}
	for _, name := range []string{"METADATA_TLS_KEY_PEM", "METADATA_ASSERTION_PRIVATE_KEY_HEX"} {
		if !strings.Contains(" "+strings.Join(entry.Secrets, " ")+" ", " "+name+" ") {
			t.Fatalf("%s must be a secret", name)
		}
	}
	for _, name := range []string{"IDENTITY_SERVICE_URL", "IDENTITY_SERVICE_TOKEN"} {
		if names[name] {
			t.Fatalf("%s belongs to edge ingress", name)
		}
	}
	// Inspect all configuration fields in runtime validation and listener selection.
	// Inactive inputs belong to edge, service moderation or cgroup isolation.
	inactive := map[string]bool{
		"IdentityServiceURL": true, "IdentityServiceToken": true, "TrustedProxyCIDRs": true,
		"ModerationPeerCertSHA256": true, "ModerationEndpoint": true, "ModerationBearerKey": true,
		"RasterCgroupParent": true,
	}
	alternatives := map[string]string{
		"METADATA_TLS_CERT_FILE": "METADATA_TLS_CERT_PEM", "METADATA_TLS_KEY_FILE": "METADATA_TLS_KEY_PEM",
		"METADATA_TLS_CLIENT_CA_FILE":      "METADATA_TLS_CLIENT_CA_PEM",
		"METADATA_PUBLICATION_CONFIG_FILE": "METADATA_PUBLICATION_CONFIG_DOCUMENT",
	}
	source, err := parser.ParseFile(token.NewFileSet(), "config.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range source.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || (fn.Name.Name != "ValidateMediaRuntime" && fn.Name.Name != "ValidateTLSMaterials" && fn.Name.Name != "tlsMaterials") {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			receiver, ok := selector.X.(*ast.Ident)
			if !ok || receiver.Name != "c" || inactive[selector.Sel.Name] {
				return true
			}
			field, ok := reflect.TypeOf(cfg).FieldByName(selector.Sel.Name)
			if !ok {
				return true
			}
			name := strings.Split(field.Tag.Get("env"), ",")[0]
			if alternate, ok := alternatives[name]; ok {
				name = alternate
			}
			if !names[name] {
				t.Errorf("runtime validation reads %s outside the metadata contract", name)
			}
			return true
		})
	}
}

func TestMutualIngressInlineKeySecrets(t *testing.T) {
	root, values := platformDocumentValues(t)
	setPlatformDocumentEnvironment(t, root, composePlatformDocuments(t, root, values))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	key := cfg.TLSKeyPEM
	cfg.TLSKeyPEM = ""
	check, stop, err := cfg.ResolveProductionSecrets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if cfg.TLSKeyPEM != key {
		t.Fatal("inline listener key was not resolved")
	}
	if err := check(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METADATA_TLS_KEY_PEM", "")
	if _, _, err := cfg.ResolveProductionSecrets(context.Background()); err == nil || err.Error() != "required_secret_unavailable: METADATA_TLS_KEY_PEM" {
		t.Fatalf("missing inline key: %v", err)
	}
	t.Setenv("METADATA_TLS_KEY_PEM", mutualIngressMaterials(t)["METADATA_TLS_KEY_PEM"])
	if err := check(context.Background()); err == nil || err.Error() != "required_secret_rotated_restart_required: METADATA_TLS_KEY_PEM" {
		t.Fatalf("rotated inline key: %v", err)
	}
	if err := check(context.Background()); err == nil {
		t.Fatal("rotated inline key became ready")
	}
}
