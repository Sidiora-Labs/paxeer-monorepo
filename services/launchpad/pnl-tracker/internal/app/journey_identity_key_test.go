package app

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/config"
)

func journeyIdentityKey(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestJourneyIdentityKeyForms(t *testing.T) {
	for _, form := range []string{"file", "variable", "variable at bound"} {
		t.Run(form, func(t *testing.T) {
			cfg, document := compositionIdentity(t)
			key := journeyIdentityKey(t, document.IdentityClientKeyFile)
			if form != "file" {
				cfg.IdentityMTLSClientKeyPEM = key
				document.IdentityClientKeyFile, document.IdentityClientKeySHA256 = "", ""
				if form == "variable at bound" {
					cfg.IdentityMTLSClientKeyPEM += strings.Repeat("\n", 1048576-len(key))
				}
			}
			authority, err := pnlIdentityAuthority(cfg, document)
			if err != nil {
				t.Fatal(err)
			}
			transport := authority.Client.Transport.(*http.Transport)
			defer transport.CloseIdleConnections()
			certificates := transport.TLSClientConfig.Certificates
			if len(certificates) != 1 {
				t.Fatal("identity requires one client certificate")
			}
			block, _ := pem.Decode([]byte(journeyIdentityKey(t, document.IdentityClientCertFile)))
			if block == nil || len(certificates[0].Certificate) != 1 || !bytes.Equal(certificates[0].Certificate[0], block.Bytes) {
				t.Fatal("identity client certificate differs from bound certificate")
			}
		})
	}
}

func TestJourneyIdentityKeyRefusals(t *testing.T) {
	_, other, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherDER, err := x509.MarshalPKCS8PrivateKey(other)
	if err != nil {
		t.Fatal(err)
	}
	otherPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: otherDER}))
	for _, tc := range []struct {
		name   string
		mutate func(*config.Config, *pnlJourneyDocument)
		want   string
	}{
		{"both forms", func(c *config.Config, d *pnlJourneyDocument) {}, "pnl: exactly one identity client key form required"},
		{"both forms before key parsing", func(c *config.Config, d *pnlJourneyDocument) {
			c.IdentityMTLSClientKeyPEM = "private-invalid-key-material"
		}, "pnl: exactly one identity client key form required"},
		{"file member with variable", func(c *config.Config, d *pnlJourneyDocument) { d.IdentityClientKeySHA256 = "" }, "pnl: exactly one identity client key form required"},
		{"digest member with variable", func(c *config.Config, d *pnlJourneyDocument) { d.IdentityClientKeyFile = "" }, "pnl: exactly one identity client key form required"},
		{"no form", func(c *config.Config, d *pnlJourneyDocument) {
			c.IdentityMTLSClientKeyPEM = ""
			d.IdentityClientKeyFile, d.IdentityClientKeySHA256 = "", ""
		}, "pnl: exactly one identity client key form required"},
		{"non PEM variable", func(c *config.Config, d *pnlJourneyDocument) {
			c.IdentityMTLSClientKeyPEM = "private-invalid-key-material"
			d.IdentityClientKeyFile, d.IdentityClientKeySHA256 = "", ""
		}, "pnl: identity client certificate invalid"},
		{"different pair", func(c *config.Config, d *pnlJourneyDocument) {
			c.IdentityMTLSClientKeyPEM = otherPEM
			d.IdentityClientKeyFile, d.IdentityClientKeySHA256 = "", ""
		}, "pnl: identity client certificate invalid"},
		{"variable over bound", func(c *config.Config, d *pnlJourneyDocument) {
			c.IdentityMTLSClientKeyPEM += strings.Repeat("\n", 1048577-len(c.IdentityMTLSClientKeyPEM))
			d.IdentityClientKeyFile, d.IdentityClientKeySHA256 = "", ""
		}, "pnl: identity client key exceeds size bound"},
		{"file without digest", func(c *config.Config, d *pnlJourneyDocument) {
			c.IdentityMTLSClientKeyPEM = ""
			d.IdentityClientKeySHA256 = ""
		}, "pnl: identity file binding missing"},
		{"digest without file", func(c *config.Config, d *pnlJourneyDocument) {
			c.IdentityMTLSClientKeyPEM = ""
			d.IdentityClientKeyFile = ""
		}, "pnl: identity file binding missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, document := compositionIdentity(t)
			cfg.IdentityMTLSClientKeyPEM = journeyIdentityKey(t, document.IdentityClientKeyFile)
			tc.mutate(&cfg, &document)
			_, err := pnlIdentityAuthority(cfg, document)
			if err == nil || err.Error() != tc.want {
				t.Fatal("identity key refusal differs from contract")
			}
			for _, part := range strings.Fields(cfg.IdentityMTLSClientKeyPEM) {
				if strings.Contains(err.Error(), part) {
					t.Fatal("identity key refusal exposes variable material")
				}
			}
		})
	}
}

func TestJourneyIdentityKeyComposedRuntime(t *testing.T) {
	journeyDocumentsConstructActualRuntime(t, true)
}
