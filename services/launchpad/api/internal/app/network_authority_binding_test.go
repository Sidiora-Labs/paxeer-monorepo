package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/capabilities"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/rpc"
)

func TestNetworkAuthorityBindsReviewedManifestAndAddresses(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client, err := rpc.DialOptions(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := capabilities.NewPinnedRPCReader(client)
	if err != nil {
		t.Fatal(err)
	}
	manifest := deployment.Current()
	engine, err := capabilities.NewPreflightEngine(reader, manifest, developmentPoolRuntime, developmentSerialInterface, time.Minute, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
	baseline := networkAuthorityConfig{SchemaVersion: 2, Manifest: manifest, ReviewedPoolRuntime: developmentPoolRuntime, SerialInterfaceSHA256: developmentSerialInterface, ExpectedRevisionSHA256: engine.Revision().RevisionSHA256, SourceSchema: "indexer", GoAddresses: manifest.Addresses(), FrontendAddresses: manifest.Addresses()}
	bound := func(t *testing.T, document networkAuthorityConfig, endpoint string) *config.Config {
		t.Helper()
		raw, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "network-authority.json")
		if err = os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return &config.Config{NetworkAuthorityFile: path, NetworkAuthoritySHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), NetworkRPCURL: endpoint}
	}
	for _, check := range []struct {
		name   string
		change func(*networkAuthorityConfig)
		url    string
	}{
		{"schema version", func(d *networkAuthorityConfig) { d.SchemaVersion = 1 }, server.URL},
		{"source schema", func(d *networkAuthorityConfig) { d.SourceSchema = "invalid-name" }, server.URL},
		{"reviewed runtime", func(d *networkAuthorityConfig) { d.ReviewedPoolRuntime = "invalid" }, server.URL},
		{"revision binding", func(d *networkAuthorityConfig) { d.ExpectedRevisionSHA256 = strings.Repeat("0", 64) }, server.URL},
		{"go address binding", func(d *networkAuthorityConfig) { d.GoAddresses = map[string]string{} }, server.URL},
		{"frontend address binding", func(d *networkAuthorityConfig) { d.FrontendAddresses = map[string]string{} }, server.URL},
		{"rpc transport", func(*networkAuthorityConfig) {}, strings.Replace(server.URL, "https://", "http://", 1)},
	} {
		t.Run(check.name, func(t *testing.T) {
			document := baseline
			check.change(&document)
			authority, err := newNetworkAuthority(context.Background(), nil, bound(t, document, check.url))
			if authority != nil {
				authority.Close()
			}
			if err == nil {
				t.Fatal("unbound network authority admitted")
			}
		})
	}
	authority, err := newNetworkAuthority(context.Background(), nil, bound(t, baseline, server.URL))
	if err != nil || authority == nil || authority.engine == nil || authority.reader == nil || authority.config.ExpectedRevisionSHA256 != baseline.ExpectedRevisionSHA256 {
		t.Fatalf("bound authority: %+v, %v", authority, err)
	}
	authority.Close()
}
