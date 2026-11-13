package config_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/config"
)

func TestPublicationDocumentValidation(t *testing.T) {
	for _, tc := range []struct {
		name, document string
		valid          bool
	}{
		{"valid", publicationDocument, true},
		{"reserved header", `{"schemaVersion":3,"environment":"production","envelopeHeader":"X-Request-ID","signatureHeader":"X-Publication-Signature"}`, false},
		{"duplicate headers", `{"schemaVersion":3,"environment":"production","envelopeHeader":"X-Publication-Envelope","signatureHeader":"x-publication-envelope"}`, false},
		{"unknown field", strings.TrimSuffix(publicationDocument, "}") + `,"extra":true}`, false},
		{"trailing JSON", publicationDocument + ` {}`, false},
		{"invalid version", strings.Replace(publicationDocument, `"schemaVersion":3`, `"schemaVersion":2`, 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := edgeConfig()
			cfg.PublicationConfigDocument = base64.StdEncoding.EncodeToString([]byte(tc.document))
			cfg.PublicationConfigSHA256 = digest([]byte(tc.document))
			runtime, key, err := cfg.ReadPublicationRuntime()
			if tc.valid {
				if err != nil || runtime.SchemaVersion != 3 || len(key) != 32 {
					t.Fatalf("publication runtime = %+v, key bytes %d, %v", runtime, len(key), err)
				}
			} else if err == nil {
				t.Fatalf("accepted publication document: %+v", runtime)
			}
		})
	}
	invalidKey := edgeConfig()
	invalidKey.PublicationAuditKey = base64.StdEncoding.EncodeToString([]byte("short"))
	if _, _, err := invalidKey.ReadPublicationRuntime(); err == nil || err.Error() != "metadata_publication_audit_key_invalid" {
		t.Fatalf("publication audit binding = %v", err)
	}

	path := filepath.Join(t.TempDir(), "publication.json")
	if err := os.WriteFile(path, []byte(publicationDocument), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "publication-link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	var decoded config.PublicationRuntime
	if err := config.ReadBoundDocument(link, digest([]byte(publicationDocument)), &decoded); err == nil {
		t.Fatal("symbolic link accepted as a bound document")
	}
	if err := config.ReadBoundDocument(path, digest([]byte(publicationDocument)), &decoded); err != nil || decoded.SchemaVersion != 3 {
		t.Fatalf("bound document = %+v, %v", decoded, err)
	}
}
