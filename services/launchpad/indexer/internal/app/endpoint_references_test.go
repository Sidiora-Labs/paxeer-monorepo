package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
)

func TestEndpointReferenceOrigin(t *testing.T) {
	_, lifecycle, _ := lifecycleDocuments(t)
	data, err := os.ReadFile(filepath.Join(coordinatortest.RepositoryRoot(t), "deploy/launchpad/documents/values/indexer.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	var values struct {
		PrimaryEndpoint string `json:"primary_endpoint"`
	}
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatal(err)
	}
	documentAddress := values.PrimaryEndpoint
	if documentAddress == "" {
		t.Fatal("primary endpoint address required")
	}
	lifecycle.Committed.Endpoints = map[string]string{"paxeer-primary": documentAddress}
	lifecycle.Committed.Endpoints["secondary"] = documentAddress + "/document-secondary"
	lifecycle.Committed.Endpoints["document-only"] = documentAddress + "/document-only"
	lifecycle.EndpointSecretReferences["secondary"] = "INDEXER_SECONDARY_ENDPOINT"
	provider, err := secrets.NewChecked(secrets.Options{Provider: secrets.ProviderEnv})
	if err != nil {
		t.Fatal(err)
	}
	privateParts := []string{"primary-resolved-address", "secondary-resolved-address"}
	for endpoint, reference := range lifecycle.EndpointSecretReferences {
		part := privateParts[0]
		if endpoint == "secondary" {
			part = privateParts[1]
		}
		t.Setenv(reference, documentAddress+"/"+part)
		value, err := provider.GetRequired(context.Background(), reference)
		if err != nil {
			t.Fatal(err)
		}
		privateParts = append(privateParts, value)
	}
	for _, format := range []string{"text", "json"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			var handler slog.Handler = slog.NewTextHandler(&output, nil)
			if format == "json" {
				handler = slog.NewJSONHandler(&output, nil)
			}
			lifecycle.logEndpointReferences(slog.New(handler))
			lines := strings.Split(strings.TrimSpace(output.String()), "\n")
			if len(lines) != len(lifecycle.EndpointSecretReferences) {
				t.Fatalf("endpoint reference line count = %d, want %d", len(lines), len(lifecycle.EndpointSecretReferences))
			}
			seen := map[string]bool{}
			for _, line := range lines {
				if format == "json" {
					var record map[string]string
					if err := json.Unmarshal([]byte(line), &record); err != nil {
						t.Fatal(err)
					}
					endpoint := record["endpoint_id"]
					reference, exists := lifecycle.EndpointSecretReferences[endpoint]
					if !exists || seen[endpoint] || record["secret_reference"] != reference || record["level"] != "INFO" || record["msg"] != "committed endpoint reference" || len(record) != 5 {
						t.Fatalf("unexpected endpoint reference record: %v", record)
					}
					seen[endpoint] = true
				} else {
					if !strings.Contains(line, "level=INFO") {
						t.Fatal("endpoint reference must use info level")
					}
					for endpoint, reference := range lifecycle.EndpointSecretReferences {
						if strings.Contains(line, "endpoint_id="+endpoint+" ") && strings.HasSuffix(line, "secret_reference="+reference) {
							if seen[endpoint] {
								t.Fatalf("duplicate endpoint reference: %s", endpoint)
							}
							seen[endpoint] = true
						}
					}
				}
			}
			if len(seen) != len(lifecycle.EndpointSecretReferences) {
				t.Fatalf("endpoint reference coverage = %d, want %d", len(seen), len(lifecycle.EndpointSecretReferences))
			}
			for _, address := range lifecycle.Committed.Endpoints {
				if strings.Contains(output.String(), address) {
					t.Fatal("endpoint reference log contains a document address")
				}
			}
			for _, part := range privateParts {
				if strings.Contains(output.String(), part) {
					t.Fatal("endpoint reference log contains a resolved address or its private path")
				}
			}
		})
	}
	for _, references := range []map[string]string{nil, {}} {
		var output bytes.Buffer
		lifecycle.EndpointSecretReferences = references
		lifecycle.logEndpointReferences(slog.New(slog.NewJSONHandler(&output, nil)))
		if output.Len() != 0 {
			t.Fatal("endpoints without references must not emit reference lines")
		}
	}
}
